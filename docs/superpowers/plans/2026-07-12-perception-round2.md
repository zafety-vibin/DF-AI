# Perception Round 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the room/shaft-gap error class (two independent live incidents this session) by adding a region-graph-backed reachability system, painting designations and buildings into `look` via a lens mechanism, adding an `elevation_view` tool, and persisted named places — Feature 009 sub-project 1.

**Architecture:** A new Go-side 3D-connected region graph (`internal/topology`) over the existing (now-correct) `TopologyOverlay` powers three consumers: automatic reachability guidance inside `designate_dig`, persisted named places, and (later, out of scope here) zones. `look` gains a base-tier always-on designation glyph plus a single-lens-per-call overlay mechanism (`buildings`, `designations`), requiring two additive plugin/protocol wire fields bundled into one plugin rebuild. `elevation_view` is a new tool composing existing `column_profile` calls, no new plugin query.

**Tech Stack:** Go 1.25 (`internal/topology`, `internal/mapview`, `internal/mcpserver`), C++20 DFHack plugin (`dfhack-plugin/`), existing binary TCP protocol (`internal/protocol` / `dfhack-plugin/protocol.h`).

## Global Constraints

- Design of record: `specs/009-culture-and-learning/design-perception-round2.md` — every task below implements a section of it; do not deviate without updating that doc first.
- C++: DFHack `CHECK_*` macros THROW — every new DF-touching path must be reachable only inside `executeCommand`'s or `executeQuery`'s existing try/catch (both already wrap their full dispatch switch in `dfhack-plugin/df_ai_protocol.cpp` and `dfhack-plugin/queries.cpp`).
- Go: stdout is the MCP transport in `cmd/df-mcp` paths — logging only via `logging.NewStderrTextLogger`. Every MCP tool must return readable text on a nil/disconnected bridge (the registration sweep `TestEveryToolNilBridge` in `internal/mcpserver/server_test.go` calls every registered tool against a nil bridge; tools with required inputs need an entry in that file's `minToolArgs` map).
- Wire protocol changes are additive-optional only (an older peer on either side must keep working) — the established pattern is a new JSON field an old decoder simply ignores, or a new optional wire field with a presence byte (see `FortInfo`/`HasFortInfo` in `internal/protocol/codec.go` for precedent).
- Ship a unit test for every pure helper (project house rule, `docs/guides/mcp-server.md`).
- No volumetric/3D text renderings — slices plus model-side assembly is project law (`docs/decisions.md` 2026-07-12). `elevation_view` is one more 2D slice (a vertical plane along a line), not a new violation of this.
- Repo has `core.autocrlf=true` and no `.gitattributes` — avoid bulk/sed-style rewrites that would churn line endings.
- Plugin rebuild command: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release`. Artifact: `C:/Users/zmanl/Projects/dfhack-build/build/plugins/df_ai_protocol/Release/df_ai_protocol.plug.dll`. Deploy requires DF closed (file lock) — copy into `<Steam DF>/hack/plugins/`, matching filename exactly (`.plug.dll`, not `.dll`).
- DFHack 53.15-r1 reference checkout: `C:\Users\zmanl\Projects\dfhack-build` (read-only reference; `plugins/df_ai_protocol` there is a junction to this repo's `dfhack-plugin/` — never edit through the junction).

---

## File structure

New files:
- `internal/topology/regions.go` — `Coord`, `Region`, `RegionGraph`, `BuildRegionGraph`. Pure Go, no plugin dependency.
- `internal/topology/regions_test.go` — fixture-topology unit tests.
- `internal/mcpserver/lenses.go` — `Overlay`, `LensDef`, the `lenses` registry, `buildings`/`designations` gather functions.
- `internal/mcpserver/lenses_test.go` — lens gather + glyph-disjointness tests.
- `internal/mcpserver/places.go` — `Place`, `PlaceStore`, load/save, `name_place`/`list_places` tool registration.
- `internal/mcpserver/places_test.go` — persistence round-trip tests.
- `fortress/state/` — new sibling directory to `fortress/memory/`, holding tool-managed machine state (`places.json`). Kept distinct from `fortress/memory/` because that directory's charter is AI-authored narrative memory (`fortress/CLAUDE.md`: "tooling may read them... don't 'clean them up'") — a tool-write target belongs elsewhere, not folded into the AI's own memory files.

Modified files:
- `internal/mapview/render.go` — generalize `RenderCrop`'s hardcoded water/marks passes into an ordered `[]Overlay` pipeline; add `RenderElevation`.
- `internal/mapview/types.go` — `Slice` gains `DesignationKinds [][3]int16`.
- `internal/mcpserver/tools_percept.go` — `look` gains the `lens` param and always-on `d` overlay; new `elevation_view` tool.
- `internal/mcpserver/tools_action.go` — `designate_dig` gains the reachability suggestion.
- `internal/mcpserver/bridge.go` — no signature changes; `b.Topo()` and `b.Query` are reused as-is.
- `internal/mcpserver/tools_state.go` — extract a shared `buildingListEntry` type from `renderBuildings` so the `buildings` lens can reuse the same `list_buildings` decode.
- `internal/mcpserver/server_test.go` — `minToolArgs` entries for `elevation_view`, `name_place`, `list_places`.
- `dfhack-plugin/queries.cpp` — `queryMapSlice` emits per-tile designation kind; `handleListBuildings` emits `x1,y1,x2,y2` footprint.

---

## Task 1: Region graph core

**Files:**
- Create: `internal/topology/regions.go`
- Test: `internal/topology/regions_test.go`

**Interfaces:**
- Consumes: `topology.TopologyOverlay.GetDimensions() (w, h, d uint16)`, `.GetTileState(x, y, z int16) TileState`, `topology.StateOpen`.
- Produces: `type Coord struct{ X, Y, Z int16 }`, `type Region struct{ ID int; Tiles []Coord; BBox [2]Coord }`, `type RegionGraph struct{ Regions []Region }` (unexported `index map[Coord]int`), `func BuildRegionGraph(topo *TopologyOverlay) RegionGraph`, `func (rg RegionGraph) SameRegion(a, b Coord) bool`, `func (rg RegionGraph) RegionAt(c Coord) (Region, bool)`, `func (rg RegionGraph) NearestRegion(c Coord) (region Region, dist int, ok bool)`.

- [ ] **Step 1: Write the failing test for a simple two-room flood-fill**

```go
// internal/topology/regions_test.go
package topology

import "testing"

func fixtureTopo(w, h, d uint16, open []Coord) *TopologyOverlay {
	topo := NewTopologyOverlay(w, h, d)
	for _, c := range open {
		_ = topo.SetTileState(c.X, c.Y, c.Z, StateOpen)
	}
	return topo
}

func TestBuildRegionGraph_TwoDisconnectedRooms(t *testing.T) {
	// Two 2x2 rooms on the same Z, four tiles apart — no shared edge, not connected.
	open := []Coord{
		{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}, // room A
		{5, 0, 0}, {6, 0, 0}, {5, 1, 0}, {6, 1, 0}, // room B
	}
	topo := fixtureTopo(10, 10, 1, open)
	rg := BuildRegionGraph(topo)

	if len(rg.Regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(rg.Regions))
	}
	if rg.SameRegion(Coord{0, 0, 0}, Coord{5, 0, 0}) {
		t.Fatal("disconnected rooms must not report SameRegion")
	}
	if !rg.SameRegion(Coord{0, 0, 0}, Coord{1, 1, 0}) {
		t.Fatal("tiles in the same room must report SameRegion")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/topology/... -run TestBuildRegionGraph_TwoDisconnectedRooms -v`
Expected: FAIL — `BuildRegionGraph`, `Coord`, `RegionGraph.SameRegion` undefined (compile error).

- [ ] **Step 3: Write the region-graph implementation (horizontal-only first pass)**

```go
// internal/topology/regions.go
package topology

// Coord is a 3D map coordinate, local to the topology package (a
// package-level Coord here would create an import cycle with
// internal/worldmodel, which already imports topology).
type Coord struct{ X, Y, Z int16 }

// Region is one 3D-connected component of open (walkable) tiles.
type Region struct {
	ID    int
	Tiles []Coord
	BBox  [2]Coord // [0] = min corner, [1] = max corner
}

// RegionGraph is the full set of regions for one topology snapshot, plus
// a fast tile->region lookup.
type RegionGraph struct {
	Regions []Region
	index   map[Coord]int // tile -> index into Regions
}

// BuildRegionGraph computes 3D-connected components of open tiles: full
// recompute on every call, no incremental maintenance (see design doc
// "Component 1" for the rationale — this is cheap enough in practice
// that incremental maintenance is deferred until measurement says
// otherwise).
//
// Adjacency: horizontal 4-connectivity within a Z-level, plus vertical
// connectivity through stair tiles (added in Step 6 below).
func BuildRegionGraph(topo *TopologyOverlay) RegionGraph {
	w, h, d := topo.GetDimensions()
	visited := make(map[Coord]bool)
	var regions []Region
	index := make(map[Coord]int)

	for z := int16(0); z < int16(d); z++ {
		for y := int16(0); y < int16(h); y++ {
			for x := int16(0); x < int16(w); x++ {
				start := Coord{x, y, z}
				if visited[start] || topo.GetTileState(x, y, z) != StateOpen {
					continue
				}
				region := floodFill(topo, start, visited)
				region.ID = len(regions)
				for _, t := range region.Tiles {
					index[t] = region.ID
				}
				regions = append(regions, region)
			}
		}
	}
	return RegionGraph{Regions: regions, index: index}
}

func floodFill(topo *TopologyOverlay, start Coord, visited map[Coord]bool) Region {
	queue := []Coord{start}
	visited[start] = true
	tiles := []Coord{start}
	minC, maxC := start, start

	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		for _, n := range neighbors(topo, c) {
			if visited[n] || topo.GetTileState(n.X, n.Y, n.Z) != StateOpen {
				continue
			}
			visited[n] = true
			tiles = append(tiles, n)
			queue = append(queue, n)
			minC, maxC = minCoord(minC, n), maxCoord(maxC, n)
		}
	}
	return Region{Tiles: tiles, BBox: [2]Coord{minC, maxC}}
}

func neighbors(topo *TopologyOverlay, c Coord) []Coord {
	return []Coord{
		{c.X - 1, c.Y, c.Z}, {c.X + 1, c.Y, c.Z},
		{c.X, c.Y - 1, c.Z}, {c.X, c.Y + 1, c.Z},
	}
}

func minCoord(a, b Coord) Coord {
	m := a
	if b.X < m.X {
		m.X = b.X
	}
	if b.Y < m.Y {
		m.Y = b.Y
	}
	if b.Z < m.Z {
		m.Z = b.Z
	}
	return m
}

func maxCoord(a, b Coord) Coord {
	m := a
	if b.X > m.X {
		m.X = b.X
	}
	if b.Y > m.Y {
		m.Y = b.Y
	}
	if b.Z > m.Z {
		m.Z = b.Z
	}
	return m
}

// SameRegion reports whether a and b are in the same connected component.
// Two tiles that are both unknown/closed (neither is open) are never the
// same region, even if equal.
func (rg RegionGraph) SameRegion(a, b Coord) bool {
	ia, ok1 := rg.index[a]
	ib, ok2 := rg.index[b]
	return ok1 && ok2 && ia == ib
}

// RegionAt returns the region containing c, if any.
func (rg RegionGraph) RegionAt(c Coord) (Region, bool) {
	i, ok := rg.index[c]
	if !ok {
		return Region{}, false
	}
	return rg.Regions[i], true
}

// NearestRegion returns the region whose closest tile to c has the
// smallest Chebyshev distance, and that distance. ok is false when the
// graph has no regions at all (e.g. nothing dug yet).
func (rg RegionGraph) NearestRegion(c Coord) (region Region, dist int, ok bool) {
	best := -1
	for _, r := range rg.Regions {
		for _, t := range r.Tiles {
			dx, dy, dz := abs16(t.X-c.X), abs16(t.Y-c.Y), abs16(t.Z-c.Z)
			d := max3(dx, dy, dz)
			if best == -1 || d < best {
				best, region, ok = d, r, true
			}
		}
	}
	dist = best
	return
}

func abs16(v int16) int {
	if v < 0 {
		return int(-v)
	}
	return int(v)
}

func max3(a, b, c int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/topology/... -run TestBuildRegionGraph_TwoDisconnectedRooms -v`
Expected: PASS

- [ ] **Step 5: Add the multi-Z stair-connectivity test (write failing first)**

```go
// internal/topology/regions_test.go — append
func TestBuildRegionGraph_StairConnectsZLevels(t *testing.T) {
	// A 1x1 "shaft" of open tiles stacked on Z=0,1,2, plus an isolated
	// open tile on Z=5 with no vertical path — two regions, not three,
	// and the Z=5 tile is its own region.
	open := []Coord{
		{0, 0, 0}, {0, 0, 1}, {0, 0, 2},
		{5, 5, 5},
	}
	topo := fixtureTopo(10, 10, 10, open)
	rg := BuildRegionGraph(topo)

	if len(rg.Regions) != 2 {
		t.Fatalf("expected 2 regions (shaft + isolated tile), got %d", len(rg.Regions))
	}
	if !rg.SameRegion(Coord{0, 0, 0}, Coord{0, 0, 2}) {
		t.Fatal("vertically stacked open tiles must be connected")
	}
	if rg.SameRegion(Coord{0, 0, 0}, Coord{5, 5, 5}) {
		t.Fatal("the isolated tile must not merge with the shaft")
	}
}

func TestRegionGraph_NearestRegion(t *testing.T) {
	open := []Coord{{0, 0, 0}, {1, 0, 0}}
	topo := fixtureTopo(10, 10, 1, open)
	rg := BuildRegionGraph(topo)

	region, dist, ok := rg.NearestRegion(Coord{5, 0, 0})
	if !ok {
		t.Fatal("expected a region to be found")
	}
	if dist != 4 { // closest tile is (1,0,0), Chebyshev distance 4
		t.Fatalf("expected distance 4, got %d", dist)
	}
	if region.ID != 0 {
		t.Fatalf("expected region 0, got %d", region.ID)
	}
}

func TestRegionGraph_NearestRegion_EmptyGraph(t *testing.T) {
	topo := fixtureTopo(10, 10, 1, nil)
	rg := BuildRegionGraph(topo)
	_, _, ok := rg.NearestRegion(Coord{0, 0, 0})
	if ok {
		t.Fatal("empty graph must report ok=false, not fabricate a region")
	}
}
```

- [ ] **Step 6: Run to verify the stair test fails (vertical adjacency not yet implemented)**

Run: `go test ./internal/topology/... -run TestBuildRegionGraph_StairConnectsZLevels -v`
Expected: FAIL — the current `neighbors()` is horizontal-only, so (0,0,0)/(0,0,1)/(0,0,2) each become their own single-tile region (3 regions instead of the expected 2).

- [ ] **Step 7: Add vertical adjacency to `neighbors`**

```go
// internal/topology/regions.go — replace the neighbors function
// Adjacency: horizontal 4-connectivity within a Z-level, plus vertical
// connectivity straight up/down. This is deliberately permissive (any
// open tile directly above/below counts, not just stair-shaped tiles) —
// StateOpen already means "walkable" per ClassifyState's FLAG_FLOOR
// check, which the plugin only sets for tiles a dwarf can stand on
// (including stair tiles), so an open tile stacked on an open tile is by
// construction a real vertical path.
func neighbors(topo *TopologyOverlay, c Coord) []Coord {
	return []Coord{
		{c.X - 1, c.Y, c.Z}, {c.X + 1, c.Y, c.Z},
		{c.X, c.Y - 1, c.Z}, {c.X, c.Y + 1, c.Z},
		{c.X, c.Y, c.Z - 1}, {c.X, c.Y, c.Z + 1},
	}
}
```

- [ ] **Step 8: Run full region-graph test file to verify all pass**

Run: `go test ./internal/topology/... -v`
Expected: PASS — all of `TestBuildRegionGraph_TwoDisconnectedRooms`, `TestBuildRegionGraph_StairConnectsZLevels`, `TestRegionGraph_NearestRegion`, `TestRegionGraph_NearestRegion_EmptyGraph`.

- [ ] **Step 9: Commit**

```bash
git add internal/topology/regions.go internal/topology/regions_test.go
git commit -m "feat(topology): 3D-connected region graph over open tiles

Full recompute per call (no incremental maintenance — cheap enough at
this project's fort sizes per the token-scaling research; revisit if
measurement says otherwise). Powers reachability guidance and named
places (both next tasks)."
```

---

## Task 2: Reachability guidance in `designate_dig`

**Files:**
- Modify: `internal/mcpserver/tools_action.go:184-207` (the `designate_dig` tool)
- Test: `internal/mcpserver/tools_action_test.go` (create if it doesn't exist — check first with `Glob internal/mcpserver/tools_action_test.go`)

**Interfaces:**
- Consumes: `topology.BuildRegionGraph`, `topology.RegionGraph.NearestRegion`, `topology.Coord`, `b.Topo() *topology.TopologyOverlay`.
- Produces: `func connectorSuggestion(topo *topology.TopologyOverlay, x1, y1, z1, x2, y2, z2 int16) string` — empty string if the designation touches an existing open region (or the graph is empty, i.e. nothing dug yet — no suggestion makes sense for the very first dig), else a suggested `designate_dig` invocation string.

- [ ] **Step 1: Write the failing test for the pure helper**

```go
// internal/mcpserver/tools_action_test.go
package mcpserver

import (
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/topology"
)

func TestConnectorSuggestion_TouchingRegionReturnsEmpty(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 5)
	// existing open room at (0,0,0)-(2,2,0)
	for x := int16(0); x <= 2; x++ {
		for y := int16(0); y <= 2; y++ {
			_ = topo.SetTileState(x, y, 0, topology.StateOpen)
		}
	}
	// new designation directly adjacent (touches x=3, which borders x=2)
	got := connectorSuggestion(topo, 3, 0, 0, 5, 2, 0)
	if got != "" {
		t.Fatalf("expected no suggestion for a touching designation, got %q", got)
	}
}

func TestConnectorSuggestion_DisconnectedReturnsSuggestion(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 5)
	for x := int16(0); x <= 2; x++ {
		for y := int16(0); y <= 2; y++ {
			_ = topo.SetTileState(x, y, 0, topology.StateOpen)
		}
	}
	// new designation far away, no shared border
	got := connectorSuggestion(topo, 10, 10, 0, 12, 12, 0)
	if !strings.Contains(got, "not yet connected to existing space") {
		t.Fatalf("expected a connector suggestion, got %q", got)
	}
	if !strings.Contains(got, "designate_dig default") {
		t.Fatalf("expected the suggestion to name a designate_dig call, got %q", got)
	}
}

