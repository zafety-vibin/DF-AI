package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

// connectorSuggestion checks whether the just-designated rectangle
// touches any existing open (already-dug) region. Returns "" when it
// does (or when nothing has been dug yet — no suggestion makes sense
// for a fort's very first designation). A neighbor inside a dig rect
// this session already ACKed (digs, nil = none) also counts as
// connected: hidden-but-designated tiles classify as Unknown in the
// topology overlay, so a room designated beside a not-yet-carved stair
// spine would otherwise false-positive as disconnected and point at
// wilderness. Never returns error/warning text: a fresh designation
// being disconnected from existing space is normal DF workflow (room
// first, corridor second), not a mistake — see design doc "Component 3".
func connectorSuggestion(topo *topology.TopologyOverlay, digs *pendingDigs, x1, y1, z1, x2, y2, z2 int16) string {
	rg := topology.BuildRegionGraph(topo)
	if len(rg.Regions) == 0 {
		return ""
	}
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	if z1 > z2 {
		z1, z2 = z2, z1
	}
	// Check every tile just outside the rectangle's boundary (one ring
	// of padding on all sides, all swept Z levels) for an open neighbor.
	for z := z1; z <= z2; z++ {
		for x := x1 - 1; x <= x2+1; x++ {
			for y := y1 - 1; y <= y2+1; y++ {
				inside := x >= x1 && x <= x2 && y >= y1 && y <= y2
				if inside {
					continue
				}
				if topo.GetTileState(x, y, z) == topology.StateOpen || digs.contains(x, y, z) {
					return "" // touches existing open space (or a pending dig) — connected
				}
			}
		}
	}
	// Vertical footprint scan: the ring above only ever checks the same
	// swept Z levels, so a designation whose ONLY connection is straight
	// up or down — a surface shaft's open top, a room dug directly over
	// or under already-open space — false-positived as disconnected.
	// Check the FOOTPRINT itself (not a padded ring) one Z above the top
	// and one Z below the bottom.
	for x := x1; x <= x2; x++ {
		for y := y1; y <= y2; y++ {
			if topo.GetTileState(x, y, z1-1) == topology.StateOpen || digs.contains(x, y, z1-1) {
				return ""
			}
			if topo.GetTileState(x, y, z2+1) == topology.StateOpen || digs.contains(x, y, z2+1) {
				return ""
			}
		}
	}
	centerX, centerY, centerZ := (x1+x2)/2, (y1+y2)/2, z1
	_, target, _, ok := rg.NearestRegion(topology.Coord{X: centerX, Y: centerY, Z: centerZ})
	if !ok {
		return ""
	}
	// The nearest actual tile in the nearest region — a heuristic
	// starting point, not a guaranteed-optimal path; the model refines it
	// with look. Must be a real tile the region's flood-fill actually
	// visited: a non-rectangular region's bounding-box corner (e.g. an
	// L-shaped corridor) is frequently NOT a member tile, which would
	// suggest a connector terminating outside the region entirely.
	switch {
	case centerZ != target.Z:
		// Any Z difference means a straight designate_dig line can't
		// reach it at all — a real DF connector needs a vertical shaft
		// (stairs dig type) plus a corridor, not a single line. Describe
		// it rather than emit a designation that's simply wrong.
		return fmt.Sprintf(
			"not yet connected to existing space — nearest open tile is at (%d,%d,%d), on a different z-level from the new designation — connect with a vertical stairway (designate_dig type=stairs) plus a corridor, not a single designate_dig line",
			target.X, target.Y, target.Z)
	case centerX == target.X || centerY == target.Y:
		// Collinear on x or y at the same z: the center and target already
		// share one axis, so a single designate_dig line is a straight
		// corridor, not a box.
		return fmt.Sprintf(
			"not yet connected to existing space — suggested connector: designate_dig default (%d,%d,%d)->(%d,%d,%d)",
			centerX, centerY, centerZ, target.X, target.Y, target.Z)
	default:
		// Same z, but neither x nor y matches: designate_dig's two
		// endpoints define a RECTANGLE, not a line — a single naive
		// suggestion here would designate a giant bounding box across
		// both axes instead of a corridor. Break it into two straight
		// legs sharing a corner tile instead.
		cornerX, cornerY := target.X, centerY
		return fmt.Sprintf(
			"not yet connected to existing space — nearest open tile is at (%d,%d,%d), diagonal from center (%d,%d,%d): a single designate_dig line would box in unwanted tiles — connect with an L-shaped corridor instead (two straight legs, not one rectangle): leg 1 designate_dig default (%d,%d,%d)->(%d,%d,%d), leg 2 designate_dig default (%d,%d,%d)->(%d,%d,%d)",
			target.X, target.Y, target.Z, centerX, centerY, centerZ,
			centerX, centerY, centerZ, cornerX, cornerY, centerZ,
			cornerX, cornerY, centerZ, target.X, target.Y, target.Z)
	}
}

