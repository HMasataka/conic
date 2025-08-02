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

	"github.com/HMasataka/conic/internal/logging"
	"github.com/HMasataka/conic/pkg/domain"
	"github.com/HMasataka/conic/pkg/signaling"
	"github.com/HMasataka/conic/pkg/webrtc"
	pionwebrtc "github.com/pion/webrtc/v4"
)

func main() {
	var (
		serverAddr = flag.String("server", "ws://localhost:3000/ws", "signaling server URL")
		role       = flag.String("role", "peer", "role: peer (default), offer, answer")
		logLevel   = flag.String("log-level", "info", "log level (debug, info, warn, error)")
	)
	flag.Parse()

	// Initialize logger
	logger := logging.New(logging.Config{
		Level:  *logLevel,
		Format: "text",
	})

	// Parse server URL
	serverURL, err := url.Parse(*serverAddr)
	if err != nil {
		log.Fatalf("invalid server URL: %v", err)
	}

	// Create signaling client
	clientOptions := signaling.DefaultClientOptions()
	clientOptions.Logger = logger

	client := signaling.NewClient(*serverURL, clientOptions)
	logger.Info("creating signaling client")

	// Connect to signaling server
	if err := client.Connect(); err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer client.Disconnect()

	logger.Info("connecting to signaling server")

	// Wait for registration
	if err := client.WaitForRegistration(5 * time.Second); err != nil {
		log.Fatalf("registration failed: %v", err)
	}

	logger.Info("connected to signaling server",
		"client_id", client.ID(),
		"server", serverAddr,
		"role", *role,
	)

	// Initialize WebRTC manager
	webrtcOptions := webrtc.DefaultPeerConnectionOptions()
	webrtcOptions.Logger = logger

	webrtcManager := webrtc.NewManager(logger, nil, webrtcOptions)
	defer webrtcManager.CloseAll()

	// Set up message handlers
	setupMessageHandlers(client, webrtcManager, logger)

	// Run based on role
	switch *role {
	case "offer":
		runOfferMode(client, webrtcManager, logger)
	case "answer":
		runAnswerMode(client, webrtcManager, logger)
	default:
		runInteractiveMode(client, webrtcManager, logger)
	}
}

func setupMessageHandlers(client *signaling.Client, manager *webrtc.Manager, logger *logging.Logger) {
	// Handle SDP messages
	client.OnMessage(domain.MessageTypeSDP, func(ctx context.Context, msg domain.Message) error {
		var sdpMsg domain.SDPMessage
		if err := json.Unmarshal(msg.Data, &sdpMsg); err != nil {
			return err
		}

		logger.Info("received SDP",
			"from", sdpMsg.FromID,
			"type", sdpMsg.SessionDescription.Type,
		)

		// Handle based on SDP type
		if sdpMsg.SessionDescription.Type == pionwebrtc.SDPTypeOffer {
			// Handle offer
			answer, err := manager.HandleOffer(ctx, sdpMsg.FromID, sdpMsg.SessionDescription)
			if err != nil {
				logger.Error("failed to handle offer", "error", err)
				return err
			}

			// Send answer back
			return client.SendSDP(sdpMsg.FromID, answer)
		} else {
			// Handle answer
			return manager.HandleAnswer(ctx, sdpMsg.FromID, sdpMsg.SessionDescription)
		}
	})

	// Handle ICE candidates
	client.OnMessage(domain.MessageTypeCandidate, func(ctx context.Context, msg domain.Message) error {
		var iceMsg domain.ICECandidateMessage
		if err := json.Unmarshal(msg.Data, &iceMsg); err != nil {
			return err
		}

		logger.Debug("received ICE candidate", "from", iceMsg.FromID)

		return manager.HandleICECandidate(ctx, iceMsg.FromID, iceMsg.Candidate)
	})
}

func runOfferMode(client *signaling.Client, manager *webrtc.Manager, logger *logging.Logger) {
	fmt.Println("=== Offer Mode ===")
	fmt.Println("This client will create offers and data channels")
	fmt.Println("Enter target peer ID to create offer:")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		targetID := strings.TrimSpace(scanner.Text())
		if targetID == "" {
			continue
		}

		// Create peer connection
		pc, err := manager.CreatePeerConnection(targetID)
		if err != nil {
			logger.Error("failed to create peer connection", "error", err)
			continue
		}

		// Set up ICE candidate handler
		pc.OnICECandidate(func(candidate *pionwebrtc.ICECandidate) error {
			return client.SendICECandidate(targetID, candidate.ToJSON())
		})

		// Create data channel
		dc, err := pc.CreateDataChannel("chat", nil)
		if err != nil {
			logger.Error("failed to create data channel", "error", err)
			continue
		}

		setupDataChannelHandlers(dc, logger)

		// Create offer
		offer, err := pc.CreateOffer(nil)
		if err != nil {
			logger.Error("failed to create offer", "error", err)
			continue
		}

		// Send offer
		if err := client.SendSDP(targetID, offer); err != nil {
			logger.Error("failed to send offer", "error", err)
			continue
		}

		logger.Info("offer sent", "target", targetID)
		break
	}

	// Keep running
	fmt.Println("Press Ctrl+C to exit")
	select {}
}

