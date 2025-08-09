package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/internal/protocol"
	"github.com/HMasataka/conic/internal/transport"
	webrtcinternal "github.com/HMasataka/conic/internal/webrtc"
	"github.com/HMasataka/conic/logging"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/rs/xid"
)

var (
	addr = flag.String("addr", "localhost:3000", "http service address")
	role = flag.String("role", "offer", "role: offer, answer")
)

func main() {
	flag.Parse()

	logger := logging.New(logging.Config{
		Level:  "debug",
		Format: "text",
	})

	id := xid.New().String()
	logger.Info("Creating peer connection with Opus audio support", "id", id)

	u := url.URL{
		Scheme: "ws",
		Host:   *addr,
		Path:   "/ws",
	}

	logger.Info("Connecting to WebSocket", "url", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		logger.Error("Failed to connect to WebSocket", "error", err)
		return
	}
	defer conn.Close()

	pc, err := webrtcinternal.NewPeerConnection(id, webrtcinternal.DefaultPeerConnectionOptions(logger))
	if err != nil {
		logger.Error("Failed to create peer connection", "error", err)
		return
	}

	pc.OnICECandidate(webrtcinternal.OnIceCandidate(conn, pc))

	router := protocol.NewPeerRouter(pc, logger)

	client := transport.NewClient(conn, router, logger, transport.DefaultClientOptions(id))
	go client.Start(context.Background())

	logger.Info("Client started", "id", client.ID())

	// Wait for WebSocket connection to be fully established
	time.Sleep(500 * time.Millisecond)

	// Register with server
	if err := registerToServer(pc, client, logger); err != nil {
		logger.Error("Failed to register with server", "error", err)
		return
	}

	switch *role {
	case "offer":
		runOfferMode(pc, client, logger)
	case "answer":
		runAnswerMode(pc, logger)
	default:
		logger.Error("Invalid role specified", "role", *role)
	}
}

// registerToServer sends registration message to the server
func registerToServer(pc *webrtcinternal.PeerConnection, client *transport.Client, logger *logging.Logger) error {
	regReq := domain.RegisterRequest{
		ClientID: pc.ID(),
	}

	regData, err := json.Marshal(regReq)
	if err != nil {
		return err
	}

	regMsg := domain.Message{
		ID:        xid.New().String(),
		Type:      domain.MessageTypeRegisterRequest,
		Timestamp: time.Now(),
		Data:      regData,
	}

	regMsgData, err := json.Marshal(regMsg)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := client.Send(ctx, regMsgData); err != nil {
		logger.Error("Failed to send registration message, continuing anyway", "error", err)
		return nil // Don't fail completely, just warn
	}

	logger.Info("Registration message sent", "id", pc.ID())
	return nil
}