// ackText renders a command result truthfully: the plugin's error text is
// the model's primary feedback and is never swallowed or softened.
func ackText(res *commands.CommandResult, err error, what string) string {
	if err != nil {
		return fmt.Sprintf("FAILED: %s — %v", what, err)
	}
	if res == nil {
		return fmt.Sprintf("FAILED: %s — no result", what)
	}
	// Severity comes from the plugin's ack STATUS byte, never inferred
	// from message presence: a SUCCESS ack carrying an informational note
	// (e.g. remove_zone's "zone removed immediately") is a clean success
	// with the note appended verbatim, not a PARTIAL.
	switch {
	case res.Status == protocol.AckStatusPartial:
		return fmt.Sprintf("PARTIAL: %s — %s", what, res.ErrorMsg)
	case res.Success && res.ErrorMsg == "":
		return fmt.Sprintf("SUCCESS: %s (ack in %s)", what, res.Duration)
	case res.Success:
		return fmt.Sprintf("SUCCESS: %s (ack in %s) — %s", what, res.Duration, res.ErrorMsg)
	default:
		return fmt.Sprintf("FAILED: %s — %s", what, res.ErrorMsg)
	}
}

// resultDetail extracts a command result's raw truthful detail text with no
// SUCCESS/PARTIAL/FAILED prefix — for callers (like queue_job's retry loop)
// that build their own single outer prefix and would otherwise double it up
// by embedding an already-prefixed ackText string inside another prefix.
func resultDetail(res *commands.CommandResult, err error) string {
	if err != nil {
		return err.Error()
	}
	if res == nil {
		return "no result"
	}
	if res.ErrorMsg != "" {
		return res.ErrorMsg
	}
	return fmt.Sprintf("ack in %s", res.Duration)
}

// buildWireCoords converts the model-facing build coordinate to the wire
// semantic. The protocol's (x,y) is a building's NW CORNER (DFHack
// allocInstance), but the tool promises CENTER for square multi-tile
// footprints — the natural way to think about placement. The corner
// offset is (footprint-1)/2 in both axes (DF centers odd footprints,
// Buildings.cpp getCorrectSize: 3x3 → center (1,1), 5x5 → center (2,2)):
// workshops and furnaces (3x3) shift by -1,-1; the trade depot (5x5,
// forced by DF regardless of requested size) shifts by -2,-2; everything
// else is 1x1 where center == corner.
func buildWireCoords(buildType uint8, x, y int) (int16, int16) {
	switch {
	case protocol.IsBuildTypeWorkshop(buildType), protocol.IsBuildTypeFurnace(buildType):
		return int16(x - 1), int16(y - 1)
	case protocol.IsBuildTypeDepot(buildType):
		return int16(x - 2), int16(y - 2)
	default:
		return int16(x), int16(y)
	}
}

// buildFootprintCorner is buildWireCoords generalized to an arbitrary WxH
// footprint (bridge's width/height are caller-chosen, unlike the fixed 3x3
// workshop/furnace or forced 5x5 depot footprints buildWireCoords already
// special-cases). DF centers a rectangle-shaped building at center=size/2
// (integer division) — Buildings.cpp's getCorrectSize groups Bridge with
// FarmPlot/Stockpile/Civzone/RoadDirt/RoadPaved under exactly that formula
// — so the NW corner is x - width/2, y - height/2. For odd dimensions this
// is numerically identical to buildWireCoords' "-1,-1"/"-2,-2" cases
// (3/2==1, 5/2==2); it also handles even dimensions correctly, which those
// two fixed cases never needed to.
func buildFootprintCorner(x, y, width, height int) (int16, int16) {
	return int16(x - width/2), int16(y - height/2)
}

// digTypeFromName maps a model-facing (or blueprint-CSV-facing) dig type
// name to the wire DigType byte. Two name families feed this: the
// designate_dig tool's own vocabulary (default|stairs|channel|ramp|
// upstair|downstair) and parseQuickfortGrid's snake_case output
// (updown_stair|down_stair|up_stair|remove_ramp) — every quickfort "i/j/u"
// stair cell in every shipped blueprints/*.csv used to fail silently here
// because only the tool's own names were accepted; the aliases below are
// the fix.
func digTypeFromName(s string) (uint8, error) {
	switch strings.ToLower(s) {
	case "default", "dig", "mine":
		return protocol.DigTypeDefault, nil
	case "stairs", "updown_stair":
		return protocol.DigTypeUpDownStair, nil
	case "channel":
		return protocol.DigTypeChannel, nil
	case "ramp":
		return protocol.DigTypeRamp, nil
	case "downstair", "down_stair":
		return protocol.DigTypeDownStair, nil
	case "upstair", "up_stair":
		return protocol.DigTypeUpStair, nil
	case "remove_ramp":
		// quickfort's "x" cell (ramp/stair removal). There is no
		// df::tile_dig_designation value on the wire for this — see
		// internal/protocol's DigType constants (Default/UpDownStair/
		// Channel/Ramp/DownStair/UpStair only) and dfhack-plugin's
		// designations.cpp switch, which has no case for it either.
		// Fail loudly instead of silently dropping the tile or
		// guessing a substitute designation.
		return 0, fmt.Errorf("remove_ramp is not on the wire protocol yet (no tile_dig_designation value for ramp/stair removal is exposed) — this blueprint tile is reported as failed, not silently skipped")
	}
	return 0, fmt.Errorf("unknown dig type %q (default|stairs|channel|ramp|upstair|downstair|updown_stair|down_stair|up_stair|remove_ramp)", s)
}

