package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	// Load configuration first (with temporary logger)
	tempLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := LoadConfig()
	if err != nil {
		tempLogger.Error("Configuration error", "error", err)
		os.Exit(1)
	}

	// Parse log level from config
	logLevel := parseLogLevel(cfg.LogLevel)

	// Set up structured logging with configured level
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Build IP allowlist from Auth0 and custom sources
	cfg.IPAllowlist = buildIPAllowlist(cfg, logger)

	logger.Info("Starting a0-logstream2loki service",
		"loki_url", cfg.LokiURL,
		"listen_addr", cfg.ListenAddr,
		"batch_size", cfg.BatchSize,
		"batch_flush_ms", cfg.BatchFlush,
		"verbose_logging", cfg.VerboseLogging,
		"allow_local_ips", cfg.AllowLocalIPs,
		"ip_allowlist_size", len(cfg.IPAllowlist),
		"ignore_auth0_ips", cfg.IgnoreAuth0IPs,
		"custom_ips_count", len(cfg.CustomIPs),
		"custom_auth_enabled", cfg.CustomAuthToken != "",
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
	handler := NewLogsHandler(cfg.HMACSecret, cfg.CustomAuthToken, entryChan, logger, cfg.VerboseLogging, cfg.AllowLocalIPs, cfg.IPAllowlist)

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

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
