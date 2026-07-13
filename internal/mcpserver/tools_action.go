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
// for a fort's very first designation). Never returns error/warning
// text: a fresh designation being disconnected from existing space is
// normal DF workflow (room first, corridor second), not a mistake — see
// design doc "Component 3".
func connectorSuggestion(topo *topology.TopologyOverlay, x1, y1, z1, x2, y2, z2 int16) string {
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
				if topo.GetTileState(x, y, z) == topology.StateOpen {
					return "" // touches existing open space — connected
				}
			}
		}
	}
	centerX, centerY, centerZ := (x1+x2)/2, (y1+y2)/2, z1
	_, target, _, ok := rg.NearestRegion(topology.Coord{X: centerX, Y: centerY, Z: centerZ})
	if !ok {
		return ""
	}
	// Suggest a 1-wide connector from the rectangle's center toward the
	// nearest actual tile in the nearest region — a heuristic starting
	// point, not a guaranteed-optimal path; the model refines it with
	// look. Must be a real tile the region's flood-fill actually visited:
	// a non-rectangular region's bounding-box corner (e.g. an L-shaped
	// corridor) is frequently NOT a member tile, which would suggest a
	// connector terminating outside the region entirely.
	return fmt.Sprintf(
		"not yet connected to existing space — suggested connector: designate_dig default (%d,%d,%d)->(%d,%d,%d)",
		centerX, centerY, centerZ, target.X, target.Y, target.Z)
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
	switch {
	case res.Success && res.ErrorMsg == "":
		return fmt.Sprintf("SUCCESS: %s (ack in %s)", what, res.Duration)
	case res.Success:
		return fmt.Sprintf("PARTIAL: %s — %s", what, res.ErrorMsg)
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
// allocInstance), but the tool promises CENTER for 3x3 workshops — the
// natural way to think about placement. Workshops (0x10-0x2F) shift by
// -1,-1; everything else is 1x1 where center == corner.
func buildWireCoords(buildType uint8, x, y int) (int16, int16) {
	if buildType >= 0x10 && buildType < 0x30 {
		return int16(x - 1), int16(y - 1)
	}
	return int16(x), int16(y)
}

func digTypeFromName(s string) (uint8, error) {
	switch strings.ToLower(s) {
	case "default", "dig", "mine":
		return protocol.DigTypeDefault, nil
	case "stairs":
		return protocol.DigTypeUpDownStair, nil
	case "channel":
		return protocol.DigTypeChannel, nil
	case "ramp":
		return protocol.DigTypeRamp, nil
	case "downstair":
		return protocol.DigTypeDownStair, nil
	case "upstair":
		return protocol.DigTypeUpStair, nil
	}
	return 0, fmt.Errorf("unknown dig type %q (default|stairs|channel|ramp|upstair|downstair)", s)
}

var zoneTypes = map[string]uint8{
	"bedroom": protocol.ZoneTypeBedroom, "dining": protocol.ZoneTypeDining,
	"meeting": protocol.ZoneTypeMeetingHall, "barracks": protocol.ZoneTypeBarracks,
	"dormitory": protocol.ZoneTypeDormitory, "farm": protocol.ZoneTypeFarm,
	"pen": protocol.ZoneTypePen, "garbage": protocol.ZoneTypeGarbageDump,
	"pit": protocol.ZoneTypePitPond, "water": protocol.ZoneTypeWaterSource,
	"fishing": protocol.ZoneTypeFishing, "hospital": protocol.ZoneTypeHospital,
	"animal_train": protocol.ZoneTypeAnimalTrain, "tomb": protocol.ZoneTypeTomb,
}

var buildTypes = map[string]uint8{
	"wall": protocol.BuildTypeWall, "floor": protocol.BuildTypeFloor,
	"upstair": protocol.BuildTypeUpStair, "downstair": protocol.BuildTypeDownStair,
	"updownstair": protocol.BuildTypeUpDownStair, "ramp": protocol.BuildTypeRamp,
	"carpenter": protocol.BuildTypeWorkshopCarpenter, "mason": protocol.BuildTypeWorkshopMason,
	"still": protocol.BuildTypeWorkshopStill, "farmer": protocol.BuildTypeWorkshopFarmer,
	"craftsdwarf": protocol.BuildTypeWorkshopCraftsdwarf, "mechanic": protocol.BuildTypeWorkshopMechanic,
	"butcher": protocol.BuildTypeWorkshopButcher, "kitchen": protocol.BuildTypeWorkshopKitchen,
	"fishery": protocol.BuildTypeWorkshopFishery,
	"bed":     protocol.BuildTypeBed, "table": protocol.BuildTypeTable,
	"chair": protocol.BuildTypeChair, "cabinet": protocol.BuildTypeCabinet,
	"coffer": protocol.BuildTypeCoffer,
	"door":   protocol.BuildTypeDoor, "hatch": protocol.BuildTypeHatch,
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
				if suggestion := connectorSuggestion(topo, int16(in.X1), int16(in.Y1), int16(in.Z1), int16(in.X2), int16(in.Y2), int16(in.Z2)); suggestion != "" {
					ack = ack + "\n" + suggestion
				}
			}
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
		Type     string `json:"type" jsonschema:"workshop (carpenter|mason|still|farmer|craftsdwarf|mechanic|butcher|kitchen|fishery), furniture (bed|table|chair|cabinet|coffer), door|hatch, or construction (wall|floor|upstair|downstair|updownstair|ramp)"`
		X        int    `json:"x" jsonschema:"for workshops this is the CENTER of the 3x3 footprint"`
		Y        int    `json:"y"`
		Z        int    `json:"z"`
		Material string `json:"material,omitempty" jsonschema:"any|wood|stone|blocks — constrains the item CLASS claimed for the build (default any)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "build",
		Description: "Place a building. Workshops are 3x3 (x,y = center; surrounding 8 tiles must be clear floor). Furniture needs the item in a stockpile first (order it). Constructions need blocks/boulders. Optional material (any|wood|stone|blocks) constrains which item CLASS gets used — wood=logs, stone=boulders, blocks=blocks — but DF's job system still picks the specific item within that class.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in buildIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
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

	type zoneIn struct {
		Type string `json:"type" jsonschema:"bedroom|dining|meeting|barracks|dormitory|farm|pen|garbage|pit|water|fishing|hospital|animal_train|tomb"`
		X1   int    `json:"x1"`
		Y1   int    `json:"y1"`
		Z    int    `json:"z"`
		X2   int    `json:"x2"`
		Y2   int    `json:"y2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "zone",
		Description: "Designate an activity zone rectangle on one z-level (bedroom/dining/farm/...). NOT YET FUNCTIONAL: the plugin's zone support is a stub in this build, so this command WILL return an error. Plan with dig + build + stockpile instead; zone assignment lands in a later phase.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in zoneIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		zt, ok := zoneTypes[strings.ToLower(in.Type)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown zone type %q", in.Type)), nil, nil
		}
		res, err := b.Exec.SendZoneCommand(zt, int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("zone %s (%d,%d)-(%d,%d) z=%d", in.Type, in.X1, in.Y1, in.X2, in.Y2, in.Z))), nil, nil
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
		X     int    `json:"x" jsonschema:"target workshop's tile (any tile of its footprint)"`
		Y     int    `json:"y"`
		Z     int    `json:"z"`
		Item  string `json:"item" jsonschema:"known short vocabulary: bed|table|chair|door|barrel|bucket|cabinet|coffer|blocks (drink/meal/crafts are NOT supported here — drink has no direct job_type mapping; meal/crafts resolve fine but the plugin rejects them, no verified material filter yet — use the order tool for all three instead) — OR any DFHack job_type enum name (e.g. ConstructHatchCover) for anything not in that list; look one up with the job_types tool"`
		Count int    `json:"count" jsonschema:"how many jobs to queue, one at a time (default 1)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "queue_job",
		Description: "Queue a job directly at an existing workshop — no manager noble or office needed (unlike order, which needs both). Use this for an immediate one-off need; use order for standing/bulk production once a manager exists.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in queueJobIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		// The short vocabulary covers the common cases with no wire
		// round-trip name lookup; anything else falls through to the
		// plugin's generalized name-based resolution (protocol.OrderTypeByName
		// — see work_orders.cpp resolveJobTypeByName) using the caller's
		// string VERBATIM (not lowercased): DFHack job_type keys are
		// case-sensitive CamelCase, e.g. "ConstructHatchCover".
		ot, known := orderTypes[strings.ToLower(in.Item)]
		jobName := ""
		if !known {
			ot = protocol.OrderTypeByName
			jobName = in.Item
		}
		count := in.Count
		if count <= 0 {
			count = 1
		}
		queued := 0
		var lastErr string
		for i := 0; i < count; i++ {
			res, err := b.Exec.SendQueueJob(int16(in.X), int16(in.Y), int16(in.Z), ot, jobName)
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
		what := fmt.Sprintf("queue %dx %s job at (%d,%d,%d)", count, in.Item, in.X, in.Y, in.Z)
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
		Description: "Mark the building at a tile for removal (deconstruction). Any tile of a multi-tile building's footprint works. Dwarves do the teardown over game time and reclaim the materials — step() and check buildings to confirm it's gone.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in xyzIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendRemoveBuilding(int16(in.X), int16(in.Y), int16(in.Z))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("remove building at (%d,%d,%d)", in.X, in.Y, in.Z))), nil, nil
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
		Description: "Apply a dig blueprint from the library at an origin. ALWAYS dry_run=true first: it reports how many tiles would carve solid ground vs hit open space. Empty name lists available blueprints.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in bpIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		if in.Name == "" {
			return withDash(b, ctx, "available blueprints: "+strings.Join(b.Blueprints.ListBlueprints(), ", ")), nil, nil
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
}