var buildTypes = map[string]uint8{
	"wall": protocol.BuildTypeWall, "floor": protocol.BuildTypeFloor,
	"upstair": protocol.BuildTypeUpStair, "downstair": protocol.BuildTypeDownStair,
	"updownstair": protocol.BuildTypeUpDownStair, "ramp": protocol.BuildTypeRamp,
	"carpenter": protocol.BuildTypeWorkshopCarpenter, "mason": protocol.BuildTypeWorkshopMason,
	"still": protocol.BuildTypeWorkshopStill, "farmer": protocol.BuildTypeWorkshopFarmer,
	"craftsdwarf": protocol.BuildTypeWorkshopCraftsdwarf, "mechanic": protocol.BuildTypeWorkshopMechanic,
	"butcher": protocol.BuildTypeWorkshopButcher, "kitchen": protocol.BuildTypeWorkshopKitchen,
	"fishery": protocol.BuildTypeWorkshopFishery, "metalsmith": protocol.BuildTypeWorkshopMetalsmith,
	"bed": protocol.BuildTypeBed, "table": protocol.BuildTypeTable,
	"chair": protocol.BuildTypeChair, "cabinet": protocol.BuildTypeCabinet,
	"coffer": protocol.BuildTypeCoffer,
	"door": protocol.BuildTypeDoor, "hatch": protocol.BuildTypeHatch,
	"lever": protocol.BuildTypeLever, "floodgate": protocol.BuildTypeFloodgate,
	"smelter": protocol.BuildTypeFurnaceSmelter, "wood_furnace": protocol.BuildTypeFurnaceWood,
	"tradedepot": protocol.BuildTypeTradeDepot,
}

// bridgeDirections maps the build tool's model-facing bridge direction
// name to the wire BridgeDirection* byte (protocol.go). "bridge" is
// deliberately NOT in buildTypes above: it has no fixed BuildType byte at
// all (it needs a rectangle + direction, not a single-tile BUILD command
// — see the build tool's handler) and is dispatched separately.
var bridgeDirections = map[string]uint8{
	"retract": protocol.BridgeDirectionRetract,
	"raise_n": protocol.BridgeDirectionRaiseN,
	"raise_s": protocol.BridgeDirectionRaiseS,
	"raise_e": protocol.BridgeDirectionRaiseE,
	"raise_w": protocol.BridgeDirectionRaiseW,
}

var orderTypes = map[string]uint8{
	"bed": protocol.OrderTypeMakeBed, "table": protocol.OrderTypeMakeTable,
	"chair": protocol.OrderTypeMakeChair, "door": protocol.OrderTypeMakeDoor,
	"barrel": protocol.OrderTypeMakeBarrel, "bucket": protocol.OrderTypeMakeBucket,
	"cabinet": protocol.OrderTypeMakeCabinet, "coffer": protocol.OrderTypeMakeCoffer,
	"drink": protocol.OrderTypeBrewDrink, "meal": protocol.OrderTypePrepareMeal,
	"blocks": protocol.OrderTypeMakeBlocks, "crafts": protocol.OrderTypeMakeCrafts,
}

// laborNames maps a model-facing labor name to the wire LaborID
// (protocol.Labor* — the real df::unit_labor enum index). Deliberately a
// curated subset relevant to a fresh 7-dwarf fort, not DF's full 94-entry
// labor list.
var laborNames = map[string]uint8{
	"mine":            protocol.LaborMine,
	"cutwood":         protocol.LaborCutwood,
	"carpenter":       protocol.LaborCarpenter,
	"stonecutter":     protocol.LaborStonecutter,
	"stone_carver":    protocol.LaborStoneCarver,
	"engrave":         protocol.LaborEngraver,
	"mason":           protocol.LaborMason,
	"brew":            protocol.LaborBrewer,
	"cook":            protocol.LaborCook,
	"plant":           protocol.LaborPlant,
	"herbalist":       protocol.LaborHerbalist,
	"fish":            protocol.LaborFish,
	"smelt":           protocol.LaborSmelt,
	"forge_weapon":    protocol.LaborForgeWeapon,
	"forge_armor":     protocol.LaborForgeArmor,
	"forge_furniture": protocol.LaborForgeFurniture,
	"metalcraft":      protocol.LaborMetalCraft,
	"mechanic":        protocol.LaborMechanic,
	"haul_stone":      protocol.LaborHaulStone,
	"haul_wood":       protocol.LaborHaulWood,
	"haul_food":       protocol.LaborHaulFood,
	"haul_item":       protocol.LaborHaulItem,
	"haul_furniture":  protocol.LaborHaulFurniture,
}

