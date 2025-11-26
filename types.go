package main

import "time"

// LogEntry represents a single log line to be sent to Loki
type LogEntry struct {
	Timestamp int64             // Unix nanoseconds
	Labels    map[string]string // Stream labels (type, environment_name, tenant_name)
	Line      string            // Original JSON line
}

// Auth0LogData represents the structure of incoming Auth0 log events
type Auth0LogData struct {
	Data struct {
		Date            string `json:"date"`
		Type            string `json:"type"`
		EnvironmentName string `json:"environment_name"`
		TenantName      string `json:"tenant_name"`
	} `json:"data"`
}

// LokiStream represents a single stream in the Loki push request
type LokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"` // [timestamp_ns, line]
}

// LokiPushRequest represents the Loki /loki/api/v1/push request payload
type LokiPushRequest struct {
	Streams []LokiStream `json:"streams"`
}

// Batch accumulates log entries for a single label set
type Batch struct {
	Labels     map[string]string
	Entries    []LogEntry
	FirstEntry time.Time // Track when first entry was added for timeout
}
