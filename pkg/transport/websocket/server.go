package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/HMasataka/conic/internal/eventbus"
	"github.com/HMasataka/conic/internal/logging"
	"github.com/HMasataka/conic/pkg/domain"
	"github.com/HMasataka/conic/pkg/errors"
	"github.com/gorilla/websocket"
	"github.com/rs/xid"
)

// MessageRouter is an interface for routing messages
type MessageRouter interface {
	Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error)
}

// ServerOptions represents websocket server options
type ServerOptions struct {
	ReadBufferSize  int
	WriteBufferSize int
	CheckOrigin     func(r *http.Request) bool
	Hub             domain.Hub
	Logger          *logging.Logger
	EventBus        eventbus.Bus
	Router          MessageRouter
}

// ServerOption is a function that configures ServerOptions
type ServerOption func(*ServerOptions)

// WithHub sets the hub for the server
func WithHub(hub domain.Hub) ServerOption {
	return func(o *ServerOptions) {
		o.Hub = hub
	}
}

// WithLogger sets the logger for the server
func WithLogger(logger *logging.Logger) ServerOption {
	return func(o *ServerOptions) {
		o.Logger = logger
	}
}

// WithEventBus sets the event bus for the server
func WithEventBus(eventBus eventbus.Bus) ServerOption {
	return func(o *ServerOptions) {
		o.EventBus = eventBus
	}
}

// WithCheckOrigin sets the check origin function
func WithCheckOrigin(checkOrigin func(r *http.Request) bool) ServerOption {
	return func(o *ServerOptions) {
		o.CheckOrigin = checkOrigin
	}
}

// Server represents a WebSocket server
type Server struct {
	upgrader websocket.Upgrader
	hub      domain.Hub
	logger   *logging.Logger
	eventBus eventbus.Bus
	options  ServerOptions
}

// NewServer creates a new WebSocket server
func NewServer(opts ...ServerOption) *Server {
	options := ServerOptions{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins by default (configure for production)
		},
	}

	for _, opt := range opts {
		opt(&options)
	}

	return &Server{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  options.ReadBufferSize,
			WriteBufferSize: options.WriteBufferSize,
			CheckOrigin:     options.CheckOrigin,
		},
		hub:      options.Hub,
		logger:   options.Logger,
		eventBus: options.EventBus,
		options:  options,
	}
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade error",
			"error", err,
			"remote_addr", r.RemoteAddr,
		)
		return
	}

	// Generate client ID
	clientID := xid.New().String()

	// Create client options
	clientOptions := DefaultClientOptions()
	clientOptions.ID = clientID

	// Create WebSocket client
	client := NewClient(clientID, conn, s.logger, clientOptions)

	// Set up message handler
	client.Receive(func(message []byte) error {
		return s.handleMessage(client, message)
	})

	// Register client with hub
	if err := s.hub.Register(client); err != nil {
		s.logger.Error("failed to register client",
			"error", err,
			"client_id", clientID,
		)
		client.Close()
		return
	}

	// Publish client connected event
	if s.eventBus != nil {
		event := eventbus.NewEvent(
			eventbus.EventClientConnected,
			"websocket-server",
			map[string]string{
				"client_id":   clientID,
				"remote_addr": r.RemoteAddr,
			},
		)
		s.eventBus.PublishAsync(event)
	}

	// Start client
	client.Start()

	s.logger.Info("client connected",
		"client_id", clientID,
		"remote_addr", r.RemoteAddr,
	)

	// Wait for client to disconnect
	<-client.Context().Done()

	// Unregister client from hub
	if err := s.hub.Unregister(clientID); err != nil {
		s.logger.Error("failed to unregister client",
			"error", err,
			"client_id", clientID,
		)
	}

	// Publish client disconnected event
	if s.eventBus != nil {
		event := eventbus.NewEvent(
			eventbus.EventClientDisconnected,
			"websocket-server",
			map[string]string{
				"client_id": clientID,
			},
		)
		s.eventBus.PublishAsync(event)
	}

	s.logger.Info("client disconnected", "client_id", clientID)
}

// handleMessage handles incoming messages from clients
func (s *Server) handleMessage(client domain.Client, message []byte) error {
	// Log the raw message for debugging
	s.logger.Debug("received message",
		"client_id", client.ID(),
		"size", len(message),
		"content", string(message),
	)

	// Parse the message
	var msg domain.Message
	if err := json.Unmarshal(message, &msg); err != nil {
		s.logger.Error("failed to unmarshal message",
			"client_id", client.ID(),
			"error", err,
			"raw_message", string(message),
		)
		return errors.Wrap(err, errors.ErrorTypeProtocol, "INVALID_MESSAGE", "failed to unmarshal message")
	}

	// Add client ID to message context
	ctx := context.WithValue(context.Background(), "client_id", client.ID())
	
	// Route to appropriate handler
	if s.options.Router != nil {
		s.logger.Info("routing message",
			"client_id", client.ID(),
			"message_type", msg.Type,
		)
		
		response, err := s.options.Router.Handle(ctx, &msg)
		if err != nil {
			s.logger.Error("handler error",
				"client_id", client.ID(),
				"message_type", msg.Type,
				"error", err,
			)
			return err
		}

		// Send response if any
		if response != nil {
			s.logger.Info("sending response",
				"client_id", client.ID(),
				"response_type", response.Type,
			)
			
			responseData, err := json.Marshal(response)
			if err != nil {
				return errors.Wrap(err, errors.ErrorTypeInternal, "MARSHAL_ERROR", "failed to marshal response")
			}
			
			s.logger.Debug("response data", 
				"client_id", client.ID(),
				"data", string(responseData),
			)
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			return client.Send(ctx, responseData)
		} else {
			s.logger.Info("no response from handler",
				"client_id", client.ID(),
				"message_type", msg.Type,
			)
		}
	} else {
		s.logger.Warn("no router configured")
	}
	
	return nil
}

