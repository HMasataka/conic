package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/HMasataka/conic"
	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
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
	logger.Info("Creating peer connection", "id", id)

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

	pc, err := conic.NewPeerConnection(id, conic.DefaultPeerConnectionOptions(logger))
	if err != nil {
		logger.Error("Failed to create peer connection", "error", err)
		return
	}

	pc.OnICECandidate(conic.OnIceCandidate(conn, pc))

	router := conic.NewRouter(pc, logger)

	client := conic.NewClient(conn, router, logger, conic.DefaultClientOptions(id))
	go client.Start(context.Background())

	logger.Info("Client started", "id", client.ID())

	// Wait for WebSocket connection to be fully established
	time.Sleep(100 * time.Millisecond)

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
func registerToServer(pc *conic.PeerConnection, client *conic.Client, logger *logging.Logger) error {
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

func runOfferMode(pc *conic.PeerConnection, client *conic.Client, logging *logging.Logger) {
	logging.Info("Running in offer mode")

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

	const dataChannelLabel = "chat"

	dataChannel, err := pc.CreateDataChannel(dataChannelLabel, nil)
	if err != nil {
		log.Fatal("create data channel:", err)
	}

	dataChannel.OnOpen(func() {
		log.Printf("📨 Data channel '%s' is open", dataChannel.Label())
	})

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

	log.Println("Waiting for data channel to open... (Press Enter to exit)")
	waitScanner := bufio.NewScanner(os.Stdin)
	waitScanner.Scan()
}

func runAnswerMode(pc *conic.PeerConnection, logging *logging.Logger) {
	logging.Info("Running in answer mode")

	pc.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		log.Printf("📨 Data channel '%s' is open", dataChannel.Label())
	})

	log.Println("Waiting for data channel to open... (Press Enter to exit)")
	waitScanner := bufio.NewScanner(os.Stdin)
	waitScanner.Scan()
}
