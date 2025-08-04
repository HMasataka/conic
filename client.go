package conic

import (
	"context"
	"encoding/json"
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
		return errors.New("client is closed")
	}
	c.mutex.RUnlock()

	select {
	case c.sendChan <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.ctx.Done():
		return errors.New("client context done")
	default:
		return errors.New("send channel full or blocked")
	}
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
		c.logger.Info("client read pump stopped")
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
					c.logger.Error("client websocket unexpected close error", "error", err)
				} else {
					c.logger.Info("client websocket connection closed", "error", err)
				}
				return
			}

			if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
				continue
			}

			c.logger.Info("Received message from server", "message", string(message))

			var msg domain.Message
			if err := json.Unmarshal(message, &msg); err != nil {
				c.logger.Error("Failed to unmarshal message", "error", err)
				return
			}

			ctx := context.Background()
			response, err := c.router.Handle(ctx, &msg)
			if err != nil {
				c.logger.Error("Failed to handle message", "error", err)
				return
			}

			if response != nil {
				respData, err := json.Marshal(response)
				if err != nil {
					c.logger.Error("Failed to marshal response", "error", err)
					return
				}

				if err := c.Send(ctx, respData); err != nil {
					c.logger.Error("Failed to send response", "error", err)
					return
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
