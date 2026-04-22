package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"syslog-server/parser"
)

type Storage struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Storage{pool: pool}, nil
}

func (s *Storage) Init(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS logs (
			id          BIGSERIAL PRIMARY KEY,
			received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			timestamp   TIMESTAMPTZ,
			hostname    TEXT,
			app_name    TEXT,
			facility    INT,
			severity    INT,
			message     TEXT,
			source_ip   TEXT,
			raw         TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_logs_received_at ON logs (received_at);
		CREATE INDEX IF NOT EXISTS idx_logs_hostname ON logs (hostname);
		CREATE INDEX IF NOT EXISTS idx_logs_severity ON logs (severity);
	`
	_, err := s.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}

func (s *Storage) Insert(ctx context.Context, msg *parser.SyslogMessage, sourceIP string) error {
	query := `
		INSERT INTO logs (timestamp, hostname, app_name, facility, severity, message, source_ip, raw)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	var ts interface{}
	if !msg.Timestamp.IsZero() {
		ts = msg.Timestamp
	}

	_, err := s.pool.Exec(ctx, query,
		ts,
		msg.Hostname,
		msg.AppName,
		msg.Facility,
		msg.Severity,
		msg.Message,
		sourceIP,
		msg.Raw,
	)
	if err != nil {
		return fmt.Errorf("insert log: %w", err)
	}
	return nil
}

func (s *Storage) Close() {
	s.pool.Close()
}
