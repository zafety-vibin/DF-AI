package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/topology"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

func TestPlaceStore_SaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "places.json")

	ps := NewPlaceStore(path)
	ps.Set(topology.Coord{X: 50, Y: 50, Z: 139}, "the storage hall")
	if err := ps.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded := NewPlaceStore(path)
	if err := loaded.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	all := loaded.All()
	if len(all) != 1 || all[0].Name != "the storage hall" {
		t.Fatalf("expected 1 place named 'the storage hall', got %+v", all)
	}
}

func TestPlaceStore_LoadMissingFileIsNotAnError(t *testing.T) {
	ps := NewPlaceStore(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err := ps.Load(); err != nil {
		t.Fatalf("loading a not-yet-created store must not error, got: %v", err)
	}
	if len(ps.All()) != 0 {
		t.Fatal("expected no places from a missing file")
	}
}

func TestPlaceStore_SaveCreatesParentDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "state")
	path := filepath.Join(dir, "places.json")
	ps := NewPlaceStore(path)
	ps.Set(topology.Coord{X: 1, Y: 1, Z: 1}, "test")
	if err := ps.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

func TestPlaceStore_ReconcileDropsUnresolvableAnchors(t *testing.T) {
	ps := NewPlaceStore(filepath.Join(t.TempDir(), "places.json"))
	ps.Set(topology.Coord{X: 0, Y: 0, Z: 0}, "still there")
	ps.Set(topology.Coord{X: 99, Y: 99, Z: 0}, "walled off")

	topo := topology.NewTopologyOverlay(100, 100, 1)
	_ = topo.SetTileState(0, 0, 0, topology.StateOpen) // only this anchor still resolves
	rg := topology.BuildRegionGraph(topo)

	ps.Reconcile(rg)
	all := ps.All()
	if len(all) != 1 || all[0].Name != "still there" {
		t.Fatalf("expected only 'still there' to survive reconciliation, got %+v", all)
	}
}

func TestResolveAnchor_TileWithNoRegionErrors(t *testing.T) {
	topo := topology.NewTopologyOverlay(10, 10, 1) // nothing dug — no regions
	_, err := resolveAnchor(topo, 5, 5, 0)
	if err == nil {
		t.Fatal("expected an error for a tile with no region (solid rock)")
	}
}

func TestResolveAnchor_OpenTileResolves(t *testing.T) {
	topo := topology.NewTopologyOverlay(10, 10, 1)
	_ = topo.SetTileState(5, 5, 0, topology.StateOpen)
	c, err := resolveAnchor(topo, 5, 5, 0)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if c != (topology.Coord{X: 5, Y: 5, Z: 0}) {
		t.Fatalf("expected the anchor to resolve to itself when open, got %+v", c)
	}
}

// TestResolveAnchorLive_NilLiveFallsBackToOverlayOnly asserts a nil live
// fetcher (the disconnected-bridge case) leaves resolveAnchor's original
// behavior untouched — no panic, same truthful error.
func TestResolveAnchorLive_NilLiveFallsBackToOverlayOnly(t *testing.T) {
	topo := topology.NewTopologyOverlay(10, 10, 1) // nothing seeded, no live fetcher
	if _, err := resolveAnchorLive(context.Background(), topo, nil, 5, 5, 0); err == nil {
		t.Fatal("expected an error: nothing seeded and no live fallback available")
	}
}

// namePlaceTestBridge builds a Bridge around a fresh (unseeded, unless the
// caller seeds it separately) TopologyOverlay, wired with a canned MapSlice
// response via Bridge.mapSliceFn — the seam added specifically because no
// TCP-mocking harness exists for dfhack.Client in this repo (see
// TestLookDescriptionDocumentsPendingBuildingMarker's doc comment in
// tools_percept_test.go). Exec is a zero-value CommandExecutor purely to
// satisfy noExec's non-nil check; name_place never calls any of its methods.
func namePlaceTestBridge(t *testing.T, w, h, d uint16, mapSlice func(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error)) *Bridge {
	t.Helper()
	topo := topology.NewTopologyOverlay(w, h, d)
	if topo == nil {
		t.Fatal("overlay nil")
	}
	logger := logging.NewStderrTextLogger("error")
	b := &Bridge{
		WM:     worldmodel.New(topo, nil, nil, logger),
		Exec:   &commands.CommandExecutor{},
		Places: NewPlaceStore(filepath.Join(t.TempDir(), "places.json")),
		Logger: logger,
	}
	b.mapSliceFn = mapSlice
	return b
}

// TestNamePlace_LiveFallbackResolvesUnseededAnchor is the regression test for
// the confirmed root cause: resolveAnchor's overlay-only lookup trusts a
// TopologyOverlay that's seeded once from FULL_STATE and updated only by the
// TILE_UPDATE stream, which empirically delivers nothing and is wiped on
// every reconnect. Here the overlay is left completely UNSEEDED — the exact
// state a stale/empty overlay leaves behind — while the live map_slice fake
// reports open floor at and around the anchor. name_place must still succeed
// via the live fallback.
func TestNamePlace_LiveFallbackResolvesUnseededAnchor(t *testing.T) {
	rows := make([]string, 11)
	for i := range rows {
		rows[i] = strings.Repeat(".", 11) // open floor everywhere in the sampled window
	}
	canned := &mapview.Slice{Z: 1, X1: 5, Y1: 5, Rows: rows}
	b := namePlaceTestBridge(t, 20, 20, 3, func(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error) {
		return canned, nil
	})

	out := callTool(t, b, "name_place", map[string]any{"x": 10, "y": 10, "z": 1, "name": "the storage hall"})
	if !strings.Contains(out, "SUCCESS") {
		t.Fatalf("expected the live fallback to resolve the unseeded anchor, got: %q", out)
	}
	places := b.Places.All()
	if len(places) != 1 || places[0].Name != "the storage hall" {
		t.Fatalf("expected the place to be persisted, got: %+v", places)
	}
}

// TestNamePlace_LiveFallbackStaysTruthfulWhenSolid asserts the fallback does
// not fabricate success: when live data ALSO shows the anchor as solid rock,
// name_place must still fail with the same truthful "no dug/open region"
// rejection, and nothing gets persisted.
func TestNamePlace_LiveFallbackStaysTruthfulWhenSolid(t *testing.T) {
	rows := make([]string, 11)
	for i := range rows {
		rows[i] = strings.Repeat("#", 11) // solid stone wall everywhere
	}
	canned := &mapview.Slice{Z: 1, X1: 5, Y1: 5, Rows: rows}
	b := namePlaceTestBridge(t, 20, 20, 3, func(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error) {
		return canned, nil
	})

	out := callTool(t, b, "name_place", map[string]any{"x": 10, "y": 10, "z": 1, "name": "should not stick"})
	if !strings.Contains(out, "no dug/open region") {
		t.Fatalf("expected the truthful solid-tile rejection, got: %q", out)
	}
	if len(b.Places.All()) != 0 {
		t.Fatalf("expected nothing persisted when even live data says solid, got: %+v", b.Places.All())
	}
}
