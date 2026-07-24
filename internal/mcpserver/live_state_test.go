package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/blueprints"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// TestParseRegionScanResponse covers the plugin's region_scan JSON shape
// directly against canned bytes — mirrors TestParseFortFootprintResponse's
// pattern for the sibling fort_footprint query.
func TestParseRegionScanResponse(t *testing.T) {
	raw := []byte(`{"x1":10,"y1":20,"z1":4,"x2":12,"y2":22,"z2":4,"count":3,"tiles":[` +
		`{"x":10,"y":20,"z":4,"kind":"floor"},` +
		`{"x":11,"y":20,"z":4,"kind":"stair_down"},` +
		`{"x":10,"y":22,"z":4,"kind":"construction"}` +
		`]}`)
	tiles, err := parseRegionScanResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tiles) != 3 {
		t.Fatalf("got %d tiles, want 3: %+v", len(tiles), tiles)
	}
	want := []blueprints.RegionScanTile{
		{X: 10, Y: 20, Z: 4, Kind: "floor"},
		{X: 11, Y: 20, Z: 4, Kind: "stair_down"},
		{X: 10, Y: 22, Z: 4, Kind: "construction"},
	}
	for i, w := range want {
		if tiles[i] != w {
			t.Errorf("tile %d = %+v, want %+v", i, tiles[i], w)
		}
	}
}

// TestParseRegionScanResponse_UnparseablePayload mirrors
// TestParseFortFootprintResponse's malformed-JSON case.
func TestParseRegionScanResponse_UnparseablePayload(t *testing.T) {
	if _, err := parseRegionScanResponse([]byte("not json")); err == nil {
		t.Fatal("expected an error for an unparseable payload")
	}
}

// TestParseRegionScanResponse_ThenCreateBlueprintFromRegionScan drives
// canned region_scan JSON all the way through the blueprint sibling
// function (parseRegionScanResponse -> blueprints.CreateBlueprintFromRegionScan),
// the same pipeline saveBlueprint's live call now uses instead of the
// session-delta Modifications overlay.
func TestParseRegionScanResponse_ThenCreateBlueprintFromRegionScan(t *testing.T) {
	raw := []byte(`{"x1":5,"y1":5,"z1":2,"x2":6,"y2":5,"z2":2,"count":2,"tiles":[` +
		`{"x":5,"y":5,"z":2,"kind":"floor"},` +
		`{"x":6,"y":5,"z":2,"kind":"construction"}` +
		`]}`)
	tiles, err := parseRegionScanResponse(raw)
	if err != nil {
		t.Fatalf("parseRegionScanResponse: %v", err)
	}
	region := modifications.Region{XMin: 5, XMax: 6, YMin: 5, YMax: 5, ZMin: 2, ZMax: 2}
	bp, err := blueprints.CreateBlueprintFromRegionScan(tiles, region, "captured")
	if err != nil {
		t.Fatalf("CreateBlueprintFromRegionScan: %v", err)
	}
	if len(bp.Digs) != 1 {
		t.Fatalf("got %d digs, want 1 (the construction tile must be skipped): %+v", len(bp.Digs), bp.Digs)
	}
	if bp.Digs[0].X != 0 || bp.Digs[0].Y != 0 || bp.Digs[0].Z != 0 || bp.Digs[0].DigType != "default" {
		t.Errorf("dig = %+v, want (0,0,0) dig_type default", bp.Digs[0])
	}
}

// TestCountDugTiles mirrors CreateBlueprintFromRegionScan's "skip
// construction, keep everything else" filter — the predicate side reads
// this same count so a save_blueprint capture and check_goals' live count
// never disagree about what "dug" means for the identical region_scan
// response, when the box is a hand-picked one save_blueprint already knows
// is a real dig site (excludeAmbiguousFloor=false).
func TestCountDugTiles(t *testing.T) {
	tiles := []blueprints.RegionScanTile{
		{Kind: "floor"}, {Kind: "stair_up"}, {Kind: "construction"}, {Kind: "ramp"},
	}
	if n := countDugTiles(tiles, false); n != 3 {
		t.Fatalf("countDugTiles = %d, want 3", n)
	}
	if n := countDugTiles(nil, false); n != 0 {
		t.Fatalf("countDugTiles(nil) = %d, want 0", n)
	}
}

