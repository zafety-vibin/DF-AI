package mapview

import (
	"strings"
	"testing"
)

func intPtr(v int) *int { return &v }

// A single isolated tile is its own cluster and renders as a bare
// coordinate, not a degenerate 1x1 rectangle.
func TestClusterItemTiles_SingleTile(t *testing.T) {
	got := ClusterItemTiles([][3]int16{{10, 20, 3}})
	if len(got) != 1 {
		t.Fatalf("expected 1 cluster, got %d: %+v", len(got), got)
	}
	want := ItemCluster{X1: 10, Y1: 20, X2: 10, Y2: 20, Tiles: 1, Items: 3}
	if got[0] != want {
		t.Fatalf("cluster mismatch: got %+v want %+v", got[0], want)
	}
	if s := got[0].String(); s != "(10,20) 3 items" {
		t.Fatalf("single-tile cluster must render as a bare coord, got %q", s)
	}
}

// An L-shaped blob is ONE cluster: its bbox covers the whole L (including
// the empty corner), tiles count only the occupied cells.
func TestClusterItemTiles_LShapedBlobIsOneCluster(t *testing.T) {
	tiles := [][3]int16{
		{5, 5, 1}, {6, 5, 1}, {7, 5, 1},
		{5, 6, 1},
		{5, 7, 2},
	}
	got := ClusterItemTiles(tiles)
	if len(got) != 1 {
		t.Fatalf("expected 1 cluster for an L-shaped blob, got %d: %+v", len(got), got)
	}
	want := ItemCluster{X1: 5, Y1: 5, X2: 7, Y2: 7, Tiles: 5, Items: 6}
	if got[0] != want {
		t.Fatalf("cluster mismatch: got %+v want %+v", got[0], want)
	}
	if s := got[0].String(); s != "(5,5)-(7,7) 6 items" {
		t.Fatalf("multi-tile cluster render wrong: %q", s)
	}
}

// Diagonal adjacency joins tiles (8-connected): a dwarf sees one pile
// there, and splitting it would report two half-piles no single follow-up
// rectangle covers.
func TestClusterItemTiles_DiagonalsConnect(t *testing.T) {
	got := ClusterItemTiles([][3]int16{{1, 1, 1}, {2, 2, 1}})
	if len(got) != 1 {
		t.Fatalf("diagonal neighbours must form one cluster, got %d: %+v", len(got), got)
	}
}

// Two separated blobs stay separate and sort by item count descending,
// regardless of the order they arrive in.
func TestClusterItemTiles_TwoBlobsSortedByItems(t *testing.T) {
	tiles := [][3]int16{
		{1, 1, 2}, {2, 1, 3}, // small blob: 5 items
		{40, 40, 50}, {41, 40, 46}, // big blob: 96 items, far away
	}
	got := ClusterItemTiles(tiles)
	if len(got) != 2 {
		t.Fatalf("expected 2 clusters, got %d: %+v", len(got), got)
	}
	if got[0].Items != 96 || got[1].Items != 5 {
		t.Fatalf("clusters must sort by item count desc, got %+v", got)
	}
	if got[0].X1 != 40 || got[0].X2 != 41 {
		t.Fatalf("big cluster bbox wrong: %+v", got[0])
	}
}

// Repeated coordinates sum their counts instead of inflating the tile
// tally (a stitched multi-block slice unions its blocks' arrays).
func TestClusterItemTiles_DuplicateCoordsSum(t *testing.T) {
	got := ClusterItemTiles([][3]int16{{3, 3, 2}, {3, 3, 5}})
	if len(got) != 1 || got[0].Tiles != 1 || got[0].Items != 7 {
		t.Fatalf("duplicate coords must sum into one tile, got %+v", got)
	}
}

func TestClusterItemTiles_Empty(t *testing.T) {
	if got := ClusterItemTiles(nil); got != nil {
		t.Fatalf("empty input must produce no clusters, got %+v", got)
	}
}

