package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
	"github.com/HMasataka/conic/router"
	ws "github.com/gorilla/websocket"
)

type ConnectionOptions struct {
	WriteTimeout    time.Duration
	ReadTimeout     time.Duration
	PingInterval    time.Duration
	MaxMessageSize  int64
	ReadBufferSize  int
	WriteBufferSize int
}

func DefaultConnectionOptions() ConnectionOptions {
	return ConnectionOptions{
		WriteTimeout:    10 * time.Second,
		ReadTimeout:     60 * time.Second,
		PingInterval:    30 * time.Second,
		MaxMessageSize:  512 * 1024, // 512KB
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
}

type Connection struct {
	ctx      context.Context
	conn     *ws.Conn
	cancel   context.CancelFunc
	router   *router.Router
	logger   *logging.Logger
	options  ConnectionOptions
	sendChan chan []byte
	mutex    sync.RWMutex
	closed   bool
}

func NewConnection(conn *ws.Conn, router *router.Router, logger *logging.Logger, options ConnectionOptions) *Connection {
	ctx, cancel := context.WithCancel(context.Background())

	return &Connection{
		ctx:      ctx,
		conn:     conn,
		router:   router,
		cancel:   cancel,
		logger:   logger,
		options:  options,
		sendChan: make(chan []byte, 256),
	}
}

func (c *Connection) Send(ctx context.Context, message []byte) error {
	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return errors.New("connection is closed")
	}
	c.mutex.RUnlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.ctx.Done():
		return errors.New("connection context done")
	case c.sendChan <- message:
		return nil
	default:
		return errors.New("send channel full or blocked")
	}
}

func (c *Connection) Close() error {
	c.mutex.Lock()
	if c.closed {
		c.mutex.Unlock()
		return nil
	}
	c.closed = true
	c.mutex.Unlock()

	c.logger.Info("closing websocket connection")

	c.cancel()
	close(c.sendChan)

	if err := c.conn.Close(); err != nil {
		c.logger.Error("error closing websocket connection", "error", err)
		return err
	}

	return nil
}

func (c *Connection) Context() context.Context {
	return c.ctx
}

func (c *Connection) Start(ctx context.Context) {
	done := make(chan struct{})

	go func() {
		defer close(done)
		c.readPump(ctx)
	}()

	go c.writePump(ctx)

	<-done
	c.logger.Info("connection closed")
}

func (c *Connection) readPump(ctx context.Context) {
	defer func() {
		c.logger.Info("read pump stopped")
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
		case <-ctx.Done():
			return
		default:
			messageType, message, err := c.conn.ReadMessage()
			if err != nil {
				if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseAbnormalClosure) {
					c.logger.Error("websocket unexpected close error", "error", err)
				} else {
					c.logger.Info("websocket connection closed", "error", err)
				}
				return
			}

			if messageType != ws.TextMessage && messageType != ws.BinaryMessage {
				continue
			}

			c.logger.Info("Received message", "message", string(message))

			var msg domain.Message
			if err := json.Unmarshal(message, &msg); err != nil {
				c.logger.Error("Failed to unmarshal message", "error", err)
				continue
			}

			response, err := c.router.Handle(ctx, &msg)
			if err != nil {
				c.logger.Error("Failed to handle message", "error", err)
				continue
			}

			if response != nil {
				respData, err := json.Marshal(response)
				if err != nil {
					c.logger.Error("Failed to marshal response", "error", err)
					continue
				}

				if err := c.Send(ctx, respData); err != nil {
					c.logger.Error("Failed to send response", "error", err)
					continue
				}
			}
		}
	}
}

func (c *Connection) writePump(ctx context.Context) {
	defer func() {
		c.logger.Debug("write pump stopped")
	}()

	var ticker *time.Ticker
	if c.options.PingInterval > 0 {
		ticker = time.NewTicker(c.options.PingInterval)
		defer ticker.Stop()
	}

	for {
		if ticker != nil {
			select {
			case <-c.ctx.Done():
				return
			case <-ctx.Done():
				return
			case message, ok := <-c.sendChan:
				c.conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))

				if !ok {
					c.conn.WriteMessage(ws.CloseMessage, []byte{})
					return
				}

				if err := c.conn.WriteMessage(ws.TextMessage, message); err != nil {
					c.logger.Error("websocket write error", "error", err)
					return
				}

				n := len(c.sendChan)
				for range n {
					select {
					case msg := <-c.sendChan:
						if err := c.conn.WriteMessage(ws.TextMessage, msg); err != nil {
							c.logger.Error("websocket write error", "error", err)
							return
						}
					default:
					}
				}

			case <-ticker.C:
				c.conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))
				if err := c.conn.WriteMessage(ws.PingMessage, nil); err != nil {
					c.logger.Error("websocket ping error", "error", err)
					return
				}
			}
		} else {
			select {
			case <-c.ctx.Done():
				return
			case <-ctx.Done():
				return
			case message, ok := <-c.sendChan:
				c.conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))

				if !ok {
					c.conn.WriteMessage(ws.CloseMessage, []byte{})
					return
				}

				if err := c.conn.WriteMessage(ws.TextMessage, message); err != nil {
					c.logger.Error("websocket write error", "error", err)
					return
				}

				n := len(c.sendChan)
				for range n {
					select {
					case msg := <-c.sendChan:
						if err := c.conn.WriteMessage(ws.TextMessage, msg); err != nil {
							c.logger.Error("websocket write error", "error", err)
							return
						}
					default:
					}
				}
			}
		}
	}
}
