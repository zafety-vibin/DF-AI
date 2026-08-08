package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/mapview"
)

// baseTerrainGlyphSet mirrors internal/mapview.Legend's occupied
// characters plus the always-on overlay glyphs, EXCLUDING 'd' — the
// designations lens deliberately reuses 'd' for the Default kind (see
// design doc "Glyph governance", the one documented exception).
var baseTerrainGlyphSet = map[rune]bool{
	'?': true, '#': true, '%': true, '=': true, '.': true, ',': true,
	'T': true, 't': true, '_': true, '<': true, '>': true, 'X': true,
	'^': true, '~': true, 'L': true, 'F': true, '@': true, 'u': true,
	'1': true, '2': true, '3': true, '4': true, '5': true, '6': true, '7': true,
}

func TestLensGlyphsDisjointFromBaseSet(t *testing.T) {
	for name, def := range lenses {
		for _, g := range lensGlyphSet(name) {
			if name == "designations" && g == 'd' {
				continue // documented exception: refines the base 'd' overlay
			}
			if baseTerrainGlyphSet[g] {
				t.Errorf("lens %q glyph %q collides with the base terrain set", name, string(g))
			}
		}
		if def.Legend == "" {
			t.Errorf("lens %q must have a non-empty legend addendum", name)
		}
	}
}

func TestBuildingCategoryGlyph(t *testing.T) {
	cases := map[string]rune{
		"Workshop": 'W', "Furnace": 'W',
		"Bed": 'B', "Chair": 'B', "Table": 'B',
		"Door": 'D', "Hatch": 'D',
		"Stockpile": 'S',
		"ScrewPump": 'M', "Well": 'M', "Bridge": 'M',
		"Trap": 'P', "Cage": 'P',
		"Construction": 'C',
		"TradeDepot":   'O', "Wagon": 'O',
	}
	for buildingType, want := range cases {
		got := buildingCategoryGlyph(buildingType)
		if got != want {
			t.Errorf("buildingCategoryGlyph(%q) = %q, want %q", buildingType, string(got), string(want))
		}
	}
}

func TestBuildingCategoryGlyph_CaseEncodesConstructionState(t *testing.T) {
	if g := buildingOverlayGlyph(buildingListEntry{Type: "Bed", Done: true}); g != 'B' {
		t.Errorf("built furniture must render uppercase, got %q", string(g))
	}
	if g := buildingOverlayGlyph(buildingListEntry{Type: "Bed", Done: false}); g != 'b' {
		t.Errorf("planned/in-progress furniture must render lowercase, got %q", string(g))
	}
}

func TestDesignationKindGlyph(t *testing.T) {
	cases := map[int16]rune{0: 'd', 1: 'c', 2: 'r', 3: 's', 4: 'm'}
	for kind, want := range cases {
		if got := designationKindGlyph(kind); got != want {
			t.Errorf("designationKindGlyph(%d) = %q, want %q", kind, string(got), string(want))
		}
	}
}

// TestGatherMineralsLens covers the actual feature: a fake slice with 2+
// distinct minerals gets letter-painted (keyed by index into MineralNames)
// and produces a dynamic legend footnote — unlike buildings/zones/
// designations, minerals has no fixed category vocabulary, so its legend
// text can't live in LensDef.Legend and must come from the Gather call
// itself.
func TestGatherMineralsLens(t *testing.T) {
	s := &mapview.Slice{
		Minerals:     [][3]int16{{10, 20, 0}, {11, 20, 1}},
		MineralNames: []string{"limonite", "native copper"},
	}
	ov, err := gatherMineralsLens(context.Background(), nil, s, 100)
	if err != nil {
		t.Fatalf("gatherMineralsLens: %v", err)
	}
	if ov.Marks[[2]int16{10, 20}] != 'a' || ov.Marks[[2]int16{11, 20}] != 'b' {
		t.Fatalf("expected a/b letter marks, got %+v", ov.Marks)
	}
	if len(ov.Footnotes) != 1 || !strings.Contains(ov.Footnotes[0], "a=limonite") || !strings.Contains(ov.Footnotes[0], "b=native copper") {
		t.Fatalf("expected a dynamic legend footnote naming both minerals, got %+v", ov.Footnotes)
	}
}

