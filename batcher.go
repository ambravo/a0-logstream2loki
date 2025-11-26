package main

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Batcher accumulates log entries and sends them to Loki in batches
type Batcher struct {
	lokiClient   *LokiClient
	entryChan    <-chan LogEntry
	batchSize    int
	flushTimeout time.Duration
	logger       *slog.Logger
	wg           *sync.WaitGroup
	ctx          context.Context
}

// NewBatcher creates a new batcher instance
func NewBatcher(
	lokiClient *LokiClient,
	entryChan <-chan LogEntry,
	batchSize int,
	flushTimeout time.Duration,
	logger *slog.Logger,
	wg *sync.WaitGroup,
	ctx context.Context,
) *Batcher {
	return &Batcher{
		lokiClient:   lokiClient,
		entryChan:    entryChan,
		batchSize:    batchSize,
		flushTimeout: flushTimeout,
		logger:       logger,
		wg:           wg,
		ctx:          ctx,
	}
}

// Run starts the batching worker
// It reads from entryChan, accumulates entries into batches grouped by label set,
// and flushes when either the batch size or timeout is reached
func (b *Batcher) Run() {
	defer b.wg.Done()

	// Map of label key -> Batch
	// Label key is computed from the label set to group entries
	batches := make(map[string]*Batch)

	// Timer for flush timeout
	flushTimer := time.NewTimer(b.flushTimeout)
	defer flushTimer.Stop()

	totalEntries := 0
	firstEntryTime := time.Time{}

	for {
		select {
		case <-b.ctx.Done():
			// Context cancelled, flush remaining batches and exit
			b.logger.Info("Batcher shutting down, flushing remaining batches",
				"pending_entries", totalEntries,
			)
			b.flush(batches)
			return

		case entry, ok := <-b.entryChan:
			if !ok {
				// Channel closed, flush and exit
				b.logger.Info("Entry channel closed, flushing remaining batches",
					"pending_entries", totalEntries,
				)
				b.flush(batches)
				return
			}

			// Track first entry time for the current batch set
			if totalEntries == 0 {
				firstEntryTime = time.Now()
				// Reset and start the flush timer
				if !flushTimer.Stop() {
					select {
					case <-flushTimer.C:
					default:
					}
				}
				flushTimer.Reset(b.flushTimeout)
			}

			// Compute label key for grouping
			labelKey := computeLabelKey(entry.Labels)

			// Get or create batch for this label set
			batch, exists := batches[labelKey]
			if !exists {
				batch = &Batch{
					Labels:     entry.Labels,
					Entries:    make([]LogEntry, 0, b.batchSize),
					FirstEntry: time.Now(),
				}
				batches[labelKey] = batch
			}

			// Add entry to batch
			batch.Entries = append(batch.Entries, entry)
			totalEntries++

			// Check if we should flush based on size
			if totalEntries >= b.batchSize {
				b.logger.Debug("Flushing batch (size limit reached)",
					"total_entries", totalEntries,
					"streams", len(batches),
				)
				b.flush(batches)
				batches = make(map[string]*Batch)
				totalEntries = 0
				firstEntryTime = time.Time{}
			}

		case <-flushTimer.C:
			// Timeout elapsed, flush if we have any entries
			if totalEntries > 0 {
				elapsed := time.Since(firstEntryTime)
				b.logger.Debug("Flushing batch (timeout reached)",
					"total_entries", totalEntries,
					"streams", len(batches),
					"elapsed_ms", elapsed.Milliseconds(),
				)
				b.flush(batches)
				batches = make(map[string]*Batch)
				totalEntries = 0
				firstEntryTime = time.Time{}
			}
			// Reset timer for next interval
			flushTimer.Reset(b.flushTimeout)
		}
	}
}

// flush sends the accumulated batches to Loki
func (b *Batcher) flush(batches map[string]*Batch) {
	if len(batches) == 0 {
		return
	}

	// Count total entries across all streams
	totalEntries := 0
	for _, batch := range batches {
		totalEntries += len(batch.Entries)
	}

	// Create a context with timeout for the Loki push
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send to Loki
	start := time.Now()
	if err := b.lokiClient.Push(ctx, batches); err != nil {
		b.logger.Error("Failed to push batch to Loki",
			"error", err,
			"total_entries", totalEntries,
			"streams", len(batches),
		)
	} else {
		elapsed := time.Since(start)
		b.logger.Info("Successfully pushed batch to Loki",
			"total_entries", totalEntries,
			"streams", len(batches),
			"duration_ms", elapsed.Milliseconds(),
		)
	}
}

// computeLabelKey creates a unique key from a label set for grouping
// This uses a simple concatenation approach - labels are assumed to have consistent keys
func computeLabelKey(labels map[string]string) string {
	// For our use case, we know the exact labels: service_name, type, environment_name, tenant_name
	// Create a deterministic key by concatenating them in a fixed order
	return labels["service_name"] + "|" + labels["type"] + "|" + labels["environment_name"] + "|" + labels["tenant_name"]
}