// TestCountDugTiles_ExcludesAmbiguousFloorWhenFlagged guards the review
// fix: liveDugTileCount's auto-derived bbox can span untouched natural
// terrain (a bare cavern floor at depth, or non-grass bare surface dirt)
// between two separate dig sites — region_scan's "floor" kind cannot tell
// that apart from genuinely-dug floor by tiletype alone (see
// dfhack-plugin/queries.cpp's regionScanKind doc comment). When the
// caller passes excludeAmbiguousFloor=true (mirroring
// fortFootprintMapState's may_include_natural_cave flag), "floor" tiles
// must NOT inflate the count — but every stronger-signal kind (stairs,
// track, construction, ramp) still must, since those remain unambiguous
// regardless of the flag.
func TestCountDugTiles_ExcludesAmbiguousFloorWhenFlagged(t *testing.T) {
	tiles := []blueprints.RegionScanTile{
		{Kind: "floor"}, {Kind: "floor"}, {Kind: "stair_up"},
		{Kind: "construction"}, {Kind: "track"}, {Kind: "ramp"},
	}
	if n := countDugTiles(tiles, false); n != 5 {
		t.Fatalf("countDugTiles(excludeAmbiguousFloor=false) = %d, want 5 (2 floor + stair_up + track + ramp)", n)
	}
	if n := countDugTiles(tiles, true); n != 3 {
		t.Fatalf("countDugTiles(excludeAmbiguousFloor=true) = %d, want 3 (floor excluded, stair_up + track + ramp kept)", n)
	}
}

// TestModeDwarfZ covers the tie-break (lowest Z wins) and the dead-unit
// exclusion (a dead dwarf's frozen last-known position must not skew which
// z-level looks "active" — mirrors the same exclusion look scope=overview
// already applies).
func TestModeDwarfZ(t *testing.T) {
	if _, ok := modeDwarfZ(nil); ok {
		t.Fatal("expected ok=false with no dwarves")
	}

	dwarves := []protocol.EntityInfo{
		{Z: 5}, {Z: 5}, {Z: 3},
	}
	if z, ok := modeDwarfZ(dwarves); !ok || z != 5 {
		t.Fatalf("modeDwarfZ = %d, ok=%v; want z=5 (2 dwarves) ok=true", z, ok)
	}

	// Tie between z=2 and z=4 (one dwarf each): lowest Z wins deterministically.
	tie := []protocol.EntityInfo{{Z: 4}, {Z: 2}}
	if z, ok := modeDwarfZ(tie); !ok || z != 2 {
		t.Fatalf("tie-break modeDwarfZ = %d, ok=%v; want z=2 (lowest wins)", z, ok)
	}

	// A dead dwarf's frozen position must not outvote the live majority.
	withDead := []protocol.EntityInfo{
		{Z: 5}, {Z: 5}, {Z: 9, Dead: true}, {Z: 9, Dead: true}, {Z: 9, Dead: true},
	}
	if z, ok := modeDwarfZ(withDead); !ok || z != 5 {
		t.Fatalf("modeDwarfZ with dead majority at z=9 = %d, ok=%v; want z=5 (dead excluded)", z, ok)
	}
}

// TestLiveDugTileCount_NoDwarvesIsNotOK exercises the nil-bridge / nothing-
// to-scan early exit without needing a live plugin connection — mirrors
// TestFortFootprintBBox_EmptyIsNotOK's use of a nil Bridge to reach the
// same fail-closed path (Bridge.Snapshot/Query are both nil-safe). With no
// dwarves and no zones, candidateShelterZLevels' union is empty and its
// fallback has nothing to fall back to either.
func TestLiveDugTileCount_NoDwarvesIsNotOK(t *testing.T) {
	if _, _, ok := liveDugTileCount(context.Background(), nil); ok {
		t.Fatal("expected ok=false with a nil bridge (nothing zoned or occupied)")
	}
}

// TestCandidateShelterZLevels covers the union-and-dedupe core: zone Z1..Z2
// spans (inclusive, either order) plus alive dwarf Z's, merged and sorted,
// with dead dwarves excluded exactly like modeDwarfZ already does.
func TestCandidateShelterZLevels(t *testing.T) {
	zones := []protocol.ZoneData{{Z1: 5, Z2: 7}, {Z1: 2, Z2: 2}, {Z1: 7, Z2: 5}} // one span reversed
	dwarves := []protocol.EntityInfo{{Z: 7}, {Z: 9, Dead: true}}                // dead dwarf must not contribute z=9

	levels, clamped := candidateShelterZLevels(zones, dwarves, 0, false)
	if clamped {
		t.Fatal("did not expect clamping for a small union")
	}
	want := []int16{2, 5, 6, 7}
	if len(levels) != len(want) {
		t.Fatalf("levels = %v, want %v", levels, want)
	}
	for i := range want {
		if levels[i] != want[i] {
			t.Fatalf("levels = %v, want %v", levels, want)
		}
	}
}

