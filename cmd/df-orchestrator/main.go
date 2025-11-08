package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/df-ai/orchestrator/internal/autonomous"
	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/config"
	appcontext "github.com/df-ai/orchestrator/internal/context"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/http"
	"github.com/df-ai/orchestrator/internal/llm"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
	"golang.org/x/sync/errgroup"
)

var (
	topologyOverlay      *topology.TopologyOverlay
	hazardManager        *hazards.HazardManager
	modificationOverlay  *modifications.ModificationOverlay
	modificationDetector *modifications.Detector
	commandExecutor      *commands.CommandExecutor
	contextAssembler     *appcontext.Assembler
	queryExecutor        *appcontext.QueryExecutor
	autonomousLoop       *autonomous.AutonomousLoop
	llmProvider          llm.Provider
)

var (
	configPath = flag.String("config", "config/orchestrator.yaml", "path to configuration file")
	version    = flag.Bool("version", false, "print version and exit")
)

const Version = "0.1.0-001-binary-protocol"

// SystemPrompt is the constant instruction set for the LLM
const SystemPrompt = `You are an AI agent controlling a Dwarf Fortress fort.

Your role is to:
1. Analyze the current fort state (modifications, hazards, dwarves, topology)
2. Review feedback from previous commands (task completion status)
3. Decide on the next action to expand and develop the fort
4. Issue commands to dig tunnels, build structures, or wait for tasks to complete

Fort State Context:
- embark_point: Your starting location (wagon/dwarf centroid, set once at embark, never changes)
  - This is where your dwarves and initial resources are located
  - On first turn, active_region shows 60×60 tile area around embark_point
- active_region: The area currently being worked on (embark area, or modification bounds if fort exists)
- Modifications: player/AI changes to the map (dug tiles, built structures)
  - Empty on first turn - you haven't built anything yet!
- Hazards: aquifers, water, lava, caverns, enemies within active_region
  - Check before digging to avoid disasters
- Chambers: extracted rooms and corridors from modifications (empty until you dig)
- Dwarves: active dwarves and their positions

Starting Strategy (First Turn):
- Check embark_point to see where your dwarves are located
- Survey hazards within active_region (60×60 area around embark)
- Identify safe digging direction (away from aquifers/water/lava)
- Start with small entrance hall (5×10 corridor) near embark_point
- Typical coordinates: embark_point.z is your surface level
- Example: If embark at (72, 89, 155), dig entrance at (72, 80, 155) to (72, 90, 155)

Task Feedback:
- Each previous command is tracked for completion
- You receive progress updates (X/Y tiles complete, status)
- Status: not_started, in_progress, completed, stalled, failed
- Dwarves work at their own pace - tasks may take 1-5 minutes

Available Commands:
1. dig from (x1, y1, z) to (x2, y2, z) - Designate area for mining (single Z-level only, z must equal z)
2. build at (x, y, z) - Place a construction (NOT YET IMPLEMENTED)
3. wait - Observe without acting, let ongoing tasks complete

Guidelines:
- Start digging near embark_point on first turn (check embark_point.z for surface level)
- Avoid digging into aquifers, water, or lava (check hazards array first)
- Wait for tasks to complete before issuing new overlapping commands (check task progress)
- If a task is stalled (no progress 60s), dwarves may be busy or path blocked
- Expand systematically: entrance hall → bedrooms → dining hall → workshops
- Fort development takes time - dwarves aren't instant, be patient

Learning from Outcomes:
- You will receive feedback about what actually happened (success, hazard encounter, failure)
- If you breach aquifer or hit lava, learn from it - adjust future digging strategy
- No actions are blocked - you learn by experiencing consequences

Respond with:
1. Your reasoning (what you observe, why you're acting)
2. The command(s) to execute (dig/wait)

Example first turn response:
"I'm at embark point (72, 89, 155). I see hazards: aquifer 15 tiles north. Safe to dig south.
Starting with entrance hall south of embark to avoid aquifer.

dig from (72, 75, 155) to (82, 85, 155)"`

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

	// Create command executor
	commandExecutor = commands.NewCommandExecutor(logger, client, 5*time.Second)
	logger.Info("command executor initialized",
		logging.Field{Key: "timeout", Value: "5s"})

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

		// Initialize modification tracking overlay
		modBounds := modifications.Bounds{
			Width:  state.Width,
			Height: state.Height,
			Depth:  state.Depth,
		}
		modificationOverlay = modifications.NewModificationOverlay(modBounds)
		modificationDetector = modifications.NewDetector(modificationOverlay)

		// Initialize baseline from full state
		modificationDetector.InitializeBaseline(state.Tiles)

		logger.Info("modification tracking initialized",
			logging.Field{Key: "baseline_tiles", Value: modificationDetector.GetBaselineCount()},
			logging.Field{Key: "memory_kb", Value: modificationOverlay.GetMemoryUsage() / 1024})

		// Initialize context assembler
		contextAssembler = appcontext.NewAssembler()
		queryExecutor = appcontext.NewQueryExecutor(topologyOverlay, hazardManager, modificationOverlay)

		logger.Info("context assembly initialized")

		// Initialize LLM provider
		llmProvider, err = llm.CreateProvider(cfg)
		if err != nil {
			logger.Error("failed to create LLM provider", err)
			logger.Warn("autonomous loop will not be started")
		} else {
			logger.Info("LLM provider initialized",
				logging.Field{Key: "provider_type", Value: llmProvider.GetProviderType()},
				logging.Field{Key: "model", Value: llmProvider.GetModelName()})

			// Initialize autonomous loop
			loopInterval := time.Duration(cfg.ContextUpdateFrequencySeconds) * time.Second
			autonomousLoop = autonomous.NewLoop(
				logger,
				loopInterval,
				modificationOverlay,
				topologyOverlay,
				hazardManager,
				contextAssembler,
				llmProvider,
				commandExecutor,
				SystemPrompt,
			)

			logger.Info("autonomous loop initialized",
				logging.Field{Key: "interval", Value: loopInterval})

			// Start autonomous loop in background
			go func() {
				time.Sleep(5 * time.Second) // Wait for state to stabilize
				autonomousLoop.Start(ctx)
				logger.Info("autonomous loop started")
			}()
		}

		// Test: Generate all 4 context levels and log sizes
		go func() {
			time.Sleep(2 * time.Second) // Wait for state to stabilize

			// Create empty entity list for test (no entities yet)
			entities := appcontext.EntityInfoSlice{}

			// Test Level 0 (text overview)
			ctx0, err := contextAssembler.AssembleContext(
				appcontext.Level0,
				modificationOverlay,
				hazardManager,
				entities,
				topologyOverlay,
			)
			if err == nil {
				logger.Info("context Level 0 generated",
					logging.Field{Key: "size_bytes", Value: ctx0.SizeBytes},
					logging.Field{Key: "budget_bytes", Value: ctx0.BudgetBytes},
					logging.Field{Key: "within_budget", Value: ctx0.WithinBudget},
					logging.Field{Key: "text", Value: ctx0.TextOverview})
			} else {
				logger.Error("failed to generate Level 0 context", err)
			}

			// Test Level 1 (active area)
			ctx1, err := contextAssembler.AssembleContext(
				appcontext.Level1,
				modificationOverlay,
				hazardManager,
				entities,
				topologyOverlay,
			)
			if err == nil {
				logger.Info("context Level 1 generated",
					logging.Field{Key: "size_bytes", Value: ctx1.SizeBytes},
					logging.Field{Key: "budget_bytes", Value: ctx1.BudgetBytes},
					logging.Field{Key: "within_budget", Value: ctx1.WithinBudget},
					logging.Field{Key: "chamber_count", Value: len(ctx1.Chambers)})
			} else {
				logger.Error("failed to generate Level 1 context", err)
			}

			// Test Level 2 (deep planning)
			ctx2, err := contextAssembler.AssembleContext(
				appcontext.Level2,
				modificationOverlay,
				hazardManager,
				entities,
				topologyOverlay,
			)
			if err == nil {
				logger.Info("context Level 2 generated",
					logging.Field{Key: "size_bytes", Value: ctx2.SizeBytes},
					logging.Field{Key: "budget_bytes", Value: ctx2.BudgetBytes},
					logging.Field{Key: "within_budget", Value: ctx2.WithinBudget},
					logging.Field{Key: "chamber_count", Value: len(ctx2.Chambers)})
			} else {
				logger.Error("failed to generate Level 2 context", err)
			}

			// Test Level 3 (full context)
			ctx3, err := contextAssembler.AssembleContext(
				appcontext.Level3,
				modificationOverlay,
				hazardManager,
				entities,
				topologyOverlay,
			)
			if err == nil {
				logger.Info("context Level 3 generated",
					logging.Field{Key: "size_bytes", Value: ctx3.SizeBytes},
					logging.Field{Key: "budget_bytes", Value: ctx3.BudgetBytes},
					logging.Field{Key: "within_budget", Value: ctx3.WithinBudget},
					logging.Field{Key: "chamber_count", Value: len(ctx3.Chambers)})
			} else {
				logger.Error("failed to generate Level 3 context", err)
			}
		}()
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

	// Test command sending after connection established (optional manual test)
	// Uncomment to test dig command when connected:
	// go func() {
	// 	time.Sleep(5 * time.Second) // Wait for connection
	// 	if client.IsConnected() && commandExecutor != nil {
	// 		logger.Info("sending test dig command...")
	// 		result, err := commandExecutor.SendDigCommand(10, 20, 95, 15, 25)
	// 		if err != nil {
	// 			logger.Error("test command failed", err)
	// 		} else {
	// 			logger.Info("test command result",
	// 				logging.Field{Key: "success", Value: result.Success},
	// 				logging.Field{Key: "status", Value: result.Status},
	// 				logging.Field{Key: "error_msg", Value: result.ErrorMsg},
	// 				logging.Field{Key: "duration_ms", Value: result.Duration.Milliseconds()})
	// 		}
	// 	}
	// }()

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

			// Detect modifications from tile updates
			if modificationDetector != nil && update.Count > 0 {
				detectedCount := modificationDetector.DetectModifications(update.Tiles)
				if detectedCount > 0 {
					modCount := modificationOverlay.GetCount()
					bounds := modificationOverlay.GetBounds()

					logger.Info("modifications detected",
						logging.Field{Key: "new_modifications", Value: detectedCount},
						logging.Field{Key: "total_modifications", Value: modCount},
						logging.Field{Key: "bounds_x", Value: fmt.Sprintf("%d-%d", bounds.XMin, bounds.XMax)},
						logging.Field{Key: "bounds_y", Value: fmt.Sprintf("%d-%d", bounds.YMin, bounds.YMax)},
						logging.Field{Key: "bounds_z", Value: fmt.Sprintf("%d-%d", bounds.ZMin, bounds.ZMax)},
						logging.Field{Key: "memory_kb", Value: modificationOverlay.GetMemoryUsage() / 1024})

					// Extract and log chambers
					extractor := modifications.NewChamberExtractor(modificationOverlay)
					chambers := extractor.ExtractChambers()
					if len(chambers) > 0 {
						logger.Info("chambers extracted",
							logging.Field{Key: "chamber_count", Value: len(chambers)})

						// Log details of largest chambers
						sorted := modifications.GetChambersBySize(chambers)
						for i, chamber := range sorted {
							if i >= 5 { // Only log top 5 chambers
								break
							}
							desc := modifications.GenerateChamberDescription(chamber)
							logger.Info("chamber details",
								logging.Field{Key: "chamber_id", Value: chamber.ID},
								logging.Field{Key: "description", Value: desc})
						}
					}
				}
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

	// Shutdown autonomous loop
	g.Go(func() error {
		if autonomousLoop != nil {
			autonomousLoop.Stop()
		}
		return nil
	})

	// Shutdown command executor
	g.Go(func() error {
		if commandExecutor != nil {
			commandExecutor.Stop()
		}
		return nil
	})

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
