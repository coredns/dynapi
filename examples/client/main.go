// Package main demonstrates authenticated record management with net/http.
// Run it against examples/Corefile with DYNAPI_TOKEN set in the environment.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	requestTimeout = 5 * time.Second
	exampleTimeout = 20 * time.Second
	exampleTTL     = 60
)

func main() {
	token := os.Getenv("DYNAPI_TOKEN")
	if token == "" {
		log.Fatal("DYNAPI_TOKEN is required")
	}

	client := &client{
		http:     &http.Client{Timeout: requestTimeout},
		endpoint: "http://127.0.0.1:8080/v1/zones/example.org/records/host.example.org/A",
		token:    token,
	}
	ctx, cancel := context.WithTimeout(context.Background(), exampleTimeout)
	err := run(ctx, client)

	cancel()

	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, client *client) error {
	// A missing set is expected on the first run. Messages are for people.
	// The stable code lets clients distinguish a missing set from a missing zone.
	_, err := client.request(ctx, http.MethodGet, nil)
	if err != nil {
		failure, ok := errors.AsType[*apiError](err)
		if !ok || failure.Code != "record_set_not_found" {
			return err
		}
	}

	// PUT replaces every A address for this name. It leaves AAAA records intact.
	desired := &recordSet{TTL: exampleTTL, Addresses: []string{"192.0.2.10", "192.0.2.11"}}

	stored, err := client.request(ctx, http.MethodPut, desired)
	if err != nil {
		return err
	}

	log.Printf("PUT: ttl=%d addresses=%v", stored.TTL, stored.Addresses)

	stored, err = client.request(ctx, http.MethodGet, nil)
	if err != nil {
		return err
	}

	log.Printf("GET: ttl=%d addresses=%v", stored.TTL, stored.Addresses)

	// DELETE succeeds even if the set is absent. A successful response has no body.
	if _, err := client.request(ctx, http.MethodDelete, nil); err != nil {
		return fmt.Errorf("delete example record set: %w", err)
	}

	log.Print("DELETE: record set removed")

	return nil
}
