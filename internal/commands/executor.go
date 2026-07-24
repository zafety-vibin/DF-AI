package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// DFHackClient interface for sending commands
type DFHackClient interface {
	SendCommand(cmd *protocol.CommandMessage) error
	SubscribeCommandAcks() <-chan *protocol.CommandAckMessage
	IsConnected() bool
}

// CommandExecutor handles sending commands to DFHack and tracking responses
type CommandExecutor struct {
	logger  *logging.Logger
	client  DFHackClient
	tracker *CommandTracker
	timeout time.Duration
	stopCh  chan struct{}
}

// NewCommandExecutor creates a new command executor
func NewCommandExecutor(logger *logging.Logger, client DFHackClient, timeout time.Duration) *CommandExecutor {
	if timeout == 0 {
		timeout = 5 * time.Second // Default 5 second timeout
	}

	e := &CommandExecutor{
		logger:  logger,
		client:  client,
		tracker: NewCommandTracker(timeout),
		timeout: timeout,
		stopCh:  make(chan struct{}),
	}

	// Start ACK handler
	go e.handleAcks()

	// Start timeout checker
	go e.checkTimeouts()

	return e
}

// SendDigCommand sends a dig designation command on a single Z-level.
// digType: DigTypeDefault (standard), DigTypeUpDownStair, DigTypeChannel, etc.
func (e *CommandExecutor) SendDigCommand(digType uint8, x1, y1, z int16, x2, y2 int16) (*CommandResult, error) {
	return e.SendDigRegion(digType, x1, y1, z, x2, y2, z)
}

// SendDigRegion sends a dig designation command across a 3D rectangle
// (Z1 may differ from Z2). The plugin handles per-tile designation
// natively. Stair-type digTypes across Z1 != Z2 produce a stair shaft:
// UpStair on the bottom Z, DownStair on the top, UpDownStair on middles.
//
// Callers naturally describe shafts top-down (z_start=surface,
// z_end=deep); the wire format and Validate() want ascending Z. Normalize
// here — stair orientation is derived from Z magnitude, not argument
// order, so the swap is semantically free.
func (e *CommandExecutor) SendDigRegion(digType uint8, x1, y1, z1, x2, y2, z2 int16) (*CommandResult, error) {
	if z1 > z2 {
		z1, z2 = z2, z1
	}
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeDig,
		DigType:     digType,
		Region: protocol.Region{
			X1: x1, Y1: y1, Z1: z1,
			X2: x2, Y2: y2, Z2: z2,
		},
	}
	return e.SendCommand(cmd)
}

// buildDesignation is the SOLE constructor of a BuildDesignation literal —
// every SendBuildCommand* helper below funnels through it, so the wire's
// trailing Material/Quality/Orientation bytes are always set explicitly to
// their real "no constraint" sentinel rather than left at a Go zero-value:
// Material=0 is a legitimate "any" already, but Quality=0 is Ordinary and
// Orientation=0 is Horizontal/North — neither is "no preference".
func (e *CommandExecutor) buildDesignation(x, y, z int16, buildType, material, quality, orientation uint8, name string) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeBuild,
		Build: protocol.BuildDesignation{
			X:             x,
			Y:             y,
			Z:             z,
			BuildType:     buildType,
			BuildTypeName: name,
			Material:      material,
			Quality:       quality,
			Orientation:   orientation,
		},
	}
	return e.SendCommand(cmd)
}

// SendBuildCommand sends a build designation command with no material
// class preference (DF's job system picks any suitable item).
func (e *CommandExecutor) SendBuildCommand(x, y, z int16, buildType uint8) (*CommandResult, error) {
	return e.SendBuildCommandWithMaterial(x, y, z, buildType, protocol.MaterialClassAny)
}

// SendBuildCommandWithMaterial sends a build designation command with an
// explicit material class constraint (protocol.MaterialClass*). The class
// narrows what DF's job system may claim (wood/boulders/blocks); DF still
// picks the specific item within the class. The wire payload always
// carries the material byte — MaterialClassAny means "no constraint".
func (e *CommandExecutor) SendBuildCommandWithMaterial(x, y, z int16, buildType, material uint8) (*CommandResult, error) {
	return e.SendBuildCommandWithMaterialAndQuality(x, y, z, buildType, material, protocol.QualityTierAny)
}

// SendBuildCommandWithMaterialAndQuality sends a build designation command
// with both an explicit material class constraint and a quality-tier
// constraint (protocol.QualityTier*). QualityTierAny means no constraint
// (DF/the plugin picks any matching item via constructWithFilters, same as
// SendBuildCommandWithMaterial); any other tier switches the plugin to
// constructWithItems, selecting an EXISTING item of at least that quality
// instead — see protocol.QualityTier*'s doc comment. Only meaningful for
// furniture build types; the plugin rejects a non-Any quality for anything
// else. Orientation is left at BuildOrientAny (no preference) — see
// SendBuildCommandFull for the water/power-transmission family, which needs
// it instead.
func (e *CommandExecutor) SendBuildCommandWithMaterialAndQuality(x, y, z int16, buildType, material, quality uint8) (*CommandResult, error) {
	return e.SendBuildCommandFull(x, y, z, buildType, material, quality, protocol.BuildOrientAny)
}

