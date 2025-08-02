package conic

import (
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	cosig "github.com/HMasataka/conic/signal"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

func NewClient(u url.URL) (*Client, error) {
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:         conn,
		done:         make(chan struct{}),
		dataChannels: make(map[string]*webrtc.DataChannel),
	}, nil
}

type Client struct {
	id             string
	conn           *websocket.Conn
	done           chan struct{}
	handshake      *Handshake
	dataChannels   map[string]*webrtc.DataChannel
	dataChannelMux sync.Mutex
	registered     bool
	registeredMux  sync.Mutex
}

func (c *Client) Read() error {
	defer close(c.done)

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return err
		}

		if err := c.handleMessage(message); err != nil {
			log.Printf("handle message error: %v", err)
			return err
		}
	}
}

func (c *Client) handleMessage(message []byte) error {
	// 登録レスポンスかどうか最初にチェック
	var res cosig.RegisterResponse
	if err := json.Unmarshal(message, &res); err == nil && res.ID != "" {
		c.registeredMux.Lock()
		c.id = res.ID
		c.registered = true
		c.registeredMux.Unlock()

		log.Printf("registered with ID: %s", c.id)
		return nil
	}

	var req cosig.Request
	if err := json.Unmarshal(message, &req); err != nil {
		log.Printf("failed to unmarshal message: %v", err)
		return err
	}

	switch req.Type {
	case cosig.RequestTypeSDP:
		return c.handleSDP(req.Raw)
	case cosig.RequestTypeCandidate:
		return c.handleCandidate(req.Raw)
	case cosig.RequestTypeDataChannel:
		return c.handleDataChannel(req.Raw)
	default:
		log.Printf("unknown request type: %s", req.Type)
	}

	return nil
}

func (c *Client) handleSDP(raw []byte) error {
	var sdpRequest cosig.SessionDescriptionRequest
	if err := json.Unmarshal(raw, &sdpRequest); err != nil {
		return err
	}

	if c.handshake != nil {
		log.Printf("📥 Received %s from %s", sdpRequest.SessionDescription.Type, sdpRequest.ID)

		if err := c.handshake.SetRemoteDescription(sdpRequest.SessionDescription); err != nil {
			return err
		}

		// Offerを受信した場合、Answerを作成して送信
		if sdpRequest.SessionDescription.Type == webrtc.SDPTypeOffer {
			if err := c.createAndSendAnswer(sdpRequest.ID); err != nil {
				log.Printf("Failed to create answer: %v", err)
				return err
			}
		}

		// 保留中の候補を処理
		return c.handshake.HandlePendingCandidate()
	}

	return nil
}

func (c *Client) createAndSendAnswer(targetID string) error {
	// Answer作成
	peerConnection := c.handshake.GetPeerConnection()
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return err
	}

	// ローカル記述設定
	if err := c.handshake.SetLocalDescription(answer); err != nil {
		return err
	}

	// Answerを送信
	log.Printf("📤 Sending answer to %s", targetID)
	return c.SendSDP(targetID, answer)
}

func (c *Client) handleCandidate(raw []byte) error {
	var candidateRequest cosig.CandidateRequest
	if err := json.Unmarshal(raw, &candidateRequest); err != nil {
		return err
	}

	if c.handshake != nil {
		var candidate webrtc.ICECandidateInit
		if err := json.Unmarshal([]byte(candidateRequest.Candidate), &candidate); err != nil {
			return err
		}
		return c.handshake.AddIceCandidate(candidate)
	}

	return nil
}

func (c *Client) handleDataChannel(raw []byte) error {
	var dataChannelRequest cosig.DataChannelRequest
	if err := json.Unmarshal(raw, &dataChannelRequest); err != nil {
		return err
	}

	c.dataChannelMux.Lock()
	defer c.dataChannelMux.Unlock()

	dataChannel, exists := c.dataChannels[dataChannelRequest.Label]
	if !exists {
		log.Printf("data channel '%s' not found", dataChannelRequest.Label)
		return nil
	}

	return dataChannel.Send(dataChannelRequest.Data)
}

func (c *Client) Write() error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return nil
		case <-ticker.C:
			c.registeredMux.Lock()
			isRegistered := c.registered
			c.registeredMux.Unlock()

			if isRegistered {
				continue
			}

			req := cosig.Request{
				Type: cosig.RequestTypeRegister,
			}

			message, err := json.Marshal(req)
			if err != nil {
				return err
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return err
			}

			log.Println("sent registration request")
		case <-interrupt:
			log.Println("interrupt")

			if err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
				return err
			}

			c.waitCloseConnection()

			return nil
		}
	}
}

