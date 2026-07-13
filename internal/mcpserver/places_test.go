package mcpserver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/df-ai/orchestrator/internal/topology"
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