// A tidy view — no items, no stockpile tiles — prints NOTHING. The footer
// runs on every look, so silence when there is nothing to say is the
// feature, not an omission.
func TestFloorItemFooter_SilentWhenClean(t *testing.T) {
	s := &Slice{StockpileTilesInView: intPtr(0), StockpileTilesOccupied: intPtr(0)}
	if got := FloorItemFooter(s); len(got) != 0 {
		t.Fatalf("clean view must print nothing, got %q", got)
	}
	if got := FloorItemSummaryLine(s); got != "" {
		t.Fatalf("clean downsampled view must print nothing, got %q", got)
	}
}

// Every item in the view sits inside a stockpile: say so plainly and skip
// the cluster machinery entirely — there is no homeless pile to point at.
// This is also the case a fort pays most often, so it must collapse to ONE
// merged line rather than two lines saying overlapping things.
func TestFloorItemFooter_AllInsideStockpiles(t *testing.T) {
	s := &Slice{
		FloorItems:             [][3]int16{{10, 10, 4}, {11, 10, 6}},
		StockpileTilesInView:   intPtr(20),
		StockpileTilesOccupied: intPtr(2),
	}
	got := FloorItemFooter(s)
	if len(got) != 1 {
		t.Fatalf("the all-stored case must merge into 1 line, got %d: %q", len(got), got)
	}
	if got[0] != "loose items in view: 10 on 2 tiles, all inside stockpiles (20 stockpile tiles, 2 taken)" {
		t.Fatalf("merged all-stored line wrong: %q", got[0])
	}
	if strings.Contains(strings.Join(got, "\n"), "clusters") {
		t.Fatalf("no homeless items means no cluster phrase: %q", got)
	}
}

// The headline case: homeless clutter gets its own count AND coordinates,
// so the follow-up `stockpile` call needs no extra perception round trip.
func TestFloorItemFooter_HomelessClusters(t *testing.T) {
	s := &Slice{
		FloorItems: [][3]int16{
			{88, 88, 50}, {89, 88, 46}, // big homeless pile
			{101, 95, 41}, // second homeless pile
			{60, 60, 9},   // lone homeless tile, far away
			{20, 20, 7},   // stockpiled
		},
		FloorItemsNoStock: [][3]int16{
			{88, 88, 50}, {89, 88, 46},
			{101, 95, 41},
			{60, 60, 9},
		},
		StockpileTilesInView:   intPtr(120),
		StockpileTilesOccupied: intPtr(101),
	}
	got := FloorItemFooter(s)
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(got), got)
	}
	want := "loose items OUTSIDE stockpiles: 146 items on 4 tiles (view total 153 on 5 tiles) — " +
		"clusters: (88,88)-(89,88) 96 items, (101,95) 41 items, +1 tiles elsewhere (lens=items for classes)"
	if got[0] != want {
		t.Fatalf("homeless line wrong:\n got %q\nwant %q", got[0], want)
	}
	// Tile-scoped wording, never a capacity claim: a pile whose tiles each
	// hold one half-empty bin must not read as "84% full".
	if got[1] != "stockpile tiles in view: 120, 101 with items on them (84% of tiles taken)" {
		t.Fatalf("stockpile fill line wrong: %q", got[1])
	}
}

// The plugin caps its homeless-tile array at 200 PER MAP BLOCK; when it
// says so, every derived number is a lower bound and the render must not
// present them as exact — nor state the cap as a flat 200, since a stitched
// scope=elevation/fort view unions many blocks and its real bound is
// 200 x blocks.
func TestFloorItemFooter_CapDisclosed(t *testing.T) {
	s := &Slice{
		FloorItems:              [][3]int16{{5, 5, 1}},
		FloorItemsNoStock:       [][3]int16{{5, 5, 1}},
		FloorItemsNoStockCapped: true,
		StockpileTilesInView:    intPtr(0),
	}
	got := FloorItemFooter(s)
	if len(got) != 1 || !strings.Contains(got[0], "capped at 200 per map block") {
		t.Fatalf("cap flag must surface as a per-block bound, got %q", got)
	}
}