// TestGatherMineralsLens_NoMinerals covers a slice with no vein tiles at
// all (the common case away from an industry quarter) — no marks, no
// footnote, matching the other lenses' behavior on an empty gather.
func TestGatherMineralsLens_NoMinerals(t *testing.T) {
	s := &mapview.Slice{}
	ov, err := gatherMineralsLens(context.Background(), nil, s, 100)
	if err != nil {
		t.Fatalf("gatherMineralsLens: %v", err)
	}
	if len(ov.Marks) != 0 || len(ov.Footnotes) != 0 {
		t.Fatalf("expected no marks/footnotes for a mineral-free slice, got %+v", ov)
	}
}

// TestMineralLetterAlphabet_SkipsReservedGlyphs pins the exact reason
// minerals can't just use a-z in order: 'd' (dig designation, always-on),
// 't' (sapling/shrub), and 'u' (pending-building) are already reserved
// base-terrain/always-on glyphs — TestLensGlyphsDisjointFromBaseSet
// enforces this for every lens, but this test names the specific letters
// so a future edit that reintroduces them fails with a direct message
// instead of a generic disjointness error.
func TestMineralLetterAlphabet_SkipsReservedGlyphs(t *testing.T) {
	if len(mineralLetterAlphabet) != 23 {
		t.Fatalf("expected 23 letters (26 minus d/t/u), got %d: %v", len(mineralLetterAlphabet), mineralLetterAlphabet)
	}
	for _, r := range mineralLetterAlphabet {
		if r == 'd' || r == 't' || r == 'u' {
			t.Fatalf("mineralLetterAlphabet must not contain reserved glyph %q", string(r))
		}
	}
}

// The items alphabet must skip both the three reserved lowercase glyphs
// (d/t/u) AND the three letters whose UPPERCASE forms are base terrain
// glyphs (x->X up/down-stair, l->L magma, f->F fortification) — this lens
// paints both cases, so it needs the stricter exclusion list.
// TestLensGlyphsDisjointFromBaseSet enforces the invariant generically;
// this test names the letters so a future edit fails with a direct message.
func TestItemLetterAlphabet_SkipsReservedAndUppercaseCollisions(t *testing.T) {
	if len(itemLetterAlphabet) != 20 {
		t.Fatalf("expected 20 letters (26 minus d/t/u/x/l/f), got %d: %v", len(itemLetterAlphabet), itemLetterAlphabet)
	}
	for _, r := range itemLetterAlphabet {
		switch r {
		case 'd', 't', 'u', 'x', 'l', 'f':
			t.Fatalf("itemLetterAlphabet must not contain %q", string(r))
		}
	}
	glyphs := lensGlyphSet("items")
	if len(glyphs) != 40 {
		t.Fatalf("items lens must declare both cases (40 glyphs), got %d", len(glyphs))
	}
}

// The headline lens behaviour: class letters keyed to a per-view footnote,
// UPPERCASE for tiles outside every stockpile.
func TestBuildItemsOverlay(t *testing.T) {
	s := &mapview.Slice{
		Z: 132, X1: 10, Y1: 10, Rows: []string{"....", "...."},
		FloorItems: [][3]int16{
			{10, 10, 41}, {11, 10, 9}, // stockpiled stone + stone
			{12, 11, 58}, // homeless food
		},
		FloorItemsNoStock:   [][3]int16{{12, 11, 58}},
		FloorItemClasses:    [][3]int16{{10, 10, 0}, {11, 10, 0}, {12, 11, 1}},
		FloorItemClassNames: []string{"stone", "food/drink"},
	}
	ov := buildItemsOverlay(s)
	if got := ov.Marks[[2]int16{10, 10}]; got != 'a' {
		t.Fatalf("stockpiled stone must paint lowercase 'a', got %q", string(got))
	}
	if got := ov.Marks[[2]int16{11, 10}]; got != 'a' {
		t.Fatalf("second stockpiled stone tile must share the class letter, got %q", string(got))
	}
	if got := ov.Marks[[2]int16{12, 11}]; got != 'B' {
		t.Fatalf("homeless food must paint UPPERCASE 'B', got %q", string(got))
	}
	if len(ov.Footnotes) != 1 {
		t.Fatalf("expected exactly one footnote, got %q", ov.Footnotes)
	}
	want := "this view's item classes: a=stone (50 items/2 tiles) b=food/drink (58 items/1 tiles)"
	if ov.Footnotes[0] != want {
		t.Fatalf("class footnote wrong:\n got %q\nwant %q", ov.Footnotes[0], want)
	}
}