// TestCandidateShelterZLevels_FallsBackWhenNothingZonedOrOccupied is the
// fresh-embark case: nothing zoned or built yet, so the union is empty and
// modeDwarfZ's single-z sample is the only thing left to scan. With no
// fallback available either (fallbackOK=false), there is truly nothing to
// scan.
func TestCandidateShelterZLevels_FallsBackWhenNothingZonedOrOccupied(t *testing.T) {
	levels, clamped := candidateShelterZLevels(nil, nil, 42, true)
	if clamped {
		t.Fatal("the fallback path must never report clamping")
	}
	if len(levels) != 1 || levels[0] != 42 {
		t.Fatalf("levels = %v, want [42] (the fallback z)", levels)
	}

	if levels, _ := candidateShelterZLevels(nil, nil, 0, false); levels != nil {
		t.Fatalf("expected nil levels when no fallback is available either, got %v", levels)
	}
}

// TestCandidateShelterZLevels_ClampsToMax is the regression test for the
// cost bound: a very tall fort's z-union must not translate into an
// unbounded number of fort_footprint+region_scan round trips.
func TestCandidateShelterZLevels_ClampsToMax(t *testing.T) {
	var zones []protocol.ZoneData
	for z := int16(0); z < shelterScanMaxZLevels+20; z++ {
		zones = append(zones, protocol.ZoneData{Z1: z, Z2: z})
	}
	levels, clamped := candidateShelterZLevels(zones, nil, 0, false)
	if !clamped {
		t.Fatal("expected clamped=true past shelterScanMaxZLevels")
	}
	if len(levels) != shelterScanMaxZLevels {
		t.Fatalf("levels has %d entries, want %d (the cap)", len(levels), shelterScanMaxZLevels)
	}
}

// TestLiveDugTileSourceMultiZ covers the evidence-string formatting:
// naming the actual z-levels scanned when there are few, falling back to a
// count+range once there are too many to read at a glance, and disclosing
// the clamp/natural-cave caveats only when they actually apply.
func TestLiveDugTileSourceMultiZ(t *testing.T) {
	named := liveDugTileSourceMultiZ([]int16{5, 10, 15}, false, false)
	if !strings.Contains(named, "3 z-level(s): 5,10,15") {
		t.Fatalf("source = %q, expected the actual z-levels named", named)
	}
	if strings.Contains(named, "natural cave") || strings.Contains(named, "capped") {
		t.Fatalf("source = %q, must not mention caveats that don't apply", named)
	}

	flagged := liveDugTileSourceMultiZ([]int16{5, 10}, false, true)
	if !strings.Contains(flagged, "natural cave") {
		t.Fatalf("source = %q, want a natural-cave disclosure when anyNaturalCave is true", flagged)
	}

	clamped := liveDugTileSourceMultiZ([]int16{1, 2, 3}, true, false)
	if !strings.Contains(clamped, "capped") {
		t.Fatalf("source = %q, want a clamp disclosure when clamped is true", clamped)
	}

	var many []int16
	for z := int16(0); z < 10; z++ {
		many = append(many, z)
	}
	ranged := liveDugTileSourceMultiZ(many, false, false)
	if !strings.Contains(ranged, "10 z-levels (0..9)") {
		t.Fatalf("source = %q, expected a count+range once beyond the per-level naming threshold", ranged)
	}
}

// shelterScanTestBridge builds a Bridge with a real (non-nil) topology
// overlay and modifications overlay — both required by fortFootprintBBox
// (sessionModBBox dereferences mods unconditionally, matching the existing
// nil-guard convention `look scope=fort` already follows in
// tools_percept.go) — with dwarves/zones installed via the same
// worldmodel.Populator.OnEntityUpdate path production code uses
// (populator_test.go's own established pattern for exercising this without
// a live socket), and a canned queryFn standing in for the plugin's
// fort_footprint/region_scan round trips — same seam shape and rationale as
// namePlaceTestBridge's mapSliceFn (places_test.go): no TCP-mocking harness
// exists for dfhack.Client.
func shelterScanTestBridge(t *testing.T, w, h, d uint16, dwarves []protocol.EntityInfo, zones []protocol.ZoneData, query func(ctx context.Context, name, argsJSON string) ([]byte, error)) *Bridge {
	t.Helper()
	topo := topology.NewTopologyOverlay(w, h, d)
	mods := modifications.NewModificationOverlay(modifications.Bounds{Width: w, Height: h, Depth: d})
	logger := logging.NewStderrTextLogger("error")
	wm := worldmodel.New(topo, nil, mods, logger)
	pop := worldmodel.NewPopulator(wm, nil, logger)
	pop.OnEntityUpdate(&protocol.EntityUpdateMessage{Entities: dwarves, Zones: zones})
	return &Bridge{WM: wm, Logger: logger, queryFn: query}
}

