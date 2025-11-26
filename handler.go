package main

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// LogsHandler handles incoming POST /logs requests
type LogsHandler struct {
	hmacSecret      string
	customAuthToken string
	entryChan       chan<- LogEntry
	logger          *slog.Logger
	verboseLogging  bool
	allowLocalIPs   bool
	ipAllowlist     []string
}

// NewLogsHandler creates a new logs handler
func NewLogsHandler(hmacSecret, customAuthToken string, entryChan chan<- LogEntry, logger *slog.Logger, verboseLogging, allowLocalIPs bool, ipAllowlist []string) *LogsHandler {
	return &LogsHandler{
		hmacSecret:      hmacSecret,
		customAuthToken: customAuthToken,
		entryChan:       entryChan,
		logger:          logger,
		verboseLogging:  verboseLogging,
		allowLocalIPs:   allowLocalIPs,
		ipAllowlist:     ipAllowlist,
	}
}

// ServeHTTP handles the HTTP request
func (h *LogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}

	// Extract client IP (supports X-Forwarded-For for Cloudflare)
	clientIP := extractClientIP(r)

	// Check IP allowlist (unless verbose logging is enabled)
	if !h.verboseLogging {
		isLocal := isLocalIP(clientIP)
		isAllowed := isIPAllowed(clientIP, h.ipAllowlist)

		// Allow if: in allowlist OR (local IP AND allow_local_ips enabled)
		if !isAllowed && !(isLocal && h.allowLocalIPs) {
			h.logger.Error("Request rejected: IP not in allowlist",
				"client_ip", clientIP,
				"is_local", isLocal,
				"remote_addr", r.RemoteAddr,
				"x_forwarded_for", r.Header.Get("X-Forwarded-For"),
			)
			writeJSONError(w, http.StatusForbidden, "ip_not_allowed")
			return
		}

		// Log if local IP was allowed due to allow_local_ips setting
		if isLocal && h.allowLocalIPs && !isAllowed {
			h.logger.Debug("Request allowed from local network IP",
				"client_ip", clientIP,
			)
		}
	}

	// Authenticate the request (custom token takes precedence over HMAC)
	tenant, ok := authenticateRequest(w, r, h.hmacSecret, h.customAuthToken, h.logger)
	if !ok {
		// authenticateRequest already wrote the error response and logged the failure
		return
	}

	if h.verboseLogging {
		h.logger.Info("Processing log stream",
			"tenant", tenant,
			"client_ip", clientIP,
			"remote_addr", r.RemoteAddr,
			"x_forwarded_for", r.Header.Get("X-Forwarded-For"),
		)
	} else {
		h.logger.Info("Processing log stream",
			"tenant", tenant,
			"client_ip", clientIP,
		)
	}

	// Stream the JSONL body line by line
	// Use bufio.Scanner for efficient line-by-line reading
	scanner := bufio.NewScanner(r.Body)
	defer r.Body.Close()

	// Increase buffer size for potentially large log lines
	const maxCapacity = 1024 * 1024 // 1MB per line
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	lineCount := 0
	errorCount := 0

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		lineCount++

		// Parse the JSON line to extract required fields
		entry, err := h.parseLogLine(line)
		if err != nil {
			errorCount++
			h.logger.Warn("Failed to parse log line",
				"error", err,
				"line_number", lineCount,
			)
			continue
		}

		// Send to batching worker via channel
		// This is non-blocking as long as the channel has capacity
		select {
		case h.entryChan <- entry:
			// Successfully enqueued
		default:
			// Channel is full - this shouldn't happen with proper buffering
			h.logger.Error("Entry channel is full, dropping log line",
				"line_number", lineCount,
			)
			errorCount++
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		h.logger.Error("Error reading request body",
			"error", err,
			"tenant", tenant,
		)
		writeJSONError(w, http.StatusBadRequest, "error_reading_body")
		return
	}

	h.logger.Info("Finished processing log stream",
		"tenant", tenant,
		"lines_processed", lineCount,
		"errors", errorCount,
	)

	// Return 202 Accepted (we don't wait for Loki to acknowledge)
	w.WriteHeader(http.StatusAccepted)
}

// parseLogLine parses a single JSON line and extracts the required fields
func (h *LogsHandler) parseLogLine(line string) (LogEntry, error) {
	var logData Auth0LogData

	// Parse the JSON to extract labels and timestamp
	if err := json.Unmarshal([]byte(line), &logData); err != nil {
		return LogEntry{}, err
	}

	// Parse the timestamp (RFC3339 format)
	timestamp, err := time.Parse(time.RFC3339Nano, logData.Data.Date)
	if err != nil {
		return LogEntry{}, err
	}

	// Create labels map
	labels := map[string]string{
		"type":             logData.Data.Type,
		"environment_name": logData.Data.EnvironmentName,
		"tenant_name":      logData.Data.TenantName,
	}

	return LogEntry{
		Timestamp: timestamp.UnixNano(),
		Labels:    labels,
		Line:      line, // Preserve the original line exactly
	}, nil
}
