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

	region, nearestTile, dist, ok := rg.NearestRegion(Coord{5, 0, 0})
	if !ok {
		t.Fatal("expected a region to be found")
	}
	if dist != 4 { // closest tile is (1,0,0), Chebyshev distance 4
		t.Fatalf("expected distance 4, got %d", dist)
	}
	if region.ID != 0 {
		t.Fatalf("expected region 0, got %d", region.ID)
	}
	wantTile := Coord{X: 1, Y: 0, Z: 0}
	if nearestTile != wantTile {
		t.Fatalf("expected nearest tile %+v, got %+v", wantTile, nearestTile)
	}
}

func TestRegionGraph_NearestRegion_EmptyGraph(t *testing.T) {
	topo := fixtureTopo(10, 10, 1, nil)
	rg := BuildRegionGraph(topo)
	_, _, _, ok := rg.NearestRegion(Coord{0, 0, 0})
	if ok {
		t.Fatal("empty graph must report ok=false, not fabricate a region")
	}
}

func TestRegionGraph_RegionAt(t *testing.T) {
	// A single 2x2 room; RegionAt on any tile inside it must resolve to
	// the same region and return all four tiles.
	open := []Coord{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}}
	topo := fixtureTopo(10, 10, 1, open)
	rg := BuildRegionGraph(topo)

	region, ok := rg.RegionAt(Coord{1, 1, 0})
	if !ok {
		t.Fatal("expected RegionAt to find a region for an open tile")
	}
	if region.ID != 0 {
		t.Fatalf("expected region 0, got %d", region.ID)
	}
	if len(region.Tiles) != 4 {
		t.Fatalf("expected 4 tiles in region, got %d", len(region.Tiles))
	}

	if _, ok := rg.RegionAt(Coord{5, 5, 0}); ok {
		t.Fatal("RegionAt on a closed/unknown tile must report ok=false")
	}
}

func TestRegionGraph_BBox(t *testing.T) {
	// An L-shaped region so the min/max corners aren't trivially the seed
	// tile flood-fill happened to start from — catches off-by-one or
	// axis-swap regressions in minCoord/maxCoord.
	open := []Coord{
		{2, 5, 0}, {3, 5, 0}, {4, 5, 0}, // horizontal arm
		{2, 6, 0}, {2, 7, 0}, // vertical arm down from the left end
	}
	topo := fixtureTopo(10, 10, 1, open)
	rg := BuildRegionGraph(topo)

	region, ok := rg.RegionAt(Coord{4, 5, 0})
	if !ok {
		t.Fatal("expected a region")
	}
	wantMin, wantMax := Coord{2, 5, 0}, Coord{4, 7, 0}
	if region.BBox[0] != wantMin {
		t.Fatalf("BBox min = %+v, want %+v", region.BBox[0], wantMin)
	}
	if region.BBox[1] != wantMax {
		t.Fatalf("BBox max = %+v, want %+v", region.BBox[1], wantMax)
	}
}

// TestRegionGraph_VerticalAdjacencyIsPermissive pins the documented
// deviation from the design of record (design-perception-round2.md,
// Component 1): vertical connectivity is "any open tile directly
// above/below", not "matching stair tiles", because the wire protocol
// carries no tile-shape sub-type to distinguish them (see neighbors()'s
// doc comment). This is a known false-positive risk, not a silent
// regression — an ordinary two-story room with a floor tile directly
// above another floor tile (no stairs anywhere) reports as ONE region,
// even though no dwarf can walk between the two floors. If a future
// change adds real stair-only vertical adjacency, this test's expected
// region count should flip from 1 to 2 and this comment should move to
// document the tightened behavior instead.
func TestRegionGraph_VerticalAdjacencyIsPermissive(t *testing.T) {
	// Two ordinary floor tiles stacked at the same X/Y on consecutive Z
	// levels — a plain floor/ceiling boundary, not a stair or ramp.
	open := []Coord{{3, 3, 0}, {3, 3, 1}}
	topo := fixtureTopo(10, 10, 2, open)
	rg := BuildRegionGraph(topo)

	if len(rg.Regions) != 1 {
		t.Fatalf("expected the permissive rule to merge the stacked floor tiles into 1 region, got %d", len(rg.Regions))
	}
	if !rg.SameRegion(Coord{3, 3, 0}, Coord{3, 3, 1}) {
		t.Fatal("expected SameRegion true for stacked plain-floor tiles under the documented permissive rule")
	}
}
