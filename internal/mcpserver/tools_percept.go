package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/modifications"
)

// fullFidelityMaxDim is the render-budget cutoff shared by look's
// elevation and fort scopes: a stitched region rendered at <=1 tok/tile
// (RenderCrop) stays full fidelity up to this size on both axes; beyond it,
// the region is downsampled block-by-block (mapview.RenderDownsampledSlice)
// with the block size disclosed in the header, matching RenderOverview's
// own disclosure contract. One policy, two entry points.
const fullFidelityMaxDim = 100

// blockFetchMax mirrors the plugin's map_slice region cap (queries.cpp:
// "region too large (max 48x48)") — the largest single map_slice query.
const blockFetchMax = 48

// fetchStitchedRegion fetches a rectangular region [x0,x1] x [y0,y1] (both
// inclusive, absolute map coords) at z as a grid of <=48-wide/tall
// map_slice blocks and stitches them into one Slice via
// mapview.StitchSlices. Shared by elevation (whole map) and fort
// (modified-footprint bbox) scope.
func fetchStitchedRegion(ctx context.Context, b *Bridge, z, x0, y0, x1, y1 int16) (*mapview.Slice, error) {
	var grid [][]*mapview.Slice
	for by := y0; by <= y1; by += blockFetchMax {
		byEnd := by + blockFetchMax - 1
		if byEnd > y1 {
			byEnd = y1
		}
		var row []*mapview.Slice
		for bx := x0; bx <= x1; bx += blockFetchMax {
			bxEnd := bx + blockFetchMax - 1
			if bxEnd > x1 {
				bxEnd = x1
			}
			s, err := b.MapSlice(ctx, bx, by, z, bxEnd, byEnd)
			if err != nil {
				return nil, fmt.Errorf("block (%d,%d)-(%d,%d): %w", bx, by, bxEnd, byEnd, err)
			}
			row = append(row, s)
		}
		grid = append(grid, row)
	}
	return mapview.StitchSlices(grid)
}

// renderFullOrDownsampled applies the shared full-fidelity/downsample
// policy to an already-stitched Slice: full RenderCrop rendering (~1
// tok/tile) when both dimensions fit within fullFidelityMaxDim, otherwise
// a block-downsampled render with the block size disclosed in the header.
func renderFullOrDownsampled(s *mapview.Slice, header string) string {
	w, h := 0, len(s.Rows)
	if h > 0 {
		w = len(s.Rows[0])
	}
	if w <= fullFidelityMaxDim && h <= fullFidelityMaxDim {
		return header + " — full fidelity\n" + mapview.RenderCrop(s, nil, "")
	}
	block := mapview.DownsampleBlockSizeFor(w, h, fullFidelityMaxDim)
	return header + " — downsampled (exceeds full-fidelity budget)\n" + mapview.RenderDownsampledSlice(s, block)
}

// fortFootprintBBox computes the render bbox for scope=fort: the bounding
// box of the player-attributed modifications recorded at z, expanded by
// margin tiles on each side and clamped to the map bounds. Entries whose
// classification is ModificationUnknown never define the footprint — only
// classified player work (dug/channeled/built) does. ok=false means no such
// modifications were recorded at that z — callers must return a truthful
// "nothing here yet" message rather than silently falling back to some
// other crop.
func fortFootprintBBox(mods *modifications.ModificationOverlay, mapW, mapH uint16, z int16, margin int16) (x0, y0, x1, y1 int16, ok bool) {
	region := modifications.Region{
		XMin: 0, XMax: int16(mapW) - 1,
		YMin: 0, YMax: int16(mapH) - 1,
		ZMin: z, ZMax: z,
	}
	hits := mods.GetModificationsInRegion(region, time.Time{})
	xMin, xMax := int16(32767), int16(-32768)
	yMin, yMax := int16(32767), int16(-32768)
	found := false
	for coord, info := range hits {
		if info.Type == modifications.ModificationUnknown {
			continue
		}
		found = true
		if coord.X < xMin {
			xMin = coord.X
		}
		if coord.X > xMax {
			xMax = coord.X
		}
		if coord.Y < yMin {
			yMin = coord.Y
		}
		if coord.Y > yMax {
			yMax = coord.Y
		}
	}
	if !found {
		return 0, 0, 0, 0, false
	}
	xMin -= margin
	yMin -= margin
	xMax += margin
	yMax += margin
	if xMin < 0 {
		xMin = 0
	}
	if yMin < 0 {
		yMin = 0
	}
	if xMax > int16(mapW)-1 {
		xMax = int16(mapW) - 1
	}
	if yMax > int16(mapH)-1 {
		yMax = int16(mapH) - 1
	}
	return xMin, yMin, xMax, yMax, true
}