func runAnswerMode(client *signaling.Client, manager *webrtc.Manager, logger *logging.Logger) {
	fmt.Println("=== Answer Mode ===")
	fmt.Println("This client will wait for offers and respond")
	fmt.Println("Waiting for offers... (Press Ctrl+C to exit)")

	// Keep running
	select {}
}

func runInteractiveMode(client *signaling.Client, manager *webrtc.Manager, logger *logging.Logger) {
	fmt.Println("=== Interactive P2P Demo ===")
	fmt.Println("Commands:")
	fmt.Println("  offer <peer_id>     - Create WebRTC offer to peer")
	fmt.Println("  channel <peer_id>   - Create data channel to peer")
	fmt.Println("  send <peer_id> <msg>- Send message via data channel")
	fmt.Println("  list                - List active peers")
	fmt.Println("  quit                - Exit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}

		command := parts[0]

		switch command {
		case "offer":
			if len(parts) < 2 {
				fmt.Println("Usage: offer <peer_id>")
				continue
			}
			handleOfferCommand(parts[1], client, manager, logger)

		case "channel":
			if len(parts) < 2 {
				fmt.Println("Usage: channel <peer_id>")
				continue
			}
			handleChannelCommand(parts[1], manager, logger)

		case "send":
			if len(parts) < 3 {
				fmt.Println("Usage: send <peer_id> <message>")
				continue
			}
			handleSendCommand(parts[1], strings.Join(parts[2:], " "), manager, logger)

		case "list":
			peers := manager.GetPeerIDs()
			fmt.Printf("Active peers (%d):\n", len(peers))
			for _, id := range peers {
				fmt.Printf("  - %s\n", id)
			}

		case "quit":
			fmt.Println("Goodbye!")
			return

		default:
			fmt.Printf("Unknown command: %s\n", command)
		}
	}
}

func handleOfferCommand(targetID string, client *signaling.Client, manager *webrtc.Manager, logger *logging.Logger) {
	// Create or get peer connection
	pc, err := manager.GetPeerConnection(targetID)
	if err != nil {
		// Create new peer connection
		pc, err = manager.CreatePeerConnection(targetID)
		if err != nil {
			logger.Error("failed to create peer connection", "error", err)
			return
		}
	}

	// Set up ICE candidate handler
	pc.OnICECandidate(func(candidate *pionwebrtc.ICECandidate) error {
		return client.SendICECandidate(targetID, candidate.ToJSON())
	})

	// Create offer
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		logger.Error("failed to create offer", "error", err)
		return
	}

	// Send offer
	if err := client.SendSDP(targetID, offer); err != nil {
		logger.Error("failed to send offer", "error", err)
		return
	}

	fmt.Printf("Offer sent to %s\n", targetID)
}

func handleChannelCommand(peerID string, manager *webrtc.Manager, logger *logging.Logger) {
	pc, err := manager.GetPeerConnection(peerID)
	if err != nil {
		fmt.Printf("No connection to peer %s\n", peerID)
		return
	}

	dc, err := pc.CreateDataChannel("chat", nil)
	if err != nil {
		logger.Error("failed to create data channel", "error", err)
		return
	}

	setupDataChannelHandlers(dc, logger)
	fmt.Printf("Created data channel to %s\n", peerID)
}

func handleSendCommand(peerID, message string, manager *webrtc.Manager, logger *logging.Logger) {
	// This is simplified - in a real implementation, you'd track data channels
	fmt.Printf("Send functionality not fully implemented\n")
}

func setupDataChannelHandlers(dc *webrtc.DataChannel, logger *logging.Logger) {
	dc.OnOpen(func() {
		fmt.Printf("✅ Data channel '%s' opened\n", dc.Label())
	})

	dc.OnMessage(func(data []byte) {
		fmt.Printf("📥 [%s]: %s\n", dc.Label(), string(data))
	})

	dc.OnClose(func() {
		fmt.Printf("❌ Data channel '%s' closed\n", dc.Label())
	})

	dc.OnError(func(err error) {
		logger.Error("data channel error", "label", dc.Label(), "error", err)
	})
}
