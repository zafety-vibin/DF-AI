package mapview

import (
	"strings"
	"testing"
)

func TestRenderCrop(t *testing.T) {
	s := &Slice{Z: 110, X1: 70, Y1: 80, Rows: []string{"##..", "#?.,", ",,,_"}}
	out := RenderCrop(s, []Overlay{{Marks: map[[2]int16]rune{{72, 81}: '@'}}}, "")
	// Header names the Z and orientation; grid has x/y labels; mark applied.
	if !strings.Contains(out, "z=110") || !strings.Contains(out, "north is up") {
		t.Fatalf("missing header: %q", out)
	}
	// Row for y=81 gets the '@' at x=72 (index 2): "#?@,"
	if !strings.Contains(out, "81 #?@,") {
		t.Fatalf("mark not applied on y=81 row: %q", out)
	}
	if !strings.Contains(out, Legend) {
		t.Fatalf("legend missing")
	}
	// No renderer produces a wagon mark — the legend must not invent one.
	if strings.Contains(out, "W wagon") {
		t.Fatalf("legend claims a W wagon mark no renderer draws: %q", out)
	}
	// x-axis label line marks the starting column.
	if !strings.Contains(out, "x=70") {
		t.Fatalf("x label missing: %q", out)
	}
}

// TestRenderCropXAxisAlignment pins the label geometry: the DIGITS of each
// x label must sit exactly over the glyph column they name. Row lines use a
// 5-column "%4d " prefix, so glyph column 0 is line index 5.
func TestRenderCropXAxisAlignment(t *testing.T) {
	const width = 12
	s := &Slice{Z: 110, X1: 70, Y1: 80, Rows: []string{
		strings.Repeat("#", width),
		strings.Repeat(".", width),
	}}
	out := RenderCrop(s, nil, "")
	lines := strings.Split(out, "\n")
	label, row := lines[1], lines[2]

	// Cross-check the row geometry the labels must match: "  80 " prefix,
	// glyphs from index 5.
	if !strings.HasPrefix(row, "  80 #") {
		t.Fatalf("row prefix geometry changed, update alignment: %q", row)
	}
	// First label digits ("70") start over glyph column 0 => line index 5.
	if idx := strings.Index(label, "70"); idx != 5 {
		t.Fatalf("first x label digits at index %d, want 5: %q", idx, label)
	}
	// Second label digits ("81" = 70+12-1) start over the last glyph
	// column => line index 5+width-1.
	if idx := strings.Index(label, "81"); idx != 5+width-1 {
		t.Fatalf("last x label digits at index %d, want %d: %q", idx, 5+width-1, label)
	}
}

// A grid too narrow for two labels keeps just the first, still aligned.
func TestRenderCropXAxisNarrow(t *testing.T) {
	s := &Slice{Z: 5, X1: 70, Y1: 80, Rows: []string{"####"}}
	out := RenderCrop(s, nil, "")
	label := strings.Split(out, "\n")[1]
	if idx := strings.Index(label, "70"); idx != 5 {
		t.Fatalf("first x label digits at index %d, want 5: %q", idx, label)
	}
	if strings.Count(label, "x=") != 1 {
		t.Fatalf("narrow grid must carry exactly one x label: %q", label)
	}
}

// TestRenderCropWater: water tiles overlay the base glyph with their depth
// digit, out-of-range depths are handled defensively, the '@' dwarf mark
// wins over water, and aquifer tiles get a count line (no glyph).
func TestRenderCropWater(t *testing.T) {
	s := &Slice{Z: 100, X1: 10, Y1: 20, Rows: []string{"....", "....", "...."},
		Water: [][3]int16{
			{10, 20, 3}, // depth digit '3'
			{11, 20, 9}, // defensive: >7 clamps to '7'
			{12, 20, 0}, // defensive: <1 ignored
			{13, 21, 7}, // dwarf stands here — '@' must win
			{99, 99, 5}, // out of crop bounds — ignored
		},
		Aquifer: [][2]int16{{10, 22}, {11, 22}}}
	out := RenderCrop(s, []Overlay{{Marks: map[[2]int16]rune{{13, 21}: '@'}}}, "")
	if !strings.Contains(out, "  20 37..") {
		t.Fatalf("water depth digits wrong on y=20 row (want \"37..\"):\n%s", out)
	}
	if !strings.Contains(out, "  21 ...@") {
		t.Fatalf("dwarf mark must override water on y=21 row:\n%s", out)
	}
	if !strings.Contains(out, "aquifer tiles in view: 2") {
		t.Fatalf("aquifer count line missing:\n%s", out)
	}
	// Aquifer is a count line only — the grid keeps its base glyphs.
	if !strings.Contains(out, "  22 ....") {
		t.Fatalf("aquifer must not draw a glyph on y=22 row:\n%s", out)
	}
	if !strings.Contains(out, "1-7 water(depth)") {
		t.Fatalf("legend missing water-depth entry:\n%s", out)
	}
}