// SendBuildCommandFull sends a build designation command with all three
// trailing constraints (protocol.MaterialClass*/QualityTier*/
// BuildOrientation*) explicit. Orientation only applies to the
// water/power-transmission building family (ScrewPump/AxleHorizontal/
// WaterWheel/Rollers — protocol.BuildOrientation*'s doc comment); none of
// those seven building types accept a material or quality constraint
// either (every reagent is already a specific finished item or a fixed
// WOOD filter, never a raw building-material class DF could narrow, and
// none are furniture) — same as Material is meaningless for furniture and
// Quality is meaningless for everything else, the plugin is the truthful
// authority on which combos apply and rejects the rest rather than the
// caller having to know in advance.
func (e *CommandExecutor) SendBuildCommandFull(x, y, z int16, buildType, material, quality, orientation uint8) (*CommandResult, error) {
	return e.buildDesignation(x, y, z, buildType, material, quality, orientation, "")
}

// SendBuildCommandByName sends a build designation command via the
// generalized name-based path (protocol.BuildTypeByName) instead of one of
// the curated BuildType* byte constants — reaches any
// building_type/workshop_type/furnace_type/trap_type name the plugin can
// resolve (dfhack-plugin/buildings.cpp: resolveBuildTypeByName, via
// DFHack's find_enum_item tried against all four enums in turn), no new
// byte constant or plugin rebuild needed for a type DFHack already knows
// about. Discover resolvable, ACTUALLY BUILDABLE names via the
// building_types tool. name is the DFHack enum key name verbatim
// (case-sensitive CamelCase, e.g. "Well"), NOT lowercased.
//
// KNOWN LIMITATION (mirrors SendQueueJob's OrderTypeByName path): a name
// resolving successfully does not by itself mean the plugin has a
// placement recipe (job_item filters) for it yet — only building types the
// building_types tool lists are actually buildable today; anything else
// fails with a truthful "resolved but no placement recipe" error.
func (e *CommandExecutor) SendBuildCommandByName(x, y, z int16, name string, material, quality uint8) (*CommandResult, error) {
	return e.SendBuildCommandByNameFull(x, y, z, name, material, quality, protocol.BuildOrientAny)
}

// SendBuildCommandByNameFull is SendBuildCommandByName plus an explicit
// orientation constraint (protocol.BuildOrientation*) — reaches the
// water/power-transmission family (e.g. "ScrewPump", exact DFHack
// CamelCase) via the by-name path with orientation still settable, the
// same way SendBuildCommandFull does for the curated BuildType* path.
func (e *CommandExecutor) SendBuildCommandByNameFull(x, y, z int16, name string, material, quality, orientation uint8) (*CommandResult, error) {
	return e.buildDesignation(x, y, z, protocol.BuildTypeByName, material, quality, orientation, name)
}

// SendCancelCommand sends a cancel designation command
func (e *CommandExecutor) SendCancelCommand(x1, y1, z int16, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeCancel,
		Region: protocol.Region{
			X1: x1, Y1: y1, Z1: z,
			X2: x2, Y2: y2, Z2: z,
		},
	}

	return e.SendCommand(cmd)
}

// SendChopCommand sends a tree chopping designation command
func (e *CommandExecutor) SendChopCommand(x1, y1, z int16, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeChop,
		Region: protocol.Region{
			X1: x1, Y1: y1, Z1: z,
			X2: x2, Y2: y2, Z2: z,
		},
	}

	return e.SendCommand(cmd)
}

// SendGatherCommand sends a plant gathering designation command
func (e *CommandExecutor) SendGatherCommand(x1, y1, z int16, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeGather,
		Region: protocol.Region{
			X1: x1, Y1: y1, Z1: z,
			X2: x2, Y2: y2, Z2: z,
		},
	}

	return e.SendCommand(cmd)
}

// SendZoneCommand designates a region as a civzone (bedroom, dining, etc.).
func (e *CommandExecutor) SendZoneCommand(zoneType uint8, x1, y1, z, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeZone,
		Zone: protocol.ZoneDesignation{
			X1: x1, Y1: y1, Z: z,
			X2: x2, Y2: y2,
			ZoneType: zoneType,
		},
	}
	return e.SendCommand(cmd)
}

// SendAssignZone assigns unitID to the zone at (x,y,z) -- owner or
// roster mechanism, selected by the plugin based on the zone's type.
func (e *CommandExecutor) SendAssignZone(x, y, z int16, unitID int32) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeAssignZone,
		AssignZone: protocol.AssignZoneDesignation{
			X: x, Y: y, Z: z, UnitID: unitID,
		},
	}
	return e.SendCommand(cmd)
}

