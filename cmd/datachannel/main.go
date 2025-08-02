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
	addr     = flag.String("addr", "localhost:3000", "http service address")
	clientID = flag.String("id", "", "client ID for peer connection (optional)")
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

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

	// WebSocket読み取り開始
	go func() {
		if err := client.Read(); err != nil {
			log.Println("read:", err)
		}
	}()

	// WebSocket書き込み開始（登録処理）
	go func() {
		if err := client.Write(); err != nil {
			log.Println("write:", err)
		}
	}()

	// 登録完了まで待機
	time.Sleep(2 * time.Second)

	// データチャネル通信のデモを開始
	runDataChannelDemo(client)
}

func runDataChannelDemo(client *conic.Client) {
	const dataChannelLabel = "chat"

	// データチャネル作成
	dataChannel, err := client.CreateDataChannel(dataChannelLabel)
	if err != nil {
		log.Fatal("create data channel:", err)
	}

	// データチャネルイベントハンドラー設定
	dataChannel.OnOpen(func() {
		log.Printf("Data channel '%s' opened", dataChannelLabel)
	})

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Printf("Received message: %s", string(msg.Data))
	})

	dataChannel.OnClose(func() {
		log.Printf("Data channel '%s' closed", dataChannelLabel)
	})

	// 受信データチャネルのハンドラー設定
	client.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("Received data channel: %s", dc.Label())

		dc.OnOpen(func() {
			log.Printf("Incoming data channel '%s' opened", dc.Label())
		})

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			log.Printf("Received on '%s': %s", dc.Label(), string(msg.Data))
		})
	})

	log.Println("\n=== Data Channel Demo ===")
	log.Println("Commands:")
	log.Println("  send <target_id> <message> - Send message via WebSocket signaling")
	log.Println("  direct <message>           - Send message directly via data channel")
	log.Println("  offer <target_id>          - Create WebRTC offer")
	log.Println("  quit                       - Exit")
	log.Println("")

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

		parts := strings.SplitN(input, " ", 3)
		command := parts[0]

		switch command {
		case "send":
			if len(parts) < 3 {
				log.Println("Usage: send <target_id> <message>")
				continue
			}
			targetID := parts[1]
			message := parts[2]

			if err := client.SendDataChannelMessage(targetID, dataChannelLabel, []byte(message)); err != nil {
				log.Printf("Failed to send message: %v", err)
			} else {
				log.Printf("Sent message to %s via signaling", targetID)
			}

		case "direct":
			if len(parts) < 2 {
				log.Println("Usage: direct <message>")
				continue
			}
			message := parts[1]

			if err := client.SendDataChannelDirect(dataChannelLabel, []byte(message)); err != nil {
				log.Printf("Failed to send direct message: %v", err)
			} else {
				log.Printf("Sent direct message: %s", message)
			}

		case "offer":
			if len(parts) < 2 {
				log.Println("Usage: offer <target_id>")
				continue
			}
			targetID := parts[1]

			if err := createOffer(client, targetID); err != nil {
				log.Printf("Failed to create offer: %v", err)
			}

		case "quit":
			log.Println("Goodbye!")
			return

		default:
			log.Printf("Unknown command: %s", command)
		}
	}
}

func createOffer(client *conic.Client, targetID string) error {
	handshake := client.GetHandshake()
	if handshake == nil {
		return fmt.Errorf("handshake not initialized")
	}

	// Offer作成
	offer, err := handshake.CreateOffer(nil)
	if err != nil {
		return err
	}

	// ローカル記述設定
	if err := handshake.SetLocalDescription(offer); err != nil {
		return err
	}

	log.Printf("Created offer for %s", targetID)
	log.Printf("SDP: %s", offer.SDP)

	return nil
}
