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
