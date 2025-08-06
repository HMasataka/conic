package signal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/HMasataka/conic"
	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
	"github.com/HMasataka/conic/router"
	"github.com/gorilla/websocket"
)

type ServerOptions struct {
	WriteTimeout    time.Duration
	ReadTimeout     time.Duration
	MaxMessageSize  int64
	ReadBufferSize  int
	WriteBufferSize int
}

func DefaultServerOptions() ServerOptions {
	return ServerOptions{
		WriteTimeout:    10 * time.Second,
		ReadTimeout:     60 * time.Second,
		MaxMessageSize:  512 * 1024, // 512KB
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
}

type Server struct {
	ctx      context.Context
	upgrader websocket.Upgrader
	router   *router.Router
	cancel   context.CancelFunc
	logger   *logging.Logger
	options  ServerOptions
	sendChan chan []byte
	mutex    sync.RWMutex
	closed   bool
}

func NewServer(router *router.Router, logger *logging.Logger, options ServerOptions) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for simplicity, adjust as needed
		},
		ReadBufferSize:  options.ReadBufferSize,
		WriteBufferSize: options.WriteBufferSize,
	}

	return &Server{
		ctx:      ctx,
		upgrader: upgrader,
		router:   router,
		cancel:   cancel,
		logger:   logger,
		options:  options,
		sendChan: make(chan []byte, 256),
	}
}

func (c *Server) Send(ctx context.Context, message []byte) error {
	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return errors.New("server is closed")
	}
	c.mutex.RUnlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.ctx.Done():
		return errors.New("server context done")
	case c.sendChan <- message:
		return nil
	default:
		return errors.New("send channel full or blocked")
	}
}

func (c *Server) Close() error {
	c.mutex.Lock()
	if c.closed {
		c.mutex.Unlock()
		return nil
	}
	c.closed = true
	c.mutex.Unlock()

	c.logger.Info("closing server connection")

	c.cancel()

	close(c.sendChan)

	return nil
}

func (c *Server) Context() context.Context {
	return c.ctx
}

func (c *Server) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		c.logger.Error("failed to upgrade connection", "error", err)
		return
	}

	c.logger.Info("websocket connection established")

	ctx := conic.WithConnection(r.Context(), conn)

	c.handleConnection(ctx, conn)
}

func (c *Server) handleConnection(ctx context.Context, conn *websocket.Conn) {
	defer conn.Close()
	defer c.logger.Info("websocket connection handler finished")

	done := make(chan struct{})

	go func() {
		defer close(done)
		c.readPump(ctx, conn)
	}()

	go c.writePump(ctx, conn)

	// Wait for read pump to finish
	<-done
	c.logger.Info("connection closed")
}

func (c *Server) readPump(ctx context.Context, conn *websocket.Conn) {
	defer func() {
		c.logger.Info("server read pump stopped")
	}()

	conn.SetReadLimit(c.options.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(c.options.ReadTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(c.options.ReadTimeout))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ctx.Done():
			return
		default:
			wsType, rawMessage, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.logger.Error("websocket unexpected close error", "error", err)
				} else {
					c.logger.Info("websocket connection closed", "error", err)
				}
				return
			}

			if wsType != websocket.TextMessage && wsType != websocket.BinaryMessage {
				continue
			}

			var message domain.Message
			if err := json.Unmarshal(rawMessage, &message); err != nil {
				c.logger.Error("failed to unmarshal message", "error", err)
				continue
			}

			c.logger.Info("received message", "type", message.Type, "id", message.ID)

			res, err := c.router.Handle(ctx, &message)
			if err != nil {
				c.logger.Error("message handler error", "error", err, "message_type", message.Type)
				continue
			}
			if res != nil {
				responseData, err := json.Marshal(res)
				if err != nil {
					c.logger.Error("failed to marshal response", "error", err)
					continue
				}

				if err := c.Send(ctx, responseData); err != nil {
					c.logger.Error("Failed to send response", "error", err)
					continue
				}
			}
		}
	}
}

func (c *Server) writePump(ctx context.Context, conn *websocket.Conn) {
	defer func() {
		c.logger.Info("server write pump stopped")
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ctx.Done():
			return
		case message, ok := <-c.sendChan:
			conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))

			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.logger.Error("websocket write error", "error", err)
				return
			}

			// Drain any queued messages
			n := len(c.sendChan)
			for range n {
				select {
				case msg := <-c.sendChan:
					if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
						c.logger.Error("websocket write error", "error", err)
						return
					}
				default:
				}
			}
		}
	}
}
