package signaling

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/HMasataka/conic/internal/logging"
	"github.com/HMasataka/conic/pkg/domain"
	"github.com/HMasataka/conic/pkg/errors"
	"github.com/HMasataka/conic/pkg/transport/websocket"
	gorillaws "github.com/gorilla/websocket"
	"github.com/rs/xid"
)

// ClientOptions represents signaling client options
type ClientOptions struct {
	Logger        *logging.Logger
	AutoReconnect bool
	ReconnectWait time.Duration
	MaxReconnect  int
}

// DefaultClientOptions returns default client options
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		AutoReconnect: true,
		ReconnectWait: 5 * time.Second,
		MaxReconnect:  10,
	}
}

// Client represents a signaling client
type Client struct {
	url      url.URL
	options  ClientOptions
	logger   *logging.Logger
	wsClient domain.Client
	conn     *gorillaws.Conn

	id         string
	registered bool

	handlers   map[domain.MessageType]domain.MessageHandlerFunc
	handlersMu sync.RWMutex

	ctx           context.Context
	cancel        context.CancelFunc
	reconnectChan chan struct{}

	mu sync.RWMutex
}

// NewClient creates a new signaling client
func NewClient(serverURL url.URL, options ClientOptions) *Client {
	if options.Logger == nil {
		options.Logger = logging.New(logging.Config{Level: "info", Format: "text"})
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		url:           serverURL,
		options:       options,
		logger:        options.Logger,
		handlers:      make(map[domain.MessageType]domain.MessageHandlerFunc),
		ctx:           ctx,
		cancel:        cancel,
		reconnectChan: make(chan struct{}, 1),
	}
}

// Connect establishes connection to the signaling server
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("connecting to signaling server", "url", c.url.String())

	// Dial websocket
	conn, _, err := gorillaws.DefaultDialer.Dial(c.url.String(), nil)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeTransport, "DIAL_ERROR", "failed to connect to server")
	}

	c.conn = conn

	// Create websocket client wrapper
	clientOptions := websocket.DefaultClientOptions()
	clientOptions.ID = xid.New().String()

	c.logger.Debug("creating websocket client", "id", clientOptions.ID, "url", c.url.String())

	c.wsClient = websocket.NewClient(clientOptions.ID, conn, c.logger, clientOptions)

	// Set up message handler
	c.wsClient.Receive(c.handleMessage)

	// Start the websocket client
	if wsClientImpl, ok := c.wsClient.(*websocket.Client); ok {
		wsClientImpl.Start()
	}

	// Send registration request
	if err := c.sendRegistration(); err != nil {
		c.wsClient.Close()
		return err
	}

	c.logger.Info("connected to signaling server", "url", c.url.String())

	return nil
}

// Disconnect closes the connection
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cancel()

	if c.wsClient != nil {
		return c.wsClient.Close()
	}

	return nil
}

// ID returns the client ID
func (c *Client) ID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.id
}

// IsRegistered returns whether the client is registered
func (c *Client) IsRegistered() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.registered
}

// SendSDP sends an SDP message
func (c *Client) SendSDP(targetID string, sdp interface{}) error {
	sdpMsg := domain.SDPMessage{
		FromID: c.ID(),
		ToID:   targetID,
	}

	// Type assert to webrtc.SessionDescription if needed
	// For now, assume sdp is already the correct type
	if sessionDesc, ok := sdp.(interface{ Type() string }); ok {
		c.logger.Debug("sending SDP", "type", sessionDesc.Type(), "to", targetID)
	}

	return c.sendMessage(domain.MessageTypeSDP, sdpMsg)
}

// SendICECandidate sends an ICE candidate
func (c *Client) SendICECandidate(targetID string, candidate interface{}) error {
	iceMsg := domain.ICECandidateMessage{
		FromID: c.ID(),
		ToID:   targetID,
	}

	// Type conversion would go here

	return c.sendMessage(domain.MessageTypeCandidate, iceMsg)
}

// SendDataChannelMessage sends a data channel message
func (c *Client) SendDataChannelMessage(targetID, label string, payload []byte) error {
	dcMsg := domain.DataChannelMessage{
		FromID:  c.ID(),
		ToID:    targetID,
		Label:   label,
		Payload: payload,
	}

	return c.sendMessage(domain.MessageTypeDataChannel, dcMsg)
}

// OnMessage registers a handler for a specific message type
func (c *Client) OnMessage(messageType domain.MessageType, handler domain.MessageHandlerFunc) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[messageType] = handler
}

// handleMessage processes incoming messages
func (c *Client) handleMessage(data []byte) error {
	var msg domain.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Error("failed to unmarshal message", "error", err)
		return err
	}

	// Handle registration response
	if msg.Type == domain.MessageTypeRegister && !c.IsRegistered() {
		var resp domain.RegisterResponse
		if err := json.Unmarshal(msg.Data, &resp); err != nil {
			return err
		}

		c.mu.Lock()
		c.id = resp.ClientID
		c.registered = resp.Success
		c.mu.Unlock()

		c.logger.Info("registered with server", "client_id", c.id)
		return nil
	}

	// Route to registered handlers
	c.handlersMu.RLock()
	handler, exists := c.handlers[msg.Type]
	c.handlersMu.RUnlock()

	if exists {
		ctx := context.WithValue(c.ctx, "message_id", msg.ID)
		return handler(ctx, msg)
	}

	c.logger.Warn("no handler for message type", "type", msg.Type)
	return nil
}

// sendMessage sends a message to the server
func (c *Client) sendMessage(messageType domain.MessageType, data interface{}) error {
	c.mu.RLock()
	wsClient := c.wsClient
	c.mu.RUnlock()

	if wsClient == nil {
		return errors.New(errors.ErrorTypeTransport, "NOT_CONNECTED", "not connected to server")
	}

	msgData, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "MARSHAL_ERROR", "failed to marshal message data")
	}

	msg := domain.Message{
		ID:        xid.New().String(),
		Type:      messageType,
		Timestamp: time.Now(),
		Data:      msgData,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "MARSHAL_ERROR", "failed to marshal message")
	}

	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()

	return wsClient.Send(ctx, msgBytes)
}

// sendRegistration sends a registration request
func (c *Client) sendRegistration() error {
	c.logger.Info("sending registration request")
	req := domain.RegisterRequest{}
	err := c.sendMessage(domain.MessageTypeRegister, req)
	if err != nil {
		c.logger.Error("failed to send registration", "error", err)
	} else {
		c.logger.Info("registration request sent successfully")
	}
	return err
}

// WaitForRegistration waits until the client is registered
func (c *Client) WaitForRegistration(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("registration timeout")
		case <-ticker.C:
			if c.IsRegistered() {
				return nil
			}
		}
	}
}
