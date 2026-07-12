package mapview

import (
	"testing"

	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

// buildTestOverlay constructs a real 40x40x3 overlay via BuildFromTiles,
// speaking the overlay's actual input language (protocol.TileState flags,
// classified by topology.ClassifyState):
//   - z=0: solid walls everywhere (FlagWall -> StateClosed)
//   - z=1: walls except an open 4x4 pocket at (10..13,10..13)
//     (FlagFloor -> StateOpen)
//   - z=2: open surface everywhere (FlagFloor -> StateOpen)
//
// BuildFromTiles requires exactly width*height*depth tiles, so every tile
// on every level is emitted.
func buildTestOverlay(t *testing.T) *topology.TopologyOverlay {
	t.Helper()
	topo := topology.NewTopologyOverlay(40, 40, 3)
	if topo == nil {
		t.Fatal("overlay nil")
	}
	var tiles []protocol.TileState
	for z := int16(0); z < 3; z++ {
		for y := int16(0); y < 40; y++ {
			for x := int16(0); x < 40; x++ {
				flags := protocol.FlagDiscovered | protocol.FlagWall
				if z == 2 {
					flags = protocol.FlagDiscovered | protocol.FlagFloor
				}
				if z == 1 && x >= 10 && x <= 13 && y >= 10 && y <= 13 {
					flags = protocol.FlagDiscovered | protocol.FlagFloor
				}
				tiles = append(tiles, protocol.TileState{X: x, Y: y, Z: z, TileType: 600, Flags: flags})
			}
		}
	}
	if err := topo.BuildFromTiles(tiles); err != nil {
		t.Fatalf("build: %v", err)
	}
	return topo
}

func TestFindDigSitesPrefersSolidNearAnchor(t *testing.T) {
	topo := buildTestOverlay(t)
	sites := FindDigSites(topo, DigSiteRequest{W: 5, H: 5, Z: 1, NearX: 12, NearY: 12, MaxCandidates: 3})
	if len(sites) == 0 {
		t.Fatal("no candidates found in an almost-all-solid level")
	}
	best := sites[0]
	if best.Open != 0 {
		t.Fatalf("best site overlaps open pocket: %+v", best)
	}
	if best.Dist > 20 {
		t.Fatalf("best site ignores anchor: %+v", best)
	}
}

func TestFindDigSitesSkipsRectsOverlappingOpenPocket(t *testing.T) {
	topo := buildTestOverlay(t)
	sites := FindDigSites(topo, DigSiteRequest{W: 5, H: 5, Z: 1, NearX: 12, NearY: 12})
	if len(sites) == 0 {
		t.Fatal("no candidates found")
	}
	for _, s := range sites {
		if s.Open != 0 {
			t.Fatalf("candidate overlaps open tiles: %+v", s)
		}
		// No candidate rectangle may intersect the open 4x4 pocket.
		if s.X1 <= 13 && s.X2 >= 10 && s.Y1 <= 13 && s.Y2 >= 10 {
			t.Fatalf("candidate rect intersects open pocket: %+v", s)
		}
		if s.Solid != 25 {
			t.Fatalf("5x5 candidate should have 25 solid tiles, got %+v", s)
		}
	}
}

func TestFindDigSitesRankedByDistanceAndCapped(t *testing.T) {
	topo := buildTestOverlay(t)
	// Default MaxCandidates is 5.
	sites := FindDigSites(topo, DigSiteRequest{W: 3, H: 3, Z: 1, NearX: 20, NearY: 20})
	if len(sites) != 5 {
		t.Fatalf("expected default cap of 5 candidates, got %d", len(sites))
	}
	for i := 1; i < len(sites); i++ {
		if sites[i].Dist < sites[i-1].Dist {
			t.Fatalf("candidates not sorted by distance: %v then %v", sites[i-1], sites[i])
		}
	}
}

func TestFindDigSitesNoCandidatesOnOpenLevel(t *testing.T) {
	topo := buildTestOverlay(t)
	// z=2 is fully open surface: nothing to dig.
	sites := FindDigSites(topo, DigSiteRequest{W: 3, H: 3, Z: 2, NearX: 20, NearY: 20})
	if len(sites) != 0 {
		t.Fatalf("expected no candidates on a fully-open level, got %d", len(sites))
	}
}
