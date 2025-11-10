package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/df-ai/orchestrator/internal/agents"
	"github.com/df-ai/orchestrator/internal/autonomous"
	"github.com/df-ai/orchestrator/internal/blueprints"
	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/config"
	appcontext "github.com/df-ai/orchestrator/internal/context"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/http"
	"github.com/df-ai/orchestrator/internal/llm"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/phases"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/spatial"
	"github.com/df-ai/orchestrator/internal/topology"
	"github.com/df-ai/orchestrator/internal/zones"
	"golang.org/x/sync/errgroup"
)

var (
	topologyOverlay      *topology.TopologyOverlay
	hazardManager        *hazards.HazardManager
	modificationOverlay  *modifications.ModificationOverlay
	modificationDetector *modifications.Detector
	modBounds            modifications.Bounds // Map dimensions for persistence
	commandExecutor      *commands.CommandExecutor
	contextAssembler     *appcontext.Assembler
	queryExecutor        *appcontext.QueryExecutor
	autonomousLoop       *autonomous.AutonomousLoop
	llmProvider          llm.Provider
	phaseManager         *phases.PhaseManager // Fort development phase tracking
	firstEntityUpdate    bool = true          // Track if we should trigger first AI cycle
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
- topology_slice: Terrain map showing SOLID ROCK vs ALREADY DUG (first turn only, 60×60 area)
  - open_tiles: Array of "X,Y" coordinates that are ALREADY DUG/OPEN (floors, air, passable space)
  - CRITICAL: open_tiles = ALREADY PASSABLE = DO NOT DIG THESE COORDINATES
  - TO DIG: Choose coordinates NOT in open_tiles (those are SOLID ROCK walls that CAN be mined)
  - REVERSED LOGIC: If tile is "open", it means ALREADY EXCAVATED, don't dig it again!
  - Example: If open_tiles contains "45,35", that coordinate is ALREADY A FLOOR - dig somewhere else!
  - Mining targets: Pick coordinates where there is NO open_tile entry (those are solid walls/rock)

Spatial Validator Planner (SVP) Guidance:
- IF SVP has analyzed terrain, you will receive designated Z-levels in fort metrics:
  - SVPHousingZ: Designated Z-level for bedrooms (typically embark - 5)
  - SVPWorkshopZ: Designated Z-level for workshops (typically housing - 1)
  - SVPFarmZ: Array of Z-levels with soil for farming
- CRITICAL: If you are at a different Z-level than SVP designations, you MUST dig stairs to reach them!
  - Example: Dwarves at Z=130, SVPHousingZ=125 → Dig DOWN stairs from Z=130 to Z=125
  - Use "dig stairs" command to create vertical shafts (up/down stairs)
  - Create 3×3 stairwell at current location, then extend downward each level
- Strategic approach:
  1. Dig entrance/gathering area at current dwarf Z-level (embark surface)
  2. Dig stairs DOWN to SVPHousingZ (dig stairs from current Z, repeat at each level going down)
  3. Once at SVPHousingZ, dig bedrooms horizontally
  4. Continue stairs to SVPWorkshopZ for workshops
- NEVER dig bedrooms at embark Z if SVP says housing should be elsewhere!

Starting Strategy (First Turn):
- Check embark_point to see where your dwarves are located
- Check dwarves array to see their ACTUAL Z-level (embark_point.z is average, dwarves may be higher/lower)
- Check if SVP designations exist (SVPHousingZ, SVPWorkshopZ fields in metrics)
- If SVP exists: Plan vertical shaft to reach designated layers
- If no SVP: Dig at the SAME Z-level where most dwarves are clustered (check dwarves[].z)
- Survey hazards within active_region (60×60 area around embark)
- Identify safe digging direction (away from aquifers/water/lava)
- Start with small entrance hall (5×10 corridor) adjacent to where dwarves are standing
- Then dig stairs DOWN if SVP designations are below current Z
- CRITICAL: Dwarves must be able to WALK to the dig site - don't dig isolated rooms on different Z-levels!
- Example: If dwarves at (72, 89, 155), dig entrance at (72, 80, 155) to (72, 90, 155) - SAME Z=155

Digging Commands:
Standard mining:
  dig from (x1, y1, z) to (x2, y2, z)
  - Removes walls, creates passable floor
  - Use for horizontal expansion on same Z-level

Stairs (connecting Z-levels vertically):
  dig stairs from (x1, y1, z) to (x2, y2, z)
  - Creates up/down staircases that connect to levels above AND below
  - Essential for multi-level forts - use these to go up or down
  - Example: "dig stairs from (50,50,120) to (52,52,120)" creates 3x3 stairwell

Channels (digging down one level):
  dig channel from (x1, y1, z) to (x2, y2, z)
  - Removes floor, creates hole to level below
  - Useful for creating openings between floors
  - WARNING: Don't channel where dwarves are standing!

Ramps (sloped access):
  dig ramp from (x1, y1, z) to (x2, y2, z)
  - Creates sloped passage to level above
  - Alternative to stairs for wagons/vehicles
  - Smoother access but takes more space

Staircase Strategy:
- First turn: Dig entrance hall on dwarf Z-level
- Second turn: Add stairs to connect to level below for expansion
- Build vertically: Stairs at (x,y,z) connect to (x,y,z-1) and (x,y,z+1)
- Always leave path from stairs to work areas

Task Feedback:
- Each previous command is tracked for completion
- You receive progress updates (X/Y tiles complete, status)
- Status: not_started, in_progress, completed, stalled, failed
- Dwarves work at their own pace - tasks may take 1-5 minutes

Available Commands:
1. dig [type] from (x1, y1, z) to (x2, y2, z)
   Types: (default), stairs, channel, ramp, upstair, downstair
2. chop from (x1, y1, z) to (x2, y2, z) - Designate trees for chopping
3. gather from (x1, y1, z) to (x2, y2, z) - Designate plants for gathering
4. wait - Observe without acting, let ongoing tasks complete

Blueprints (Templates):
- Available blueprints are listed in the "blueprints" array in context
- Blueprints are SUGGESTIONS, not requirements - feel free to design your own layouts
- Use blueprints to learn patterns, then create unique variations
- Each fort should be unique to the world, not copy-pasted templates
- Example: "entrance_hall_10x10" shows a standard entrance with stairs in corner
  - You can use it as-is, modify it, or ignore it and design from scratch
- Blueprints help you learn good practices (stairs placement, room proportions)
  - But adapt to terrain, hazards, and your specific fort needs

Guidelines:
- Start digging near embark_point on first turn (check embark_point.z for surface level)
- Avoid digging into aquifers, water, or lava (check hazards array first)
- Use chop to designate trees for wood (essential for beds, barrels, bins)
- Use gather to collect surface plants (food and brewing materials)
- Wait for tasks to complete before issuing new overlapping commands (check task progress)
- If a task is stalled (no progress 60s), dwarves may be busy or path blocked
- Expand systematically: entrance hall → bedrooms → dining hall → workshops
- Gather food/wood early before digging deep (surface resources are safe)
- Fort development takes time - dwarves aren't instant, be patient

Learning from Outcomes:
- You will receive feedback about what actually happened (success, hazard encounter, failure)
- If you breach aquifer or hit lava, learn from it - adjust future digging strategy
- No actions are blocked - you learn by experiencing consequences

Multi-Command Strategy:
- You can issue MULTIPLE commands in one response for connected work
- Each command executes in parallel (dwarves work on all simultaneously)
- Example: Dig entrance + add stairs + gather food all at once
- Connected designs: Place stairs in corner of entrance hall for vertical expansion

Respond with:
1. Your reasoning (what you observe, why you're acting)
2. Multiple commands if doing connected work (entrance + stairs, or entrance + food gathering)

Example multi-command response:
"I'm at embark point (72, 89, 130) on Z=130. Dwarves are clustered here. I'll dig an entrance hall
with stairs in the corner for future vertical expansion, and gather surface plants for food:

dig from (70, 85, 130) to (80, 95, 130)
dig stairs from (78, 93, 130) to (80, 95, 130)
gather from (60, 80, 130) to (85, 100, 130)

This creates entrance (10x10), stairs in SE corner, and food collection simultaneously."

Example single command:
"Entrance hall in progress (23/100 tiles). Waiting for completion before adding stairs.

wait"`

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

	// Set up save request callback (from ai-save command)
	client.SetOnSaveRequest(func() {
		if modificationOverlay == nil || !cfg.EnablePersistence {
			logger.Warn("save requested but persistence not enabled")
			return
		}

		// Create persistent wrapper and save
		fortName := fmt.Sprintf("fort_%d", time.Now().Unix())
		persistent := modifications.NewPersistentModifications(
			modBounds,
			cfg.PersistencePath,
			time.Duration(cfg.AutoSaveIntervalSec)*time.Second,
			cfg.EnablePersistence,
		)

		// Copy modifications
		for coord, info := range modificationOverlay.GetAll() {
			persistent.Add(coord, info)
		}

		if err := persistent.Save(fortName); err != nil {
			logger.Error("save failed", err)
		} else {
			logger.Info("modifications saved",
				logging.Field{Key: "fort", Value: fortName},
				logging.Field{Key: "count", Value: modificationOverlay.GetCount()})
		}
	})

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
		modBounds = modifications.Bounds{
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

		// Feature 007: Notify autonomous loop that topology received
		if autonomousLoop != nil {
			autonomousLoop.OnTopologyReceived()
		}

		// Initialize context assembler
		contextAssembler = appcontext.NewAssembler()

		// Enable room detection if configured
		if cfg.EnableRoomDetection {
			contextAssembler.EnableRoomDetection(cfg.RoomMinSize, cfg.RoomMaxSize)
			logger.Info("room detection enabled",
				logging.Field{Key: "min_size", Value: cfg.RoomMinSize},
				logging.Field{Key: "max_size", Value: cfg.RoomMaxSize})
		}

		// Load blueprint library
		blueprintLib := blueprints.NewBlueprintLibrary("blueprints")
		blueprintList := blueprintLib.ListBlueprints()
		if len(blueprintList) > 0 {
			// Convert to BlueprintInfo for context
			bpInfos := make([]appcontext.BlueprintInfo, 0, len(blueprintList))
			for _, name := range blueprintList {
				bp := blueprintLib.GetBlueprint(name)
				if bp != nil {
					bpInfos = append(bpInfos, appcontext.BlueprintInfo{
						Name:       name,
						Description: bp.Description,
						Dimensions: fmt.Sprintf("%dx%dx%d", bp.Width, bp.Height, bp.Depth),
						TileCount:  len(bp.Digs),
						Tags:       bp.Tags,
					})
				}
			}
			contextAssembler.SetBlueprints(bpInfos)
			logger.Info("blueprints loaded",
				logging.Field{Key: "count", Value: len(blueprintList)})
		}

		queryExecutor = appcontext.NewQueryExecutor(topologyOverlay, hazardManager, modificationOverlay)

		logger.Info("context assembly initialized")

		// Initialize phase manager
		phaseConfig := phases.PhaseConfig{
			EmbarkDays:    cfg.PhaseEmbarkDays,
			EstablishDays: cfg.PhaseEstablishDays,
			ExpandDays:    cfg.PhaseExpandDays,
		}
		phaseManager = phases.NewPhaseManager(phaseConfig, cfg.EnablePhaseSystem)

		logger.Info("phase manager initialized",
			logging.Field{Key: "enabled", Value: cfg.EnablePhaseSystem},
			logging.Field{Key: "embark_days", Value: cfg.PhaseEmbarkDays})

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
				client,
				modificationOverlay,
				topologyOverlay,
				hazardManager,
				contextAssembler,
				llmProvider,
				commandExecutor,
				SystemPrompt,
				phaseManager, // Pass phase manager for adaptive prompts
			)

			logger.Info("autonomous loop initialized",
				logging.Field{Key: "interval", Value: loopInterval})

			// Initialize goal agents if enabled (Feature 006)
			if cfg.EnableGoalAgents {
				agentRegistry := agents.NewAgentRegistry()

				// Register FoodSecurityAgent
				if cfg.AgentFood.Enabled {
					foodAgent := agents.NewFoodSecurityAgent(cfg.AgentFood)
					agentRegistry.Register(foodAgent)
					logger.Info("registered agent", logging.Field{Key: "agent", Value: foodAgent.Name()}, logging.Field{Key: "priority", Value: foodAgent.Priority()})
				}

				// Register HousingAgent
				if cfg.AgentHousing.Enabled {
					housingAgent := agents.NewHousingAgent(cfg.AgentHousing)
					agentRegistry.Register(housingAgent)
					logger.Info("registered agent", logging.Field{Key: "agent", Value: housingAgent.Name()}, logging.Field{Key: "priority", Value: housingAgent.Priority()})
				}

				// Register MiningAgent
				if cfg.AgentMining.Enabled {
					miningAgent := agents.NewMiningAgent(cfg.AgentMining)
					agentRegistry.Register(miningAgent)
					logger.Info("registered agent", logging.Field{Key: "agent", Value: miningAgent.Name()}, logging.Field{Key: "priority", Value: miningAgent.Priority()})
				}

				// Register WealthAgent
				if cfg.AgentWealth.Enabled {
					wealthAgent := agents.NewWealthAgent(cfg.AgentWealth)
					agentRegistry.Register(wealthAgent)
					logger.Info("registered agent", logging.Field{Key: "agent", Value: wealthAgent.Name()}, logging.Field{Key: "priority", Value: wealthAgent.Priority()})
				}

				// Register DefenseAgent
				if cfg.AgentDefense.Enabled {
					defenseAgent := agents.NewDefenseAgent(cfg.AgentDefense)
					agentRegistry.Register(defenseAgent)
					logger.Info("registered agent", logging.Field{Key: "agent", Value: defenseAgent.Name()})
				}

				// Set registry on autonomous loop
				autonomousLoop.SetAgentRegistry(agentRegistry, true)
				logger.Info("goal agents enabled", logging.Field{Key: "agent_count", Value: len(agentRegistry.List())})

				// Feature 007: Configure intent-based planning (HRM architecture)
				autonomousLoop.SetUseIntentPlanning(cfg.UseIntentPlanning)
			} else {
				logger.Info("goal agents disabled - using direct-LLM mode (Feature 005 behavior)")
			}

			// Feature 007: Initialize SVP if enabled
			if cfg.UseSVP {
				// Fort name will be set when we receive fort info
				// For now use placeholder - will be updated when fort info available
				fortName := "unknown_fort"
				svp := spatial.NewSpatialValidatorPlanner(fortName, cfg.SVPPersistenceDir, logger)

				// Try to load existing layout
				if err := svp.Load(fortName); err != nil {
					logger.Debug("SVP: No existing layout found, will analyze on first connection",
						logging.Field{Key: "error", Value: err.Error()})
				} else {
					logger.Info("SVP: Loaded existing layout from disk")
				}

				autonomousLoop.SetSVP(svp)
				logger.Info("SVP enabled", logging.Field{Key: "persistence_dir", Value: cfg.SVPPersistenceDir})
			} else {
				logger.Info("SVP disabled")
			}

			// Feature 007: Initialize zone extractor if enabled
			if cfg.ExtractZones {
				zoneExtractor := zones.NewZoneExtractor(logger, true)
				autonomousLoop.SetZoneExtractor(zoneExtractor)
				logger.Info("zone extraction enabled",
					logging.Field{Key: "interval_ms", Value: cfg.ZoneExtractionIntervalMS})
			} else {
				logger.Info("zone extraction disabled")
			}

			// Feature 007: Load blueprint metadata if enabled (T066)
			if cfg.IncludeBlueprintMetadata && cfg.BlueprintDirectory != "" {
				metadata, err := blueprints.LoadMetadata(cfg.BlueprintDirectory, logger)
				if err != nil {
					logger.Warn("failed to load blueprint metadata",
						logging.Field{Key: "error", Value: err.Error()})
				} else {
					autonomousLoop.SetBlueprintMetadata(metadata)
					logger.Info("blueprint metadata loaded for arbiter",
						logging.Field{Key: "directory", Value: cfg.BlueprintDirectory})
				}
			} else {
				logger.Info("blueprint metadata disabled")
			}

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

		// Set AI history callback
		httpServer.SetGetAIHistory(func() interface{} {
			if autonomousLoop == nil {
				return nil
			}
			history := autonomousLoop.GetHistory()
			return map[string]interface{}{
				"turns":       history.GetRecentTurns(10), // Last 10 turns
				"total_turns": history.GetTurnCount(),
				"in_memory":   history.GetHistorySize(),
			}
		})

		// Set save modifications callback
		httpServer.SetSaveModifications(func() error {
			if modificationOverlay == nil || !cfg.EnablePersistence {
				return fmt.Errorf("persistence not enabled")
			}

			// Create persistent wrapper and save
			// TODO: Get actual fort name from DF (for now use timestamp)
			fortName := fmt.Sprintf("fort_%d", time.Now().Unix())

			persistent := modifications.NewPersistentModifications(
				modBounds, // Use global map bounds
				cfg.PersistencePath,
				time.Duration(cfg.AutoSaveIntervalSec)*time.Second,
				cfg.EnablePersistence,
			)

			// Copy current modifications to persistent overlay
			// TODO: This is a workaround - should use PersistentModifications from start
			for coord, info := range modificationOverlay.GetAll() {
				persistent.Add(coord, info)
			}

			return persistent.Save(fortName)
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
	// 		result, err := commandExecutor.SendDigCommand(protocol.DigTypeDefault, 10, 20, 95, 15, 25)
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
				// CRITICAL: Cache entities FIRST before triggering AI cycle
				// Update autonomous loop entity cache
				if autonomousLoop != nil {
					autonomousLoop.UpdateEntities(update.Entities)

					// Feature 007: Cache zone data from ENTITY_UPDATE
					if len(update.Zones) > 0 {
						autonomousLoop.UpdateZones(update.Zones)
						logger.Debug("zones cached from entity update",
							logging.Field{Key: "zone_count", Value: len(update.Zones)})
					}

					// Feature 007: Notify SVP that entities received
					autonomousLoop.OnEntitiesReceived()
				}

				hazardManager.BuildFromEntities(update.Entities)

				// Log entity counts
				counts := hazardManager.GetAllCounts()
				logger.Debug("entity overlays updated",
					logging.Field{Key: "enemy_count", Value: counts["enemies"]},
					logging.Field{Key: "dwarf_count", Value: counts["dwarves"]})

				// Update phase manager with REAL DF days (if available)
				if phaseManager != nil && update.FortInfo != nil {
					phaseManager.UpdateFromFortInfo(int(update.FortInfo.DaysElapsed))
					logger.Debug("phase manager updated",
						logging.Field{Key: "days_elapsed", Value: update.FortInfo.DaysElapsed},
						logging.Field{Key: "current_phase", Value: phaseManager.GetPhase().String()},
						logging.Field{Key: "wealth", Value: update.FortInfo.CreatedWealth})
				}

				// Trigger first AI cycle immediately AFTER entity cache updated
				if firstEntityUpdate && autonomousLoop != nil {
					firstEntityUpdate = false
					logger.Info("initial entity data received - triggering first AI decision")
					// Small delay to ensure cache is written
					time.Sleep(100 * time.Millisecond)
					autonomousLoop.TriggerImmediate()
				}
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
