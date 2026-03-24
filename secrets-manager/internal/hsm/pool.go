package hsm

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// ConnectionPool manages a pool of connections to a single HSM endpoint.
type ConnectionPool struct {
	endpoint HSMEndpoint
	conns    chan *HSMConnection
	mu       sync.Mutex
	active   atomic.Int32
	total    atomic.Int32
	draining atomic.Bool
	closed   atomic.Bool
	log      *zap.Logger
}

// NewPool creates a connection pool for the given HSM endpoint.
// It does not pre-connect; connections are created lazily on Acquire.
func NewPool(endpoint HSMEndpoint, log *zap.Logger) *ConnectionPool {
	maxConns := endpoint.MaxConnections
	if maxConns <= 0 {
		maxConns = 8
	}
	p := &ConnectionPool{
		endpoint: endpoint,
		conns:    make(chan *HSMConnection, maxConns),
		log:      log,
	}
	return p
}

// Acquire gets a connection from the pool, creating a new one if needed.
// Blocks if all connections are in use and the pool is at max capacity.
func (p *ConnectionPool) Acquire(ctx context.Context) (*HSMConnection, error) {
	if p.closed.Load() {
		return nil, fmt.Errorf("pool is closed")
	}
	if p.draining.Load() {
		return nil, fmt.Errorf("pool is draining, not accepting new requests")
	}

	// Try to get an existing idle connection
	select {
	case conn := <-p.conns:
		if conn.IsAlive() {
			p.active.Add(1)
			return conn, nil
		}
		// Connection is dead, discard and create new
		conn.Close() //nolint:errcheck
		p.total.Add(-1)
	default:
		// No idle connections available
	}

	// Create a new connection if under limit
	if int(p.total.Load()) < p.endpoint.MaxConnections {
		conn, err := p.dial()
		if err != nil {
			return nil, err
		}
		p.total.Add(1)
		p.active.Add(1)
		return conn, nil
	}

	// At capacity -- wait for a connection to be returned
	select {
	case conn := <-p.conns:
		if conn.IsAlive() {
			p.active.Add(1)
			return conn, nil
		}
		conn.Close() //nolint:errcheck
		p.total.Add(-1)
		// Try to create a replacement
		newConn, err := p.dial()
		if err != nil {
			return nil, err
		}
		p.total.Add(1)
		p.active.Add(1)
		return newConn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Release returns a connection to the pool.
func (p *ConnectionPool) Release(conn *HSMConnection) {
	p.active.Add(-1)
	if p.closed.Load() || !conn.IsAlive() {
		conn.Close() //nolint:errcheck
		p.total.Add(-1)
		return
	}
	select {
	case p.conns <- conn:
		// returned to pool
	default:
		// pool is full, discard
		conn.Close() //nolint:errcheck
		p.total.Add(-1)
	}
}

// HealthCheck sends an NC command on a temporary or pooled connection.
func (p *ConnectionPool) HealthCheck() error {
	if p.closed.Load() {
		return fmt.Errorf("pool is closed")
	}

	timeout := time.Duration(p.endpoint.ConnectionTimeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	// Try to get an idle connection for the health check
	var conn *HSMConnection
	select {
	case c := <-p.conns:
		if c.IsAlive() {
			conn = c
		} else {
			c.Close() //nolint:errcheck
			p.total.Add(-1)
		}
	default:
	}

	if conn == nil {
		// Create a temporary connection
		var err error
		conn, err = p.dial()
		if err != nil {
			return err
		}
		defer func() {
			conn.Close() //nolint:errcheck
		}()
		err = conn.Ping(timeout)
		return err
	}

	cmdTimeout := time.Duration(p.endpoint.HostCommandTimeout) * time.Second
	if cmdTimeout <= 0 {
		cmdTimeout = 15 * time.Second
	}

	err := conn.Ping(cmdTimeout)
	// Return the connection to the pool
	if err == nil {
		select {
		case p.conns <- conn:
		default:
			conn.Close() //nolint:errcheck
			p.total.Add(-1)
		}
	} else {
		conn.Close() //nolint:errcheck
		p.total.Add(-1)
	}
	return err
}

// Drain stops accepting new requests but allows in-flight to complete.
func (p *ConnectionPool) Drain() {
	p.draining.Store(true)
}

// Resume reverses a Drain.
func (p *ConnectionPool) Resume() {
	p.draining.Store(false)
}

// Close closes all connections in the pool.
func (p *ConnectionPool) Close() {
	p.closed.Store(true)
	close(p.conns)
	for conn := range p.conns {
		conn.Close() //nolint:errcheck
	}
}

// Stats returns current pool statistics.
func (p *ConnectionPool) Stats() PoolStats {
	active := int(p.active.Load())
	total := int(p.total.Load())
	return PoolStats{
		Active:   active,
		Idle:     total - active,
		Total:    total,
		Draining: p.draining.Load(),
	}
}

// Endpoint returns the endpoint configuration for this pool.
func (p *ConnectionPool) Endpoint() HSMEndpoint {
	return p.endpoint
}

func (p *ConnectionPool) dial() (*HSMConnection, error) {
	timeout := time.Duration(p.endpoint.ConnectionTimeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return Connect(p.endpoint.Address, p.endpoint.Port, p.endpoint.Protocol, p.endpoint.HeaderLength, timeout)
}
