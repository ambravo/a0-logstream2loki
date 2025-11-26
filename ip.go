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

// isLocalIP checks if the given IP address is in a private/local network range
// Supports IPv4 private ranges (RFC1918), loopback, and link-local addresses
func isLocalIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Check for IPv4 private ranges
	if ip.To4() != nil {
		// Loopback: 127.0.0.0/8
		if ip.IsLoopback() {
			return true
		}

		// Link-local: 169.254.0.0/16
		if ip.IsLinkLocalUnicast() {
			return true
		}

		// RFC1918 Private ranges:
		// 10.0.0.0/8
		if ip[0] == 10 {
			return true
		}

		// 172.16.0.0/12
		if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
			return true
		}

		// 192.168.0.0/16
		if ip[0] == 192 && ip[1] == 168 {
			return true
		}
	}

	// Check for IPv6 private ranges
	if ip.To4() == nil {
		// Loopback: ::1
		if ip.IsLoopback() {
			return true
		}

		// Link-local: fe80::/10
		if ip.IsLinkLocalUnicast() {
			return true
		}

		// Unique local: fc00::/7
		if len(ip) >= 1 && (ip[0]&0xfe) == 0xfc {
			return true
		}
	}

	return false
}