// TestRenderCropWater_HiddenTile: a hidden ('?') tile carrying a water
// entry must still get its depth digit painted — RenderCrop's water overlay
// is unconditional (see the "always on" doc comment above), matching the
// plugin's map_slice fix that stopped excluding hidden tiles from the
// water[] array (queries.cpp queryMapSlice). Pins the Go side of the same
// fog-honesty contract exercised for column_profile in
// TestRenderColumnFluids_HiddenWater.
func TestRenderCropWater_HiddenTile(t *testing.T) {
	s := &Slice{Z: 118, X1: 51, Y1: 46, Rows: []string{"?"},
		Water: [][3]int16{{51, 46, 7}}}
	out := RenderCrop(s, nil, "")
	if !strings.Contains(out, "  46 7") {
		t.Fatalf("water digit must overlay the hidden glyph, not defer to '?':\n%s", out)
	}
}

// A slice without water/aquifer fields (older plugin) renders exactly as
// before: no aquifer line, no overlay.
func TestRenderCropNoWaterFields(t *testing.T) {
	s := &Slice{Z: 5, X1: 0, Y1: 0, Rows: []string{"##"}}
	out := RenderCrop(s, nil, "")
	if strings.Contains(out, "aquifer") {
		t.Fatalf("no aquifer line expected without aquifer data:\n%s", out)
	}
	if !strings.Contains(out, "   0 ##") {
		t.Fatalf("base glyphs must be untouched:\n%s", out)
	}
}

// TestRenderCropSmoothed: smoothed tiles get a count line, not a glyph —
// same convention as aquifer, since a smooth job doesn't change shape/material.
func TestRenderCropSmoothed(t *testing.T) {
	s := &Slice{Z: 100, X1: 10, Y1: 20, Rows: []string{"##"},
		Smoothed: [][2]int16{{10, 20}, {11, 20}}}
	out := RenderCrop(s, nil, "")
	if !strings.Contains(out, "smoothed tiles in view: 2") {
		t.Fatalf("smoothed count line missing:\n%s", out)
	}
	if !strings.Contains(out, "  20 ##") {
		t.Fatalf("smoothed must not draw a glyph on y=20 row:\n%s", out)
	}
}

// TestRenderCropFloorItems: loose floor items get a text footer, not a
// glyph — same convention as smoothed/aquifer, since the base terrain
// classifier renders such a tile as plain clean floor. Against a plugin
// that never measured stockpile coverage the footer reports totals and
// cluster positions only, and says so.
func TestRenderCropFloorItems(t *testing.T) {
	s := &Slice{Z: 100, X1: 10, Y1: 20, Rows: []string{"##"},
		FloorItems: [][3]int16{{10, 20, 2}}}
	out := RenderCrop(s, nil, "")
	if !strings.Contains(out, "loose items in view: 2 on 1 tiles (stockpile coverage not reported by this plugin build) — clusters: (10,20) 2 items") {
		t.Fatalf("floor items footer missing:\n%s", out)
	}
	if !strings.Contains(out, "  20 ##") {
		t.Fatalf("floor items must not draw a glyph on y=20 row:\n%s", out)
	}
}

// TestRenderCropFloorItemsSplit: with a plugin that measured stockpile
// coverage, the same view splits homeless clutter out of the total and
// hands back its cluster bbox — the line that would have made a live
// session notice a stockpile shortage on its own.
func TestRenderCropFloorItemsSplit(t *testing.T) {
	inView, occupied := 12, 1
	s := &Slice{Z: 100, X1: 10, Y1: 20, Rows: []string{"####", "####"},
		FloorItems:             [][3]int16{{10, 20, 2}, {12, 21, 30}, {13, 21, 25}},
		FloorItemsNoStock:      [][3]int16{{12, 21, 30}, {13, 21, 25}},
		StockpileTilesInView:   &inView,
		StockpileTilesOccupied: &occupied}
	out := RenderCrop(s, nil, "")
	want := "loose items OUTSIDE stockpiles: 55 items on 2 tiles (view total 57 on 3 tiles) — clusters: (12,21)-(13,21) 55 items (lens=items for classes)"
	if !strings.Contains(out, want) {
		t.Fatalf("homeless split line missing:\n%s", out)
	}
	if !strings.Contains(out, "stockpile tiles in view: 12, 1 with items on them (8% of tiles taken)") {
		t.Fatalf("stockpile fill line missing:\n%s", out)
	}
	if strings.Contains(out, "tiles with loose items on floor") {
		t.Fatalf("the legacy count line must be gone:\n%s", out)
	}
}

