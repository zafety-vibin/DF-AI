package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/topology"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

// resolveAnchor finds the region containing (x,y,z) and returns its
// anchor coordinate — for name_place, the anchor IS the tile the model
// pointed at (not the region's own min-corner or centroid), since that's
// the stable, model-chosen reference point per the design doc.
func resolveAnchor(topo *topology.TopologyOverlay, x, y, z int16) (topology.Coord, error) {
	rg := topology.BuildRegionGraph(topo)
	c := topology.Coord{X: x, Y: y, Z: z}
	if _, ok := rg.RegionAt(c); !ok {
		return topology.Coord{}, fmt.Errorf("no dug/open region at (%d,%d,%d) — pick a tile inside carved space", x, y, z)
	}
	return c, nil
}

// anchorLiveFetcher is the minimal live-query capability resolveAnchorLive's
// overlay-fallback needs — MapSlice alone, a subset of mapview.SliceProvider
// (which Bridge already implements; see the compile-time check in
// tools_percept_test.go). A narrow interface here, rather than *Bridge
// directly, is what lets a test fake the live plugin round-trip with a
// canned Slice.
type anchorLiveFetcher interface {
	MapSlice(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error)
}

// anchorLiveWindow is how far past the anchor tile resolveAnchorLive samples
// on each side when it falls back to a live query. A single reseeded tile
// would already resolve (RegionAt treats a lone open tile as its own
// region), but sampling a small neighborhood makes the reseed connect into
// whatever's actually around the anchor rather than leaving it an isolated
// speck.
const anchorLiveWindow = 5

// resolveAnchorLive is resolveAnchor with a live-query fallback. The
// overlay-based lookup (resolveAnchor) trusts topology.BuildRegionGraph over
// a TopologyOverlay that is seeded once from FULL_STATE and updated only by
// the TILE_UPDATE stream — which empirically delivers nothing and is wiped
// on every reconnect, so the overlay can say "no region here" for tiles the
// live game state shows are perfectly good open floor. When that happens,
// this queries the same live map_slice channel `look` uses on every call
// and, if the live data shows the anchor open, seeds the overlay from it and
// rebuilds the region graph before giving up. live may be nil (e.g. a
// disconnected bridge); the overlay-only result is then returned unchanged.
// If live data ALSO fails to resolve a region at the anchor, the original
// overlay error is returned as-is — a second, redundant error would be less
// truthful, not more.
func resolveAnchorLive(ctx context.Context, topo *topology.TopologyOverlay, live anchorLiveFetcher, x, y, z int16) (topology.Coord, error) {
	c, err := resolveAnchor(topo, x, y, z)
	if err == nil || live == nil {
		return c, err
	}

	w, h, _ := topo.GetDimensions()
	x1, y1 := clampAnchorLow(x-anchorLiveWindow), clampAnchorLow(y-anchorLiveWindow)
	x2, y2 := clampAnchorHigh(x+anchorLiveWindow, int16(w)-1), clampAnchorHigh(y+anchorLiveWindow, int16(h)-1)
	s, liveErr := live.MapSlice(ctx, x1, y1, z, x2, y2)
	if liveErr != nil {
		return topology.Coord{}, err
	}
	seedOverlayFromSlice(topo, s)

	target := topology.Coord{X: x, Y: y, Z: z}
	rg := topology.BuildRegionGraph(topo)
	if _, ok := rg.RegionAt(target); !ok {
		// Live data agrees: the tile is solid, or otherwise not classifiable
		// as open floor. Keep the original truthful error.
		return topology.Coord{}, err
	}
	return target, nil
}

func clampAnchorLow(v int16) int16 {
	if v < 0 {
		return 0
	}
	return v
}

func clampAnchorHigh(v, max int16) int16 {
	if v > max {
		return max
	}
	return v
}

// glyphTileState maps a map_slice/look glyph (mapview.Legend) to the
// three-state classification resolveAnchorLive can safely assert from live
// data alone, mirroring topology.ClassifyState's FLAG_FLOOR/FLAG_WALL
// priority applied to the glyphs queries.cpp's classifyTile already reduced
// the raw tiletype to (see dfhack-plugin/tile_extractor.cpp's shape->flag
// switch for the source mapping). ok=false for any glyph that can't be
// classified from the glyph alone (hidden fog, open air/ramp-top, trees,
// liquids, ...) — SetTileState would otherwise assert a state the glyph
// doesn't actually justify.
func glyphTileState(g byte) (topology.TileState, bool) {
	switch g {
	case '.', ',', '<', '>', 'X', '^':
		return topology.StateOpen, true
	case '#', '%', '=':
		return topology.StateClosed, true
	default:
		return topology.StateUnknown, false
	}
}

// seedOverlayFromSlice writes the live glyph classification for every
// unambiguous tile in s into topo (see glyphTileState).
func seedOverlayFromSlice(topo *topology.TopologyOverlay, s *mapview.Slice) {
	for dy, row := range s.Rows {
		for dx := 0; dx < len(row); dx++ {
			state, ok := glyphTileState(row[dx])
			if !ok {
				continue
			}
			_ = topo.SetTileState(s.X1+int16(dx), s.Y1+int16(dy), s.Z, state)
		}
	}
}

func registerPlaceTools(srv *mcp.Server, b *Bridge) {
	type namePlaceIn struct {
		X    int    `json:"x" jsonschema:"a tile inside the region to name"`
		Y    int    `json:"y"`
		Z    int    `json:"z"`
		Name string `json:"name" jsonschema:"the label for this region, e.g. 'the storage hall'"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "name_place",
		Description: "Attach a persistent name to the dug/open region containing a tile — for anything worth tracking spatially (a hallway, a quarry section, an informal storage area), not just formal DF zones. Names survive df-mcp restarts. The tile must be inside already-carved space.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in namePlaceIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		topo := b.Topo()
		if topo == nil {
			return withDash(b, ctx, "topology not built yet (waiting for full state)"), nil, nil
		}
		anchor, err := resolveAnchorLive(ctx, topo, b, int16(in.X), int16(in.Y), int16(in.Z))
		if err != nil {
			return withDash(b, ctx, err.Error()), nil, nil
		}
		b.Places.Set(anchor, in.Name)
		if err := b.Places.Save(); err != nil {
			return withDash(b, ctx, fmt.Sprintf("named %q but failed to persist: %v", in.Name, err)), nil, nil
		}
		return withDash(b, ctx, fmt.Sprintf("SUCCESS: named the region at (%d,%d,%d) %q", in.X, in.Y, in.Z, in.Name)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_places",
		Description: "List every named region (anchor tile + name). Use to recall what you've already named before naming something new nearby.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		if b == nil || b.Places == nil {
			return TextResult("NOT CONNECTED: start DF, then run `ai-connect` in the DFHack console."), nil, nil
		}
		places := b.Places.All()
		if len(places) == 0 {
			return withDash(b, ctx, "No named places yet."), nil, nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d named places:\n", len(places))
		for _, p := range places {
			fmt.Fprintf(&sb, "- %q at (%d,%d,%d)\n", p.Name, p.Anchor.X, p.Anchor.Y, p.Anchor.Z)
		}
		return withDash(b, ctx, sb.String()), nil, nil
	})
}
