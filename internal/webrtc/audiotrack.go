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
	ErrAudioTrackClosed   = errors.New("audio track closed")
	ErrAudioTrackNotReady = errors.New("audio track not ready")
	ErrInvalidAudioFormat = errors.New("invalid audio format")
	ErrNoAudioCodec       = errors.New("no audio codec available")
)

type AudioTrackStats struct {
	PacketsSent     uint64
	PacketsReceived uint64
	BytesSent       uint64
	BytesReceived   uint64
	SampleRate      uint32
	Channels        uint8
	CodecName       string
}

type AudioTrack struct {
	id          string
	localTrack  *webrtc.TrackLocalStaticSample
	remoteTrack *webrtc.TrackRemote
	stats       AudioTrackStats
	mu          sync.RWMutex
	closed      bool
	onSample    func(*media.Sample)
	logger      *logging.Logger
}

func NewAudioTrack(id string, codecCapability webrtc.RTPCodecCapability) (*AudioTrack, error) {
	logger := logging.FromContext(context.Background())

	localTrack, err := webrtc.NewTrackLocalStaticSample(
		codecCapability,
		id,
		"audio-stream-"+id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create local audio track: %w", err)
	}

	return &AudioTrack{
		id:         id,
		localTrack: localTrack,
		stats: AudioTrackStats{
			CodecName:  codecCapability.MimeType,
			SampleRate: codecCapability.ClockRate,
			Channels:   uint8(codecCapability.Channels),
		},
		logger: logger,
	}, nil
}

func (at *AudioTrack) ID() string {
	return at.id
}

func (at *AudioTrack) LocalTrack() *webrtc.TrackLocalStaticSample {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.localTrack
}

func (at *AudioTrack) WriteSample(sample *media.Sample) error {
	at.mu.Lock()
	if at.closed {
		at.mu.Unlock()
		return ErrAudioTrackClosed
	}
	if at.localTrack == nil {
		at.mu.Unlock()
		return ErrAudioTrackNotReady
	}
	at.stats.PacketsSent++
	at.stats.BytesSent += uint64(len(sample.Data))
	at.mu.Unlock()

	return at.localTrack.WriteSample(*sample)
}

func (at *AudioTrack) SetRemoteTrack(remoteTrack *webrtc.TrackRemote) {
	at.mu.Lock()
	defer at.mu.Unlock()

	at.remoteTrack = remoteTrack
	at.stats.CodecName = remoteTrack.Codec().MimeType
	at.stats.SampleRate = remoteTrack.Codec().ClockRate
	at.stats.Channels = uint8(remoteTrack.Codec().Channels)
}

func (at *AudioTrack) ReadSamples(ctx context.Context) error {
	at.mu.RLock()
	if at.closed {
		at.mu.RUnlock()
		return ErrAudioTrackClosed
	}
	if at.remoteTrack == nil {
		at.mu.RUnlock()
		return ErrAudioTrackNotReady
	}
	remoteTrack := at.remoteTrack
	at.mu.RUnlock()

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
					at.logger.Debug("short buffer error, trying ReadRTP")
					rtpPacket, _, rtpErr := remoteTrack.ReadRTP()
					if rtpErr != nil {
						return fmt.Errorf("failed to read RTP packet: %w", rtpErr)
					}
					// Process RTP packet
					sample := &media.Sample{
						Data:            rtpPacket.Payload,
						Duration:        20 * time.Millisecond,
						PacketTimestamp: rtpPacket.Timestamp,
					}
					at.processSample(sample, uint64(len(rtpPacket.Payload)))
					continue
				}
				return fmt.Errorf("failed to read RTP data: %w", err)
			}

			// Create a sample from the buffer data
			sample := &media.Sample{
				Data:     make([]byte, n),
				Duration: 20 * time.Millisecond,
			}
			copy(sample.Data, buffer[:n])

			at.processSample(sample, uint64(n))
		}
	}
}

func (at *AudioTrack) processSample(sample *media.Sample, byteCount uint64) {
	at.mu.Lock()
	defer at.mu.Unlock()

	at.stats.PacketsReceived++
	at.stats.BytesReceived += byteCount
	if at.onSample != nil {
		at.onSample(sample)
	}
}

func (at *AudioTrack) OnSample(fn func(*media.Sample)) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.onSample = fn
}

func (at *AudioTrack) Stats() AudioTrackStats {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.stats
}

func (at *AudioTrack) Close() error {
	at.mu.Lock()
	defer at.mu.Unlock()

	if at.closed {
		return nil
	}

	at.closed = true
	at.logger.Info("Audio track closed",
		"id", at.id,
		"packets_sent", at.stats.PacketsSent,
		"packets_received", at.stats.PacketsReceived,
	)

	return nil
}

func GetOpusCodec() webrtc.RTPCodecCapability {
	return webrtc.RTPCodecCapability{
		MimeType:     webrtc.MimeTypeOpus,
		ClockRate:    48000,
		Channels:     2,
		SDPFmtpLine:  "minptime=10;useinbandfec=1",
		RTCPFeedback: nil,
	}
}

func CreateOpusMediaEngine() (*webrtc.MediaEngine, error) {
	m := &webrtc.MediaEngine{}

	opusCodec := GetOpusCodec()
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: opusCodec,
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("failed to register Opus codec: %w", err)
	}

	return m, nil
}
