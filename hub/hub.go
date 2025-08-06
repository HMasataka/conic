package hub

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
)

type HubOptions struct {
	Logger *logging.Logger
}

type Hub struct {
	clients    sync.Map // map[string]domain.Client
	register   chan domain.Client
	unregister chan string
	broadcast  chan []byte
	sendTo     chan sendMessage
	logger     *logging.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	messagesSent     int64
	messagesReceived int64
	startTime        time.Time
}

type sendMessage struct {
	clientID string
	message  []byte
}

func New(logger *logging.Logger) *Hub {
	return &Hub{
		register:   make(chan domain.Client, 100),
		unregister: make(chan string, 100),
		broadcast:  make(chan []byte, 1000),
		sendTo:     make(chan sendMessage, 1000),
		logger:     logger,
		startTime:  time.Now(),
	}
}

func (h *Hub) Start(ctx context.Context) error {
	h.ctx, h.cancel = context.WithCancel(ctx)
	h.wg.Add(1)
	go h.run()
	h.logger.Info("hub started")
	return nil
}

func (h *Hub) Stop() error {
	h.logger.Info("stopping hub")
	h.cancel()
	h.wg.Wait()

	h.clients.Range(func(key, value any) bool {
		if client, ok := value.(domain.Client); ok {
			client.Close()
		}
		return true
	})

	close(h.register)
	close(h.unregister)
	close(h.broadcast)
	close(h.sendTo)

	h.logger.Info("hub stopped")
	return nil
}

func (h *Hub) Register(client domain.Client) error {
	select {
	case h.register <- client:
		return nil
	case <-h.ctx.Done():
		return errors.New("hub context cancelled during registration")
	default:
		return errors.New("registration channel is full")
	}
}

func (h *Hub) Unregister(clientID string) error {
	select {
	case h.unregister <- clientID:
		return nil
	case <-h.ctx.Done():
		return errors.New("hub context cancelled during unregistration")
	default:
		return errors.New("unregistration channel is full")
	}
}

func (h *Hub) Broadcast(message []byte) error {
	select {
	case h.broadcast <- message:
		atomic.AddInt64(&h.messagesReceived, 1)
		return nil
	case <-h.ctx.Done():
		return errors.New("hub context cancelled during broadcast")
	default:
		return errors.New("broadcast channel is full")
	}
}

func (h *Hub) SendTo(clientID string, message []byte) error {
	select {
	case h.sendTo <- sendMessage{clientID: clientID, message: message}:
		atomic.AddInt64(&h.messagesReceived, 1)
		return nil
	case <-h.ctx.Done():
		return errors.New("hub context cancelled during send")
	default:
		return errors.New("send channel is full")
	}
}

func (h *Hub) SendToMultiple(clientIDs []string, message []byte) error {
	for _, clientID := range clientIDs {
		if err := h.SendTo(clientID, message); err != nil {
			h.logger.Error("failed to send to client",
				"client_id", clientID,
				"error", err,
			)
		}
	}
	return nil
}

func (h *Hub) GetClient(clientID string) (domain.Client, bool) {
	if value, ok := h.clients.Load(clientID); ok {
		return value.(domain.Client), true
	}
	return nil, false
}

func (h *Hub) GetClients() []domain.Client {
	var clients []domain.Client
	h.clients.Range(func(key, value interface{}) bool {
		if client, ok := value.(domain.Client); ok {
			clients = append(clients, client)
		}
		return true
	})
	return clients
}

func (h *Hub) run() {
	defer h.wg.Done()

	for {
		select {
		case <-h.ctx.Done():
			return

		case client := <-h.register:
			h.handleRegister(client)

		case clientID := <-h.unregister:
			h.handleUnregister(clientID)

		case message := <-h.broadcast:
			h.handleBroadcast(message)

		case msg := <-h.sendTo:
			h.handleSendTo(msg.clientID, msg.message)
		}
	}
}

func (h *Hub) handleRegister(client domain.Client) {
	clientID := client.ID()

	// Check if client already exists
	if _, exists := h.clients.Load(clientID); exists {
		h.logger.Warn("client already registered", "client_id", clientID)
		return
	}

	// Store client
	h.clients.Store(clientID, client)

	h.logger.Info("client registered",
		"client_id", clientID,
		"total_clients", h.getClientCount(),
	)
}

func (h *Hub) handleUnregister(clientID string) {
	if client, ok := h.clients.LoadAndDelete(clientID); ok {
		// Close client connection
		if c, ok := client.(domain.Client); ok {
			c.Close()
		}

		h.logger.Info("client unregistered",
			"client_id", clientID,
			"total_clients", h.getClientCount(),
		)
	}
}

func (h *Hub) handleBroadcast(message []byte) {
	var successCount, errorCount int

	h.clients.Range(func(key, value interface{}) bool {
		if client, ok := value.(domain.Client); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := client.Send(ctx, message)
			cancel()

			if err != nil {
				errorCount++
				h.logger.Error("failed to send to client",
					"client_id", client.ID(),
					"error", err,
				)
			} else {
				successCount++
				atomic.AddInt64(&h.messagesSent, 1)
			}
		}
		return true
	})

	h.logger.Debug("broadcast complete",
		"success_count", successCount,
		"error_count", errorCount,
	)
}

func (h *Hub) handleSendTo(clientID string, message []byte) {
	client, ok := h.GetClient(clientID)
	if !ok {
		h.logger.Warn("client not found", "client_id", clientID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := client.Send(ctx, message)
	cancel()

	if err != nil {
		h.logger.Error("failed to send to client",
			"client_id", clientID,
			"error", err,
		)
	} else {
		atomic.AddInt64(&h.messagesSent, 1)
	}
}

func (h *Hub) getClientCount() int {
	count := 0
	h.clients.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func (h *Hub) GetStats() domain.HubStats {
	return domain.HubStats{
		ConnectedClients: h.getClientCount(),
		MessagesSent:     atomic.LoadInt64(&h.messagesSent),
		MessagesReceived: atomic.LoadInt64(&h.messagesReceived),
		Uptime:           time.Since(h.startTime).Seconds(),
	}
}
