package main

import (
	"flag"
	"fmt"
	"os"
)

// Config holds all configuration for the service
type Config struct {
	LokiURL       string
	LokiUsername  string // Optional: Loki basic auth username
	LokiPassword  string // Optional: Loki basic auth password
	ListenAddr    string
	HMACSecret    string
	BatchSize     int
	BatchFlush    int  // milliseconds
	VerboseLogging bool // Enable verbose logging and bypass IP allowlist
	IPAllowlist   []string // Allowed source IP addresses
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
	batchSize := flag.Int("batch-size", 500, "Maximum number of entries per batch")
	batchFlush := flag.Int("batch-flush-ms", 200, "Maximum milliseconds before flushing a batch")
	verbose := flag.Bool("verbose", false, "Enable verbose logging and bypass IP allowlist")

	flag.Parse()

	// Load from environment variables first
	cfg.LokiURL = getEnv("LOKI_URL", "")
	cfg.LokiUsername = getEnv("LOKI_USERNAME", "")
	cfg.LokiPassword = getEnv("LOKI_PASSWORD", "")
	cfg.ListenAddr = getEnv("LISTEN_ADDR", ":8080")
	cfg.HMACSecret = getEnv("HMAC_SECRET", "")
	cfg.BatchSize = getEnvInt("BATCH_SIZE", 500)
	cfg.BatchFlush = getEnvInt("BATCH_FLUSH_MS", 200)
	cfg.VerboseLogging = getEnvBool("VERBOSE_LOGGING", false)
	cfg.IPAllowlist = getDefaultIPAllowlist()

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
	if flag.Lookup("batch-size").Value.String() != "500" {
		cfg.BatchSize = *batchSize
	}
	if flag.Lookup("batch-flush-ms").Value.String() != "200" {
		cfg.BatchFlush = *batchFlush
	}
	if *verbose {
		cfg.VerboseLogging = true
	}

	// Validate required configuration
	if cfg.LokiURL == "" {
		return nil, fmt.Errorf("LOKI_URL is required (set via environment variable or -loki-url flag)")
	}
	if cfg.HMACSecret == "" {
		return nil, fmt.Errorf("HMAC_SECRET is required (set via environment variable or -hmac-secret flag)")
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

// getDefaultIPAllowlist returns the default Auth0 public IP allowlist
// Source: https://auth0.com/docs/troubleshoot/customer-support/operational-policies/ip-addresses
func getDefaultIPAllowlist() []string {
	// Auth0 publicly announced IP addresses for Log Streaming
	// These IPs are used by Auth0's US region
	return []string{
		"138.91.154.99",
		"54.183.64.135",
		"54.67.77.38",
		"54.67.15.170",
		"54.183.204.205",
		"54.173.21.107",
		"52.7.35.158",
		"35.167.74.121",
		"35.160.3.103",
		"52.14.17.114",
		"52.14.38.78",
		"52.14.40.253",
		"18.233.90.226",
		"3.211.189.167",
		"3.88.245.107",
		"34.195.142.251",
		"138.91.154.99",
		"52.7.35.158",
		"52.14.17.114",
		"52.14.38.78",
		"52.14.40.253",
		"54.67.15.170",
		"54.67.77.38",
		"54.173.21.107",
		"54.183.64.135",
		"54.183.204.205",
		"35.160.3.103",
		"35.167.74.121",
		// EU region IPs
		"52.28.56.226",
		"52.28.45.240",
		"52.16.224.164",
		"52.16.193.66",
		"34.253.4.94",
		"52.50.106.250",
		"52.211.56.181",
		"52.213.38.246",
		"52.213.74.69",
		"52.213.216.142",
		// AU region IPs
		"54.66.205.24",
		"54.66.202.17",
		"13.54.254.182",
		"13.55.232.24",
		"13.210.52.131",
		"13.236.8.164",
		"13.238.158.225",
	}
}
