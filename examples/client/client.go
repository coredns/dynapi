package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"log"
	"net/http"
)

type client struct {
	http     *http.Client
	endpoint string
	token    string
}

func (client *client) request(
	ctx context.Context,
	method string,
	set *recordSet,
) (recordSet, error) {
	var body []byte

	if set != nil {
		encoded, err := json.Marshal(set)
		if err != nil {
			return recordSet{}, fmt.Errorf("encode record set: %w", err)
		}

		body = encoded
	}

	request, err := http.NewRequestWithContext(ctx, method, client.endpoint, bytes.NewReader(body))
	if err != nil {
		return recordSet{}, fmt.Errorf("create HTTP request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+client.token)

	if set != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	// Do not retry failed writes automatically. A lost response can follow a commit.
	response, err := client.http.Do(request)
	if err != nil {
		return recordSet{}, fmt.Errorf("send HTTP request; a write may have committed: %w", err)
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Printf("close response body: %v", err)
		}
	}()

	return client.decode(response)
}

func (*client) decode(response *http.Response) (recordSet, error) {
	if response.StatusCode == http.StatusNoContent {
		return recordSet{}, nil
	}

	if response.StatusCode != http.StatusOK {
		failure := &apiError{Code: "", Message: "", Status: response.StatusCode}
		if err := json.UnmarshalRead(response.Body, failure); err != nil {
			return recordSet{}, fmt.Errorf("decode HTTP %d error: %w", response.StatusCode, err)
		}

		return recordSet{}, failure
	}

	var set recordSet

	if err := json.UnmarshalRead(response.Body, &set); err != nil {
		return recordSet{}, fmt.Errorf("decode record set: %w", err)
	}

	return set, nil
}
