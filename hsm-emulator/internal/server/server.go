package server

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/your-org/hsm-emulator/internal/config"
	"github.com/your-org/hsm-emulator/internal/processor"
)

// Server is the TCP host-command server that mimics a PayShield 10K.
type Server struct {
	cfg        *config.Config
	proc       *processor.Processor
	listener   net.Listener
	wg         sync.WaitGroup
	connCount  atomic.Int32
	totalCmds  atomic.Int64
	startTime  time.Time
	shutdownCh chan struct{}
}

func New(cfg *config.Config, proc *processor.Processor) *Server {
	return &Server{
		cfg:        cfg,
		proc:       proc,
		shutdownCh: make(chan struct{}),
	}
}

// Start begins listening for host commands on the configured TCP port.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.HostAddress, s.cfg.HostPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	s.listener = ln
	s.startTime = time.Now()
	log.Printf("[HSM] Host interface listening on %s (header=%d bytes)", addr, s.cfg.HeaderLength)

	go s.acceptLoop()
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdownCh:
				return
			default:
				log.Printf("[HSM] Accept error: %v", err)
				continue
			}
		}

		if int(s.connCount.Load()) >= s.cfg.MaxConnections {
			log.Printf("[HSM] Max connections reached (%d), rejecting", s.cfg.MaxConnections)
			conn.Close()
			continue
		}

		s.connCount.Add(1)
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		s.connCount.Add(-1)
		s.wg.Done()
	}()

	remote := conn.RemoteAddr().String()
	log.Printf("[HSM] Connection from %s (active=%d)", remote, s.connCount.Load())

	for {
		select {
		case <-s.shutdownCh:
			return
		default:
		}

		// Set read timeout
		timeout := time.Duration(s.cfg.CommandTimeout) * time.Second
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		conn.SetReadDeadline(time.Now().Add(timeout))

		// Read length prefix
		headerBuf := make([]byte, s.cfg.HeaderLength)
		if _, err := io.ReadFull(conn, headerBuf); err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // idle timeout, keep connection alive
			}
			if err != io.EOF {
				log.Printf("[HSM] Read header from %s: %v", remote, err)
			}
			return
		}

		bodyLen := readLength(headerBuf, s.cfg.HeaderLength)
		if bodyLen <= 0 || bodyLen > 65536 {
			log.Printf("[HSM] Invalid message length %d from %s", bodyLen, remote)
			return
		}

		// Read message body
		body := make([]byte, bodyLen)
		conn.SetReadDeadline(time.Now().Add(timeout))
		if _, err := io.ReadFull(conn, body); err != nil {
			log.Printf("[HSM] Read body from %s: %v", remote, err)
			return
		}

		// Parse command code (first 2 bytes of body)
		if len(body) < 2 {
			log.Printf("[HSM] Message too short from %s", remote)
			continue
		}
		cmdCode := string(body[:2])
		cmdData := body[2:]

		// Process command
		start := time.Now()
		respCode, errCode, respData := s.proc.Execute(cmdCode, cmdData)
		elapsed := time.Since(start)
		s.totalCmds.Add(1)

		if s.cfg.AuditEnabled {
			log.Printf("[HSM] %s → %s err=%s (%v) from %s", cmdCode, respCode, errCode, elapsed, remote)
		}

		// Build and send response
		respBody := []byte(respCode + errCode)
		if len(respData) > 0 {
			respBody = append(respBody, respData...)
		}
		frame := encodeFrame(respBody, s.cfg.HeaderLength)

		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write(frame); err != nil {
			log.Printf("[HSM] Write to %s: %v", remote, err)
			return
		}
	}
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	close(s.shutdownCh)
	if s.listener != nil {
		s.listener.Close()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) Stats() (conns int, cmds int64, uptime time.Duration) {
	return int(s.connCount.Load()), s.totalCmds.Load(), time.Since(s.startTime)
}

func readLength(header []byte, headerLen int) int {
	switch headerLen {
	case 2:
		return int(binary.BigEndian.Uint16(header))
	case 4:
		return int(binary.BigEndian.Uint32(header))
	default:
		return int(binary.BigEndian.Uint16(header))
	}
}

func encodeFrame(body []byte, headerLen int) []byte {
	frame := make([]byte, headerLen+len(body))
	switch headerLen {
	case 2:
		binary.BigEndian.PutUint16(frame[:2], uint16(len(body)))
	case 4:
		binary.BigEndian.PutUint32(frame[:4], uint32(len(body)))
	default:
		binary.BigEndian.PutUint16(frame[:2], uint16(len(body)))
	}
	copy(frame[headerLen:], body)
	return frame
}
