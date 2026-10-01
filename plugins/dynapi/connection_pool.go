package dynapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type connectionPool struct {
	client      *dns.Client
	connections map[*pooledConnection]struct{}
	slots       chan struct{}
	done        chan struct{}
	address     string
	idle        []*pooledConnection
	idleTimeout time.Duration
	maxQueries  int
	mutex       sync.Mutex
	closed      bool
}

func newConnectionPool(client *dns.Client, address string, limit int,
	idleTimeout time.Duration, maxQueries int,
) *connectionPool {
	return &connectionPool{
		client: client, address: address, idleTimeout: idleTimeout, maxQueries: maxQueries,
		connections: make(map[*pooledConnection]struct{}),
		idle:        nil, mutex: sync.Mutex{}, closed: false,
		slots: make(chan struct{}, limit), done: make(chan struct{}),
	}
}

func (connectionPool *connectionPool) acquire(ctx context.Context) (*pooledConnection, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for DNS connection: %w", ctx.Err())
	case <-connectionPool.done:
		return nil, errPoolClosed
	case connectionPool.slots <- struct{}{}:
	}

	connectionPool.mutex.Lock()

	if connectionPool.closed {
		connectionPool.mutex.Unlock()
		<-connectionPool.slots

		return nil, errPoolClosed
	}

	for len(connectionPool.idle) > 0 {
		index := len(connectionPool.idle) - 1
		connection := connectionPool.idle[index]

		connectionPool.idle[index] = nil
		connectionPool.idle = connectionPool.idle[:index]

		if time.Since(connection.lastUsed) < connectionPool.idleTimeout &&
			(connectionPool.maxQueries < 0 || connection.queries < connectionPool.maxQueries) {

			connection.reusable = false
			connectionPool.mutex.Unlock()

			return connection, nil
		}

		delete(connectionPool.connections, connection)

		_ = connection.Conn.Close()
	}

	connectionPool.mutex.Unlock()

	connection, err := connectionPool.dial(ctx)
	if err != nil {
		<-connectionPool.slots
	}

	return connection, err
}

func (connectionPool *connectionPool) dial(ctx context.Context) (*pooledConnection, error) {
	conn, err := connectionPool.client.DialContext(ctx, connectionPool.address)
	if err != nil {
		return nil, fmt.Errorf("dial DNS backend: %w", err)
	}

	connection := &pooledConnection{
		Conn:     conn.Conn,
		lastUsed: time.Time{},
		queries:  0,
		reusable: false,
	}

	connectionPool.mutex.Lock()
	defer connectionPool.mutex.Unlock()

	if connectionPool.closed {
		_ = connection.Conn.Close()

		return nil, errPoolClosed
	}

	connectionPool.connections[connection] = struct{}{}

	return connection, nil
}

func (connectionPool *connectionPool) release(connection *pooledConnection) {
	connectionPool.mutex.Lock()
	defer connectionPool.mutex.Unlock()
	defer func() { <-connectionPool.slots }()

	if !connection.reusable || connectionPool.closed {
		delete(connectionPool.connections, connection)

		_ = connection.Conn.Close()

		return
	}

	connection.queries++

	connection.lastUsed = time.Now()
	connectionPool.idle = append(connectionPool.idle, connection)
}

func (connectionPool *connectionPool) close() {
	connectionPool.mutex.Lock()
	defer connectionPool.mutex.Unlock()

	if connectionPool.closed {
		return
	}

	connectionPool.closed = true
	close(connectionPool.done)

	for connection := range connectionPool.connections {
		_ = connection.Conn.Close()
	}

	clear(connectionPool.connections)

	connectionPool.idle = nil
}
