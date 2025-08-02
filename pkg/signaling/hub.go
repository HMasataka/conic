package signaling

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HMasataka/conic/internal/eventbus"
	"github.com/HMasataka/conic/internal/logging"
	"github.com/HMasataka/conic/pkg/domain"
	"github.com/HMasataka/conic/pkg/errors"
)

// HubOptions represents hub configuration options
type HubOptions struct {
	Logger   *logging.Logger
	EventBus eventbus.Bus
}

// Hub implements the domain.Hub interface
type Hub struct {
	clients    sync.Map // map[string]domain.Client
	register   chan domain.Client
	unregister chan string
	broadcast  chan []byte
	sendTo     chan sendMessage
	logger     *logging.Logger
	eventBus   eventbus.Bus
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	// Statistics
	messagesSent     int64
	messagesReceived int64
	startTime        time.Time
}

type sendMessage struct {
	clientID string
	message  []byte
}

// NewHub creates a new hub
func NewHub(logger *logging.Logger, eventBus eventbus.Bus) *Hub {
	return &Hub{
		register:   make(chan domain.Client, 100),
		unregister: make(chan string, 100),
		broadcast:  make(chan []byte, 1000),
		sendTo:     make(chan sendMessage, 1000),
		logger:     logger,
		eventBus:   eventBus,
		startTime:  time.Now(),
	}
}

// Start implements domain.Hub
func (h *Hub) Start(ctx context.Context) error {
	h.ctx, h.cancel = context.WithCancel(ctx)
	h.wg.Add(1)
	go h.run()
	h.logger.Info("hub started")
	return nil
}

// Stop implements domain.Hub
func (h *Hub) Stop() error {
	h.logger.Info("stopping hub")
	h.cancel()
	h.wg.Wait()

	// Close all client connections
	h.clients.Range(func(key, value interface{}) bool {
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

// Register implements domain.Hub
func (h *Hub) Register(client domain.Client) error {
	select {
	case h.register <- client:
		return nil
	case <-h.ctx.Done():
		return domain.ErrHubStopped
	default:
		return errors.New(errors.ErrorTypeInternal, "REGISTER_QUEUE_FULL", "register queue is full")
	}
}

// Unregister implements domain.Hub
func (h *Hub) Unregister(clientID string) error {
	select {
	case h.unregister <- clientID:
		return nil
	case <-h.ctx.Done():
		return domain.ErrHubStopped
	default:
		return errors.New(errors.ErrorTypeInternal, "UNREGISTER_QUEUE_FULL", "unregister queue is full")
	}
}

// Broadcast implements domain.Hub
func (h *Hub) Broadcast(message []byte) error {
	select {
	case h.broadcast <- message:
		atomic.AddInt64(&h.messagesReceived, 1)
		return nil
	case <-h.ctx.Done():
		return domain.ErrHubStopped
	default:
		return errors.New(errors.ErrorTypeInternal, "BROADCAST_QUEUE_FULL", "broadcast queue is full")
	}
}

// SendTo implements domain.Hub
func (h *Hub) SendTo(clientID string, message []byte) error {
	select {
	case h.sendTo <- sendMessage{clientID: clientID, message: message}:
		atomic.AddInt64(&h.messagesReceived, 1)
		return nil
	case <-h.ctx.Done():
		return domain.ErrHubStopped
	default:
		return errors.New(errors.ErrorTypeInternal, "SENDTO_QUEUE_FULL", "send queue is full")
	}
}

// SendToMultiple implements domain.Hub
func (h *Hub) SendToMultiple(clientIDs []string, message []byte) error {
	for _, clientID := range clientIDs {
		if err := h.SendTo(clientID, message); err != nil {
			// Log error but continue sending to other clients
			h.logger.Error("failed to send to client",
				"client_id", clientID,
				"error", err,
			)
		}
	}
	return nil
}

// GetClient implements domain.Hub
func (h *Hub) GetClient(clientID string) (domain.Client, bool) {
	if value, ok := h.clients.Load(clientID); ok {
		return value.(domain.Client), true
	}
	return nil, false
}

// GetClients implements domain.Hub
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

// run is the main hub loop
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

// handleRegister handles client registration
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

// handleUnregister handles client unregistration
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

// handleBroadcast handles broadcasting messages to all clients
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

// handleSendTo handles sending a message to a specific client
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

// getClientCount returns the number of connected clients
func (h *Hub) getClientCount() int {
	count := 0
	h.clients.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// GetStats returns hub statistics
func (h *Hub) GetStats() domain.HubStats {
	return domain.HubStats{
		ConnectedClients: h.getClientCount(),
		MessagesSent:     atomic.LoadInt64(&h.messagesSent),
		MessagesReceived: atomic.LoadInt64(&h.messagesReceived),
		Uptime:           time.Since(h.startTime).Seconds(),
	}
}
