# a0-logstream2loki

A minimal, high-performance Go service that receives Auth0 Log Streaming as JSON Lines (JSONL) over HTTP POST and translates it into Loki `/loki/api/v1/push` requests.

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
| `HMAC_SECRET` | `-hmac-secret` | Secret key for HMAC token validation |
| `LOKI_USERNAME` | `-loki-username` | Loki basic auth username (optional) |
| `LOKI_PASSWORD` | `-loki-password` | Loki basic auth password (optional) |

### Optional Configuration

| Environment Variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `LISTEN_ADDR` | `-listen-addr` | `:8080` | HTTP listen address |
| `BATCH_SIZE` | `-batch-size` | `500` | Maximum entries per batch |
| `BATCH_FLUSH_MS` | `-batch-flush-ms` | `200` | Maximum milliseconds before flushing |
| `VERBOSE_LOGGING` | `-verbose` | `false` | Enable verbose logging and bypass IP allowlist |

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

The service includes a built-in allowlist of Auth0's publicly announced IP addresses for all regions (US, EU, AU). This provides an additional layer of security by rejecting requests from unknown IPs.

**Sources**:
- [Auth0 IP Addresses Documentation](https://auth0.com/docs/troubleshoot/customer-support/operational-policies/ip-addresses)

The IP allowlist includes:
- US region IPs (18.233.90.226, 3.211.189.167, 3.88.245.107, etc.)
- EU region IPs (52.28.56.226, 52.28.45.240, 52.16.224.164, etc.)
- AU region IPs (54.66.205.24, 54.66.202.17, 13.54.254.182, etc.)

**Cloudflare Support**: The service automatically extracts the real client IP from `X-Forwarded-For` headers when behind Cloudflare or other proxies.

**Verbose Mode**: Use `-verbose` or `VERBOSE_LOGGING=true` to bypass the IP allowlist during testing or development.

```bash
# Enable verbose logging (bypasses IP allowlist)
./a0-logstream2loki -verbose
```

## Usage

### Authentication

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
docker-compose up -d

# View logs
docker-compose logs -f a0-logstream2loki
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