// A class present in the name table but with no tiles inside the crop
// (possible after a stitched merge) must not pad the footnote.
func TestBuildItemsOverlay_SkipsUnusedClasses(t *testing.T) {
	s := &mapview.Slice{
		FloorItems:          [][3]int16{{1, 1, 2}},
		FloorItemClasses:    [][3]int16{{1, 1, 1}},
		FloorItemClassNames: []string{"stone", "wood"},
	}
	ov := buildItemsOverlay(s)
	if strings.Contains(ov.Footnotes[0], "a=stone") {
		t.Fatalf("a class with no tiles must not appear: %q", ov.Footnotes[0])
	}
	if !strings.Contains(ov.Footnotes[0], "b=wood (2 items/1 tiles)") {
		t.Fatalf("used class missing: %q", ov.Footnotes[0])
	}
}

// The plugin caps its class array at 200 tiles while floor_items stays
// uncapped — the difference is reported, not silently unpainted.
func TestBuildItemsOverlay_ReportsUnlabeledTiles(t *testing.T) {
	s := &mapview.Slice{
		FloorItems:          [][3]int16{{1, 1, 2}, {5, 5, 3}, {6, 6, 1}},
		FloorItemClasses:    [][3]int16{{1, 1, 0}},
		FloorItemClassNames: []string{"stone"},
	}
	ov := buildItemsOverlay(s)
	if len(ov.Footnotes) != 2 || !strings.Contains(ov.Footnotes[1], "+2 item-bearing tile(s) past the plugin's 200-tile class cap") {
		t.Fatalf("unlabeled-tile footnote missing: %q", ov.Footnotes)
	}
}

// This lens encodes homelessness in letter CASE, so a truncated homeless
// list makes lowercase ("inside a stockpile") an assertion nothing
// verified. When the plugin flags its 200-tile cap, the render must say so
// — self-disclosure that holds even if the plugin's two caps are ever
// changed out from under the ordering invariant that protects it today.
func TestBuildItemsOverlay_NoStockCapDisclosed(t *testing.T) {
	s := &mapview.Slice{
		FloorItems:              [][3]int16{{1, 1, 2}},
		FloorItemsNoStock:       [][3]int16{{1, 1, 2}},
		FloorItemsNoStockCapped: true,
		FloorItemClasses:        [][3]int16{{1, 1, 0}},
		FloorItemClassNames:     []string{"stone"},
	}
	ov := buildItemsOverlay(s)
	joined := strings.Join(ov.Footnotes, "\n")
	if !strings.Contains(joined, "outside-stockpile tile list hit its 200-tile cap") {
		t.Fatalf("capped homeless list must be disclosed: %q", ov.Footnotes)
	}
	// Uncapped views must not carry the warning at all.
	s.FloorItemsNoStockCapped = false
	if joined := strings.Join(buildItemsOverlay(s).Footnotes, "\n"); strings.Contains(joined, "200-tile cap") {
		t.Fatalf("an uncapped view must not warn about truncation: %q", joined)
	}
}

// VERSION SKEW: an older plugin sends floor_items but no class table and
// no homeless split. Every tile gets one uniform letter, and the footnote
// states outright that case carries no meaning — a case-encoded glyph
// whose case is unknown must never read as an assertion.
func TestBuildItemsOverlay_OldPluginFallback(t *testing.T) {
	s := &mapview.Slice{FloorItems: [][3]int16{{3, 3, 1}, {4, 4, 9}}}
	ov := buildItemsOverlay(s)
	for _, pos := range [][2]int16{{3, 3}, {4, 4}} {
		if got := ov.Marks[pos]; got != 'i' {
			t.Fatalf("old-plugin fallback must paint 'i' at %v, got %q", pos, string(got))
		}
	}
	if len(ov.Footnotes) != 1 || !strings.Contains(ov.Footnotes[0], "case carries NO meaning") {
		t.Fatalf("fallback footnote must disclaim the case split: %q", ov.Footnotes)
	}
	if strings.Contains(ov.Footnotes[0], "this view's item classes") {
		t.Fatalf("fallback must not fabricate a class table: %q", ov.Footnotes[0])
	}
}

// A view with no loose items paints nothing and says nothing.
func TestBuildItemsOverlay_Empty(t *testing.T) {
	ov := buildItemsOverlay(&mapview.Slice{})
	if len(ov.Marks) != 0 || len(ov.Footnotes) != 0 {
		t.Fatalf("empty view must produce no marks/footnotes: %+v", ov)
	}
}

func TestLensNamesErrorMessage(t *testing.T) {
	names := strings.Join(lensNames(), ", ")
	if !strings.Contains(names, "buildings") || !strings.Contains(names, "designations") {
		t.Fatalf("lensNames must list both registered lenses, got %q", names)
	}
}

