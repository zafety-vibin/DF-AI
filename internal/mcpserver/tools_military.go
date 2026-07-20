package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/commands"
)

type squadMemberEntry struct {
	PositionIndex int  `json:"position_index"`
	IsCommander   bool `json:"is_commander"`
	Vacant        bool `json:"vacant"`
	UnitID        int  `json:"unit_id"`
}

type squadOrderBurrowEntry struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type squadOrderEntry struct {
	Type    string                  `json:"type"`
	X       int                     `json:"x"`
	Y       int                     `json:"y"`
	Z       int                     `json:"z"`
	Burrows []squadOrderBurrowEntry `json:"burrows"`
}

type squadListEntry struct {
	ID       int                `json:"id"`
	Name     string             `json:"name"`
	EntityID int                `json:"entity_id"`
	Members  []squadMemberEntry `json:"members"`
	Orders   []squadOrderEntry  `json:"orders"`
}

// renderSquads renders list_squads' response. squad_position::occupant is
// a HISTORICAL FIGURE id on the plugin side -- already resolved to a live
// unit id (or -1 if vacant/no live unit) before it reaches here, so this
// renderer never does that lookup itself.
func renderSquads(raw []byte) string {
	var resp struct {
		Squads []squadListEntry `json:"squads"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_squads response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Squads) == 0 {
		return "No squads."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d squad%s:\n", len(resp.Squads), plural(len(resp.Squads)))
	for _, s := range resp.Squads {
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		filled := 0
		for _, m := range s.Members {
			if !m.Vacant {
				filled++
			}
		}
		fmt.Fprintf(&sb, "- squad #%d %q: %d/%d slots filled\n", s.ID, name, filled, len(s.Members))
		for _, m := range s.Members {
			role := "soldier"
			if m.IsCommander {
				role = "commander"
			}
			if m.Vacant {
				fmt.Fprintf(&sb, "    [%d] %s: vacant\n", m.PositionIndex, role)
			} else {
				fmt.Fprintf(&sb, "    [%d] %s: unit#%d\n", m.PositionIndex, role, m.UnitID)
			}
		}
		if len(s.Orders) == 0 {
			sb.WriteString("    orders: none\n")
			continue
		}
		for _, o := range s.Orders {
			switch o.Type {
			case "station":
				fmt.Fprintf(&sb, "    order: station at (%d,%d,%d)\n", o.X, o.Y, o.Z)
			case "defend_burrow":
				names := make([]string, 0, len(o.Burrows))
				for _, bur := range o.Burrows {
					names = append(names, fmt.Sprintf("%q", bur.Name))
				}
				fmt.Fprintf(&sb, "    order: defend burrow(s) %s\n", strings.Join(names, ", "))
			default:
				sb.WriteString("    order: (other order type, not decoded by this tool)\n")
			}
		}
	}
	return sb.String()
}

// registerMilitaryTools registers the minimal DF v50 military surface:
// create/staff/order a squad, and a read-only list. See docs/decisions.md
// (2026-07-19 military research pass) for the full data-model citation and
// what's deliberately left out (training schedules, uniforms, patrol
// routes, kill-list/kill-hf orders, and the site-leaving raid/drive-off/
// rescue/retrieve order family).
func registerMilitaryTools(srv *mcp.Server, b *Bridge) {
	type createSquadIn struct {
		PositionCode string `json:"position_code,omitempty" jsonschema:"entity_position code to lead the new squad (default MILITIA_CAPTAIN) -- discover others (any with squad_size > 0) via position_vacancies"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_squad",
		Description: "Create a new squad led by an entity_position (default MILITIA_CAPTAIN). Fills an existing vacant leader slot if one exists, or -- for this fort's first-ever squad under that position -- mints a fresh one; the ack names which happened and flags the mint path as unverified (no DFHack precedent for that exact write). The new squad starts empty with no orders -- use assign_squad to staff it and squad_order to give it a task.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createSquadIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendCreateSquad(in.PositionCode)
		what := "create_squad"
		if in.PositionCode != "" {
			what += " position=" + in.PositionCode
		}
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type assignSquadIn struct {
		SquadID int  `json:"squad_id" jsonschema:"from create_squad's ack or list_squads"`
		UnitID  int  `json:"unit_id" jsonschema:"the dwarf's id from the dwarves tool"`
		Add     bool `json:"add" jsonschema:"true assigns the dwarf to the squad (auto-picks the first free non-commander slot); false returns them to civilian duty"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "assign_squad",
		Description: "Add or remove one dwarf from a squad's membership. Civilian labors are NOT auto-disabled when a dwarf joins (v50 doesn't do this) -- use set_labor if a dedicated, non-working soldier is wanted. The commander slot (position 0) cannot be filled this way.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in assignSquadIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendAssignSquad(int32(in.SquadID), int32(in.UnitID), in.Add)
		what := fmt.Sprintf("assign_squad squad=%d unit=%d add=%v", in.SquadID, in.UnitID, in.Add)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type squadOrderIn struct {
		SquadID    int    `json:"squad_id"`
		Type       string `json:"type" jsonschema:"station|defend_burrow|cancel"`
		X          int    `json:"x,omitempty" jsonschema:"station only: target tile"`
		Y          int    `json:"y,omitempty"`
		Z          int    `json:"z,omitempty"`
		BurrowName string `json:"burrow_name,omitempty" jsonschema:"defend_burrow only: an existing burrow name from designate_burrow"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "squad_order",
		Description: "Give a squad exactly one standing order, replacing any it already had: station (hold a tile -- unverified order type, confirm the squad actually moves before relying on it) or defend_burrow (hold a named burrow -- the better-fit, DF-native chokepoint order). type=cancel clears the order with no replacement; combine with assign_squad add=false per member to fully stand a squad down to civilian duty.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in squadOrderIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		var res *commands.CommandResult
		var err error
		var what string
		switch strings.ToLower(in.Type) {
		case "station":
			res, err = b.Exec.SendSquadOrderStation(int32(in.SquadID), int16(in.X), int16(in.Y), int16(in.Z))
			what = fmt.Sprintf("squad_order squad=%d station (%d,%d,%d)", in.SquadID, in.X, in.Y, in.Z)
		case "defend_burrow":
			res, err = b.Exec.SendSquadOrderDefendBurrow(int32(in.SquadID), in.BurrowName)
			what = fmt.Sprintf("squad_order squad=%d defend_burrow %q", in.SquadID, in.BurrowName)
		case "cancel":
			res, err = b.Exec.SendSquadOrderCancel(int32(in.SquadID))
			what = fmt.Sprintf("squad_order squad=%d cancel", in.SquadID)
		default:
			return withDash(b, ctx, fmt.Sprintf("unknown squad_order type %q -- valid types: station, defend_burrow, cancel", in.Type)), nil, nil
		}
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_squads",
		Description: "List squads: id, name, membership (position index, commander flag, vacancy, unit id), and current orders.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "list_squads", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderSquads(raw)), nil, nil
	})
}
