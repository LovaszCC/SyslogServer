package server

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	proxyproto "github.com/pires/go-proxyproto"

	"syslog-server/parser"
	"syslog-server/storage"
)

type Server struct {
	port          string
	protocol      string
	proxyProtocol bool
	storage       *storage.Storage
	tcpListener   net.Listener
	udpConn       *net.UDPConn
	wg            sync.WaitGroup
}

func New(port, protocol string, proxyProtocol bool, store *storage.Storage) *Server {
	return &Server{
		port:          port,
		protocol:      strings.ToLower(protocol),
		proxyProtocol: proxyProtocol,
		storage:       store,
	}
}

func (s *Server) Start(ctx context.Context) error {
	switch s.protocol {
	case "tcp":
		return s.startTCP(ctx)
	case "udp":
		return s.startUDP(ctx)
	case "both":
		errCh := make(chan error, 2)
		go func() { errCh <- s.startTCP(ctx) }()
		go func() { errCh <- s.startUDP(ctx) }()
		var firstErr error
		for i := 0; i < 2; i++ {
			if err := <-errCh; err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	default:
		return fmt.Errorf("unsupported protocol: %s (want tcp|udp|both)", s.protocol)
	}
}

func (s *Server) startTCP(ctx context.Context) error {
	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", ":"+s.port)
	if err != nil {
		return fmt.Errorf("listen tcp: %w", err)
	}

	if s.proxyProtocol {
		listener = &proxyproto.Listener{
			Listener:          listener,
			ReadHeaderTimeout: 5 * time.Second,
			Policy: func(upstream net.Addr) (proxyproto.Policy, error) {
				return proxyproto.REQUIRE, nil
			},
		}
		log.Printf("Syslog server listening on TCP port %s (PROXY protocol enabled)", s.port)
	} else {
		log.Printf("Syslog server listening on TCP port %s", s.port)
	}
	s.tcpListener = listener

	go func() {
		<-ctx.Done()
		log.Println("Shutting down TCP syslog server...")
		s.tcpListener.Close()
	}()

	for {
		conn, err := s.tcpListener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				s.wg.Wait()
				return nil
			}
			log.Printf("Accept error: %v", err)
			continue
		}

		s.wg.Add(1)
		go s.handleConn(ctx, conn)
	}
}

func (s *Server) startUDP(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", ":"+s.port)
	if err != nil {
		return fmt.Errorf("resolve udp: %w", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}
	s.udpConn = conn
	log.Printf("Syslog server listening on UDP port %s", s.port)

	go func() {
		<-ctx.Done()
		log.Println("Shutting down UDP syslog server...")
		s.udpConn.Close()
	}()

	buf := make([]byte, 64*1024)
	for {
		n, src, err := s.udpConn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			log.Printf("UDP read error: %v", err)
			continue
		}
		s.handleDatagram(ctx, buf[:n], src)
	}
}

func (s *Server) handleDatagram(ctx context.Context, data []byte, src *net.UDPAddr) {
	sourceIP := ""
	if src != nil {
		sourceIP = src.IP.String()
	}

	raw := strings.TrimRight(string(data), "\r\n\x00")
	if raw == "" {
		return
	}

	msg, err := parser.Parse(raw)
	if err != nil {
		log.Printf("Parse error from %s: %v (raw: %q)", sourceIP, err, raw)
		return
	}

	if err := s.storage.Insert(ctx, msg, sourceIP); err != nil {
		log.Printf("Storage error: %v", err)
		return
	}

	log.Printf("Stored log from %s [%s] %s: %s",
		sourceIP, msg.Hostname, msg.AppName, truncate(msg.Message, 100))
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	sourceIP := remoteIP(conn.RemoteAddr())

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(splitSyslog)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		raw := scanner.Text()
		if raw == "" {
			continue
		}

		msg, err := parser.Parse(raw)
		if err != nil {
			log.Printf("Parse error from %s: %v (raw: %q)", sourceIP, err, raw)
			continue
		}

		if err := s.storage.Insert(ctx, msg, sourceIP); err != nil {
			log.Printf("Storage error: %v", err)
			continue
		}

		log.Printf("Stored log from %s [%s] %s: %s",
			sourceIP, msg.Hostname, msg.AppName, truncate(msg.Message, 100))
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		log.Printf("Read error from %s: %v", sourceIP, err)
	}
}

func (s *Server) Stop() {
	if s.tcpListener != nil {
		s.tcpListener.Close()
	}
	if s.udpConn != nil {
		s.udpConn.Close()
	}
	s.wg.Wait()
}

func remoteIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	if tcp, ok := addr.(*net.TCPAddr); ok {
		return tcp.IP.String()
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// splitSyslog handles RFC6587 octet-counting, newline, and null-byte framing.
func splitSyslog(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if data[0] >= '0' && data[0] <= '9' {
		if sp := bytes.IndexByte(data, ' '); sp > 0 {
			if n, err := strconv.Atoi(string(data[:sp])); err == nil && n > 0 {
				if len(data) >= sp+1+n {
					return sp + 1 + n, data[sp+1 : sp+1+n], nil
				}
				if atEOF {
					return len(data), data, nil
				}
				return 0, nil, nil
			}
		}
	}

	for i, b := range data {
		if b == '\n' || b == 0 {
			return i + 1, dropCR(data[:i]), nil
		}
	}

	if atEOF {
		return len(data), dropCR(data), nil
	}
	return 0, nil, nil
}

func dropCR(d []byte) []byte {
	if len(d) > 0 && d[len(d)-1] == '\r' {
		return d[:len(d)-1]
	}
	return d
}
