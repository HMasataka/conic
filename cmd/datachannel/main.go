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

	router := conic.NewRouter(id, pc, logger)

	client := conic.NewClient(conn, router, logger, conic.DefaultClientOptions())
	go client.Start()
	logger.Info("Client started", "id", client.ID())

	switch *role {
	case "offer":
		runOfferMode(pc, client, logger)
	case "answer":
		runAnswerMode(pc, logger)
	default:
		logger.Error("Invalid role specified", "role", *role)
	}
}

func runOfferMode(pc *conic.PeerConnection, client *conic.Client, logging *logging.Logger) {
	logging.Info("Running in offer mode")

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		log.Fatal("create offer:", err)
	}

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
		ID:        pc.TargetID(),
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

	const dataChannelLabel = "chat"

	dataChannel, err := pc.CreateDataChannel(dataChannelLabel, nil)
	if err != nil {
		log.Fatal("create data channel:", err)
	}

	dataChannel.OnOpen(func() {
		log.Printf("📨 Data channel '%s' is open", dataChannel.Label())
	})

	log.Println("Waiting for data channel to open... (Press Enter to exit)")
	scanner.Scan()
}

func runAnswerMode(pc *conic.PeerConnection, logging *logging.Logger) {
	logging.Info("Running in answer mode")

	pc.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		log.Printf("📨 Data channel '%s' is open", dataChannel.Label())
	})

	log.Println("Waiting for offers... (Press Enter to exit)")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
}
