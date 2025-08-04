package conic

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
	"github.com/HMasataka/conic/router"
	"github.com/gorilla/websocket"
	"github.com/rs/xid"
)

type ClientOptions struct {
	ID              string
	WriteTimeout    time.Duration
	ReadTimeout     time.Duration
	PingInterval    time.Duration
	MaxMessageSize  int64
	ReadBufferSize  int
	WriteBufferSize int
}

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

type Client struct {
	id       string
	ctx      context.Context
	conn     *websocket.Conn
	cancel   context.CancelFunc
	router   *router.Router
	logger   *logging.Logger
	options  ClientOptions
	sendChan chan []byte
	handler  domain.MessageHandler
	mutex    sync.RWMutex
	closed   bool
	wg       sync.WaitGroup
}

func NewClient(conn *websocket.Conn, router *router.Router, logger *logging.Logger, options ClientOptions) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	id := xid.New().String()

	return &Client{
		id:       id,
		conn:     conn,
		router:   router,
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger.WithFields(map[string]any{"client_id": id}),
		options:  options,
		sendChan: make(chan []byte, 256),
	}
}

func (c *Client) ID() string {
	return c.id
}

func (c *Client) Send(ctx context.Context, message []byte) error {
	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return errors.New("")
	}
	c.mutex.RUnlock()

	select {
	case c.sendChan <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.ctx.Done():
		return errors.New("")
	default:
		return errors.New("")
	}
}

func (c *Client) SetHandler(handler domain.MessageHandler) error {
	c.handler = handler
	return nil
}

func (c *Client) Close() error {
	c.mutex.Lock()
	if c.closed {
		c.mutex.Unlock()
		return nil
	}
	c.closed = true
	c.mutex.Unlock()

	c.logger.Info("closing client connection")

	c.cancel()

	close(c.sendChan)

	if err := c.conn.Close(); err != nil {
		c.logger.Error("error closing websocket connection", "error", err)
	}

	c.wg.Wait()

	return nil
}

func (c *Client) Context() context.Context {
	return c.ctx
}

func (c *Client) Start() {
	c.wg.Add(2)
	go c.readPump()
	c.writePump()
}

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
			for range n {
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
