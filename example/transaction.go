package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"

	"github.com/triple-a/go-electrum/electrum"
)

func main() {

	txid := "66555dfb0f823623caae5ac27dc1458a78a1cfe36ab85792a05583453446d9e2"

	ctx := context.Background()
	client, err := electrum.NewClientSSL(
		ctx,
		"reports-electrumx1.triple-a.xyz:50002",
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)
	if err != nil {
		panic(err)
	}
	client.ServerVersion(ctx, "2.7.11", "1.4.2")

	// Get transaction
	tx, err := client.GetTransaction(ctx, txid)
	if err != nil {
		panic(err)
	}

	x, _ := json.Marshal(tx)
	fmt.Printf("%s", x)
}
