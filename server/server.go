package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	proxyproto "github.com/pires/go-proxyproto"

	"syslog-server/parser"
	"syslog-server/storage"
)

type Server struct {
	port          string
	proxyProtocol bool
	storage       *storage.Storage
	listener      net.Listener
	wg            sync.WaitGroup
}

func New(port string, proxyProtocol bool, store *storage.Storage) *Server {
	return &Server{
		port:          port,
		proxyProtocol: proxyProtocol,
		storage:       store,
	}
}

func (s *Server) Start(ctx context.Context) error {
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
	s.listener = listener

	go func() {
		<-ctx.Done()
		log.Println("Shutting down syslog server...")
		s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
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

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	sourceIP := remoteIP(conn.RemoteAddr())

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

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
	if s.listener != nil {
		s.listener.Close()
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
