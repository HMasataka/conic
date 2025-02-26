package conic

import (
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func NewClient(u url.URL) (*Client, error) {
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn: conn,
		done: make(chan struct{}),
	}, nil
}

type Client struct {
	conn *websocket.Conn
	done chan struct{}
}

func (c *Client) Read() error {
	defer close(c.done)

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return err
		}

		log.Printf("recv: %s", message)
	}
}

func (c *Client) Write(message string) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return nil
		case t := <-ticker.C:
			err := c.conn.WriteMessage(websocket.TextMessage, []byte(t.String()))
			if err != nil {
				return err
			}
		case <-interrupt:
			log.Println("interrupt")

			if err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
				return err
			}

			c.waitCloseConnection()

			return nil
		}
	}
}

func (c *Client) waitCloseConnection() {
	select {
	case <-c.done:
	case <-time.After(time.Second):
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Error(err error) {
	log.Println("err:", err)
}
