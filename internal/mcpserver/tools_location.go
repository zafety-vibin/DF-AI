package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/protocol"
)

var locationWireTypes = map[string]uint8{
	"tavern": protocol.LocationTypeTavern, "temple": protocol.LocationTypeTemple,
	"library": protocol.LocationTypeLibrary, "guildhall": protocol.LocationTypeGuildhall,
	"hospital": protocol.LocationTypeHospital,
}

// locationTypeDisplayNames maps the plugin's ENUM_KEY_STR(abstract_building_type, ...)
// names to the friendlier names this tool surfaces.
var locationTypeDisplayNames = map[string]string{
	"INN_TAVERN": "Tavern", "TEMPLE": "Temple", "LIBRARY": "Library", "GUILDHALL": "Guildhall",
	"HOSPITAL": "Hospital",
}

type lodgingEntry struct {
	CivzoneID int `json:"civzone_id"`
	X         int `json:"x"`
	Y         int `json:"y"`
	Z         int `json:"z"`
}

type locationListEntry struct {
	ID      int            `json:"id"`
	Type    string         `json:"type"`
	X1      int            `json:"x1"`
	Y1      int            `json:"y1"`
	X2      int            `json:"x2"`
	Y2      int            `json:"y2"`
	Z       int            `json:"z"`
	Lodging []lodgingEntry `json:"lodging"`

	// Temple only (queries.cpp handleListLocations): the dedication
	// create_location can write. Without these the write was one-way — the
	// plugin's own ACK calls a deity dedication UNVERIFIED and says to confirm
	// it in-game, which the playing model cannot do.
	//
	// DeityType is the religious_practice_type enum key ("NONE" /
	// "WORSHIP_HFID" / "RELIGION_ENID"); empty means a plugin build that
	// predates this field, which is NOT the same as "dedicated to no one" —
	// renderLocations keeps those two apart rather than reporting an absent
	// field as a measured NONE. DeityHfID/DeityEntityID are pointers because 0
	// is a valid id, the same nil-as-presence convention the stockpile
	// container ceilings use.
	DeityType     string `json:"deity_type,omitempty"`
	DeityHfID     *int   `json:"deity_hf_id,omitempty"`
	DeityEntityID *int   `json:"deity_entity_id,omitempty"`
	DeityName     string `json:"deity_name,omitempty"`
}

// templeDedication renders a temple's dedication clause, or "" for anything
// that isn't a temple. Degradation is layered exactly like stockpileLine's:
// no field at all from an older plugin says so instead of inventing "no
// dedication", and an id with no resolvable name still reports the id.
func templeDedication(loc locationListEntry) string {
	if loc.Type != "TEMPLE" {
		return ""
	}
	switch loc.DeityType {
	case "":
		return " — dedication not reported by this plugin build"
	case "NONE":
		return " — dedicated to no particular deity"
	case "WORSHIP_HFID":
		if loc.DeityHfID == nil {
			return " — dedicated to a deity (hf id not reported)"
		}
		if loc.DeityName != "" {
			return fmt.Sprintf(" — dedicated to %s (hf %d)", loc.DeityName, *loc.DeityHfID)
		}
		return fmt.Sprintf(" — dedicated to historical figure %d (name unresolved)", *loc.DeityHfID)
	case "RELIGION_ENID":
		if loc.DeityEntityID == nil {
			return " — dedicated to a religion (entity id not reported)"
		}
		if loc.DeityName != "" {
			return fmt.Sprintf(" — dedicated to the religion %s (entity %d)", loc.DeityName, *loc.DeityEntityID)
		}
		return fmt.Sprintf(" — dedicated to religion entity %d (name unresolved)", *loc.DeityEntityID)
	default:
		return fmt.Sprintf(" — deity_type=%s", loc.DeityType)
	}
}

