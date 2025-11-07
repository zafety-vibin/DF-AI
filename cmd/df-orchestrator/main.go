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
	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/http"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
	"golang.org/x/sync/errgroup"
)

var (
	topologyOverlay *topology.TopologyOverlay
	hazardManager   *hazards.HazardManager
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

	// Set up topology overlay callback
	client.SetOnFullState(func(state *protocol.FullStateMessage) {
		// Build topology overlay
		topologyOverlay = topology.NewTopologyOverlay(state.Width, state.Height, state.Depth)
		if topologyOverlay == nil {
			logger.Error("failed to create topology overlay", fmt.Errorf("invalid dimensions"))
			return
		}

		buildStart := time.Now()
		if err := topologyOverlay.BuildFromTiles(state.Tiles); err != nil {
			logger.Error("failed to build topology overlay", err)
			return
		}

		buildDuration := time.Since(buildStart)
		logger.Info("topology overlay built",
			logging.Field{Key: "memory_kb", Value: topologyOverlay.GetMemoryUsage() / 1024},
			logging.Field{Key: "open_pct", Value: topologyOverlay.GetOpenPercentage()},
			logging.Field{Key: "build_ms", Value: buildDuration.Milliseconds()})

		// Build hazard overlays
		hazardManager = hazards.NewHazardManager(state.Width, state.Height, state.Depth)
		hazardManager.BuildFromTiles(state.Tiles)

		// Log hazard overlay statistics
		counts := hazardManager.GetAllCounts()
		logger.Info("hazard overlays built",
			logging.Field{Key: "aquifer_count", Value: counts["aquifer"]},
			logging.Field{Key: "water_count", Value: counts["water"]},
			logging.Field{Key: "lava_count", Value: counts["lava"]},
			logging.Field{Key: "cavern_count", Value: counts["caverns"]},
			logging.Field{Key: "memory_kb", Value: hazardManager.GetMemoryUsage() / 1024})

		// Test compression
		compConfig := topology.CompressionConfig{
			Mode:    cfg.TopologyCompressionMode,
			CenterZ: cfg.TopologyCenterZ,
			ZRadius: cfg.TopologyZRadius,
		}

		compressed, err := topologyOverlay.Compress(compConfig)
		if err != nil {
			logger.Error("topology compression failed", err)
			return
		}

		logger.Info("topology compressed",
			logging.Field{Key: "mode", Value: compressed.Mode},
			logging.Field{Key: "compressed_kb", Value: compressed.GetSize() / 1024},
			logging.Field{Key: "ratio", Value: compressed.GetRatio()},
			logging.Field{Key: "z_levels", Value: len(compressed.ZLevelsIncluded)})

		// Validate round-trip
		if err := compressed.Validate(topologyOverlay); err != nil {
			logger.Error("topology compression validation failed", err)
		} else {
			logger.Info("topology compression validated (lossless round-trip)")
		}
	})

	// Start client (begins listening for plugin connections)
	if err := client.Start(ctx, cfg.ListenPort); err != nil {
		logger.Error("failed to start client", err)
		os.Exit(1)
	}

	// Create and start HTTP server if enabled
	var httpServer *http.Server
	if cfg.EnableHttpApi {
		httpServer = http.NewServer(logger, client, configMgr)

		// Set hazard metrics callback
		httpServer.SetGetHazardMetrics(func() *http.HazardMetrics {
			if hazardManager == nil {
				return nil
			}
			counts := hazardManager.GetAllCounts()
			return &http.HazardMetrics{
				AquiferCount: counts["aquifer"],
				WaterCount:   counts["water"],
				LavaCount:    counts["lava"],
				CavernCount:  counts["caverns"],
				EnemyCount:   counts["enemies"],
				DwarfCount:   counts["dwarves"],
				MemoryKB:     hazardManager.GetMemoryUsage() / 1024,
			}
		})

		if err := httpServer.Start(ctx); err != nil {
			logger.Error("failed to start HTTP server", err)
			os.Exit(1)
		}
	}

	logger.Info("waiting for DFHack plugin to connect...")
	logger.Info("topology overlay will build automatically when FULL_STATE received")

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Subscribe to tile updates
	updates := client.SubscribeTileUpdates()
	go func() {
		for update := range updates {
			logger.Info("tile update received",
				logging.Field{Key: "changed_tiles", Value: update.Count})

			// Update topology overlay incrementally
			if topologyOverlay != nil && update.Count > 0 {
				updateCount := 0
				for _, tile := range update.Tiles {
					isOpen := topology.IsPathable(tile.TileType)
					if err := topologyOverlay.SetTile(tile.X, tile.Y, tile.Z, isOpen); err != nil {
						logger.Error("failed to update topology tile", err,
							logging.Field{Key: "x", Value: tile.X},
							logging.Field{Key: "y", Value: tile.Y},
							logging.Field{Key: "z", Value: tile.Z})
					} else {
						updateCount++
					}
				}

				if updateCount > 0 {
					logger.Info("topology overlay updated",
						logging.Field{Key: "tiles_updated", Value: updateCount},
						logging.Field{Key: "open_pct", Value: topologyOverlay.GetOpenPercentage()})
				}
			}

			// Update hazard overlays incrementally
			if hazardManager != nil && update.Count > 0 {
				hazardManager.UpdateFromTiles(update.Tiles)
			}
		}
	}()

	// Subscribe to entity updates
	entityUpdates := client.SubscribeEntityUpdates()
	go func() {
		for update := range entityUpdates {
			// Update entity overlays (enemies and dwarves)
			if hazardManager != nil && update.Count > 0 {
				hazardManager.BuildFromEntities(update.Entities)

				// Log entity counts
				counts := hazardManager.GetAllCounts()
				logger.Debug("entity overlays updated",
					logging.Field{Key: "enemy_count", Value: counts["enemies"]},
					logging.Field{Key: "dwarf_count", Value: counts["dwarves"]})
			}
		}
	}()

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