// SendUnassignZone removes unitID's assignment from the zone at (x,y,z).
func (e *CommandExecutor) SendUnassignZone(x, y, z int16, unitID int32) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeUnassignZone,
		UnassignZone: protocol.UnassignZoneDesignation{
			X: x, Y: y, Z: z, UnitID: unitID,
		},
	}
	return e.SendCommand(cmd)
}

// SendCreateLocation converts the MeetingHall civzone at (x,y,z) into a
// Location of locationType. profession is required only when locationType
// is protocol.LocationTypeGuildhall.
func (e *CommandExecutor) SendCreateLocation(x, y, z int16, locationType uint8, profession string) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeCreateLocation,
		CreateLocation: protocol.CreateLocationDesignation{
			X: x, Y: y, Z: z, LocationType: locationType, Profession: profession,
		},
	}
	return e.SendCommand(cmd)
}

// SendAssignLodging links the Bedroom civzone at (bedroomX,bedroomY,bedroomZ)
// as guest lodging inside the Tavern Location founded by the civzone at
// (tavernX,tavernY,tavernZ).
func (e *CommandExecutor) SendAssignLodging(tavernX, tavernY, tavernZ, bedroomX, bedroomY, bedroomZ int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeAssignLodging,
		AssignLodging: protocol.AssignLodgingDesignation{
			TavernX: tavernX, TavernY: tavernY, TavernZ: tavernZ,
			BedroomX: bedroomX, BedroomY: bedroomY, BedroomZ: bedroomZ,
		},
	}
	return e.SendCommand(cmd)
}

// SendUnassignLodging removes the Bedroom civzone at
// (bedroomX,bedroomY,bedroomZ) from whichever tavern it's lodging for.
func (e *CommandExecutor) SendUnassignLodging(bedroomX, bedroomY, bedroomZ int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeUnassignLodging,
		UnassignLodging: protocol.UnassignLodgingDesignation{
			BedroomX: bedroomX, BedroomY: bedroomY, BedroomZ: bedroomZ,
		},
	}
	return e.SendCommand(cmd)
}

// SendUnsuspendCommand clears the suspend flag on jobs at the given tile.
// Used to resume an auto-suspended construction once the underlying
// blocker has been cleared.
func (e *CommandExecutor) SendUnsuspendCommand(x, y, z int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeUnsuspend,
		Unsuspend: protocol.UnsuspendDesignation{
			X: x, Y: y, Z: z,
		},
	}
	return e.SendCommand(cmd)
}

// SendRemoveBuilding marks the building occupying (x,y,z) for
// deconstruction. Any tile of a multi-tile building's footprint works;
// dwarves do the actual teardown over game time.
func (e *CommandExecutor) SendRemoveBuilding(x, y, z int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeRemoveBuilding,
		Remove: protocol.RemoveBuildingDesignation{
			X: x, Y: y, Z: z,
		},
	}
	return e.SendCommand(cmd)
}

// SendRemoveZone deconstructs the civzone occupying (x,y,z) immediately —
// civzones take DFHack's on_civzone_delete branch, unlike constructed
// buildings (see SendRemoveBuilding), so this never queues dwarf labor.
// The plugin rejects the call if the zone founds a Location.
func (e *CommandExecutor) SendRemoveZone(x, y, z int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeRemoveZone,
		RemoveZone: protocol.RemoveZoneDesignation{
			X: x, Y: y, Z: z,
		},
	}
	return e.SendCommand(cmd)
}

// SendWorkOrderCommand adds a manager work order to produce N items of
// the given type. The manager dispatches to whichever workshop can fulfill
// the order, drawing reagents from stockpiles automatically.
//
// orderType is normally one of the protocol.OrderType* byte constants.
// Pass protocol.OrderTypeByName with jobTypeName set to a DFHack job_type
// enum key name (e.g. "ProcessPlants") to reach any job type the plugin
// can resolve by name instead — no new byte constant needed (see the
// job_types tool for discovery). No hand-maintained workshop-compatibility
// TABLE is needed on this side: the manager itself picks a compatible
// workshop once it dispatches the order. jobTypeName is ignored (and left
// unset on the wire) for the plain byte-vocabulary orderTypes.
//
// material is a job_material_category keyword (wood, bone, shell, leather,
// silk, plant, cloth, yarn) or an exact DFHack material token (e.g.
// "INORGANIC", "INORGANIC:LIMONITE") — needed for job types with real
// material ambiguity (e.g. MakeFigurine, whose material choice also picks
// the workshop); pass "" for job types with none. subtype is a bare raws
// itemdef token (e.g. "ITEM_WEAPON_PICK") pinning which specific item the
// manager should make — optional for every job type (leave "" to let the
// manager pick); discover valid tokens via the job_types tool's
// subtype_of param. frequency is a protocol.WorkOrderFrequency* constant
// selecting a recurring cadence instead of the default one-time order.
func (e *CommandExecutor) SendWorkOrderCommand(orderType uint8, quantity uint16, jobTypeName, material, subtype string, frequency uint8) (*CommandResult, error) {
	order := protocol.WorkOrderDesignation{
		OrderType: orderType,
		Quantity:  quantity,
		Material:  material,
		Subtype:   subtype,
		Frequency: frequency,
	}
	if orderType == protocol.OrderTypeByName {
		order.JobTypeName = jobTypeName
	}
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeWorkOrder,
		Order:       order,
	}
	return e.SendCommand(cmd)
}

