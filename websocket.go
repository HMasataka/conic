package conic

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/rs/xid"
)

var upgrader = websocket.Upgrader{}

func NewServer(hub Hub) Server {
	return &server{
		hub: hub,
	}
}

type server struct {
	hub Hub
}

type Server interface {
	Serve(w http.ResponseWriter, r *http.Request)
}

func NewSocket(hub Hub, conn *websocket.Conn) Socket {
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
	hub          Hub
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

type WebsocketRegisterResponse struct {
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

func (s *socket) handleMessage(message []byte) error {
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

		res := WebsocketRegisterResponse{
			ID: id,
		}

		message, err := json.Marshal(res)
		if err != nil {
			return err
		}

		if _, err := s.Write(message); err != nil {
			return err
		}
	case RequestTypeUnRegister:
		var unregister WebsocketUnRegisterRequest

		if err := json.Unmarshal(req.Raw, &unregister); err != nil {
			return err
		}

		s.hub.Unregister(UnRegisterRequest{
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
