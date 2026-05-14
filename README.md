# Syslog Server

A Go-based syslog server that accepts TCP and/or UDP syslog messages (RFC 3164 and RFC 5424; TCP framing supports RFC 6587 octet-counting, newline, and null-byte delimiters) and stores them in PostgreSQL. Runs in Docker with full environment-based configuration for multi-instance deployments.

## Configuration

All settings are configured via environment variables:

| Variable      | Description              | Default     |
|---------------|--------------------------|-------------|
| `SYSLOG_PORT` | Port to listen on        | `514`       |
| `PROTOCOL`    | Transport: `tcp`, `udp`, or `both` | `tcp` |
| `PROXY_PROTOCOL` | Expect HAProxy PROXY protocol header (v1/v2) on each TCP connection | `false` |
| `VENDOR_TYPE` | Vendor-specific parser: `mikrotik`, `vpn`, `opnsense`, `unifi`, or empty for generic RFC3164/RFC5424 | `""` |
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

### Send a test syslog message (RFC 3164, TCP)

```bash
echo "<13>Apr 22 10:00:00 myhost myapp: test message" | nc -w1 localhost 514
```

### Send a test syslog message (RFC 5424, TCP)

```bash
echo '<165>1 2026-04-22T10:00:00Z myhost myapp 1234 ID47 - Test message from RFC 5424' | nc -w1 localhost 514
```

### Send a test syslog message over UDP