// TestRenderCropPendingBuilding: a tile carrying a pending-building entry
// (DF's tile_building_occ::Planned occupancy — any building type, including
// a wall/floor Construction) renders the always-on 'u' glyph, gains a
// legend entry, and gets a count line — the same "always on, count line
// only when nonzero" treatment as designations/aquifer/smoothed/floor
// items. This is the actual fix: plain look (no lens) must show a queued
// building without requiring lens=buildings.
func TestRenderCropPendingBuilding(t *testing.T) {
	s := &Slice{Z: 100, X1: 10, Y1: 20, Rows: []string{"##", "##"},
		PendingBuilding: [][2]int16{{10, 20}}}
	out := RenderCrop(s, nil, "")
	if !strings.Contains(out, "  20 u#") {
		t.Fatalf("pending-building glyph not painted on y=20 row (want \"u#\"):\n%s", out)
	}
	if !strings.Contains(out, "u pending-building") {
		t.Fatalf("legend missing pending-building entry:\n%s", out)
	}
	if !strings.Contains(out, "pending buildings in view: 1 tiles (exact type: lens=buildings)") {
		t.Fatalf("pending-building count line missing:\n%s", out)
	}
	// y=21 carries no pending-building entry — must stay plain base glyphs.
	if !strings.Contains(out, "  21 ##") {
		t.Fatalf("pending-building must not leak onto y=21 row:\n%s", out)
	}
}

// TestRenderCropNoPendingBuildingFields: a slice without the pending_building
// field (older plugin) renders exactly as before — no glyph, no count line.
func TestRenderCropNoPendingBuildingFields(t *testing.T) {
	s := &Slice{Z: 5, X1: 0, Y1: 0, Rows: []string{"##"}}
	out := RenderCrop(s, nil, "")
	if strings.Contains(out, "pending building") {
		t.Fatalf("no pending-building line expected without pending_building data:\n%s", out)
	}
	if !strings.Contains(out, "   0 ##") {
		t.Fatalf("base glyphs must be untouched:\n%s", out)
	}
}

func TestRenderCropDesignated(t *testing.T) {
	s := &Slice{Z: 5, X1: 0, Y1: 0, Rows: []string{"##", "##"},
		Designated: [][2]int16{{0, 0}, {1, 1}}}
	out := RenderCrop(s, nil, "")
	if !strings.Contains(out, "designated for digging: 2 tiles in view") {
		t.Fatalf("designated summary missing: %q", out)
	}
}

