package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// LokiClient handles sending batches to Loki
type LokiClient struct {
	client   *http.Client
	baseURL  string
	username string // Optional: basic auth username
	password string // Optional: basic auth password
	logger   *slog.Logger
}

// NewLokiClient creates a new Loki client
func NewLokiClient(baseURL, username, password string, logger *slog.Logger) *LokiClient {
	return &LokiClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DisableKeepAlives:   false,
			},
		},
		baseURL:  baseURL,
		username: username,
		password: password,
		logger:   logger,
	}
}

// Push sends a batch of log entries to Loki
// The batches map contains entries grouped by their label set
func (lc *LokiClient) Push(ctx context.Context, batches map[string]*Batch) error {
	if len(batches) == 0 {
		return nil
	}

	// Build the Loki push request payload
	payload := lc.buildPushRequest(batches)

	// Serialize to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Loki payload: %w", err)
	}

	// Create the HTTP request
	url := lc.baseURL + "/loki/api/v1/push"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add basic auth if configured
	if lc.username != "" && lc.password != "" {
		req.SetBasicAuth(lc.username, lc.password)
	}

	// Send the request
	resp, err := lc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Loki: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read a snippet of the response body for logging
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return fmt.Errorf("Loki returned non-2xx status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// buildPushRequest constructs a LokiPushRequest from batches
func (lc *LokiClient) buildPushRequest(batches map[string]*Batch) LokiPushRequest {
	streams := make([]LokiStream, 0, len(batches))

	for _, batch := range batches {
		if len(batch.Entries) == 0 {
			continue
		}

		// Build values array: [[timestamp_ns, line], ...]
		values := make([][]string, 0, len(batch.Entries))
		for _, entry := range batch.Entries {
			values = append(values, []string{
				strconv.FormatInt(entry.Timestamp, 10), // timestamp as string
				entry.Line,                             // original JSON line
			})
		}

		stream := LokiStream{
			Stream: batch.Labels,
			Values: values,
		}

		streams = append(streams, stream)
	}

	return LokiPushRequest{
		Streams: streams,
	}
}
