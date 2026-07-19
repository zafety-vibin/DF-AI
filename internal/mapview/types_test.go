package mapview

import "testing"

func TestDecodeSlice(t *testing.T) {
	raw := []byte(`{"z":110,"x1":70,"y1":80,"rows":["##..","#?.,",",,,_"],"designated":[[71,80]]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Z != 110 || s.X1 != 70 || s.Y1 != 80 {
		t.Fatalf("header mismatch: %+v", s)
	}
	if len(s.Rows) != 3 || s.Rows[1] != "#?.," {
		t.Fatalf("rows mismatch: %+v", s.Rows)
	}
	if len(s.Designated) != 1 || s.Designated[0] != [2]int16{71, 80} {
		t.Fatalf("designated mismatch: %+v", s.Designated)
	}
}

// Water/aquifer fields decode when present; the absent case (older plugin)
// is covered by TestDecodeSlice above — fields stay nil.
func TestDecodeSliceWater(t *testing.T) {
	raw := []byte(`{"z":110,"x1":70,"y1":80,"rows":["##"],"water":[[70,80,3],[71,80,7]],"aquifer":[[71,80]]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s.Water) != 2 || s.Water[0] != [3]int16{70, 80, 3} {
		t.Fatalf("water mismatch: %+v", s.Water)
	}
	if len(s.Aquifer) != 1 || s.Aquifer[0] != [2]int16{71, 80} {
		t.Fatalf("aquifer mismatch: %+v", s.Aquifer)
	}
}

func TestDecodeColumnProfile(t *testing.T) {
	raw := []byte(`{"x":70,"y":80,"levels":[{"z":111,"glyph":",","shape":"floor","material":"grass","hidden":false},{"z":110,"glyph":"?","shape":"hidden","material":"unknown","hidden":true}]}`)
	c, err := DecodeColumnProfile(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(c.Levels) != 2 || c.Levels[1].Z != 110 || !c.Levels[1].Hidden {
		t.Fatalf("levels mismatch: %+v", c.Levels)
	}
	// Fluid fields absent (older plugin) => zero values, no error.
	if c.Levels[0].Water != 0 || c.Levels[0].Aquifer || c.Levels[0].Damp {
		t.Fatalf("absent fluid fields must be zero: %+v", c.Levels[0])
	}
}

func TestDecodeColumnProfileFluids(t *testing.T) {
	raw := []byte(`{"x":70,"y":80,"levels":[{"z":110,"glyph":"~","shape":"floor","material":"water","water":5,"aquifer":true,"damp":true}]}`)
	c, err := DecodeColumnProfile(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	lv := c.Levels[0]
	if lv.Water != 5 || !lv.Aquifer || !lv.Damp {
		t.Fatalf("fluid fields not decoded: %+v", lv)
	}
}

// Smooth decodes when present (a completed "smooth mode=wall/floor" job);
// the absent case (older plugin) is covered by TestDecodeColumnProfile
// above — the field stays false.
func TestDecodeColumnProfileSmooth(t *testing.T) {
	raw := []byte(`{"x":70,"y":80,"levels":[{"z":110,"glyph":"#","shape":"wall","material":"stone","smooth":true}]}`)
	c, err := DecodeColumnProfile(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !c.Levels[0].Smooth {
		t.Fatalf("expected Smooth=true, got %+v", c.Levels[0])
	}
}

// Smoothed decodes when present, mirroring the Water/Aquifer coverage
// above — the absent case (older plugin) is covered by TestDecodeSlice,
// where the field stays nil.
func TestDecodeSliceSmoothed(t *testing.T) {
	raw := []byte(`{"z":110,"x1":70,"y1":80,"rows":["##"],"smoothed":[[70,80]]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s.Smoothed) != 1 || s.Smoothed[0] != [2]int16{70, 80} {
		t.Fatalf("smoothed mismatch: %+v", s.Smoothed)
	}
}

// A player-built Construction wall/floor (e.g. an aquifer seal from the
// build tool) glyphs identically to natural stone (shape=wall,
// material=stone — classifyTileRevealed has no separate case for
// tiletype_material::CONSTRUCTION) and DF stamps the same tiletype_special
// SMOOTH value on it as genuine dwarf-smoothing. The plugin's isSmoothedAt
// guards on material != CONSTRUCTION, so a fixture for such a tile must
// carry no "smooth"/"smoothed" entry at all; these tests pin that the
// decode side treats that absence as false, not as an accidental true.
func TestDecodeColumnProfileConstructedWallNotSmoothed(t *testing.T) {
	raw := []byte(`{"x":70,"y":80,"levels":[{"z":110,"glyph":"#","shape":"wall","material":"stone","hidden":false}]}`)
	c, err := DecodeColumnProfile(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if c.Levels[0].Smooth {
		t.Fatalf("built Construction wall must not decode as smoothed: %+v", c.Levels[0])
	}
}

func TestDecodeSliceConstructedWallNotSmoothed(t *testing.T) {
	raw := []byte(`{"z":110,"x1":70,"y1":80,"rows":["##"],"smoothed":[]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, xy := range s.Smoothed {
		if xy == [2]int16{70, 80} {
			t.Fatalf("built Construction tile must not appear in smoothed list: %+v", s.Smoothed)
		}
	}
}

// FloorItems decodes when present, mirroring the Smoothed coverage above —
// the absent case (older plugin) is covered by TestDecodeSlice, where the
// field stays nil.
func TestDecodeSliceFloorItems(t *testing.T) {
	raw := []byte(`{"z":110,"x1":70,"y1":80,"rows":["##"],"floor_items":[[70,80,2]]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s.FloorItems) != 1 || s.FloorItems[0] != [3]int16{70, 80, 2} {
		t.Fatalf("floor_items mismatch: %+v", s.FloorItems)
	}
}

// FloorItems decodes when present (a loose item at rest on this tile); the
// absent case (older plugin) is covered by TestDecodeColumnProfile above —
// the field stays 0.
func TestDecodeColumnProfileFloorItems(t *testing.T) {
	raw := []byte(`{"x":70,"y":80,"levels":[{"z":110,"glyph":".","shape":"floor","material":"rock_or_soil","floor_items":3}]}`)
	c, err := DecodeColumnProfile(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if c.Levels[0].FloorItems != 3 {
		t.Fatalf("expected FloorItems=3, got %+v", c.Levels[0])
	}
}

// PendingBuilding decodes when present, mirroring the Smoothed/FloorItems
// coverage above — the absent case (older plugin) is covered by
// TestDecodeSlice, where the field stays nil.
func TestDecodeSlicePendingBuilding(t *testing.T) {
	raw := []byte(`{"z":110,"x1":70,"y1":80,"rows":["##"],"pending_building":[[70,80]]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s.PendingBuilding) != 1 || s.PendingBuilding[0] != [2]int16{70, 80} {
		t.Fatalf("pending_building mismatch: %+v", s.PendingBuilding)
	}
}

// An older plugin simply omits pending_building — the field must decode to
// a nil slice, no error, mirroring TestDecodeSlice's coverage of the other
// optional fields.
func TestDecodeSlicePendingBuildingAbsentIsFine(t *testing.T) {
	raw := []byte(`{"z":110,"x1":70,"y1":80,"rows":["##"]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("an older-plugin payload without pending_building must still decode: %v", err)
	}
	if len(s.PendingBuilding) != 0 {
		t.Fatalf("expected empty PendingBuilding, got %v", s.PendingBuilding)
	}
}

func TestDecodeSlice_DesignationKindsOptional(t *testing.T) {
	raw := []byte(`{"z":100,"x1":0,"y1":0,"rows":["..","d."],"designated":[[0,1]],"designation_kinds":[[0,1,2]]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s.DesignationKinds) != 1 || s.DesignationKinds[0] != [3]int16{0, 1, 2} {
		t.Fatalf("expected DesignationKinds [[0,1,2]], got %v", s.DesignationKinds)
	}
}

// Minerals/MineralNames decode when present — a per-tile index into a
// per-slice name table, mirroring the Smoothed/FloorItems coverage above.
// Two distinct minerals confirm the index (not just presence) round-trips.
func TestDecodeSliceMinerals(t *testing.T) {
	raw := []byte(`{"z":110,"x1":70,"y1":80,"rows":["=="],"minerals":[[70,80,0],[71,80,1]],"mineral_names":["limonite","native copper"]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s.Minerals) != 2 || s.Minerals[0] != [3]int16{70, 80, 0} || s.Minerals[1] != [3]int16{71, 80, 1} {
		t.Fatalf("minerals mismatch: %+v", s.Minerals)
	}
	if len(s.MineralNames) != 2 || s.MineralNames[0] != "limonite" || s.MineralNames[1] != "native copper" {
		t.Fatalf("mineral_names mismatch: %+v", s.MineralNames)
	}
}

// An older plugin simply omits minerals/mineral_names — both fields must
// decode to nil/empty, no error, mirroring TestDecodeSlicePendingBuildingAbsentIsFine.
func TestDecodeSliceMineralsAbsentIsFine(t *testing.T) {
	raw := []byte(`{"z":110,"x1":70,"y1":80,"rows":["##"]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("an older-plugin payload without minerals must still decode: %v", err)
	}
	if len(s.Minerals) != 0 || len(s.MineralNames) != 0 {
		t.Fatalf("expected empty Minerals/MineralNames, got %+v / %+v", s.Minerals, s.MineralNames)
	}
}

func TestDecodeSlice_DesignationKindsAbsentIsFine(t *testing.T) {
	raw := []byte(`{"z":100,"x1":0,"y1":0,"rows":["..","d."],"designated":[[0,1]]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("an older-plugin payload without designation_kinds must still decode: %v", err)
	}
	if len(s.DesignationKinds) != 0 {
		t.Fatalf("expected empty DesignationKinds, got %v", s.DesignationKinds)
	}
}
