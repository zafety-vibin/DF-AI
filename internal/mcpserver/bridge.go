package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/config"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/predicate"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// Bridge owns the game connection: dfhack client, command executor,
// world model + overlays, predicate library. One Bridge per server.
type Bridge struct {
	Client    *dfhack.Client
	Exec      *commands.CommandExecutor
	WM        *worldmodel.WorldModel
	Populator *worldmodel.Populator
	Preds     *predicate.Library
	Logger    *logging.Logger
	port      uint16
}

func NewBridge(cfgPath string) (*Bridge, error) {
	logger := logging.NewTextLogger("info") // stderr; stdout is MCP protocol
	configMgr, err := config.NewConfigManager(cfgPath, logger)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	cfg := configMgr.Get()

	client := dfhack.NewClient(logger)
	exec := commands.NewCommandExecutor(logger, client, 30*time.Second)
	wm := worldmodel.New(nil, nil, nil, logger)
	populator := worldmodel.NewPopulator(wm, client, logger)

	b := &Bridge{
		Client: client, Exec: exec, WM: wm, Populator: populator,
		Preds: predicate.NewStarterLibrary(), Logger: logger,
		port: cfg.ListenPort,
	}

	client.SetOnFullState(func(state *protocol.FullStateMessage) {
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
		bounds := modifications.Bounds{Width: state.Width, Height: state.Height, Depth: state.Depth}
		mods := modifications.NewModificationOverlay(bounds)
		det := modifications.NewDetector(mods)
		det.InitializeBaseline(state.Tiles)
		wm.SetOverlays(topo, hzd, mods)
		populator.SetModDetector(det)
		populator.OnFullState(state)
		logger.Info("worldmodel installed")
	})
	return b, nil
}

func (b *Bridge) Start(ctx context.Context) error {
	if err := b.Client.Start(ctx, b.port); err != nil {
		return err
	}
	go b.Populator.Run(ctx)
	return nil
}

func (b *Bridge) Stop() {
	b.Exec.Stop()
	_ = b.Client.Stop()
}

// The accessors below are nil-safe: New() documents that the bridge may be
// nil during scaffolding, and the server tests exercise that path.

func (b *Bridge) Connected() bool {
	return b != nil && b.Client != nil && b.Client.IsConnected()
}

func (b *Bridge) Snapshot() worldmodel.Snapshot {
	if b == nil || b.WM == nil {
		return worldmodel.Snapshot{}
	}
	return b.WM.Snapshot()
}

// Topo returns the topology overlay, nil until the first FULL_STATE
// arrives. Every consumer must nil-check.
func (b *Bridge) Topo() *topology.TopologyOverlay {
	if b == nil || b.WM == nil {
		return nil
	}
	return b.WM.Observed.Topology
}

// Query forwards a named JSON query to the plugin.
func (b *Bridge) Query(ctx context.Context, name, argsJSON string) ([]byte, error) {
	if !b.Connected() {
		return nil, fmt.Errorf("plugin not connected")
	}
	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return b.Client.SendQuery(qctx, name, argsJSON, 10*time.Second)
}

func (b *Bridge) StatusLine(ctx context.Context) string {
	if b == nil {
		return renderDashboard(worldmodel.Snapshot{}, false, "")
	}
	// Empty string = pause state unknown (query failed or timed out);
	// renderDashboard keeps paused=unknown rather than asserting false.
	sim := ""
	if raw, err := b.Query(ctx, "sim_status", "{}"); err == nil {
		sim = string(raw)
	}
	return renderDashboard(b.Snapshot(), b.Connected(), sim)
}
