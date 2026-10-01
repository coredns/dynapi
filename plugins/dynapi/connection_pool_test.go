package dynapi

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestAcquire(t *testing.T) {
	t.Parallel()
	t.Run("bounded and cancellable", checkAcquireLimit)

	for _, test := range []struct {
		name       string
		maxQueries int
		age        time.Duration
	}{
		{name: "expired idle connection", maxQueries: -1, age: 2 * time.Second},
		{name: "query limit", maxQueries: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			checkAcquireRetirement(t, test.maxQueries, test.age)
		})
	}
}

func TestRelease(t *testing.T) {
	t.Parallel()

	pool := newTestConnectionPool(t, 1)

	connection, err := pool.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	connection.reusable = true
	pool.release(connection)

	next, err := pool.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if next != connection || next.reusable {
		t.Fatal("connection reuse did not reset its state")
	}

	pool.release(next)

	if deadlineErr := next.SetDeadline(time.Now()); !errors.Is(deadlineErr, net.ErrClosed) {
		t.Fatalf("failed connection remains open: %v", deadlineErr)
	}

	replacement, err := pool.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if replacement == connection {
		t.Fatal("failed connection was reused")
	}

	pool.release(replacement)
}

func TestClose(t *testing.T) {
	t.Parallel()

	pool := newTestConnectionPool(t, 2)

	idle, err := pool.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if closeErr := idle.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}

	if deadlineErr := idle.SetDeadline(time.Time{}); deadlineErr != nil {
		t.Fatalf("transfer closed a pooled socket: %v", deadlineErr)
	}

	borrowed, err := pool.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	idle.reusable = true
	pool.release(idle)
	pool.close()
	pool.close()

	for _, connection := range []*pooledConnection{idle, borrowed} {
		if err := connection.SetDeadline(time.Now()); !errors.Is(err, net.ErrClosed) {
			t.Fatalf("shutdown left a socket open: %v", err)
		}
	}

	if _, err := pool.acquire(t.Context()); !errors.Is(err, errPoolClosed) {
		t.Fatalf("closed acquire error=%v", err)
	}

	pool.release(borrowed)
}

func checkAcquireLimit(t *testing.T) {
	t.Helper()
	t.Parallel()

	pool := newTestConnectionPool(t, 1)

	connection, err := pool.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := pool.acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("saturated acquire error=%v", err)
	}

	if len(pool.connections) != 1 {
		t.Fatal("pool exceeded its limit")
	}

	pool.release(connection)
}

func checkAcquireRetirement(t *testing.T, maxQueries int, age time.Duration) {
	t.Helper()

	pool := newTestConnectionPool(t, 1)

	pool.maxQueries = maxQueries

	connection, err := pool.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	connection.reusable = true
	pool.release(connection)

	connection.lastUsed = connection.lastUsed.Add(-age)

	next, err := pool.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if next == connection {
		t.Fatal("retired connection was reused")
	}

	if err := connection.SetDeadline(time.Now()); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("retired connection remains open: %v", err)
	}

	pool.release(next)
}

func newTestConnectionPool(t *testing.T, limit int) *connectionPool {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	pool := newConnectionPool(
		&dns.Client{Net: "tcp"},
		listener.Addr().String(),
		limit,
		time.Second,
		-1,
	)
	done := make(chan struct{})

	go func() {
		defer close(done)

		var peers []net.Conn

		defer func() {
			for _, peer := range peers {
				_ = peer.Close()
			}
		}()

		for {
			peer, err := listener.Accept()
			if err != nil {
				return
			}

			peers = append(peers, peer)
		}
	}()

	t.Cleanup(func() {
		pool.close()

		_ = listener.Close()

		<-done
	})

	return pool
}
