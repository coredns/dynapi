package dynapi

import (
	"net"
	"time"
)

type pooledConnection struct {
	net.Conn

	lastUsed time.Time
	queries  int
	reusable bool
}

// Close leaves ownership with the pool because dns.Transfer closes its input.
// Cancellation and disposal close the underlying Conn directly.
func (*pooledConnection) Close() error { return nil }
