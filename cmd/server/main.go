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
	defer client.Close()

	go func() {
		if err := client.Read(); err != nil {
			log.Println("read:", err)
		}
	}()

	if err := client.Write(); err != nil {
		log.Println("write:", err)
	}
}
