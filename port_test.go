//go:build integration

package main

import (
	"net"
	"testing"
)

func unusedPort(t *testing.T) int {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err := listener.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("expected a TCP listener")
	}

	return address.Port
}
