package main

import (
	"net"
	"net/http"
	"strings"
)

// extractClientIP extracts the real client IP from the request
// Checks X-Forwarded-For header first (for Cloudflare and other proxies)
// Falls back to X-Real-IP, then RemoteAddr
func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (Cloudflare and other proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// The first IP is the original client
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	// RemoteAddr is in format "IP:port", extract just the IP
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// isIPAllowed checks if the given IP is in the allowlist
func isIPAllowed(ip string, allowlist []string) bool {
	for _, allowedIP := range allowlist {
		if ip == allowedIP {
			return true
		}
	}
	return false
}
