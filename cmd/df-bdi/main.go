// Command df-bdi is the BDI-architecture entry point for the DF AI
// orchestrator. It replaces the legacy autonomous-loop binary
// (cmd/df-orchestrator). Sharing the substrate packages (dfhack protocol,
// overlays, LLM providers, command executor) but driven by a world model
// + predicate library + plan DAG + reconciler instead of the poll-based
// loop.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/df-ai/orchestrator/internal/bdi"
	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/config"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/llm"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/plan"
	"github.com/df-ai/orchestrator/internal/predicate"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/query"
	"github.com/df-ai/orchestrator/internal/reconcile"
	"github.com/df-ai/orchestrator/internal/skill"
	"github.com/df-ai/orchestrator/internal/topology"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

const Version = "0.3.0-bdi"

// pluginQueryAdapter implements query.PluginDispatcher by forwarding to
// the dfhack client. Lives here (rather than in dfhack/) to keep the
// client package free of any reference to the query types.
type pluginQueryAdapter struct {
	client *dfhack.Client
}

func (a pluginQueryAdapter) Dispatch(ctx context.Context, name, argsJSON string) ([]byte, error) {
	if a.client == nil {
		return nil, fmt.Errorf("dfhack client unavailable")
	}
	return a.client.SendQuery(ctx, name, argsJSON, 5*time.Second)
}

var (
	configPath = flag.String("config", "config/orchestrator.yaml", "path to configuration file")
	cadenceArg = flag.Duration("cadence", 0, "deliberator cadence (overrides config); 0 uses default 45s")
	versionArg = flag.Bool("version", false, "print version and exit")
)

