package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/HMasataka/conic"
	"github.com/pion/webrtc/v4"
)

var (
	addr = flag.String("addr", "localhost:3000", "http service address")
	role = flag.String("role", "peer", "role: peer (default), offer, answer")
)

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	log.Printf("connecting to %s as %s", u.String(), *role)

	client, err := conic.NewClient(u)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer client.Close()

	// WebRTC設定
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// ハンドシェイク初期化
	if err := client.InitHandshake(config); err != nil {
		log.Fatal("init handshake:", err)
	}

	handshake := client.GetHandshake()

	// WebRTC接続状態の監視
	handshake.SetOnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Peer connection state changed: %s", state.String())

		if state == webrtc.PeerConnectionStateConnected {
			log.Println("🎉 P2P connection established!")
		}
	})

	// ICE候補処理の設定
	handshake.OnIceCandidate()

	// WebSocket読み取り開始
	go func() {
		if err := client.Read(); err != nil {
			log.Println("read error:", err)
		}
	}()

	// WebSocket書き込み開始（登録処理）
	go func() {
		if err := client.Write(); err != nil {
			log.Println("write error:", err)
		}
	}()

	// 登録完了まで待機
	time.Sleep(2 * time.Second)

	// ロールに応じた処理開始
	switch *role {
	case "offer":
		runOfferMode(client)
	case "answer":
		runAnswerMode(client)
	default:
		runInteractiveMode(client)
	}
}

func runOfferMode(client *conic.Client) {
	log.Println("=== Offer Mode ===")
	log.Println("This client will create offers and data channels")

	const dataChannelLabel = "chat"

	// データチャネル作成
	dataChannel, err := client.CreateDataChannel(dataChannelLabel)
	if err != nil {
		log.Fatal("create data channel:", err)
	}

	setupDataChannelHandlers(dataChannel, dataChannelLabel)

	log.Println("Enter target peer ID to create offer:")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		targetID := strings.TrimSpace(scanner.Text())
		if targetID == "" {
			continue
		}

		if err := createAndSendOffer(client, targetID); err != nil {
			log.Printf("Failed to create offer: %v", err)
		} else {
			log.Printf("Offer sent to %s", targetID)
			break
		}
	}

	// データチャネル通信を開始
	startDataChannelChat(client, dataChannelLabel)
}

func runAnswerMode(client *conic.Client) {
	log.Println("=== Answer Mode ===")
	log.Println("This client will wait for offers and respond")

	// 受信データチャネルのハンドラー設定
	client.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("📨 Received data channel: %s", dc.Label())
		setupDataChannelHandlers(dc, dc.Label())
	})

	log.Println("Waiting for offers... (Press Enter to exit)")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
}

func runInteractiveMode(client *conic.Client) {
	log.Println("=== Interactive P2P Demo ===")
	log.Println("Commands:")
	log.Println("  offer <peer_id>     - Create WebRTC offer to peer")
	log.Println("  channel <label>     - Create data channel")
	log.Println("  send <label> <msg>  - Send message via data channel")
	log.Println("  list                - List active data channels")
	log.Println("  quit                - Exit")

	// 受信データチャネルのハンドラー設定
	client.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("📨 Received data channel: %s", dc.Label())
		setupDataChannelHandlers(dc, dc.Label())
	})

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
				log.Println("Usage: offer <peer_id>")
				continue
			}
			peerID := parts[1]

			if err := createAndSendOffer(client, peerID); err != nil {
				log.Printf("Failed to create offer: %v", err)
			}

		case "channel":
			if len(parts) < 2 {
				log.Println("Usage: channel <label>")
				continue
			}
			label := parts[1]

			dataChannel, err := client.CreateDataChannel(label)
			if err != nil {
				log.Printf("Failed to create data channel: %v", err)
				continue
			}

			setupDataChannelHandlers(dataChannel, label)
			log.Printf("Created data channel: %s", label)

		case "send":
			if len(parts) < 3 {
				log.Println("Usage: send <label> <message>")
				continue
			}
			label := parts[1]
			message := strings.Join(parts[2:], " ")

			if err := client.SendDataChannelDirect(label, []byte(message)); err != nil {
				log.Printf("Failed to send message: %v", err)
			} else {
				log.Printf("📤 Sent via %s: %s", label, message)
			}

		case "list":
			// This would require adding a method to list active channels
			log.Println("Active data channels: (feature not implemented)")

		case "quit":
			log.Println("Goodbye!")
			return

		default:
			log.Printf("Unknown command: %s", command)
		}
	}
}

func setupDataChannelHandlers(dataChannel *webrtc.DataChannel, label string) {
	dataChannel.OnOpen(func() {
		log.Printf("✅ Data channel '%s' opened", label)
	})

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Printf("📥 [%s]: %s", label, string(msg.Data))
	})

	dataChannel.OnClose(func() {
		log.Printf("❌ Data channel '%s' closed", label)
	})

	dataChannel.OnError(func(err error) {
		log.Printf("🚨 Data channel '%s' error: %v", label, err)
	})
}

func createAndSendOffer(client *conic.Client, targetID string) error {
	handshake := client.GetHandshake()
	if handshake == nil {
		return fmt.Errorf("handshake not initialized")
	}

	// Offer作成
	offer, err := handshake.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("create offer: %w", err)
	}

	// ローカル記述設定
	if err := handshake.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("set local description: %w", err)
	}

	// SDP経由でOfferを送信
	return sendSDP(client, targetID, offer)
}

func sendSDP(client *conic.Client, targetID string, sdp webrtc.SessionDescription) error {
	log.Printf("📡 Sending %s to %s", sdp.Type, targetID)
	return client.SendSDP(targetID, sdp)
}

func startDataChannelChat(client *conic.Client, label string) {
	log.Printf("💬 Starting chat on data channel '%s'", label)
	log.Println("Type messages and press Enter (type 'quit' to exit):")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("chat> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "quit" {
			break
		}

		if err := client.SendDataChannelDirect(label, []byte(input)); err != nil {
			log.Printf("Failed to send message: %v", err)
		}
	}
}
