package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/mapview"
)

// NOTE: Task 11 adds find_dig_site to this file and extends this import
// block with "fmt" and "strings" when its code needs it.

func registerPerceptTools(srv *mcp.Server, b *Bridge) {
	type lookIn struct {
		X      int `json:"x" jsonschema:"center x"`
		Y      int `json:"y" jsonschema:"center y"`
		Z      int `json:"z" jsonschema:"z-level to view"`
		Radius int `json:"radius,omitempty" jsonschema:"half-width of the crop, default 12, max 15"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "look",
		Description: "Render a small annotated map crop of one z-level around (x,y). Glyph grid with legend; dwarves marked @. '?' tiles are hidden fog — solid undug ground you CAN designate digging into. Use for local layout checks; use find_dig_site for choosing dig locations.",
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
		return withDash(b, ctx, mapview.RenderCrop(s, marks)), nil, nil
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
		surfaceZ := int16(0)
		minDZ, maxDZ := int16(32767), int16(-32768)
		for _, d := range snap.Entities.Dwarves {
			if d.Z < minDZ {
				minDZ = d.Z
			}
			if d.Z > maxDZ {
				maxDZ = d.Z
			}
		}
		if len(snap.Entities.Dwarves) > 0 {
			surfaceZ = maxDZ
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
		return withDash(b, ctx, mapview.RenderColumn(c, surfaceZ)), nil, nil
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
		surfaceZ := snap.Entities.Dwarves[0].Z
		cx, cy := snap.Entities.Dwarves[0].X, snap.Entities.Dwarves[0].Y
		for _, e := range snap.Entities.Dwarves {
			if e.Z > surfaceZ {
				surfaceZ, cx, cy = e.Z, e.X, e.Y
			}
		}
		data := SurveyData{MapW: w, MapH: h, MapD: d, SurfaceZ: surfaceZ, Dwarves: snap.Entities.Dwarves}
		// Three stratigraphy samples: at the crew and two offsets.
		for _, off := range [][2]int16{{0, 0}, {12, 0}, {0, 12}} {
			c, err := b.ColumnProfile(ctx, cx+off[0], cy+off[1], surfaceZ+2, surfaceZ-25)
			if err == nil {
				data.Columns = append(data.Columns, c)
			}
		}
		if s, err := b.MapSlice(ctx, cx-15, cy-15, surfaceZ, cx+15, cy+15); err == nil {
			data.SurfaceSlice = s
		}
		return withDash(b, ctx, renderSurvey(data)), nil, nil
	})
}
