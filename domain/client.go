package domain

import (
	"context"
	"errors"

	"github.com/gorilla/websocket"
)

type client struct {
	id   string
	conn *websocket.Conn
}

func NewClient(id string, conn *websocket.Conn) Client {
	return &client{
		id:   id,
		conn: conn,
	}
}

func (c *client) ID() string {
	return c.id
}

func (c *client) Send(ctx context.Context, message []byte) error {
	if c.conn == nil {
		return errors.New("connection is closed")
	}

	err := c.conn.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		return err
	}

	return nil
}

func (c *client) Close() error {
	if c.conn == nil {
		return errors.New("connection is closed")
	}

	err := c.conn.Close()
	if err != nil {
		return err
	}

	c.conn = nil
	return nil
}

type Client interface {
	ID() string

	Send(ctx context.Context, message []byte) error

	Close() error
}
