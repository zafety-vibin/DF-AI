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

func TestDecodeColumnProfile(t *testing.T) {
	raw := []byte(`{"x":70,"y":80,"levels":[{"z":111,"glyph":",","shape":"floor","material":"grass","hidden":false},{"z":110,"glyph":"?","shape":"hidden","material":"unknown","hidden":true}]}`)
	c, err := DecodeColumnProfile(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(c.Levels) != 2 || c.Levels[1].Z != 110 || !c.Levels[1].Hidden {
		t.Fatalf("levels mismatch: %+v", c.Levels)
	}
}
