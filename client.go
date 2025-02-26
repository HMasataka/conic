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

func (c *Client) Read() {
	defer close(c.done)

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}

		log.Printf("recv: %s", message)
	}
}

func (c *Client) Write() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case t := <-ticker.C:
			err := c.conn.WriteMessage(websocket.TextMessage, []byte(t.String()))
			if err != nil {
				log.Println("write:", err)
				return
			}
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}

			c.waitCloseConnection()

			return
		}
	}
}

func (c *Client) waitCloseConnection() {
	select {
	case <-c.done:
	case <-time.After(time.Second):
	}
}

func (c *Client) Teardown() error {
	return c.conn.Close()
}