func TestRenderCrop_DesignationsAlwaysPainted(t *testing.T) {
	s := &Slice{
		Z: 100, X1: 0, Y1: 0,
		Rows:       []string{"..", ".."},
		Designated: [][2]int16{{1, 0}},
	}
	out := RenderCrop(s, nil, "")
	if !strings.Contains(out, "d designated") {
		t.Fatalf("legend must gain a 'd designated' entry:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	// Row 0 (y=0) is rendered on the line starting "   0 " (4-wide right-aligned + space).
	found := false
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimLeft(l, " "), "0 ") {
			// glyph at x=1 should be 'd', not '.'
			if strings.Contains(l, ".d") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("designated tile at (1,0) must render as 'd':\n%s", out)
	}
}

func TestRenderCrop_OverlayPaintsLastAndAddsFootnoteOnDwarfCollision(t *testing.T) {
	s := &Slice{Z: 100, X1: 0, Y1: 0, Rows: []string{".."}}
	dwarfMarks := Overlay{Marks: map[[2]int16]rune{{0, 0}: '@'}}
	lensOverlay := Overlay{
		Marks:     map[[2]int16]rune{{0, 0}: 'B'},
		Footnotes: []string{"dwarves in view: 1 (1 under overlay at (0,0))"},
	}
	out := RenderCrop(s, []Overlay{dwarfMarks, lensOverlay}, "lens=buildings: B furniture")
	if !strings.Contains(out, "under overlay at (0,0)") {
		t.Fatalf("expected the collision footnote:\n%s", out)
	}
	if !strings.Contains(out, "lens=buildings: B furniture") {
		t.Fatalf("expected the lens legend addendum:\n%s", out)
	}
}

func TestRenderCrop_NoLensNoAddendum(t *testing.T) {
	s := &Slice{Z: 100, X1: 0, Y1: 0, Rows: []string{"."}}
	out := RenderCrop(s, nil, "")
	if strings.Contains(out, "lens=") {
		t.Fatalf("no lens active must mean no lens addendum:\n%s", out)
	}
}

// TestRenderCropMinerals exercises RenderCrop the way lens=minerals uses it
// (internal/mcpserver's gatherMineralsLens builds this exact overlay shape
// from a Slice's Minerals/MineralNames fields — those live on the Slice
// itself, set here, even though RenderCrop only ever consumes the Overlay,
// never the raw fields directly): a fake slice with 2+ distinct minerals in
// view, painted as letters keyed to a per-view legend footnote, mirroring
// lens=buildings' category-glyph treatment but with a dynamic (not fixed)
// legend line.
func TestRenderCropMinerals(t *testing.T) {
	s := &Slice{Z: 100, X1: 10, Y1: 20, Rows: []string{"=="},
		Minerals:     [][3]int16{{10, 20, 0}, {11, 20, 1}},
		MineralNames: []string{"limonite", "native copper"}}
	mineralsOverlay := Overlay{
		Marks:     map[[2]int16]rune{{10, 20}: 'a', {11, 20}: 'b'},
		Footnotes: []string{"this view's minerals: a=limonite b=native copper"},
	}
	out := RenderCrop(s, []Overlay{mineralsOverlay}, "lens=minerals: vein tiles painted a,b,c...")
	if !strings.Contains(out, "  20 ab") {
		t.Fatalf("mineral letters not painted on y=20 row (want \"ab\"):\n%s", out)
	}
	if !strings.Contains(out, "lens=minerals: vein tiles painted a,b,c...") {
		t.Fatalf("expected the lens legend addendum:\n%s", out)
	}
	if !strings.Contains(out, "a=limonite b=native copper") {
		t.Fatalf("expected the per-view mineral footnote:\n%s", out)
	}
}

func TestRenderColumn(t *testing.T) {
	c := &ColumnProfile{X: 72, Y: 81, Levels: []ColumnLevel{
		{Z: 111, Glyph: "_", Shape: "open", Material: "air"},
		{Z: 110, Glyph: ",", Shape: "floor", Material: "grass"},
		{Z: 109, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true},
		{Z: 108, Glyph: "?", Shape: "wall", Material: "stone", Hidden: true},
	}}
	out := RenderColumn(c)
	if !strings.Contains(out, "column (72,81)") {
		t.Fatalf("header missing: %q", out)
	}
	if !strings.Contains(out, "z=110") || !strings.Contains(out, "<- SURFACE") {
		t.Fatalf("surface annotation missing: %q", out)
	}
	if !strings.Contains(out, "<- first layer below surface") {
		t.Fatalf("below-surface annotation missing: %q", out)
	}
	if !strings.Contains(out, "(hidden/undug — diggable)") {
		t.Fatalf("hidden annotation missing: %q", out)
	}
	if !strings.Contains(out, "wall/soil") {
		t.Fatalf("shape/material missing: %q", out)
	}
	// No fluid fields set — no fluid annotations.
	for _, banned := range []string{"water", "AQUIFER", "DAMP"} {
		if strings.Contains(out, banned) {
			t.Fatalf("unexpected fluid annotation %q: %q", banned, out)
		}
	}
}

// TestRenderColumnFluids: water depth, aquifer, and damp annotations append
// to the level line; levels without them stay clean.
func TestRenderColumnFluids(t *testing.T) {
	c := &ColumnProfile{X: 72, Y: 81, Levels: []ColumnLevel{
		{Z: 110, Glyph: "~", Shape: "floor", Material: "water", Water: 4},
		{Z: 109, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true, Aquifer: true, Damp: true},
		{Z: 108, Glyph: "?", Shape: "wall", Material: "stone", Hidden: true, Damp: true},
		{Z: 107, Glyph: "?", Shape: "wall", Material: "stone", Hidden: true},
	}}
	out := RenderColumn(c)
	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[1], "~4/7 water") {
		t.Fatalf("water depth annotation missing on z=110: %q", lines[1])
	}
	if !strings.Contains(lines[2], "AQUIFER") || !strings.Contains(lines[2], "DAMP") {
		t.Fatalf("aquifer/damp annotations missing on z=109: %q", lines[2])
	}
	if strings.Contains(lines[3], "AQUIFER") || !strings.Contains(lines[3], "DAMP") {
		t.Fatalf("z=108 must be DAMP but not AQUIFER: %q", lines[3])
	}
	for _, banned := range []string{"water", "AQUIFER", "DAMP"} {
		if strings.Contains(lines[4], banned) {
			t.Fatalf("clean level must carry no fluid annotation: %q", lines[4])
		}
	}
	// Annotations must not displace the hidden marker.
	if !strings.Contains(lines[2], "(hidden/undug — diggable)") {
		t.Fatalf("hidden marker lost on annotated level: %q", lines[2])
	}
}

