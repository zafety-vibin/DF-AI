package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/mapview"
)

func registerPerceptTools(srv *mcp.Server, b *Bridge) {
	type lookIn struct {
		X      int    `json:"x" jsonschema:"center x"`
		Y      int    `json:"y" jsonschema:"center y"`
		Z      int    `json:"z" jsonschema:"z-level to view"`
		Radius int    `json:"radius,omitempty" jsonschema:"half-width of the crop, default 12, max 15"`
		Lens   string `json:"lens,omitempty" jsonschema:"optional overlay: buildings|designations. Paints one annotation layer onto the terrain grid; omit for the plain terrain view (which still shows dig designations as 'd'). Exact building/zone types via buildings/building_status."`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "look",
		Description: "Render a small annotated map crop of one z-level around (x,y). Glyph grid with legend; dwarves marked @, dig designations marked d. Pass lens=buildings or lens=designations for detail overlays. '?' tiles are hidden fog — solid undug ground you CAN designate digging into. Use for local layout checks; use find_dig_site for choosing dig locations.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in lookIn) (*mcp.CallToolResult, any, error) {
		r := in.Radius
		if r <= 0 {
			r = 12
		}
		if r > 15 {
			r = 15
		}
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
