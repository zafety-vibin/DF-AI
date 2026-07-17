package mapview

import (
	"strings"
	"testing"
)

func TestDownsampleBlockSizeFor(t *testing.T) {
	cases := []struct {
		w, h, maxCells int
		want           int
	}{
		{80, 80, 100, 1},   // fits raw, no downsampling
		{100, 100, 100, 1}, // exactly at cap
		{101, 50, 100, 2},  // one over on one axis forces 2:1
		{200, 200, 100, 2},
		{192, 40, 100, 2}, // driven by the larger dimension
	}
	for _, c := range cases {
		if got := DownsampleBlockSizeFor(c.w, c.h, c.maxCells); got != c.want {
			t.Errorf("DownsampleBlockSizeFor(%d,%d,%d) = %d, want %d", c.w, c.h, c.maxCells, got, c.want)
		}
	}
}

func TestDownsampleGlyphs_MajorityVote(t *testing.T) {
	// 4x2 grid, block=2 -> 2x1 output. Left block: 3 '#' + 1 '.': '#' wins.
	// Right block: 2 '~' + 2 '.': tie -> lexicographically smaller ('.').
	rows := []string{
		"##~.",
		"#.~.",
	}
	got := DownsampleGlyphs(rows, 2)
	want := []string{"#."}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("DownsampleGlyphs = %v, want %v", got, want)
	}
}

func TestDownsampleGlyphs_NoopBelowBlockTwo(t *testing.T) {
	rows := []string{"ab", "cd"}
	got := DownsampleGlyphs(rows, 1)
	if len(got) != 2 || got[0] != "ab" || got[1] != "cd" {
		t.Fatalf("block=1 should be a no-op, got %v", got)
	}
}

func TestDownsampleGlyphs_RemainderBlock(t *testing.T) {
	// width 3, block 2 -> output width 2 (last block is a 1-wide remainder).
	rows := []string{"AAB", "AAB"}
	got := DownsampleGlyphs(rows, 2)
	if len(got) != 1 {
		t.Fatalf("expected 1 output row, got %d", len(got))
	}
	if got[0] != "AB" {
		t.Fatalf("got %q, want %q", got[0], "AB")
	}
}

func TestModalGlyph_EmptyIsDefensive(t *testing.T) {
	if got := modalGlyph(map[rune]int{}); got != '?' {
		t.Errorf("modalGlyph(empty) = %q, want '?'", got)
	}
}

// TestRenderDownsampledSlice_DisclosesBlockSize asserts the header states
// the source dims, output dims, and block size — the same disclosure
// contract RenderOverview holds itself to.
func TestRenderDownsampledSlice_DisclosesBlockSize(t *testing.T) {
	rows := make([]string, 6)
	for i := range rows {
		rows[i] = strings.Repeat(".", 6)
	}
	s := &Slice{Z: 3, X1: 10, Y1: 20, Rows: rows}
	out := RenderDownsampledSlice(s, 2)
	if !strings.Contains(out, "6x6 tiles downsampled to 3x3 cells") {
		t.Fatalf("missing dims disclosure: %q", out)
	}
	if !strings.Contains(out, "1 cell = 2x2 tiles") {
		t.Fatalf("missing block size disclosure: %q", out)
	}
	if !strings.Contains(out, "origin (10,20)") {
		t.Fatalf("missing origin: %q", out)
	}
}

// TestRenderDownsampledSlice_WaterAndDesignationSurvive checks that a block
// containing a designated/water tile among mostly plain floor still shows
// those classes when they hold the majority within their own sub-block.
func TestRenderDownsampledSlice_WaterAndDesignationSurvive(t *testing.T) {
	rows := []string{"..", ".."}
	s := &Slice{Z: 1, X1: 0, Y1: 0, Rows: rows,
		Designated: [][2]int16{{0, 0}, {1, 0}, {0, 1}}, // 3 of 4 tiles designated -> majority
	}
	out := RenderDownsampledSlice(s, 2)
	lines := strings.Split(strings.TrimSpace(strings.Split(out, "\n")[1]), "\n")
	if lines[0] != "d" {
		t.Fatalf("expected the single output cell to be 'd' (designation majority), got %q in:\n%s", lines[0], out)
	}
}

// TestRenderDownsampledSlice_SmoothedCountLine: smoothed tiles get a count
// line, same convention as aquifer — no glyph paint, since a smooth job
// doesn't change shape/material and would be meaningless to majority-vote.
func TestRenderDownsampledSlice_SmoothedCountLine(t *testing.T) {
	rows := []string{"##", "##"}
	s := &Slice{Z: 1, X1: 0, Y1: 0, Rows: rows,
		Smoothed: [][2]int16{{0, 0}, {1, 1}},
	}
	out := RenderDownsampledSlice(s, 2)
	if !strings.Contains(out, "smoothed tiles in source view: 2") {
		t.Fatalf("smoothed count line missing:\n%s", out)
	}
}

// TestRenderDownsampledSlice_FloorItemsCountLine: loose floor items get a
// count line, same convention as smoothed/aquifer — no glyph paint.
func TestRenderDownsampledSlice_FloorItemsCountLine(t *testing.T) {
	rows := []string{"##", "##"}
	s := &Slice{Z: 1, X1: 0, Y1: 0, Rows: rows,
		FloorItems: [][3]int16{{0, 0, 1}, {1, 1, 3}},
	}
	out := RenderDownsampledSlice(s, 2)
	if !strings.Contains(out, "tiles with loose items on floor in source view: 2") {
		t.Fatalf("floor items count line missing:\n%s", out)
	}
}
