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
