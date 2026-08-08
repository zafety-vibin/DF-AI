package mapview

import (
	"strings"
	"testing"
)

// quadrantSlice builds a w x h Slice at absolute origin (x1,y1) filled with
// one distinctive glyph, plus one Designated tile, one Water tile, one
// Smoothed tile, and one FloorItems tile at absolute coordinates inside the
// block — used to check that overlay entries survive stitching unchanged
// (they're already absolute).
func quadrantSlice(x1, y1 int16, w, h int, glyph rune) *Slice {
	row := strings.Repeat(string(glyph), w)
	rows := make([]string, h)
	for i := range rows {
		rows[i] = row
	}
	return &Slice{
		Z: 5, X1: x1, Y1: y1, Rows: rows,
		Designated: [][2]int16{{x1, y1}},
		Water:      [][3]int16{{x1 + 1, y1, 3}},
		Smoothed:   [][2]int16{{x1, y1}},
		FloorItems: [][3]int16{{x1, y1, 2}},
	}
}

func TestStitchSlices_2x2Quadrants(t *testing.T) {
	// 2x2 grid of 4x4 blocks -> 8x8 stitched slice. Distinct glyph per
	// quadrant so a stitching bug (dup row/col, wrong seam) is visible.
	grid := [][]*Slice{
		{quadrantSlice(0, 0, 4, 4, 'A'), quadrantSlice(4, 0, 4, 4, 'B')},
		{quadrantSlice(0, 4, 4, 4, 'C'), quadrantSlice(4, 4, 4, 4, 'D')},
	}
	out, err := StitchSlices(grid)
	if err != nil {
		t.Fatalf("stitch: %v", err)
	}
	if out.Z != 5 || out.X1 != 0 || out.Y1 != 0 {
		t.Fatalf("origin/z wrong: Z=%d X1=%d Y1=%d", out.Z, out.X1, out.Y1)
	}
	if len(out.Rows) != 8 {
		t.Fatalf("expected 8 rows, got %d", len(out.Rows))
	}
	for i, row := range out.Rows {
		if len(row) != 8 {
			t.Fatalf("row %d width = %d, want 8: %q", i, len(row), row)
		}
	}
	// Quadrant seams: no duplication, no gap.
	wantTop := "AAAABBBB"
	wantBottom := "CCCCDDDD"
	for i := 0; i < 4; i++ {
		if out.Rows[i] != wantTop {
			t.Errorf("row %d = %q, want %q", i, out.Rows[i], wantTop)
		}
	}
	for i := 4; i < 8; i++ {
		if out.Rows[i] != wantBottom {
			t.Errorf("row %d = %q, want %q", i, out.Rows[i], wantBottom)
		}
	}
	// Overlay entries from all 4 blocks survive, at their original absolute
	// coordinates (no re-basing).
	if len(out.Designated) != 4 {
		t.Fatalf("expected 4 designated tiles (one per block), got %d: %v", len(out.Designated), out.Designated)
	}
	wantDesignated := map[[2]int16]bool{{0, 0}: true, {4, 0}: true, {0, 4}: true, {4, 4}: true}
	for _, d := range out.Designated {
		if !wantDesignated[[2]int16{d[0], d[1]}] {
			t.Errorf("unexpected designated tile %v", d)
		}
	}
	if len(out.Water) != 4 {
		t.Fatalf("expected 4 water tiles, got %d: %v", len(out.Water), out.Water)
	}
	if len(out.Smoothed) != 4 {
		t.Fatalf("expected 4 smoothed tiles (one per block), got %d: %v", len(out.Smoothed), out.Smoothed)
	}
	if len(out.FloorItems) != 4 {
		t.Fatalf("expected 4 floor item tiles (one per block), got %d: %v", len(out.FloorItems), out.FloorItems)
	}
}

func TestStitchSlices_RemainderTile(t *testing.T) {
	// width 100 = 48+48+4: the last column block is a narrower remainder.
	// Single tile-row (height 10, no remainder on that axis) so this test
	// isolates the width-remainder case.
	grid := [][]*Slice{
		{
			quadrantSlice(0, 0, 48, 10, 'A'),
			quadrantSlice(48, 0, 48, 10, 'B'),
			quadrantSlice(96, 0, 4, 10, 'C'),
		},
	}
	out, err := StitchSlices(grid)
	if err != nil {
		t.Fatalf("stitch: %v", err)
	}
	if len(out.Rows) != 10 {
		t.Fatalf("expected 10 rows, got %d", len(out.Rows))
	}
	wantWidth := 48 + 48 + 4
	for i, row := range out.Rows {
		if len(row) != wantWidth {
			t.Fatalf("row %d width = %d, want %d: %q", i, len(row), wantWidth, row)
		}
		if row[47] != 'A' || row[48] != 'B' || row[95] != 'B' || row[96] != 'C' || row[99] != 'C' {
			t.Fatalf("row %d seam wrong: %q", i, row)
		}
	}
	// 3 designated tiles (one per block), all overlay entries survive.
	if len(out.Designated) != 3 {
		t.Fatalf("expected 3 designated tiles, got %d", len(out.Designated))
	}
}