func main() {
	flag.Parse()

	if *versionArg {
		fmt.Printf("df-bdi v%s\n", Version)
		fmt.Printf("Protocol Version: %d\n", protocol.ProtocolVersion)
		os.Exit(0)
	}

	tempLogger := logging.NewTextLogger("info")

	configMgr, err := config.NewConfigManager(*configPath, tempLogger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	cfg := configMgr.Get()

	var logger *logging.Logger
	if cfg.LogFormat == "json" {
		logger = logging.NewJSONLogger(cfg.LogLevel)
	} else {
		logger = logging.NewTextLogger(cfg.LogLevel)
	}

	logger.Info("df-bdi starting",
		logging.Field{Key: "version", Value: Version},
		logging.Field{Key: "protocol_version", Value: protocol.ProtocolVersion},
		logging.Field{Key: "port", Value: cfg.ListenPort})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configMgr.Watch(ctx)

	// Substrate: dfhack client, command executor.
	dfClient := dfhack.NewClient(logger)
	cmdExec := commands.NewCommandExecutor(logger, dfClient, 5*time.Second)

	// World model with nil overlays. The FULL_STATE callback below installs
	// them once the plugin connects.
	wm := worldmodel.New(nil, nil, nil, logger)
	populator := worldmodel.NewPopulator(wm, dfClient, logger)

	// LLM provider.
	llmProvider, err := llm.CreateProvider(cfg)
	if err != nil {
		logger.Error("failed to create LLM provider", err)
		os.Exit(1)
	}
	logger.Info("llm provider ready",
		logging.Field{Key: "provider", Value: llmProvider.GetProviderType()},
		logging.Field{Key: "model", Value: llmProvider.GetModelName()})

	// Predicate library.
	library := predicate.NewStarterLibrary()
	logger.Info("predicate library loaded",
		logging.Field{Key: "count", Value: len(library.All())})

	// Query registry — the deliberator's tool catalog.
	registry := query.NewStarterRegistry()
	logger.Info("query registry loaded",
		logging.Field{Key: "count", Value: len(registry.All())})

	// Skill library — procedural recipes for DF operations.
	skills := skill.NewStarterLibrary()
	logger.Info("skill library loaded",
		logging.Field{Key: "count", Value: len(skills.All())})

	// Plugin dispatcher: forwards Tier 2 queries to the dfhack client.
	pluginDispatcher := pluginQueryAdapter{client: dfClient}

	// Plan DAG + executor.
	dag := plan.New()
	exec := plan.NewExecutor(dag, cmdExec, wm, logger, 500*time.Millisecond)

	// Reconciler.
	reconciler := reconcile.New(reconcile.DefaultConfig(), wm, dag, logger)

	// FULL_STATE handler: build overlays, install on world model, set the
	// modification detector on the populator, and notify populator of the
	// initial tick. Modifications detector cannot be created until we know
	// map dimensions, so it lives in this callback.
	dfClient.SetOnFullState(func(state *protocol.FullStateMessage) {
		buildStart := time.Now()

		topo := topology.NewTopologyOverlay(state.Width, state.Height, state.Depth)
		if topo == nil {
			logger.Error("topology overlay nil — invalid dimensions",
				fmt.Errorf("W=%d H=%d D=%d", state.Width, state.Height, state.Depth))
			return
		}
		if err := topo.BuildFromTiles(state.Tiles); err != nil {
			logger.Error("topology build failed", err)
			return
		}

		hzd := hazards.NewHazardManager(state.Width, state.Height, state.Depth)
		hzd.BuildFromTiles(state.Tiles)

		bounds := modifications.Bounds{
			Width:  state.Width,
			Height: state.Height,
			Depth:  state.Depth,
		}
		mods := modifications.NewModificationOverlay(bounds)
		modDetector := modifications.NewDetector(mods)
		modDetector.InitializeBaseline(state.Tiles)

		wm.SetOverlays(topo, hzd, mods)
		populator.SetModDetector(modDetector)
		populator.OnFullState(state)

		counts := hzd.GetAllCounts()
		logger.Info("worldmodel installed",
			logging.Field{Key: "build_ms", Value: time.Since(buildStart).Milliseconds()},
			logging.Field{Key: "topology_open_pct", Value: topo.GetOpenPercentage()},
			logging.Field{Key: "topology_kb", Value: topo.GetMemoryUsage() / 1024},
			logging.Field{Key: "hazards_aquifer", Value: counts["aquifer"]},
			logging.Field{Key: "hazards_water", Value: counts["water"]},
			logging.Field{Key: "hazards_lava", Value: counts["lava"]},
			logging.Field{Key: "hazards_caverns", Value: counts["caverns"]},
			logging.Field{Key: "modifications_baseline", Value: modDetector.GetBaselineCount()})
	})

	if err := dfClient.Start(ctx, cfg.ListenPort); err != nil {
		logger.Error("failed to start dfhack client", err)
		os.Exit(1)
	}

	// Cadence: CLI > config > default.
	cadence := bdi.DefaultConfig().Cadence
	if *cadenceArg > 0 {
		cadence = *cadenceArg
	} else if cfg.ContextUpdateFrequencySeconds > 0 {
		cadence = time.Duration(cfg.ContextUpdateFrequencySeconds) * time.Second
	}

	loopCfg := bdi.DefaultConfig()
	loopCfg.Cadence = cadence

	loop, err := bdi.NewLoop(loopCfg, wm, populator, dag, exec, reconciler, llmProvider, library, registry, skills, pluginDispatcher, logger)
	if err != nil {
		logger.Error("failed to construct bdi loop", err)
		os.Exit(1)
	}

	loop.Start(ctx)
	logger.Info("bdi loop started",
		logging.Field{Key: "cadence", Value: cadence.String()})

	logger.Info("waiting for DFHack plugin to connect — run `ai-connect` in DFHack console once DF is ready")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	sig := <-sigCh
	logger.Info("shutdown signal received",
		logging.Field{Key: "signal", Value: sig.String()})

	loop.Stop()
	cmdExec.Stop()
	if err := dfClient.Stop(); err != nil {
		logger.Warn("dfhack client stop reported error",
			logging.Field{Key: "error", Value: err.Error()})
	}

	cancel()

	logger.Info("shutdown complete")
}
