package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/internal/protocol"
	"github.com/HMasataka/conic/internal/transport"
	"github.com/HMasataka/conic/internal/video"
	webrtcinternal "github.com/HMasataka/conic/internal/webrtc"
	"github.com/HMasataka/conic/logging"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/rs/xid"
)

var (
	addr     = flag.String("addr", "localhost:3000", "http service address")
	role     = flag.String("role", "offer", "role: offer, answer")
	yuvFile  = flag.String("yuv", "", "YUV file to play (optional, uses test pattern if not specified)")
)

func main() {
	flag.Parse()

	logger := logging.New(logging.Config{
		Level:  "debug",
		Format: "text",
	})

	id := xid.New().String()
	logger.Info("Creating peer connection with VP8 video support", "id", id)

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
	logger.Info("Running in offer mode with video")

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

	// Create video track
	videoTrack, err := webrtcinternal.NewVideoTrack("video-"+xid.New().String(), webrtcinternal.GetVP8Codec())
	if err != nil {
		log.Fatal("Failed to create video track:", err)
	}

	// Add video track to peer connection
	_, err = pc.AddVideoTrack(videoTrack)
	if err != nil {
		log.Fatal("Failed to add video track:", err)
	}

	logger.Info("Video track added", "track_id", videoTrack.ID())

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

	// Start sending video frames
	if *yuvFile != "" {
		// Play YUV file
		go playYUVFile(*yuvFile, videoTrack, logger)
	} else {
		// Generate test pattern
		go generateTestPattern(videoTrack, logger)
	}

	log.Println("Video transmission started... Press Enter to display stats or 'q' to quit")

	for {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())

		if input == "q" || input == "quit" {
			break
		}

		// Display video track stats
		stats := videoTrack.Stats()
		fmt.Printf("\n=== Video Track Stats ===\n")
		fmt.Printf("Codec: %s\n", stats.CodecName)
		fmt.Printf("Packets Sent: %d\n", stats.PacketsSent)
		fmt.Printf("Bytes Sent: %d\n", stats.BytesSent)
		fmt.Printf("Frames Sent: %d\n", stats.FramesSent)
		fmt.Printf("========================\n\n")
	}
}

func runAnswerMode(pc *webrtcinternal.PeerConnection, logger *logging.Logger) {
	logger.Info("Running in answer mode - waiting for video")

	// Set up track handler to receive video
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		logger.Info("Track received",
			"track_id", track.ID(),
			"codec", track.Codec().MimeType,
			"kind", track.Kind().String(),
		)

		if track.Kind() == webrtc.RTPCodecTypeVideo {
			videoTrack, exists := pc.GetVideoTrack(track.ID())
			if !exists {
				logger.Error("Video track not found", "track_id", track.ID())
				return
			}

			// Set up video sample handler
			frameCount := uint64(0)
			videoTrack.OnSample(func(sample *media.Sample) {
				frameCount++
			})

			// Display stats periodically
			go func() {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()

				for range ticker.C {
					stats := videoTrack.Stats()
					logger.Info("Video receiving stats",
						"packets", stats.PacketsReceived,
						"bytes", stats.BytesReceived,
						"frames", stats.FramesReceived,
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

	log.Println("Waiting for video stream... Press Enter to display stats or 'q' to quit")

	for {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())

		if input == "q" || input == "quit" {
			break
		}

		// Display all video track stats
		fmt.Printf("\n=== Video Reception Stats ===\n")
		// Note: In a real implementation, you would iterate through received tracks
		fmt.Printf("Waiting for video tracks...\n")
		fmt.Printf("============================\n\n")
	}
}

func generateTestPattern(videoTrack *webrtcinternal.VideoTrack, logger *logging.Logger) {
	time.Sleep(3 * time.Second) // Wait for connection to stabilize
	logger.Info("Starting test pattern video transmission")

	width := 640
	height := 480
	frameSize := width * height * 3 / 2 // YUV420
	frameCounter := 0

	ticker := time.NewTicker(33 * time.Millisecond) // ~30fps
	defer ticker.Stop()

	for range ticker.C {
		// Generate a simple test pattern (alternating black and white frames)
		data := make([]byte, frameSize)

		// Y plane
		yValue := byte(0)
		if frameCounter%60 < 30 {
			yValue = 255
		}
		for i := range width * height {
			data[i] = yValue
		}

		// U and V planes (grayscale)
		for i := width * height; i < frameSize; i++ {
			data[i] = 128
		}

		sample := &media.Sample{
			Data:     data,
			Duration: 33 * time.Millisecond,
		}

		if err := videoTrack.WriteSample(sample); err != nil {
			logger.Error("Failed to write video sample", "error", err)
		}

		frameCounter++
	}
}

func playYUVFile(filename string, videoTrack *webrtcinternal.VideoTrack, logger *logging.Logger) {
	time.Sleep(3 * time.Second) // Wait for connection to stabilize
	logger.Info("Starting YUV file playback", "file", filename)

	yuvReader, err := video.NewYUVReader(filename)
	if err != nil {
		logger.Error("Failed to open YUV file", "error", err)
		return
	}
	defer yuvReader.Close()

	logger.Info("YUV file info",
		"width", yuvReader.Width(),
		"height", yuvReader.Height(),
		"fps", yuvReader.FrameRate(),
		"frames", yuvReader.FrameCount(),
	)

	frameDuration := time.Second / time.Duration(yuvReader.FrameRate())
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	for range ticker.C {
		// Read frame from YUV file
		frameData, err := yuvReader.ReadFrame()
		if err != nil {
			if err.Error() == "EOF" {
				// Loop back to beginning
				logger.Info("Reached end of YUV file, looping back")
				yuvReader.Reset()
				continue
			}
			logger.Error("Failed to read YUV frame", "error", err)
			return
		}

		sample := &media.Sample{
			Data:     frameData,
			Duration: frameDuration,
		}

		if err := videoTrack.WriteSample(sample); err != nil {
			logger.Error("Failed to write video sample", "error", err)
		}
	}
}
