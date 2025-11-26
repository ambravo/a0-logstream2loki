package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Auth0IPRanges represents the structure of Auth0's IP ranges JSON
type Auth0IPRanges struct {
	LogStreaming []string `json:"log_streaming"`
}

// fetchAuth0IPRanges fetches the latest IP ranges from Auth0's CDN
func fetchAuth0IPRanges(logger *slog.Logger) ([]string, error) {
	const auth0IPRangesURL = "https://cdn.auth0.com/ip-ranges.json"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(auth0IPRangesURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Auth0 IP ranges: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Auth0 IP ranges endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Auth0 IP ranges response: %w", err)
	}

	var ipRanges Auth0IPRanges
	if err := json.Unmarshal(body, &ipRanges); err != nil {
		return nil, fmt.Errorf("failed to parse Auth0 IP ranges JSON: %w", err)
	}

	logger.Info("Fetched Auth0 IP ranges",
		"count", len(ipRanges.LogStreaming),
		"source", auth0IPRangesURL,
	)

	return ipRanges.LogStreaming, nil
}

// buildIPAllowlist constructs the final IP allowlist based on configuration
func buildIPAllowlist(cfg *Config, logger *slog.Logger) []string {
	var allowlist []string

	// Add Auth0's official IP ranges unless disabled
	if !cfg.IgnoreAuth0IPs {
		auth0IPs, err := fetchAuth0IPRanges(logger)
		if err != nil {
			logger.Warn("Failed to fetch Auth0 IP ranges, using empty list",
				"error", err,
			)
		} else {
			allowlist = append(allowlist, auth0IPs...)
			logger.Info("Added Auth0 IP ranges to allowlist",
				"count", len(auth0IPs),
			)
		}
	} else {
		logger.Info("Auth0 IP ranges ignored (IGNORE_AUTH0_IPS=true)")
	}

	// Add custom IPs on top of Auth0's list
	if len(cfg.CustomIPs) > 0 {
		allowlist = append(allowlist, cfg.CustomIPs...)
		logger.Info("Added custom IPs to allowlist",
			"count", len(cfg.CustomIPs),
		)
	}

	// Remove duplicates
	allowlist = removeDuplicates(allowlist)

	logger.Info("Final IP allowlist built",
		"total_count", len(allowlist),
	)

	return allowlist
}

// removeDuplicates removes duplicate IPs from the allowlist
func removeDuplicates(ips []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, ip := range ips {
		if !seen[ip] {
			seen[ip] = true
			result = append(result, ip)
		}
	}

	return result
}
