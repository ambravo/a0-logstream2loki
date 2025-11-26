package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// ErrorResponse represents a JSON error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// authenticateRequest validates the bearer token
// If customAuthToken is set, it uses exact token matching (takes precedence)
// Otherwise, it validates using HMAC-SHA256 of the tenant
// Returns the tenant string if authentication succeeds, otherwise writes an error response and returns empty string
func authenticateRequest(w http.ResponseWriter, r *http.Request, hmacSecret, customAuthToken string) (string, bool) {
	// Extract tenant query parameter
	tenant := r.URL.Query().Get("tenant")
	if tenant == "" {
		writeJSONError(w, http.StatusBadRequest, "missing_tenant")
		return "", false
	}

	// Extract bearer token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing_authorization")
		return "", false
	}

	// Parse "Bearer <token>" format (case-insensitive for "Bearer")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		writeJSONError(w, http.StatusUnauthorized, "invalid_authorization_format")
		return "", false
	}
	token := parts[1]

	// If custom auth token is configured, use exact matching (takes precedence)
	if customAuthToken != "" {
		// Timing-safe comparison of custom token
		if !hmac.Equal([]byte(token), []byte(customAuthToken)) {
			writeJSONError(w, http.StatusUnauthorized, "invalid_token")
			return "", false
		}
		return tenant, true
	}

	// Otherwise, use HMAC-SHA256 validation
	if hmacSecret == "" {
		writeJSONError(w, http.StatusUnauthorized, "authentication_not_configured")
		return "", false
	}

	// Compute HMAC-SHA256 of the tenant string using the configured secret
	mac := hmac.New(sha256.New, []byte(hmacSecret))
	mac.Write([]byte(tenant))
	expectedMAC := mac.Sum(nil)

	// Decode the provided token from hex
	providedMAC, err := hex.DecodeString(token)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "invalid_token")
		return "", false
	}

	// Timing-safe comparison
	if !hmac.Equal(expectedMAC, providedMAC) {
		writeJSONError(w, http.StatusUnauthorized, "invalid_token")
		return "", false
	}

	return tenant, true
}

// writeJSONError writes a JSON error response
func writeJSONError(w http.ResponseWriter, statusCode int, errorMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: errorMsg})
}
