# a0-logstream2loki

A minimal, high-performance Go service that receives Auth0 Log Streaming as JSON Lines (JSONL) over HTTP POST and translates it into Loki `/loki/api/v1/push` requests.

**Built with Go 1.23** | **Runs as non-root** | **Distroless container**

## Features

- **HMAC-based authentication**: Validates bearer tokens using HMAC-SHA256
- **IP allowlist**: Built-in Auth0 public IP allowlist for enhanced security
- **Cloudflare support**: Extracts real client IP from X-Forwarded-For headers
- **Streaming JSONL processing**: Memory-efficient line-by-line processing without loading entire request bodies
- **Intelligent batching**: Batches up to 500 entries or flushes after 200ms (configurable)
- **Label grouping**: Groups logs by `type`, `environment_name`, and `tenant_name` into separate Loki streams
- **Loki basic auth**: Optional basic authentication for Loki endpoints
- **Verbose logging**: Optional verbose mode that bypasses IP allowlist for testing
- **Graceful shutdown**: Handles SIGINT/SIGTERM, drains pending batches before exit
- **Zero dependencies**: Uses only Go standard library (log/slog for structured logging)
- **High performance**: Concurrent processing with bounded goroutines and efficient memory usage
- **Secure by default**: Distroless container, runs as non-root user, hardened build flags

## Architecture

```
┌─────────────┐
│   Auth0     │
│ Log Stream  │
└──────┬──────┘
       │ POST /logs?tenant=X + Bearer token
       ▼
┌─────────────────────────────┐
│   HTTP Handler              │
│  - HMAC auth validation     │
│  - JSONL streaming          │
│  - Line-by-line parsing     │
└──────┬──────────────────────┘
       │ LogEntry via buffered channel
       ▼
┌─────────────────────────────┐
│   Batcher Worker            │
│  - Groups by label set      │
│  - 500 entries OR 200ms     │
│  - Builds Loki payload      │
└──────┬──────────────────────┘
       │ Batch flush
       ▼
┌─────────────────────────────┐
│   Loki Client               │
│  - POST /loki/api/v1/push   │
│  - Connection pooling       │
│  - Timeout handling         │
└─────────────────────────────┘
```

## Installation

### Build from source

```bash
git clone https://github.com/amba/a0-logstream2loki.git
cd a0-logstream2loki
go build -o a0-logstream2loki
```

### Run with Go

```bash
go run .
```

## Configuration

The service can be configured via **environment variables** or **command-line flags**. Flags take precedence over environment variables.

### Required Configuration

| Environment Variable | Flag | Description |
|---------------------|------|-------------|
| `LOKI_URL` | `-loki-url` | Base URL of Loki (e.g., `http://loki:3100`) |
| `HMAC_SECRET` | `-hmac-secret` | Secret key for HMAC token validation (Mode 1) |
| `CUSTOM_AUTH_TOKEN` | `-custom-auth-token` | Custom static token (Mode 2, takes precedence) |
| `LOKI_USERNAME` | `-loki-username` | Loki basic auth username (optional) |
| `LOKI_PASSWORD` | `-loki-password` | Loki basic auth password (optional) |

### Optional Configuration

| Environment Variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `LISTEN_ADDR` | `-listen-addr` | `:8080` | HTTP listen address |
| `BATCH_SIZE` | `-batch-size` | `500` | Maximum entries per batch |
| `BATCH_FLUSH_MS` | `-batch-flush-ms` | `200` | Maximum milliseconds before flushing |
| `LOG_LEVEL` | `-log-level` | `INFO` | Log level: DEBUG, INFO, WARN, ERROR |
| `VERBOSE_LOGGING` | `-verbose` | `false` | Bypass ALL IP checks (testing mode) |
| `ALLOW_LOCAL_IPS` | `-allow-local-ips` | `false` | Allow requests from local/private network IPs |
| `IGNORE_AUTH0_IPS` | `-ignore-auth0-ips` | `false` | Don't fetch/use Auth0's official IP ranges |
| `CUSTOM_IPS` | `-custom-ips` | - | Comma-separated custom IPs to add to allowlist |

### Example: Environment Variables

```bash
export LOKI_URL="http://loki:3100"
export HMAC_SECRET="your-secret-key"
export LISTEN_ADDR=":8080"
./a0-logstream2loki
```

### Example: Command-Line Flags

