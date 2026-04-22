# Syslog Server

A Go-based syslog server that accepts UDP syslog messages (RFC 3164 and RFC 5424) and stores them in PostgreSQL. Runs in Docker with full environment-based configuration for multi-instance deployments.

## Configuration

All settings are configured via environment variables:

| Variable      | Description              | Default     |
|---------------|--------------------------|-------------|
| `SYSLOG_PORT` | UDP port to listen on    | `514`       |
| `DB_HOST`     | PostgreSQL host          | `localhost` |
| `DB_PORT`     | PostgreSQL port          | `5432`      |
| `DB_USER`     | PostgreSQL user          | `syslog`    |
| `DB_PASSWORD` | PostgreSQL password      | `syslog`    |
| `DB_NAME`     | PostgreSQL database name | `syslog`    |
| `DB_SSLMODE`  | PostgreSQL SSL mode      | `disable`   |

## Running

1. Create your environment file:

```bash
cp .env.example .env
```

2. Edit `.env` as needed, then start the containers:

```bash
docker compose up --build -d
```

3. Check logs to verify both services are running:

```bash
docker compose logs -f
```

## Running Multiple Instances

Create separate `.env` files for each device:

```bash
# .env.device-a
SYSLOG_PORT=1514
DB_NAME=syslog_device_a
DB_HOST=postgres
DB_PORT=5432
DB_USER=syslog
DB_PASSWORD=syslog
DB_SSLMODE=disable

# .env.device-b
SYSLOG_PORT=1515
DB_NAME=syslog_device_b
DB_HOST=postgres
DB_PORT=5432
DB_USER=syslog
DB_PASSWORD=syslog
DB_SSLMODE=disable
```

Start each instance with its own project name:

```bash
docker compose --env-file .env.device-a -p device-a up --build -d
docker compose --env-file .env.device-b -p device-b up --build -d
```

## Testing

### Send a test syslog message (RFC 3164)

```bash
echo "<13>Apr 22 10:00:00 myhost myapp: test message" | nc -u -w1 localhost 514
```

### Send a test syslog message (RFC 5424)

```bash
echo '<165>1 2026-04-22T10:00:00Z myhost myapp 1234 ID47 - Test message from RFC 5424' | nc -u -w1 localhost 514
```

### Query stored logs

```bash
docker compose exec postgres psql -U syslog -d syslog -c "SELECT id, received_at, hostname, app_name, severity, message FROM logs ORDER BY id DESC LIMIT 10;"
```

### View all log fields

```bash
docker compose exec postgres psql -U syslog -d syslog -c "SELECT * FROM logs;"
```

### Stop the services

```bash
docker compose down
```

### Stop and remove all data

```bash
docker compose down -v
```
