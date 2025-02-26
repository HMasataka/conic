package conic

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/rs/xid"
)

var upgrader = websocket.Upgrader{}

func NewSocket(hub Hub) *Socket {
	return &Socket{
		hub:          hub,
		dataChannel:  make(chan []byte),
		done:         make(chan struct{}),
		closeChannel: make(chan struct{}),
		errorChannel: make(chan error),
	}
}

type Socket struct {
	conn         *websocket.Conn
	hub          Hub
	dataChannel  chan []byte
	errorChannel chan error
	done         chan struct{}
	closeChannel chan struct{}
}

func (s *Socket) Serve(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	s.conn = conn

	go s.read()

	s.write()
}

func (s *Socket) Write(message []byte) (int, error) {
	s.dataChannel <- message
	return len(message), nil
}

func (s *Socket) Error(err error) {
	s.errorChannel <- err
}

func (s *Socket) Close() {
	close(s.closeChannel)
}

func (s *Socket) read() {
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
			close(s.done)
		default:
			log.Printf("message type: %v, message: %s\n", messageType, string(message))
		}
	}
}

const (
	RequestTypeRegister   = "register"
	RequestTypeUnRegister = "unregister"
	RequestTypeSDP        = "sdp"
	RequestTypeCandidate  = "candidate"
)

type Request struct {
	Type string
	Raw  []byte
}

type WebsocketUnRegisterRequest struct {
	ID string
}

type SessionDescriptionRequest struct {
	ID                 string
	TargetID           string
	SessionDescription webrtc.SessionDescription
}

type CandidateRequest struct {
	ID        string
	TargetID  string
	Candidate string
}

func (s *Socket) handleMessage(message []byte) error {
	var req Request

	if err := json.Unmarshal(message, &req); err != nil {
		return err
	}

	switch req.Type {
	case RequestTypeRegister:
		id := xid.New().String()

		s.hub.Register(RegisterRequest{
			ID:     id,
			Client: s,
		})
	case RequestTypeUnRegister:
		var unregister WebsocketUnRegisterRequest

		if err := json.Unmarshal(req.Raw, &unregister); err != nil {
			return err
		}

		s.hub.Register(RegisterRequest{
			ID: unregister.ID,
		})
	case RequestTypeSDP:
		var sdpRequest SessionDescriptionRequest

		if err := json.Unmarshal(req.Raw, &sdpRequest); err != nil {
			return err
		}

		s.hub.SendMessage(MessageRequest{
			ID:       sdpRequest.ID,
			TargetID: sdpRequest.TargetID,
			Message:  message,
		})
	case RequestTypeCandidate:
		var candidateRequest CandidateRequest

		if err := json.Unmarshal(req.Raw, &candidateRequest); err != nil {
			return err
		}

		s.hub.SendMessage(MessageRequest{
			ID:       candidateRequest.ID,
			TargetID: candidateRequest.TargetID,
			Message:  message,
		})
	}

	return nil
}

func (s *Socket) write() {
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