// TestRenderColumnSmooth: a completed smooth job annotates its level line
// with SMOOTHED, alongside (not instead of) any fluid annotations, since
// shape/material don't change on smooth and the glyph looks identical
// before and after.
func TestRenderColumnSmooth(t *testing.T) {
	c := &ColumnProfile{X: 72, Y: 81, Levels: []ColumnLevel{
		{Z: 110, Glyph: "#", Shape: "wall", Material: "stone", Smooth: true},
		{Z: 109, Glyph: "#", Shape: "wall", Material: "stone"},
	}}
	out := RenderColumn(c)
	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[1], "SMOOTHED") {
		t.Fatalf("smoothed annotation missing on z=110: %q", lines[1])
	}
	if strings.Contains(lines[2], "SMOOTHED") {
		t.Fatalf("unsmoothed level must not carry SMOOTHED: %q", lines[2])
	}
}

// TestRenderColumnFloorItems: a tile with loose items at rest gets an
// "ITEM(S) ON FLOOR" annotation alongside (not instead of) other fluid
// annotations.
func TestRenderColumnFloorItems(t *testing.T) {
	c := &ColumnProfile{X: 72, Y: 81, Levels: []ColumnLevel{
		{Z: 110, Glyph: ".", Shape: "floor", Material: "rock_or_soil", FloorItems: 2},
		{Z: 109, Glyph: "~", Shape: "floor", Material: "water", Water: 4, FloorItems: 1},
		{Z: 108, Glyph: ".", Shape: "floor", Material: "rock_or_soil"},
	}}
	out := RenderColumn(c)
	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[1], "2 ITEM(S) ON FLOOR") {
		t.Fatalf("floor items annotation missing on z=110: %q", lines[1])
	}
	if !strings.Contains(lines[2], "~4/7 water") || !strings.Contains(lines[2], "1 ITEM(S) ON FLOOR") {
		t.Fatalf("expected both water and floor items annotations on z=109: %q", lines[2])
	}
	if strings.Contains(lines[3], "ITEM(S) ON FLOOR") {
		t.Fatalf("clean level must not carry floor items annotation: %q", lines[3])
	}
}

// TestRenderColumnFluids_HiddenWater: a hidden tile carrying real standing
// water (flow_size>0 from the plugin, regardless of the fog bit) must still
// render its water annotation — this is the exact live incident: a hidden
// under-brook channel tile held 7/7 water but the plugin's map_slice path
// once dropped hidden water from its response (fixed in queries.cpp's
// queryMapSlice; column_profile's queryColumnProfile never had this gate).
// This fixture pins the Go-side contract: hidden must never suppress a
// present water annotation, whichever query produced it.
func TestRenderColumnFluids_HiddenWater(t *testing.T) {
	c := &ColumnProfile{X: 51, Y: 46, Levels: []ColumnLevel{
		{Z: 119, Glyph: ",", Shape: "floor", Material: "grass"},
		{Z: 118, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true, Water: 7},
	}}
	out := RenderColumn(c)
	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[2], "(hidden/undug — diggable)") {
		t.Fatalf("hidden marker missing on z=118: %q", lines[2])
	}
	if !strings.Contains(lines[2], "~7/7 water") {
		t.Fatalf("hidden tile must still carry its water annotation: %q", lines[2])
	}
}

