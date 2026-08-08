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
	// Smoothed tiles as [x,y] in absolute map coords — a tile carries DF's
	// SMOOTH/SMOOTH_DEAD tiletype_special variant, the only signal a
	// completed "smooth mode=wall/floor" job took effect (shape/material
	// don't change). Optional: an older plugin simply omits it.
	Smoothed [][2]int16 `json:"smoothed"`
	// FloorItems as [x,y,count] in absolute map coords — count of loose
	// items at rest on the tile (on_ground, and not yet absorbed into a
	// building or construction). These tiles render as plain clean floor
	// in Rows but can still block a new building placement. Optional: an
	// older plugin simply omits it.
	FloorItems [][3]int16 `json:"floor_items"`
	// StockpileTilesInView / StockpileTilesOccupied count the tiles in
	// THIS window covered by some stockpile's extents, and the subset of
	// those holding at least one loose item. Membership is pure geometry:
	// DF marks no item as "stockpiled", so the plugin resolves it through
	// world->buildings.other.STOCKPILE + Buildings::containsTile.
	//
	// They are POINTERS on purpose, following the stockItem Units lesson:
	// a plain int cannot distinguish "the plugin never measured this"
	// from "it measured zero stockpile tiles here", and those two demand
	// opposite renders — the first must say nothing about stockpiles at
	// all, the second is a real, reportable all-clear. nil means NO DATA;
	// StockpileTilesInView is emitted by every plugin build that
	// understands stockpile coverage, so it doubles as the new-plugin
	// sentinel for every field below.
	StockpileTilesInView   *int `json:"stockpile_tiles_in_view"`
	StockpileTilesOccupied *int `json:"stockpile_tiles_occupied"`
	// FloorItemsNoStock is the subset of FloorItems whose tile is NOT
	// covered by any stockpile — the homeless clutter. Capped plugin-side
	// at 200 entries; FloorItemsNoStockCapped reports the cap was hit, in
	// which case every number derived from this array is a lower bound.
	// Optional: an older plugin omits both.
	FloorItemsNoStock       [][3]int16 `json:"floor_items_nostock"`
	FloorItemsNoStockCapped bool       `json:"floor_items_nostock_capped"`
	// FloorItemClasses is per-item-tile coarse class identity as
	// [x,y,idx] where idx indexes FloorItemClassNames — a per-slice
	// table, exactly like Minerals/MineralNames, so a cluttered quarter
	// doesn't repeat "finished goods" on every tile. The class is the
	// dominant one on that tile (most items; ties break to the lower
	// class id). Reachable only via lens=items. Optional: an older plugin
	// simply omits both fields.
	FloorItemClasses    [][3]int16 `json:"floor_item_classes"`
	FloorItemClassNames []string   `json:"floor_item_class_names"`
	// PendingBuilding tiles as [x,y] in absolute map coords — DF's
	// tile_building_occ::Planned occupancy value, set the instant a
	// building of ANY type (including a wall/floor/ramp Construction) is
	// placed and cleared/promoted on completion or removal. This is the
	// always-on "something is queued here, not yet real" signal — exact
	// type/category still needs lens=buildings. Optional: an older
	// plugin simply omits it.
	PendingBuilding [][2]int16 `json:"pending_building"`
	// Minerals is per-vein-tile identity as [x,y,idx] in absolute map
	// coords, where idx indexes MineralNames — a per-slice table, so a
	// vein-ringed area doesn't repeat the same inorganic name per tile.
	// Base terrain (Rows) already renders these tiles '=' either way;
	// this is only reachable via lens=minerals. Optional: an older plugin
	// simply omits both fields.
	Minerals     [][3]int16 `json:"minerals"`
	MineralNames []string   `json:"mineral_names"`
}

type ColumnLevel struct {
	Z        int16  `json:"z"`
	Glyph    string `json:"glyph"`
	Shape    string `json:"shape"`
	Material string `json:"material"`
	Hidden   bool   `json:"hidden"`
	// Optional fluid annotations (absent from older plugins = zero values):
	// Water depth 1-7, Aquifer = water_table bit, Damp = a horizontal or
	// above neighbor is wet, Smooth = tile carries DF's SMOOTH/SMOOTH_DEAD
	// tiletype_special variant (a completed "smooth mode=wall/floor" job).
	Water   int  `json:"water"`
	Aquifer bool `json:"aquifer"`
	Damp    bool `json:"damp"`
	Smooth  bool `json:"smooth"`
	// FloorItems is the count of loose items at rest on this tile (see
	// Slice.FloorItems). Zero/absent from older plugins.
	FloorItems int `json:"floor_items"`
}

type ColumnProfile struct {
	X      int16         `json:"x"`
	Y      int16         `json:"y"`
	Levels []ColumnLevel `json:"levels"`
}

// SurfaceResult classifies what ColumnSurfaceZ found in a queried z-window.
// A single ok=false was not enough: "the window never left open air" and
// "the window's top was already solid ground" need opposite fixes (widen
// z_bottom downward vs. raise z_top upward) and must not be reported as
// the same inconclusive case.
type SurfaceResult int

const (
	// SurfaceFound: Levels[0] is open air and a solid level exists below
	// it in the window — the column's own surface is confirmed at that Z.
	SurfaceFound SurfaceResult = iota
	// SurfaceAboveWindow: Levels[0] (the window's top) is already solid.
	// The true surface may sit exactly there (with open air just above,
	// outside the window) or the ground may keep rising well above z_top
	// (a mountain, or a deep stone cross-section that never reached the
	// surface) — this function cannot tell those apart from inside the
	// window, so it does not guess. Caller fix: raise z_top.
	SurfaceAboveWindow
	// SurfaceUnknownAllAir: every level in the window is open air; the
	// surface (if any) is below z_bottom. Caller fix: widen z_bottom
	// downward.
	SurfaceUnknownAllAir
)

// ColumnSurfaceZ finds THIS column's own surface within the queried
// z-window: the first solid (non-open-air) level, but ONLY when the
// window's top (Levels[0], top-to-bottom order) is itself open air —
// otherwise there is no way to tell a confirmed surface from solid ground
// that merely happens to sit at the window's top (deep stone picked via an
// explicit z_top/z_bottom window, or terrain rising above the window on a
// hill/mountain). Two live incidents traced back to callers instead using
// one map-wide proxy (highest dwarf Z) stamped onto every column queried,
// which drifts as dwarves walk and is simply wrong for any column whose
// real ground sits at a different Z (a hill, a valley, anywhere off the
// exact dwarf tile); a third traced to this function itself confirming a
// "surface" at a non-air Levels[0], mislabeling solid stone deep
// underground and terrain that rises above the sampled window.
func ColumnSurfaceZ(c *ColumnProfile) (z int16, result SurfaceResult) {
	if c == nil || len(c.Levels) == 0 {
		return 0, SurfaceUnknownAllAir
	}
	if !isOpenAir(c.Levels[0]) {
		return 0, SurfaceAboveWindow
	}
	for _, lv := range c.Levels {
		if isOpenAir(lv) {
			continue
		}
		return lv.Z, SurfaceFound
	}
	return 0, SurfaceUnknownAllAir
}

func isOpenAir(lv ColumnLevel) bool {
	return lv.Shape == "open" || lv.Material == "air"
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
