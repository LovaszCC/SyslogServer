package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"syslog-server/config"
	"syslog-server/server"
	"syslog-server/storage"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("Connecting to database at %s:%s/%s...", cfg.DBHost, cfg.DBPort, cfg.DBName)

	store, err := storage.New(ctx, cfg.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer store.Close()

	if err := store.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}
	log.Println("Database schema initialized")

	srv := server.New(cfg.SyslogPort, cfg.Protocol, cfg.ProxyProtocol, store)

	if err := srv.Start(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
