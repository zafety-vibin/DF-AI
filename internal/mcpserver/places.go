package mcpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/df-ai/orchestrator/internal/topology"
)

// Place is one named region, keyed by a stable anchor tile rather than a
// region ID — region IDs are recomputed fresh on every RegionGraph
// rebuild and are not stable across process restarts; the anchor tile
// is (the player-chosen tile the name was attached to).
type Place struct {
	Anchor topology.Coord `json:"anchor"`
	Name   string         `json:"name"`
}

// PlaceStore holds named places in memory and persists them to a JSON
// file. Lives under fortress/state/ (NOT fortress/memory/ — that
// directory's charter is AI-authored narrative memory per
// fortress/CLAUDE.md; a tool-write target is a different kind of thing
// and gets its own directory), so it survives the df-mcp restart pattern
// this project's live sessions hit repeatedly (three times in one
// session during the pre-009 fix waves).
type PlaceStore struct {
	mu    sync.RWMutex
	path  string
	items map[topology.Coord]string // anchor -> name
}

func NewPlaceStore(path string) *PlaceStore {
	return &PlaceStore{path: path, items: make(map[topology.Coord]string)}
}

// Load reads the persisted places. A missing file is not an error — a
// fresh fort or a fresh checkout simply has no named places yet.
func (ps *PlaceStore) Load() error {
	data, err := os.ReadFile(ps.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var places []Place
	if err := json.Unmarshal(data, &places); err != nil {
		return err
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.items = make(map[topology.Coord]string, len(places))
	for _, p := range places {
		ps.items[p.Anchor] = p.Name
	}
	return nil
}

// Save writes the current places, creating the parent directory if
// needed (fortress/state/ may not exist yet on a fresh checkout).
func (ps *PlaceStore) Save() error {
	ps.mu.RLock()
	places := ps.allLocked()
	ps.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(ps.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(places, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ps.path, data, 0o644)
}

func (ps *PlaceStore) Set(anchor topology.Coord, name string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.items[anchor] = name
}

func (ps *PlaceStore) All() []Place {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.allLocked()
}

func (ps *PlaceStore) allLocked() []Place {
	out := make([]Place, 0, len(ps.items))
	for anchor, name := range ps.items {
		out = append(out, Place{Anchor: anchor, Name: name})
	}
	return out
}

// Reconcile drops any place whose anchor tile no longer resolves to a
// region in rg (e.g. walled off, merged, or the fort simply hasn't
// finished loading topology yet) — silently, per the design doc: a
// dropped name is not worth erroring over, and the model can re-name
// the area if it still cares.
func (ps *PlaceStore) Reconcile(rg topology.RegionGraph) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for anchor := range ps.items {
		if _, ok := rg.RegionAt(anchor); !ok {
			delete(ps.items, anchor)
		}
	}
}