```bash
echo "<13>Apr 22 10:00:00 myhost myapp: udp test" | nc -u -w1 localhost 514
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

## Vendor Parsers

Some devices wrap their own payload format inside a syslog message. Setting `VENDOR_TYPE` (env) or `vendorType` (Helm) activates a vendor-specific post-processor that runs after the generic RFC3164/RFC5424 parse and overrides selected fields. One vendor per instance/database — deploy multiple instances if you need to ingest several vendors.

Storage schema: `facility` and `severity` are stored as `TEXT` so vendor-defined labels (e.g. `wireguard`, `Low`) can coexist with the numeric RFC priority values used by the generic parser. The startup migration converts existing `INT` columns to `TEXT` automatically.

### `mikrotik`

Expected `message` body (pipe-delimited):

```
0|MikroTik|<model>|<firmware>|<id>|<topics>|<severity>|<key=value payload msg=...>
```

Example raw record:

```
0|MikroTik|CHR QEMU Standard PC (Q35 + ICH9, 2009)|7.20 (stable)|81|wireguard,debug|Low|dvchost=r-alphazoo-szfv-main-2 dvc=10.1.1.14 msg=AZWG4: [r-alphavet-aote] xaCnXzdvmw3pjYzJ5KxcOhVndWPbS+Owz2etocbUHk0=: Receiving keepalive packet from peer (109.74.57.32:13231)
```

Field overrides:

| Column     | Source                                     | Result for the example                  |
|------------|--------------------------------------------|-----------------------------------------|
| `facility` | First comma-token of pipe field `[5]`      | `wireguard`                             |
| `severity` | Pipe field `[6]`                           | `Low`                                   |
| `message`  | Substring after `msg=` in pipe field `[7]` | `AZWG4: [r-alphavet-aote] ... peer (109.74.57.32:13231)` |

If the pipe layout does not match (e.g. field `[1]` is not `MikroTik` or fewer than 8 parts), the record is left untouched and stored as parsed by the generic parser.

### `vpn`

Generic RFC5424 parsing strips the structured-data block (`[sd-id ...]`) from `message`. The `vpn` vendor keeps it: `message` is set to the substring of `raw` starting at the first `[` so that both the SD block and the trailing free-text are preserved.

Example raw record:

```
<134>1 2026-05-11T08:43:48.661Z vpnsrv vpn-management 2754235 vpn.disconnect [vpn@32473 common_name="koczor2" vpn_ip="10.214.12.181" client_ip="39.144.89.149" bytes_received="0" bytes_sent="0" rules_removed="2"] VPN disconnect koczor2 (10.214.12.181) duration=0s
```

Resulting `message`:

```
[vpn@32473 common_name="koczor2" vpn_ip="10.214.12.181" client_ip="39.144.89.149" bytes_received="0" bytes_sent="0" rules_removed="2"] VPN disconnect koczor2 (10.214.12.181) duration=0s
```

All other columns (`timestamp`, `hostname`, `app_name`, `facility`, `severity` from PRI) come from the standard RFC5424 parse and are left as-is.

### `opnsense`

OPNsense emits a mix of free-form syslog records (lighttpd, configd.py, openvpn, ...) and a CSV-style payload from `filterlog` (pf packet log). The `opnsense` vendor leaves every non-`filterlog` record untouched and rewrites `filterlog` records into a compact, human-readable `message`.

Expected `message` body for `filterlog` (pf CSV, common prefix):

```
rulenr,subrulenr,anchorname,rule_uuid,iface,reason,action,dir,ipver,...proto-specific fields...,src,dst,srcport,dstport,...
```

Example raw record:

```
<134>May 13 11:06:14 szfv-fw2.alpha-vet.hu filterlog[70387]: 26,,,02f4bab031b57d1e30553ce08e0ec131,vlan0.116,match,block,in,4,0x0,,64,0,0,DF,6,tcp,64,10.1.16.19,17.253.53.204,57374,443,0,S,695714130,,65535,,mss;nop;wscale;nop;nop;TS;sackOK;eol
```

Field overrides:

| Column     | Source                              | Result for the example                                                  |
|------------|-------------------------------------|-------------------------------------------------------------------------|
| `facility` | CSV field `action` (index 6)        | `block`                                                                 |
| `message`  | Composed from action/dir/iface/proto/src:port/dst:port/flags/length | `block in vlan0.116 tcp 10.1.16.19:57374 -> 17.253.53.204:443 flags=S len=64` |

UDP example:

```
313,,,7ce275edbb8cbeec24be89e9fbc19d7d,vlan0.1001,match,pass,in,4,0x0,,62,51261,0,DF,17,udp,69,10.240.5.12,10.1.5.250,3240,53,49
```

becomes:

```
pass in vlan0.1001 udp 10.240.5.12:3240 -> 10.1.5.250:53 len=69
```

Records that are not `filterlog` (e.g. `lighttpd`, `configd.py`, `openvpn_server2`) or that do not match the expected CSV layout are stored as parsed by the generic parser.

### `unifi`

UniFi emits two distinct shapes; the `unifi` vendor handles both.

**1. UniFi Network Controller** — a CEF record carried in an RFC3164 frame:

```
May 14 12:25:22 UniFi-Controller CEF:0|Ubiquiti|UniFi Network|10.0.162|546|Admin Made Config Changes|2|src=10.1.14.42 UNIFIcategory=System ... msg=sepsigav made 2 changes to System settings. Source IP: 10.1.14.42
```

CEF layout: `version|vendor|product|dev-version|signature-id|name|severity|extension`. Field overrides:

| Column     | Source                          | Result for the example                                            |
|------------|---------------------------------|-------------------------------------------------------------------|
| `app_name` | CEF `name` (index 5)            | `Admin Made Config Changes`                                       |
| `severity` | CEF `severity` (index 6)        | `2`                                                               |
| `message`  | text after `msg=` in extension  | `sepsigav made 2 changes to System settings. Source IP: 10.1.14.42` |

**2. UniFi APs / switches** — RFC3164 with a `<MAC>,<MODEL-FW>:` device prefix:

```
<13>May 14 12:25:27 acc-storage-tolto 6c63f8356535,USW-Lite-8-PoE-7.4.1+16850: syswrapper[27473]: Provision took 3 sec, full=0
```

The generic parser captures the whole `<MAC>,<MODEL-FW>` token as `app_name`. The vendor moves the model into `facility`, then re-extracts the real app tag and message body:

| Column     | Source                                | Result for the example        |
|------------|---------------------------------------|-------------------------------|
| `facility` | model-firmware (after the comma)      | `USW-Lite-8-PoE-7.4.1+16850`  |
| `app_name` | app tag from the message body         | `syswrapper`                  |
| `message`  | text after the app tag                | `Provision took 3 sec, full=0`|

Empty leading tags (`: cfgmtd[2393]: ...`) and nested tags (`mcad: mcad[416]: ...`) are handled. Records matching neither shape are stored as parsed by the generic parser.

### Deploying a vendor instance

Docker Compose:

```bash
# .env.mikrotik
SYSLOG_PORT=1514
VENDOR_TYPE=mikrotik
DB_NAME=syslog_mikrotik
DB_HOST=postgres
DB_PORT=5432
DB_USER=syslog
DB_PASSWORD=syslog
DB_SSLMODE=disable
```

```bash
docker compose --env-file .env.mikrotik -p mikrotik up --build -d
```

Helm:

```bash
helm install mikrotik ./helm/syslog-server \
  --set image.repository=your-registry/syslog-server \
  --set syslogPort=1514 \
  --set vendorType=mikrotik \
  --set db.host=your-postgres-host \
  --set db.password=your-password \
  --set db.name=syslog_mikrotik

helm install vpn ./helm/syslog-server \
  --set image.repository=your-registry/syslog-server \
  --set syslogPort=1515 \
  --set vendorType=vpn \
  --set db.host=your-postgres-host \
  --set db.password=your-password \
  --set db.name=syslog_vpn
```

## Kubernetes Deployment (Helm)

A Helm chart is provided under `helm/syslog-server/` to deploy the syslog server against an external PostgreSQL cluster. All settings — including the syslog port — are configurable via Helm values.


### Example for AV usage:
helm install syslog-server ./helm/syslog-server --namespace syslog-server --create-namespace   --set image.repository=harbor.alpha-vet.local/library/syslog-server   --set image.tag=v2.1.7   --set db.host=k8s.alpha-vet.local   --set db.password=PASSWORD   --set db.user=avit   --set db.name=syslog --set proxyProtocol=true  --set syslogPort=1514 --set protocol=tcp --set vendorType=mikrotik

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
| `syslogPort`       | Port to listen on        | `514`                                    |
| `protocol`         | Transport: `tcp`, `udp`, or `both` | `udp`                          |
| `proxyProtocol`    | Expect HAProxy PROXY protocol (TCP only) | `false`                      |
| `vendorType`       | Vendor-specific parser: `mikrotik`, `vpn`, `opnsense`, `unifi`, or empty | `""`        |
| `db.host`          | PostgreSQL host          | `postgres.database.svc.cluster.local`    |
| `db.port`          | PostgreSQL port          | `5432`                                   |
| `db.user`          | PostgreSQL user          | `syslog`                                 |
| `db.password`      | PostgreSQL password      | `syslog`                                 |
| `db.name`          | PostgreSQL database name | `syslog`                                 |
| `db.sslmode`       | PostgreSQL SSL mode      | `disable`                                |
| `cleanup.enabled`  | Deploy daily archive CronJob | `true`                               |
| `cleanup.schedule` | Cron expression          | `0 0 * * *` (midnight)                   |
| `cleanup.timeZone` | CronJob timeZone (K8s 1.27+) | `Europe/Budapest`                    |
| `cleanup.resources`| CronJob pod resources    | 50m/64Mi req, 500m/256Mi limit           |

## Log Archival

On startup the server creates `logs_archive` alongside `logs`. The Helm chart deploys a Kubernetes `CronJob` (`<release>-cleanup`) that runs daily at midnight (configurable via `cleanup.schedule` / `cleanup.timeZone`). The job uses the same container image with `args: ["cleanup"]`, which:

1. Connects to the same database as the server (same env/secret).
2. Ensures schema exists (idempotent).
3. In a single transaction: `LOCK TABLE logs EXCLUSIVE`, `INSERT INTO logs_archive SELECT ... FROM logs`, `TRUNCATE logs RESTART IDENTITY`, `COMMIT`.

The exclusive lock holds for the duration of the copy; inserts from the server briefly block but resume after commit. `logs_archive` retains `received_at` plus a fresh `archived_at` column so you can tell when each row was rolled over.

Disable the CronJob with `--set cleanup.enabled=false`. Run an ad-hoc archive manually:

```bash
kubectl create job --from=cronjob/<release>-cleanup <release>-cleanup-manual
```

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