// SendQueueJob queues a single job directly at the workshop occupying
// (x,y,z) — the same mechanism a player uses when right-clicking a
// workshop and picking a task, no Manager noble or office required. Use
// SendWorkOrderCommand instead for standing/bulk production once a manager
// exists. Call again to queue more than one job.
//
// orderType is normally one of the protocol.OrderType* byte constants.
// Pass protocol.OrderTypeByName with name set to a DFHack job_type enum
// key name (e.g. "ConstructHatchCover") to reach any job type the plugin
// can resolve by name instead — no new byte constant needed (see the
// job_types tool for discovery). Pass protocol.OrderTypeCustomReaction
// with name set to a df::reaction code (e.g. "BREW_DRINK_FROM_PLANT",
// discoverable via the list_reactions tool) to reach a raw-defined
// reaction directly instead — this is how brewing and anything else with
// no job_type mapping gets queued. name is ignored (and left unset on the
// wire) for the plain byte-vocabulary orderTypes.
//
// material is required only for OrderTypeByName name "SmeltOre" (an exact
// ore token, e.g. "INORGANIC:LIMONITE" — pins job.mat_type/mat_index and
// the BOULDER job_item to that one ore raw); the plugin returns a truthful
// FAILED ack if it's missing there. Ignored (and left unset on the wire)
// otherwise.
//
// subtype is required only for OrderTypeByName names "MakeWeapon",
// "MakeArmor", or "MakeTool" (a bare raws itemdef token, e.g.
// "ITEM_WEAPON_PICK" — pins job.item_type/item_subtype to that one raws
// entry); the plugin returns a truthful FAILED ack if it's missing there.
// Discover valid tokens via the job_types tool's subtype_of param. Ignored
// (and left unset on the wire) otherwise.
func (e *CommandExecutor) SendQueueJob(x, y, z int16, orderType uint8, name, material, subtype string) (*CommandResult, error) {
	qj := protocol.QueueJobDesignation{
		X: x, Y: y, Z: z,
		OrderType: orderType,
		Material:  material,
		Subtype:   subtype,
	}
	switch orderType {
	case protocol.OrderTypeByName:
		qj.JobTypeName = name
	case protocol.OrderTypeCustomReaction:
		qj.ReactionCode = name
	}
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeQueueJob,
		QueueJob:    qj,
	}
	return e.SendCommand(cmd)
}

// SendSetLabor enables or disables one labor on a unit. laborID is a
// protocol.Labor* constant (the real df::unit_labor enum index).
func (e *CommandExecutor) SendSetLabor(unitID int32, laborID uint8, enable bool) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeSetLabor,
		SetLabor: protocol.SetLaborDesignation{
			UnitID:  unitID,
			LaborID: laborID,
			Enable:  enable,
		},
	}
	return e.SendCommand(cmd)
}

// SendStockpileCommand designates a rectangular region as a stockpile
// accepting items in the named groups. groupMask is a bitfield of
// protocol.StockpileGroup* constants (use StockpileGroupAll for an
// "everything" stockpile).
func (e *CommandExecutor) SendStockpileCommand(x1, y1, z, x2, y2 int16, groupMask uint32) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeStockpile,
		Stockpile: protocol.StockpileDesignation{
			X1: x1, Y1: y1, Z: z,
			X2: x2, Y2: y2,
			GroupMask: groupMask,
		},
	}
	return e.SendCommand(cmd)
}

// SendBuildFarmPlot designates a rectangular farm plot at (x1,y1)-(x2,y2)
// on level z. Extent-shaped like a stockpile — must sit on open,
// non-aquatic soil or mud floor. A freshly built plot grows NOTHING until
// SendSetFarmCrop assigns a crop to at least one season slot.
func (e *CommandExecutor) SendBuildFarmPlot(x1, y1, z, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeBuildFarmPlot,
		FarmPlot: protocol.FarmPlotDesignation{
			X1: x1, Y1: y1, Z: z,
			X2: x2, Y2: y2,
		},
	}
	return e.SendCommand(cmd)
}

// SendSetFarmCrop programs the farm plot at (x,y,z) to grow cropName in
// one season slot (protocol.FarmSeasonSpring..FarmSeasonWinter) or all
// four (protocol.FarmSeasonAll). cropName is a plant raw token/display
// name (see the list_crops query) or the literal string "fallow" to clear
// the slot(s).
func (e *CommandExecutor) SendSetFarmCrop(x, y, z int16, season uint8, cropName string) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeSetFarmCrop,
		SetFarmCrop: protocol.SetFarmCropDesignation{
			X: x, Y: y, Z: z,
			Season:   season,
			CropName: cropName,
		},
	}
	return e.SendCommand(cmd)
}