```bash
./a0-logstream2loki \
  -loki-url "http://loki:3100" \
  -loki-username "admin" \
  -loki-password "password" \
  -hmac-secret "your-secret-key" \
  -listen-addr ":8080" \
  -batch-size 1000 \
  -batch-flush-ms 500 \
  -verbose
```

### Example: Using .env file

```bash
# Copy the example
cp .env.example .env

# Edit .env with your values
nano .env

# Run with environment variables
export $(cat .env | xargs)
./a0-logstream2loki
```

## Security

### IP Allowlist

The service automatically fetches and uses Auth0's **official IP ranges** from their CDN at startup:
- **Source**: https://cdn.auth0.com/ip-ranges.json ([Documentation](https://auth0.com/docs/secure/security-guidance/data-security/allowlist))
- **Updated automatically** on service restart
- Includes all regions (US, EU, AU, etc.)

#### IP Allowlist Configuration

**1. Default Behavior** - Fetch Auth0's official IPs:
```bash
# Service automatically fetches from https://cdn.auth0.com/ip-ranges.json
./a0-logstream2loki
```

**2. Add Custom IPs** - Extend Auth0's list with your own IPs:
```bash
export CUSTOM_IPS="192.168.1.100,10.0.0.5"
./a0-logstream2loki
```

**3. Allow Local Networks** - Accept requests from private IPs (RFC1918):
```bash
export ALLOW_LOCAL_IPS=true
./a0-logstream2loki
# Allows: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 127.0.0.0/8, etc.
```

**4. Ignore Auth0 IPs** - Only use custom IPs:
```bash
export IGNORE_AUTH0_IPS=true
export CUSTOM_IPS="192.168.1.100"
./a0-logstream2loki
```

**5. Verbose Mode** - Bypass ALL IP checks (testing/development):
```bash
./a0-logstream2loki -verbose
```

**Cloudflare Support**: The service automatically extracts the real client IP from `X-Forwarded-For` headers when behind Cloudflare or other proxies.

**Local Networks**: When `ALLOW_LOCAL_IPS=true`, the following IP ranges are automatically allowed:
- **IPv4**: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 127.0.0.0/8, 169.254.0.0/16
- **IPv6**: ::1 (loopback), fe80::/10 (link-local), fc00::/7 (unique local)

## Usage

### Authentication

The service supports **two authentication modes**:

#### Mode 1: HMAC-SHA256 (Default)

The service uses HMAC-SHA256 for authentication. Each request must include:

1. **Query parameter**: `tenant` (e.g., `/logs?tenant=amba`)
2. **Authorization header**: `Authorization: Bearer <token>`

The token is computed as:
```
token = hex(HMAC-SHA256(HMAC_SECRET, tenant))
```

#### Generating a valid token

Using `openssl`:
```bash
TENANT="amba"
SECRET="your-secret-key"
TOKEN=$(echo -n "$TENANT" | openssl dgst -sha256 -hmac "$SECRET" | cut -d' ' -f2)
echo "Bearer token: $TOKEN"
```

Using Python:
```python
import hmac
import hashlib

tenant = "amba"
secret = "your-secret-key"
token = hmac.new(secret.encode(), tenant.encode(), hashlib.sha256).hexdigest()
print(f"Bearer token: {token}")
```

#### Mode 2: Custom Static Token (Takes Precedence)

For simpler setups, you can configure a **custom static token** that takes precedence over HMAC validation:

```bash
export CUSTOM_AUTH_TOKEN="my-secret-static-token-12345"
./a0-logstream2loki
```

Then use it in requests:
```bash
curl -X POST "http://localhost:8080/logs?tenant=amba" \
  -H "Authorization: Bearer my-secret-static-token-12345" \
  -H "Content-Type: application/x-ndjson" \
  --data-binary @logs.jsonl
```

**Note**: If `CUSTOM_AUTH_TOKEN` is set, it takes precedence and HMAC validation is bypassed. The tenant parameter is still required but not used for authentication validation.

### Sending Logs

Send JSONL data to the `/logs` endpoint:

```bash
curl -X POST "http://localhost:8080/logs?tenant=amba" \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/x-ndjson" \
  --data-binary @logs.jsonl
```

### Input Format

Each line must be a valid JSON object with the following structure:

```json
{
  "log_id": "...",
  "data": {
    "date": "2025-11-10T16:46:51.387Z",
    "type": "gd_update_device_account",
    "environment_name": "production",
    "tenant_name": "amba",
    ...
  }
}
```

