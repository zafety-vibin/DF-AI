package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/df-ai/orchestrator/internal/blueprints"
	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/config"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/predicate"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// Bridge owns the game connection: dfhack client, command executor,
// world model + overlays, predicate library. One Bridge per server.
type Bridge struct {
	Client     *dfhack.Client
	Exec       *commands.CommandExecutor
	WM         *worldmodel.WorldModel
	Populator  *worldmodel.Populator
	Preds      *predicate.Library
	Blueprints *blueprints.BlueprintLibrary
	Places     *PlaceStore
	// AlertsPath is where alert DISMISSALS are persisted, alongside
	// Places' own file and for the same reason: a df-mcp process restart
	// otherwise loses the model's record of what it already handled, and
	// the plugin replays DF's whole announcement backlog on every
	// reconnect. Empty disables persistence (a Bridge built outside
	// NewBridge, e.g. in tests).
	AlertsPath string
	Logger     *logging.Logger
	Digs       *pendingDigs // session-scoped ACKed dig rects (see digrects.go)
	port       uint16
	// mapSliceFn, when non-nil, replaces MapSlice's real plugin round-trip
	// below with a canned fetch. The only seam of its kind in Bridge: no
	// TCP-mocking harness exists for dfhack.Client (see
	// TestLookDescriptionDocumentsPendingBuildingMarker's doc comment in
	// tools_percept_test.go), but places.go's resolveAnchorLive needs a way
	// to exercise a live map_slice fallback in tests without a real
	// connection. Always nil on a Bridge built by NewBridge.
	mapSliceFn func(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error)
	// queryFn, when non-nil, replaces Query's real plugin round-trip below
	// with a canned response — same rationale and shape as mapSliceFn
	// above. Added so liveDugTileCount's multi-z fort_footprint +
	// region_scan pipeline (live_state.go) can be exercised end-to-end in
	// tests without a live connection. Always nil on a Bridge built by
	// NewBridge.
	queryFn func(ctx context.Context, name, argsJSON string) ([]byte, error)
}

func NewBridge(cfgPath string) (*Bridge, error) {
	logger := logging.NewStderrTextLogger("info") // stdout is the MCP protocol channel
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
		Preds:      predicate.NewStarterLibrary(),
		Blueprints: blueprints.NewBlueprintLibrary("blueprints"),
		Places:     NewPlaceStore(filepath.Join("fortress", "state", "places.json")),
		AlertsPath: filepath.Join("fortress", "state", "alerts.json"),
		Logger:     logger,
		Digs:       &pendingDigs{},
		port:       cfg.ListenPort,
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
		if err := b.Places.Load(); err != nil {
			logger.Error("places: load failed", err)
		} else {
			b.Places.Reconcile(topology.BuildRegionGraph(topo))
		}
		// Same reconnect hook, same reason as Places: restore what the model
		// had already dismissed before the plugin replays its whole
		// announcement backlog. The world-identity check happens later, on
		// the first ENTITY_UPDATE (Populator) — FULL_STATE carries no world
		// identity to check against.
		if b.AlertsPath != "" && wm.Observed.Alerts != nil {
			if err := wm.Observed.Alerts.Load(b.AlertsPath); err != nil {
				logger.Error("alerts: load failed", err)
			}
		}
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

// Mods returns the modification overlay, nil until the first FULL_STATE
// arrives. Every consumer must nil-check (mirrors Topo()).
func (b *Bridge) Mods() *modifications.ModificationOverlay {
	if b == nil || b.WM == nil {
		return nil
	}
	return b.WM.Observed.Modifications
}

// persistAlertDismissals writes the current dismissal set to disk so it
// survives a df-mcp restart (the plugin replays DF's whole announcement
// backlog on every reconnect, so without this the model re-reads everything
// it already handled). Returns "" on success or when persistence is
// disabled, and a truthful suffix to append to the tool's ACK when the write
// fails — a silent failure here would look exactly like success until the
// next restart.
func (b *Bridge) persistAlertDismissals() string {
	if b == nil || b.AlertsPath == "" || b.WM == nil || b.WM.Observed.Alerts == nil {
		return ""
	}
	// World identity goes through Snapshot (not a bare Observed.World read):
	// the Populator writes that field under the world model's own lock.
	if err := b.WM.Observed.Alerts.Save(b.AlertsPath, b.Snapshot().World); err != nil {
		return fmt.Sprintf(" (WARNING: not persisted — %v; these will reappear if df-mcp restarts)", err)
	}
	return ""
}

// Query forwards a named JSON query to the plugin.
func (b *Bridge) Query(ctx context.Context, name, argsJSON string) ([]byte, error) {
	if b != nil && b.queryFn != nil {
		return b.queryFn(ctx, name, argsJSON)
	}
	if !b.Connected() {
		return nil, fmt.Errorf("plugin not connected")
	}
	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return b.Client.SendQuery(qctx, name, argsJSON, 10*time.Second)
}

// MapSlice / ColumnProfile implement mapview.SliceProvider over the
// plugin query channel.
func (b *Bridge) MapSlice(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error) {
	if b != nil && b.mapSliceFn != nil {
		return b.mapSliceFn(ctx, x1, y1, z, x2, y2)
	}
	args := fmt.Sprintf(`{"x1":%d,"y1":%d,"z":%d,"x2":%d,"y2":%d}`, x1, y1, z, x2, y2)
	raw, err := b.Query(ctx, "map_slice", args)
	if err != nil {
		return nil, err
	}
	return mapview.DecodeSlice(raw)
}

func (b *Bridge) ColumnProfile(ctx context.Context, x, y, zTop, zBottom int16) (*mapview.ColumnProfile, error) {
	args := fmt.Sprintf(`{"x":%d,"y":%d,"z_top":%d,"z_bottom":%d}`, x, y, zTop, zBottom)
	raw, err := b.Query(ctx, "column_profile", args)
	if err != nil {
		return nil, err
	}
	return mapview.DecodeColumnProfile(raw)
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
