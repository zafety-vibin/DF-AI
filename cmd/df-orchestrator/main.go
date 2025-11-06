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
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
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

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create logger
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

	// Create DFHack client
	client := dfhack.NewClient(logger)

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start client (begins listening for plugin connections)
	if err := client.Start(ctx, cfg.ListenPort); err != nil {
		logger.Error("failed to start client", err)
		os.Exit(1)
	}
	defer client.Stop()

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
	cancel()

	logger.Info("shutdown complete")
}
