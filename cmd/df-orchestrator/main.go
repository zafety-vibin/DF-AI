package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/df-ai/orchestrator/internal/config"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/http"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
	"golang.org/x/sync/errgroup"
)

var (
	configPath = flag.String("config", "config/orchestrator.yaml", "path to configuration file")
	version    = flag.Bool("version", false, "print version and exit")
)

const Version = "0.1.0-001-binary-protocol"

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("DF AI Orchestrator v%s\n", Version)
		fmt.Printf("Protocol Version: %d\n", protocol.ProtocolVersion)
		os.Exit(0)
	}

	// Create temporary logger for config loading
	tempLogger := logging.NewTextLogger("info")

	// Load configuration with hot-reload support
	configMgr, err := config.NewConfigManager(*configPath, tempLogger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Get initial config
	cfg := configMgr.Get()

	// Create logger with config settings
	var logger *logging.Logger
	if cfg.LogFormat == "json" {
		logger = logging.NewJSONLogger(cfg.LogLevel)
	} else {
		logger = logging.NewTextLogger(cfg.LogLevel)
	}

	logger.Info("DF AI Orchestrator starting",
		logging.Field{Key: "version", Value: Version},
		logging.Field{Key: "protocol_version", Value: protocol.ProtocolVersion},
		logging.Field{Key: "port", Value: cfg.ListenPort})

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start config watcher
	configMgr.Watch(ctx)
	logger.Info("config watcher started")

	// Create DFHack client
	client := dfhack.NewClient(logger)

	// Start client (begins listening for plugin connections)
	if err := client.Start(ctx, cfg.ListenPort); err != nil {
		logger.Error("failed to start client", err)
		os.Exit(1)
	}

	// Create and start HTTP server if enabled
	var httpServer *http.Server
	if cfg.EnableHttpApi {
		httpServer = http.NewServer(logger, client, configMgr)
		if err := httpServer.Start(ctx); err != nil {
			logger.Error("failed to start HTTP server", err)
			os.Exit(1)
		}
	}

	logger.Info("waiting for DFHack plugin to connect...")

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Wait for connection and request initial state
	// In a real implementation, we'd wait for connection event
	// For now, we'll just wait a bit and request full state
	time.Sleep(2 * time.Second)

	if client.IsConnected() {
		stateCh, err := client.RequestFullState(protocol.ReasonManual)
		if err != nil {
			logger.Error("failed to request full state", err)
		} else {
			logger.Info("requested full state from plugin")

			// Wait for full state with timeout
			select {
			case state := <-stateCh:
				logger.Info("received full state",
					logging.Field{Key: "width", Value: state.Width},
					logging.Field{Key: "height", Value: state.Height},
					logging.Field{Key: "depth", Value: state.Depth},
					logging.Field{Key: "tiles", Value: len(state.Tiles)})
			case <-time.After(10 * time.Second):
				logger.Warn("timeout waiting for full state")
			}
		}

		// Subscribe to tile updates
		updates := client.SubscribeTileUpdates()
		go func() {
			for update := range updates {
				logger.Info("tile update received",
					logging.Field{Key: "changed_tiles", Value: update.Count})

				// Log first few tiles for debugging
				if update.Count > 0 && update.Count <= 10 {
					for i, tile := range update.Tiles {
						logger.Debug("tile changed",
							logging.Field{Key: "index", Value: i},
							logging.Field{Key: "x", Value: tile.X},
							logging.Field{Key: "y", Value: tile.Y},
							logging.Field{Key: "z", Value: tile.Z},
							logging.Field{Key: "type", Value: tile.TileType})
					}
				}
			}
		}()
	} else {
		logger.Warn("plugin not connected yet")
	}

	// Wait for shutdown signal
	sig := <-sigCh
	logger.Info("received shutdown signal",
		logging.Field{Key: "signal", Value: sig.String()})

	logger.Info("shutting down gracefully...")

	// Cancel context to stop servers
	cancel()

	// Use errgroup for coordinated shutdown
	g, shutdownCtx := errgroup.WithContext(context.Background())

	// Shutdown DFHack client
	g.Go(func() error {
		return client.Stop()
	})

	// Shutdown HTTP server if running
	if httpServer != nil {
		g.Go(func() error {
			shutdownCtx, cancel := context.WithTimeout(shutdownCtx, 30*time.Second)
			defer cancel()
			return httpServer.Stop(shutdownCtx)
		})
	}

	// Wait for all servers to stop
	if err := g.Wait(); err != nil {
		logger.Error("shutdown error", err)
	}

	logger.Info("shutdown complete")
}
