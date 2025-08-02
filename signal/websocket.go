package signal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/HMasataka/conic/hub"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func NewServer(hub hub.Hub) Server {
	return &server{
		hub: hub,
	}
}

type server struct {
	hub hub.Hub
}

type Server interface {
	Serve(w http.ResponseWriter, r *http.Request)
}

func NewSocket(hub hub.Hub, conn *websocket.Conn) Socket {
	return &socket{
		conn:         conn,
		hub:          hub,
		dataChannel:  make(chan []byte),
		done:         make(chan struct{}),
		closeChannel: make(chan struct{}),
		errorChannel: make(chan error),
	}
}

type Socket interface {
	Serve()
	io.WriteCloser
	Error(err error)
}

type socket struct {
	conn         *websocket.Conn
	hub          hub.Hub
	dataChannel  chan []byte
	errorChannel chan error
	done         chan struct{}
	closeChannel chan struct{}
}

func (s *server) Serve(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	socket := NewSocket(s.hub, conn)
	socket.Serve()
}

func (s *socket) Serve() {
	go s.read()
	s.write()
}

func (s *socket) Write(message []byte) (int, error) {
	s.dataChannel <- message
	return len(message), nil
}

func (s *socket) Error(err error) {
	s.errorChannel <- err
}

func (s *socket) Close() error {
	close(s.closeChannel)
	return nil
}

func (s *socket) read() {
	defer close(s.done)

	for {
		messageType, message, err := s.conn.ReadMessage()
		if err != nil {
			log.Print("read error:", err)
			return
		}

		switch messageType {
		case websocket.TextMessage:
			if err := s.handleMessage(message); err != nil {
				log.Printf("err: %v\n", err)
				return
			}
		case websocket.CloseMessage:
			return
		default:
			log.Printf("message type: %v, message: %s\n", messageType, string(message))
		}
	}
}

const (
	RequestTypeRegister    = "register"
	RequestTypeUnRegister  = "unregister"
	RequestTypeSDP         = "sdp"
	RequestTypeCandidate   = "candidate"
	RequestTypeDataChannel = "data_channel"
)

type Request struct {
	Type string `json:"type"`
	Raw  []byte `json:"raw"`
}

type UnRegisterRequest struct {
	ID string `json:"id"`
}

type RegisterResponse struct {
	ID string `json:"id"`
}

type SessionDescriptionRequest struct {
	ID                 string                    `json:"id"`
	TargetID           string                    `json:"target_id"`
	SessionDescription webrtc.SessionDescription `json:"session_description"`
}

type CandidateRequest struct {
	ID        string `json:"id"`
	TargetID  string `json:"target_id"`
	Candidate string `json:"candidate"`
}

type DataChannelRequest struct {
	ID       string `json:"id"`
	TargetID string `json:"target_id"`
	Label    string `json:"label"`
	Data     []byte `json:"data"`
}

func validateRequest(req Request) error {
	if req.Type == "" {
		return errors.New("request type is required")
	}

	switch req.Type {
	case RequestTypeRegister:
		// 登録リクエストの検証
	case RequestTypeUnRegister:
		// 登録解除リクエストの検証
		if len(req.Raw) == 0 {
			return errors.New("unregister request requires ID")
		}
	case RequestTypeSDP, RequestTypeCandidate:
		// SDP/候補リクエストの検証
		if len(req.Raw) == 0 {
			return errors.New("SDP/candidate request requires data")
		}
	case RequestTypeDataChannel:
		// データチャネルリクエストの検証
		if len(req.Raw) == 0 {
			return errors.New("data channel request requires data")
		}
	default:
		return fmt.Errorf("unknown request type: %s", req.Type)
	}

	return nil
}

func (s *socket) getHandler(requestType string) MessageHandler {
	switch requestType {
	case RequestTypeRegister:
		return &RegisterHandler{}
	case RequestTypeUnRegister:
		return &UnregisterHandler{}
	case RequestTypeSDP:
		return &SDPHandler{}
	case RequestTypeCandidate:
		return &CandidateHandler{}
	case RequestTypeDataChannel:
		return &DataChannelHandler{}
	default:
		return nil
	}
}

func (s *socket) handleMessage(message []byte) error {
	var req Request
	if err := json.Unmarshal(message, &req); err != nil {
		return err
	}

	if err := validateRequest(req); err != nil {
		return err
	}

	handler := s.getHandler(req.Type)
	if handler == nil {
		return fmt.Errorf("unknown request type: %s", req.Type)
	}

	return handler.Handle(req.Raw, s)
}

func (s *socket) write() {
	for {
		select {
		case <-s.done:
			return
		case t := <-s.dataChannel:
			if err := s.conn.WriteMessage(websocket.TextMessage, t); err != nil {
				return
			}
		case e := <-s.errorChannel:
			if err := s.conn.WriteMessage(websocket.TextMessage, []byte(e.Error())); err != nil {
				return
			}
		case <-s.closeChannel:
			if err := s.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
				return
			}
			return
		}
	}
}
