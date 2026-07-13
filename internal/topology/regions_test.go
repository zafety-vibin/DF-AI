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