// TestLiveDugTileCount_SumsAcrossMultipleZLevels is the regression test for
// the live incident (fortress/memory/goals.md, 2026-07-22): a single-z
// sample of a 9-level fort reported "8 dug tiles" where the street alone
// held ~70. Three z-levels are wired up here — one from a zone span, one
// from the (deliberately small) busiest-dwarf z the OLD single-z sampling
// would have picked alone, and one from another zone span — each with its
// own canned fort_footprint+region_scan pair. The total must be the SUM
// across all three, not just the busiest level's count, and plain "floor"
// tiles must still count even on the level whose fort_footprint response
// flags may_include_natural_cave (the within-bbox undercount's root cause:
// excludeAmbiguousFloor used to be driven by that same flag).
func TestLiveDugTileCount_SumsAcrossMultipleZLevels(t *testing.T) {
	dwarves := []protocol.EntityInfo{ // "busiest z" under the old single-z sampling
		{Z: 20, Type: protocol.EntityTypeDwarf},
		{Z: 20, Type: protocol.EntityTypeDwarf},
	}
	zones := []protocol.ZoneData{{Z1: 10, Z2: 10}, {Z1: 30, Z2: 30}}

	// z=10: 5 plain floor tiles — an ordinary corridor with no stairs or
	// smoothing, exactly the shape the old excludeAmbiguousFloor gate
	// would zero out whenever the bbox also touched bare stone.
	tilesByZ := map[int16][]string{
		10: {"floor", "floor", "floor", "floor", "floor"},
		20: {"floor", "floor", "construction"}, // construction never counts as "dug"
		30: {"floor", "floor", "floor", "stair_down"},
	}
	naturalCaveByZ := map[int16]bool{10: false, 20: false, 30: true}

	query := func(ctx context.Context, name, argsJSON string) ([]byte, error) {
		switch name {
		case "fort_footprint":
			var args struct {
				Z int16 `json:"z"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				t.Fatalf("bad fort_footprint args %q: %v", argsJSON, err)
			}
			if _, known := tilesByZ[args.Z]; !known {
				t.Fatalf("fort_footprint queried for unexpected z=%d", args.Z)
			}
			return []byte(fmt.Sprintf(`{"found":true,"x1":0,"y1":0,"x2":9,"y2":9,"may_include_natural_cave":%v}`,
				naturalCaveByZ[args.Z])), nil
		case "region_scan":
			var args struct {
				Z1 int16 `json:"z1"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				t.Fatalf("bad region_scan args %q: %v", argsJSON, err)
			}
			kinds, known := tilesByZ[args.Z1]
			if !known {
				t.Fatalf("region_scan queried for unexpected z=%d", args.Z1)
			}
			var tiles strings.Builder
			for i, k := range kinds {
				if i > 0 {
					tiles.WriteString(",")
				}
				fmt.Fprintf(&tiles, `{"x":%d,"y":0,"z":%d,"kind":%q}`, i, args.Z1, k)
			}
			return []byte(fmt.Sprintf(`{"x1":0,"y1":0,"z1":%d,"x2":9,"y2":9,"z2":%d,"count":%d,"tiles":[%s]}`,
				args.Z1, args.Z1, len(kinds), tiles.String())), nil
		default:
			t.Fatalf("unexpected query %q", name)
			return nil, nil
		}
	}

	b := shelterScanTestBridge(t, 50, 50, 40, dwarves, zones, query)

	count, source, ok := liveDugTileCount(context.Background(), b)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// z=10: 5 floor -> 5. z=20: 2 floor + 1 construction (skipped) -> 2.
	// z=30: 3 floor + 1 stair_down -> 4. Sum = 11, more than any single
	// z alone (the busiest z=20 alone would only report 2) — proving the
	// count spans all three levels rather than sampling just one.
	if count != 11 {
		t.Fatalf("count = %d, want 11 (5+2+4 summed across z=10,20,30)", count)
	}
	if !strings.Contains(source, "3 z-level(s): 10,20,30") {
		t.Fatalf("source = %q, expected it to name all 3 scanned z-levels", source)
	}
	if !strings.Contains(source, "natural cave") {
		t.Fatalf("source = %q, expected the natural-cave caveat since z=30 flagged it", source)
	}
}