// SendDesignateBurrow creates the named burrow if it doesn't exist yet and
// paints the tile rect (x1,y1,z1)-(x2,y2,z2) into it. Repeatable: calling
// again with the same name adds more tiles to the same burrow. z1 may
// differ from z2 for a cheap multi-level paint in one call.
func (e *CommandExecutor) SendDesignateBurrow(name string, x1, y1, z1, x2, y2, z2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeDesignateBurrow,
		DesignateBurrow: protocol.DesignateBurrowDesignation{
			Name: name,
			X1:   x1, Y1: y1, Z1: z1,
			X2: x2, Y2: y2, Z2: z2,
		},
	}
	return e.SendCommand(cmd)
}

// SendRemoveBurrow deletes the named burrow entirely (unassigns its tiles
// and units first).
func (e *CommandExecutor) SendRemoveBurrow(name string) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeRemoveBurrow,
		RemoveBurrow: protocol.RemoveBurrowDesignation{
			Name: name,
		},
	}
	return e.SendCommand(cmd)
}

// SendAssignBurrow assigns (assign=true) or unassigns (assign=false) a unit
// to/from the named burrow. Set allCitizens=true to target every current
// citizen in one call instead of a single unitID (unitID is ignored then).
func (e *CommandExecutor) SendAssignBurrow(name string, unitID int32, allCitizens bool, assign bool) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeAssignBurrow,
		AssignBurrow: protocol.AssignBurrowDesignation{
			Name:        name,
			Assign:      assign,
			AllCitizens: allCitizens,
			UnitID:      unitID,
		},
	}
	return e.SendCommand(cmd)
}

// SendSetAlert sounds (active=true) or clears (active=false) DF's v50
// civilian alert against the named burrow -- restricts every citizen
// assigned to that burrow (see SendAssignBurrow) to its tiles while
// sounding.
func (e *CommandExecutor) SendSetAlert(name string, active bool) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeSetAlert,
		SetAlert: protocol.SetAlertDesignation{
			Name:   name,
			Active: active,
		},
	}
	return e.SendCommand(cmd)
}

// SendLinkBuilding wires the lever at (leverX,leverY,leverZ) to the
// trigger target (bridge/floodgate/door/hatch/support/gear_assembly) at
// (targetX,targetY,targetZ). Consumes two free mechanisms from the fort's
// stockpiles; the plugin rejects the call truthfully if fewer than two are
// available, if either building is still under construction, or if the
// target isn't a supported trigger type.
func (e *CommandExecutor) SendLinkBuilding(leverX, leverY, leverZ, targetX, targetY, targetZ int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeLinkBuilding,
		LinkBuilding: protocol.LinkBuildingDesignation{
			LeverX: leverX, LeverY: leverY, LeverZ: leverZ,
			TargetX: targetX, TargetY: targetY, TargetZ: targetZ,
		},
	}
	return e.SendCommand(cmd)
}

// SendPullLever queues DF's real PullLever job against the lever at
// (x,y,z). The lever must already be built and linked (SendLinkBuilding)
// to have any effect on a target.
func (e *CommandExecutor) SendPullLever(x, y, z int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypePullLever,
		PullLever:   protocol.PullLeverDesignation{X: x, Y: y, Z: z},
	}
	return e.SendCommand(cmd)
}

// SendBuildBridge designates a rectangular bridge at (x1,y1)-(x2,y2) on
// level z, raising/retracting toward direction (protocol.BridgeDirection*
// constants).
func (e *CommandExecutor) SendBuildBridge(x1, y1, z, x2, y2 int16, direction uint8) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeBuildBridge,
		BuildBridge: protocol.BuildBridgeDesignation{
			X1: x1, Y1: y1, Z: z,
			X2: x2, Y2: y2,
			Direction: direction,
		},
	}
	return e.SendCommand(cmd)
}

// SendSetWorkshopProfile writes min/max skill-level gating and (optionally)
// appends one permitted worker to the workshop_profile of the ACTUAL BUILT
// workshop, furnace, or mechanism trap at (x,y,z). maxSkillLevel == -1
// means uncapped; workerUnitID == -1 means "don't touch permitted_workers".
func (e *CommandExecutor) SendSetWorkshopProfile(x, y, z int16, minSkillLevel, maxSkillLevel, workerUnitID int32) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeSetWorkshopProfile,
		SetWorkshopProfile: protocol.SetWorkshopProfileDesignation{
			X: x, Y: y, Z: z,
			MinSkillLevel: minSkillLevel,
			MaxSkillLevel: maxSkillLevel,
			WorkerUnitID:  workerUnitID,
		},
	}
	return e.SendCommand(cmd)
}

// SendAssignWorkDetail adds (add=true) or removes (add=false) unitID
// from the work detail at detailIndex's membership (assigned_units) --
// see protocol.AssignWorkDetailDesignation. detailIndex is a position in
// the work_details tool's response, not a stable id.
func (e *CommandExecutor) SendAssignWorkDetail(detailIndex uint16, unitID int32, add bool) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeAssignWorkDetail,
		AssignWorkDetail: protocol.AssignWorkDetailDesignation{
			DetailIndex: detailIndex,
			UnitID:      unitID,
			Add:         add,
		},
	}
	return e.SendCommand(cmd)
}

