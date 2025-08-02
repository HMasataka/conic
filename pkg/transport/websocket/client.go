package websocket

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/HMasataka/conic/internal/logging"
	"github.com/HMasataka/conic/pkg/domain"
	"github.com/HMasataka/conic/pkg/errors"
	"github.com/gorilla/websocket"
)

// ClientOptions represents websocket client options
type ClientOptions struct {
	ID              string
	WriteTimeout    time.Duration
	ReadTimeout     time.Duration
	PingInterval    time.Duration
	MaxMessageSize  int64
	ReadBufferSize  int
	WriteBufferSize int
}

// DefaultClientOptions returns default client options
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		WriteTimeout:    10 * time.Second,
		ReadTimeout:     60 * time.Second,
		PingInterval:    30 * time.Second,
		MaxMessageSize:  512 * 1024, // 512KB
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
}

// Client implements the domain.Client interface for WebSocket
type Client struct {
	id       string
	conn     *websocket.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	logger   *logging.Logger
	options  ClientOptions
	sendChan chan []byte
	handler  domain.MessageHandler
	mu       sync.RWMutex
	closed   bool
	wg       sync.WaitGroup
}

// NewClient creates a new WebSocket client
func NewClient(id string, conn *websocket.Conn, logger *logging.Logger, options ClientOptions) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		id:       id,
		conn:     conn,
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger.WithFields(map[string]any{"client_id": id}),
		options:  options,
		sendChan: make(chan []byte, 256),
	}
}

// ID implements domain.Client
func (c *Client) ID() string {
	return c.id
}

// Send implements domain.Client
func (c *Client) Send(ctx context.Context, message []byte) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return domain.ErrConnectionClosed
	}
	c.mu.RUnlock()

	select {
	case c.sendChan <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.ctx.Done():
		return domain.ErrConnectionClosed
	default:
		return errors.New(errors.ErrorTypeTransport, "SEND_BUFFER_FULL", "send buffer is full")
	}
}

// Receive implements domain.Client
func (c *Client) Receive(handler domain.MessageHandler) error {
	c.handler = handler
	return nil
}

// Close implements domain.Client
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	c.logger.Info("closing client connection")

	// Cancel context to stop goroutines
	c.cancel()

	// Close send channel
	close(c.sendChan)

	// Close websocket connection
	if err := c.conn.Close(); err != nil {
		c.logger.Error("error closing websocket connection", "error", err)
	}

	// Wait for goroutines to finish
	c.wg.Wait()

	return nil
}

// Context implements domain.Client
func (c *Client) Context() context.Context {
	return c.ctx
}

// Start starts the client read and write pumps
func (c *Client) Start() {
	c.wg.Add(2)
	go c.readPump()
	go c.writePump()
}

// readPump pumps messages from the websocket connection
func (c *Client) readPump() {
	defer c.wg.Done()
	defer func() {
		c.logger.Debug("read pump stopped")
		c.Close()
	}()

	c.conn.SetReadLimit(c.options.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.options.ReadTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.options.ReadTimeout))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			messageType, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.logger.Error("websocket read error", "error", err)
				}
				return
			}

			if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
				continue
			}

			if c.handler != nil {
				if err := c.handler(message); err != nil {
					c.logger.Error("message handler error", "error", err)
				}
			}
		}
	}
}

// writePump pumps messages to the websocket connection
func (c *Client) writePump() {
	defer c.wg.Done()
	defer func() {
		c.logger.Debug("write pump stopped")
	}()

	ticker := time.NewTicker(c.options.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return

		case message, ok := <-c.sendChan:
			c.conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))

			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.logger.Error("websocket write error", "error", err)
				return
			}

			// Drain any queued messages
			n := len(c.sendChan)
			for i := 0; i < n; i++ {
				select {
				case msg := <-c.sendChan:
					if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
						c.logger.Error("websocket write error", "error", err)
						return
					}
				default:
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Error("websocket ping error", "error", err)
				return
			}
		}
	}
}

// ClientFactory creates WebSocket clients
type ClientFactory struct {
	logger  *logging.Logger
	options ClientOptions
}

// NewClientFactory creates a new client factory
func NewClientFactory(logger *logging.Logger, options ClientOptions) *ClientFactory {
	return &ClientFactory{
		logger:  logger,
		options: options,
	}
}

// Create implements domain.ClientFactory
func (f *ClientFactory) Create(conn io.ReadWriteCloser) (domain.Client, error) {
	// For WebSocket, we expect the connection to be passed as-is
	// The factory is designed for generic connections, but WebSocket needs special handling
	return nil, errors.New(errors.ErrorTypeInternal, "NOT_IMPLEMENTED", "WebSocket client factory not implemented for generic connections")
}

// CreateFromWebSocket creates a client from a WebSocket connection
func (f *ClientFactory) CreateFromWebSocket(conn *websocket.Conn) (domain.Client, error) {
	client := NewClient(f.options.ID, conn, f.logger, f.options)
	client.Start()

	return client, nil
}
