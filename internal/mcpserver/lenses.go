package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/df-ai/orchestrator/internal/mapview"
)

// LensDef is one registered look overlay. Adding a future lens (zones,
// traffic, burrows — see design doc "Extensibility test") is exactly one
// LensDef entry plus one enum literal on the look tool's input schema —
// zero changes to RenderCrop or the paint pipeline.
type LensDef struct {
	Name   string
	Legend string
	Gather func(ctx context.Context, b *Bridge, s *mapview.Slice, z int16) (mapview.Overlay, error)
}

var lenses = map[string]LensDef{
	"buildings": {
		Name:   "buildings",
		Legend: "lens=buildings: W workshop/furnace B furniture D door/hatch S stockpile M mechanism/well/bridge P trap/cage C construction O other — UPPERCASE built, lowercase planned/in-progress; exact type: buildings tool",
		Gather: gatherBuildingsLens,
	},
	"designations": {
		Name:   "designations",
		Legend: "lens=designations: d dig c channel r ramp s stair m smooth — designated, not yet dug",
		Gather: gatherDesignationsLens,
	},
	"zones": {
		Name:   "zones",
		Legend: "lens=zones: H housing (bedroom/office/tomb/dining/meeting/dormitory) K barracks A animal (pen/pond/training/archery) R resource-gather (water/dump/sand/fishing/clay/plants) J dungeon — exact owner/roster: list_zones",
		Gather: gatherZonesLens,
	},
	"minerals": {
		Name:   "minerals",
		Legend: "lens=minerals: vein tiles painted a,b,c... (skipping d/t/u, reserved elsewhere) keyed to THIS view's mineral names, listed in a footnote below",
		Gather: gatherMineralsLens,
	},
}