func (c *Client) waitCloseConnection() {
	select {
	case <-c.done:
	case <-time.After(time.Second):
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Error(err error) {
	log.Println("err:", err)
}

func (c *Client) InitHandshake(config webrtc.Configuration) error {
	var err error
	c.handshake, err = NewHandshake(config, func(candidate *webrtc.ICECandidate) error {
		return c.sendCandidate(candidate)
	})
	return err
}

func (c *Client) sendCandidate(candidate *webrtc.ICECandidate) error {
	candidateJSON, err := json.Marshal(candidate.ToJSON())
	if err != nil {
		return err
	}

	candidateRequest := cosig.CandidateRequest{
		ID:        c.GetID(),
		TargetID:  "",
		Candidate: string(candidateJSON),
	}

	requestRaw, err := json.Marshal(candidateRequest)
	if err != nil {
		return err
	}

	req := cosig.Request{
		Type: cosig.RequestTypeCandidate,
		Raw:  requestRaw,
	}

	message, err := json.Marshal(req)
	if err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, message)
}

func (c *Client) CreateDataChannel(label string) (*webrtc.DataChannel, error) {
	if c.handshake == nil {
		return nil, errors.New("handshake not initialized")
	}

	c.dataChannelMux.Lock()
	defer c.dataChannelMux.Unlock()

	peerConnection := c.handshake.GetPeerConnection()
	dataChannel, err := peerConnection.CreateDataChannel(label, nil)
	if err != nil {
		return nil, err
	}

	c.dataChannels[label] = dataChannel
	return dataChannel, nil
}

func (c *Client) SendDataChannelMessage(targetID, label string, data []byte) error {
	dataChannelRequest := cosig.DataChannelRequest{
		ID:       c.GetID(),
		TargetID: targetID,
		Label:    label,
		Data:     data,
	}

	requestRaw, err := json.Marshal(dataChannelRequest)
	if err != nil {
		return err
	}

	req := cosig.Request{
		Type: cosig.RequestTypeDataChannel,
		Raw:  requestRaw,
	}

	message, err := json.Marshal(req)
	if err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, message)
}

func (c *Client) SetupDataChannelHandlers(label string, onOpen func(), onMessage func(webrtc.DataChannelMessage)) error {
	c.dataChannelMux.Lock()
	defer c.dataChannelMux.Unlock()

	dataChannel, exists := c.dataChannels[label]
	if !exists {
		return errors.New("data channel not found")
	}

	dataChannel.OnOpen(onOpen)
	dataChannel.OnMessage(onMessage)
	return nil
}

func (c *Client) GetHandshake() *Handshake {
	return c.handshake
}

func (c *Client) GetID() string {
	c.registeredMux.Lock()
	defer c.registeredMux.Unlock()
	return c.id
}

func (c *Client) IsRegistered() bool {
	c.registeredMux.Lock()
	defer c.registeredMux.Unlock()
	return c.registered
}

func (c *Client) GetDataChannel(label string) *webrtc.DataChannel {
	c.dataChannelMux.Lock()
	defer c.dataChannelMux.Unlock()
	return c.dataChannels[label]
}

func (c *Client) OnDataChannel(fn func(*webrtc.DataChannel)) {
	if c.handshake != nil {
		peerConnection := c.handshake.GetPeerConnection()
		peerConnection.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
			c.dataChannelMux.Lock()
			c.dataChannels[dataChannel.Label()] = dataChannel
			c.dataChannelMux.Unlock()

			if fn != nil {
				fn(dataChannel)
			}
		})
	}
}

func (c *Client) SendDataChannelDirect(label string, data []byte) error {
	c.dataChannelMux.Lock()
	defer c.dataChannelMux.Unlock()

	dataChannel, exists := c.dataChannels[label]
	if !exists {
		return errors.New("data channel not found")
	}

	return dataChannel.Send(data)
}

func (c *Client) SendSDP(targetID string, sdp webrtc.SessionDescription) error {
	sdpRequest := cosig.SessionDescriptionRequest{
		ID:                 c.GetID(),
		TargetID:           targetID,
		SessionDescription: sdp,
	}

	requestRaw, err := json.Marshal(sdpRequest)
	if err != nil {
		return err
	}

	req := cosig.Request{
		Type: cosig.RequestTypeSDP,
		Raw:  requestRaw,
	}

	message, err := json.Marshal(req)
	if err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, message)
}
