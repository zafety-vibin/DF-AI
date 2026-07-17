package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// farmSeasons maps the tool's season vocabulary to the wire byte
// (protocol.FarmSeason*). "all" writes the same crop into all four of the
// farm plot's season slots in one call — a wire-level convenience, not a
// DF planting mode.
var farmSeasons = map[string]uint8{
	"spring": protocol.FarmSeasonSpring,
	"summer": protocol.FarmSeasonSummer,
	"autumn": protocol.FarmSeasonAutumn,
	"winter": protocol.FarmSeasonWinter,
	"all":    protocol.FarmSeasonAll,
}

func registerFarmTools(srv *mcp.Server, b *Bridge) {
	type farmPlotIn struct {
		X1 int `json:"x1"`
		Y1 int `json:"y1"`
		Z  int `json:"z"`
		X2 int `json:"x2"`
		Y2 int `json:"y2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "build_farm_plot",
		Description: "Designate a rectangular farm plot at (x1,y1)-(x2,y2) on level z — extent-shaped like a stockpile, not a single-tile build. Needs open, non-aquatic soil or mud floor; DF itself rejects bad ground (the ACK carries DF's real error verbatim, not a guess). A freshly built plot grows NOTHING until assign_crop programs a crop into at least one season slot — always call assign_crop right after this succeeds.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in farmPlotIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendBuildFarmPlot(int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2))
		what := fmt.Sprintf("build_farm_plot (%d,%d)-(%d,%d) z=%d", in.X1, in.Y1, in.X2, in.Y2, in.Z)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type assignCropIn struct {
		X      int    `json:"x" jsonschema:"a tile inside the farm plot's footprint"`
		Y      int    `json:"y"`
		Z      int    `json:"z"`
		Season string `json:"season" jsonschema:"spring|summer|autumn|winter|all — 'all' assigns the same crop to every season slot in one call"`
		Crop   string `json:"crop" jsonschema:"a plant raw token or display name from list_crops, matched exactly (case-insensitive, NOT a substring) — e.g. MUSHROOM_HELMET_PLUMP or 'plump helmet'; or 'fallow' to clear the slot(s) — a plot with no crop assigned for a season grows NOTHING that season"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "assign_crop",
		Description: "Assign a crop (or 'fallow' to clear) to one or all four season slots of the farm plot at (x,y,z). Use list_crops to discover valid crop names and check underground/surface eligibility first — the plugin also rejects a crop that doesn't grow in the requested single season (pass season=all to skip that check).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in assignCropIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		season, ok := farmSeasons[strings.ToLower(in.Season)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown season %q (spring|summer|autumn|winter|all)", in.Season)), nil, nil
		}
		res, err := b.Exec.SendSetFarmCrop(int16(in.X), int16(in.Y), int16(in.Z), season, in.Crop)
		what := fmt.Sprintf("assign_crop %s @(%d,%d,%d) season=%s", in.Crop, in.X, in.Y, in.Z, strings.ToLower(in.Season))
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})
}
