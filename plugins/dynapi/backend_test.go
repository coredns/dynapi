package dynapi

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestRead(t *testing.T) {
	t.Parallel()

	backend, peer := newBlockedBackend(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)

	defer cancel()

	result := make(chan error, 1)

	go func() {
		_, err := backend.read(ctx, "host.example.org.", dns.TypeA)
		result <- err
	}()

	request, err := peer.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}

	if request.Question[0].Qtype != dns.TypeAXFR {
		t.Fatal("read did not request AXFR")
	}

	cancel()

	if err := <-result; err == nil {
		t.Fatal("canceled read succeeded")
	}

	if len(backend.pool.connections) != 0 || len(backend.pool.idle) != 0 {
		t.Fatal("canceled read returned a connection to the pool")
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"canceled", "lost response"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			checkFailedUpdate(t, mode)
		})
	}
}

func checkFailedUpdate(t *testing.T, mode string) {
	t.Helper()

	backend, peer := newBlockedBackend(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)

	defer cancel()

	result := make(chan error, 1)

	go func() { result <- backend.update(ctx, nil, nil) }()

	request, err := peer.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}

	if request.Opcode != dns.OpcodeUpdate {
		t.Fatal("write did not request UPDATE")
	}

	if mode == "canceled" {
		cancel()
	} else if closeErr := peer.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}

	if err := <-result; err == nil {
		t.Fatal("failed update succeeded")
	}

	if len(backend.pool.connections) != 0 || len(backend.pool.idle) != 0 {
		t.Fatal("failed update returned a connection to the pool")
	}
}

func newBlockedBackend(t *testing.T) (*backend, *dns.Conn) {
	t.Helper()

	const key = "update-key.example.org."

	client := &dns.Client{
		Net: "tcp", Timeout: time.Second,
		TsigSecret: map[string]string{key: "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4"},
	}
	pool := newConnectionPool(client, "127.0.0.1:0", 1, time.Second, -1)
	local, remote := net.Pipe()
	connection := &pooledConnection{Conn: local, lastUsed: time.Now()}

	pool.connections[connection] = struct{}{}
	pool.idle = append(pool.idle, connection)

	t.Cleanup(func() {
		pool.close()

		_ = remote.Close()
	})

	if err := remote.SetDeadline(time.Now().Add(requestTimeout)); err != nil {
		t.Fatal(err)
	}

	return &backend{
		client: client,
		pool:   pool,
		zone:   "example.org.",
		key:    key,
	}, &dns.Conn{
		Conn:       remote,
		TsigSecret: client.TsigSecret,
	}
}
