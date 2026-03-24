package hsm

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// HSMConnection wraps a single TCP/TLS connection to a PayShield HSM.
type HSMConnection struct {
	conn       net.Conn
	headerLen  int
	mu         sync.Mutex
	alive      bool
	createdAt  time.Time
	lastUsedAt time.Time
}

// Connect establishes a connection to an HSM endpoint.
func Connect(address string, port int, protocol string, headerLen int, timeout time.Duration) (*HSMConnection, error) {
	addr := fmt.Sprintf("%s:%d", address, port)
	dialer := &net.Dialer{Timeout: timeout}

	var conn net.Conn
	var err error

	switch protocol {
	case "TLS":
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // HSMs typically use self-signed certs on private networks
		})
	default: // TCP, UDP
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("HSM connect to %s (%s): %w", addr, protocol, err)
	}

	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetKeepAlive(true)                  //nolint:errcheck
		tc.SetKeepAlivePeriod(30 * time.Second) //nolint:errcheck
	}

	return &HSMConnection{
		conn:       conn,
		headerLen:  headerLen,
		alive:      true,
		createdAt:  time.Now(),
		lastUsedAt: time.Now(),
	}, nil
}

// SendCommand sends a host command and reads the response.
// This is the core request/response exchange with the HSM.
func (c *HSMConnection) SendCommand(commandCode string, data []byte, timeout time.Duration) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.alive {
		return nil, fmt.Errorf("connection is closed")
	}

	msg := EncodeCommand(c.headerLen, commandCode, data)

	// Set write deadline
	if err := c.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		c.alive = false
		return nil, fmt.Errorf("set write deadline: %w", err)
	}

	if _, err := c.conn.Write(msg); err != nil {
		c.alive = false
		return nil, fmt.Errorf("write command %s: %w", commandCode, err)
	}

	// Set read deadline
	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		c.alive = false
		return nil, fmt.Errorf("set read deadline: %w", err)
	}

	// Read response header to get message length
	header := make([]byte, c.headerLen)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		c.alive = false
		return nil, fmt.Errorf("read response header: %w", err)
	}

	bodyLen := ReadMessageLength(header, c.headerLen)
	if bodyLen <= 0 || bodyLen > 65536 {
		c.alive = false
		return nil, fmt.Errorf("invalid response length: %d", bodyLen)
	}

	// Read response body
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(c.conn, body); err != nil {
		c.alive = false
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Return full frame (header + body) for DecodeResponse
	response := make([]byte, c.headerLen+bodyLen)
	copy(response, header)
	copy(response[c.headerLen:], body)

	c.lastUsedAt = time.Now()
	return response, nil
}

// Ping sends an NC (diagnostics) command to verify the connection is alive.
func (c *HSMConnection) Ping(timeout time.Duration) error {
	resp, err := c.SendCommand(CmdNC, nil, timeout)
	if err != nil {
		return err
	}
	_, respCode, _, err := DecodeResponse(c.headerLen, resp)
	if err != nil {
		return err
	}
	return ErrorForCode(respCode)
}

// IsAlive returns whether the connection is still considered alive.
func (c *HSMConnection) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.alive
}

// Close closes the underlying connection.
func (c *HSMConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.alive = false
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// LastUsed returns the time the connection was last used.
func (c *HSMConnection) LastUsed() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastUsedAt
}