// Stockpile coverage sums across blocks (their windows are disjoint), and
// each block's OWN first-seen class-name table is re-keyed against a merged
// table — otherwise a stitched elevation/fort view would paint block 2's
// index 0 with block 1's class letter.
func TestStitchSlices_StockpileCoverageAndClassTables(t *testing.T) {
	a := quadrantSlice(0, 0, 4, 4, 'A')
	a.StockpileTilesInView, a.StockpileTilesOccupied = intPtr(10), intPtr(4)
	a.FloorItemsNoStock = [][3]int16{{0, 0, 2}}
	a.FloorItemClasses = [][3]int16{{0, 0, 0}, {1, 0, 1}}
	a.FloorItemClassNames = []string{"stone", "food/drink"}

	b := quadrantSlice(4, 0, 4, 4, 'B')
	b.StockpileTilesInView, b.StockpileTilesOccupied = intPtr(6), intPtr(1)
	b.FloorItemsNoStockCapped = true
	// Block b saw food/drink FIRST, so its local index 0 is food/drink and
	// index 1 is furniture — a straight union would mislabel both.
	b.FloorItemClasses = [][3]int16{{4, 0, 0}, {5, 0, 1}}
	b.FloorItemClassNames = []string{"food/drink", "furniture"}

	out, err := StitchSlices([][]*Slice{{a, b}})
	if err != nil {
		t.Fatalf("stitch: %v", err)
	}
	if out.StockpileTilesInView == nil || *out.StockpileTilesInView != 16 {
		t.Fatalf("stockpile tiles must sum to 16, got %v", out.StockpileTilesInView)
	}
	if out.StockpileTilesOccupied == nil || *out.StockpileTilesOccupied != 5 {
		t.Fatalf("occupied tiles must sum to 5, got %v", out.StockpileTilesOccupied)
	}
	if !out.FloorItemsNoStockCapped {
		t.Fatal("a capped block must make the stitched slice report the cap")
	}
	wantNames := []string{"stone", "food/drink", "furniture"}
	if len(out.FloorItemClassNames) != 3 {
		t.Fatalf("merged class table wrong: %v", out.FloorItemClassNames)
	}
	for i, n := range wantNames {
		if out.FloorItemClassNames[i] != n {
			t.Fatalf("merged class table wrong at %d: %v", i, out.FloorItemClassNames)
		}
	}
	want := [][3]int16{{0, 0, 0}, {1, 0, 1}, {4, 0, 1}, {5, 0, 2}}
	if len(out.FloorItemClasses) != len(want) {
		t.Fatalf("class entries: got %v want %v", out.FloorItemClasses, want)
	}
	for i := range want {
		if out.FloorItemClasses[i] != want[i] {
			t.Fatalf("class entry %d remapped wrong: got %v want %v", i, out.FloorItemClasses[i], want[i])
		}
	}
}

// One unmeasured block poisons the whole stitched sum: a partial coverage
// total presented as a whole is the same lie the nil sentinel exists to
// prevent, just quieter.
func TestStitchSlices_MixedCoverageDegradesToNil(t *testing.T) {
	a := quadrantSlice(0, 0, 4, 4, 'A')
	a.StockpileTilesInView, a.StockpileTilesOccupied = intPtr(10), intPtr(4)
	b := quadrantSlice(4, 0, 4, 4, 'B') // no coverage fields

	out, err := StitchSlices([][]*Slice{{a, b}})
	if err != nil {
		t.Fatalf("stitch: %v", err)
	}
	if out.StockpileTilesInView != nil || out.StockpileTilesOccupied != nil {
		t.Fatalf("partial coverage must degrade to nil, got %v/%v",
			out.StockpileTilesInView, out.StockpileTilesOccupied)
	}
}

func TestStitchSlices_RejectsRaggedGrid(t *testing.T) {
	grid := [][]*Slice{
		{quadrantSlice(0, 0, 4, 4, 'A'), quadrantSlice(4, 0, 4, 4, 'B')},
		{quadrantSlice(0, 4, 4, 4, 'C')}, // missing second column
	}
	if _, err := StitchSlices(grid); err == nil {
		t.Fatal("expected error for ragged grid, got nil")
	}
}

func TestStitchSlices_EmptyGrid(t *testing.T) {
	if _, err := StitchSlices(nil); err == nil {
		t.Fatal("expected error for empty grid, got nil")
	}
	if _, err := StitchSlices([][]*Slice{{}}); err == nil {
		t.Fatal("expected error for empty tile-row, got nil")
	}
}
