package conic

import (
	"errors"
	"io"
)

type Hub interface {
	io.WriteCloser
	Error(err error)
}

type hub struct {
	clients     map[string]Client
	dataChannel chan MessageRequest
	register    chan RegisterRequest
	unregister  chan UnRegisterRequest
}

func NewHub() *hub {
	return &hub{
		dataChannel: make(chan MessageRequest),
		register:    make(chan RegisterRequest),
		unregister:  make(chan UnRegisterRequest),
		clients:     make(map[string]Client),
	}
}

func (h *hub) Run() {
	for {
		select {
		case req := <-h.register:
			h.clients[req.ID] = req.Client
		case req := <-h.unregister:
			if client, ok := h.clients[req.ID]; ok {
				delete(h.clients, req.ID)

				if err := client.Close(); err != nil {
					client.Error(err)
				}
			}
		case req := <-h.dataChannel:
			if client, ok := h.clients[req.TargetID]; ok {
				if err := client.Write(req.Message); err != nil {
					client.Error(err)
				}
			} else {
				if client, ok := h.clients[req.ID]; ok {
					client.Error(errors.New("target not found"))
				}
			}
		}
	}
}

type MessageRequest struct {
	ID       string
	TargetID string
	Message  string
}

func (h *hub) SendMessage(req MessageRequest) {
	h.dataChannel <- req
}

type RegisterRequest struct {
	ID     string
	Client Client
}

func (h *hub) Register(req RegisterRequest) {
	h.register <- req
}

type UnRegisterRequest struct {
	ID     string
	Client Client
}

func (h *hub) Unregister(req UnRegisterRequest) {
	h.unregister <- req
}