// lensNames returns the registered lens names, sorted, for error messages.
func lensNames() []string {
	names := make([]string, 0, len(lenses))
	for n := range lenses {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// lensGlyphSet returns every glyph a lens can emit — used by the
// disjointness test and documented here as the single source of truth
// for what each lens is allowed to paint.
func lensGlyphSet(name string) []rune {
	switch name {
	case "buildings":
		return []rune{'W', 'B', 'D', 'S', 'M', 'P', 'C', 'O',
			'w', 'b', 'd', 's', 'm', 'p', 'c', 'o'} // lowercase = planned
	case "designations":
		return []rune{'d', 'c', 'r', 's', 'm'}
	case "zones":
		return []rune{'H', 'K', 'A', 'R', 'J'}
	case "minerals":
		return mineralLetterAlphabet
	default:
		return nil
	}
}

// mineralLetterAlphabet is a-z minus 'd' (the designations lens's always-on
// dig-designation glyph — RenderCrop paints s.Designated as 'd' on every
// crop regardless of lens, so a vein tile that also happens to be
// designated for digging would otherwise have its minerals-lens letter
// silently overwrite that dig indicator, or vice versa depending on paint
// order — a live collision confirmed on a fort tour), 't' (sapling/shrub,
// base terrain glyph), and 'u' (pending-building, an always-on overlay) —
// the three lowercase letters already reserved outside any lens. 23
// letters remain for per-view mineral identities; a view with more
// distinct minerals than that just stops labeling beyond the cap (see
// gatherMineralsLens).
var mineralLetterAlphabet = func() []rune {
	var out []rune
	for c := 'a'; c <= 'z'; c++ {
		if c == 'd' || c == 't' || c == 'u' {
			continue
		}
		out = append(out, c)
	}
	return out
}()

// buildingCategoryGlyph maps a DFHack building_type enum key name (as
// returned by list_buildings' "type" field) to one of 8 category
// glyphs. Every df::building_type value from the DFHack 53.15-r1
// checkout (library/xml/df.d_basics.xml) is covered; Civzone is
// deliberately excluded (that's the future zones lens, a different data
// plane) and NONE never appears in a real building list.
func buildingCategoryGlyph(buildingType string) rune {
	switch buildingType {
	case "Workshop", "Furnace":
		return 'W'
	case "Bed", "Chair", "Table", "Cabinet", "Box", "Coffin", "Statue",
		"Weaponrack", "Armorstand", "TractionBench", "Slab", "Bookcase",
		"DisplayFurniture", "Instrument", "NestBox", "Hive", "Nest":
		return 'B'
	case "Door", "Hatch", "Floodgate", "GrateWall", "GrateFloor",
		"BarsVertical", "BarsFloor", "WindowGlass", "WindowGem":
		return 'D'
	case "Stockpile":
		return 'S'
	case "ScrewPump", "GearAssembly", "AxleHorizontal", "AxleVertical",
		"WaterWheel", "Windmill", "Rollers", "Well", "Bridge", "Support", "Chain":
		return 'M'
	case "Trap", "AnimalTrap", "Cage", "SiegeEngine", "ArcheryTarget":
		return 'P'
	case "Construction":
		return 'C'
	case "FarmPlot": // reserved: cannot exist in a fort today (no farm-plot build type)
		return 'G'
	default: // TradeDepot, Shop, Wagon, RoadDirt, RoadPaved, Weapon, and any future/unmapped type
		return 'O'
	}
}

// buildingOverlayGlyph applies the built/planned case encoding on top of
// the category glyph — the mechanism that makes a silently-dying
// building plan visible: a lowercase glyph that never becomes uppercase
// and then vanishes from the lens IS the death announcement DF never
// gives.
func buildingOverlayGlyph(e buildingListEntry) rune {
	g := buildingCategoryGlyph(e.Type)
	if e.Done {
		return g
	}
	return []rune(string(g))[0] + ('a' - 'A') // uppercase -> lowercase
}

func gatherBuildingsLens(ctx context.Context, b *Bridge, s *mapview.Slice, z int16) (mapview.Overlay, error) {
	args := fmt.Sprintf(`{"z":%d}`, z)
	raw, err := b.Query(ctx, "list_buildings", args)
	if err != nil {
		return mapview.Overlay{}, err
	}
	var resp struct {
		Buildings []buildingListEntry `json:"buildings"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return mapview.Overlay{}, fmt.Errorf("buildings lens: unparseable list_buildings response: %w", err)
	}
	marks := map[[2]int16]rune{}
	for _, e := range resp.Buildings {
		g := buildingOverlayGlyph(e)
		if e.X1 != 0 || e.Y1 != 0 || e.X2 != 0 || e.Y2 != 0 {
			for x := e.X1; x <= e.X2; x++ {
				for y := e.Y1; y <= e.Y2; y++ {
					marks[[2]int16{int16(x), int16(y)}] = g
				}
			}
		} else {
			// Older plugin (pre this task) omits extents — fall back to
			// center-only paint per the design doc's stated fallback.
			marks[[2]int16{int16(e.X), int16(e.Y)}] = g
		}
	}
	return mapview.Overlay{Marks: marks}, nil
}

func designationKindGlyph(kind int16) rune {
	switch kind {
	case 1:
		return 'c'
	case 2:
		return 'r'
	case 3:
		return 's'
	case 4:
		return 'm'
	default:
		return 'd'
	}
}

func gatherDesignationsLens(ctx context.Context, b *Bridge, s *mapview.Slice, z int16) (mapview.Overlay, error) {
	marks := map[[2]int16]rune{}
	for _, dk := range s.DesignationKinds {
		marks[[2]int16{dk[0], dk[1]}] = designationKindGlyph(dk[2])
	}
	return mapview.Overlay{Marks: marks}, nil
}

// dwarfCollisionFootnote reports which dwarves a lens overlay would
// silently hide, per the design doc's "never silently hide a dwarf"
// requirement (Component 2: "A lens overpainting a dwarf appends a
// footnote rather than silently hiding it"). Returns nil when there's
// nothing worth reporting — no dwarves in view at all, or dwarves in
// view but none of them collide with the lens's marks.
func dwarfCollisionFootnote(dwarfMarks map[[2]int16]rune, lensMarks map[[2]int16]rune) []string {
	if len(dwarfMarks) == 0 {
		return nil
	}
	var collided [][2]int16
	for pos := range dwarfMarks {
		if _, hit := lensMarks[pos]; hit {
			collided = append(collided, pos)
		}
	}
	if len(collided) == 0 {
		return nil
	}
	// deterministic order for stable test assertions and readable output
	sort.Slice(collided, func(i, j int) bool {
		if collided[i][0] != collided[j][0] {
			return collided[i][0] < collided[j][0]
		}
		return collided[i][1] < collided[j][1]
	})
	var sb strings.Builder
	fmt.Fprintf(&sb, "dwarves in view: %d (%d under overlay at", len(dwarfMarks), len(collided))
	for _, pos := range collided {
		fmt.Fprintf(&sb, " (%d,%d)", pos[0], pos[1])
	}
	sb.WriteString(")")
	return []string{sb.String()}
}

// unknownLensError renders the truthful-error text for an unrecognized
// lens name, listing the registered names (never a bare failure — this
// project's ACK/error discipline).
func unknownLensError(name string) string {
	return "unknown lens " + strconv.Quote(name) + " — registered lenses: " + fmt.Sprint(lensNames())
}

// zoneCategoryGlyph maps a zone type name (list_zones' "type_name" field,
// which comes straight from DFHack's own civzone_type enum key strings)
// to one of 5 category glyphs -- a handful of glyphs, not one per
// civzone_type, per the project's house rule against per-type ASCII
// budget blowout (the same philosophy buildingCategoryGlyph already
// applies to the 52 real building_type values).
func zoneCategoryGlyph(typeName string) rune {
	switch typeName {
	case "Bedroom", "Office", "Tomb", "DiningHall", "MeetingHall", "Dormitory":
		return 'H' // housing/social
	case "Barracks":
		return 'K'
	case "Pen", "Pond", "AnimalTraining", "ArcheryRange":
		return 'A' // animal/training
	case "WaterSource", "Dump", "SandCollection", "FishingArea", "ClayCollection", "PlantGathering":
		return 'R' // resource-gathering
	case "Dungeon":
		return 'J'
	default:
		return 'H'
	}
}

// gatherMineralsLens paints vein tiles with a-z (minus t/u — see
// mineralLetterAlphabet) keyed to THIS slice's own MineralNames table
// (queries.cpp's queryMapSlice builds it fresh per call, not a fixed
// per-fort catalog — a different look call over different terrain gets a
// different letter->name mapping). The legend line itself must therefore
// be dynamic, unlike buildings/zones/designations' fixed category text —
// carried as a Footnote (appended after the static legend by RenderCrop)
// rather than LensDef.Legend, which is a compile-time constant.
func gatherMineralsLens(ctx context.Context, b *Bridge, s *mapview.Slice, z int16) (mapview.Overlay, error) {
	marks := map[[2]int16]rune{}
	if len(s.MineralNames) == 0 {
		return mapview.Overlay{Marks: marks}, nil
	}
	for _, m := range s.Minerals {
		idx := int(m[2])
		if idx < 0 || idx >= len(mineralLetterAlphabet) {
			continue // beyond the lettered cap — tile stays plain '=' in the grid
		}
		marks[[2]int16{m[0], m[1]}] = mineralLetterAlphabet[idx]
	}
	var legend strings.Builder
	legend.WriteString("this view's minerals:")
	shown := len(s.MineralNames)
	if shown > len(mineralLetterAlphabet) {
		shown = len(mineralLetterAlphabet)
	}
	for i := 0; i < shown; i++ {
		fmt.Fprintf(&legend, " %c=%s", mineralLetterAlphabet[i], s.MineralNames[i])
	}
	if len(s.MineralNames) > shown {
		fmt.Fprintf(&legend, " (+%d more mineral(s) in view, unlabeled)", len(s.MineralNames)-shown)
	}
	return mapview.Overlay{Marks: marks, Footnotes: []string{legend.String()}}, nil
}

func gatherZonesLens(ctx context.Context, b *Bridge, s *mapview.Slice, z int16) (mapview.Overlay, error) {
	args := fmt.Sprintf(`{"z":%d}`, z)
	raw, err := b.Query(ctx, "list_zones", args)
	if err != nil {
		return mapview.Overlay{}, err
	}
	var resp struct {
		Zones []zoneListEntry `json:"zones"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return mapview.Overlay{}, fmt.Errorf("zones lens: unparseable list_zones response: %w", err)
	}
	marks := map[[2]int16]rune{}
	for _, zn := range resp.Zones {
		g := zoneCategoryGlyph(zn.TypeName)
		for x := zn.X1; x <= zn.X2; x++ {
			for y := zn.Y1; y <= zn.Y2; y++ {
				marks[[2]int16{int16(x), int16(y)}] = g
			}
		}
	}
	return mapview.Overlay{Marks: marks}, nil
}
