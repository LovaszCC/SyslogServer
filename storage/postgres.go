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
			facility    TEXT,
			severity    TEXT,
			message     TEXT,
			source_ip   TEXT,
			raw         TEXT NOT NULL
		);
		ALTER TABLE logs DROP COLUMN IF EXISTS source_hostname;
		DO $$ BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.columns
				WHERE table_name='logs' AND column_name='facility' AND data_type='integer') THEN
				ALTER TABLE logs ALTER COLUMN facility TYPE TEXT USING facility::text;
			END IF;
			IF EXISTS (SELECT 1 FROM information_schema.columns
				WHERE table_name='logs' AND column_name='severity' AND data_type='integer') THEN
				ALTER TABLE logs ALTER COLUMN severity TYPE TEXT USING severity::text;
			END IF;
		END $$;
		CREATE INDEX IF NOT EXISTS idx_logs_received_at ON logs (received_at);
		CREATE INDEX IF NOT EXISTS idx_logs_hostname ON logs (hostname);
		CREATE INDEX IF NOT EXISTS idx_logs_severity ON logs (severity);
		CREATE INDEX IF NOT EXISTS idx_logs_facility ON logs (facility);

		CREATE TABLE IF NOT EXISTS logs_archive (
			id           BIGSERIAL PRIMARY KEY,
			archived_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			received_at  TIMESTAMPTZ NOT NULL,
			timestamp    TIMESTAMPTZ,
			hostname     TEXT,
			app_name     TEXT,
			facility     TEXT,
			severity     TEXT,
			message      TEXT,
			source_ip    TEXT,
			raw          TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_logs_archive_received_at ON logs_archive (received_at);
		CREATE INDEX IF NOT EXISTS idx_logs_archive_archived_at ON logs_archive (archived_at);
		CREATE INDEX IF NOT EXISTS idx_logs_archive_hostname ON logs_archive (hostname);
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

// Archive moves every row from logs into logs_archive and truncates logs.
// Runs in a single transaction so a partial archive cannot leave the source
// table truncated. Returns the number of rows moved.
func (s *Storage) Archive(ctx context.Context) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin archive tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "LOCK TABLE logs IN EXCLUSIVE MODE"); err != nil {
		return 0, fmt.Errorf("lock logs: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO logs_archive
			(received_at, timestamp, hostname, app_name, facility, severity, message, source_ip, raw)
		SELECT received_at, timestamp, hostname, app_name, facility, severity, message, source_ip, raw
		FROM logs
	`)
	if err != nil {
		return 0, fmt.Errorf("copy logs to archive: %w", err)
	}
	moved := tag.RowsAffected()

	if _, err := tx.Exec(ctx, "TRUNCATE TABLE logs RESTART IDENTITY"); err != nil {
		return 0, fmt.Errorf("truncate logs: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit archive tx: %w", err)
	}
	return moved, nil
}

func (s *Storage) Close() {
	s.pool.Close()
}