var materialClasses = map[string]uint8{
	"any":    protocol.MaterialClassAny,
	"wood":   protocol.MaterialClassWood,
	"stone":  protocol.MaterialClassStone,
	"blocks": protocol.MaterialClassBlocks,
}

var stockpileGroups = map[string]uint32{
	"all":            protocol.StockpileGroupAll,
	"animals":        protocol.StockpileGroupAnimals,
	"food":           protocol.StockpileGroupFood,
	"furniture":      protocol.StockpileGroupFurniture,
	"corpses":        protocol.StockpileGroupCorpses,
	"refuse":         protocol.StockpileGroupRefuse,
	"stone":          protocol.StockpileGroupStone,
	"ammo":           protocol.StockpileGroupAmmo,
	"coins":          protocol.StockpileGroupCoins,
	"bars_blocks":    protocol.StockpileGroupBarsBlocks,
	"gems":           protocol.StockpileGroupGems,
	"finished_goods": protocol.StockpileGroupFinishedGoods,
	"leather":        protocol.StockpileGroupLeather,
	"cloth":          protocol.StockpileGroupCloth,
	"wood":           protocol.StockpileGroupWood,
	"weapons":        protocol.StockpileGroupWeapons,
	"armor":          protocol.StockpileGroupArmor,
	"sheet":          protocol.StockpileGroupSheet,
}

// noExec returns a scaffold-safe result when the bridge (or its executor)
// is absent — New() documents that a nil bridge reports the missing
// connection instead of panicking. Returns nil when commands can be sent.
func noExec(b *Bridge) *mcp.CallToolResult {
	if b == nil || b.Exec == nil {
		return TextResult("NOT CONNECTED: start DF, then run `ai-connect` in the DFHack console.")
	}
	return nil
}