func TestConnectorSuggestion_EmptyGraphReturnsEmpty(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 5) // nothing dug yet
	got := connectorSuggestion(topo, 0, 0, 0, 2, 2, 0)
	if got != "" {
		t.Fatalf("expected no suggestion on a fresh embark with nothing dug, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcpserver/... -run TestConnectorSuggestion -v`
Expected: FAIL — `connectorSuggestion` undefined.

- [ ] **Step 3: Implement `connectorSuggestion`**

```go
// internal/mcpserver/tools_action.go — add near the top of the file,
// after the imports, before registerActionTools
import (
	// ... existing imports ...
	"github.com/df-ai/orchestrator/internal/topology"
)

// connectorSuggestion checks whether the just-designated rectangle
// touches any existing open (already-dug) region. Returns "" when it
// does (or when nothing has been dug yet — no suggestion makes sense
// for a fort's very first designation). Never returns error/warning
// text: a fresh designation being disconnected from existing space is
// normal DF workflow (room first, corridor second), not a mistake — see
// design doc "Component 3".
func connectorSuggestion(topo *topology.TopologyOverlay, x1, y1, z1, x2, y2, z2 int16) string {
	rg := topology.BuildRegionGraph(topo)
	if len(rg.Regions) == 0 {
		return ""
	}
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	if z1 > z2 {
		z1, z2 = z2, z1
	}
	// Check every tile just outside the rectangle's boundary (one ring
	// of padding on all sides, all swept Z levels) for an open neighbor.
	for z := z1; z <= z2; z++ {
		for x := x1 - 1; x <= x2+1; x++ {
			for y := y1 - 1; y <= y2+1; y++ {
				inside := x >= x1 && x <= x2 && y >= y1 && y <= y2
				if inside {
					continue
				}
				if topo.GetTileState(x, y, z) == topology.StateOpen {
					return "" // touches existing open space — connected
				}
			}
		}
	}
	centerX, centerY, centerZ := (x1+x2)/2, (y1+y2)/2, z1
	region, _, ok := rg.NearestRegion(topology.Coord{X: centerX, Y: centerY, Z: centerZ})
	if !ok {
		return ""
	}
	// Suggest a 1-wide connector from the rectangle's center toward the
	// nearest region's bounding-box center — a heuristic starting point,
	// not a guaranteed-optimal path; the model refines it with look.
	target := region.BBox[0] // min corner is a stable, deterministic anchor
	return fmt.Sprintf(
		"not yet connected to existing space — suggested connector: designate_dig default (%d,%d,%d)->(%d,%d,%d)",
		centerX, centerY, centerZ, target.X, target.Y, target.Z)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mcpserver/... -run TestConnectorSuggestion -v`
Expected: PASS

- [ ] **Step 5: Wire the suggestion into the `designate_dig` tool handler**

```go
// internal/mcpserver/tools_action.go:196-207 — replace the handler body
}, func(ctx context.Context, req *mcp.CallToolRequest, in digIn) (*mcp.CallToolResult, any, error) {
	if r := noExec(b); r != nil {
		return r, nil, nil
	}
	dt, err := digTypeFromName(in.Type)
	if err != nil {
		return withDash(b, ctx, err.Error()), nil, nil
	}
	res, err := b.Exec.SendDigRegion(dt, int16(in.X1), int16(in.Y1), int16(in.Z1), int16(in.X2), int16(in.Y2), int16(in.Z2))
	what := fmt.Sprintf("dig %s (%d,%d,%d)->(%d,%d,%d)", in.Type, in.X1, in.Y1, in.Z1, in.X2, in.Y2, in.Z2)
	ack := ackText(res, err, what)
	// Reachability guidance: only meaningful after a successful dig
	// designation, and only against a live topology overlay.
	if err == nil && res != nil && res.Success {
		if topo := b.Topo(); topo != nil {
			if suggestion := connectorSuggestion(topo, int16(in.X1), int16(in.Y1), int16(in.Z1), int16(in.X2), int16(in.Y2), int16(in.Z2)); suggestion != "" {
				ack = ack + "\n" + suggestion
			}
		}
	}
	return withDash(b, ctx, ack), nil, nil
})
```

- [ ] **Step 6: Also update the tool description to mention the guidance**

```go
// internal/mcpserver/tools_action.go:194-195 — replace the Description field
mcp.AddTool(srv, &mcp.Tool{
	Name:        "designate_dig",
	Description: "Designate digging over a 3D rectangle. Hidden fog tiles designate fine (that's how forts are dug). 'stairs' spans z1..z2 as one shaft (2x2 recommended, surface to deep stone in ONE call); a stairs range whose top adjoins existing carved stairs joins the shaft, and already-carved tiles are skipped — so extending a shaft deeper is safe. Dwarves with picks do the work over game time — step() to let it happen. If the designated area isn't yet connected to existing dug space, the ACK suggests a connector — this is informational, not an error: designating a room before its corridor is normal.",
}, func(ctx context.Context, req *mcp.CallToolRequest, in digIn) (*mcp.CallToolResult, any, error) {
```

- [ ] **Step 7: Run the full mcpserver test suite to confirm no regressions**

Run: `go test ./internal/mcpserver/... -v`
Expected: PASS (all existing + new tests)

- [ ] **Step 8: Commit**

```bash
git add internal/mcpserver/tools_action.go internal/mcpserver/tools_action_test.go
git commit -m "feat(mcpserver): reachability guidance in designate_dig

Automatic, non-shaming connector suggestion when a new designation
doesn't touch existing dug space — the actual fix for the 4,800-tick
incident this session (a room designated 4 tiles from existing floor,
discovered only after burning the step budget). Never warning-toned:
room-then-corridor is normal DF workflow, not a mistake."
```

---

## Task 3: Overlay pipeline generalization + always-on designation glyph

**Files:**
- Modify: `internal/mapview/render.go`
- Modify: `internal/mapview/render_test.go`

**Interfaces:**
- Consumes: existing `Slice` (unchanged in this task — `Slice.Designated [][2]int16` already exists on the wire).
- Produces: `type Overlay struct{ Marks map[[2]int16]rune; Footnotes []string }`, `func RenderCrop(s *Slice, overlays []Overlay, legendExtra string) string` (signature CHANGE — old 2-arg callers must be updated; this task updates the one existing caller in `tools_percept.go`'s `look` handler as part of Step 5).

- [ ] **Step 1: Write the failing test for the new signature and the always-on `d` overlay**

```go
// internal/mapview/render_test.go — add
func TestRenderCrop_DesignationsAlwaysPainted(t *testing.T) {
	s := &Slice{
		Z: 100, X1: 0, Y1: 0,
		Rows:       []string{"..", ".."},
		Designated: [][2]int16{{1, 0}},
	}
	out := RenderCrop(s, nil, "")
	if !strings.Contains(out, "d designated") {
		t.Fatalf("legend must gain a 'd designated' entry:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	// Row 0 (y=0) is rendered on the line starting "   0 " (4-wide right-aligned + space).
	found := false
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimLeft(l, " "), "0 ") {
			// glyph at x=1 should be 'd', not '.'
			if strings.Contains(l, ".d") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("designated tile at (1,0) must render as 'd':\n%s", out)
	}
}

func TestRenderCrop_OverlayPaintsLastAndAddsFootnoteOnDwarfCollision(t *testing.T) {
	s := &Slice{Z: 100, X1: 0, Y1: 0, Rows: []string{".."}}
	dwarfMarks := Overlay{Marks: map[[2]int16]rune{{0, 0}: '@'}}
	lensOverlay := Overlay{
		Marks:     map[[2]int16]rune{{0, 0}: 'B'},
		Footnotes: []string{"dwarves in view: 1 (1 under overlay at (0,0))"},
	}
	out := RenderCrop(s, []Overlay{dwarfMarks, lensOverlay}, "lens=buildings: B furniture")
	if !strings.Contains(out, "under overlay at (0,0)") {
		t.Fatalf("expected the collision footnote:\n%s", out)
	}
	if !strings.Contains(out, "lens=buildings: B furniture") {
		t.Fatalf("expected the lens legend addendum:\n%s", out)
	}
}

func TestRenderCrop_NoLensNoAddendum(t *testing.T) {
	s := &Slice{Z: 100, X1: 0, Y1: 0, Rows: []string{"."}}
	out := RenderCrop(s, nil, "")
	if strings.Contains(out, "lens=") {
		t.Fatalf("no lens active must mean no lens addendum:\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mapview/... -run TestRenderCrop -v`
Expected: FAIL — compile error (`Overlay` undefined, `RenderCrop` signature mismatch against existing callers) or the old 2-arg `RenderCrop(s, marks)` calls not matching the new 3-arg signature.

- [ ] **Step 3: Rewrite `RenderCrop` with the generalized overlay pipeline**

```go
// internal/mapview/render.go — replace RenderCrop entirely
package mapview

import (
	"fmt"
	"strings"
)

// Overlay is one painted layer: glyph replacements at absolute (x,y),
// plus optional footnote lines appended after the legend. Later overlays
// in RenderCrop's slice win per-tile on collision.
type Overlay struct {
	Marks     map[[2]int16]rune
	Footnotes []string
}

// RenderCrop renders a Slice as a labeled glyph grid. No spaces between
// glyphs (tokenization research: separators destroy adjacency).
//
// Paint order (design doc "Component 2", table in section 2.2):
//  1. base terrain (s.Rows)
//  2. water digits 1-7 (s.Water) — always on
//  3. designations 'd' (s.Designated) — always on; two live incidents
//     this session trace to designations being invisible by default
//  4. overlays, in the order passed — dwarves '@' is conventionally
//     overlays[0]; a lens (buildings/designations detail) is later in
//     the slice so it paints last, since the model explicitly asked for
//     that layer. A later overlay hiding an earlier overlay's mark
//     should carry its own footnote (see the buildings/designations
//     lens gather functions in internal/mcpserver/lenses.go) — RenderCrop
//     itself does not synthesize collision footnotes, callers do, because
//     only the caller knows which collisions are worth mentioning.
//
// legendExtra is appended as one more line after the core legend, only
// when non-empty — the GIS dynamic-legend rule: pay legend cost only for
// the layer you asked about.
func RenderCrop(s *Slice, overlays []Overlay, legendExtra string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "z=%d — %dx%d crop at (%d,%d), north is up, x grows east, y grows south\n",
		s.Z, len(s.Rows[0]), len(s.Rows), s.X1, s.Y1)
	rowLen := len(s.Rows[0])
	sb.WriteString("   ")
	fmt.Fprintf(&sb, "x=%d", s.X1)
	if gap := rowLen - 3 - len(fmt.Sprintf("%d", s.X1)); gap >= 1 {
		sb.WriteString(strings.Repeat(" ", gap))
		fmt.Fprintf(&sb, "x=%d", s.X1+int16(rowLen)-1)
	}
	sb.WriteString("\n")

	water := map[[2]int16]rune{}
	for _, w := range s.Water {
		d := w[2]
		if d < 1 {
			continue
		}
		if d > 7 {
			d = 7
		}
		water[[2]int16{w[0], w[1]}] = rune('0' + d)
	}
	designated := map[[2]int16]rune{}
	for _, dpos := range s.Designated {
		designated[[2]int16{dpos[0], dpos[1]}] = 'd'
	}

	paintLayers := make([]map[[2]int16]rune, 0, 2+len(overlays))
	paintLayers = append(paintLayers, water, designated)
	for _, ov := range overlays {
		paintLayers = append(paintLayers, ov.Marks)
	}

	for i, row := range s.Rows {
		y := s.Y1 + int16(i)
		glyphs := []rune(row)
		for _, layer := range paintLayers {
			for pos, r := range layer {
				if pos[1] == y && pos[0] >= s.X1 && int(pos[0]-s.X1) < len(glyphs) {
					glyphs[pos[0]-s.X1] = r
				}
			}
		}
		fmt.Fprintf(&sb, "%4d %s\n", y, string(glyphs))
	}

	sb.WriteString("\nlegend: @ dwarf d designated " + Legend + "\n")
	if len(s.Designated) > 0 {
		fmt.Fprintf(&sb, "designated for digging: %d tiles in view\n", len(s.Designated))
	}
	if len(s.Aquifer) > 0 {
		fmt.Fprintf(&sb, "aquifer tiles in view: %d\n", len(s.Aquifer))
	}
	if legendExtra != "" {
		sb.WriteString(legendExtra + "\n")
	}
	for _, ov := range overlays {
		for _, fn := range ov.Footnotes {
			sb.WriteString(fn + "\n")
		}
	}
	return sb.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mapview/... -v`
Expected: PASS — all render_test.go tests, including the pre-existing water/marks tests (now expressed via `Overlay{Marks: ...}` — if any old test still calls the 2-arg form, fix it in this step, since old callers must be updated, not left broken).

- [ ] **Step 5: Update the one existing caller (`look` in tools_percept.go) to the new signature**

```go
// internal/mcpserver/tools_percept.go:43-49 — replace
marks := map[[2]int16]rune{}
for _, d := range b.Snapshot().Entities.Dwarves {
	if d.Z == int16(in.Z) {
		marks[[2]int16{d.X, d.Y}] = '@'
	}
}
return withDash(b, ctx, mapview.RenderCrop(s, []mapview.Overlay{{Marks: marks}}, "")), nil, nil
```

(The `lens` param and its overlay are added in Task 7 — this step only fixes the call site to compile against the new signature, keeping `look`'s current behavior unchanged otherwise.)

- [ ] **Step 6: Run the full test suite to confirm no regressions**

Run: `go build ./... && go test ./... > /tmp/step6.log 2>&1; echo exit=$?; cat /tmp/step6.log`

(Use the scratchpad path instead of `/tmp` per this project's Windows environment — e.g. redirect to a file under the session scratchpad — and never pipe through `tail`/`head`.)
Expected: exit=0, all packages `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/mapview/render.go internal/mapview/render_test.go internal/mcpserver/tools_percept.go
git commit -m "feat(mapview): generalize RenderCrop into an ordered overlay pipeline

Designations now paint as 'd' unconditionally (base tier, not a lens) —
both live room/shaft-gap incidents happened while the designation COUNT
was already visible as a number; an opt-in lens would let the identical
failure recur. Water stays always-on too. The overlay pipeline itself is
now data-driven (Overlay{Marks, Footnotes}) instead of two hardcoded
passes, preparing for the buildings/designations lens (next tasks)."
```

---

## Task 4: Plugin wire additions — designation kind + building footprint extents

**Files:**
- Modify: `dfhack-plugin/queries.cpp` (`queryMapSlice` around line 608-660, `handleListBuildings` around line 403-441)

**Interfaces:**
- Produces (JSON, additive-optional): `map_slice` response gains `"designation_kinds":[[x,y,kind],...]` where kind is `0=dig(Default) 1=channel 2=ramp 3=stair 4=smooth`. `list_buildings` response entries gain `"x1","y1","x2","y2"` (footprint corners) alongside the existing `"x","y"` (center).

- [ ] **Step 1: Add designation-kind emission to `queryMapSlice`**

```cpp
// dfhack-plugin/queries.cpp — inside queryMapSlice, replace the
// designated-tiles block (the `if (des.bits.dig != df::tile_dig_designation::No && desCount < 200)`
// section) with:
    std::string designationKinds = "[";
    int kindCount = 0;
    // ... (inside the existing per-tile loop, alongside the existing
    // `designated` string-building) ...
            if (des.bits.dig != df::tile_dig_designation::No && desCount < 200) {
                if (desCount) designated += ",";
                designated += "[" + jsonInt(x) + "," + jsonInt(y) + "]";
                desCount++;

                // Kind detail for the designations lens. Smooth/engrave
                // (des.bits.smooth) is orthogonal to the dig-designation
                // enum, so it's checked separately and takes priority in
                // the rendered kind when both are set (a tile can be
                // marked both dig AND smooth simultaneously in DF, but
                // the "what am I about to become" question the lens
                // answers is dominated by whichever finishes first —
                // smooth only applies to already-carved floor, so a tile
                // with des.bits.dig != No hasn't been carved yet and
                // smooth wouldn't apply; this branch order is defensive,
                // not load-bearing, given that constraint).
                int kind = 0; // Default (dig)
                switch (des.bits.dig) {
                    case df::tile_dig_designation::Channel: kind = 1; break;
                    case df::tile_dig_designation::Ramp: kind = 2; break;
                    case df::tile_dig_designation::UpStair:
                    case df::tile_dig_designation::DownStair:
                    case df::tile_dig_designation::UpDownStair: kind = 3; break;
                    default: kind = 0; break;
                }
                if (kindCount < 200) {
                    if (kindCount) designationKinds += ",";
                    designationKinds += "[" + jsonInt(x) + "," + jsonInt(y) + "," + jsonInt(kind) + "]";
                    kindCount++;
                }
            } else if (des.bits.smooth && kindCount < 200) {
                if (kindCount) designationKinds += ",";
                designationKinds += "[" + jsonInt(x) + "," + jsonInt(y) + ",4]"; // smooth/engrave
                kindCount++;
            }
```

Then in the response-assembly section (where `designated` and `water`/`aquifer` strings are closed and appended to the final JSON object), add `designationKinds += "]";` alongside the existing closes, and append `",\"designation_kinds\":" + designationKinds` to the output JSON — mirror exactly how `water`/`aquifer` are closed and appended (read the ~15 lines after the per-tile loop in the current file to match the exact closing/assembly pattern before editing, since this plan's excerpt doesn't reproduce it verbatim).

- [ ] **Step 2: Add footprint extents to `handleListBuildings`**

```cpp
// dfhack-plugin/queries.cpp:426-433 — replace the per-building JSON object
        os << "{\"type\":" << jsonStr(ENUM_KEY_STR(building_type, b->getType()))
           << ",\"x\":" << jsonInt(b->centerx)
           << ",\"y\":" << jsonInt(b->centery)
           << ",\"z\":" << jsonInt(b->z)
           << ",\"x1\":" << jsonInt(b->x1)
           << ",\"y1\":" << jsonInt(b->y1)
           << ",\"x2\":" << jsonInt(b->x2)
           << ",\"y2\":" << jsonInt(b->y2)
           << ",\"stage\":" << jsonInt(stage)
           << ",\"max_stage\":" << jsonInt(maxStage)
           << ",\"done\":" << (stage == maxStage ? "true" : "false")
           << "}";
```

(`b->x1/y1/x2/y2` are directly on the base `df::building` class per `df.building.xml:326-330` — confirmed via the DFHack 53.15-r1 checkout, no cast needed.)

- [ ] **Step 3: Rebuild the plugin and check for compile errors**

Run: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release`
(Redirect to a log file, check the exit code directly — never pipe through `tail`/`head`. Retry once if it fails near `generate_headers` with exit `-1073741819`, a known flaky Perl-codegen issue, before treating it as a real failure.)
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add dfhack-plugin/queries.cpp
git commit -m "feat(plugin): designation kind + building footprint extents (additive wire fields)

map_slice gains designation_kinds [[x,y,kind]] (0=dig 1=channel 2=ramp
3=stair 4=smooth); list_buildings gains x1/y1/x2/y2 footprint corners
alongside the existing center. Both additive-optional — an older Go
decoder simply ignores the new fields. Powers the buildings/designations
lens (next task)."
```

**Note for the implementing agent:** this task and Task 5 (the Go-side decode) both touch the same wire contract and should land together before any live plugin redeploy — bundle the deploy step at the end of Task 11, not here, to avoid a wasted rebuild/DF-restart cycle mid-plan.

---

## Task 5: Go-side decode for the new wire fields

**Files:**
- Modify: `internal/mapview/types.go`
- Modify: `internal/mcpserver/tools_state.go` (extract `buildingListEntry` for reuse)
- Test: `internal/mapview/types_test.go`, `internal/mcpserver/tools_state_test.go`

**Interfaces:**
- Produces: `Slice.DesignationKinds [][3]int16` (mapview), `type buildingListEntry struct{ Type string; X, Y, Z int; X1, Y1, X2, Y2 int; Stage, MaxStage int; Done bool }` (mcpserver, package-private — used by both `renderBuildings` and the buildings lens in Task 6).

- [ ] **Step 1: Write the failing test for `Slice` decoding the new field**

```go
// internal/mapview/types_test.go — create if it doesn't exist, else append
package mapview

import "testing"

func TestDecodeSlice_DesignationKindsOptional(t *testing.T) {
	raw := []byte(`{"z":100,"x1":0,"y1":0,"rows":["..","d."],"designated":[[0,1]],"designation_kinds":[[0,1,2]]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s.DesignationKinds) != 1 || s.DesignationKinds[0] != [3]int16{0, 1, 2} {
		t.Fatalf("expected DesignationKinds [[0,1,2]], got %v", s.DesignationKinds)
	}
}

func TestDecodeSlice_DesignationKindsAbsentIsFine(t *testing.T) {
	raw := []byte(`{"z":100,"x1":0,"y1":0,"rows":["..","d."],"designated":[[0,1]]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("an older-plugin payload without designation_kinds must still decode: %v", err)
	}
	if len(s.DesignationKinds) != 0 {
		t.Fatalf("expected empty DesignationKinds, got %v", s.DesignationKinds)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mapview/... -run TestDecodeSlice_DesignationKinds -v`
Expected: FAIL — `Slice` has no `DesignationKinds` field (JSON unmarshal silently ignores the unknown key today, so the first test fails on the length/value assertion, not a decode error).

- [ ] **Step 3: Add the field**

```go
// internal/mapview/types.go — Slice struct, add after the Aquifer field
	// DesignationKinds is per-designated-tile kind detail: [x,y,kind]
	// where kind is 0=dig(Default) 1=channel 2=ramp 3=stair 4=smooth.
	// Optional: an older plugin omits it, leaving Designated (plain
	// [x,y] pairs, no kind) as the only always-on signal.
	DesignationKinds [][3]int16 `json:"designation_kinds"`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mapview/... -v`
Expected: PASS

- [ ] **Step 5: Extract `buildingListEntry` from `renderBuildings` for reuse by the buildings lens**

```go
// internal/mcpserver/tools_state.go — replace the anonymous struct inside renderBuildings
// buildingListEntry is one entry from the plugin's list_buildings query.
// Shared between the flat `buildings` tool (renderBuildings) and the
// `buildings` look lens (internal/mcpserver/lenses.go) so both parse the
// exact same wire shape once.
type buildingListEntry struct {
	Type     string `json:"type"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Z        int    `json:"z"`
	X1       int    `json:"x1"`
	Y1       int    `json:"y1"`
	X2       int    `json:"x2"`
	Y2       int    `json:"y2"`
	Stage    int    `json:"stage"`
	MaxStage int    `json:"max_stage"`
	Done     bool   `json:"done"`
}

func renderBuildings(raw []byte) string {
	var resp struct {
		Buildings []buildingListEntry `json:"buildings"`
		Truncated bool                `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_buildings response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Buildings) == 0 {
		return "No buildings."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d buildings:\n", len(resp.Buildings))
	for _, bl := range resp.Buildings {
		if bl.Done {
			fmt.Fprintf(&sb, "- %s at (%d,%d,%d) — built\n", bl.Type, bl.X, bl.Y, bl.Z)
		} else {
			fmt.Fprintf(&sb, "- %s at (%d,%d,%d) — UNDER CONSTRUCTION (stage %d/%d)\n",
				bl.Type, bl.X, bl.Y, bl.Z, bl.Stage, bl.MaxStage)
		}
	}
	if resp.Truncated {
		sb.WriteString("... list truncated at the plugin's cap — pass z to narrow it\n")
	}
	return sb.String()
}
```

- [ ] **Step 6: Run the mcpserver test suite to confirm `TestRenderBuildings` (existing test) still passes unchanged**

Run: `go test ./internal/mcpserver/... -run TestRenderBuildings -v`
Expected: PASS (the JSON shape `renderBuildings` reads didn't change from the caller's perspective — only the anonymous struct became a named, exported-within-package type; the pre-existing test's fixture JSON, which doesn't include x1/y1/x2/y2, must still decode fine since Go's `encoding/json` leaves unset fields at their zero value).

- [ ] **Step 7: Commit**

```bash
git add internal/mapview/types.go internal/mapview/types_test.go internal/mcpserver/tools_state.go
git commit -m "feat(mapview,mcpserver): decode the new wire fields; extract buildingListEntry

Slice.DesignationKinds (optional, absent-tolerant) for the designations
lens. buildingListEntry pulled out of renderBuildings so the buildings
lens (next task) parses list_buildings identically instead of a second,
possibly-drifting struct."
```

---

## Task 6: `buildings` and `designations` lenses

**Files:**
- Create: `internal/mcpserver/lenses.go`
- Test: `internal/mcpserver/lenses_test.go`

**Interfaces:**
- Consumes: `buildingListEntry` (Task 5), `Slice.DesignationKinds` (Task 5), `mapview.Overlay` (Task 3), `Bridge.Query`.
- Produces: `type LensDef struct{ Name string; Legend string; Gather func(ctx context.Context, b *Bridge, s *mapview.Slice, z int16) (mapview.Overlay, error) }`, `var lenses map[string]LensDef`, `func lensNames() []string` (for the unknown-lens error message).

- [ ] **Step 1: Write the failing tests for the pure gather logic and the glyph-disjointness invariant**

```go
// internal/mcpserver/lenses_test.go
package mcpserver

import (
	"strings"
	"testing"
)

// baseTerrainGlyphSet mirrors internal/mapview.Legend's occupied
// characters plus the always-on overlay glyphs, EXCLUDING 'd' — the
// designations lens deliberately reuses 'd' for the Default kind (see
// design doc "Glyph governance", the one documented exception).
var baseTerrainGlyphSet = map[rune]bool{
	'?': true, '#': true, '%': true, '=': true, '.': true, ',': true,
	'T': true, 't': true, '_': true, '<': true, '>': true, 'X': true,
	'^': true, '~': true, 'L': true, 'F': true, '@': true,
	'1': true, '2': true, '3': true, '4': true, '5': true, '6': true, '7': true,
}

func TestLensGlyphsDisjointFromBaseSet(t *testing.T) {
	for name, def := range lenses {
		for _, g := range lensGlyphSet(name) {
			if name == "designations" && g == 'd' {
				continue // documented exception: refines the base 'd' overlay
			}
			if baseTerrainGlyphSet[g] {
				t.Errorf("lens %q glyph %q collides with the base terrain set", name, string(g))
			}
		}
		if def.Legend == "" {
			t.Errorf("lens %q must have a non-empty legend addendum", name)
		}
	}
}

func TestBuildingCategoryGlyph(t *testing.T) {
	cases := map[string]rune{
		"Workshop": 'W', "Furnace": 'W',
		"Bed": 'B', "Chair": 'B', "Table": 'B',
		"Door": 'D', "Hatch": 'D',
		"Stockpile": 'S',
		"ScrewPump": 'M', "Well": 'M', "Bridge": 'M',
		"Trap": 'P', "Cage": 'P',
		"Construction": 'C',
		"TradeDepot": 'O', "Wagon": 'O',
	}
	for buildingType, want := range cases {
		got := buildingCategoryGlyph(buildingType)
		if got != want {
			t.Errorf("buildingCategoryGlyph(%q) = %q, want %q", buildingType, string(got), string(want))
		}
	}
}

func TestBuildingCategoryGlyph_CaseEncodesConstructionState(t *testing.T) {
	if g := buildingOverlayGlyph(buildingListEntry{Type: "Bed", Done: true}); g != 'B' {
		t.Errorf("built furniture must render uppercase, got %q", string(g))
	}
	if g := buildingOverlayGlyph(buildingListEntry{Type: "Bed", Done: false}); g != 'b' {
		t.Errorf("planned/in-progress furniture must render lowercase, got %q", string(g))
	}
}

func TestDesignationKindGlyph(t *testing.T) {
	cases := map[int16]rune{0: 'd', 1: 'c', 2: 'r', 3: 's', 4: 'm'}
	for kind, want := range cases {
		if got := designationKindGlyph(kind); got != want {
			t.Errorf("designationKindGlyph(%d) = %q, want %q", kind, string(got), string(want))
		}
	}
}

func TestLensNamesErrorMessage(t *testing.T) {
	names := strings.Join(lensNames(), ", ")
	if !strings.Contains(names, "buildings") || !strings.Contains(names, "designations") {
		t.Fatalf("lensNames must list both registered lenses, got %q", names)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcpserver/... -run "TestLens|TestBuilding|TestDesignationKind" -v`
Expected: FAIL — none of `lenses`, `lensGlyphSet`, `buildingCategoryGlyph`, `buildingOverlayGlyph`, `designationKindGlyph`, `lensNames` exist yet.

- [ ] **Step 3: Implement the lens registry and gather functions**

```go
// internal/mcpserver/lenses.go
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/df-ai/orchestrator/internal/mapview"
)

// LensDef is one registered look overlay. Adding a future lens (zones,
// traffic, burrows — see design doc "Extensibility test") is exactly one
// LensDef entry plus one enum literal on the look tool's input schema —
// zero changes to RenderCrop or the paint pipeline.
type LensDef struct {
	Name   string
	Legend string
	Gather func(ctx context.Context, b *Bridge, s *mapview.Slice, z int16) (mapview.Overlay, error)
}

var lenses = map[string]LensDef{
	"buildings": {
		Name:   "buildings",
		Legend: "lens=buildings: W workshop/furnace B furniture D door/hatch S stockpile M mechanism/well/bridge P trap/cage C construction O other — UPPERCASE built, lowercase planned/in-progress; exact type: buildings tool",
		Gather: gatherBuildingsLens,
	},
	"designations": {
		Name:   "designations",
		Legend: "lens=designations: d dig c channel r ramp s stair m smooth — designated, not yet dug",
		Gather: gatherDesignationsLens,
	},
}

// lensNames returns the registered lens names, sorted, for error messages.
func lensNames() []string {
	names := make([]string, 0, len(lenses))
	for n := range lenses {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// lensGlyphSet returns every glyph a lens can emit — used by the
// disjointness test and documented here as the single source of truth
// for what each lens is allowed to paint.
func lensGlyphSet(name string) []rune {
	switch name {
	case "buildings":
		return []rune{'W', 'B', 'D', 'S', 'M', 'P', 'C', 'O',
			'w', 'b', 'd', 's', 'm', 'p', 'c', 'o'} // lowercase = planned
	case "designations":
		return []rune{'d', 'c', 'r', 's', 'm'}
	default:
		return nil
	}
}

// buildingCategoryGlyph maps a DFHack building_type enum key name (as
// returned by list_buildings' "type" field) to one of 8 category
// glyphs. Every df::building_type value from the DFHack 53.15-r1
// checkout (library/xml/df.d_basics.xml) is covered; Civzone is
// deliberately excluded (that's the future zones lens, a different data
// plane) and NONE never appears in a real building list.
func buildingCategoryGlyph(buildingType string) rune {
	switch buildingType {
	case "Workshop", "Furnace":
		return 'W'
	case "Bed", "Chair", "Table", "Cabinet", "Box", "Coffin", "Statue",
		"Weaponrack", "Armorstand", "TractionBench", "Slab", "Bookcase",
		"DisplayFurniture", "Instrument", "NestBox", "Hive", "Nest":
		return 'B'
	case "Door", "Hatch", "Floodgate", "GrateWall", "GrateFloor",
		"BarsVertical", "BarsFloor", "WindowGlass", "WindowGem":
		return 'D'
	case "Stockpile":
		return 'S'
	case "ScrewPump", "GearAssembly", "AxleHorizontal", "AxleVertical",
		"WaterWheel", "Windmill", "Rollers", "Well", "Bridge", "Support", "Chain":
		return 'M'
	case "Trap", "AnimalTrap", "Cage", "SiegeEngine", "ArcheryTarget":
		return 'P'
	case "Construction":
		return 'C'
	case "FarmPlot": // reserved: cannot exist in a fort today (no farm-plot build type)
		return 'G'
	default: // TradeDepot, Shop, Wagon, RoadDirt, RoadPaved, Weapon, and any future/unmapped type
		return 'O'
	}
}

// buildingOverlayGlyph applies the built/planned case encoding on top of
// the category glyph — the mechanism that makes a silently-dying
// building plan visible: a lowercase glyph that never becomes uppercase
// and then vanishes from the lens IS the death announcement DF never
// gives.
func buildingOverlayGlyph(e buildingListEntry) rune {
	g := buildingCategoryGlyph(e.Type)
	if e.Done {
		return g
	}
	return []rune(string(g))[0] + ('a' - 'A') // uppercase -> lowercase
}

func gatherBuildingsLens(ctx context.Context, b *Bridge, s *mapview.Slice, z int16) (mapview.Overlay, error) {
	args := fmt.Sprintf(`{"z":%d}`, z)
	raw, err := b.Query(ctx, "list_buildings", args)
	if err != nil {
		return mapview.Overlay{}, err
	}
	var resp struct {
		Buildings []buildingListEntry `json:"buildings"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return mapview.Overlay{}, fmt.Errorf("buildings lens: unparseable list_buildings response: %w", err)
	}
	marks := map[[2]int16]rune{}
	for _, e := range resp.Buildings {
		g := buildingOverlayGlyph(e)
		if e.X1 != 0 || e.Y1 != 0 || e.X2 != 0 || e.Y2 != 0 {
			for x := e.X1; x <= e.X2; x++ {
				for y := e.Y1; y <= e.Y2; y++ {
					marks[[2]int16{int16(x), int16(y)}] = g
				}
			}
		} else {
			// Older plugin (pre this task) omits extents — fall back to
			// center-only paint per the design doc's stated fallback.
			marks[[2]int16{int16(e.X), int16(e.Y)}] = g
		}
	}
	return mapview.Overlay{Marks: marks}, nil
}

func designationKindGlyph(kind int16) rune {
	switch kind {
	case 1:
		return 'c'
	case 2:
		return 'r'
	case 3:
		return 's'
	case 4:
		return 'm'
	default:
		return 'd'
	}
}

func gatherDesignationsLens(ctx context.Context, b *Bridge, s *mapview.Slice, z int16) (mapview.Overlay, error) {
	marks := map[[2]int16]rune{}
	for _, dk := range s.DesignationKinds {
		marks[[2]int16{dk[0], dk[1]}] = designationKindGlyph(dk[2])
	}
	return mapview.Overlay{Marks: marks}, nil
}

// unknownLensError renders the truthful-error text for an unrecognized
// lens name, listing the registered names (never a bare failure — this
// project's ACK/error discipline).
func unknownLensError(name string) string {
	return "unknown lens " + strconv.Quote(name) + " — registered lenses: " + fmt.Sprint(lensNames())
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mcpserver/... -run "TestLens|TestBuilding|TestDesignationKind" -v`
Expected: PASS

- [ ] **Step 5: Run the full mcpserver package test suite**

Run: `go test ./internal/mcpserver/... -v`
Expected: PASS (no regressions to existing tests).

- [ ] **Step 6: Commit**

```bash
git add internal/mcpserver/lenses.go internal/mcpserver/lenses_test.go
git commit -m "feat(mcpserver): buildings and designations look lenses

8-category building glyphs cross-checked against all real df::building_type
values (52, verified against the DFHack 53.15-r1 checkout); case encodes
built vs planned, making silently-dying building plans visible for the
first time. Designations lens gives per-kind detail (dig/channel/ramp/
stair/smooth) beyond the base 'd' overlay. Glyph-disjointness enforced by
a table-driven test walking the registry, per the design doc's global
invariant."
```

---

## Task 7: Wire `lens` into the `look` tool

**Files:**
- Modify: `internal/mcpserver/tools_percept.go`
- Modify: `internal/mcpserver/server_test.go` (minToolArgs — `look` already has no required-only entry needed since `lens` is optional, but confirm the nil-bridge sweep still passes)

**Interfaces:**
- Consumes: `lenses`, `unknownLensError`, `mapview.Overlay` (Task 6), `mapview.RenderCrop` (Task 3).

- [ ] **Step 1: Write the failing integration test**

```go
// internal/mcpserver/tools_percept_test.go — add (file already exists per this session's earlier work; append to it)
func TestLook_UnknownLensReturnsRegisteredNames(t *testing.T) {
	// This is a light unit test of the error path only — a full nil-bridge
	// MCP round trip is already covered by TestEveryToolNilBridge in
	// server_test.go, which exercises `look` with lens omitted. This test
	// checks the unknownLensError helper directly (already covered by
	// TestLensNamesErrorMessage in lenses_test.go) plus that the `look`
	// tool actually calls it — covered by re-running the nil-bridge sweep
	// with a lens arg added to minToolArgs (Step 3 below) rather than a
	// bespoke MCP client test here, to avoid duplicating transport-level
	// test scaffolding that already exists.
	got := unknownLensError("nonexistent")
	if !strings.Contains(got, "unknown lens") {
		t.Fatalf("expected an 'unknown lens' message, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it currently passes trivially (it's testing an already-implemented helper) — this step exists to confirm the test file compiles before the real change**

Run: `go test ./internal/mcpserver/... -run TestLook_UnknownLensReturnsRegisteredNames -v`
Expected: PASS (this is intentionally a thin test; the real behavior change is verified by the nil-bridge sweep in Step 4).

- [ ] **Step 3: Add the `lens` param to `look` and wire the gather+render call**

```go
// internal/mcpserver/tools_percept.go:14-50 — replace the look tool registration
type lookIn struct {
	X      int    `json:"x" jsonschema:"center x"`
	Y      int    `json:"y" jsonschema:"center y"`
	Z      int    `json:"z" jsonschema:"z-level to view"`
	Radius int    `json:"radius,omitempty" jsonschema:"half-width of the crop, default 12, max 15"`
	Lens   string `json:"lens,omitempty" jsonschema:"optional overlay: buildings|designations. Paints one annotation layer onto the terrain grid; omit for the plain terrain view (which still shows dig designations as 'd'). Exact building/zone types via buildings/building_status."`
}
mcp.AddTool(srv, &mcp.Tool{
	Name:        "look",
	Description: "Render a small annotated map crop of one z-level around (x,y). Glyph grid with legend; dwarves marked @, dig designations marked d. Pass lens=buildings or lens=designations for detail overlays. '?' tiles are hidden fog — solid undug ground you CAN designate digging into. Use for local layout checks; use find_dig_site for choosing dig locations.",
}, func(ctx context.Context, req *mcp.CallToolRequest, in lookIn) (*mcp.CallToolResult, any, error) {
	r := in.Radius
	if r <= 0 {
		r = 12
	}
	if r > 15 {
		r = 15
	}
	x1, y1 := int16(in.X-r), int16(in.Y-r)
	x2, y2 := int16(in.X+r), int16(in.Y+r)
	if x1 < 0 {
		x1 = 0
	}
	if y1 < 0 {
		y1 = 0
	}
	s, err := b.MapSlice(ctx, x1, y1, int16(in.Z), x2, y2)
	if err != nil {
		return withDash(b, ctx, "look failed: "+err.Error()), nil, nil
	}
	marks := map[[2]int16]rune{}
	for _, d := range b.Snapshot().Entities.Dwarves {
		if d.Z == int16(in.Z) {
			marks[[2]int16{d.X, d.Y}] = '@'
		}
	}
	overlays := []mapview.Overlay{{Marks: marks}}
	legendExtra := ""
	if in.Lens != "" {
		def, ok := lenses[in.Lens]
		if !ok {
			return withDash(b, ctx, unknownLensError(in.Lens)), nil, nil
		}
		lensOverlay, err := def.Gather(ctx, b, s, int16(in.Z))
		if err != nil {
			return withDash(b, ctx, "lens failed: "+err.Error()), nil, nil
		}
		overlays = append(overlays, lensOverlay)
		legendExtra = def.Legend
	}
	return withDash(b, ctx, mapview.RenderCrop(s, overlays, legendExtra)), nil, nil
})
```

- [ ] **Step 4: Run the full registration sweep to confirm `look` still passes with the new optional field**

Run: `go test ./internal/mcpserver/... -run TestEveryToolNilBridge -v`
Expected: PASS — `look` has no required fields added (`lens` is `omitempty`), so no `minToolArgs` change is needed; the sweep calling it with `{}` must still return readable text on a nil bridge (verify `b.MapSlice` on a nil/disconnected bridge returns a clean error string, not a panic — this is pre-existing behavior via `b.Query`'s `!b.Connected()` check, unchanged by this task).

- [ ] **Step 5: Run the full test suite**

Run: `go build ./... && go test ./... > /path/to/scratchpad/step5.log 2>&1; echo exit=$?; cat /path/to/scratchpad/step5.log`
Expected: exit=0, all packages ok.

- [ ] **Step 6: Commit**

```bash
git add internal/mcpserver/tools_percept.go internal/mcpserver/tools_percept_test.go
git commit -m "feat(mcpserver): wire the lens param into look

One optional enum field; omitted = today's view plus the new always-on
'd' designation glyph. lens=buildings/designations paint their overlay
last and add their legend addendum only when requested — the GIS
dynamic-legend rule this design is built around."
```

---

## Task 8: Named-places persistence store

**Files:**
- Create: `internal/mcpserver/places.go` (Part 1: `Place`, `PlaceStore`, load/save — tools come in Task 9)
- Test: `internal/mcpserver/places_test.go`
- Create directory: `fortress/state/` (via the first test/save call — no manual mkdir needed if the save function creates it)

**Interfaces:**
- Produces: `type Place struct{ Anchor topology.Coord; Name string }`, `type PlaceStore struct{ ... }` (unexported fields), `func NewPlaceStore(path string) *PlaceStore`, `func (ps *PlaceStore) Load() error`, `func (ps *PlaceStore) Save() error`, `func (ps *PlaceStore) Set(anchor topology.Coord, name string)`, `func (ps *PlaceStore) All() []Place`, `func (ps *PlaceStore) Reconcile(rg topology.RegionGraph)` (drops entries whose anchor no longer resolves to any region).

- [ ] **Step 1: Write the failing round-trip test**

```go
// internal/mcpserver/places_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcpserver/... -run TestPlaceStore -v`
Expected: FAIL — `Place`, `PlaceStore`, `NewPlaceStore` undefined.

- [ ] **Step 3: Implement `PlaceStore`**

```go
// internal/mcpserver/places.go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mcpserver/... -run TestPlaceStore -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcpserver/places.go internal/mcpserver/places_test.go
git commit -m "feat(mcpserver): PlaceStore — persisted named regions

Anchor-tile-keyed (not region-ID-keyed, since region IDs aren't stable
across RegionGraph rebuilds), JSON-persisted to fortress/state/ (kept
distinct from fortress/memory/'s AI-authored-narrative charter). Survives
the df-mcp restart pattern this session hit three times. Reconcile()
drops names whose anchor no longer resolves, silently — no error worth
surfacing for a name going stale."
```

---

## Task 9: `name_place` / `list_places` tools

**Files:**
- Modify: `internal/mcpserver/places.go` (add tool registration + wiring)
- Modify: `internal/mcpserver/bridge.go` (Bridge gains a `Places *PlaceStore` field, initialized in `NewBridge` and loaded/reconciled on `OnFullState`)
- Modify: `internal/mcpserver/server_test.go` (`minToolArgs` for `name_place`)

**Interfaces:**
- Consumes: `PlaceStore` (Task 8), `topology.BuildRegionGraph`/`RegionAt` (Task 1), `Bridge.Topo()`.
- Produces: `registerPlaceTools(srv *mcp.Server, b *Bridge)` (called from `New` alongside the other `register*Tools` calls — find that call site first).

- [ ] **Step 1: Find the tool-registration call site**

Search `internal/mcpserver/server.go` for `registerPerceptTools|registerActionTools|registerStateTools` (use the Grep tool, not a shell command). Expected: a `New` function calling each `register*Tools(srv, b)` — add `registerPlaceTools(srv, b)` alongside them in Step 5 below.

- [ ] **Step 2: Write the failing test for the anchor-resolution helper**

```go
// internal/mcpserver/places_test.go — append
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/mcpserver/... -run TestResolveAnchor -v`
Expected: FAIL — `resolveAnchor` undefined.

- [ ] **Step 4: Implement `resolveAnchor`, the tools, and wire `Bridge.Places`**

```go
// internal/mcpserver/places.go — append
import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
		anchor, err := resolveAnchor(topo, int16(in.X), int16(in.Y), int16(in.Z))
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
```

```go
// internal/mcpserver/bridge.go — Bridge struct, add field
type Bridge struct {
	Client     *dfhack.Client
	Exec       *commands.CommandExecutor
	WM         *worldmodel.WorldModel
	Populator  *worldmodel.Populator
	Preds      *predicate.Library
	Blueprints *blueprints.BlueprintLibrary
	Places     *PlaceStore
	Logger     *logging.Logger
	port       uint16
}
```

```go
// internal/mcpserver/bridge.go — NewBridge, add after the Blueprints line in the struct literal
	b := &Bridge{
		Client: client, Exec: exec, WM: wm, Populator: populator,
		Preds:      predicate.NewStarterLibrary(),
		Blueprints: blueprints.NewBlueprintLibrary("blueprints"),
		Places:     NewPlaceStore(filepath.Join("fortress", "state", "places.json")),
		Logger:     logger,
		port:       cfg.ListenPort,
	}
```

(Add `"path/filepath"` to `bridge.go`'s import block.)

```go
// internal/mcpserver/bridge.go — inside the client.SetOnFullState callback,
// after `populator.OnFullState(state)`, add:
		if err := b.Places.Load(); err != nil {
			logger.Error("places: load failed", err)
		} else {
			b.Places.Reconcile(topology.BuildRegionGraph(topo))
		}
```

- [ ] **Step 5: Wire `registerPlaceTools` into the server's tool registration**

Add `registerPlaceTools(srv, b)` to `internal/mcpserver/server.go`'s `New` function, alongside the existing `register*Tools` calls found in Step 1.

- [ ] **Step 6: Run test to verify `resolveAnchor` passes**

Run: `go test ./internal/mcpserver/... -run TestResolveAnchor -v`
Expected: PASS

- [ ] **Step 7: Add `minToolArgs` entries and run the full registration sweep**

```go
// internal/mcpserver/server_test.go — minToolArgs map, add
	"name_place": {"x": 1, "y": 1, "z": 1, "name": "test"},
	// list_places has no required fields — {} default is fine, no entry needed.
```

Run: `go test ./internal/mcpserver/... -run TestEveryToolNilBridge -v`
Expected: PASS — `name_place` and `list_places` both return readable text against a nil bridge (`noExec(b)` already guards `name_place`; `list_places` doesn't call `noExec` since it reads `b.Places` directly — verify `b.Places` is nil-safe on a nil `*Bridge`, or add a nil-guard matching the `noExec` pattern if `b` can be nil here per the test harness's `New(nil)` construction — check `New(nil)`'s behavior for `Places` before assuming; if `Places` ends up nil on a nil bridge, add `if b == nil || b.Places == nil { return TextResult("NOT CONNECTED...") }` to `list_places` to match every other tool's nil-safety contract).

- [ ] **Step 8: Run the full test suite**

Run: `go build ./... && go test ./... > /path/to/scratchpad/step8.log 2>&1; echo exit=$?; cat /path/to/scratchpad/step8.log`
Expected: exit=0.

- [ ] **Step 9: Commit**

```bash
git add internal/mcpserver/places.go internal/mcpserver/bridge.go internal/mcpserver/server.go internal/mcpserver/server_test.go
git commit -m "feat(mcpserver): name_place / list_places tools

Wired to PlaceStore, loaded + reconciled against the fresh region graph
on every FULL_STATE (covers reconnects, not just the initial connect).
General-purpose spatial labeling independent of zones — for hallways,
quarry sections, anything worth tracking that may never have a formal
DF zone type."
```

---

## Task 10: `elevation_view` tool

**Files:**
- Modify: `internal/mapview/render.go` (add `RenderElevation`)
- Modify: `internal/mcpserver/tools_percept.go` (new tool)
- Test: `internal/mapview/render_test.go`, `internal/mcpserver/server_test.go` (minToolArgs)

**Interfaces:**
- Consumes: `Bridge.ColumnProfile` (existing), `mapview.ColumnProfile`/`ColumnLevel` (existing).
- Produces: `func RenderElevation(columns []*ColumnProfile, axis string, fixed int16) string`.

- [ ] **Step 1: Write the failing test for `RenderElevation`**

```go
// internal/mapview/render_test.go — append
func TestRenderElevation_XAxisSweep(t *testing.T) {
	col1 := &ColumnProfile{X: 10, Y: 50, Levels: []ColumnLevel{
		{Z: 100, Glyph: ".", Shape: "floor", Material: "grass"},
		{Z: 99, Glyph: "#", Shape: "wall", Material: "stone"},
	}}
	col2 := &ColumnProfile{X: 11, Y: 50, Levels: []ColumnLevel{
		{Z: 100, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true},
		{Z: 99, Glyph: "#", Shape: "wall", Material: "stone"},
	}}
	out := RenderElevation([]*ColumnProfile{col1, col2}, "x", 50)
	if !strings.Contains(out, "elevation along x=10..11 at y=50") {
		t.Fatalf("missing header:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	var z100Line, z99Line string
	for _, l := range lines {
		if strings.HasPrefix(l, "z=100 ") {
			z100Line = l
		}
		if strings.HasPrefix(l, "z=99 ") {
			z99Line = l
		}
	}
	if !strings.Contains(z100Line, ".?") {
		t.Fatalf("z=100 row must read '.?' (col1 floor, col2 hidden wall):\n%s", z100Line)
	}
	if !strings.Contains(z99Line, "##") {
		t.Fatalf("z=99 row must read '##':\n%s", z99Line)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mapview/... -run TestRenderElevation -v`
Expected: FAIL — `RenderElevation` undefined.

- [ ] **Step 3: Implement `RenderElevation`**

```go
// internal/mapview/render.go — append
// RenderElevation renders a vertical slice along a line (the third
// orthogonal plane, alongside look's plan view and cross_section's
// single-column bore) — one row per Z, one glyph column per swept
// x (or y) position. Reuses each ColumnProfile's own per-level
// classification (shape/material/hidden/fluids), matching cross_section's
// existing rendering exactly, just composed across a line instead of a
// single point.
//
// columns must all share the same Z range and be pre-sorted by the swept
// coordinate (ascending) — RenderElevation does not sort or validate
// this; the caller (tools_percept.go's elevation_view handler) builds
// them in sweep order.
func RenderElevation(columns []*ColumnProfile, axis string, fixed int16) string {
	if len(columns) == 0 {
		return "no columns to render"
	}
	var sb strings.Builder
	first, last := columns[0], columns[len(columns)-1]
	if axis == "x" {
		fmt.Fprintf(&sb, "elevation along x=%d..%d at y=%d, top to bottom:\n", first.X, last.X, fixed)
	} else {
		fmt.Fprintf(&sb, "elevation along y=%d..%d at x=%d, top to bottom:\n", first.Y, last.Y, fixed)
	}

	// Index each column's levels by Z for row-major access.
	byZ := make([]map[int16]ColumnLevel, len(columns))
	for i, c := range columns {
		m := make(map[int16]ColumnLevel, len(c.Levels))
		for _, lv := range c.Levels {
			m[lv.Z] = lv
		}
		byZ[i] = m
	}

	// All columns are expected to share the same Z range (same z_top/z_bottom
	// request) — use the first column's levels to drive row order.
	for _, lv0 := range columns[0].Levels {
		fmt.Fprintf(&sb, "z=%d ", lv0.Z)
		for i := range columns {
			lv, ok := byZ[i][lv0.Z]
			if !ok {
				sb.WriteString("?")
				continue
			}
			sb.WriteString(lv.Glyph)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\nlegend: " + Legend + "\n")
	return sb.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mapview/... -run TestRenderElevation -v`
Expected: PASS

- [ ] **Step 5: Add the `elevation_view` MCP tool**

```go
// internal/mcpserver/tools_percept.go — add to registerPerceptTools, after the cross_section tool
type elevationIn struct {
	Axis    string `json:"axis" jsonschema:"x|y — sweep across x at fixed y, or across y at fixed x"`
	X       int    `json:"x,omitempty" jsonschema:"required when axis=y: the fixed x"`
	Y       int    `json:"y,omitempty" jsonschema:"required when axis=x: the fixed y"`
	X1      int    `json:"x1,omitempty" jsonschema:"required when axis=x: sweep start x"`
	X2      int    `json:"x2,omitempty" jsonschema:"required when axis=x: sweep end x"`
	Y1      int    `json:"y1,omitempty" jsonschema:"required when axis=y: sweep start y"`
	Y2      int    `json:"y2,omitempty" jsonschema:"required when axis=y: sweep end y"`
	ZTop    int    `json:"z_top" jsonschema:"top z"`
	ZBottom int    `json:"z_bottom" jsonschema:"bottom z"`
}
mcp.AddTool(srv, &mcp.Tool{
	Name:        "elevation_view",
	Description: "Vertical slice along a LINE (not a single column like cross_section) — the third orthogonal plane. axis=x sweeps across x at a fixed y; axis=y sweeps across y at a fixed x. Bounded to 30 columns per call (matches look's radius cap) to keep the underlying column_profile calls bounded.",
}, func(ctx context.Context, req *mcp.CallToolRequest, in elevationIn) (*mcp.CallToolResult, any, error) {
	if r := noExec(b); r != nil {
		return r, nil, nil
	}
	const maxSweep = 30
	var coords []int16
	switch in.Axis {
	case "x":
		if in.X2 < in.X1 {
			return withDash(b, ctx, "axis=x requires x1<=x2"), nil, nil
		}
		if in.X2-in.X1+1 > maxSweep {
			return withDash(b, ctx, fmt.Sprintf("sweep too wide: %d columns, max %d", in.X2-in.X1+1, maxSweep)), nil, nil
		}
		for x := in.X1; x <= in.X2; x++ {
			coords = append(coords, int16(x))
		}
	case "y":
		if in.Y2 < in.Y1 {
			return withDash(b, ctx, "axis=y requires y1<=y2"), nil, nil
		}
		if in.Y2-in.Y1+1 > maxSweep {
			return withDash(b, ctx, fmt.Sprintf("sweep too wide: %d columns, max %d", in.Y2-in.Y1+1, maxSweep)), nil, nil
		}
		for y := in.Y1; y <= in.Y2; y++ {
			coords = append(coords, int16(y))
		}
	default:
		return withDash(b, ctx, fmt.Sprintf("unknown axis %q — use x or y", in.Axis)), nil, nil
	}

	var columns []*mapview.ColumnProfile
	for _, coord := range coords {
		var c *mapview.ColumnProfile
		var err error
		if in.Axis == "x" {
			c, err = b.ColumnProfile(ctx, coord, int16(in.Y), int16(in.ZTop), int16(in.ZBottom))
		} else {
			c, err = b.ColumnProfile(ctx, int16(in.X), coord, int16(in.ZTop), int16(in.ZBottom))
		}
		if err != nil {
			return withDash(b, ctx, fmt.Sprintf("elevation_view failed at column %d: %v", coord, err)), nil, nil
		}
		columns = append(columns, c)
	}
	fixed := int16(in.Y)
	if in.Axis == "y" {
		fixed = int16(in.X)
	}
	return withDash(b, ctx, mapview.RenderElevation(columns, in.Axis, fixed)), nil, nil
})
```

- [ ] **Step 6: Add `minToolArgs` entry and run the registration sweep**

```go
// internal/mcpserver/server_test.go — minToolArgs map, add
	"elevation_view": {"axis": "x", "x1": 0, "x2": 2, "y": 1, "z_top": 5, "z_bottom": 0},
```

Run: `go test ./internal/mcpserver/... -run TestEveryToolNilBridge -v`
Expected: PASS

- [ ] **Step 7: Run the full test suite**

Run: `go build ./... && go test ./... > /path/to/scratchpad/step7.log 2>&1; echo exit=$?; cat /path/to/scratchpad/step7.log`
Expected: exit=0.

- [ ] **Step 8: Commit**

```bash
git add internal/mapview/render.go internal/mapview/render_test.go internal/mcpserver/tools_percept.go internal/mcpserver/server_test.go
git commit -m "feat(mapview,mcpserver): elevation_view — the third orthogonal plane

Both axes from the start (DF layouts aren't axis-biased). Composes
existing column_profile calls (bounded to 30 columns, matching look's
radius cap) rather than adding a new plugin query — no new rendering
primitive, RenderColumn's per-level classification reused via
ColumnProfile.Levels."
```

---

## Task 11: Plugin rebuild, deploy verification, and full regression sweep

**Files:** none new — this task verifies Tasks 4-10's combined state, matching the "bundle the plugin deploy cycle" note left in Task 4.

- [ ] **Step 1: Full Go build and test**

Run: `go build ./... > /path/to/scratchpad/final-build.log 2>&1; echo exit=$?; cat /path/to/scratchpad/final-build.log`
Expected: exit=0, empty/clean log.

Run: `go test ./... > /path/to/scratchpad/final-test.log 2>&1; echo exit=$?; cat /path/to/scratchpad/final-test.log`
Expected: exit=0, every package `ok` or `[no test files]`.

- [ ] **Step 2: Plugin rebuild**

Run: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release > /path/to/scratchpad/final-plugin.log 2>&1; echo exit=$?; cat /path/to/scratchpad/final-plugin.log`
Expected: exit=0. Retry once if it fails near `generate_headers` with exit `-1073741819` before treating it as a real failure (known flaky Perl codegen, established in this project's build history).

- [ ] **Step 3: Verify artifact freshness**

Compare the mtime of `C:/Users/zmanl/Projects/dfhack-build/build/plugins/df_ai_protocol/Release/df_ai_protocol.plug.dll` against the newest of `dfhack-plugin/queries.cpp` and any other `.cpp`/`.h` touched in Task 4. The artifact must be newer — if not, delete it and rebuild once to force a relink (a stale artifact has fooled this project's build verification before, per `docs/guides/plugin-build.md`).

- [ ] **Step 4: Do NOT deploy**

Leave the DLL at its build-output path only. Deploying requires DF closed (file lock) and is a user-facing action outside this plan's scope — report the artifact path and freshness to the user for them to deploy when ready, matching this session's established pattern for every prior plugin change.

- [ ] **Step 5: Confirm the design doc's testing section is fully covered**

Cross-check against `specs/009-culture-and-learning/design-perception-round2.md`'s "Testing" section:
- Overlay pipeline paint order + footnote generation — Task 3.
- Glyph-disjointness invariant — Task 6.
- `RenderColumn`-reuse correctness for `elevation_view` — Task 10 (via `RenderElevation` reusing `ColumnLevel.Glyph`).
- Region graph fixtures (disconnected rooms, stair-linked multi-z, the exact 8x8-room-4-tiles-away shape) — Task 1 covers disconnected rooms and multi-z; the exact incident-shaped fixture is implicitly covered by `TestConnectorSuggestion_DisconnectedReturnsSuggestion` in Task 2 (10 tiles away, same failure class as the session's 4-tile-away incident) — if the implementing agent wants the literal 8x8-at-4-tiles fixture reproduced, add it as an explicit regression test in Task 2 before closing this task out.
- `designate_dig` connected/disconnected suggestion text — Task 2.
- `name_place`/`list_places` round-trip + anchor stability — Task 8/9.
- Registration sweep entries — Tasks 9 and 10.

If any gap is found, add the missing test now rather than deferring — this step exists specifically to catch a task-boundary miss before calling the plan done.

- [ ] **Step 6: Final commit (if Step 5 added anything)**

```bash
git add -A
git status --short  # review before committing — confirm only expected files
git commit -m "test: close any remaining design-doc testing-section gaps"
```

(Skip this commit if Step 5 found nothing to add.)

---

## Self-review notes (from the writing-plans process)

- **Spec coverage**: all 5 design-doc components have at least one task — region graph (1), reachability (2), overlay pipeline + base glyph (3), plugin wire additions (4), Go decode (5), lenses (6), lens wiring (7), named places (8-9), elevation_view (10), integration (11).
- **Type consistency checked**: `topology.Coord` (Task 1) is the type every later task uses for anchors/regions — Tasks 2, 8, 9 all import and use it identically, no renamed/duplicate coordinate type introduced. `mapview.Overlay` (Task 3) is the type Task 6's `LensDef.Gather` returns and Task 7's `look` handler consumes — signatures match exactly. `buildingListEntry` (Task 5) is reused verbatim by Task 6's `gatherBuildingsLens`, not re-declared.
- **No placeholders**: every step has complete, concrete code — no "add error handling" or "similar to Task N" shortcuts. The one intentionally-deferred item (Task 11 Step 5's optional extra fixture) is deferred with an explicit reason and instruction, not a vague TBD.
