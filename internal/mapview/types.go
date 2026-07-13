// Package mapview decodes the plugin's classified map queries and renders
// model-facing views (crops, cross-sections). It is the perception layer:
// the harness owns the map; the model sees small semantic views of it.
package mapview

import (
	"context"
	"encoding/json"
	"fmt"
)

// Legend is the fixed glyph legend appended to every rendered view.
// Keep in sync with classifyTile in dfhack-plugin/queries.cpp.
const Legend = "? hidden(undug fog: diggable!) # stone-wall % soil-wall = mineral-vein " +
	". floor , grass T tree t sapling/shrub _ open-air < up-stair > down-stair " +
	"X up/down-stair ^ ramp ~ water 1-7 water(depth) L magma F fortification"

type Slice struct {
	Z          int16      `json:"z"`
	X1         int16      `json:"x1"`
	Y1         int16      `json:"y1"`
	Rows       []string   `json:"rows"`
	Designated [][2]int16 `json:"designated"`
	// Water tiles as [x,y,depth] in absolute map coords, depth 1-7 (dry
	// tiles omitted). Aquifer tiles as [x,y] — includes hidden tiles by
	// design (DF's damp-dig warnings make aquifers player-knowable).
	// Both are optional: an older plugin simply omits them.
	Water   [][3]int16 `json:"water"`
	Aquifer [][2]int16 `json:"aquifer"`
	// DesignationKinds is per-designated-tile kind detail: [x,y,kind]
	// where kind is 0=dig(Default) 1=channel 2=ramp 3=stair 4=smooth.
	// Optional: an older plugin omits it, leaving Designated (plain
	// [x,y] pairs, no kind) as the only always-on signal.
	DesignationKinds [][3]int16 `json:"designation_kinds"`
}

type ColumnLevel struct {
	Z        int16  `json:"z"`
	Glyph    string `json:"glyph"`
	Shape    string `json:"shape"`
	Material string `json:"material"`
	Hidden   bool   `json:"hidden"`
	// Optional fluid annotations (absent from older plugins = zero values):
	// Water depth 1-7, Aquifer = water_table bit, Damp = a horizontal or
	// above neighbor is wet.
	Water   int  `json:"water"`
	Aquifer bool `json:"aquifer"`
	Damp    bool `json:"damp"`
}

type ColumnProfile struct {
	X      int16         `json:"x"`
	Y      int16         `json:"y"`
	Levels []ColumnLevel `json:"levels"`
}

func DecodeSlice(raw []byte) (*Slice, error) {
	var s Slice
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("map_slice decode: %w (raw: %.120s)", err, raw)
	}
	if len(s.Rows) == 0 {
		return nil, fmt.Errorf("map_slice returned no rows (raw: %.120s)", raw)
	}
	return &s, nil
}

func DecodeColumnProfile(raw []byte) (*ColumnProfile, error) {
	var c ColumnProfile
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("column_profile decode: %w (raw: %.120s)", err, raw)
	}
	return &c, nil
}

// SliceProvider fetches classified map data. Implemented over the dfhack
// client's SendQuery in cmd/df-mcp; faked in tests.
type SliceProvider interface {
	MapSlice(ctx context.Context, x1, y1, z, x2, y2 int16) (*Slice, error)
	ColumnProfile(ctx context.Context, x, y, zTop, zBottom int16) (*ColumnProfile, error)
}