// fortFootprintMargin is the padding added around the modified-tile bbox
// for scope=fort — enough to see the room's threshold/approach, not the
// whole map.
const fortFootprintMargin = 8

// clampLookRadius applies look's radius default/cap: 0 (or unset) becomes
// the 12-tile default; anything above 23 is clamped down. 23 is the largest
// radius whose 47x47 crop still fits under the plugin's 48x48 map_slice
// region cap (queries.cpp) — going wider needs a plugin change, not just a
// client-side raise.
func clampLookRadius(r int) int {
	if r <= 0 {
		return 12
	}
	if r > 23 {
		return 23
	}
	return r
}

func registerPerceptTools(srv *mcp.Server, b *Bridge) {
	type lookIn struct {
		X      int    `json:"x" jsonschema:"center x"`
		Y      int    `json:"y" jsonschema:"center y"`
		Z      int    `json:"z" jsonschema:"z-level to view"`
		Radius int    `json:"radius,omitempty" jsonschema:"radius 12 (25x25) default; up to 23 (47x47, ~2.3k tokens). For whole-fort orientation use scope=overview instead."`
		Lens   string `json:"lens,omitempty" jsonschema:"optional overlay: buildings|designations|minerals. Paints one annotation layer onto the terrain grid; omit for the plain terrain view (which still shows dig designations as 'd' and queued-but-unfinished buildings as 'u'). Exact building/zone types via buildings/building_status."`
		Scope  string `json:"scope,omitempty" jsonschema:"Cost table (tokens ~= 1.05*W*H + 80): local (default, ignores z-window scoping) terrain crop around (x,y) at radius, up to 47x47 ~2.3k tokens. overview: whole-map downsampled orientation (ignores x/y/radius/lens), ~1-1.5k tokens on any map size, no plugin round-trip. elevation: the FULL z-level (ignores x/y/radius/lens) at full ~1 tok/tile fidelity when the map is <=100 wide/tall (e.g. 96x96 ~9.6k tokens); auto-downsamples into majority-vote blocks above that, with the block size disclosed in the header. fort: the bounding box of tiles you've actually modified at z (dug/built/smoothed) plus an 8-tile margin, same fidelity/downsample rule as elevation but scoped to your footprint instead of the whole map — the recommended planning view once you have dug something."`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "look",
		Description: "Render an annotated map view of one z-level. Glyph grid with legend; dwarves marked @, dig designations marked d, queued-but-unfinished buildings (any type, including a wall/floor Construction) marked u. Pass lens=buildings or lens=designations for detail overlays. '?' tiles are hidden fog — solid undug ground you CAN designate digging into. Default scope=local is a small crop around (x,y); use find_dig_site for choosing dig locations. scope=overview/elevation/fort give whole-map or whole-footprint views instead — see the scope parameter for the cost/fidelity tradeoffs of each.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in lookIn) (*mcp.CallToolResult, any, error) {
		switch in.Scope {
		case "", "local":
			// falls through to the default crop below
		case "overview":
			topo := b.Topo()
			if topo == nil {
				return withDash(b, ctx, "overview unavailable: topology not built yet (waiting for full state)"), nil, nil
			}
			var dwarfXY [][2]int16
			for _, dw := range b.Snapshot().Entities.Dwarves {
				// See the local-scope marks loop below: a dead dwarf's
				// frozen last-known position isn't a live actor to show.
				if dw.Dead {
					continue
				}
				if dw.Z == int16(in.Z) {
					dwarfXY = append(dwarfXY, [2]int16{dw.X, dw.Y})
				}
			}
			return withDash(b, ctx, mapview.RenderOverview(topo, int16(in.Z), dwarfXY)), nil, nil
		case "elevation":
			topo := b.Topo()
			if topo == nil {
				return withDash(b, ctx, "elevation unavailable: topology not built yet (waiting for full state)"), nil, nil
			}
			w, h, d := topo.GetDimensions()
			if in.Z < 0 || in.Z >= int(d) {
				return withDash(b, ctx, fmt.Sprintf("z=%d out of range: map has %d levels (valid z is 0..%d)", in.Z, d, int(d)-1)), nil, nil
			}
			s, err := fetchStitchedRegion(ctx, b, int16(in.Z), 0, 0, int16(w)-1, int16(h)-1)
			if err != nil {
				return withDash(b, ctx, "elevation failed: "+err.Error()), nil, nil
			}
			header := fmt.Sprintf("elevation view (full z-level) at z=%d, map %dx%d", in.Z, w, h)
			return withDash(b, ctx, renderFullOrDownsampled(s, header)), nil, nil
		case "fort":
			topo := b.Topo()
			if topo == nil {
				return withDash(b, ctx, "fort view unavailable: topology not built yet (waiting for full state)"), nil, nil
			}
			mods := b.Mods()
			if mods == nil {
				return withDash(b, ctx, "fort view unavailable: modifications overlay not built yet (waiting for full state)"), nil, nil
			}
			w, h, d := topo.GetDimensions()
			if in.Z < 0 || in.Z >= int(d) {
				return withDash(b, ctx, fmt.Sprintf("z=%d out of range: map has %d levels (valid z is 0..%d)", in.Z, d, int(d)-1)), nil, nil
			}
			x0, y0, x1, y1, ok := fortFootprintBBox(mods, w, h, int16(in.Z), fortFootprintMargin)
			if !ok {
				return withDash(b, ctx, fmt.Sprintf("no modifications recorded on z=%d — nothing dug/built at this level yet; try scope=overview or a z you've worked", in.Z)), nil, nil
			}
			s, err := fetchStitchedRegion(ctx, b, int16(in.Z), x0, y0, x1, y1)
			if err != nil {
				return withDash(b, ctx, "fort view failed: "+err.Error()), nil, nil
			}
			header := fmt.Sprintf("fort view (modified footprint + %d-tile margin) at z=%d: bbox (%d,%d)-(%d,%d)", fortFootprintMargin, in.Z, x0, y0, x1, y1)
			return withDash(b, ctx, renderFullOrDownsampled(s, header)), nil, nil
		default:
			return withDash(b, ctx, fmt.Sprintf("unknown scope %q — use local, overview, elevation, or fort", in.Scope)), nil, nil
		}
		r := clampLookRadius(in.Radius)
		x1, y1 := int16(in.X-r), int16(in.Y-r)
		x2, y2 := int16(in.X+r), int16(in.Y+r)
		if x1 < 0 {
			x1 = 0
		}
		if y1 < 0 {
			y1 = 0
		}
		s, err := b.MapSlice(ctx, x1, y1, int16(in.Z), x2, y2)
		if err != nil {
			return withDash(b, ctx, "look failed: "+err.Error()), nil, nil
		}
		marks := map[[2]int16]rune{}
		for _, d := range b.Snapshot().Entities.Dwarves {
			// A dead dwarf's df::unit lingers in the source list at a frozen
			// last-known position (see EntityInfo.Dead's doc comment) --
			// painting it as '@' put a corpse on the map like a live actor
			// to path around, sometimes for a week of game time after
			// burial. Omit it entirely; dwarves/dwarf_detail already
			// surface "last-known (dead)" position and burial state.
			if d.Dead {
				continue
			}
			if d.Z == int16(in.Z) {
				marks[[2]int16{d.X, d.Y}] = '@'
			}
		}
		overlays := []mapview.Overlay{{Marks: marks}}
		legendExtra := ""
		if in.Lens != "" {
			def, ok := lenses[in.Lens]
			if !ok {
				return withDash(b, ctx, unknownLensError(in.Lens)), nil, nil
			}
			lensOverlay, err := def.Gather(ctx, b, s, int16(in.Z))
			if err != nil {
				return withDash(b, ctx, "lens failed: "+err.Error()), nil, nil
			}
			if fn := dwarfCollisionFootnote(marks, lensOverlay.Marks); fn != nil {
				lensOverlay.Footnotes = append(lensOverlay.Footnotes, fn...)
			}
			overlays = append(overlays, lensOverlay)
			legendExtra = def.Legend
		}
		return withDash(b, ctx, mapview.RenderCrop(s, overlays, legendExtra)), nil, nil
	})

	type xsecIn struct {
		X       int `json:"x" jsonschema:"column x"`
		Y       int `json:"y" jsonschema:"column y"`
		ZTop    int `json:"z_top,omitempty" jsonschema:"top z (default: 3 above the highest dwarf)"`
		ZBottom int `json:"z_bottom,omitempty" jsonschema:"bottom z (default: 20 below the lowest dwarf)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "cross_section",
		Description: "Vertical slice at column (x,y): one line per z-level showing surface, soil bands, stone, and hidden layers. THE tool for judging how deep to dig stairs and where soil ends.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in xsecIn) (*mcp.CallToolResult, any, error) {
		snap := b.Snapshot()
		minDZ, maxDZ := int16(32767), int16(-32768)
		for _, d := range snap.Entities.Dwarves {
			if d.Z < minDZ {
				minDZ = d.Z
			}
			if d.Z > maxDZ {
				maxDZ = d.Z
			}
		}
		zt, zb := int16(in.ZTop), int16(in.ZBottom)
		if in.ZTop == 0 && len(snap.Entities.Dwarves) > 0 {
			zt = maxDZ + 3
		}
		if in.ZBottom == 0 && len(snap.Entities.Dwarves) > 0 {
			zb = minDZ - 20
		}
		if zb < 0 {
			zb = 0
		}
		c, err := b.ColumnProfile(ctx, int16(in.X), int16(in.Y), zt, zb)
		if err != nil {
			return withDash(b, ctx, "cross_section failed: "+err.Error()), nil, nil
		}
		// RenderColumn now computes THIS column's own surface (ColumnSurfaceZ)
		// instead of trusting a map-wide proxy passed in from here — that
		// proxy (highest dwarf Z) was live-observed drifting between calls
		// as dwarves walked uphill, and was wrong for any column whose real
		// ground isn't at the exact dwarf tile.
		return withDash(b, ctx, mapview.RenderColumn(c)), nil, nil
	})

	type elevationIn struct {
		Axis    string `json:"axis" jsonschema:"x|y — sweep across x at fixed y, or across y at fixed x"`
		X       int    `json:"x,omitempty" jsonschema:"required when axis=y: the fixed x"`
		Y       int    `json:"y,omitempty" jsonschema:"required when axis=x: the fixed y"`
		X1      int    `json:"x1,omitempty" jsonschema:"required when axis=x: sweep start x"`
		X2      int    `json:"x2,omitempty" jsonschema:"required when axis=x: sweep end x"`
		Y1      int    `json:"y1,omitempty" jsonschema:"required when axis=y: sweep start y"`
		Y2      int    `json:"y2,omitempty" jsonschema:"required when axis=y: sweep end y"`
		ZTop    int    `json:"z_top" jsonschema:"top z"`
		ZBottom int    `json:"z_bottom" jsonschema:"bottom z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "elevation_view",
		Description: "Vertical slice along a LINE (not a single column like cross_section) — the third orthogonal plane. axis=x sweeps across x at a fixed y; axis=y sweeps across y at a fixed x. Bounded to 30 columns per call (matches look's radius cap) to keep the underlying column_profile calls bounded.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in elevationIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		const maxSweep = 30
		var coords []int16
		switch in.Axis {
		case "x":
			if in.X2 < in.X1 {
				return withDash(b, ctx, "axis=x requires x1<=x2"), nil, nil
			}
			if in.X2-in.X1+1 > maxSweep {
				return withDash(b, ctx, fmt.Sprintf("sweep too wide: %d columns, max %d", in.X2-in.X1+1, maxSweep)), nil, nil
			}
			for x := in.X1; x <= in.X2; x++ {
				coords = append(coords, int16(x))
			}
		case "y":
			if in.Y2 < in.Y1 {
				return withDash(b, ctx, "axis=y requires y1<=y2"), nil, nil
			}
			if in.Y2-in.Y1+1 > maxSweep {
				return withDash(b, ctx, fmt.Sprintf("sweep too wide: %d columns, max %d", in.Y2-in.Y1+1, maxSweep)), nil, nil
			}
			for y := in.Y1; y <= in.Y2; y++ {
				coords = append(coords, int16(y))
			}
		default:
			return withDash(b, ctx, fmt.Sprintf("unknown axis %q — use x or y", in.Axis)), nil, nil
		}

		var columns []*mapview.ColumnProfile
		for _, coord := range coords {
			var c *mapview.ColumnProfile
			var err error
			if in.Axis == "x" {
				c, err = b.ColumnProfile(ctx, coord, int16(in.Y), int16(in.ZTop), int16(in.ZBottom))
			} else {
				c, err = b.ColumnProfile(ctx, int16(in.X), coord, int16(in.ZTop), int16(in.ZBottom))
			}
			if err != nil {
				return withDash(b, ctx, fmt.Sprintf("elevation_view failed at column %d: %v", coord, err)), nil, nil
			}
			columns = append(columns, c)
		}
		fixed := int16(in.Y)
		if in.Axis == "y" {
			fixed = int16(in.X)
		}
		return withDash(b, ctx, mapview.RenderElevation(columns, in.Axis, fixed)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "survey_site",
		Description: "Embark orientation report: map dimensions, surface z, dwarf cluster, sampled soil/stone stratigraphy, surface vegetation. Call FIRST on any new fort or after reconnecting.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		snap := b.Snapshot()
		topo := b.Topo()
		if topo == nil || len(snap.Entities.Dwarves) == 0 {
			return withDash(b, ctx, "survey unavailable: waiting for full state / dwarves (is ai-connect done?)"), nil, nil
		}
		w, h, d := topo.GetDimensions()
		// crewZ anchors z-window sampling; it is NOT "the surface" — real
		// ground height varies per column (hills, valleys). See SurveyData.
		crewZ := snap.Entities.Dwarves[0].Z
		cx, cy := snap.Entities.Dwarves[0].X, snap.Entities.Dwarves[0].Y
		for _, e := range snap.Entities.Dwarves {
			if e.Z > crewZ {
				crewZ, cx, cy = e.Z, e.X, e.Y
			}
		}
		data := SurveyData{MapW: w, MapH: h, MapD: d, CrewZ: crewZ, Dwarves: snap.Entities.Dwarves}
		// Close-in stratigraphy: crew position and two offsets.
		for _, off := range [][2]int16{{0, 0}, {12, 0}, {0, 12}} {
			c, err := b.ColumnProfile(ctx, cx+off[0], cy+off[1], crewZ+2, crewZ-25)
			if err == nil {
				data.Columns = append(data.Columns, c)
			}
		}
		// Coarse 3x3 grid spread across the whole map, purely to find each
		// sampled column's own surface Z and report the range — a single
		// dwarf-Z proxy stamped onto every column is the exact bug this
		// replaces. Window is capped at 60 z-levels per column_profile call
		// (queries.cpp:926), so the requested crewZ+20/-40 anchor is
		// trimmed by one level (crewZ-39, not crewZ-40) to fit.
		zTop, zBottom := crewZ+20, crewZ-39
		if zBottom < 0 {
			zBottom = 0
		}
		const gridDivisions = 4 // 3x3 interior grid points at w/4, 2w/4, 3w/4 etc.
		for gx := 1; gx < gridDivisions; gx++ {
			for gy := 1; gy < gridDivisions; gy++ {
				sx := int16(int(w) * gx / gridDivisions)
				sy := int16(int(h) * gy / gridDivisions)
				c, err := b.ColumnProfile(ctx, sx, sy, zTop, zBottom)
				if err == nil {
					data.SurfaceSamples = append(data.SurfaceSamples, c)
				}
			}
		}
		if s, err := b.MapSlice(ctx, cx-15, cy-15, crewZ, cx+15, cy+15); err == nil {
			data.SurfaceSlice = s
		}
		return withDash(b, ctx, renderSurvey(data)), nil, nil
	})

	type findIn struct {
		Width  int `json:"width" jsonschema:"room width in tiles"`
		Height int `json:"height" jsonschema:"room height in tiles"`
		Z      int `json:"z" jsonschema:"z-level to search (use cross_section to pick a stone level)"`
		NearX  int `json:"near_x" jsonschema:"anchor x (e.g. your stair shaft)"`
		NearY  int `json:"near_y" jsonschema:"anchor y"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "find_dig_site",
		Description: "Search a z-level for fully-solid rectangles where a WxH room can be dug, ranked by distance from your anchor. Returns concrete coordinates — use these instead of guessing. Solid includes hidden fog tiles (they're undug rock: diggable).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in findIn) (*mcp.CallToolResult, any, error) {
		topo := b.Topo()
		if topo == nil {
			return withDash(b, ctx, "topology not built yet (waiting for full state)"), nil, nil
		}
		// Validate against real map bounds before scanning: out-of-range z
		// (or a degenerate footprint) would otherwise read StateUnknown
		// everywhere and fabricate confident "fully solid" candidates on a
		// nonexistent level — the exact failure mode this tool prevents.
		w, h, d := topo.GetDimensions()
		if in.Width <= 0 || in.Height <= 0 || in.Width > int(w) || in.Height > int(h) {
			return withDash(b, ctx, fmt.Sprintf(
				"invalid room size %dx%d: width and height must be between 1 and the map size (%dx%d)",
				in.Width, in.Height, w, h)), nil, nil
		}
		if in.Z < 0 || in.Z >= int(d) {
			return withDash(b, ctx, fmt.Sprintf(
				"z=%d out of range: map has %d levels (valid z is 0..%d) — use cross_section or survey_site to pick a real level",
				in.Z, d, int(d)-1)), nil, nil
		}
		sites := mapview.FindDigSites(topo, mapview.DigSiteRequest{
			W: int16(in.Width), H: int16(in.Height), Z: int16(in.Z),
			NearX: int16(in.NearX), NearY: int16(in.NearY), MaxCandidates: 5,
		})
		if len(sites) == 0 {
			return withDash(b, ctx, "no fully-solid candidates on that z — try another level or smaller room"), nil, nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d candidates for a %dx%d room on z=%d:\n", len(sites), in.Width, in.Height, in.Z)
		for i, s := range sites {
			fmt.Fprintf(&sb, "%d. dig from (%d,%d,%d) to (%d,%d,%d) — %s\n",
				i+1, s.X1, s.Y1, s.Z, s.X2, s.Y2, s.Z, s.Rationale)
		}
		return withDash(b, ctx, sb.String()), nil, nil
	})
}
