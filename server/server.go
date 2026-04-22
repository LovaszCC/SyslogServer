package server

import (
	"context"
	"fmt"
	"log"
	"net"

	"syslog-server/parser"
	"syslog-server/storage"
)

type Server struct {
	port    string
	storage *storage.Storage
	conn    *net.UDPConn
}

func New(port string, store *storage.Storage) *Server {
	return &Server{
		port:    port,
		storage: store,
	}
}

func (s *Server) Start(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", ":"+s.port)
	if err != nil {
		return fmt.Errorf("resolve address: %w", err)
	}

	s.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}

	log.Printf("Syslog server listening on UDP port %s", s.port)

	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down syslog server...")
			return s.conn.Close()
		default:
		}

		n, remoteAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("Read error: %v", err)
			continue
		}

		raw := string(buf[:n])
		sourceIP := remoteAddr.IP.String()

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
}

func (s *Server) Stop() {
	if s.conn != nil {
		s.conn.Close()
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
