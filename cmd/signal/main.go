package main

import (
	"log"
	"net/http"

	"github.com/HMasataka/conic"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	hub := conic.NewHub()
	go hub.Run()

	socket := conic.NewSocket(hub)

	r.Get("/ws", socket.Serve)

	if err := http.ListenAndServe(":3000", r); err != nil {
		log.Println(err)
	}
}
