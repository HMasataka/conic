package main

import (
	"log"
	"net/http"

	"github.com/HMasataka/conic/hub"
	"github.com/HMasataka/conic/signal"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	hub := hub.NewHub()
	go hub.Run()

	server := signal.NewServer(hub)

	r.Get("/ws", server.Serve)

	if err := http.ListenAndServe(":3000", r); err != nil {
		log.Println(err)
	}
}