func TestZoneCategoryGlyph(t *testing.T) {
	cases := map[string]rune{
		"Bedroom": 'H', "Office": 'H', "Tomb": 'H', "DiningHall": 'H', "MeetingHall": 'H', "Dormitory": 'H',
		"Barracks": 'K',
		"Pen":      'A', "Pond": 'A', "AnimalTraining": 'A', "ArcheryRange": 'A',
		"WaterSource": 'R', "Dump": 'R', "SandCollection": 'R', "FishingArea": 'R', "ClayCollection": 'R', "PlantGathering": 'R',
		"Dungeon": 'J',
	}
	for typeName, want := range cases {
		if got := zoneCategoryGlyph(typeName); got != want {
			t.Errorf("zoneCategoryGlyph(%q) = %q, want %q", typeName, string(got), string(want))
		}
	}
}

// TestDwarfCollisionFootnote covers the design doc's "never silently hide
// a dwarf" requirement (Component 2): a lens overpainting a dwarf's tile
// must produce a footnote in the exact quoted format ("dwarves in view: 3
// (2 under overlay at (46,50) (47,51))"), not silence.
func TestDwarfCollisionFootnote(t *testing.T) {
	cases := []struct {
		name       string
		dwarfMarks map[[2]int16]rune
		lensMarks  map[[2]int16]rune
		want       []string
	}{
		{
			name:       "no dwarves in view",
			dwarfMarks: map[[2]int16]rune{},
			lensMarks:  map[[2]int16]rune{{10, 10}: 'W'},
			want:       nil,
		},
		{
			name:       "dwarves present but no collision",
			dwarfMarks: map[[2]int16]rune{{5, 5}: '@'},
			lensMarks:  map[[2]int16]rune{{10, 10}: 'W'},
			want:       nil,
		},
		{
			name:       "one collision",
			dwarfMarks: map[[2]int16]rune{{46, 50}: '@'},
			lensMarks:  map[[2]int16]rune{{46, 50}: 'W'},
			want:       []string{"dwarves in view: 1 (1 under overlay at (46,50))"},
		},
		{
			name:       "multiple collisions, sorted",
			dwarfMarks: map[[2]int16]rune{{47, 51}: '@', {46, 50}: '@', {1, 1}: '@'},
			lensMarks:  map[[2]int16]rune{{46, 50}: 'W', {47, 51}: 'W'},
			want:       []string{"dwarves in view: 3 (2 under overlay at (46,50) (47,51))"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dwarfCollisionFootnote(tc.dwarfMarks, tc.lensMarks)
			if len(got) != len(tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %#v, want %#v", got, tc.want)
				}
			}
		})
	}
}

func TestWildlifeGlyph(t *testing.T) {
	if g := wildlifeGlyph(true); g != 'V' {
		t.Errorf("wildlifeGlyph(true) = %q, want 'V'", string(g))
	}
	if g := wildlifeGlyph(false); g != 'v' {
		t.Errorf("wildlifeGlyph(false) = %q, want 'v'", string(g))
	}
}

func TestWildlifeFootnoteLine(t *testing.T) {
	dangerous := wildlifeListEntry{Name: "grizzly bear", Tame: false, Dangerous: true, DangerReason: "large predator", X: 10, Y: 20}
	if got, want := wildlifeFootnoteLine(dangerous), "grizzly bear (WILD, DANGEROUS: large predator) at (10,20)"; got != want {
		t.Errorf("wildlifeFootnoteLine(dangerous) = %q, want %q", got, want)
	}
	tame := wildlifeListEntry{Name: "cat", Tame: true, Dangerous: false, X: 5, Y: 6}
	if got, want := wildlifeFootnoteLine(tame), "cat (TAME) at (5,6)"; got != want {
		t.Errorf("wildlifeFootnoteLine(tame) = %q, want %q", got, want)
	}
}

// TestWildlifeInView covers why this lens can't reuse gatherMineralsLens's
// no-filter approach: list_wildlife is queried per-z only (like
// list_buildings/list_zones), so unlike Slice.Minerals (pre-windowed by the
// same map_slice call) an animal anywhere on the z-level comes back, and
// only some of them are actually inside the rendered crop.
func TestWildlifeInView(t *testing.T) {
	s := &mapview.Slice{X1: 40, Y1: 40, Rows: []string{"....", "....", "...."}} // 4 wide, 3 tall -> x in [40,43], y in [40,42]
	cases := []struct {
		x, y int16
		want bool
	}{
		{40, 40, true},  // top-left corner, in view
		{43, 42, true},  // bottom-right corner, in view
		{39, 40, false}, // one west of the window
		{44, 40, false}, // one east of the window
		{40, 43, false}, // one south of the window
	}
	for _, tc := range cases {
		if got := wildlifeInView(s, tc.x, tc.y); got != tc.want {
			t.Errorf("wildlifeInView(%d,%d) = %v, want %v", tc.x, tc.y, got, tc.want)
		}
	}
	if wildlifeInView(&mapview.Slice{}, 0, 0) {
		t.Error("wildlifeInView must report false for an empty (no Rows) slice, not panic or false-positive")
	}
	if wildlifeInView(nil, 0, 0) {
		t.Error("wildlifeInView must report false for a nil slice")
	}
}

