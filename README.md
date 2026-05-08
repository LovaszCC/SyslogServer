# Syslog Server

A Go-based syslog server that accepts TCP syslog messages (RFC 3164 and RFC 5424, newline-delimited per RFC 6587 non-transparent framing) and stores them in PostgreSQL. Runs in Docker with full environment-based configuration for multi-instance deployments.

## Configuration

All settings are configured via environment variables:

| Variable      | Description              | Default     |
|---------------|--------------------------|-------------|
| `SYSLOG_PORT` | TCP port to listen on    | `514`       |
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
echo "<13>Apr 22 10:00:00 myhost myapp: test message" | nc -w1 localhost 514
```

### Send a test syslog message (RFC 5424)

```bash
echo '<165>1 2026-04-22T10:00:00Z myhost myapp 1234 ID47 - Test message from RFC 5424' | nc -w1 localhost 514
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

## Kubernetes Deployment (Helm)

A Helm chart is provided under `helm/syslog-server/` to deploy the syslog server against an external PostgreSQL cluster. All settings — including the syslog port — are configurable via Helm values.

### Prerequisites

- Helm 3 installed
- A container registry with the built image
- An existing PostgreSQL cluster accessible from the Kubernetes cluster

### Build and push the image

```bash
docker build -t your-registry/syslog-server:latest .
docker push your-registry/syslog-server:latest
```

### Deploy with default values

```bash
helm install syslog-server ./helm/syslog-server \
  --set image.repository=your-registry/syslog-server \
  --set db.host=your-postgres-host \
  --set db.password=your-password
```

### Deploy with a custom port and database

```bash
helm install syslog-server ./helm/syslog-server \
  --set image.repository=your-registry/syslog-server \
  --set syslogPort=1514 \
  --set db.host=your-postgres-host \
  --set db.password=your-password \
  --set db.name=syslog_device_a
```

### All configurable values

| Value              | Description              | Default                                  |
|--------------------|--------------------------|------------------------------------------|
| `image.repository` | Container image          | `syslog-server`                          |
| `image.tag`        | Image tag                | `latest`                                 |
| `syslogPort`       | TCP port to listen on    | `514`                                    |
| `db.host`          | PostgreSQL host          | `postgres.database.svc.cluster.local`    |
| `db.port`          | PostgreSQL port          | `5432`                                   |
| `db.user`          | PostgreSQL user          | `syslog`                                 |
| `db.password`      | PostgreSQL password      | `syslog`                                 |
| `db.name`          | PostgreSQL database name | `syslog`                                 |
| `db.sslmode`       | PostgreSQL SSL mode      | `disable`                                |

### Verify

```bash
kubectl get pods -l app=syslog-server
kubectl logs -l app=syslog-server
```

### Get the external IP

```bash
kubectl get svc syslog-server
```

### Running multiple instances on Kubernetes

Install the chart multiple times with different release names, ports, and databases:

```bash
helm install device-a ./helm/syslog-server \
  --set image.repository=your-registry/syslog-server \
  --set syslogPort=1514 \
  --set db.host=your-postgres-host \
  --set db.password=your-password \
  --set db.name=syslog_device_a

helm install device-b ./helm/syslog-server \
  --set image.repository=your-registry/syslog-server \
  --set syslogPort=1515 \
  --set db.host=your-postgres-host \
  --set db.password=your-password \
  --set db.name=syslog_device_b
```
