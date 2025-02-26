package conic

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

func Handle(fn func(conn *websocket.Conn)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		defer conn.Close()

		fn(conn)
	}
}
