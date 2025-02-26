package main

import (
	"log"
	"net/http"

	"github.com/HMasataka/conic"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/ws", conic.Handle(func(conn *websocket.Conn) {
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}

			log.Printf("recv: %s", message)

			if err = conn.WriteMessage(mt, message); err != nil {
				log.Println("write:", err)
				break
			}
		}
	}))

	if err := http.ListenAndServe(":3000", r); err != nil {
		log.Println(err)
	}
}