// A view with stockpile tiles but zero items is a real, reportable
// all-clear ("there is room here") — distinct from the old-plugin case
// below, which knows nothing about stockpiles at all.
func TestFloorItemFooter_EmptyStockpileStillReports(t *testing.T) {
	s := &Slice{StockpileTilesInView: intPtr(52), StockpileTilesOccupied: intPtr(0)}
	got := FloorItemFooter(s)
	if len(got) != 1 || got[0] != "stockpile tiles in view: 52, 0 with items on them (0% of tiles taken)" {
		t.Fatalf("empty stockpile must still report its capacity, got %q", got)
	}
}

// VERSION SKEW (the contract that matters): an older plugin omits every
// stockpile field, so StockpileTilesInView decodes to nil. The footer must
// then report ONLY what that plugin actually sent — totals and cluster
// positions from floor_items — and say outright that the split is
// unavailable. It must never render the absent split as "0 outside
// stockpiles", which reads as an all-clear the plugin never asserted.
func TestFloorItemFooter_OldPluginNoStockpileFields(t *testing.T) {
	s := &Slice{FloorItems: [][3]int16{{88, 88, 50}, {89, 88, 46}, {60, 60, 9}}}
	got := FloorItemFooter(s)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 line from an old plugin, got %d: %q", len(got), got)
	}
	want := "loose items in view: 105 on 3 tiles (stockpile coverage not reported by this plugin build) — " +
		"clusters: (88,88)-(89,88) 96 items, (60,60) 9 items"
	if got[0] != want {
		t.Fatalf("old-plugin line wrong:\n got %q\nwant %q", got[0], want)
	}
	if strings.Contains(got[0], "OUTSIDE") || strings.Contains(got[0], "all inside") {
		t.Fatalf("old plugin must not imply a stockpile split: %q", got[0])
	}
}

// A nil-pointer sentinel is not the same as a zero value: a plugin that
// measured zero stockpile tiles reports an honest split, an older one
// reports no split at all. Both slices carry identical item data.
func TestFloorItemFooter_ZeroCoverageIsNotAbsentCoverage(t *testing.T) {
	items := [][3]int16{{5, 5, 3}}
	measured := FloorItemFooter(&Slice{
		FloorItems: items, FloorItemsNoStock: items,
		StockpileTilesInView: intPtr(0), StockpileTilesOccupied: intPtr(0),
	})
	absent := FloorItemFooter(&Slice{FloorItems: items})
	if len(measured) != 1 || !strings.Contains(measured[0], "OUTSIDE stockpiles") {
		t.Fatalf("measured-zero-coverage view must assert the split, got %q", measured)
	}
	if len(absent) != 1 || !strings.Contains(absent[0], "not reported by this plugin build") {
		t.Fatalf("absent-coverage view must disclaim the split, got %q", absent)
	}
}

func TestFloorItemSummaryLine(t *testing.T) {
	cases := []struct {
		name string
		s    *Slice
		want string
	}{
		{
			name: "old plugin",
			s:    &Slice{FloorItems: [][3]int16{{1, 1, 4}, {2, 2, 6}}},
			want: "loose items in source view: 10 on 2 tiles (stockpile coverage not reported by this plugin build)",
		},
		{
			name: "all stockpiled",
			s: &Slice{FloorItems: [][3]int16{{1, 1, 4}},
				StockpileTilesInView: intPtr(9), StockpileTilesOccupied: intPtr(1)},
			want: "loose items in source view: 4 on 1 tiles, all inside stockpiles",
		},
		{
			name: "homeless, no cluster bboxes at block granularity",
			s: &Slice{FloorItems: [][3]int16{{1, 1, 4}, {9, 9, 6}},
				FloorItemsNoStock:    [][3]int16{{9, 9, 6}},
				StockpileTilesInView: intPtr(9), StockpileTilesOccupied: intPtr(1)},
			want: "loose items in source view: 10 on 2 tiles — 6 items on 1 tiles OUTSIDE stockpiles",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FloorItemSummaryLine(tc.s); got != tc.want {
				t.Fatalf("\n got %q\nwant %q", got, tc.want)
			}
			if strings.Contains(FloorItemSummaryLine(tc.s), "clusters:") {
				t.Fatal("downsampled summary must not carry tile-exact cluster bboxes")
			}
		})
	}
}
