package main

import (
	"flag"
	"log"
	"net/url"

	"github.com/HMasataka/conic"
)

var addr = flag.String("addr", "localhost:3000", "http service address")

func main() {
	flag.Parse()
	log.SetFlags(0)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	client, err := conic.NewClient(u)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer client.Teardown()

	go client.Read()

	client.Write()
}
