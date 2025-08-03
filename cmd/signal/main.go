package main

import (
	"context"
	"log"
	"net/http"

	"github.com/HMasataka/conic/hub"
	"github.com/HMasataka/conic/logging"
	"github.com/HMasataka/conic/signal"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	logger := logging.New(logging.Config{
		Level:  "info",
		Format: "text",
	})

	ctx := context.Background()

	hub := hub.New(logger)
	go hub.Start(ctx)

	router := signal.NewRouter(hub, logger)
	server := signal.NewServer(router, logger, signal.DefaultServerOptions())

	r.Get("/ws", server.Handle)

	if err := http.ListenAndServe(":3000", r); err != nil {
		log.Println(err)
	}
}