// SendSetWorkDetailMode changes the work detail at detailIndex's mode
// (protocol.WorkDetailMode* constant) and triggers a fort-wide labor
// recompute (every current citizen, not just the detail's own
// membership).
func (e *CommandExecutor) SendSetWorkDetailMode(detailIndex uint16, mode uint8) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeSetWorkDetailMode,
		SetWorkDetailMode: protocol.SetWorkDetailModeDesignation{
			DetailIndex: detailIndex,
			Mode:        mode,
		},
	}
	return e.SendCommand(cmd)
}

// SendCreateWorkDetail allocates a new custom work detail (name + mode +
// pre-enabled labor list) into a free CUSTOM_1..CUSTOM_8 icon slot.
// Truthful FAILED ack results if all 8 slots are already in use.
func (e *CommandExecutor) SendCreateWorkDetail(name string, mode uint8, laborIDs []uint8) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeCreateWorkDetail,
		CreateWorkDetail: protocol.CreateWorkDetailDesignation{
			Name:     name,
			Mode:     mode,
			LaborIDs: laborIDs,
		},
	}
	return e.SendCommand(cmd)
}

// SendBringGoodsToDepot marks up to maxCount free fort items (filtered by
// itemTypeFilter/materialFilter substrings, "" = no filter, plus an
// itemClass bucket — protocol.ItemClassAny/ItemClassCrafts) for hauling to
// the built trade depot at (x,y,z) — see protocol.BringGoodsToDepotDesignation
// for the exact eligibility rules and known limitations. maxTotalValue <= 0
// means no cap on the running total estimated value of marked items.
func (e *CommandExecutor) SendBringGoodsToDepot(x, y, z int16, itemTypeFilter, materialFilter string, itemClass uint8, maxCount int32, maxTotalValue int64) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeBringGoodsToDepot,
		BringGoodsToDepot: protocol.BringGoodsToDepotDesignation{
			X: x, Y: y, Z: z,
			ItemTypeFilter: itemTypeFilter,
			MaterialFilter: materialFilter,
			MaxCount:       maxCount,
			MaxTotalValue:  maxTotalValue,
			ItemClass:      itemClass,
		},
	}
	return e.SendCommand(cmd)
}

// SendUnmarkTradeGoods reverses bring_goods_to_depot's marking at the trade
// depot at (x,y,z), filtered by the same itemTypeFilter/materialFilter/
// itemClass surface (no maxTotalValue — nothing to cap when releasing
// goods). See protocol.UnmarkTradeGoodsDesignation for the exact PENDING/
// STAGED release sequence and its safety invariant (never touches
// merchant-owned goods).
func (e *CommandExecutor) SendUnmarkTradeGoods(x, y, z int16, itemTypeFilter, materialFilter string, itemClass uint8, maxCount int32) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeUnmarkTradeGoods,
		UnmarkTradeGoods: protocol.UnmarkTradeGoodsDesignation{
			X: x, Y: y, Z: z,
			ItemTypeFilter: itemTypeFilter,
			MaterialFilter: materialFilter,
			ItemClass:      itemClass,
			MaxCount:       maxCount,
		},
	}
	return e.SendCommand(cmd)
}

// SendSetDepotTradeFlags writes the trade depot at (x,y,z)'s trade_flags
// bitfield directly -- both fields are the WHOLE desired final state, not a
// delta (read current values via caravan_status first). See
// protocol.SetDepotTradeFlagsDesignation for the exact write sequence and
// its trader_requested true->false job-cleanup companion mutation.
func (e *CommandExecutor) SendSetDepotTradeFlags(x, y, z int16, traderRequested, anyoneCanTrade bool) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeSetDepotTradeFlags,
		SetDepotTradeFlags: protocol.SetDepotTradeFlagsDesignation{
			X: x, Y: y, Z: z,
			TraderRequested: traderRequested,
			AnyoneCanTrade:  anyoneCanTrade,
		},
	}
	return e.SendCommand(cmd)
}

// SendAppointPosition appoints (or replaces the holder of) unitID into the
// entity_position_assignment slot named by positionCode -- see
// protocol.AppointPositionDesignation for the exact eligibility rules and
// dfhack-plugin/nobles.cpp applyAppointPosition for the write sequence.
// Discover live vacant/appointable position codes via the
// position_vacancies tool first.
func (e *CommandExecutor) SendAppointPosition(unitID int32, positionCode string) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeAppointPosition,
		AppointPosition: protocol.AppointPositionDesignation{
			UnitID:       unitID,
			PositionCode: positionCode,
		},
	}
	return e.SendCommand(cmd)
}

