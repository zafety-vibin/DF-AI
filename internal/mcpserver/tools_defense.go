package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type burrowListEntry struct {
	Name          string `json:"name"`
	ID            int    `json:"id"`
	TileCount     int    `json:"tile_count"`
	AssignedUnits int    `json:"assigned_units"`
	InActiveAlert bool   `json:"in_active_alert"`
}

func renderBurrows(raw []byte) string {
	var resp struct {
		Burrows       []burrowListEntry `json:"burrows"`
		AlertSounding bool              `json:"alert_sounding"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_burrows response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	var sb strings.Builder
	if resp.AlertSounding {
		sb.WriteString("Civilian alert is SOUNDING.\n")
	} else {
		sb.WriteString("Civilian alert is clear.\n")
	}
	if len(resp.Burrows) == 0 {
		sb.WriteString("No burrows.\n")
		return sb.String()
	}
	fmt.Fprintf(&sb, "%d burrows:\n", len(resp.Burrows))
	for _, bu := range resp.Burrows {
		fmt.Fprintf(&sb, "- %q: %d tiles, %d units assigned", bu.Name, bu.TileCount, bu.AssignedUnits)
		if bu.InActiveAlert {
			sb.WriteString(" [ACTIVE ALERT MEMBER]")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func registerDefenseTools(srv *mcp.Server, b *Bridge) {
	type designateBurrowIn struct {
		Name string `json:"name" jsonschema:"burrow name -- an existing name adds tiles to that burrow instead of creating a new one"`
		X1   int    `json:"x1"`
		Y1   int    `json:"y1"`
		Z1   int    `json:"z1"`
		X2   int    `json:"x2"`
		Y2   int    `json:"y2"`
		Z2   *int   `json:"z2,omitempty" jsonschema:"optional: top of a multi-z paint (defaults to z1 for a single-level burrow)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "designate_burrow",
		Description: "Create a named burrow (if new) and paint the tile rect (x1,y1,z1)-(x2,y2,z2) into it. Repeatable: calling again with the same name adds more tiles to that burrow. Paints through hidden tiles by design (same as dig designations) -- no floor/carved-space requirement, unlike designate_zone. Once painted, set_alert can make this the active civilian shelter directly -- assign_burrow is a separate, optional standing-restriction tool, not a prerequisite.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in designateBurrowIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		z2 := in.Z1
		if in.Z2 != nil {
			z2 = *in.Z2
		}
		res, err := b.Exec.SendDesignateBurrow(in.Name, int16(in.X1), int16(in.Y1), int16(in.Z1), int16(in.X2), int16(in.Y2), int16(z2))
		what := fmt.Sprintf("designate_burrow %q (%d,%d,%d)-(%d,%d,%d)", in.Name, in.X1, in.Y1, in.Z1, in.X2, in.Y2, z2)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type removeBurrowIn struct {
		Name string `json:"name"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "remove_burrow",
		Description: "Delete a named burrow entirely -- clears its tiles and unit assignments first, and detaches it from the civilian alert if it was a member.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in removeBurrowIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendRemoveBurrow(in.Name)
		what := fmt.Sprintf("remove_burrow %q", in.Name)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type assignBurrowIn struct {
		Name        string `json:"name"`
		UnitID      *int   `json:"unit_id,omitempty" jsonschema:"a single unit to assign -- required unless all_citizens is true"`
		AllCitizens bool   `json:"all_citizens,omitempty" jsonschema:"assign every current citizen in one call instead of a single unit_id"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "assign_burrow",
		Description: "Assign a unit (unit_id) or every current citizen (all_citizens=true) to a named burrow. Independent of set_alert -- DF's civilian alert pulls every non-military citizen to the alerted burrow regardless of assignment -- and an assignment made here persists after the alert clears (unassign_burrow to remove it).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in assignBurrowIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		if !in.AllCitizens && in.UnitID == nil {
			return withDash(b, ctx, "assign_burrow needs either unit_id or all_citizens=true"), nil, nil
		}
		unitID := 0
		if in.UnitID != nil {
			unitID = *in.UnitID
		}
		res, err := b.Exec.SendAssignBurrow(in.Name, int32(unitID), in.AllCitizens, true)
		what := fmt.Sprintf("assign_burrow %q", in.Name)
		if in.AllCitizens {
			what += " all_citizens"
		} else {
			what += fmt.Sprintf(" unit#%d", unitID)
		}
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type unassignBurrowIn struct {
		Name        string `json:"name"`
		UnitID      *int   `json:"unit_id,omitempty" jsonschema:"a single unit to unassign -- required unless all_citizens is true"`
		AllCitizens bool   `json:"all_citizens,omitempty" jsonschema:"unassign every current citizen in one call instead of a single unit_id"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unassign_burrow",
		Description: "Remove a unit's (or every current citizen's) assignment from a named burrow.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in unassignBurrowIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		if !in.AllCitizens && in.UnitID == nil {
			return withDash(b, ctx, "unassign_burrow needs either unit_id or all_citizens=true"), nil, nil
		}
		unitID := 0
		if in.UnitID != nil {
			unitID = *in.UnitID
		}
		res, err := b.Exec.SendAssignBurrow(in.Name, int32(unitID), in.AllCitizens, false)
		what := fmt.Sprintf("unassign_burrow %q", in.Name)
		if in.AllCitizens {
			what += " all_citizens"
		} else {
			what += fmt.Sprintf(" unit#%d", unitID)
		}
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type setAlertIn struct {
		Name   string `json:"name"`
		Active bool   `json:"active" jsonschema:"true sounds the civilian alert against this burrow (pulls every non-military citizen inside, no assign_burrow needed), false clears it"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_alert",
		Description: "Sound or clear DF's vanilla civilian alert against a named burrow. active=true rushes every non-military citizen to that burrow and confines them there; active=false clears it (and auto-clears the underlying alarm if this was the last burrow restricting it). assign_burrow is NOT a prerequisite -- DF's own gui/civ-alert never assigns units, it only paints the burrow and flips this alert.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in setAlertIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendSetAlert(in.Name, in.Active)
		what := fmt.Sprintf("set_alert %q active=%v", in.Name, in.Active)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_burrows",
		Description: "List burrows: name, tile count, assigned-unit count, and whether each is a member of the currently-sounding civilian alert (if any).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "list_burrows", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderBurrows(raw)), nil, nil
	})
}
