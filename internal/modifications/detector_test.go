package modifications

import (
	"testing"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// Real df::tiletype values (dfhack 53.15 tiletype.h) used throughout these
// tests so the scenarios pin the live bugs they reproduce:
//
//	MurkyPool        = 2   (shape FLOOR — a water feature, not a wall)
//	StoneWall        = 215
//	SoilWall         = 261
//	StoneFloor1      = 332
//	GrassDarkFloor1  = 344
//	SoilFloor1..4    = 348..351 (straddle the old fictional 350 boundary)
//	GrassLightFloor1 = 394
const (
	ttMurkyPool       = 2
	ttStoneWall       = 215
	ttSoilWall        = 261
	ttStoneFloor1     = 332
	ttGrassDarkFloor  = 344
	ttGrassLightFloor = 394
	ttSoilFloor1      = 348
	// ttStairUpDown is a representative carved up/down staircase tiletype.
	// The classifier only reads the shape flag byte (FlagFloor, per
	// tile_extractor.cpp's STAIR_UPDOWN case), never the numeric tiletype,
	// so the exact value is inert here — it only needs to round-trip
	// through the OldTileType/NewTileType audit trail.
	ttStairUpDown = 396
)

func TestInferModificationTypeFromFlags(t *testing.T) {
	const (
		wall  = protocol.FlagWall
		floor = protocol.FlagFloor
		void  = protocol.FlagVoid
		disc  = protocol.FlagDiscovered
		hid   = protocol.FlagHidden
	)
	cases := []struct {
		name     string
		old, new uint8
		want     ModificationType
	}{
		{"dig: hidden wall to floor", hid | wall, disc | floor, ModificationDug},
		{"channel: floor to void", disc | floor, disc | void, ModificationChanneled},
		{"channel through wall", hid | wall, disc | void, ModificationChanneled},
		{"build wall on floor", disc | floor, disc | wall, ModificationBuiltWall},
		{"build wall over open space", disc | void, disc | wall, ModificationBuiltWall},
		{"build floor over open space", disc | void, disc | floor, ModificationBuiltFloor},
		{"ambient: floor stays floor (grass/pool churn)", disc | floor, disc | floor, ModificationUnknown},
		{"reveal only: hidden->discovered wall is not a modification", hid | wall, disc | wall, ModificationUnknown},
		{"shapeless baseline", hid, disc | floor, ModificationUnknown},
		{"shapeless new state", disc | floor, hid, ModificationUnknown},
	}
	for _, c := range cases {
		if got := InferModificationTypeFromFlags(c.old, c.new); got != c.want {
			t.Errorf("%s: InferModificationTypeFromFlags(%#02x, %#02x) = %v, want %v",
				c.name, c.old, c.new, got, c.want)
		}
	}
}

func newTestDetector() (*Detector, *ModificationOverlay) {
	overlay := NewModificationOverlay(Bounds{Width: 96, Height: 96, Depth: 8})
	return NewDetector(overlay), overlay
}

func ts(x, y, z int16, tt uint16, flags uint8) protocol.TileState {
	return protocol.TileState{X: x, Y: y, Z: z, TileType: tt, Flags: flags}
}

// TestDetectModifications_AmbientChurnNotRecorded pins the scope=fort bbox
// pollution fix: grass dark<->light cycling and murky pools drying to soil
// floors change the tiletype but keep the FLOOR shape — they must never be
// recorded as modifications (the old tiletype-range heuristic recorded both
// as DUG/BUILT_WALL and dragged the fort footprint out to the valley).
func TestDetectModifications_AmbientChurnNotRecorded(t *testing.T) {
	det, overlay := newTestDetector()
	det.InitializeBaseline([]protocol.TileState{
		ts(80, 80, 3, ttGrassDarkFloor, protocol.FlagDiscovered|protocol.FlagFloor),
		ts(81, 80, 3, ttMurkyPool, protocol.FlagDiscovered|protocol.FlagFloor),
	})
	n := det.DetectModifications([]protocol.TileState{
		ts(80, 80, 3, ttGrassLightFloor, protocol.FlagDiscovered|protocol.FlagFloor), // grass churn
		ts(81, 80, 3, 350, protocol.FlagDiscovered|protocol.FlagFloor),               // pool dries to SoilFloor3
	})
	if n != 0 {
		t.Fatalf("ambient churn recorded %d modifications, want 0", n)
	}
	if got := overlay.GetCount(); got != 0 {
		t.Fatalf("overlay holds %d entries after ambient churn, want 0", got)
	}
}

// TestDetectModifications_StoneDigRecorded pins the inverse half of the same
// bug: StoneWall(215) -> StoneFloor1(332) was wall->wall under the fictional
// ranges (both <350) and never recorded, leaving fort digs at stone levels
// invisible. By shape it is wall->floor = DUG.
func TestDetectModifications_StoneDigRecorded(t *testing.T) {
	det, overlay := newTestDetector()
	det.InitializeBaseline([]protocol.TileState{
		ts(10, 10, 3, ttStoneWall, protocol.FlagHidden|protocol.FlagWall),
	})
	n := det.DetectModifications([]protocol.TileState{
		ts(10, 10, 3, ttStoneFloor1, protocol.FlagDiscovered|protocol.FlagFloor),
	})
	if n != 1 {
		t.Fatalf("stone dig recorded %d modifications, want 1", n)
	}
	info, ok := overlay.Get(Coordinate{X: 10, Y: 10, Z: 3})
	if !ok {
		t.Fatal("dug tile missing from overlay")
	}
	if info.Type != ModificationDug {
		t.Fatalf("type = %v, want DUG", info.Type)
	}
	if info.OldTileType != ttStoneWall || info.NewTileType != ttStoneFloor1 {
		t.Fatalf("audit trail = %d -> %d, want %d -> %d",
			info.OldTileType, info.NewTileType, ttStoneWall, ttStoneFloor1)
	}
}

// TestDetectModifications_SoilDigAllFloorVariantsRecorded: SoilFloor1..4 =
// 348..351 straddle the old fictional 350 boundary, which is why
// save_blueprint captured ~half of a fully dug soil room. All four variants
// must record as DUG.
func TestDetectModifications_SoilDigAllFloorVariantsRecorded(t *testing.T) {
	det, overlay := newTestDetector()
	var baseline, dug []protocol.TileState
	for i, tt := range []uint16{348, 349, 350, 351} {
		x := int16(20 + i)
		baseline = append(baseline, ts(x, 20, 4, ttSoilWall, protocol.FlagHidden|protocol.FlagWall))
		dug = append(dug, ts(x, 20, 4, tt, protocol.FlagDiscovered|protocol.FlagFloor))
	}
	det.InitializeBaseline(baseline)
	if n := det.DetectModifications(dug); n != 4 {
		t.Fatalf("recorded %d of 4 soil floor variants, want all 4", n)
	}
	for i := range dug {
		info, ok := overlay.Get(Coordinate{X: int16(20 + i), Y: 20, Z: 4})
		if !ok || info.Type != ModificationDug {
			t.Errorf("variant %d (tiletype %d): ok=%v type=%v, want DUG", i, dug[i].TileType, ok, info.Type)
		}
	}
}

// TestDetectModifications_ShapelessBaselineRebaselines: tiles in unallocated
// blocks arrive as tiletype 0 with FLAG_HIDDEN only (no shape bits). The
// first shape-bearing observation must become the baseline — not a fake
// modification — and a later dig against that baseline must record.
func TestDetectModifications_ShapelessBaselineRebaselines(t *testing.T) {
	det, overlay := newTestDetector()
	det.InitializeBaseline([]protocol.TileState{
		ts(5, 5, 2, 0, protocol.FlagHidden),
	})
	if n := det.DetectModifications([]protocol.TileState{
		ts(5, 5, 2, ttSoilWall, protocol.FlagHidden|protocol.FlagWall), // block allocates as solid wall
	}); n != 0 {
		t.Fatalf("block allocation recorded %d modifications, want 0", n)
	}
	if n := det.DetectModifications([]protocol.TileState{
		ts(5, 5, 2, 349, protocol.FlagDiscovered|protocol.FlagFloor), // then dug
	}); n != 1 {
		t.Fatalf("dig after rebaseline recorded %d modifications, want 1", n)
	}
	info, ok := overlay.Get(Coordinate{X: 5, Y: 5, Z: 2})
	if !ok || info.Type != ModificationDug || info.OldTileType != ttSoilWall {
		t.Fatalf("ok=%v type=%v old=%d, want DUG with old=%d", ok, info.Type, info.OldTileType, ttSoilWall)
	}
}

// TestDetectModifications_ShapelessBaselineJumpsStraightToDug pins the live
// regression: a fresh z-level a fort has never touched before can sit as a
// shapeless HIDDEN placeholder (tiletype 0) all the way through a plugin
// reconnect's FULL_STATE, because MapCache can observe a block but never
// forces DF to generate one that doesn't exist yet -- only a dig job
// actually touching the block makes DF materialize it. When that happens,
// the very first delta the plugin ever sends for that tile can already BE
// the finished dig (or a carved stair), with no intervening wall-shaped
// observation ever crossing the wire. Before the fix this was silently
// swallowed as "just a rebaseline" and nothing was recorded -- reproducing
// the live symptom where a dug 5x5 hall plus a carved up/down stair shaft at
// a freshly-reached z-level recorded zero modifications.
func TestDetectModifications_ShapelessBaselineJumpsStraightToDug(t *testing.T) {
	// z=118 (matching the live incident) exceeds newTestDetector's Depth:8
	// fixture bounds, so build one deep enough here.
	overlay := NewModificationOverlay(Bounds{Width: 96, Height: 96, Depth: 200})
	det := NewDetector(overlay)
	det.InitializeBaseline([]protocol.TileState{
		ts(44, 42, 118, 0, protocol.FlagHidden), // floor tile: block never materialized
		ts(45, 43, 118, 0, protocol.FlagHidden), // stair tile: block never materialized
	})

	// Reconnect's FULL_STATE never saw a wall here -- the dig job that
	// carved it also materialized the block, so this is the first delta
	// ever observed for these coordinates.
	n := det.DetectModifications([]protocol.TileState{
		ts(44, 42, 118, ttSoilFloor1, protocol.FlagDiscovered|protocol.FlagFloor),  // dug floor
		ts(45, 43, 118, ttStairUpDown, protocol.FlagDiscovered|protocol.FlagFloor), // carved up/down stair
	})
	if n != 2 {
		t.Fatalf("shapeless-to-dug jump recorded %d modifications, want 2", n)
	}

	floor, ok := overlay.Get(Coordinate{X: 44, Y: 42, Z: 118})
	if !ok || floor.Type != ModificationDug {
		t.Fatalf("floor: ok=%v type=%v, want DUG", ok, floor.Type)
	}
	stair, ok := overlay.Get(Coordinate{X: 45, Y: 43, Z: 118})
	if !ok || stair.Type != ModificationDug {
		t.Fatalf("stair: ok=%v type=%v, want DUG", ok, stair.Type)
	}
}

// TestDetectModifications_ShapelessBaselineAmbientRevealNotRecorded pins the
// inverse guard: a shapeless placeholder that materializes as ambient,
// unworked terrain -- ordinary hidden solid rock finally coming online,
// still just a wall -- must not be misread as a modification (the implied
// wall baseline used for the shapeless->floor jump above must not fabricate
// a transition when the new observation is ALSO wall-shaped).
func TestDetectModifications_ShapelessBaselineAmbientRevealNotRecorded(t *testing.T) {
	overlay := NewModificationOverlay(Bounds{Width: 96, Height: 96, Depth: 200})
	det := NewDetector(overlay)
	det.InitializeBaseline([]protocol.TileState{
		ts(90, 90, 60, 0, protocol.FlagHidden), // far-off, never-materialized rock
	})
	n := det.DetectModifications([]protocol.TileState{
		ts(90, 90, 60, ttStoneWall, protocol.FlagHidden|protocol.FlagWall), // still just rock
	})
	if n != 0 {
		t.Fatalf("ambient block materialization recorded %d modifications, want 0", n)
	}
	if got := overlay.GetCount(); got != 0 {
		t.Fatalf("overlay holds %d entries after ambient materialization, want 0", got)
	}
}