func registerActionTools(srv *mcp.Server, b *Bridge) {
	type digIn struct {
		Type string `json:"type" jsonschema:"default|stairs|channel|ramp|upstair|downstair"`
		X1   int    `json:"x1"`
		Y1   int    `json:"y1"`
		Z1   int    `json:"z1"`
		X2   int    `json:"x2"`
		Y2   int    `json:"y2"`
		Z2   int    `json:"z2" jsonschema:"for stairs, the ending z (deep!); for flat digs same as z1"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "designate_dig",
		Description: "Designate digging over a 3D rectangle. Hidden fog tiles designate fine (that's how forts are dug). 'stairs' spans z1..z2 as one shaft (2x2 recommended, surface to deep stone in ONE call); a stairs range whose top adjoins existing carved stairs joins the shaft, and already-carved tiles are skipped — so extending a shaft deeper is safe. Dwarves with picks do the work over game time — step() to let it happen. If the designated area isn't yet connected to existing dug space, the ACK suggests a connector — this is informational, not an error: designating a room before its corridor is normal.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in digIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		dt, err := digTypeFromName(in.Type)
		if err != nil {
			return withDash(b, ctx, err.Error()), nil, nil
		}
		res, err := b.Exec.SendDigRegion(dt, int16(in.X1), int16(in.Y1), int16(in.Z1), int16(in.X2), int16(in.Y2), int16(in.Z2))
		what := fmt.Sprintf("dig %s (%d,%d,%d)->(%d,%d,%d)", in.Type, in.X1, in.Y1, in.Z1, in.X2, in.Y2, in.Z2)
		ack := ackText(res, err, what)
		// Reachability guidance: only meaningful after a successful dig
		// designation, and only against a live topology overlay.
		if err == nil && res != nil && res.Success {
			if topo := b.Topo(); topo != nil {
				if suggestion := connectorSuggestion(topo, b.Digs, int16(in.X1), int16(in.Y1), int16(in.Z1), int16(in.X2), int16(in.Y2), int16(in.Z2)); suggestion != "" {
					ack = ack + "\n" + suggestion
				}
			}
			// Recorded AFTER the suggestion so a designation can't count
			// itself as its own connection.
			b.Digs.add(int16(in.X1), int16(in.Y1), int16(in.Z1), int16(in.X2), int16(in.Y2), int16(in.Z2))
		}
		return withDash(b, ctx, ack), nil, nil
	})

	// Chop, gather, and cancel share the plugin's single-Z rectangle
	// commands (executor.go: (x1,y1,z,x2,y2)) — the input mirrors that
	// instead of exposing an unusable z2.
	type rectZIn struct {
		X1 int `json:"x1"`
		Y1 int `json:"y1"`
		Z  int `json:"z"`
		X2 int `json:"x2"`
		Y2 int `json:"y2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "chop",
		Description: "Designate tree felling over a rectangle on one z-level (trees are 'T' in look crops). Designates the map tiles directly — no console dependency. Dwarves with axes fell them over game time — this starts the wood chain.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in rectZIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendChopCommand(int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("chop (%d,%d)-(%d,%d) z=%d", in.X1, in.Y1, in.X2, in.Y2, in.Z))), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "gather",
		Description: "Designate plant gathering over a rectangle on one z-level (harvest wild shrubs and berries for food without farming). Designates the map tiles directly — no console dependency.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in rectZIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendGatherCommand(int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("gather (%d,%d)-(%d,%d) z=%d", in.X1, in.Y1, in.X2, in.Y2, in.Z))), nil, nil
	})

	type buildIn struct {
		Type      string `json:"type" jsonschema:"workshop (carpenter|mason|still|farmer|craftsdwarf|mechanic|butcher|kitchen|fishery|metalsmith), furnace (smelter|wood_furnace), tradedepot, furniture (bed|table|chair|cabinet|coffer), door|hatch|lever|floodgate, bridge, or construction (wall|floor|upstair|downstair|updownstair|ramp)"`
		X         int    `json:"x" jsonschema:"CENTER of the footprint: workshops/furnaces are 3x3, tradedepot is 5x5, bridge is width x height (surrounding tiles must be clear floor)"`
		Y         int    `json:"y"`
		Z         int    `json:"z"`
		Material  string `json:"material,omitempty" jsonschema:"any|wood|stone|blocks — constrains the item CLASS claimed for the build (default any); ignored for bridge"`
		Width     int    `json:"width,omitempty" jsonschema:"bridge only (required): footprint width, x-span"`
		Height    int    `json:"height,omitempty" jsonschema:"bridge only (required): footprint height, y-span"`
		Direction string `json:"direction,omitempty" jsonschema:"bridge only (required): retract|raise_n|raise_s|raise_e|raise_w — which way the bridge lifts when raised (retract slides away instead)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "build",
		Description: "Place a building. Workshops/furnaces are 3x3, tradedepot is 5x5, bridge is width x height (x,y = center; surrounding tiles must be clear floor). Furniture/lever/floodgate need the item in a stockpile first (order or queue_job it — lever needs 1 mechanism, floodgate needs 1 floodgate item). Constructions need blocks/boulders. metalsmith needs an ANVIL item (craft or buy one) plus a fire-safe building material; smelter/wood_furnace need a fire-safe boulder; tradedepot needs 3x any building material; bridge needs building material scaled to its footprint (DF computes the amount) and takes width/height/direction instead of material. Optional material (any|wood|stone|blocks) constrains which item CLASS gets used — wood=logs, stone=boulders, blocks=blocks — but DF's job system still picks the specific item within that class. A lever must be link_building'd to a target before pull_lever has any effect.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in buildIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		if strings.ToLower(in.Type) == "bridge" {
			if in.Width <= 0 || in.Height <= 0 {
				return withDash(b, ctx, "bridge requires width and height > 0"), nil, nil
			}
			// DF hard-caps bridges at 31x31 (its own UI; DFHack's quickfort
			// enforces the same limit) -- reject early rather than round-trip
			// to the plugin, which also enforces this as the authoritative check.
			if in.Width > 31 || in.Height > 31 {
				return withDash(b, ctx, "bridge exceeds DF's maximum size of 31x31 tiles"), nil, nil
			}
			dir, ok := bridgeDirections[strings.ToLower(in.Direction)]
			if !ok {
				return withDash(b, ctx, fmt.Sprintf("unknown bridge direction %q (retract|raise_n|raise_s|raise_e|raise_w)", in.Direction)), nil, nil
			}
			x1, y1 := buildFootprintCorner(in.X, in.Y, in.Width, in.Height)
			x2 := x1 + int16(in.Width) - 1
			y2 := y1 + int16(in.Height) - 1
			res, err := b.Exec.SendBuildBridge(x1, y1, int16(in.Z), x2, y2, dir)
			what := fmt.Sprintf("build bridge %dx%d at center (%d,%d,%d) direction=%s", in.Width, in.Height, in.X, in.Y, in.Z, strings.ToLower(in.Direction))
			return withDash(b, ctx, ackText(res, err, what)), nil, nil
		}
		bt, ok := buildTypes[strings.ToLower(in.Type)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown build type %q", in.Type)), nil, nil
		}
		mat := protocol.MaterialClassAny
		if in.Material != "" {
			mat, ok = materialClasses[strings.ToLower(in.Material)]
			if !ok {
				return withDash(b, ctx, fmt.Sprintf("unknown material %q (any|wood|stone|blocks)", in.Material)), nil, nil
			}
		}
		wx, wy := buildWireCoords(bt, in.X, in.Y)
		res, err := b.Exec.SendBuildCommandWithMaterial(wx, wy, int16(in.Z), bt, mat)
		what := fmt.Sprintf("build %s at (%d,%d,%d)", in.Type, in.X, in.Y, in.Z)
		if mat != protocol.MaterialClassAny {
			what += " [" + strings.ToLower(in.Material) + "]"
		}
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type stockIn struct {
		Category string `json:"category" jsonschema:"all|animals|food|furniture|corpses|refuse|stone|ammo|coins|bars_blocks|gems|finished_goods|leather|cloth|wood|weapons|armor|sheet"`
		X1       int    `json:"x1"`
		Y1       int    `json:"y1"`
		Z        int    `json:"z"`
		X2       int    `json:"x2"`
		Y2       int    `json:"y2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "stockpile",
		Description: "Designate a stockpile rectangle accepting a category (or 'all'). Must be on open floor tiles.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in stockIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		mask, ok := stockpileGroups[strings.ToLower(in.Category)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown stockpile category %q", in.Category)), nil, nil
		}
		res, err := b.Exec.SendStockpileCommand(int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2), mask)
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("stockpile %s (%d,%d)-(%d,%d) z=%d", in.Category, in.X1, in.Y1, in.X2, in.Y2, in.Z))), nil, nil
	})

	type orderIn struct {
		Item  string `json:"item" jsonschema:"bed|table|chair|door|barrel|bucket|cabinet|coffer|drink|meal|blocks|crafts"`
		Count int    `json:"count" jsonschema:"how many to queue (start small: 1-3)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "order",
		Description: "Queue a manager work order (needs a manager noble + office to dispatch; the matching workshop must exist). E.g. order bed x2 after building a carpenter workshop.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in orderIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		ot, ok := orderTypes[strings.ToLower(in.Item)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown order item %q", in.Item)), nil, nil
		}
		res, err := b.Exec.SendWorkOrderCommand(ot, uint16(in.Count))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("order %dx %s", in.Count, in.Item))), nil, nil
	})

	type queueJobIn struct {
		X        int    `json:"x" jsonschema:"target workshop's tile (any tile of its footprint)"`
		Y        int    `json:"y"`
		Z        int    `json:"z"`
		Item     string `json:"item,omitempty" jsonschema:"known short vocabulary: bed|table|chair|door|barrel|bucket|cabinet|coffer|blocks (drink/meal/crafts are NOT supported here — drink has no direct job_type mapping; meal/crafts resolve fine but the plugin rejects them, no verified material filter yet — use the order tool for all three instead) — OR any DFHack job_type enum name (e.g. ConstructHatchCover) for anything not in that list; look one up with the job_types tool. Mutually exclusive with reaction — set exactly one."`
		Reaction string `json:"reaction,omitempty" jsonschema:"reaction code e.g. BREW_DRINK_FROM_PLANT — discover via list_reactions; requires plugin rebuild to take effect. Mutually exclusive with item — set exactly one."`
		Count    int    `json:"count" jsonschema:"how many jobs to queue, one at a time (default 1)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "queue_job",
		Description: "Queue a job directly at an existing workshop — no manager noble or office needed (unlike order, which needs both). Use this for an immediate one-off need; use order for standing/bulk production once a manager exists. Pass exactly one of item (job-type vocabulary) or reaction (raw reaction code, e.g. for brewing — see list_reactions).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in queueJobIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		hasItem := in.Item != ""
		hasReaction := in.Reaction != ""
		if hasItem == hasReaction {
			return withDash(b, ctx, "queue_job requires exactly one of item or reaction, not both/neither"), nil, nil
		}

		var ot uint8
		var wireName string
		var label string
		if hasReaction {
			ot = protocol.OrderTypeCustomReaction
			wireName = in.Reaction
			label = "reaction " + in.Reaction
		} else {
			// The short vocabulary covers the common cases with no wire
			// round-trip name lookup; anything else falls through to the
			// plugin's generalized name-based resolution (protocol.OrderTypeByName
			// — see work_orders.cpp resolveJobTypeByName) using the caller's
			// string VERBATIM (not lowercased): DFHack job_type keys are
			// case-sensitive CamelCase, e.g. "ConstructHatchCover".
			known, ok := orderTypes[strings.ToLower(in.Item)]
			if ok {
				ot = known
			} else {
				ot = protocol.OrderTypeByName
				wireName = in.Item
			}
			label = in.Item
		}
		count := in.Count
		if count <= 0 {
			count = 1
		}
		queued := 0
		var lastErr string
		for i := 0; i < count; i++ {
			res, err := b.Exec.SendQueueJob(int16(in.X), int16(in.Y), int16(in.Z), ot, wireName)
			if err == nil && res != nil && res.Success && res.ErrorMsg == "" {
				queued++
				continue
			}
			// resultDetail (not ackText) here: the switch below already adds
			// the single FAILED:/PARTIAL: outer prefix — using ackText would
			// double it up (e.g. "FAILED: ... - FAILED: ..."), per review.
			lastErr = resultDetail(res, err)
			break // stop on first failure (queue full, wrong workshop, unrecognized name, etc.) — don't spam retries
		}
		what := fmt.Sprintf("queue %dx %s job at (%d,%d,%d)", count, label, in.X, in.Y, in.Z)
		switch {
		case queued == count:
			return withDash(b, ctx, fmt.Sprintf("SUCCESS: %s (%d/%d queued)", what, queued, count)), nil, nil
		case queued > 0:
			return withDash(b, ctx, fmt.Sprintf("PARTIAL: %s — %d/%d queued, then: %s", what, queued, count, lastErr)), nil, nil
		default:
			return withDash(b, ctx, fmt.Sprintf("FAILED: %s — %s", what, lastErr)), nil, nil
		}
	})

	type setLaborIn struct {
		ID     int    `json:"id" jsonschema:"the dwarf's id from the dwarves tool"`
		Labor  string `json:"labor" jsonschema:"mine|cutwood|carpenter|stonecutter|stone_carver|engrave|mason|brew|cook|plant|herbalist|fish|smelt|forge_weapon|forge_armor|forge_furniture|metalcraft|mechanic|haul_stone|haul_wood|haul_food|haul_item|haul_furniture"`
		Enable bool   `json:"enable" jsonschema:"true to enable the labor, false to disable it"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_labor",
		Description: "Enable or disable one labor on a dwarf. DF's own labor system then assigns matching jobs to whichever dwarf has that labor on — this doesn't queue work directly. Check dwarf_detail first to see a dwarf's current labors before changing them. A labor a dwarf's caste can't perform is a harmless no-op: DF just never generates matching work for that dwarf.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in setLaborIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		lid, ok := laborNames[strings.ToLower(in.Labor)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown labor %q", in.Labor)), nil, nil
		}
		res, err := b.Exec.SendSetLabor(int32(in.ID), lid, in.Enable)
		what := fmt.Sprintf("set_labor %s=%v for dwarf id=%d", in.Labor, in.Enable, in.ID)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type xyzIn struct {
		X int `json:"x"`
		Y int `json:"y"`
		Z int `json:"z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unsuspend",
		Description: "Resume a suspended construction at a tile (after fixing its blocker: materials, path, occupant).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in xyzIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendUnsuspendCommand(int16(in.X), int16(in.Y), int16(in.Z))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("unsuspend (%d,%d,%d)", in.X, in.Y, in.Z))), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "remove_building",
		Description: "Mark the building at a tile for removal (deconstruction). Any tile of a multi-tile building's footprint works. Dwarves do the teardown over game time and reclaim the materials — step() and check buildings to confirm it's gone. If a stockpile's rectangle overlaps a real building at that tile (stockpiles have no hole punched out for enclosed buildings), the call returns a FAILED ack listing all candidates instead of guessing — removing the stockpile first is destructive to its WHOLE rectangle, not just the shared tiles, so read the ack text before reissuing.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in xyzIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendRemoveBuilding(int16(in.X), int16(in.Y), int16(in.Z))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("remove building at (%d,%d,%d)", in.X, in.Y, in.Z))), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "pull_lever",
		Description: "Queue DF's real PullLever job against the built lever at a tile. The lever must already be built and, for the pull to do anything, link_building'd to a target (bridge/floodgate/door/hatch). Dwarves do the actual pulling over game time — step() to let it happen.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in xyzIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendPullLever(int16(in.X), int16(in.Y), int16(in.Z))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("pull lever at (%d,%d,%d)", in.X, in.Y, in.Z))), nil, nil
	})

	type linkBuildingIn struct {
		LeverX  int `json:"lever_x"`
		LeverY  int `json:"lever_y"`
		LeverZ  int `json:"lever_z"`
		TargetX int `json:"target_x" jsonschema:"any tile of the target building's footprint"`
		TargetY int `json:"target_y"`
		TargetZ int `json:"target_z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "link_building",
		Description: "Wire a built lever to a trigger target (bridge, floodgate, door, or hatch) so pull_lever operates it. Consumes two free mechanism items (craft with queue_job item=ConstructMechanisms at a mechanic workshop) — fails truthfully if fewer than two are available, if either building is still under construction, or if the target type isn't supported. Both buildings must already be built.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in linkBuildingIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendLinkBuilding(int16(in.LeverX), int16(in.LeverY), int16(in.LeverZ), int16(in.TargetX), int16(in.TargetY), int16(in.TargetZ))
		what := fmt.Sprintf("link lever (%d,%d,%d) -> target (%d,%d,%d)", in.LeverX, in.LeverY, in.LeverZ, in.TargetX, in.TargetY, in.TargetZ)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "cancel_designation",
		Description: "Clear dig designations in a rectangle on one z-level (undo a mistaken dig order).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in rectZIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendCancelCommand(int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("cancel designations (%d,%d)-(%d,%d) z=%d", in.X1, in.Y1, in.X2, in.Y2, in.Z))), nil, nil
	})

	type smoothIn struct {
		Mode string `json:"mode" jsonschema:"smooth|engrave (engrave requires smoothed first; natural stone only)"`
		X1   int    `json:"x1"`
		Y1   int    `json:"y1"`
		Z    int    `json:"z"`
		X2   int    `json:"x2"`
		Y2   int    `json:"y2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "smooth",
		Description: "Smooth or engrave natural stone in a rectangle (soil is silently skipped by DF).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in smoothIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		st := protocol.SmoothTypeSmooth
		if strings.ToLower(in.Mode) == "engrave" {
			st = protocol.SmoothTypeEngrave
		}
		res, err := b.Exec.SendSmoothCommand(st, int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("%s (%d,%d)-(%d,%d) z=%d", in.Mode, in.X1, in.Y1, in.X2, in.Y2, in.Z))), nil, nil
	})

	type bpIn struct {
		Name    string `json:"name" jsonschema:"blueprint name from the list shown on error/empty name"`
		OriginX int    `json:"origin_x" jsonschema:"absolute x where the blueprint's (0,0,0) lands"`
		OriginY int    `json:"origin_y"`
		OriginZ int    `json:"origin_z"`
		DryRun  bool   `json:"dry_run,omitempty" jsonschema:"true = preview only (ALWAYS dry-run first)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "apply_blueprint",
		Description: "Apply a dig blueprint from the library at an origin. ALWAYS dry_run=true first: it reports how many tiles would carve solid ground vs hit open space. Empty name lists available blueprints. blueprints/*.csv is re-scanned on every call, so a CSV you hand-authored or captured with save_blueprint this session is picked up without restarting the server. Quickfort 'remove ramp' cells (dig_type remove_ramp) aren't on the wire protocol yet — those tiles report as a specific failure, not silence.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in bpIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		if in.Name == "" {
			return withDash(b, ctx, "available blueprints: "+strings.Join(listBlueprints(b.Blueprints), ", ")), nil, nil
		}
		origin := modifications.Coordinate{X: int16(in.OriginX), Y: int16(in.OriginY), Z: int16(in.OriginZ)}
		cmds, err := expandBlueprint(b.Blueprints, in.Name, origin)
		if err != nil {
			return withDash(b, ctx, err.Error()), nil, nil
		}
		if in.DryRun {
			return withDash(b, ctx, summarizeDryRun(b.Topo(), cmds)), nil, nil
		}
		ok, fail, firstErr := applyBlueprintCmds(ctx, b, cmds)
		body := fmt.Sprintf("applied %q: %d tiles designated, %d failed", in.Name, ok, fail)
		if firstErr != "" {
			body += " — first failure: " + firstErr
		}
		return withDash(b, ctx, body), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_blueprints",
		Description: "List dig blueprints available in the library. Re-scans blueprints/*.csv on every call (see apply_blueprint) so hand-authored or freshly captured (save_blueprint) CSVs show up without restarting the server.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		if b == nil || b.Blueprints == nil {
			return TextResult("blueprint library not available (bridge not initialized)"), nil, nil
		}
		names := listBlueprints(b.Blueprints)
		if len(names) == 0 {
			return withDash(b, ctx, "no blueprints available (blueprints/ is empty)"), nil, nil
		}
		return withDash(b, ctx, "available blueprints: "+strings.Join(names, ", ")), nil, nil
	})

	type saveBpIn struct {
		Name string `json:"name" jsonschema:"name for the new blueprint (becomes blueprints/<name>.csv; re-usable via apply_blueprint)"`
		X1   int    `json:"x1"`
		Y1   int    `json:"y1"`
		Z1   int    `json:"z1"`
		X2   int    `json:"x2"`
		Y2   int    `json:"y2"`
		Z2   int    `json:"z2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "save_blueprint",
		Description: "Capture the dig modifications this session made inside a 3D region as a named blueprint CSV under blueprints/ — design a pod once, capture it, re-stamp it elsewhere with apply_blueprint. Only tiles this session actually DUG are captured (built walls/floors, smoothing, etc are not); every captured tile is recorded as dig_type 'default' today (inferring stairs/ramps/channels from tile shape is a known gap), so re-designate those by hand after re-applying if the source pod had any.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in saveBpIn) (*mcp.CallToolResult, any, error) {
		if b == nil || b.WM == nil || b.WM.Observed.Modifications == nil {
			return TextResult("NOT CONNECTED: start DF, then run `ai-connect` in the DFHack console — no modification history to capture yet."), nil, nil
		}
		region := normalizeRegion(int16(in.X1), int16(in.Y1), int16(in.Z1), int16(in.X2), int16(in.Y2), int16(in.Z2))
		count, path, err := saveBlueprint(b, in.Name, region)
		if err != nil {
			return withDash(b, ctx, fmt.Sprintf("save_blueprint %q failed: %v", in.Name, err)), nil, nil
		}
		return withDash(b, ctx, fmt.Sprintf("captured blueprint %q: %d tiles written to %s — re-apply with apply_blueprint{name:%q, origin_x, origin_y, origin_z}", in.Name, count, path, in.Name)), nil, nil
	})
}
