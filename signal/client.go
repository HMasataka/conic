package signal

import (
	"context"

	"github.com/gorilla/websocket"
)

type client struct {
	id   string
	conn *websocket.Conn
}

func NewClient(id string, conn *websocket.Conn) *client {
	return &client{
		id:   id,
		conn: conn,
	}
}

func (c client) ID() string {
	return c.id
}

func (c *client) Send(_ context.Context, message []byte) error {
	return c.conn.WriteMessage(websocket.TextMessage, message)
}

func (c *client) Close() error {
	return c.conn.Close()
}
