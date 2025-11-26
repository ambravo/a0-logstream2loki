package main

import (
	"flag"
	"fmt"
	"os"
)

// Config holds all configuration for the service
type Config struct {
	LokiURL         string
	LokiUsername    string   // Optional: Loki basic auth username
	LokiPassword    string   // Optional: Loki basic auth password
	ListenAddr      string
	HMACSecret      string
	CustomAuthToken string   // Optional: Custom authorization token (takes precedence over HMAC)
	BatchSize       int
	BatchFlush      int      // milliseconds
	VerboseLogging  bool     // Enable verbose logging and bypass IP allowlist
	IgnoreAuth0IPs  bool     // Ignore Auth0's official IP ranges
	CustomIPs       []string // Custom IPs to add to allowlist
	IPAllowlist     []string // Final computed allowlist (not configured directly)
}

// LoadConfig loads configuration from environment variables and command-line flags
// Command-line flags take precedence over environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// Define flags
	lokiURL := flag.String("loki-url", "", "Loki base URL (e.g. http://loki:3100)")
	lokiUsername := flag.String("loki-username", "", "Loki basic auth username (optional)")
	lokiPassword := flag.String("loki-password", "", "Loki basic auth password (optional)")
	listenAddr := flag.String("listen-addr", "", "HTTP listen address (e.g. :8080)")
	hmacSecret := flag.String("hmac-secret", "", "HMAC secret key for bearer token validation")
	customAuthToken := flag.String("custom-auth-token", "", "Custom authorization token (takes precedence over HMAC)")
	batchSize := flag.Int("batch-size", 500, "Maximum number of entries per batch")
	batchFlush := flag.Int("batch-flush-ms", 200, "Maximum milliseconds before flushing a batch")
	verbose := flag.Bool("verbose", false, "Enable verbose logging and bypass IP allowlist")
	ignoreAuth0IPs := flag.Bool("ignore-auth0-ips", false, "Ignore Auth0's official IP ranges")
	customIPs := flag.String("custom-ips", "", "Comma-separated list of custom IPs to add to allowlist")

	flag.Parse()

	// Load from environment variables first
	cfg.LokiURL = getEnv("LOKI_URL", "")
	cfg.LokiUsername = getEnv("LOKI_USERNAME", "")
	cfg.LokiPassword = getEnv("LOKI_PASSWORD", "")
	cfg.ListenAddr = getEnv("LISTEN_ADDR", ":8080")
	cfg.HMACSecret = getEnv("HMAC_SECRET", "")
	cfg.CustomAuthToken = getEnv("CUSTOM_AUTH_TOKEN", "")
	cfg.BatchSize = getEnvInt("BATCH_SIZE", 500)
	cfg.BatchFlush = getEnvInt("BATCH_FLUSH_MS", 200)
	cfg.VerboseLogging = getEnvBool("VERBOSE_LOGGING", false)
	cfg.IgnoreAuth0IPs = getEnvBool("IGNORE_AUTH0_IPS", false)
	cfg.CustomIPs = getEnvSlice("CUSTOM_IPS", []string{})

	// Override with flags if provided
	if *lokiURL != "" {
		cfg.LokiURL = *lokiURL
	}
	if *lokiUsername != "" {
		cfg.LokiUsername = *lokiUsername
	}
	if *lokiPassword != "" {
		cfg.LokiPassword = *lokiPassword
	}
	if *listenAddr != "" {
		cfg.ListenAddr = *listenAddr
	}
	if *hmacSecret != "" {
		cfg.HMACSecret = *hmacSecret
	}
	if *customAuthToken != "" {
		cfg.CustomAuthToken = *customAuthToken
	}
	if flag.Lookup("batch-size").Value.String() != "500" {
		cfg.BatchSize = *batchSize
	}
	if flag.Lookup("batch-flush-ms").Value.String() != "200" {
		cfg.BatchFlush = *batchFlush
	}
	if *verbose {
		cfg.VerboseLogging = true
	}
	if *ignoreAuth0IPs {
		cfg.IgnoreAuth0IPs = true
	}
	if *customIPs != "" {
		cfg.CustomIPs = parseCommaSeparated(*customIPs)
	}

	// Validate required configuration
	if cfg.LokiURL == "" {
		return nil, fmt.Errorf("LOKI_URL is required (set via environment variable or -loki-url flag)")
	}

	// Either HMAC_SECRET or CUSTOM_AUTH_TOKEN must be set
	if cfg.HMACSecret == "" && cfg.CustomAuthToken == "" {
		return nil, fmt.Errorf("either HMAC_SECRET or CUSTOM_AUTH_TOKEN is required")
	}

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an integer environment variable or returns a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intVal int
		if _, err := fmt.Sscanf(value, "%d", &intVal); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvBool retrieves a boolean environment variable or returns a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch value {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return defaultValue
}

// getEnvSlice retrieves a comma-separated environment variable as a slice
func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return parseCommaSeparated(value)
	}
	return defaultValue
}

// parseCommaSeparated parses a comma-separated string into a slice
func parseCommaSeparated(s string) []string {
	if s == "" {
		return []string{}
	}

	parts := []string{}
	for _, part := range splitAndTrim(s, ",") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// splitAndTrim splits a string and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		parts = append(parts, trimmed)
	}
	return parts
}

// splitString splits a string by separator
func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}

	result := []string{}
	current := ""

	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, current)
			current = ""
			i += len(sep) - 1
		} else {
			current += string(s[i])
		}
	}
	result = append(result, current)
	return result
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}
