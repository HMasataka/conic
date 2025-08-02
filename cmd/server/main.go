package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HMasataka/conic/internal/config"
	"github.com/HMasataka/conic/internal/eventbus"
	"github.com/HMasataka/conic/internal/logging"
	"github.com/HMasataka/conic/pkg/signaling"
	"github.com/HMasataka/conic/pkg/transport/websocket"
)

func main() {
	var (
		configPath = flag.String("config", "", "config file path")
		host       = flag.String("host", "", "server host")
		port       = flag.Int("port", 0, "server port")
		logLevel   = flag.String("log-level", "", "log level (debug, info, warn, error)")
	)
	flag.Parse()

	// Load configuration
	cfg := loadConfig(*configPath, *host, *port, *logLevel)

	// Initialize logger
	logger := logging.New(cfg.Logging)
	logger.Info("starting conic server",
		"version", "1.0.0",
		"config", fmt.Sprintf("%+v", cfg),
	)

	// Initialize event bus
	eventBus := eventbus.NewInMemoryBus(1000)
	eventBus.Start(context.Background())
	defer eventBus.Stop()

	// Initialize hub
	hub := signaling.NewHub(logger, eventBus)

	// Create router
	router := signaling.NewRouter(hub, logger, eventBus)

	// Create WebSocket server
	wsServer := websocket.NewServer(
		websocket.WithHub(hub),
		websocket.WithLogger(logger),
		websocket.WithEventBus(eventBus),
		websocket.WithRouter(router),
	)

	// Create HTTP router
	mux := http.NewServeMux()
	mux.Handle("/ws", wsServer)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("received shutdown signal")
		cancel()
	}()

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start hub
	if err := hub.Start(ctx); err != nil {
		logger.Error("failed to start hub", "error", err)
		os.Exit(1)
	}

	logger.Info("starting websocket server", "addr", addr)

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown server
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	logger.Info("shutting down websocket server")

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	// Stop hub
	if err := hub.Stop(); err != nil {
		logger.Error("failed to stop hub", "error", err)
	}

	logger.Info("server stopped gracefully")
}

func loadConfig(configPath, host string, port int, logLevel string) *config.Config {
	var cfg *config.Config
	var err error

	if configPath != "" {
		cfg, err = config.Load(config.LoadOptions{Path: configPath})
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
	} else {
		cfg = config.Default()
	}

	// Override with flags
	if host != "" {
		cfg.Server.Host = host
	}
	if port > 0 {
		cfg.Server.Port = port
	}
	if logLevel != "" {
		cfg.Logging.Level = logLevel
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	return cfg
}
