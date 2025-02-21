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

	r.Get("/ws", conic.Handle)

	if err := http.ListenAndServe(":3000", r); err != nil {
		log.Println(err)
	}
}
