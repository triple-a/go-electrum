package main

import (
	"context"
	"log"
	"time"

	"github.com/triple-a/go-electrum/electrum"
)

func TestServer() {
	client, err := electrum.NewClientTCP(
		context.Background(),
		"bch.imaginary.cash:50001",
	)

	if err != nil {
		log.Fatal(err)
	}

	serverVer, protocolVer, err := client.ServerVersion(
		context.Background(),
		"",
		"1.4.2",
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Server version: %s [Protocol %s]", serverVer, protocolVer)

	go func() {
		for {
			if err := client.Ping(context.Background()); err != nil {
				log.Fatal(err)
			}
			time.Sleep(60 * time.Second)
		}
	}()
}
