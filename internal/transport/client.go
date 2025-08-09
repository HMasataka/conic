package transport

import (
	"context"

	"github.com/HMasataka/conic/internal/protocol"
	"github.com/HMasataka/conic/logging"
	ws "github.com/gorilla/websocket"
	"github.com/rs/xid"
)

type ClientOptions struct {
	ID string
	ConnectionOptions
}

func DefaultClientOptions(id string) ClientOptions {
	return ClientOptions{
		ID:                id,
		ConnectionOptions: DefaultConnectionOptions(),
	}
}

type Client struct {
	id         string
	connection *Connection
	logger     *logging.Logger
}

func NewClient(conn *ws.Conn, router *protocol.Router, logger *logging.Logger, options ClientOptions) *Client {
	id := options.ID
	if id == "" {
		id = xid.New().String()
	}

	clientLogger := logger.WithFields(map[string]any{"client_id": id})
	connection := NewConnection(conn, router, clientLogger, options.ConnectionOptions)

	return &Client{
		id:         id,
		connection: connection,
		logger:     clientLogger,
	}
}

func (c *Client) ID() string {
	return c.id
}

func (c *Client) Send(ctx context.Context, message []byte) error {
	return c.connection.Send(ctx, message)
}

func (c *Client) Close() error {
	return c.connection.Close()
}

func (c *Client) Context() context.Context {
	return c.connection.Context()
}

func (c *Client) Start(ctx context.Context) {
	c.connection.Start(ctx)
}