// SendSetBookkeeperPrecision writes plotinfo->nobles.bookkeeper_settings
// (a protocol.BookkeeperPrecision* constant) -- the Bookkeeper's goal
// record-keeping precision. UNVERIFIED whether DF clamps/ignores a
// precision beyond what the current bookkeeper's Appraisal skill supports.
func (e *CommandExecutor) SendSetBookkeeperPrecision(precision uint8) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeSetBookkeeperPrecision,
		SetBookkeeperPrecision: protocol.SetBookkeeperPrecisionDesignation{
			Precision: precision,
		},
	}
	return e.SendCommand(cmd)
}

// SendCreateSquad fills (or, if none exists yet, mints -- see
// protocol.CreateSquadDesignation's doc comment for the UNVERIFIED risk
// flag on that path) a vacant entity_position_assignment slot for
// positionCode ("" defaults to "MILITIA_CAPTAIN") and calls DFHack's own
// Military::makeSquad on it. Discover other valid position codes (with
// squad_size > 0) via the position_vacancies tool.
func (e *CommandExecutor) SendCreateSquad(positionCode string) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeCreateSquad,
		CreateSquad: protocol.CreateSquadDesignation{PositionCode: positionCode},
	}
	return e.SendCommand(cmd)
}

// SendAssignSquad adds (add=true) or removes (add=false) unitID from
// squadID's membership -- see protocol.AssignSquadDesignation for the
// exact semantics (first-free-non-commander-slot auto-pick on add;
// civilian labors are not auto-disabled).
func (e *CommandExecutor) SendAssignSquad(squadID, unitID int32, add bool) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeAssignSquad,
		AssignSquad: protocol.AssignSquadDesignation{
			SquadID: squadID,
			UnitID:  unitID,
			Add:     add,
		},
	}
	return e.SendCommand(cmd)
}

// SendSquadOrderStation orders squadID to station at (x,y,z) -- builds a
// squad_order_movest, replacing the squad's entire orders queue. See
// protocol.SquadOrderStation's doc comment for its UNVERIFIED status.
func (e *CommandExecutor) SendSquadOrderStation(squadID int32, x, y, z int16) (*CommandResult, error) {
	return e.sendSquadOrder(squadID, protocol.SquadOrderStation, x, y, z, "")
}

// SendSquadOrderDefendBurrow orders squadID to defend the named burrow
// (must already exist -- see designate_burrow) -- builds a
// squad_order_defend_burrowsst, replacing the squad's entire orders queue.
func (e *CommandExecutor) SendSquadOrderDefendBurrow(squadID int32, burrowName string) (*CommandResult, error) {
	return e.sendSquadOrder(squadID, protocol.SquadOrderDefendBurrow, 0, 0, 0, burrowName)
}

// SendSquadOrderCancel clears squadID's orders queue entirely, with no
// replacement -- combine with SendAssignSquad(add=false) per member to
// fully return a squad to civilian duty.
func (e *CommandExecutor) SendSquadOrderCancel(squadID int32) (*CommandResult, error) {
	return e.sendSquadOrder(squadID, protocol.SquadOrderCancel, 0, 0, 0, "")
}

func (e *CommandExecutor) sendSquadOrder(squadID int32, orderType uint8, x, y, z int16, burrowName string) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeSquadOrder,
		SquadOrder: protocol.SquadOrderDesignation{
			SquadID:    squadID,
			Type:       orderType,
			X:          x,
			Y:          y,
			Z:          z,
			BurrowName: burrowName,
		},
	}
	return e.SendCommand(cmd)
}

// SendCancelOrder deletes manager work order orderID entirely -- cancels
// every job it already spawned, frees its own condition/item pointers, and
// cleans up any surviving order's dangling dependency reference to it. See
// protocol.CancelOrderDesignation's doc comment for the full sequence.
func (e *CommandExecutor) SendCancelOrder(orderID int32) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeCancelOrder,
		CancelOrder: protocol.CancelOrderDesignation{OrderID: orderID},
	}
	return e.SendCommand(cmd)
}

// SendEditOrder changes manager work order orderID's amount_total (a NEW
// target total, not a delta -- see protocol.EditOrderDesignation's doc
// comment) and/or frequency in place. Pass hasAmount/hasFrequency false to
// leave that field untouched; at least one must be true.
func (e *CommandExecutor) SendEditOrder(orderID int32, hasAmount bool, amount uint16, hasFrequency bool, frequency uint8) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeEditOrder,
		EditOrder: protocol.EditOrderDesignation{
			OrderID:      orderID,
			HasAmount:    hasAmount,
			Amount:       amount,
			HasFrequency: hasFrequency,
			Frequency:    frequency,
		},
	}
	return e.SendCommand(cmd)
}

// SendSmoothCommand designates a rectangular region for smoothing or
// engraving. smoothType is protocol.SmoothTypeSmooth (1) or
// SmoothTypeEngrave (2). Only natural stone walls/floors will be acted on
// by DF; soil/sand and constructed walls are silently skipped.
func (e *CommandExecutor) SendSmoothCommand(smoothType uint8, x1, y1, z, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeSmooth,
		Smooth: protocol.SmoothDesignation{
			X1: x1, Y1: y1, Z: z,
			X2: x2, Y2: y2,
			SmoothType: smoothType,
		},
	}
	return e.SendCommand(cmd)
}

