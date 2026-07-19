package mcpserver

import (
	"context"
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
		"TradeDepot": 'O', "Wagon": 'O',
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
// minerals can't just use a-z in order: 't' (sapling/shrub) and 'u'
// (pending-building) are already reserved base-terrain/always-on glyphs —
// TestLensGlyphsDisjointFromBaseSet enforces this for every lens, but this
// test names the specific letters so a future edit that reintroduces them
// fails with a direct message instead of a generic disjointness error.
func TestMineralLetterAlphabet_SkipsReservedGlyphs(t *testing.T) {
	if len(mineralLetterAlphabet) != 24 {
		t.Fatalf("expected 24 letters (26 minus t/u), got %d: %v", len(mineralLetterAlphabet), mineralLetterAlphabet)
	}
	for _, r := range mineralLetterAlphabet {
		if r == 't' || r == 'u' {
			t.Fatalf("mineralLetterAlphabet must not contain reserved glyph %q", string(r))
		}
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
		"Pen": 'A', "Pond": 'A', "AnimalTraining": 'A', "ArcheryRange": 'A',
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
