package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// zoneWireTypes maps a lowercase user-facing name to its wire byte —
// matches the plan's corrected wire table exactly.
var zoneWireTypes = map[string]uint8{
	"bedroom": protocol.ZoneTypeBedroom, "office": protocol.ZoneTypeOffice,
	"tomb": protocol.ZoneTypeTomb, "dining_hall": protocol.ZoneTypeDiningHall,
	"meeting_hall": protocol.ZoneTypeMeetingHall, "dormitory": protocol.ZoneTypeDormitory,
	"barracks": protocol.ZoneTypeBarracks, "pen": protocol.ZoneTypePen,
	"pond": protocol.ZoneTypePond, "archery_range": protocol.ZoneTypeArcheryRange,
	"plant_gathering": protocol.ZoneTypePlantGathering, "water_source": protocol.ZoneTypeWaterSource,
	"dump": protocol.ZoneTypeDump, "sand_collection": protocol.ZoneTypeSandCollection,
	"fishing_area": protocol.ZoneTypeFishingArea, "clay_collection": protocol.ZoneTypeClayCollection,
	"dungeon": protocol.ZoneTypeDungeon, "animal_training": protocol.ZoneTypeAnimalTraining,
}

// zoneTypeVocabulary renders zoneWireTypes' keys sorted, for use in
// educational error messages (house style: never leave the model guessing
// at the valid vocabulary).
func zoneTypeVocabulary() string {
	keys := make([]string, 0, len(zoneWireTypes))
	for k := range zoneWireTypes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

type zoneListEntry struct {
	Kind          int    `json:"kind"`
	TypeName      string `json:"type_name"`
	X1            int    `json:"x1"`
	Y1            int    `json:"y1"`
	X2            int    `json:"x2"`
	Y2            int    `json:"y2"`
	Z             int    `json:"z"`
	OwnerUnitID   int    `json:"owner_unit_id"`
	AssignedUnits []int  `json:"assigned_units"`
}

// renderZones renders list_zones' response. Default (no typeFilter) is a
// summary grouped by type -- one line per type, not one line per zone --
// per the design doc's token-scaling correction (recommendation #5:
// the model needs the shape of the fort far more often than N
// coordinates). Passing typeFilter switches to full per-zone detail for
// that type only. zFilter < 0 means no z filter (informational only in
// the render; the actual filtering happens plugin-side via the query
// args — this parameter only affects the summary-vs-detail choice
// alongside typeFilter, kept simple: any explicit filter narrows to detail).
func renderZones(raw []byte, typeFilter string, zFilter int) string {
	var resp struct {
		Zones     []zoneListEntry `json:"zones"`
		Truncated bool            `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_zones response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Zones) == 0 {
		return "No zones."
	}

	var sb strings.Builder
	if typeFilter != "" || zFilter >= 0 {
		// typeFilter actively narrows the list here (in production the
		// plugin has already filtered by the same criterion via the query
		// args, so this is a defense-in-depth match against whatever the
		// caller passed — case-insensitive since the tool's user-facing
		// Type argument is lowercase ("bedroom") while the wire's
		// type_name is CamelCase ("Bedroom")).
		zones := resp.Zones
		if typeFilter != "" {
			filtered := make([]zoneListEntry, 0, len(zones))
			for _, z := range zones {
				if strings.EqualFold(z.TypeName, typeFilter) {
					filtered = append(filtered, z)
				}
			}
			zones = filtered
		}
		if len(zones) == 0 {
			sb.WriteString("No zones matching filter.\n")
		} else {
			fmt.Fprintf(&sb, "%d zones:\n", len(zones))
			for _, z := range zones {
				fmt.Fprintf(&sb, "- %s at (%d,%d)-(%d,%d) z=%d", z.TypeName, z.X1, z.Y1, z.X2, z.Y2, z.Z)
				if z.OwnerUnitID >= 0 {
					fmt.Fprintf(&sb, " owner=unit#%d", z.OwnerUnitID)
				}
				if len(z.AssignedUnits) > 0 {
					fmt.Fprintf(&sb, " assigned=%v", z.AssignedUnits)
				}
				sb.WriteString("\n")
			}
		}
	} else {
		byType := map[string][]zoneListEntry{}
		for _, z := range resp.Zones {
			byType[z.TypeName] = append(byType[z.TypeName], z)
		}
		types := make([]string, 0, len(byType))
		for t := range byType {
			types = append(types, t)
		}
		sort.Strings(types)

		fmt.Fprintf(&sb, "%d zones:\n", len(resp.Zones))
		for _, t := range types {
			zones := byType[t]
			owned, unowned, animals := 0, 0, 0
			hasOwnerData := false
			for _, z := range zones {
				if z.OwnerUnitID >= 0 {
					owned++
					hasOwnerData = true
				} else if len(z.AssignedUnits) == 0 {
					unowned++
				}
				animals += len(z.AssignedUnits)
			}
			fmt.Fprintf(&sb, "- %d %s", len(zones), t)
			// "animals" phrasing only makes sense for Pen/Pond (roster =
			// livestock); every other roster-type zone (e.g. Dormitory)
			// rosters units, not animals, so say "assigned" instead.
			isAnimalZone := strings.EqualFold(t, "Pen") || strings.EqualFold(t, "Pond")
			switch {
			case animals > 0 && isAnimalZone:
				fmt.Fprintf(&sb, " (%d animals total)", animals)
			case animals > 0:
				fmt.Fprintf(&sb, " (%d assigned)", animals)
			case hasOwnerData:
				fmt.Fprintf(&sb, " (%d owned, %d unowned)", owned, unowned)
			}
			sb.WriteString("\n")
		}
	}
	if resp.Truncated {
		sb.WriteString("... list truncated at the plugin's cap — pass type or z to narrow it\n")
	}
	return sb.String()
}

// zoneValueComponent is one furniture item inside a zone's contained
// buildings, from the plugin's zone_value query (queries.cpp
// handleZoneValue) -- material/quality/value are read via DFHack's own
// Items::getValue and item->getQuality(), the same DF-formula-confirmed
// path handleZoneValue's file comment documents.
type zoneValueComponent struct {
	BuildingType string `json:"building_type"`
	ItemType     string `json:"item_type"`
	Material     string `json:"material"`
	Quality      string `json:"quality"`
	Value        int    `json:"value"`
}

type zoneValueEngravingQuality struct {
	Quality string `json:"quality"`
	Count   int    `json:"count"`
}

// zoneValueResponse mirrors the plugin's zone_value query response.
// EstimatedValue is this tool's OWN sum of Components' values -- NOT a
// reimplementation of DF's internal room-quality-tier logic, which
// handleZoneValue's file comment confirms this DFHack checkout cannot run
// (Buildings::getRoomDescription's body is disabled upstream). RoomValueField
// names which of noble_demands' required_office/required_bedroom/
// required_dining/required_tomb minimums this zone type compares against,
// nil when this zone kind has no such analog. ExperimentalPersonalValue is
// a SEPARATE, uncalibrated bonus number (building->getPersonalValue) --
// never folded into EstimatedValue.
type zoneValueResponse struct {
	Kind                      int                         `json:"kind"`
	TypeName                  string                      `json:"type_name"`
	RoomValueField            *string                     `json:"room_value_field"`
	X1                        int                         `json:"x1"`
	Y1                        int                         `json:"y1"`
	X2                        int                         `json:"x2"`
	Y2                        int                         `json:"y2"`
	Z                         int                         `json:"z"`
	Components                []zoneValueComponent        `json:"components"`
	ComponentCount            int                         `json:"component_count"`
	EstimatedValue            int64                       `json:"estimated_value"`
	TilesTotal                int                         `json:"tiles_total"`
	TilesSmoothed             int                         `json:"tiles_smoothed"`
	EngravingsCount           int                         `json:"engravings_count"`
	EngravingsByQuality       []zoneValueEngravingQuality `json:"engravings_by_quality"`
	ExperimentalPersonalValue int32                       `json:"experimental_personal_value"`
}

// renderZoneValue renders the zone_value response. The estimated_value
// line and the experimental_personal_value line are deliberately labeled
// differently and never combined -- see zoneValueResponse's doc comment.
func renderZoneValue(raw []byte) string {
	var resp zoneValueResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable zone_value response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s at (%d,%d)-(%d,%d) z=%d\n", resp.TypeName, resp.X1, resp.Y1, resp.X2, resp.Y2, resp.Z)
	fmt.Fprintf(&sb, "estimated_value (DF-formula-confirmed, sum of %d furniture component%s via DFHack's Items::getValue): %d\n",
		resp.ComponentCount, plural(resp.ComponentCount), resp.EstimatedValue)
	if resp.RoomValueField != nil {
		fmt.Fprintf(&sb, "compare against noble_demands' %s minimum for any position requiring this room\n", *resp.RoomValueField)
	}
	for _, c := range resp.Components {
		item := c.ItemType
		if c.Material != "" {
			item = c.Material + " " + item
		}
		fmt.Fprintf(&sb, "  - %s (%s, quality %s): %d\n", c.BuildingType, item, c.Quality, c.Value)
	}
	fmt.Fprintf(&sb, "tiles: %d/%d smoothed\n", resp.TilesSmoothed, resp.TilesTotal)
	if resp.EngravingsCount > 0 {
		fmt.Fprintf(&sb, "engravings: %d", resp.EngravingsCount)
		tiers := make([]string, 0, len(resp.EngravingsByQuality))
		for _, eq := range resp.EngravingsByQuality {
			tiers = append(tiers, fmt.Sprintf("%d %s", eq.Count, eq.Quality))
		}
		if len(tiers) > 0 {
			fmt.Fprintf(&sb, " (%s)", strings.Join(tiers, ", "))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("engravings: none\n")
	}
	fmt.Fprintf(&sb, "UNCALIBRATED/experimental bonus number (building->getPersonalValue -- not confirmed meaningful for this DF version's zones; do not treat as a second real total): %d\n",
		resp.ExperimentalPersonalValue)
	return sb.String()
}

func registerZoneTools(srv *mcp.Server, b *Bridge) {
	type designateZoneIn struct {
		Type string `json:"type" jsonschema:"bedroom|office|tomb|dining_hall|meeting_hall|dormitory|barracks|pen|pond|archery_range|plant_gathering|water_source|dump|sand_collection|fishing_area|clay_collection|dungeon|animal_training"`
		X1   int    `json:"x1"`
		Y1   int    `json:"y1"`
		Z    int    `json:"z"`
		X2   int    `json:"x2"`
		Y2   int    `json:"y2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "designate_zone",
		Description: "Designate a civzone rectangle over EXISTING built/carved floor on one z-level — zones claim space, they don't dig it (dig with designate_dig first). Assignment support varies by type: assign_zone works for bedroom/office/tomb/dining_hall (single owner) and pen/pond/dormitory (roster); meeting_hall/archery_range/dungeon/animal_training use a different, non-unit mechanism (see assign_zone's own errors for specifics); the remaining types designate and list fine but have no confirmed DFHack assignment mechanism yet.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in designateZoneIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		zt, ok := zoneWireTypes[strings.ToLower(in.Type)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown zone type %q -- valid types: %s", in.Type, zoneTypeVocabulary())), nil, nil
		}
		res, err := b.Exec.SendZoneCommand(zt, int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2))
		what := fmt.Sprintf("designate_zone %s (%d,%d)-(%d,%d) z=%d", in.Type, in.X1, in.Y1, in.X2, in.Y2, in.Z)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type assignZoneIn struct {
		X      int `json:"x" jsonschema:"a tile inside the target zone"`
		Y      int `json:"y"`
		Z      int `json:"z"`
		UnitID int `json:"unit_id"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "assign_zone",
		Description: "Assign a unit to the zone at a tile (any tile inside the zone's footprint — see list_zones for extents). Owner-type zones (bedroom/office/tomb/dining_hall) get a single owner; roster-type zones (pen/pond/dormitory) get a unit added to the roster. Other zone types return a clear error naming the real reason (a different mechanism entirely, or genuinely not yet confirmed).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in assignZoneIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendAssignZone(int16(in.X), int16(in.Y), int16(in.Z), int32(in.UnitID))
		what := fmt.Sprintf("assign_zone (%d,%d,%d) -> unit#%d", in.X, in.Y, in.Z, in.UnitID)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type unassignZoneIn struct {
		X      int `json:"x" jsonschema:"a tile inside the target zone"`
		Y      int `json:"y"`
		Z      int `json:"z"`
		UnitID int `json:"unit_id"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unassign_zone",
		Description: "Remove a unit's assignment from the zone at a tile.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in unassignZoneIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendUnassignZone(int16(in.X), int16(in.Y), int16(in.Z), int32(in.UnitID))
		what := fmt.Sprintf("unassign_zone (%d,%d,%d) -> unit#%d", in.X, in.Y, in.Z, in.UnitID)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type listZonesIn struct {
		Type string `json:"type,omitempty" jsonschema:"optional: only list zones of this type, and show full per-zone detail instead of the type-summary default"`
		Z    *int   `json:"z,omitempty" jsonschema:"optional: only list zones on this z-level"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_zones",
		Description: "List zones. Default: summary grouped by type. Pass type (and/or z) to see full per-zone extents and owner/roster detail for that type.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listZonesIn) (*mcp.CallToolResult, any, error) {
		args := "{"
		parts := []string{}
		if in.Type != "" {
			if kind, ok := zoneWireTypes[strings.ToLower(in.Type)]; ok {
				parts = append(parts, fmt.Sprintf(`"type":%d`, kind))
			}
		}
		if in.Z != nil {
			parts = append(parts, fmt.Sprintf(`"z":%d`, *in.Z))
		}
		args += strings.Join(parts, ",") + "}"
		raw, err := b.Query(ctx, "list_zones", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		zFilter := -1
		if in.Z != nil {
			zFilter = *in.Z
		}
		return withDash(b, ctx, renderZones(raw, in.Type, zFilter)), nil, nil
	})

	type zoneValueIn struct {
		X int `json:"x" jsonschema:"a tile inside the target zone"`
		Y int `json:"y"`
		Z int `json:"z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "zone_value",
		Description: "Estimate a zone's furniture-derived value: a DF-formula-confirmed component list (each contained item's material/quality/value via DFHack's own Items::getValue) summed into estimated_value, plus smoothed-tile and real engraving counts. For bedroom/office/dining_hall/tomb, room_value_field names the noble_demands minimum this compares against. Also reports one extra UNCALIBRATED/experimental number (building->getPersonalValue) alongside estimated_value -- never treat it as a second real total.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in zoneValueIn) (*mcp.CallToolResult, any, error) {
		args := fmt.Sprintf(`{"x":%d,"y":%d,"z":%d}`, in.X, in.Y, in.Z)
		raw, err := b.Query(ctx, "zone_value", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderZoneValue(raw)), nil, nil
	})

	type removeZoneIn struct {
		X int `json:"x" jsonschema:"a tile inside the target zone"`
		Y int `json:"y"`
		Z int `json:"z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "remove_zone",
		Description: "Deconstruct the zone at a tile immediately (civzones are removed on the spot, no dwarf labor involved -- unlike remove_building). Rejected if the zone founds a Location (Tavern/Temple/Library/Guildhall/Hospital) -- unassign/retire the Location first.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in removeZoneIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendRemoveZone(int16(in.X), int16(in.Y), int16(in.Z))
		what := fmt.Sprintf("remove_zone (%d,%d,%d)", in.X, in.Y, in.Z)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})
}