// TestParseListWildlifeJSON decodes a canned list_wildlife response (the
// shape queries.cpp handleListWildlife actually emits) the same way
// gatherWildlifeLens does, mirroring the buildings/zones lens tests' focus
// on the wire-shape contract rather than a live Bridge round-trip (no
// TCP-mocking seam exists for arbitrary Query() calls -- see Bridge's
// mapSliceFn doc comment).
func TestParseListWildlifeJSON(t *testing.T) {
	raw := []byte(`{"wildlife":[
		{"id":101,"x":30,"y":40,"z":5,"tame":false,"name":"grizzly bear","dangerous":true,"danger_reason":"large predator"},
		{"id":102,"x":31,"y":41,"z":5,"tame":true,"name":"cat","dangerous":false}
	]}`)
	var resp struct {
		Wildlife []wildlifeListEntry `json:"wildlife"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal canned list_wildlife JSON: %v", err)
	}
	if len(resp.Wildlife) != 2 {
		t.Fatalf("expected 2 wildlife entries, got %d", len(resp.Wildlife))
	}
	bear := resp.Wildlife[0]
	if bear.ID != 101 || bear.X != 30 || bear.Y != 40 || bear.Z != 5 || bear.Tame || !bear.Dangerous ||
		bear.Name != "grizzly bear" || bear.DangerReason != "large predator" {
		t.Errorf("grizzly bear entry parsed wrong: %+v", bear)
	}
	cat := resp.Wildlife[1]
	if cat.ID != 102 || !cat.Tame || cat.Dangerous || cat.Name != "cat" || cat.DangerReason != "" {
		t.Errorf("cat entry parsed wrong: %+v", cat)
	}
}

// TestBuildWildlifeOverlay_MarksAndFootnote exercises the full
// parse-paint-footnote pipeline (buildWildlifeOverlay, the part of
// gatherWildlifeLens that runs after the plugin round-trip) against a
// canned list_wildlife payload, checking both the paint marks (dangerous
// vs harmless glyph) and that the footnote only covers the in-view animal.
func TestBuildWildlifeOverlay_MarksAndFootnote(t *testing.T) {
	raw := []byte(`{"wildlife":[
		{"id":1,"x":10,"y":10,"z":5,"tame":false,"name":"wolf","dangerous":true,"danger_reason":"large predator"},
		{"id":2,"x":999,"y":999,"z":5,"tame":true,"name":"cat","dangerous":false}
	]}`)
	s := &mapview.Slice{X1: 0, Y1: 0, Rows: []string{"...........", "...........", "...........", "...........",
		"...........", "...........", "...........", "...........", "...........", "...........", "...........", "..........."}}
	ov, err := buildWildlifeOverlay(raw, s)
	if err != nil {
		t.Fatalf("buildWildlifeOverlay: %v", err)
	}
	if ov.Marks[[2]int16{10, 10}] != 'V' {
		t.Errorf("expected dangerous wolf marked 'V', got %+v", ov.Marks)
	}
	if ov.Marks[[2]int16{999, 999}] != 'v' {
		t.Errorf("expected harmless cat marked 'v', got %+v", ov.Marks)
	}
	if len(ov.Footnotes) != 1 || !strings.Contains(ov.Footnotes[0], "wolf") {
		t.Fatalf("expected exactly one in-view footnote (the wolf), got %+v", ov.Footnotes)
	}
}

// TestBuildWildlifeOverlay_UnparseableJSON covers the truthful-error path:
// a malformed response must surface as an error, never a silently empty
// overlay (this project's ACK/error discipline).
func TestBuildWildlifeOverlay_UnparseableJSON(t *testing.T) {
	if _, err := buildWildlifeOverlay([]byte(`not json`), &mapview.Slice{}); err == nil {
		t.Fatal("expected an error for unparseable list_wildlife JSON, got nil")
	}
}
