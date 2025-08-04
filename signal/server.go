package signal

import (
	"context"
	"encoding/json"
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
	wg       sync.WaitGroup
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
		cancel:   cancel,
		logger:   logger,
		options:  options,
		sendChan: make(chan []byte, 256),
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

	c.wg.Wait()

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
	defer conn.Close()

	ctx := conic.WithConnection(r.Context(), conn)

	c.wg.Add(2)
	go c.readPump(ctx, conn)
	go c.writePump(conn)
}

func (c *Server) readPump(ctx context.Context, conn *websocket.Conn) {
	defer c.wg.Done()
	defer func() {
		c.logger.Debug("read pump stopped")
		c.Close()
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

				c.mutex.RLock()
				if !c.closed {
					c.sendChan <- responseData
				}
				c.mutex.RUnlock()
			}
		}
	}
}

func (c *Server) writePump(conn *websocket.Conn) {
	defer c.wg.Done()
	defer func() {
		c.logger.Debug("write pump stopped")
	}()

	for {
		select {
		case <-c.ctx.Done():
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