**Required fields**:
- `data.date`: RFC3339 timestamp
- `data.type`: Log type (becomes Loki label)
- `data.environment_name`: Environment name (becomes Loki label)
- `data.tenant_name`: Tenant name (becomes Loki label)

### Health Check

```bash
curl http://localhost:8080/health
```

Returns `200 OK` if the service is running.

## Error Handling

### HTTP Status Codes

- `202 Accepted`: Request authenticated and logs enqueued successfully
- `400 Bad Request`: Missing or invalid `tenant` query parameter
- `401 Unauthorized`: Missing, malformed, or invalid bearer token
- `405 Method Not Allowed`: Non-POST request to `/logs`

### Error Response Format

```json
{
  "error": "error_code"
}
```

Error codes:
- `missing_tenant`: Tenant query parameter not provided
- `missing_authorization`: Authorization header not provided
- `invalid_authorization_format`: Authorization header not in `Bearer <token>` format
- `invalid_token`: HMAC validation failed
- `ip_not_allowed`: Request IP not in allowlist (enable verbose logging to bypass)
- `method_not_allowed`: Request method is not POST

### Logging

The service uses structured JSON logging (via Go's `log/slog`). All logs are written to stdout.

Example log output:
```json
{"time":"2025-11-25T20:53:00Z","level":"INFO","msg":"Starting a0-logstream2loki service","loki_url":"http://loki:3100","listen_addr":":8080","batch_size":500,"batch_flush_ms":200}
{"time":"2025-11-25T20:53:05Z","level":"INFO","msg":"Processing log stream","tenant":"amba","remote_addr":"192.168.1.100:52314"}
{"time":"2025-11-25T20:53:05Z","level":"INFO","msg":"Successfully pushed batch to Loki","total_entries":350,"streams":5,"duration_ms":45}
```

## Graceful Shutdown

The service handles `SIGINT` and `SIGTERM` signals gracefully:

1. Stops accepting new HTTP requests
2. Closes the internal entry channel
3. Waits for the batcher to flush remaining entries to Loki
4. Exits cleanly

```bash
# Send SIGTERM
kill -TERM <pid>

# Or use Ctrl+C (SIGINT)
```

## Performance Considerations

- **Streaming**: Request bodies are processed line-by-line, not loaded entirely into memory
- **Batching**: Reduces Loki API calls by grouping up to 500 entries
- **Connection pooling**: Reuses HTTP connections to Loki
- **Bounded concurrency**: Fixed number of worker goroutines (no goroutine explosion)
- **Buffer reuse**: Minimizes allocations by reusing internal buffers

## Docker

### Using Docker Compose

The easiest way to run the service with Loki and Grafana:

```bash
# Set required environment variables
export HMAC_SECRET="your-secret-key"

# Start all services
docker compose -f docker/docker-compose.yml up -d

# View logs
docker compose -f docker/docker-compose.yml logs -f a0-logstream2loki
```

This starts:
- **a0-logstream2loki** on port 8080
- **Loki** on port 3100
- **Grafana** on port 3000 (optional, for visualization)

### Build Docker Image Manually

```bash
docker build -f docker/Dockerfile -t a0-logstream2loki .
```

### Run with Docker

```bash
docker run -d \
  -p 8080:8080 \
  -e LOKI_URL="http://loki:3100" \
  -e HMAC_SECRET="your-secret-key" \
  -e VERBOSE_LOGGING="false" \
  ghcr.io/amba/a0-logstream2loki:latest
```

### Pull from GitHub Container Registry

```bash
docker pull ghcr.io/amba/a0-logstream2loki:latest
```

## Development

### Run tests

```bash
go test ./...
```

### Run with race detector

```bash
go run -race .
```

### Build for production

```bash
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o a0-logstream2loki .
```

## CI/CD

The project includes a GitHub Actions workflow that automatically builds and pushes Docker images to GitHub Container Registry (ghcr.io) on every push to `master`/`main` or when a version tag is created.

### Automatic Builds

- **On push to master/main**: Builds and tags as `latest`
- **On version tag (v*.*.*)**: Builds and tags with semantic version

### Creating a Release

```bash
# Tag a new version
git tag v1.0.0
git push origin v1.0.0

# GitHub Actions will automatically build and push:
# - ghcr.io/amba/a0-logstream2loki:v1.0.0
# - ghcr.io/amba/a0-logstream2loki:1.0
# - ghcr.io/amba/a0-logstream2loki:1
# - ghcr.io/amba/a0-logstream2loki:latest
```

## License

MIT

## Contributing

Contributions are welcome! Please open an issue or pull request.