// TestRenderColumn_TopAlreadySolid: a window whose top level (Levels[0])
// is already solid stone (e.g. an explicit deep-stone z_top/z_bottom pick,
// or a hill rising above the sampled window) must NOT be stamped SURFACE —
// that was the exact live bug (blocking review finding, surface-honesty).
// It gets the distinct "raise z_top" inconclusive message instead.
func TestRenderColumn_TopAlreadySolid(t *testing.T) {
	c := &ColumnProfile{X: 72, Y: 81, Levels: []ColumnLevel{
		{Z: 111, Glyph: "#", Shape: "wall", Material: "stone"},
		{Z: 110, Glyph: "#", Shape: "wall", Material: "stone"},
	}}
	out := RenderColumn(c)
	if strings.Contains(out, "SURFACE") {
		t.Fatalf("solid Levels[0] must never be stamped SURFACE: %q", out)
	}
	if !strings.Contains(out, "surface at or above z_top — raise z_top") {
		t.Fatalf("missing raise-z_top inconclusive message: %q", out)
	}
}

// TestRenderColumn_AllAir: a window that never leaves open air gets the
// distinct "widen z_bottom" inconclusive message (opposite fix from the
// top-already-solid case above — the two must not share one message).
func TestRenderColumn_AllAir(t *testing.T) {
	c := &ColumnProfile{X: 72, Y: 81, Levels: []ColumnLevel{
		{Z: 111, Glyph: "_", Shape: "open", Material: "air"},
		{Z: 110, Glyph: "_", Shape: "open", Material: "air"},
	}}
	out := RenderColumn(c)
	if strings.Contains(out, "SURFACE") {
		t.Fatalf("all-air column must never be stamped SURFACE: %q", out)
	}
	if !strings.Contains(out, "no surface in this z-window — widen z_bottom") {
		t.Fatalf("missing widen-z_bottom inconclusive message: %q", out)
	}
}

func TestRenderElevation_XAxisSweep(t *testing.T) {
	col1 := &ColumnProfile{X: 10, Y: 50, Levels: []ColumnLevel{
		{Z: 100, Glyph: ".", Shape: "floor", Material: "grass"},
		{Z: 99, Glyph: "#", Shape: "wall", Material: "stone"},
	}}
	col2 := &ColumnProfile{X: 11, Y: 50, Levels: []ColumnLevel{
		{Z: 100, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true},
		{Z: 99, Glyph: "#", Shape: "wall", Material: "stone"},
	}}
	out := RenderElevation([]*ColumnProfile{col1, col2}, "x", 50)
	if !strings.Contains(out, "elevation along x=10..11 at y=50") {
		t.Fatalf("missing header:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	var z100Line, z99Line string
	for _, l := range lines {
		if strings.HasPrefix(l, "z=100 ") {
			z100Line = l
		}
		if strings.HasPrefix(l, "z=99 ") {
			z99Line = l
		}
	}
	if !strings.Contains(z100Line, ".?") {
		t.Fatalf("z=100 row must read '.?' (col1 floor, col2 hidden wall):\n%s", z100Line)
	}
	if !strings.Contains(z99Line, "##") {
		t.Fatalf("z=99 row must read '##':\n%s", z99Line)
	}
}

// TestRenderElevation_SurfacesLine covers the "surfaces:" summary's three
// distinct outcomes side by side: a confirmed surface, a column whose
// window-top is already solid (terrain rising above the sweep — "above"),
// and a column that never leaves open air ("none"). Folding the middle
// case into a confirmed number was the exact live bug this task fixes.
func TestRenderElevation_SurfacesLine(t *testing.T) {
	confirmed := &ColumnProfile{X: 10, Y: 50, Levels: []ColumnLevel{
		{Z: 100, Glyph: "_", Shape: "open", Material: "air"},
		{Z: 99, Glyph: "#", Shape: "wall", Material: "stone"},
	}}
	above := &ColumnProfile{X: 11, Y: 50, Levels: []ColumnLevel{
		{Z: 100, Glyph: "#", Shape: "wall", Material: "stone"},
		{Z: 99, Glyph: "#", Shape: "wall", Material: "stone"},
	}}
	allAir := &ColumnProfile{X: 12, Y: 50, Levels: []ColumnLevel{
		{Z: 100, Glyph: "_", Shape: "open", Material: "air"},
		{Z: 99, Glyph: "_", Shape: "open", Material: "air"},
	}}
	out := RenderElevation([]*ColumnProfile{confirmed, above, allAir}, "x", 50)
	for _, want := range []string{"x=10:99", "x=11:above", "x=12:none"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing surfaces entry %q:\n%s", want, out)
		}
	}
}
