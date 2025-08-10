package webrtc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/HMasataka/conic/logging"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

var (
	ErrVideoTrackClosed   = errors.New("video track closed")
	ErrVideoTrackNotReady = errors.New("video track not ready")
	ErrInvalidVideoFormat = errors.New("invalid video format")
	ErrNoVideoCodec       = errors.New("no video codec available")
)

type VideoTrackStats struct {
	PacketsSent     uint64
	PacketsReceived uint64
	BytesSent       uint64
	BytesReceived   uint64
	FramesSent      uint64
	FramesReceived  uint64
	Width           uint32
	Height          uint32
	FrameRate       uint32
	CodecName       string
}

type VideoTrack struct {
	id          string
	localTrack  *webrtc.TrackLocalStaticSample
	remoteTrack *webrtc.TrackRemote
	stats       VideoTrackStats
	mu          sync.RWMutex
	closed      bool
	onSample    func(*media.Sample)
	logger      *logging.Logger
}

func NewVideoTrack(id string, codecCapability webrtc.RTPCodecCapability) (*VideoTrack, error) {
	logger := logging.FromContext(context.Background())

	localTrack, err := webrtc.NewTrackLocalStaticSample(
		codecCapability,
		id,
		"video-stream-"+id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create local video track: %w", err)
	}

	return &VideoTrack{
		id:         id,
		localTrack: localTrack,
		stats: VideoTrackStats{
			CodecName: codecCapability.MimeType,
		},
		logger: logger,
	}, nil
}

func (vt *VideoTrack) ID() string {
	return vt.id
}

func (vt *VideoTrack) LocalTrack() *webrtc.TrackLocalStaticSample {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return vt.localTrack
}

func (vt *VideoTrack) WriteSample(sample *media.Sample) error {
	vt.mu.Lock()
	if vt.closed {
		vt.mu.Unlock()
		return ErrVideoTrackClosed
	}
	if vt.localTrack == nil {
		vt.mu.Unlock()
		return ErrVideoTrackNotReady
	}
	vt.stats.PacketsSent++
	vt.stats.BytesSent += uint64(len(sample.Data))
	vt.stats.FramesSent++
	vt.mu.Unlock()

	return vt.localTrack.WriteSample(*sample)
}

func (vt *VideoTrack) SetRemoteTrack(remoteTrack *webrtc.TrackRemote) {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	vt.remoteTrack = remoteTrack
	vt.stats.CodecName = remoteTrack.Codec().MimeType
}

func (vt *VideoTrack) ReadSamples(ctx context.Context) error {
	vt.mu.RLock()
	if vt.closed {
		vt.mu.RUnlock()
		return ErrVideoTrackClosed
	}
	if vt.remoteTrack == nil {
		vt.mu.RUnlock()
		return ErrVideoTrackNotReady
	}
	remoteTrack := vt.remoteTrack
	vt.mu.RUnlock()

	// Increase buffer size for RTP packets
	// Maximum RTP packet size is 65535 bytes (UDP limit)
	const maxRTPPacketSize = 65535
	buffer := make([]byte, maxRTPPacketSize)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Try reading with a large buffer first
			n, _, err := remoteTrack.Read(buffer)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				// If short buffer error, try ReadRTP as fallback
				if err.Error() == "short buffer" || err.Error() == "failed to read from packetio.Buffer short buffer" {
					vt.logger.Debug("short buffer error, trying ReadRTP")
					rtpPacket, _, rtpErr := remoteTrack.ReadRTP()
					if rtpErr != nil {
						return fmt.Errorf("failed to read RTP packet: %w", rtpErr)
					}
					// Process RTP packet
					sample := &media.Sample{
						Data:            rtpPacket.Payload,
						Duration:        33 * time.Millisecond, // ~30fps
						PacketTimestamp: rtpPacket.Timestamp,
					}
					vt.processSample(sample, uint64(len(rtpPacket.Payload)))
					continue
				}
				return fmt.Errorf("failed to read RTP data: %w", err)
			}

			// Create a sample from the buffer data
			sample := &media.Sample{
				Data:     make([]byte, n),
				Duration: 33 * time.Millisecond, // ~30fps
			}
			copy(sample.Data, buffer[:n])

			vt.processSample(sample, uint64(n))
		}
	}
}

func (vt *VideoTrack) processSample(sample *media.Sample, byteCount uint64) {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	vt.stats.PacketsReceived++
	vt.stats.BytesReceived += byteCount
	vt.stats.FramesReceived++
	if vt.onSample != nil {
		vt.onSample(sample)
	}
}

func (vt *VideoTrack) OnSample(fn func(*media.Sample)) {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	vt.onSample = fn
}

func (vt *VideoTrack) Stats() VideoTrackStats {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return vt.stats
}

func (vt *VideoTrack) Close() error {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	if vt.closed {
		return nil
	}

	vt.closed = true
	vt.logger.Info("Video track closed",
		"id", vt.id,
		"packets_sent", vt.stats.PacketsSent,
		"packets_received", vt.stats.PacketsReceived,
		"frames_sent", vt.stats.FramesSent,
		"frames_received", vt.stats.FramesReceived,
	)

	return nil
}

func GetVP8Codec() webrtc.RTPCodecCapability {
	return webrtc.RTPCodecCapability{
		MimeType:     webrtc.MimeTypeVP8,
		ClockRate:    90000,
		Channels:     0,
		SDPFmtpLine:  "",
		RTCPFeedback: nil,
	}
}

func CreateVP8MediaEngine() (*webrtc.MediaEngine, error) {
	m := &webrtc.MediaEngine{}

	vp8Codec := GetVP8Codec()
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: vp8Codec,
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("failed to register VP8 codec: %w", err)
	}

	return m, nil
}

func CreateAudioVideoMediaEngine() (*webrtc.MediaEngine, error) {
	m := &webrtc.MediaEngine{}

	// Register Opus codec for audio
	opusCodec := GetOpusCodec()
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: opusCodec,
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("failed to register Opus codec: %w", err)
	}

	// Register VP8 codec for video
	vp8Codec := GetVP8Codec()
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: vp8Codec,
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("failed to register VP8 codec: %w", err)
	}

	return m, nil
}