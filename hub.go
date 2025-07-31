package conic

import (
	"errors"
	"io"
	"log"
)

type HubClient interface {
	io.WriteCloser
	Error(err error)
}

type Hub interface {
	Run()
	SendMessage(req MessageRequest)
	Register(req RegisterRequest)
	Unregister(req UnRegisterRequest)
}

type hub struct {
	clients     map[string]HubClient
	dataChannel chan MessageRequest
	register    chan RegisterRequest
	unregister  chan UnRegisterRequest
}

func NewHub() Hub {
	return &hub{
		dataChannel: make(chan MessageRequest),
		register:    make(chan RegisterRequest),
		unregister:  make(chan UnRegisterRequest),
		clients:     make(map[string]HubClient),
	}
}

func (h *hub) Run() {
	for {
		select {
		case req := <-h.register:
			h.clients[req.ID] = req.Client
			log.Println("register", req.ID)
		case req := <-h.unregister:
			if client, ok := h.clients[req.ID]; ok {
				if err := client.Close(); err != nil {
					log.Println("client close err: ", err)
				}

				delete(h.clients, req.ID)
				log.Println("unregister", req.ID)
			}
		case req := <-h.dataChannel:
			if client, ok := h.clients[req.TargetID]; ok {
				if _, err := client.Write(req.Message); err != nil {
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
	ID       string `json:"id"`
	TargetID string `json:"target_id"`
	Message  []byte `json:"message"`
}

func (h *hub) SendMessage(req MessageRequest) {
	h.dataChannel <- req
}

type RegisterRequest struct {
	ID     string    `json:"id"`
	Client HubClient `json:"client"`
}

func (h *hub) Register(req RegisterRequest) {
	h.register <- req
}

type UnRegisterRequest struct {
	ID string `json:"id"`
}

func (h *hub) Unregister(req UnRegisterRequest) {
	h.unregister <- req
}
