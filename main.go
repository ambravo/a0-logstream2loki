package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// Set up structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration from environment variables and flags
	cfg, err := LoadConfig()
	if err != nil {
		logger.Error("Configuration error", "error", err)
		os.Exit(1)
	}

	logger.Info("Starting a0-logstream2loki service",
		"loki_url", cfg.LokiURL,
		"listen_addr", cfg.ListenAddr,
		"batch_size", cfg.BatchSize,
		"batch_flush_ms", cfg.BatchFlush,
		"verbose_logging", cfg.VerboseLogging,
		"ip_allowlist_size", len(cfg.IPAllowlist),
		"loki_auth_enabled", cfg.LokiUsername != "",
	)

	// Create a buffered channel for log entries
	// Buffer size should be large enough to handle bursts
	entryChan := make(chan LogEntry, 10000)

	// Create Loki client
	lokiClient := NewLokiClient(cfg.LokiURL, cfg.LokiUsername, cfg.LokiPassword, logger)

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// WaitGroup to track worker goroutines
	var wg sync.WaitGroup

	// Start the batcher worker
	batcher := NewBatcher(
		lokiClient,
		entryChan,
		cfg.BatchSize,
		time.Duration(cfg.BatchFlush)*time.Millisecond,
		logger,
		&wg,
		ctx,
	)
	wg.Add(1)
	go batcher.Run()

	// Create HTTP handler
	handler := NewLogsHandler(cfg.HMACSecret, entryChan, logger, cfg.VerboseLogging, cfg.IPAllowlist)

	// Set up HTTP server with mux
	mux := http.NewServeMux()
	mux.Handle("/logs", handler)

	// Add a health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Channel to listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start HTTP server in a goroutine
	go func() {
		logger.Info("HTTP server listening", "addr", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	sig := <-sigChan
	logger.Info("Received shutdown signal", "signal", sig.String())

	// Graceful shutdown sequence:
	// 1. Stop accepting new HTTP requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	logger.Info("Shutting down HTTP server...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error during HTTP server shutdown", "error", err)
	}

	// 2. Close the entry channel to signal batcher to finish
	logger.Info("Closing entry channel...")
	close(entryChan)

	// 3. Cancel context to signal batcher to exit after flushing
	cancel()

	// 4. Wait for batcher to finish processing and flush remaining batches
	logger.Info("Waiting for batcher to finish...")
	wg.Wait()

	logger.Info("Shutdown complete")
}