// SendPauseCommand pauses (true) or unpauses (false) the DF simulation.
func (e *CommandExecutor) SendPauseCommand(pause bool) (*CommandResult, error) {
	mode := protocol.PauseModeUnpause
	if pause {
		mode = protocol.PauseModePause
	}
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypePause,
		Pause:       protocol.PauseControl{Mode: mode},
	}
	return e.SendCommand(cmd)
}

// SendStepCommand unpauses DF and asks the plugin to re-pause after N
// ticks. The ACK arrives immediately ("step started"); poll the
// sim_status query for completion.
func (e *CommandExecutor) SendStepCommand(ticks uint32) (*CommandResult, error) {
	if ticks == 0 {
		return nil, fmt.Errorf("step ticks must be > 0")
	}
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypePause,
		Pause:       protocol.PauseControl{Mode: protocol.PauseModeStep, Ticks: ticks},
	}
	return e.SendCommand(cmd)
}

// SendCommand sends a command and waits for acknowledgment
func (e *CommandExecutor) SendCommand(cmdMsg *protocol.CommandMessage) (*CommandResult, error) {
	// Check if connected
	if !e.client.IsConnected() {
		return nil, fmt.Errorf("not connected to DFHack")
	}

	// Create command tracking entry
	cmd := &Command{
		ID:         cmdMsg.CommandID,
		Type:       cmdMsg.CommandType,
		Message:    cmdMsg,
		Status:     CommandStatusPending,
		SentAt:     time.Now(),
		TimeoutAt:  time.Now().Add(e.timeout),
		ResultChan: make(chan *CommandResult, 1),
	}

	// Add to tracker
	e.tracker.AddCommand(cmd)

	e.logger.Debug("sending command",
		logging.Field{Key: "command_id", Value: cmd.ID},
		logging.Field{Key: "command_type", Value: cmd.Type})

	// Send command to DFHack
	if err := e.client.SendCommand(cmdMsg); err != nil {
		cmd.Status = CommandStatusFailed
		e.tracker.RemoveCommand(cmd.ID)
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Mark as sent
	cmd.Status = CommandStatusSent
	cmd.SentAt = time.Now()
	cmd.TimeoutAt = time.Now().Add(e.timeout)

	// Wait for result with timeout
	select {
	case result := <-cmd.ResultChan:
		e.logger.Info("command completed",
			logging.Field{Key: "command_id", Value: cmd.ID},
			logging.Field{Key: "success", Value: result.Success},
			logging.Field{Key: "duration_ms", Value: result.Duration.Milliseconds()})
		return result, nil
	case <-time.After(e.timeout + time.Second):
		// Extra second grace period
		e.logger.Warn("command timeout",
			logging.Field{Key: "command_id", Value: cmd.ID})
		return &CommandResult{
			Success:  false,
			Status:   protocol.AckStatusFailure,
			ErrorMsg: "timeout waiting for acknowledgment",
			Duration: time.Since(cmd.SentAt),
		}, nil
	}
}

// WaitForAck waits for acknowledgment of a specific command
func (e *CommandExecutor) WaitForAck(commandID uint32, timeout time.Duration) (*CommandResult, error) {
	cmd, ok := e.tracker.GetCommand(commandID)
	if !ok {
		return nil, fmt.Errorf("command %d not found", commandID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case result := <-cmd.ResultChan:
		return result, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout waiting for acknowledgment")
	}
}

// HandleAck processes an acknowledgment from DFHack
func (e *CommandExecutor) HandleAck(ack *protocol.CommandAckMessage) error {
	e.logger.Debug("received command ack",
		logging.Field{Key: "command_id", Value: ack.CommandID},
		logging.Field{Key: "status", Value: ack.Status})

	if err := e.tracker.MarkAcked(ack.CommandID, ack); err != nil {
		e.logger.Warn("failed to mark command as acked",
			logging.Field{Key: "command_id", Value: ack.CommandID},
			logging.Field{Key: "error", Value: err.Error()})
		return err
	}

	return nil
}

// handleAcks listens for ACK messages from DFHack
func (e *CommandExecutor) handleAcks() {
	ackCh := e.client.SubscribeCommandAcks()

	for {
		select {
		case <-e.stopCh:
			return
		case ack, ok := <-ackCh:
			if !ok {
				e.logger.Warn("ACK channel closed")
				return
			}
			e.HandleAck(ack)
		}
	}
}

// checkTimeouts periodically checks for timed-out commands
func (e *CommandExecutor) checkTimeouts() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			timeoutCount := e.tracker.CheckTimeouts()
			if timeoutCount > 0 {
				e.logger.Warn("commands timed out",
					logging.Field{Key: "count", Value: timeoutCount})
			}
		}
	}
}

// GetStats returns statistics about command execution
func (e *CommandExecutor) GetStats() (total, pending, acked, timeout, failed int) {
	return e.tracker.GetStats()
}

// Stop stops the command executor
func (e *CommandExecutor) Stop() {
	close(e.stopCh)
	e.tracker.Stop()
}