func renderLocations(raw []byte) string {
	var resp struct {
		Locations []locationListEntry `json:"locations"`
		Truncated bool                `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_locations response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Locations) == 0 {
		return "No locations."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d locations:\n", len(resp.Locations))
	for _, loc := range resp.Locations {
		name := locationTypeDisplayNames[loc.Type]
		if name == "" {
			name = loc.Type
		}
		fmt.Fprintf(&sb, "- %s at (%d,%d)-(%d,%d) z=%d", name, loc.X1, loc.Y1, loc.X2, loc.Y2, loc.Z)
		sb.WriteString(templeDedication(loc))
		if len(loc.Lodging) > 0 {
			fmt.Fprintf(&sb, " — %d lodging room(s)", len(loc.Lodging))
		}
		sb.WriteString("\n")
	}
	if resp.Truncated {
		sb.WriteString("... list truncated at the plugin's cap\n")
	}
	return sb.String()
}

func registerLocationTools(srv *mcp.Server, b *Bridge) {
	type createLocationIn struct {
		X          int    `json:"x" jsonschema:"a tile inside the founding MeetingHall zone"`
		Y          int    `json:"y"`
		Z          int    `json:"z"`
		Type       string `json:"type" jsonschema:"tavern|temple|library|guildhall|hospital"`
		Profession string `json:"profession,omitempty" jsonschema:"required for guildhall only — the profession this guild serves, e.g. CARPENTER, MASON"`
		DeityHfID  *int   `json:"deity_hf_id,omitempty" jsonschema:"temple only — a historical figure id from fort_story mode=social kind=worship (the b_id of a relation=deity edge); omit for a temple dedicated to no one in particular"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_location",
		Description: "Convert an existing MeetingHall zone (see designate_zone) into a real Location — a Tavern (social/entertainment, and can house lodging via assign_lodging), Temple, Library, Guildhall (requires a profession), or Hospital. The MeetingHall itself must already exist and not already have a location.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createLocationIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		lt, ok := locationWireTypes[strings.ToLower(in.Type)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown location type %q", in.Type)), nil, nil
		}
		var deity *int32
		if in.DeityHfID != nil {
			v := int32(*in.DeityHfID)
			deity = &v
		}
		res, err := b.Exec.SendCreateLocation(int16(in.X), int16(in.Y), int16(in.Z), lt, in.Profession, deity)
		what := fmt.Sprintf("create_location %s at (%d,%d,%d)", in.Type, in.X, in.Y, in.Z)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_locations",
		Description: "List every Tavern/Temple/Library/Guildhall/Hospital Location, with the founding zone's extents, (for taverns) the lodging roster, and (for temples) the deity dedication.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "list_locations", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderLocations(raw)), nil, nil
	})

	type assignLodgingIn struct {
		TavernX  int `json:"tavern_x" jsonschema:"a tile inside the tavern's founding zone"`
		TavernY  int `json:"tavern_y"`
		TavernZ  int `json:"tavern_z"`
		BedroomX int `json:"bedroom_x" jsonschema:"a tile inside the bedroom zone to make into a guest room"`
		BedroomY int `json:"bedroom_y"`
		BedroomZ int `json:"bedroom_z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "assign_lodging",
		Description: "Link a Bedroom zone as guest lodging inside a Tavern. First-of-its-kind capability for this project — verify with list_locations and in-game observation that it behaves as expected, don't assume it silently works just because the ACK is SUCCESS.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in assignLodgingIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendAssignLodging(int16(in.TavernX), int16(in.TavernY), int16(in.TavernZ), int16(in.BedroomX), int16(in.BedroomY), int16(in.BedroomZ))
		what := fmt.Sprintf("assign_lodging tavern(%d,%d,%d) bedroom(%d,%d,%d)", in.TavernX, in.TavernY, in.TavernZ, in.BedroomX, in.BedroomY, in.BedroomZ)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type unassignLodgingIn struct {
		BedroomX int `json:"bedroom_x"`
		BedroomY int `json:"bedroom_y"`
		BedroomZ int `json:"bedroom_z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unassign_lodging",
		Description: "Remove a bedroom's tavern-lodging assignment.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in unassignLodgingIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendUnassignLodging(int16(in.BedroomX), int16(in.BedroomY), int16(in.BedroomZ))
		what := fmt.Sprintf("unassign_lodging bedroom(%d,%d,%d)", in.BedroomX, in.BedroomY, in.BedroomZ)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})
}
