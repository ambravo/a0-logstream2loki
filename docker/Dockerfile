# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY *.go ./

# Build the application with security hardening flags
# Use TARGETARCH for multi-platform builds (set by buildx)
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -a \
    -installsuffix cgo \
    -ldflags="-s -w -extldflags '-static'" \
    -trimpath \
    -o a0-logstream2loki .

# Final stage - use distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Copy timezone data and CA certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder
COPY --from=builder /app/a0-logstream2loki /a0-logstream2loki

# Use non-root user (distroless nonroot user)
USER nonroot:nonroot

# Expose the default port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/a0-logstream2loki"]
