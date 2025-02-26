package conic

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func NewSocket() *Socket {
	return &Socket{
		done: make(chan struct{}),
	}
}

type Socket struct {
	dataChannel chan []byte
	done        chan struct{}
}

func (s *Socket) Serve(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	go s.read(conn)

	s.write(conn)
}

func (s *Socket) Write(message []byte) (int, error) {
	s.dataChannel <- message
	return len(message), nil
}

func (s *Socket) read(conn *websocket.Conn) {
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Print("read error:", err)
			return
		}

		switch messageType {
		case websocket.TextMessage:
			log.Printf("text recv: %s\n", string(message))
		case websocket.CloseMessage:
			close(s.done)
		default:
			log.Printf("message type: %v, message: %s\n", messageType, string(message))
		}
	}
}

func (s *Socket) write(conn *websocket.Conn) {
	for {
		select {
		case <-s.done:
			return
		case t := <-s.dataChannel:
			if err := conn.WriteMessage(websocket.TextMessage, t); err != nil {
				return
			}
		}
	}
}