func runOfferMode(pc *webrtcinternal.PeerConnection, client *transport.Client, logger *logging.Logger) {
	logger.Info("Running in offer mode with audio")

	var targetID string

	log.Println("Enter target peer ID to create offer:")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		targetID = input
		break
	}

	pc.SetTargetID(targetID)

	// Create audio track
	audioTrack, err := webrtcinternal.NewAudioTrack("audio-"+xid.New().String(), webrtcinternal.GetOpusCodec())
	if err != nil {
		log.Fatal("Failed to create audio track:", err)
	}

	// Add audio track to peer connection
	_, err = pc.AddAudioTrack(audioTrack)
	if err != nil {
		log.Fatal("Failed to add audio track:", err)
	}

	logger.Info("Audio track added", "track_id", audioTrack.ID())

	// Create data channel for control messages
	const dataChannelLabel = "control"
	dataChannel, err := pc.CreateDataChannel(dataChannelLabel, nil)
	if err != nil {
		log.Fatal("create data channel:", err)
	}

	dataChannel.OnMessage(func(data []byte) {
		log.Printf("📩 Control message: %s", string(data))
	})

	dataChannel.OnOpen(func() {
		log.Printf("📨 Control channel '%s' is open", dataChannel.Label())
	})

	// Create offer
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		log.Fatal("create offer:", err)
	}

	sdpMessage := domain.SDPMessage{
		FromID:             pc.ID(),
		ToID:               targetID,
		SessionDescription: offer,
	}

	data, err := json.Marshal(sdpMessage)
	if err != nil {
		log.Fatal("marshal SDP message:", err)
	}

	req := domain.Message{
		ID:        xid.New().String(),
		Type:      domain.MessageTypeSDP,
		Timestamp: time.Now(),
		Data:      data,
	}

	msg, err := json.Marshal(req)
	if err != nil {
		log.Fatal("marshal message:", err)
	}

	ctx := context.Background()
	if err := client.Send(ctx, msg); err != nil {
		log.Fatal("send message:", err)
	}

	// Start sending audio samples (sine wave for testing)
	go func() {
		time.Sleep(3 * time.Second) // Wait for connection to stabilize
		logger.Info("Starting audio transmission")

		sampleRate := uint32(48000)
		channelCount := uint16(2)
		frameSize := uint32(960) // 20ms at 48kHz
		frequency := 440.0       // A4 note

		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()

		sampleCount := uint32(0)

		for range ticker.C {
			// Generate sine wave samples
			samples := make([]int16, frameSize*uint32(channelCount))
			for i := uint32(0); i < frameSize; i++ {
				t := float64(sampleCount+i) / float64(sampleRate)
				value := int16(math.Sin(2*math.Pi*frequency*t) * 32767 * 0.3)

				// Stereo: same value for both channels
				samples[i*uint32(channelCount)] = value
				samples[i*uint32(channelCount)+1] = value
			}

			// Convert to bytes
			data := make([]byte, len(samples)*2)
			for i, sample := range samples {
				data[i*2] = byte(sample)
				data[i*2+1] = byte(sample >> 8)
			}

			sample := &media.Sample{
				Data:     data,
				Duration: 20 * time.Millisecond,
			}

			if err := audioTrack.WriteSample(sample); err != nil {
				logger.Error("Failed to write audio sample", "error", err)
			}

			sampleCount += frameSize
		}
	}()

	log.Println("Audio transmission started... Press Enter to display stats or 'q' to quit")

	for {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())

		if input == "q" || input == "quit" {
			break
		}

		// Display audio track stats
		stats := audioTrack.Stats()
		fmt.Printf("\n=== Audio Track Stats ===\n")
		fmt.Printf("Codec: %s\n", stats.CodecName)
		fmt.Printf("Sample Rate: %d Hz\n", stats.SampleRate)
		fmt.Printf("Channels: %d\n", stats.Channels)
		fmt.Printf("Packets Sent: %d\n", stats.PacketsSent)
		fmt.Printf("Bytes Sent: %d\n", stats.BytesSent)
		fmt.Printf("========================\n\n")
	}
}

func runAnswerMode(pc *webrtcinternal.PeerConnection, logger *logging.Logger) {
	logger.Info("Running in answer mode - waiting for audio")

	// Set up track handler to receive audio
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		logger.Info("Track received",
			"track_id", track.ID(),
			"codec", track.Codec().MimeType,
			"channels", track.Codec().Channels,
			"clock_rate", track.Codec().ClockRate,
		)

		if track.Kind() == webrtc.RTPCodecTypeAudio {
			audioTrack, exists := pc.GetAudioTrack(track.ID())
			if !exists {
				logger.Error("Audio track not found", "track_id", track.ID())
				return
			}

			// Set up audio sample handler
			audioTrack.OnSample(func(sample *media.Sample) {
				// For demo purposes, just log reception
				// In a real application, you would decode and play the audio
			})

			// Display stats periodically
			go func() {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()

				for range ticker.C {
					stats := audioTrack.Stats()
					logger.Info("Audio receiving stats",
						"packets", stats.PacketsReceived,
						"bytes", stats.BytesReceived,
					)
				}
			}()
		}
	})

	// Set up data channel handler
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("📨 Data channel '%s' is open", dc.Label())

		dataChannel := webrtcinternal.NewDataChannel(dc, logger)

		dataChannel.OnMessage(func(data []byte) {
			log.Printf("📩 Control message: %s", string(data))
		})
	})

	log.Println("Waiting for audio stream... Press Enter to display stats or 'q' to quit")

	for {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())

		if input == "q" || input == "quit" {
			break
		}

		// Display all audio track stats
		fmt.Printf("\n=== Audio Reception Stats ===\n")
		// Note: In a real implementation, you would iterate through received tracks
		fmt.Printf("Waiting for audio tracks...\n")
		fmt.Printf("============================\n\n")
	}
}
