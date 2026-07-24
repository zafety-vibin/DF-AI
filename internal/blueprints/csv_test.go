package blueprints

import (
	"testing"

	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// Regression test for save_blueprint capturing only 28/49 tiles of a fully
// dug 7x7 soil room. It pins two failure modes at once, end-to-end through
// the real capture pipeline (Detector -> overlay -> blueprint):
//
//  1. Dug soil floors come back as SoilFloor1..4 = tiletype 348..351, which
//     straddled the old fictional tiletype-range boundary at 350 — roughly
//     half the room classified wall->wall and never entered the overlay.
//     Classification is now by shape flags, so every variant must record.
//  2. Tiles that later host a building (a farm plot placed on the dug
//     floor) get re-reported by the plugin when the building lands. The
//     tile's shape stays FLOOR — buildings never change it — so the
//     re-report must neither reclassify nor drop the DUG entry, and the
//     hosting tiles must appear in the saved blueprint.
func TestCreateBlueprintFromModifications_DugRoomHostingBuilding(t *testing.T) {
	overlay := modifications.NewModificationOverlay(modifications.Bounds{Width: 96, Height: 96, Depth: 8})
	det := modifications.NewDetector(overlay)

	const (
		x0, y0, z int16 = 10, 10, 4
		size      int16 = 7
		soilWall        = 261 // df::tiletype SoilWall
	)

	var baseline, dug []protocol.TileState
	for dy := int16(0); dy < size; dy++ {
		for dx := int16(0); dx < size; dx++ {
			baseline = append(baseline, protocol.TileState{
				X: x0 + dx, Y: y0 + dy, Z: z, TileType: soilWall,
				Flags: protocol.FlagHidden | protocol.FlagWall,
			})
			// DF picks the floor variant per tile essentially at random;
			// cycle all four so the room straddles the old 350 boundary.
			dug = append(dug, protocol.TileState{
				X: x0 + dx, Y: y0 + dy, Z: z, TileType: uint16(348 + (dx+dy)%4),
				Flags: protocol.FlagDiscovered | protocol.FlagFloor,
			})
		}
	}
	det.InitializeBaseline(baseline)
	if n := det.DetectModifications(dug); n != int(size*size) {
		t.Fatalf("dig pass recorded %d modifications, want %d", n, size*size)
	}

	// A 3x3 farm plot lands on the middle of the dug floor: the plugin
	// re-reports the hosting tiles (tiletype churn, shape still FLOOR).
	farmMin, farmMax := int16(2), int16(4) // room-relative
	var farm []protocol.TileState
	for dy := farmMin; dy <= farmMax; dy++ {
		for dx := farmMin; dx <= farmMax; dx++ {
			farm = append(farm, protocol.TileState{
				X: x0 + dx, Y: y0 + dy, Z: z, TileType: 350,
				Flags: protocol.FlagDiscovered | protocol.FlagFloor,
			})
		}
	}
	if n := det.DetectModifications(farm); n != 0 {
		t.Fatalf("building re-report recorded %d new modifications, want 0", n)
	}

	region := modifications.Region{
		XMin: x0, XMax: x0 + size - 1,
		YMin: y0, YMax: y0 + size - 1,
		ZMin: z, ZMax: z,
	}
	bp, err := CreateBlueprintFromModifications(overlay, region, "hall7")
	if err != nil {
		t.Fatalf("CreateBlueprintFromModifications: %v", err)
	}
	if len(bp.Digs) != int(size*size) {
		t.Fatalf("captured %d of %d dug tiles — building-hosted tiles must be included", len(bp.Digs), size*size)
	}

	// Every tile must appear exactly once at its region-relative coordinate.
	seen := make(map[[2]int16]bool, len(bp.Digs))
	for _, d := range bp.Digs {
		if d.Z != 0 {
			t.Fatalf("dig (%d,%d) has relative z=%d, want 0", d.X, d.Y, d.Z)
		}
		if d.X < 0 || d.X >= size || d.Y < 0 || d.Y >= size {
			t.Fatalf("dig (%d,%d) outside the %dx%d room", d.X, d.Y, size, size)
		}
		key := [2]int16{d.X, d.Y}
		if seen[key] {
			t.Fatalf("dig (%d,%d) captured twice", d.X, d.Y)
		}
		seen[key] = true
	}
	for dy := farmMin; dy <= farmMax; dy++ {
		for dx := farmMin; dx <= farmMax; dx++ {
			if !seen[[2]int16{dx, dy}] {
				t.Fatalf("farm-plot-hosted tile (%d,%d) missing from blueprint", dx, dy)
			}
		}
	}
}

// TestCreateBlueprintFromRegionScan_SkipsConstructionKeepsRest is the
// region_scan-sourced sibling of TestCreateBlueprintFromModifications above:
// it must reproduce CreateBlueprintFromModifications' "only save DUG tiles,
// skip built walls" filter even though the live region_scan input carries a
// richer kind vocabulary (floor/stairs/ramp/fortification/smooth/track/
// construction) than the overlay's binary Dug/not-Dug classification ever
// did. Every non-construction kind must survive as a dig_type "default"
// entry at region-relative coordinates; "construction" tiles must be
// dropped entirely, matching the original behavior for built walls.
func TestCreateBlueprintFromRegionScan_SkipsConstructionKeepsRest(t *testing.T) {
	region := modifications.Region{XMin: 10, XMax: 12, YMin: 20, YMax: 22, ZMin: 4, ZMax: 4}
	tiles := []RegionScanTile{
		{X: 10, Y: 20, Z: 4, Kind: "floor"},
		{X: 11, Y: 20, Z: 4, Kind: "stair_down"},
		{X: 12, Y: 20, Z: 4, Kind: "ramp"},
		{X: 10, Y: 21, Z: 4, Kind: "fortification"},
		{X: 11, Y: 21, Z: 4, Kind: "smooth"},
		{X: 12, Y: 21, Z: 4, Kind: "track"},
		{X: 10, Y: 22, Z: 4, Kind: "construction"}, // must be dropped
	}

	bp, err := CreateBlueprintFromRegionScan(tiles, region, "capture1")
	if err != nil {
		t.Fatalf("CreateBlueprintFromRegionScan: %v", err)
	}
	if len(bp.Digs) != 6 {
		t.Fatalf("got %d digs, want 6 (construction tile must be skipped): %+v", len(bp.Digs), bp.Digs)
	}
	for _, d := range bp.Digs {
		if d.DigType != "default" {
			t.Errorf("dig (%d,%d,%d) has dig_type %q, want \"default\" (kind inference is a known gap)", d.X, d.Y, d.Z, d.DigType)
		}
	}
	// Region-relative coordinates: origin is (10,20,4).
	want := [2]int16{0, 0} // (10,20,4) -> (0,0,0)
	found := false
	for _, d := range bp.Digs {
		if d.X == want[0] && d.Y == want[1] && d.Z == 0 {
			found = true
		}
		if d.X == 0 && d.Y == 2 { // the construction tile's coords must NOT appear
			t.Errorf("construction tile at region-relative (0,2) leaked into the blueprint")
		}
	}
	if !found {
		t.Fatalf("expected a dig at region-relative (0,0,0), got %+v", bp.Digs)
	}
}

// TestCreateBlueprintFromRegionScan_AllConstructionErrors guards the empty
// case: a region that's entirely built (no dug tiles at all) must error
// instead of silently writing a zero-tile blueprint, mirroring
// CreateBlueprintFromModifications' "no modifications in region" error for
// its own empty case.
func TestCreateBlueprintFromRegionScan_AllConstructionErrors(t *testing.T) {
	region := modifications.Region{XMin: 0, XMax: 1, YMin: 0, YMax: 0, ZMin: 0, ZMax: 0}
	tiles := []RegionScanTile{
		{X: 0, Y: 0, Z: 0, Kind: "construction"},
		{X: 1, Y: 0, Z: 0, Kind: "construction"},
	}
	if _, err := CreateBlueprintFromRegionScan(tiles, region, "empty"); err == nil {
		t.Fatal("expected an error when every scanned tile is a construction")
	}
}
