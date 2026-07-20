package protocol

import (
	"errors"
	"fmt"
)

// Message is the base interface for all protocol messages
type Message interface {
	// Type returns the message type ID (0x01-0x07)
	Type() uint8

	// Serialize converts the message to binary format
	// Returns byte slice with full message including length header
	Serialize() ([]byte, error)

	// Deserialize populates the message from binary data
	// Input should be the full message including headers
	// Returns error if data is malformed or wrong type
	Deserialize(data []byte) error

	// Validate checks that message fields are valid
	// Returns error describing validation failure
	Validate() error
}

// Message type constants
const (
	MessageTypeHandshake          uint8 = 0x01
	MessageTypeFullState          uint8 = 0x02
	MessageTypeTileUpdate         uint8 = 0x03
	MessageTypeResyncRequest      uint8 = 0x04
	MessageTypeHeartbeat          uint8 = 0x05
	MessageTypeError              uint8 = 0x06
	MessageTypeDisconnect         uint8 = 0x07
	MessageTypeEntityUpdate       uint8 = 0x08
	MessageTypeCommand            uint8 = 0x09
	MessageTypeCommandAck         uint8 = 0x0A
	MessageTypeQuery              uint8 = 0x0B
	MessageTypeQueryResponse      uint8 = 0x0C
	MessageTypeAnnouncementUpdate uint8 = 0x0D // plugin → server: new DF announcements
)

// Common errors
var (
	ErrInvalidMessageLength = errors.New("invalid message length")
	ErrInvalidMessageType   = errors.New("invalid message type")
	ErrVersionMismatch      = errors.New("protocol version mismatch")
)

// HandshakeMessage is sent by both sides on connection establishment
type HandshakeMessage struct {
	ProtocolVersion uint8
	ConnectionID    uint32
	Capabilities    string
}

func (m *HandshakeMessage) Type() uint8 { return MessageTypeHandshake }

func (m *HandshakeMessage) Validate() error {
	if m.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("%w: got %d, expected %d", ErrVersionMismatch, m.ProtocolVersion, ProtocolVersion)
	}
	if m.ConnectionID == 0 {
		return errors.New("connection ID must be non-zero")
	}
	return nil
}

// TileState represents the state of a single tile in Dwarf Fortress
type TileState struct {
	X        int16  // X coordinate
	Y        int16  // Y coordinate
	Z        int16  // Z coordinate (can be negative for underground levels)
	TileType uint16 // DF tiletype enum value
	Flags    uint8  // Packed bit flags
}

// Flag bit masks for TileState.Flags
const (
	FlagHidden     uint8 = 0x01 // Tile in fog of war
	FlagDiscovered uint8 = 0x02 // Tile has been seen before
	FlagDesignated uint8 = 0x04 // Designated for mining
	FlagConstruct  uint8 = 0x08 // Designated for construction
	FlagWall       uint8 = 0x10 // Solid wall/rock (can dig)
	FlagFloor      uint8 = 0x20 // Walkable floor/ramp/stair (don't dig, but can channel)
	FlagVoid       uint8 = 0x40 // Open air/missing floor (fall hazard, don't dig)
	FlagLiquid7    uint8 = 0x80 // Water/magma at 7/7 depth (disaster if breached)
)

// HasFlag checks if a specific flag bit is set
func (t *TileState) HasFlag(flag uint8) bool {
	return (t.Flags & flag) != 0
}

// SetFlag sets a specific flag bit
func (t *TileState) SetFlag(flag uint8) {
	t.Flags |= flag
}

// ClearFlag clears a specific flag bit
func (t *TileState) ClearFlag(flag uint8) {
	t.Flags &^= flag
}

// FullStateMessage contains complete map state
type FullStateMessage struct {
	Width       uint16
	Height      uint16
	Depth       uint16
	Tiles       []TileState
	IsSoilLayer []bool // Feature 007: Per Z-level soil flags (200 bools, 1 per Z-level)
}

func (m *FullStateMessage) Type() uint8 { return MessageTypeFullState }

func (m *FullStateMessage) Validate() error {
	expectedCount := int(m.Width) * int(m.Height) * int(m.Depth)
	if len(m.Tiles) != expectedCount {
		return fmt.Errorf("tile count mismatch: got %d, expected %d (W=%d H=%d D=%d)",
			len(m.Tiles), expectedCount, m.Width, m.Height, m.Depth)
	}
	if m.Width == 0 || m.Height == 0 || m.Depth == 0 {
		return errors.New("dimensions must be greater than zero")
	}
	return nil
}

// ResyncRequestMessage requests a full state resynchronization
type ResyncRequestMessage struct {
	Reason uint8
}

func (m *ResyncRequestMessage) Type() uint8 { return MessageTypeResyncRequest }

func (m *ResyncRequestMessage) Validate() error {
	// Reason codes 0x00-0x04 are valid
	if m.Reason > 0x04 {
		return fmt.Errorf("invalid resync reason: 0x%02X", m.Reason)
	}
	return nil
}

// Resync reason constants
const (
	ReasonManual        uint8 = 0x00
	ReasonInconsistency uint8 = 0x01
	ReasonReconnect     uint8 = 0x02
	ReasonPeriodic      uint8 = 0x03
	ReasonSaveRequest   uint8 = 0x04 // NEW: Plugin requesting save
)

// TileUpdateMessage contains incremental tile changes
type TileUpdateMessage struct {
	Count uint32
	Tiles []TileState
}

func (m *TileUpdateMessage) Type() uint8 { return MessageTypeTileUpdate }

func (m *TileUpdateMessage) Validate() error {
	if uint32(len(m.Tiles)) != m.Count {
		return fmt.Errorf("tile count mismatch: Count=%d, len(Tiles)=%d", m.Count, len(m.Tiles))
	}
	if m.Count == 0 {
		return errors.New("tile update must have at least one tile")
	}
	// No maximum - we can handle full map updates if needed
	return nil
}

// HeartbeatMessage is used to detect connection liveness
type HeartbeatMessage struct {
	Timestamp uint64 // Unix timestamp in milliseconds
	Sequence  uint8  // Incrementing sequence number for echo tracking
}

func (m *HeartbeatMessage) Type() uint8 { return MessageTypeHeartbeat }

func (m *HeartbeatMessage) Validate() error {
	// Heartbeat messages are always valid - no constraints
	return nil
}

// DisconnectMessage signals intentional connection closure
type DisconnectMessage struct {
	Reason uint8
}

func (m *DisconnectMessage) Type() uint8 { return MessageTypeDisconnect }

func (m *DisconnectMessage) Validate() error {
	// Reason codes 0x00-0x03 are valid
	if m.Reason > 0x03 {
		return fmt.Errorf("invalid disconnect reason: 0x%02X", m.Reason)
	}
	return nil
}

// Disconnect reason constants
const (
	ReasonNormalShutdown uint8 = 0x00
	ReasonPluginUnload   uint8 = 0x01
	ReasonServerShutdown uint8 = 0x02
	ReasonRestart        uint8 = 0x03
)

// ErrorMessage signals protocol errors
type ErrorMessage struct {
	Code    uint16
	Message string
}

func (m *ErrorMessage) Type() uint8 { return MessageTypeError }

func (m *ErrorMessage) Validate() error {
	// All error codes are valid
	// Message can be any string (including empty)
	return nil
}

// Error code constants
const (
	ErrUnknownType      uint16 = 0x0001
	ErrInvalidPayload   uint16 = 0x0002
	ErrVersionMismatch_ uint16 = 0x0003
	ErrInternal         uint16 = 0x0004
)

// EntityInfo represents a single entity (dwarf, enemy, animal)
type EntityInfo struct {
	ID      uint32 // Unit ID from DF
	X       int16  // X coordinate
	Y       int16  // Y coordinate
	Z       int16  // Z coordinate
	Type    uint8  // 1=dwarf, 2=enemy, 3=animal, 4=other
	Subtype uint16 // Race ID for detailed classification

	// Dead reports DFHack's own Units::isDead() (flags2.bits.killed ||
	// flags3.bits.ghostly) for this unit. It is NOT part of each entity's
	// fixed 13-byte wire record -- it is cross-referenced from the
	// optional trailing DeadUnits block (see deserializeEntityUpdate) so
	// old peers that don't emit that block simply decode Dead=false for
	// everyone (backward compatible, same additive pattern as Zones).
	// A dead unit typically stays in units.active (and thus in this list)
	// reporting a frozen last-known position -- Dead is the only truthful
	// signal that it's a corpse, not an idle citizen; never infer death
	// from position staleness elsewhere.
	Dead bool
}

// Entity type constants
const (
	EntityTypeDwarf  uint8 = 0x01
	EntityTypeEnemy  uint8 = 0x02
	EntityTypeAnimal uint8 = 0x03
	EntityTypeOther  uint8 = 0x04
)

// FortInfo contains fort-level statistics (optional extension to ENTITY_UPDATE)
type FortInfo struct {
	DaysElapsed   uint32 // In-game days since embark (from cur_year_tick)
	CreatedWealth uint64 // Total fort value (world->status.created_wealth)
	Season        uint8  // 0=Spring, 1=Summer, 2=Autumn, 3=Winter
	Year          uint32 // Current year
}

// WorldIdentity fingerprints the currently loaded save, read once per
// ENTITY_UPDATE from df::global::world->cur_savegame (DFHack 53.15-r2:
// save_dir + world_header.id1/id2). SaveDir alone is not a safe fingerprint
// (two save folders could coincidentally share a name across reinstalls);
// ID1/ID2 ("based on tick at start of game" / "based on tick at creation
// time") are a numeric pair that cannot collide the same way -- together
// they change the instant a different save loads, even mid-connection.
// See Populator.OnEntityUpdate for how a change is detected and surfaced.
type WorldIdentity struct {
	SaveDir string
	ID1     uint32
	ID2     uint32
}

// ZoneData represents a DF civzone extracted from game state. ZoneID is
// DFHack's internal building id — informational only; assign_zone and
// unassign_zone target zones by tile coordinate (matching
// remove_building/unsuspend's existing convention), never by this ID.
type ZoneData struct {
	ZoneID        uint32  // DFHack building id (informational only)
	ZoneType      uint8   // One of the ZoneType* constants above
	X1, Y1, Z1    int16   // Start coordinates
	X2, Y2, Z2    int16   // End coordinates
	OwnerUnitID   int32   // -1 if unowned or not an owner-type zone
	AssignedUnits []int32 // roster (empty for owner-type or unconfirmed-mechanism zones)
}

// EntityUpdateMessage contains entity position updates + optional fort info + zones
type EntityUpdateMessage struct {
	Count    uint32
	Entities []EntityInfo
	FortInfo *FortInfo  // Optional (nil if not included)
	Zones    []ZoneData // Feature 007: Zone data (empty if zone extraction disabled)
	// World is the currently loaded save's identity fingerprint -- optional
	// additive trailing block (nil when the plugin peer predates it, same
	// pattern as FortInfo). See WorldIdentity's doc comment.
	World *WorldIdentity
}

func (m *EntityUpdateMessage) Type() uint8 { return MessageTypeEntityUpdate }

func (m *EntityUpdateMessage) Validate() error {
	if uint32(len(m.Entities)) != m.Count {
		return fmt.Errorf("entity count mismatch: Count=%d, len(Entities)=%d", m.Count, len(m.Entities))
	}
	// Count can be 0 (no entities) - that's valid
	return nil
}

// CommandType constants for command messages
const (
	CommandTypeDig             uint8 = 0x01
	CommandTypeBuild           uint8 = 0x02
	CommandTypeCancel          uint8 = 0x03
	CommandTypeChop            uint8 = 0x04
	CommandTypeGather          uint8 = 0x05
	CommandTypeZone            uint8 = 0x06 // Designate zones (bedroom, dining, etc.)
	CommandTypeBlueprint       uint8 = 0x07 // Feature 007: Apply blueprint pattern with zones
	CommandTypeUnsuspend       uint8 = 0x08 // Clear suspend flag on a building's jobs
	CommandTypeWorkOrder       uint8 = 0x09 // Add a manager work order
	CommandTypeStockpile       uint8 = 0x0A // Designate a stockpile zone with category flags
	CommandTypeSmooth          uint8 = 0x0B // Designate stone tiles for smoothing or engraving
	CommandTypePause           uint8 = 0x0C // pause/unpause/step simulation control
	CommandTypeRemoveBuilding  uint8 = 0x0D // Mark the building at a tile for deconstruction
	CommandTypeQueueJob        uint8 = 0x0E // Queue a job directly at an existing workshop (no manager/office needed)
	CommandTypeSetLabor        uint8 = 0x0F // Enable/disable one labor on a unit
	CommandTypeAssignZone      uint8 = 0x10 // Assign a unit to the zone at a tile (owner or roster, depending on zone type)
	CommandTypeUnassignZone    uint8 = 0x11 // Remove a unit's zone assignment
	CommandTypeCreateLocation  uint8 = 0x12 // Convert a MeetingHall civzone into a Location (Tavern/Temple/Library/Guildhall)
	CommandTypeAssignLodging   uint8 = 0x13 // Link a Bedroom civzone as guest lodging for a Tavern Location
	CommandTypeUnassignLodging uint8 = 0x14 // Remove a Bedroom civzone from lodging duty
	CommandTypeRemoveZone      uint8 = 0x15 // Deconstruct the civzone at a tile (rejected if it founds a Location)
	CommandTypeBuildFarmPlot   uint8 = 0x16 // Designate a rectangular farm plot (extent-shaped, like a stockpile)
	CommandTypeSetFarmCrop     uint8 = 0x17 // Assign a crop (or clear) to one or all season slots of a farm plot
	CommandTypeDesignateBurrow uint8 = 0x18 // Create (if new) a named burrow and paint a tile rect into it
	CommandTypeRemoveBurrow    uint8 = 0x19 // Delete a named burrow entirely (unassigns units/tiles first)
	CommandTypeAssignBurrow    uint8 = 0x1A // Assign/unassign one unit, or all current citizens, to/from a named burrow
	CommandTypeSetAlert        uint8 = 0x1B // Sound/clear the v50 civilian alert against a named burrow
	CommandTypeLinkBuilding    uint8 = 0x1C // Wire a lever or pressure plate to a trigger target (bridge/floodgate/door/hatch/support/gear_assembly)
	CommandTypePullLever       uint8 = 0x1D // Queue DF's real PullLever job against a built lever
	CommandTypeBuildBridge     uint8 = 0x1E // Designate a rectangular bridge with a raise/retract direction

	// CommandTypeSetWorkshopProfile writes df::workshop_profile's
	// min_level/max_level (skill-rating gate) and optionally appends one
	// unit to permitted_workers on an existing BUILT workshop, furnace, or
	// mechanism trap (Lever) -- DF's real mechanism for probabilistically
	// improving job output quality, complementing BuildDesignation.Quality
	// (which picks among items that already exist).
	CommandTypeSetWorkshopProfile uint8 = 0x1F

	// Work Detail (labor-group) commands -- DF's own work_detail mechanism
	// (df.plotinfo.xml labor_infost.work_details, since v0.50.01). This is
	// the AUTHORITATIVE store behind a unit's derived status.labors cache
	// (see SetLaborDesignation's doc comment). CommandTypeAssignWorkDetail
	// edits one detail's membership (assigned_units); CommandTypeSetWorkDetailMode
	// changes a detail's mode (WorkDetailMode* constants) and recomputes
	// derived labors for EVERY current citizen, not just the detail's own
	// membership, since a mode change (e.g. EverybodyDoesThis) reshuffles
	// who does what fort-wide; CommandTypeCreateWorkDetail allocates a
	// brand-new custom detail into a free CUSTOM_1..CUSTOM_8 icon slot.
	// Matches dfhack-plugin/protocol.h COMMAND_TYPE_ASSIGN_WORK_DETAIL /
	// COMMAND_TYPE_SET_WORK_DETAIL_MODE / COMMAND_TYPE_CREATE_WORK_DETAIL.
	CommandTypeAssignWorkDetail  uint8 = 0x20
	CommandTypeSetWorkDetailMode uint8 = 0x21
	CommandTypeCreateWorkDetail  uint8 = 0x22

	// CommandTypeBringGoodsToDepot marks up to MaxCount free fort items
	// (filtered by ItemTypeFilter/MaterialFilter substrings) for hauling to
	// the built trade depot at (X,Y,Z) -- see BringGoodsToDepotDesignation's
	// doc comment and dfhack-plugin/protocol.h COMMAND_TYPE_BRING_GOODS_TO_DEPOT.
	CommandTypeBringGoodsToDepot uint8 = 0x23

	// CommandTypeAppointPosition fills (or replaces the holder of) one
	// entity_position_assignment slot -- see AppointPositionDesignation's
	// doc comment and dfhack-plugin/protocol.h COMMAND_TYPE_APPOINT_POSITION.
	CommandTypeAppointPosition uint8 = 0x24

	// CommandTypeSetBookkeeperPrecision writes plotinfo->nobles.
	// bookkeeper_settings (df::record_precision_level_type) directly -- see
	// SetBookkeeperPrecisionDesignation's doc comment and
	// dfhack-plugin/protocol.h COMMAND_TYPE_SET_BOOKKEEPER_PRECISION.
	CommandTypeSetBookkeeperPrecision uint8 = 0x25

	// CommandTypeCreateSquad fills (or, if none exists yet for the
	// requested position code, mints -- see CreateSquadDesignation's doc
	// comment for the UNVERIFIED risk flag on that path) a vacant
	// entity_position_assignment slot and calls DFHack's own
	// Military::makeSquad on it -- see docs/decisions.md (2026-07-19
	// military research pass) for the full data-model citation.
	CommandTypeCreateSquad uint8 = 0x26

	// CommandTypeAssignSquad adds or removes one unit from a squad's
	// membership -- see AssignSquadDesignation's doc comment.
	CommandTypeAssignSquad uint8 = 0x27

	// CommandTypeSquadOrder replaces a squad's entire orders queue with
	// at most one order (station/defend-burrow) or clears it entirely --
	// see SquadOrderDesignation's doc comment.
	CommandTypeSquadOrder uint8 = 0x28

	// CommandTypeCancelOrder deletes ONE manager work order by id --
	// cancels every job it already spawned, frees its own condition/item
	// pointers, and cleans up any surviving order's dangling dependency
	// reference to it. See CancelOrderDesignation's doc comment and
	// dfhack-plugin/work_orders.cpp applyCancelOrder for the full sequence
	// (docs/decisions.md 2026-07-19 manager-work-order-lifecycle pass, Q1).
	CommandTypeCancelOrder uint8 = 0x29

	// CommandTypeEditOrder changes an existing manager work order's
	// amount_total/amount_left and/or frequency IN PLACE -- see
	// EditOrderDesignation's doc comment and dfhack-plugin/work_orders.cpp
	// applyEditOrder.
	CommandTypeEditOrder uint8 = 0x2A
)

// SquadOrder* constants for SquadOrderDesignation.Type -- DF-AI's own wire
// values (the minimal, chokepoint-defense-scoped subset of DF's real
// squad_order subtype family; see docs/decisions.md 2026-07-19 military
// research pass for why kill-list/kill-hf, patrol-route, and the
// site-leaving raid/drive-off/rescue/retrieve order family are all left
// out). Matches dfhack-plugin/protocol.h SQUAD_ORDER_* constants.
const (
	// SquadOrderStation constructs a squad_order_movest targeting
	// SquadOrderDesignation.X/Y/Z -- DF's "station here" order. UNVERIFIED:
	// no DFHack code in this checkout ever constructs one; point_id is
	// always sent as -1 (no associated saved Notes-screen waypoint).
	SquadOrderStation uint8 = 0x00

	// SquadOrderDefendBurrow constructs a squad_order_defend_burrowsst
	// referencing the burrow named SquadOrderDesignation.BurrowName (see
	// the existing designate_burrow tool) -- DF's native "hold this area"
	// order, and the best-fit real order type for chokepoint defense.
	SquadOrderDefendBurrow uint8 = 0x01

	// SquadOrderCancel clears the squad's orders queue entirely with no
	// replacement -- "return to its default schedule/idle behavior".
	// Combine with assign_squad (remove) to fully return a unit to
	// civilian duty.
	SquadOrderCancel uint8 = 0x02
)

// Labor constants for the SET_LABOR command. The value IS the real
// df::unit_labor enum index (DFHack 53.15-r1 library/include/df/unit_labor.h,
// base-type int32_t, valid range 0-93) — sent directly as the wire byte, no
// translation table on either side. This is a curated subset relevant to a
// fresh 7-dwarf fort, not DF's full 94-entry labor list. Kept in sync with
// dfhack-plugin/protocol.h (LABOR_* constants).
const (
	LaborMine           uint8 = 0
	LaborHaulStone      uint8 = 1
	LaborHaulWood       uint8 = 2
	LaborHaulFood       uint8 = 4
	LaborHaulItem       uint8 = 6
	LaborHaulFurniture  uint8 = 7
	LaborCutwood        uint8 = 10
	LaborCarpenter      uint8 = 11
	LaborStonecutter    uint8 = 12
	LaborStoneCarver    uint8 = 13
	LaborEngraver       uint8 = 14 // caption "Stone Engraving" — not DETAIL
	LaborMason          uint8 = 15
	LaborBrewer         uint8 = 30
	LaborCook           uint8 = 38
	LaborPlant          uint8 = 39
	LaborHerbalist      uint8 = 40
	LaborFish           uint8 = 41
	LaborSmelt          uint8 = 45
	LaborForgeWeapon    uint8 = 46
	LaborForgeArmor     uint8 = 47
	LaborForgeFurniture uint8 = 48
	LaborMetalCraft     uint8 = 49
	LaborMechanic       uint8 = 60

	// LaborMaxIndex is the highest valid df::unit_labor array index
	// (unit_labor.h last_item_value=93). SET_LABOR rejects any LaborID
	// above this.
	LaborMaxIndex uint8 = 93
)

// WorkDetailMode constants for the SET_WORK_DETAIL_MODE and
// CREATE_WORK_DETAIL commands' Mode byte. Matches df::work_detail_mode
// directly (df.plotinfo.xml) — no translation table. NobodyDoesThis's
// actual in-game effect beyond "not auto-assigned via this detail" is NOT
// confirmed by anything in the DFHack 53.15-r2 checkout; acks stay
// truthful about what byte was written, not a guess about DF's
// job-assignment behavior. Kept in sync with dfhack-plugin/protocol.h
// WORK_DETAIL_MODE_* constants.
const (
	WorkDetailModeDefault              uint8 = 0x00
	WorkDetailModeEverybodyDoesThis    uint8 = 0x01
	WorkDetailModeNobodyDoesThis       uint8 = 0x02
	WorkDetailModeOnlySelectedDoesThis uint8 = 0x03
)

// BookkeeperPrecision constants for the SET_BOOKKEEPER_PRECISION command's
// single payload byte. Matches df::record_precision_level_type directly
// (df.d_basics.xml: NONE=-1 is never sent over the wire -- there is no
// "unset" sentinel here, only a concrete goal precision to write), so the
// wire byte IS the enum value with no translation table, same pattern as
// WorkDetailMode above. Kept in sync with dfhack-plugin/protocol.h
// BOOKKEEPER_PRECISION_* constants.
const (
	BookkeeperPrecisionNearest10    uint8 = 0x00
	BookkeeperPrecisionNearest100   uint8 = 0x01
	BookkeeperPrecisionNearest1000  uint8 = 0x02
	BookkeeperPrecisionNearest10000 uint8 = 0x03
	BookkeeperPrecisionAllAccurate  uint8 = 0x04
)

// MaterialClass constants for the BUILD command's first trailing byte.
// Constrains which item CLASS DF's job system may claim for a generic
// building-material job_item filter (constructions, workshops, furnaces,
// trade depot); DF still picks the specific item within that class. Does
// NOT apply to furniture or doors/hatches/levers/floodgates, which already
// filter on one specific finished item type rather than a raw material
// class -- the plugin rejects a non-ANY class for those build types. The
// plugin treats a BUILD payload without the byte as MaterialClassAny
// (backward compatible); new Go always appends it.
const (
	MaterialClassAny    uint8 = 0x00 // no constraint (DF picks anything suitable)
	MaterialClassWood   uint8 = 0x01 // logs (job_item item_type=WOOD)
	MaterialClassStone  uint8 = 0x02 // boulders (item_type=BOULDER)
	MaterialClassBlocks uint8 = 0x03 // blocks (item_type=BLOCKS)
)

// QualityTier constants for the BUILD command's second trailing byte
// (right after Material). Matches df::item_quality (DFHack-only enum,
// confirmed against df.dfhack.xml: Ordinary/WellCrafted/FinelyCrafted/
// Superior/Exceptional/Masterful/Artifact -- "Masterful", NOT
// "Masterwork"). Only meaningful for furniture build types (Bed/Table/
// Chair/Cabinet/Coffer/Coffin): selects an EXISTING item of at least this
// quality at PLACEMENT time (Buildings::constructWithItems) instead of
// accepting any matching item (Buildings::constructWithFilters) -- quality
// cannot be requested at craft time in vanilla DF, only chosen among what
// already exists. The plugin rejects a non-Any tier for any non-furniture
// build type. QualityTierAny (0xFF) is the backward-compatible "no
// constraint" sentinel: a BUILD payload without this byte (pre-quality
// peer, or one that only appends Material) decodes as QualityTierAny; new
// Go always appends it.
const (
	QualityTierOrdinary      uint8 = 0x00
	QualityTierWellCrafted   uint8 = 0x01
	QualityTierFinelyCrafted uint8 = 0x02
	QualityTierSuperior      uint8 = 0x03
	QualityTierExceptional   uint8 = 0x04
	QualityTierMasterful     uint8 = 0x05
	QualityTierArtifact      uint8 = 0x06
	QualityTierAny           uint8 = 0xFF // sentinel: no constraint (default)
)

// BuildOrientation constants for the BUILD command's THIRD trailing byte
// (right after Quality) -- used only by the water/power-transmission
// building family (BuildTypeScrewPump/BuildTypeAxleHorizontal/
// BuildTypeWaterWheel/BuildTypeRollers). Mirrors DF's own single
// `direction` int parameter to Buildings::setSize (dfhack-build
// library/modules/Buildings.cpp:922-991), which the engine casts TWO
// different ways depending on building type:
//
//	AxleHorizontal, WaterWheel  -> axis: Horizontal (0, an E-W line) or
//	                               Vertical (nonzero, an N-S line) -- DF
//	                               casts via `!!direction`, so ANY nonzero
//	                               value means vertical; this package only
//	                               ever emits/accepts 0 or 1.
//	ScrewPump, Rollers          -> compass direction, matching
//	                               df::screw_pump_direction (FromNorth=0,
//	                               FromEast=1, FromSouth=2, FromWest=3).
//	                               ScrewPump: which side draws water FROM.
//	                               Rollers: which direction items are
//	                               pushed.
//	GearAssembly, AxleVertical  -> ignored entirely (always 1x1 --
//	                               Buildings::getCorrectSize has no case
//	                               for either).
//
// BuildOrientHorizontal/BuildOrientNorth (and BuildOrientVertical/
// BuildOrientEast) are deliberately the SAME wire byte value -- DF's own
// `direction` field is one int with dual meaning depending on building
// type, not two separate concepts; the alias names just let a caller
// spell whichever reads naturally for the type it's building.
// BuildOrientAny (0xFF) is the backward-compatible "unspecified" sentinel:
// a BUILD payload without this byte decodes as BuildOrientAny, and the
// plugin treats BuildOrientAny as 0 -- a legal default for every one of
// these types.
const (
	BuildOrientHorizontal uint8 = 0x00
	BuildOrientVertical   uint8 = 0x01
	BuildOrientNorth      uint8 = 0x00
	BuildOrientEast       uint8 = 0x01
	BuildOrientSouth      uint8 = 0x02
	BuildOrientWest       uint8 = 0x03
	BuildOrientAny        uint8 = 0xFF // sentinel: unspecified (plugin default: 0)
)

// SmoothType constants (matches df::tile_designation::smooth bitfield: 1=smooth, 2=engrave).
const (
	SmoothTypeSmooth  uint8 = 0x01
	SmoothTypeEngrave uint8 = 0x02
)

// Stockpile group bits — match df::stockpile_group_set bitfield positions.
// LLM picks an "all" mask or a category-specific subset by name.
const (
	StockpileGroupAnimals       uint32 = 1 << 0
	StockpileGroupFood          uint32 = 1 << 1
	StockpileGroupFurniture     uint32 = 1 << 2
	StockpileGroupCorpses       uint32 = 1 << 3
	StockpileGroupRefuse        uint32 = 1 << 4
	StockpileGroupStone         uint32 = 1 << 5
	StockpileGroupAmmo          uint32 = 1 << 6
	StockpileGroupCoins         uint32 = 1 << 7
	StockpileGroupBarsBlocks    uint32 = 1 << 8
	StockpileGroupGems          uint32 = 1 << 9
	StockpileGroupFinishedGoods uint32 = 1 << 10
	StockpileGroupLeather       uint32 = 1 << 11
	StockpileGroupCloth         uint32 = 1 << 12
	StockpileGroupWood          uint32 = 1 << 13
	StockpileGroupWeapons       uint32 = 1 << 14
	StockpileGroupArmor         uint32 = 1 << 15
	StockpileGroupSheet         uint32 = 1 << 16

	// All 17 categories.
	StockpileGroupAll uint32 = 0x1FFFF
)

// ZoneType constants for zone designations. These are DF-AI's OWN wire
// values, matching dfhack-plugin/protocol.h's ZONE_TYPE_* constants —
// NOT DFHack's own df::civzone_type values directly (those are scattered
// 79-97, confirmed against the DFHack 53.15-r1 checkout). Assignment
// support: Owner (setOwner) works for Bedroom/Office/Tomb/DiningHall;
// Roster (assigned_units) works for Pen/Pond; Barracks uses a separate
// squad-based mechanism (out of scope for assign_zone); the rest have no
// confirmed DFHack assignment mechanism at all — assign_zone returns an
// explicit "not implemented" error for them.
const (
	ZoneTypeBedroom        uint8 = 0x01 // Owner
	ZoneTypeOffice         uint8 = 0x02 // Owner
	ZoneTypeTomb           uint8 = 0x03 // Owner
	ZoneTypeDiningHall     uint8 = 0x04 // Owner
	ZoneTypeMeetingHall    uint8 = 0x05 // Unconfirmed
	ZoneTypeDormitory      uint8 = 0x06 // Unconfirmed
	ZoneTypeBarracks       uint8 = 0x07 // Squad (out of scope)
	ZoneTypePen            uint8 = 0x08 // Roster
	ZoneTypePond           uint8 = 0x09 // Roster
	ZoneTypeArcheryRange   uint8 = 0x0A // Unconfirmed
	ZoneTypePlantGathering uint8 = 0x0B // Unconfirmed
	ZoneTypeWaterSource    uint8 = 0x0C // Unconfirmed
	ZoneTypeDump           uint8 = 0x0D // Unconfirmed
	ZoneTypeSandCollection uint8 = 0x0E // Unconfirmed
	ZoneTypeFishingArea    uint8 = 0x0F // Unconfirmed
	ZoneTypeClayCollection uint8 = 0x10 // Unconfirmed
	ZoneTypeDungeon        uint8 = 0x11 // Unconfirmed
	ZoneTypeAnimalTraining uint8 = 0x12 // Unconfirmed
)

// LocationType constants -- DF-AI's own wire values for
// df::abstract_building_type's INN_TAVERN/TEMPLE/LIBRARY/GUILDHALL/HOSPITAL.
// A Location is created FROM an existing MeetingHall civzone via
// create_location, not designated directly.
const (
	LocationTypeTavern    uint8 = 0x01
	LocationTypeTemple    uint8 = 0x02
	LocationTypeLibrary   uint8 = 0x03
	LocationTypeGuildhall uint8 = 0x04 // requires CreateLocationDesignation.Profession
	LocationTypeHospital  uint8 = 0x05
)

// FarmSeason constants for the SET_FARM_CROP command's Season byte. 0-3
// target one of building_farmplotst::plant_id's four season slots (matches
// dfhack-plugin/protocol.h SEASON_* and df::season Spring/Summer/Autumn/
// Winter). FarmSeasonAll is a wire-level convenience with no DF
// equivalent — write the same crop into all four slots in one call.
const (
	FarmSeasonSpring uint8 = 0x00
	FarmSeasonSummer uint8 = 0x01
	FarmSeasonAutumn uint8 = 0x02
	FarmSeasonWinter uint8 = 0x03
	FarmSeasonAll    uint8 = 0xFF
)

// OrderType constants for manager work orders.
const (
	OrderTypeMakeBed     uint8 = 0x01
	OrderTypeMakeTable   uint8 = 0x02
	OrderTypeMakeChair   uint8 = 0x03
	OrderTypeMakeDoor    uint8 = 0x04
	OrderTypeMakeBarrel  uint8 = 0x05
	OrderTypeMakeBucket  uint8 = 0x06
	OrderTypeMakeCabinet uint8 = 0x07
	OrderTypeMakeCoffer  uint8 = 0x08
	OrderTypeBrewDrink   uint8 = 0x09
	OrderTypePrepareMeal uint8 = 0x0A
	OrderTypeMakeBlocks  uint8 = 0x0B
	OrderTypeMakeCrafts  uint8 = 0x0C

	// OrderTypeCustomReaction is the sentinel for QueueJobDesignation's
	// reaction-based path — reach ANY raw-defined df::reaction a workshop
	// supports (e.g. BREW_DRINK_FROM_PLANT), keyed by its reaction CODE
	// (df::reaction.code, NOT the display name) rather than by
	// df::job_type. This is how queue_job reaches reactions like brewing
	// that have no job_type mapping at all (protocolToJobType returns -1
	// for OrderTypeBrewDrink — see work_orders.cpp). Only
	// CommandTypeQueueJob understands this sentinel — unlike OrderTypeByName
	// below, WorkOrderDesignation does NOT support it; OrderTypeBrewDrink
	// above remains the one reaction-backed order type the manager-queue
	// path supports. Matches dfhack-plugin/protocol.h
	// ORDER_TYPE_CUSTOM_REACTION. Discover valid codes via the
	// list_reactions query/tool.
	OrderTypeCustomReaction uint8 = 0x0D

	// OrderTypeByName is not one of the hand-maintained order types above —
	// it's the sentinel for the generalized name-based job-type path,
	// shared by BOTH QueueJobDesignation and, as of a later pass,
	// WorkOrderDesignation (see each type's doc comment for its own
	// trailing-name wire shape). CommandTypeWorkOrder
	// (SendWorkOrderCommand) needs no per-job_type workshop-compatibility
	// or material-class filter to use this — DF's own manager fills in
	// job_items and picks a compatible workshop once dispatched, so any
	// job_type find_enum_item resolves is immediately queueable there.
	// Matches dfhack-plugin/protocol.h ORDER_TYPE_BY_NAME.
	OrderTypeByName uint8 = 0x00
)

// WorkOrderFrequency constants for WorkOrderDesignation.Frequency — matches
// df::workquota_frequency_type directly (library/xml/df.workquota.xml, DFHack
// 53.15-r1 checkout): OneTime=0 (also df::manager_order::frequency's own
// struct-zero default, so an omitted/legacy payload and an explicit
// WorkOrderFrequencyOneTime byte decode identically), Daily=1, Monthly=2,
// Seasonally=3, Yearly=4. Matches dfhack-plugin/protocol.h WORK_ORDER_FREQUENCY_*.
const (
	WorkOrderFrequencyOneTime    uint8 = 0x00
	WorkOrderFrequencyDaily      uint8 = 0x01
	WorkOrderFrequencyMonthly    uint8 = 0x02
	WorkOrderFrequencySeasonally uint8 = 0x03
	WorkOrderFrequencyYearly     uint8 = 0x04
)

// DigType constants (matches df::tile_dig_designation enum)
const (
	DigTypeDefault     uint8 = 0x01 // Standard mining (Default)
	DigTypeUpDownStair uint8 = 0x02 // Staircase up+down (UpDownStair)
	DigTypeChannel     uint8 = 0x03 // Dig down creating hole (Channel)
	DigTypeRamp        uint8 = 0x04 // Create ramp (Ramp)
	DigTypeDownStair   uint8 = 0x05 // Staircase down only (DownStair)
	DigTypeUpStair     uint8 = 0x06 // Staircase up only (UpStair)
)

// AckStatus constants for command acknowledgment messages
const (
	AckStatusSuccess uint8 = 0x00
	AckStatusPartial uint8 = 0x01
	AckStatusFailure uint8 = 0x02
)

// BuildType constants for build designation commands. The enum is broken
// into ranges by category so plugin-side switch statements can quickly
// route to the right handler:
//
//	0x01 – 0x0F : Constructions (wall, floor, ramp, stairs) — built from
//	              a single material reagent against a tile.
//	0x10 – 0x2F : Workshops — built from one of several material reagents
//	              over a multi-tile footprint. This includes
//	              MetalsmithsForge AND MagmaForge: DF models BOTH as
//	              Workshop subtypes (df::workshop_type), not a separate
//	              building_type — confirmed against df.building.xml, where
//	              MagmaForge (original-name LAVAMILL) sits inside the
//	              workshop_type enum-type block, not the neighboring
//	              furnace_type block, despite "Magma Forge" sounding like a
//	              furnace-family member.
//	0x30 – 0x4F : Furniture — single-tile placed buildings using one item
//	              from a stockpile.
//	0x50 – 0x6F : Doors / Hatches — single-tile portal buildings.
//	0x70 – 0x7F : Furnaces — df::building_type::Furnace (a top-level type
//	              distinct from Workshop, DFHack source-verified in
//	              df.building.xml). Multi-tile footprint like a workshop,
//	              but its own subtype enum (df::furnace_type) — all seven
//	              real values are covered: WoodFurnace, Smelter,
//	              GlassFurnace, Kiln (all four fire-safe-material), and the
//	              magma-fueled MagmaSmelter/MagmaGlassFurnace/MagmaKiln
//	              (magma-safe-material instead — see
//	              dfhack-plugin/buildings.cpp placeFurnace's doc comment for
//	              the fire-safe/magma-safe distinction and for what magma
//	              placement validation DFHack does/does not perform).
//	0x80 – 0x8F : Trade Depot — its own building_type, forced 5x5 by DF
//	              (Buildings::getCorrectSize), not a workshop/furnace.
//	0x90 – 0x9F : Misc/infrastructure (Well, Support, ArcheryTarget, the
//	              room-value furniture family Statue/Slab/WindowGlass/
//	              WindowGem/Bookcase/DisplayFurniture/OfferingPlace/
//	              Instrument, and TractionBench/NestBox/Hive) — each its
//	              own top-level building_type, forced 1x1 by DF like the
//	              doors/hatches range.
//	0xA0 – 0xAF : Water/power-transmission infrastructure (ScrewPump,
//	              GearAssembly, AxleHorizontal, AxleVertical, WaterWheel,
//	              Windmill, Rollers) — each its own top-level
//	              building_type. ScrewPump/AxleHorizontal/WaterWheel/
//	              Rollers additionally take the Orientation trailing byte
//	              (see BuildOrientation* consts); GearAssembly/AxleVertical
//	              are forced 1x1 and Windmill forced 3x3, none with an
//	              orientation concept.
//	0xB0 – 0xBF : More df::trap_type subtypes (building_type::Trap) beyond
//	              Lever, which stays at BuildTypeLever in the doors/hatches
//	              range, sharing its single-mechanism-item shape —
//	              PressurePlate, StoneFallTrap, WeaponTrap, TrackStop. See
//	              dfhack-plugin/buildings.cpp placeTrap for the exact filter
//	              shape and known limitations of each.
//
// Add new values at the end of each range as plugin support expands.
const (
	// Constructions (built from blocks or boulders).
	BuildTypeWall        uint8 = 0x01
	BuildTypeFloor       uint8 = 0x02
	BuildTypeUpStair     uint8 = 0x03
	BuildTypeDownStair   uint8 = 0x04
	BuildTypeUpDownStair uint8 = 0x05
	BuildTypeRamp        uint8 = 0x06

	// Workshops (3x3 unless noted; require a build material).
	BuildTypeWorkshopCarpenter   uint8 = 0x10
	BuildTypeWorkshopMason       uint8 = 0x11
	BuildTypeWorkshopStill       uint8 = 0x12
	BuildTypeWorkshopFarmer      uint8 = 0x13
	BuildTypeWorkshopCraftsdwarf uint8 = 0x14
	BuildTypeWorkshopMechanic    uint8 = 0x15
	BuildTypeWorkshopButcher     uint8 = 0x16
	BuildTypeWorkshopKitchen     uint8 = 0x17
	BuildTypeWorkshopFishery     uint8 = 0x18
	BuildTypeWorkshopMetalsmith  uint8 = 0x19
	BuildTypeWorkshopMagmaForge  uint8 = 0x1A // df::workshop_type::MagmaForge -- NOT a furnace_type (see range comment above); needs an anvil (magma-safe, not fire-safe) + magma-safe building material

	// Cloth/leather-industry and misc remaining workshop types -- closes out
	// "the entire cloth/leather industry is absent end to end" (a prior
	// research pass's finding). Recipes per buildings.lua workshop_inputs;
	// see dfhack-plugin/buildings.cpp placeWorkshop for the exact filter
	// shape of each. Deliberately NOT included: df::workshop_type::Tool and
	// ::Custom -- buildings.lua's workshop_inputs table has no entry for
	// either (confirmed by reading the table directly), and DFHack's own
	// gui/buildings.lua BuildingDialog excludes Tool from its default
	// workshop list the same opt-in-gated way it excludes Custom -- i.e.
	// DFHack's own canonical consumer treats Tool as belonging to the same
	// "no universal recipe" class as Custom, not merely an oversight here.
	// Both remain resolvable via BuildTypeByName (real df::workshop_type
	// keys) but yield a truthful "no placement recipe yet" ACK rather than
	// a guessed filter -- see the building_types tool.
	BuildTypeWorkshopJewelers     uint8 = 0x1B // df::workshop_type::Jewelers -- 1x generic building material
	BuildTypeWorkshopBowyers      uint8 = 0x1C // df::workshop_type::Bowyers -- 1x generic building material
	BuildTypeWorkshopSiege        uint8 = 0x1D // df::workshop_type::Siege -- 3x generic building material (quantity=3); the ammunition-prep Siege Workshop, NOT df::building_type::SiegeEngine (the catapult/ballista building, deliberately out of scope)
	BuildTypeWorkshopLeatherworks uint8 = 0x1E // df::workshop_type::Leatherworks -- 1x generic building material
	BuildTypeWorkshopTanners      uint8 = 0x1F // df::workshop_type::Tanners -- 1x generic building material
	BuildTypeWorkshopClothiers    uint8 = 0x20 // df::workshop_type::Clothiers -- 1x generic building material
	BuildTypeWorkshopLoom         uint8 = 0x21 // df::workshop_type::Loom -- 1x generic building material
	BuildTypeWorkshopKennels      uint8 = 0x22 // df::workshop_type::Kennels -- 1x generic building material
	BuildTypeWorkshopAshery       uint8 = 0x23 // df::workshop_type::Ashery -- 3 specific-item reagents (BLOCKS, empty BARREL, lye_milk_free BUCKET), no generic building-material reagent -- materialClass is rejected
	BuildTypeWorkshopDyers        uint8 = 0x24 // df::workshop_type::Dyers -- 2 specific-item reagents (empty BARREL, lye_milk_free BUCKET), no generic building-material reagent -- materialClass is rejected

	// Furniture (single-tile, requires an item from stockpile).
	BuildTypeBed     uint8 = 0x30
	BuildTypeTable   uint8 = 0x31
	BuildTypeChair   uint8 = 0x32
	BuildTypeCabinet uint8 = 0x33
	BuildTypeCoffer  uint8 = 0x34
	BuildTypeCoffin  uint8 = 0x35

	// Doors / hatches.
	BuildTypeDoor  uint8 = 0x50
	BuildTypeHatch uint8 = 0x51

	// Furnaces (3x3, forced by DF regardless of requested size).
	BuildTypeFurnaceSmelter      uint8 = 0x70 // fire-safe build material
	BuildTypeFurnaceWood         uint8 = 0x71 // fire-safe build material
	BuildTypeFurnaceKiln         uint8 = 0x72 // df::furnace_type::Kiln -- fire-safe build material
	BuildTypeFurnaceGlass        uint8 = 0x73 // df::furnace_type::GlassFurnace -- fire-safe build material
	BuildTypeFurnaceMagmaSmelter uint8 = 0x74 // df::furnace_type::MagmaSmelter -- magma-safe (NOT fire-safe) build material; see dfhack-plugin/buildings.cpp placeFurnace for magma-placement-validation details
	BuildTypeFurnaceMagmaGlass   uint8 = 0x75 // df::furnace_type::MagmaGlassFurnace -- magma-safe build material
	BuildTypeFurnaceMagmaKiln    uint8 = 0x76 // df::furnace_type::MagmaKiln -- magma-safe build material

	// Trade depot (5x5, forced by DF regardless of requested size).
	BuildTypeTradeDepot uint8 = 0x80

	// Misc/infrastructure (1x1, forced by DF like doors/hatches).
	BuildTypeWell    uint8 = 0x90
	BuildTypeSupport uint8 = 0x91

	// Room-value furniture family (1x1, forced by DF like doors/hatches;
	// placed from one already-crafted/existing item, no quality-tier
	// selection support — see dfhack-plugin/buildings.cpp
	// placeRoomValueFurniture). Closes the "craftable but not placeable"
	// gap for Statue/Slab specifically (ConstructStatue/ConstructSlab were
	// already whitelisted for Carpenters/Masons in a prior wave).
	BuildTypeStatue           uint8 = 0x92
	BuildTypeSlab             uint8 = 0x93
	BuildTypeWindowGlass      uint8 = 0x94
	BuildTypeWindowGem        uint8 = 0x95
	BuildTypeBookcase         uint8 = 0x96
	BuildTypeDisplayFurniture uint8 = 0x97
	BuildTypeOfferingPlace    uint8 = 0x98
	BuildTypeInstrument       uint8 = 0x99

	// Additional 1x1-forced-footprint building types, byte-range neighbors
	// of the two families above but not members of either: ArcheryTarget
	// takes a generic building-material filter (like Well/Support), while
	// TractionBench/NestBox/Hive share the room-value-furniture family's
	// dispatch mechanism in dfhack-plugin/buildings.cpp purely as code
	// reuse (single specific-item or tool-use filter, no materialClass
	// knob) — none of the three actually raises a bedroom's
	// furnishing-value score the way Statue/Slab/etc. do.
	BuildTypeArcheryTarget uint8 = 0x9A // df::building_type::ArcheryTarget -- 1x generic building material; marksman-dwarf training target, not justice-related
	BuildTypeTractionBench uint8 = 0x9B // df::building_type::TractionBench -- 1x TRACTION_BENCH item; hospital splint-traction furniture, crafted via ConstructTractionBench at a Mechanic's workshop (see queue_job)
	BuildTypeNestBox       uint8 = 0x9C // df::building_type::NestBox -- 1x TOOL item with has_tool_use=NEST_BOX; egg-laying animal nesting -- crafting the NEST_BOX tool item itself needs a job_item item_subtype queue_job cannot express yet (pre-existing, out of scope)
	BuildTypeHive          uint8 = 0x9D // df::building_type::Hive -- 1x TOOL item with has_tool_use=HIVE; beekeeping -- same TOOL item_subtype gap as NestBox

	// Water/power-transmission infrastructure family. Recipes per
	// buildings.lua building_inputs (dfhack-build library/lua/dfhack/
	// buildings.lua) — see dfhack-plugin/buildings.cpp
	// placeWaterPowerBuilding for the exact filter shape of each and for
	// the orientation-byte handling shared by ScrewPump/AxleHorizontal/
	// WaterWheel/Rollers (see BuildOrientation* consts above).
	//
	// KNOWN LIMITATION -- adjacency is NOT modeled: DF links two touching
	// machine buildings (an axle end abutting a gear assembly's tile, a
	// gear abutting a water wheel, etc.) automatically at the ENGINE
	// level purely from tile adjacency once both exist and the fort is
	// unpaused — there is no separate "connect A to B" parameter to set
	// at placement time, so placement here is exactly as automatable as
	// vanilla DF's own build UI: place each piece touching its intended
	// neighbor and DF's machine-network code (not this plugin) does the
	// rest. Neither the plugin nor this package can verify two placed
	// pieces actually formed one working machine short of a live
	// in-game check.
	BuildTypeScrewPump      uint8 = 0xA0 // df::building_type::ScrewPump -- BLOCKS + screw (TRAPCOMP) + pipe (PIPE_SECTION); orientation = intake side
	BuildTypeGearAssembly   uint8 = 0xA1 // df::building_type::GearAssembly -- 1x mechanism (TRAPPARTS); no orientation
	BuildTypeAxleHorizontal uint8 = 0xA2 // df::building_type::AxleHorizontal -- WOOD; orientation = axis (horizontal vs vertical); ALWAYS 1 tile long today -- see dfhack-plugin/buildings.cpp placeWaterPowerBuilding's KNOWN LIMITATION 2 (no caller-chosen length parameter yet; DF itself supports a multi-tile line)
	BuildTypeAxleVertical   uint8 = 0xA3 // df::building_type::AxleVertical -- 1x WOOD; no orientation (single-tile Z-shaft)
	BuildTypeWaterWheel     uint8 = 0xA4 // df::building_type::WaterWheel -- 3x WOOD; orientation = axis (horizontal vs vertical)
	BuildTypeWindmill       uint8 = 0xA5 // df::building_type::Windmill -- 4x WOOD; no orientation, forced 3x3
	BuildTypeRollers        uint8 = 0xA6 // df::building_type::Rollers -- mechanism (TRAPPARTS) + CHAIN; orientation = push direction; ALWAYS 1 tile long today (same limitation as AxleHorizontal)

	// More df::trap_type subtypes (building_type::Trap) beyond
	// BuildTypeLever below -- see dfhack-plugin/buildings.cpp placeTrap for
	// the exact filter shape of each and for known limitations
	// (StoneFallTrap builds unarmed; PressurePlate's trigger-condition
	// fields are left at DF's raw constructor defaults; TrackStop is basic
	// placement only, no minecart track-piece linkage).
	BuildTypePressurePlate uint8 = 0xB0 // df::trap_type::PressurePlate -- 1x mechanism (TRAPPARTS), same shape as Lever; can also serve as a link_building SOURCE, same as Lever (see LinkBuildingDesignation)
	BuildTypeStoneFallTrap uint8 = 0xB1 // df::trap_type::StoneFallTrap -- 1x mechanism (TRAPPARTS), same shape as Lever; builds UNARMED -- arming with a boulder is DF's own separate post-construction Load Stone Trap job, not queued by this command
	BuildTypeWeaponTrap    uint8 = 0xB2 // df::trap_type::WeaponTrap -- 2x reagents: mechanism (TRAPPARTS) + weapon/trap-component (ANY_WEAPON) -- armed at construction time (unlike StoneFallTrap)
	BuildTypeTrackStop     uint8 = 0xB3 // df::trap_type::TrackStop -- 1x generic building material, same shape as Support/ArcheryTarget; anchors minecart track infrastructure -- basic placement only
)

// BuildTypeByName is not one of the curated BuildType values above — it's
// the sentinel for BuildDesignation's generalized name-based path (see
// that type's doc comment below). Set BuildTypeName to a DFHack
// building_type/workshop_type/furnace_type/trap_type enum key name (e.g.
// "Statue", "Jewelers", "StoneFallTrap") to reach any building type the
// plugin can resolve by name (dfhack-plugin/buildings.cpp:
// resolveBuildTypeByName, via DFHack's find_enum_item tried against all
// four enums in turn) — no new BuildType byte or plugin rebuild needed for
// a type DFHack already knows about. Matches dfhack-plugin/protocol.h
// BUILD_TYPE_BY_NAME. Discover resolvable, ACTUALLY BUILDABLE names via the
// building_types tool.
//
// KNOWN LIMITATION (mirrors OrderTypeByName/work_orders.cpp
// resolveJobTypeByName): resolving a name to a real DFHack building_type/
// workshop_type/furnace_type/trap_type does NOT by itself mean the plugin
// has a placement recipe (job_item filters) for it yet — only names the
// building_types tool lists are buildable today; anything else fails with
// a truthful "resolved but no placement recipe" error.
const BuildTypeByName uint8 = 0x00

// BuildTypeLever and BuildTypeFloodgate share the doors/hatches wire range
// (0x50-0x6F, isBuildTypeDoor) — both are 1x1 ACTUAL buildings taking one
// specific pre-made item, same shape as Door/Hatch (dfhack-plugin/
// buildings.cpp: placeDoor). Lever needs 1 mechanism (TRAPPARTS) at build
// time; Floodgate needs 1 FLOODGATE item and consumes NO mechanism until
// it is later linked to a lever (see LinkBuildingDesignation).
const (
	BuildTypeLever     uint8 = 0x52
	BuildTypeFloodgate uint8 = 0x53
)

// BridgeDirection* constants for BuildBridgeDesignation.Direction — DF-AI's
// own wire values, translated by the plugin (buildings.cpp: placeBridge)
// to df::building_bridgest::T_direction (Retracting=-1, Left=0, Right=1,
// Up=2, Down=3). Matches dfhack-plugin/protocol.h BRIDGE_DIR_* constants.
// Names describe the visible effect: Up raises to North, Right raises to
// East, Down raises to South, Left raises to West; Retract slides the
// bridge away instead of raising it vertically.
const (
	BridgeDirectionRetract uint8 = 0x00
	BridgeDirectionRaiseN  uint8 = 0x01
	BridgeDirectionRaiseS  uint8 = 0x02
	BridgeDirectionRaiseE  uint8 = 0x03
	BridgeDirectionRaiseW  uint8 = 0x04
)

// IsBuildTypeWorkshop reports whether the given BuildType refers to a
// workshop. Useful in routing tables.
func IsBuildTypeWorkshop(t uint8) bool { return t >= 0x10 && t < 0x30 }

// IsBuildTypeFurniture reports whether the given BuildType refers to a
// piece of furniture.
func IsBuildTypeFurniture(t uint8) bool { return t >= 0x30 && t < 0x50 }

// IsBuildTypeConstruction reports whether the given BuildType refers to
// a construction (wall, floor, etc.).
func IsBuildTypeConstruction(t uint8) bool { return t >= 0x01 && t < 0x10 }

// IsBuildTypeDoor reports whether the given BuildType refers to a door
// or hatch.
func IsBuildTypeDoor(t uint8) bool { return t >= 0x50 && t < 0x70 }

// IsBuildTypeFurnace reports whether the given BuildType refers to a
// furnace (df::building_type::Furnace — Smelter, WoodFurnace, ...).
func IsBuildTypeFurnace(t uint8) bool { return t >= 0x70 && t < 0x80 }

// IsBuildTypeDepot reports whether the given BuildType refers to a
// trade depot.
func IsBuildTypeDepot(t uint8) bool { return t >= 0x80 && t < 0x90 }

// IsBuildTypeInfra reports whether the given BuildType refers to a
// misc/infrastructure building (Well, Support, or the room-value furniture
// family Statue/Slab/WindowGlass/WindowGem/Bookcase/DisplayFurniture/
// OfferingPlace/Instrument).
func IsBuildTypeInfra(t uint8) bool { return t >= 0x90 && t < 0xA0 }

// IsBuildTypeWaterPower reports whether the given BuildType refers to a
// water/power-transmission infrastructure building (ScrewPump,
// GearAssembly, AxleHorizontal, AxleVertical, WaterWheel, Windmill,
// Rollers).
func IsBuildTypeWaterPower(t uint8) bool { return t >= 0xA0 && t < 0xB0 }

// IsBuildTypeTrap reports whether the given BuildType refers to one of the
// df::trap_type subtypes in this range (PressurePlate, StoneFallTrap,
// WeaponTrap, TrackStop). Lever lives in the doors/hatches range instead
// (IsBuildTypeDoor) — see BuildTypeLever's doc comment.
func IsBuildTypeTrap(t uint8) bool { return t >= 0xB0 && t < 0xC0 }

// Region represents a 3D bounding box for designations
type Region struct {
	X1, Y1, Z1 int16 // Start coordinates
	X2, Y2, Z2 int16 // End coordinates (inclusive)
}

// BuildDesignation represents a single build command. BuildTypeName is only
// used when BuildType == BuildTypeByName (see that constant's doc comment
// above) — ignored, and left empty on the wire, for the curated
// BuildType* vocabulary.
type BuildDesignation struct {
	X, Y, Z       int16  // Build location
	BuildType     uint8  // Type of construction, or BuildTypeByName
	Material      uint8  // MaterialClass* constraint (MaterialClassAny = no preference)
	Quality       uint8  // QualityTier* constraint (QualityTierAny = no preference); furniture only
	Orientation   uint8  // BuildOrientation* constraint (BuildOrientAny = no preference); water/power infra only (ScrewPump/AxleHorizontal/WaterWheel/Rollers)
	BuildTypeName string // DFHack building_type/workshop_type/furnace_type/trap_type enum key name; only used when BuildType == BuildTypeByName
}

// ZoneDesignation represents a zone assignment command
type ZoneDesignation struct {
	X1, Y1, Z int16 // Start coordinates
	X2, Y2    int16 // End coordinates (same Z-level)
	ZoneType  uint8 // Bedroom, dining, etc.
}

// AssignZoneDesignation targets the zone at (X,Y,Z) and assigns UnitID.
type AssignZoneDesignation struct {
	X, Y, Z int16
	UnitID  int32
}

// UnassignZoneDesignation targets the zone at (X,Y,Z) and removes UnitID's assignment.
type UnassignZoneDesignation struct {
	X, Y, Z int16
	UnitID  int32
}

// RemoveZoneDesignation targets the civzone occupying (X,Y,Z) for immediate
// deconstruction (Buildings::deconstruct takes the on_civzone_delete
// immediate-removal branch for civzones — no dwarf labor involved, unlike
// RemoveBuildingDesignation's constructed buildings). Rejected plugin-side
// if the zone founds a Location (location_id set) — cascade behavior there
// is unconfirmed, so unassign/retire the Location first.
type RemoveZoneDesignation struct {
	X, Y, Z int16
}

// FarmPlotDesignation designates a rectangular farm plot at (X1,Y1)-(X2,Y2)
// on level Z — extent-shaped like a stockpile (StockpileDesignation minus
// GroupMask), NOT a single-tile build. Needs open, non-aquatic soil or mud
// floor; DF itself enforces that at placement time. A freshly built plot
// grows NOTHING until SetFarmCropDesignation assigns a crop to at least one
// season slot.
type FarmPlotDesignation struct {
	X1, Y1, Z int16
	X2, Y2    int16
}

// SetFarmCropDesignation programs one (Season != FarmSeasonAll) or all four
// (Season == FarmSeasonAll) season slots of the farm plot at (X,Y,Z) to
// grow CropName — a plant raw token or display name (see the list_crops
// query), or the literal string "fallow" to clear the slot(s).
type SetFarmCropDesignation struct {
	X, Y, Z  int16
	Season   uint8
	CropName string
}

// DesignateBurrowDesignation creates the burrow named Name if it doesn't
// exist yet (findByName is exact, case-sensitive — matches DFHack's own
// dfhack.burrows.findByName), then paints the tile rect (X1,Y1,Z1)-(X2,Y2,Z2)
// into it. Repeatable: calling again with the same Name adds more tiles to
// the same burrow instead of creating a second one. Z1 may differ from Z2
// for a cheap multi-level paint in one call; DF map bounds are a single
// rectangular prism so if both corners are in-bounds the whole enclosed box
// is too — no per-tile bounds skipping needed once the corners are
// validated. Burrows paint through hidden tiles by design (same house rule
// as dig designations — DF's own burrow UI does this too), so there is no
// floor/walkability check unlike DesignateZone.
type DesignateBurrowDesignation struct {
	Name       string
	X1, Y1, Z1 int16
	X2, Y2, Z2 int16
}

// RemoveBurrowDesignation deletes the named burrow entirely — clears its
// tiles and unit assignments first (matching quickfort's burrow.lua
// deletion path), detaches it from the civilian alert's burrow set if it
// was a member, then frees the struct and removes it from
// plotinfo->burrows.list. Not reversible; a burrow removed this way has no
// name-based way back (a fresh designate_burrow with the same name creates
// an unrelated new burrow with a new id).
type RemoveBurrowDesignation struct {
	Name string
}

// AssignBurrowDesignation assigns (Assign=true) or unassigns (Assign=false)
// units to/from the named burrow. Set AllCitizens=true to target every
// current citizen (Units::isCitizen — sane, non-dead, current-fort) in one
// call instead of a single UnitID; UnitID is ignored when AllCitizens=true.
type AssignBurrowDesignation struct {
	Name        string
	Assign      bool
	AllCitizens bool
	UnitID      int32
}

// SetAlertDesignation sounds (Active=true) or clears (Active=false) DF's
// v50 civilian alert against the named burrow. Active=true adds the burrow
// to the alert's restriction set (if not already a member) and turns the
// alarm on if it wasn't already sounding; Active=false removes the burrow
// from that set and auto-clears the alarm if the set becomes empty as a
// result (mirrors scripts/gui/civ-alert.lua's remove_civalert_burrow
// exactly). This is DF's vanilla civilian-alert mechanism (see
// dfhack-build/scripts/docs/gui/civ-alert.rst): while active, ALL
// non-military citizens rush to the burrow and are confined there,
// regardless of whether AssignBurrowDesignation was ever used on it — no
// per-unit assignment is required or checked. Deactivate promptly once the
// danger passes; leaving it active keeps every civilian confined and can
// starve/unhappy them (per the same doc).
type SetAlertDesignation struct {
	Name   string
	Active bool
}

// CreateLocationDesignation targets the MeetingHall civzone at (X,Y,Z)
// and converts it into a Location of LocationType. Profession is
// required only when LocationType is LocationTypeGuildhall.
type CreateLocationDesignation struct {
	X, Y, Z      int16
	LocationType uint8
	Profession   string
}

// AssignLodgingDesignation links the Bedroom civzone at
// (BedroomX,BedroomY,BedroomZ) as guest lodging inside the Tavern
// Location founded by the civzone at (TavernX,TavernY,TavernZ).
type AssignLodgingDesignation struct {
	TavernX, TavernY, TavernZ    int16
	BedroomX, BedroomY, BedroomZ int16
}

// UnassignLodgingDesignation removes the Bedroom civzone at
// (BedroomX,BedroomY,BedroomZ) from whichever tavern it's lodging for.
type UnassignLodgingDesignation struct {
	BedroomX, BedroomY, BedroomZ int16
}

// LinkBuildingDesignation wires the lever OR pressure plate at
// (LeverX,LeverY,LeverZ) to the trigger target (bridge/floodgate/door/
// hatch/support/gear_assembly) at (TargetX,TargetY,TargetZ) — DF's
// mechanism-linking mechanism. Field names stay Lever*-prefixed for
// wire/history continuity, but the plugin (dfhack-plugin/mechanisms.cpp
// applyLinkBuilding) accepts either trap_type::Lever or
// trap_type::PressurePlate as the source since 2026-07-18 — both share the
// same underlying building_trapst struct and linked_mechanisms field.
// Support (collapse trigger) and GearAssembly (power shutoff) joined the
// accepted TARGET set in a later 2026-07-18 pass — both carry their own
// dedicated mechanism-response bitfield (support_flags.bits.triggered,
// gear_flags.bits.disengaged) confirming they're genuinely triggerable,
// same as bridge/floodgate/door/hatch. Consumes two free mechanisms
// (TRAPPARTS items) from the fort's stockpiles; rejected with a truthful
// error if fewer than two are available, if either building is still under
// construction, or if the target isn't a supported trigger type.
type LinkBuildingDesignation struct {
	LeverX, LeverY, LeverZ    int16
	TargetX, TargetY, TargetZ int16
}

// PullLeverDesignation queues DF's real PullLever job against the lever
// at (X,Y,Z).
type PullLeverDesignation struct {
	X, Y, Z int16
}

// BuildBridgeDesignation designates a rectangular bridge at
// (X1,Y1)-(X2,Y2) on level Z, raising/retracting toward Direction
// (BridgeDirection* constants). Bridge is rectangle-shaped (like
// FarmPlotDesignation/StockpileDesignation) rather than a fixed-footprint
// building, and needs its own wire command because BuildDesignation's
// single-tile shape has no room for a second corner or a direction byte.
type BuildBridgeDesignation struct {
	X1, Y1, Z int16
	X2, Y2    int16
	Direction uint8
}

// SetWorkshopProfileDesignation writes DF's workshop_profile struct
// (df.building.xml struct-type workshop_profile: permitted_workers vector,
// min_level, max_level) on the ACTUAL BUILT workshop, furnace, or
// mechanism trap (Lever) occupying (X,Y,Z) -- df::building::
// getWorkshopProfile() returns null for anything else. This is DF's real
// mechanism for probabilistically improving job output quality (gate a
// shop to a legendary worker), the complement to BuildDesignation.Quality
// (which picks a specific already-existing item at placement time).
//
// MinSkillLevel/MaxSkillLevel are raw df::skill_rating tier indices
// (DFHack-only enum, dfhack-build/library/xml/df.dfhack.xml: 0=Dabbling
// .. 20=Legendary+5). MaxSkillLevel == -1 means uncapped (translated to
// DF's own 3000 sentinel, dfhack-build/plugins/lua/orders.lua
// MAX_SKILL_RATINGS[#MAX_SKILL_RATINGS]).
//
// WorkerUnitID, if >= 0, is appended to permitted_workers if not already
// present -- a non-empty permitted_workers list restricts the shop to
// ONLY listed workers, overriding the skill range entirely for them
// (DF's own vanilla semantics). -1 means "don't touch permitted_workers".
type SetWorkshopProfileDesignation struct {
	X, Y, Z       int16
	MinSkillLevel int32
	MaxSkillLevel int32
	WorkerUnitID  int32
}

// UnsuspendDesignation represents a single-tile unsuspend command, used to
// resume a stalled construction (wall placement, building, etc.) after the
// agent has cleared whatever blocker caused DF to auto-suspend it.
type UnsuspendDesignation struct {
	X, Y, Z int16
}

// RemoveBuildingDesignation represents a single-tile building removal
// command: mark the building occupying (X,Y,Z) for deconstruction. Any
// tile of a multi-tile building's footprint works. Dwarves do the actual
// teardown over game time.
type RemoveBuildingDesignation struct {
	X, Y, Z int16
}

// QueueJobDesignation represents a single job queued directly at the
// workshop occupying (X,Y,Z) — the same mechanism a player uses when
// right-clicking a workshop and picking a task from its build menu.
// Bypasses the manager/work-order system entirely: no Manager noble or
// office needed, unlike WorkOrderDesignation. Use this for an immediate,
// one-off need; use WorkOrderDesignation for standing/bulk production once
// a manager exists. Call again to queue more than one.
//
// OrderType is normally one of the OrderType* byte constants — the same
// ~11-entry hand-maintained vocabulary WorkOrderDesignation uses. Set it
// to OrderTypeByName instead to reach ANY DFHack job_type the plugin can
// resolve by name (dfhack-plugin/work_orders.cpp: resolveJobTypeByName,
// via DFHack's find_enum_item) — no new OrderType byte or plugin rebuild
// needed for a job_type DFHack already knows about. JobTypeName is the
// DFHack job_type enum key name (e.g. "ConstructHatchCover", not the raw
// bay12 token) and is ignored unless OrderType == OrderTypeByName.
//
// Set OrderType to OrderTypeCustomReaction instead to reach a raw-defined
// df::reaction directly by its reaction CODE (e.g.
// "BREW_DRINK_FROM_PLANT") — the mechanism behind brewing, and anything
// else with no job_type mapping at all. ReactionCode is ignored unless
// OrderType == OrderTypeCustomReaction. Only QueueJobDesignation supports
// either generalized path; WorkOrderDesignation does not.
//
// Material is a raw, unresolved caller token (mirrors JobTypeName/
// ReactionCode's pass-through convention) — either a job_material_category
// keyword (wood, bone, shell, leather, silk, plant, cloth, yarn) or an exact
// DFHack material token (e.g. "INORGANIC", "INORGANIC:LIMONITE", "COAL"),
// resolved plugin-side (dfhack-plugin/work_orders.cpp). Required for
// OrderType naming SmeltOre (the direct-queue path needs an exact ore raw to
// pin job.mat_type/mat_index and the BOULDER job_item — see work_orders.cpp
// applyQueueJob's SmeltOre case); ignored for job types with no material
// ambiguity. Wire shape: unconditionally appended [2:MaterialLen][N:Material]
// after the existing (conditional) name/reaction tail — old encoded payloads
// missing this trailer decode with Material="".
//
// Subtype is the item-SUBTYPE pinning wave's addition (docs/decisions.md,
// follows the Material entry above): a bare raws itemdef `id` token (e.g.
// "ITEM_WEAPON_PICK" — no "WEAPON:" type prefix; work_orders.cpp derives
// item_type from the job_type itself), resolved plugin-side onto
// job->item_type/item_subtype. Required for OrderType naming MakeWeapon,
// MakeArmor, or MakeTool (a bare job_type name has no way to pick which
// weapon/armor/tool to forge, the same reasoning as SmeltOre's Material
// requirement); ignored for every other job type. Discover valid tokens via
// the job_types tool's subtype_of param. Wire shape: unconditionally
// appended [2:SubtypeLen][N:Subtype] after the existing Material trailer —
// old encoded payloads missing this trailer decode with Subtype="".
type QueueJobDesignation struct {
	X, Y, Z      int16
	OrderType    uint8  // What to produce (OrderTypeMakeBed etc.), OrderTypeByName, or OrderTypeCustomReaction
	JobTypeName  string // DFHack job_type enum key name; only used when OrderType == OrderTypeByName
	ReactionCode string // df::reaction.code; only used when OrderType == OrderTypeCustomReaction
	Material     string // job_material_category keyword or DFHack material token; required for SmeltOre, else optional
	Subtype      string // bare raws itemdef token (e.g. ITEM_WEAPON_PICK); required for MakeWeapon/MakeArmor/MakeTool, else ignored
}

// SetLaborDesignation represents a single labor toggle on one unit.
// LaborID is a Labor* constant (the real df::unit_labor enum index,
// transmitted directly). Enable=true turns the labor on, false turns it
// off. A labor a unit's caste can't perform is a graceful no-op on DF's
// side — the plugin doesn't reject it.
type SetLaborDesignation struct {
	UnitID  int32
	LaborID uint8
	Enable  bool
}

// AssignWorkDetailDesignation adds (Add=true) or removes (Add=false)
// UnitID from the work detail at DetailIndex's assigned_units
// (df::work_detail.assigned_units) -- DF's own work-details membership
// list, the authoritative store behind a unit's derived status.labors
// cache (see SetLaborDesignation's doc comment). DetailIndex is a
// position in the work_details tool's response, not a stable ID --
// work_details carries no ID field of its own, so a caller that deletes
// or reorders details between calls must re-resolve the index.
type AssignWorkDetailDesignation struct {
	DetailIndex uint16
	UnitID      int32
	Add         bool
}

// SetWorkDetailModeDesignation changes the work detail at DetailIndex's
// mode (df::work_detail_mode, via its flags.bits.mode subfield). A mode
// change reshuffles every current citizen's derived labors fort-wide, not
// just the detail's own assigned_units membership.
type SetWorkDetailModeDesignation struct {
	DetailIndex uint16
	Mode        uint8 // WorkDetailMode* constant
}

// CreateWorkDetailDesignation allocates a new custom work detail into the
// first free CUSTOM_1..CUSTOM_8 icon slot (df::work_detail_icon_type) --
// a truthful FAILED ack results if all 8 are already in use. LaborIDs is
// a list of Labor* constants (the real df::unit_labor enum index) to
// pre-enable on the new detail's allowed_labors; may be empty (an
// empty-labor detail is legal DF state, just useless until edited
// further via AssignWorkDetailDesignation/SetWorkDetailModeDesignation).
type CreateWorkDetailDesignation struct {
	Name     string
	Mode     uint8 // WorkDetailMode* constant
	LaborIDs []uint8
}

// WorkOrderDesignation represents a manager work order: produce N items of
// the given type. The manager dispatches to whichever workshop can fulfill
// the order, drawing reagents from stockpiles automatically.
//
// OrderType is normally one of the OrderType* byte constants. Set it to
// OrderTypeByName instead to reach ANY DFHack job_type the plugin can
// resolve by name (dfhack-plugin/work_orders.cpp: resolveJobTypeByName,
// via DFHack's find_enum_item) — no new OrderType byte or plugin rebuild
// needed for a job_type DFHack already knows about. This is structurally
// the cheapest way to reach job types with no ORDER_TYPE_* byte at all,
// e.g. the Farmers-workshop economy (ProcessPlants, MakeCheese,
// MilkCreature, ShearCreature, SpinThread): unlike QueueJobDesignation's
// by-name path, no workshop-compatibility or material-class filter is
// needed on this side at all — the manager itself fills in job_items and
// picks a compatible workshop once it dispatches the order. JobTypeName is
// the DFHack job_type enum key name (e.g. "ProcessPlants", not the raw
// bay12 token) and is ignored unless OrderType == OrderTypeByName.
// WorkOrderDesignation does NOT support OrderTypeCustomReaction (the
// reaction-code path) — that remains QueueJobDesignation-only;
// OrderTypeBrewDrink is still the one reaction-backed order type this path
// supports.
//
// Material is a raw, unresolved caller token — a job_material_category
// keyword (wood, bone, shell, leather, silk, plant, cloth, yarn) or an exact
// DFHack material token (e.g. "INORGANIC", "INORGANIC:LIMONITE") — resolved
// plugin-side onto the manager_order's material_category bitfield or
// mat_type/mat_index pair (dfhack-plugin/work_orders.cpp applyWorkOrder).
// Needed for job types with real material ambiguity (e.g. MakeFigurine's
// wood/stone/metal choice, which also picks the workshop); a job type with
// none (PrepareMeal, ConstructBlocks' generic-INORGANIC default) needs no
// Material at all. Frequency is a WorkOrderFrequency* constant selecting a
// recurring cadence (df::workquota_frequency_type) instead of the default
// one-time order. Wire shape: both are unconditionally appended
// ([2:MaterialLen][N:Material] then [1:Frequency]) after the existing
// (conditional) name tail — old encoded payloads missing this trailer
// decode with Material="" and Frequency=WorkOrderFrequencyOneTime.
//
// Subtype is the item-SUBTYPE pinning wave's addition (docs/decisions.md,
// follows the Material/Frequency entries above): a bare raws itemdef `id`
// token (e.g. "ITEM_WEAPON_PICK"), resolved plugin-side onto
// manager_order::item_type/item_subtype. "" is a legal no-op ("let the
// manager pick", the pre-existing behavior) — unlike QueueJobDesignation's
// own Subtype, this is NOT required for any particular job type: applyWorkOrder
// needs no workshop-compatibility whitelist at all, so any job type whose
// ENUM_ATTR(item) resolves a real itemdef vector can be pinned here.
// Discover valid tokens via the job_types tool's subtype_of param. Wire
// shape: unconditionally appended [2:SubtypeLen][N:Subtype] after the
// existing Frequency byte — old encoded payloads missing this trailer
// decode with Subtype="".
type WorkOrderDesignation struct {
	OrderType   uint8  // What to produce (OrderTypeMakeBed etc.), or OrderTypeByName
	Quantity    uint16 // How many to produce (1..100)
	JobTypeName string // DFHack job_type enum key name; only used when OrderType == OrderTypeByName
	Material    string // job_material_category keyword or DFHack material token; optional
	Frequency   uint8  // WorkOrderFrequency* constant; default WorkOrderFrequencyOneTime
	Subtype     string // bare raws itemdef token (e.g. ITEM_WEAPON_PICK); optional, pins item_type/item_subtype when set
}

// BringGoodsToDepotDesignation marks up to MaxCount free fort items for
// hauling to the built trade depot at (X,Y,Z) -- DFHack's own
// Items::markForTrade commit (library/modules/Items.cpp:1912), the same
// mechanism scripts/internal/caravan/movegoods.lua uses, reached here
// without the viewscreen. Filter-based, not ID-based (no tool today
// surfaces individual item IDs — see depot_goods/stocks): ItemTypeFilter
// and MaterialFilter are optional case-sensitive/case-insensitive
// substring matches (respectively) against the plugin's own decoded
// item_type name and material state_name — "" matches everything, mirroring
// stockpile_inventory's category filter (dfhack-plugin/queries.cpp
// handleStockpileInventory) rather than requiring an exact DFHack enum key.
//
// MaxCount is a required cap (>=1) — this is a bulk filter-driven action
// with real consequences (arbitrarily many fort items marked away), so
// there is no "unlimited" sentinel; pass a large number to approximate one.
// MaxTotalValue caps the running total estimated value of marked items;
// <= 0 means no value cap. Reachability (DFHack's own
// Maps::canWalkBetween) and DF's own build-stage/pending-removal checks on
// the depot are additional gates — see dfhack-plugin/trade.cpp
// applyBringGoodsToDepot for the full eligibility rules and KNOWN
// LIMITATIONs (nested/carried items and items already committed to some
// other building are not reached).
type BringGoodsToDepotDesignation struct {
	X, Y, Z        int16
	ItemTypeFilter string
	MaterialFilter string
	MaxCount       int32
	MaxTotalValue  int64
}

// AppointPositionDesignation appoints (or replaces the holder of) one
// entity_position_assignment slot -- see docs/decisions.md (2026-07-19
// nobles research pass) for the full data model. PositionCode is DFHack's
// own entity_position.code token (e.g. "MANAGER", "BOOKKEEPER", "BROKER",
// "SHERIFF") -- discover live vacant/appointable codes via the
// position_vacancies query/tool rather than a hardcoded list here, per this
// project's house rule for enum-like params with per-value facts (the
// vacancies view IS the discovery tool). Only fills an EXISTING assignment
// slot DF itself already created (vacant, or currently held for a
// replacement); a position with no assignment record at all fails
// truthfully instead of fabricating one -- see dfhack-plugin/nobles.cpp
// applyAppointPosition for the exact mutation sequence (mirrors DFHack's
// own scripts/make-monarch.lua). Restricted to current citizens
// (Units::isCitizen) as a safety net independent of raw-defined caste
// eligibility rules. Justice-mechanic consequences of Sheriff/Captain of
// the Guard appointment are explicitly out of scope -- only the
// appointment write itself.
type AppointPositionDesignation struct {
	UnitID       int32
	PositionCode string
}

// SetBookkeeperPrecisionDesignation writes plotinfo->nobles.
// bookkeeper_settings (df::record_precision_level_type) -- the goal
// precision the Nobles screen lets the player pick for the appointed
// Bookkeeper's record-keeping. UNVERIFIED (see nobles research pass):
// whether DF clamps/ignores a precision beyond what the current
// bookkeeper's Appraisal skill supports (the in-game UI grays out unearned
// options; this direct write bypasses that gate).
type SetBookkeeperPrecisionDesignation struct {
	Precision uint8 // BookkeeperPrecision* constant
}

// CreateSquadDesignation fills (or, if none exists yet, mints) a vacant
// entity_position_assignment slot for PositionCode (a df::entity_position.
// code token on the fort's own historical_entity, e.g. "MILITIA_CAPTAIN" --
// discover other codes with squad_size > 0 via the existing
// position_vacancies tool) and calls DFHack's own Military::makeSquad on
// it. PositionCode == "" defaults to "MILITIA_CAPTAIN", the position
// vanilla DF's own [SQUAD:...] raw token attaches to.
//
// Two-phase per docs/decisions.md (2026-07-19 military research pass):
//  1. If an entity_position_assignment already exists for this position
//     with squad_id == -1 (vacant), reuse it -- this is the common case
//     once at least one prior squad-leader slot has ever existed.
//  2. Otherwise (the fresh-embark case -- nothing forces an assignment
//     record to exist before the first squad), MINT a brand-new one:
//     allocate, assign id from positions.next_assignment_id, wire
//     position_id, leave histfig/histfig2/squad_id at the codegen
//     constructor's own -1 defaults. UNVERIFIED, highest risk: no DFHack
//     code anywhere in the 53.15-r2 checkout does this write; it is
//     inferred from entity_position_assignment's struct shape and
//     df::create_squad_interfacest's own candidate-list field (proving the
//     closed DF binary treats this as a distinct step) -- not confirmed
//     against any known-working DFHack script. The plugin's success ACK
//     says so explicitly when this path is taken, rather than claiming
//     unearned certainty; live-verify (Squads/Nobles screen) before
//     leaning on it in a real fort.
type CreateSquadDesignation struct {
	PositionCode string
}

// AssignSquadDesignation adds (Add=true) or removes (Add=false) UnitID
// from SquadID's membership -- thin wrapper around DFHack's own
// Military::addToSquad/removeFromSquad, both proven-safe (real callers in
// scripts/autotraining.lua at this exact tag). Add auto-picks the first
// free NON-commander slot (position 0 is never auto-assignable via this
// path -- addToSquad itself refuses it; assigning a commander is out of
// scope for this minimal surface). Remove only needs the unit id
// (Military::removeFromSquad's own signature) -- SquadID is still carried
// so the plugin can cross-check the unit is actually in the squad the
// caller thinks it's in, not because the underlying DFHack call requires
// it. Civilian labors are NOT auto-disabled when a unit joins a squad (v50
// does not do this -- see scripts/uniform-unstick.lua's own warning about
// this exact conflict); use the existing set_labor tool if a dedicated,
// non-working soldier is wanted.
type AssignSquadDesignation struct {
	SquadID int32
	UnitID  int32
	Add     bool
}

// SquadOrderDesignation replaces SquadID's entire orders queue with at
// most one order, or clears it entirely -- see SquadOrder* constants.
// Type selects the operation:
//   - SquadOrderStation: builds a squad_order_movest targeting X/Y/Z.
//   - SquadOrderDefendBurrow: builds a squad_order_defend_burrowsst
//     referencing the burrow named BurrowName (must already exist --
//     see designate_burrow).
//   - SquadOrderCancel: clears the queue with no replacement.
//
// The plugin always clears any existing orders first (mirrors DFHack's
// own Military.cpp room-removal deletion idiom: delete each pointer, then
// clear the vector) before pushing at most one new order -- this
// sidesteps the UNVERIFIED question of how DF actually processes a
// multi-entry squad->orders queue, since no code in this checkout ever
// reads or writes that vector outside construction. X/Y/Z are ignored
// unless Type == SquadOrderStation; BurrowName is ignored unless Type ==
// SquadOrderDefendBurrow.
type SquadOrderDesignation struct {
	SquadID    int32
	Type       uint8
	X, Y, Z    int16
	BurrowName string
}

// CancelOrderDesignation deletes ONE manager_order (OrderID) entirely --
// see docs/decisions.md (2026-07-19 manager-work-order-lifecycle research
// pass, Q1) for the confirmed-safe basis and
// dfhack-plugin/work_orders.cpp's applyCancelOrder for the exact sequence:
// every job the order already spawned is cancelled (matched via
// df::job.order_id, since a manager_order carries no back-pointer to its
// own jobs), the order's own item_conditions/order_conditions/items
// pointers are freed (mirrors DFHack's own orders_clear_command, the only
// in-tree removal path, applied per-order instead of to every order at
// once), the order object is deleted and erased, and finally every
// surviving order's own order_conditions are scanned for a dangling
// dependency reference to the deleted id (cleaned up too, since nothing
// in-tree does this on its own).
type CancelOrderDesignation struct {
	OrderID int32
}

// EditOrderDesignation changes an existing, already-queued manager_order's
// amount_total/amount_left and/or frequency IN PLACE -- see
// docs/decisions.md (2026-07-19 manager-work-order-lifecycle research
// pass, Q1) for the confirmed-safe basis. HasAmount/HasFrequency gate each
// edit independently; at least one must be set (Validate rejects neither).
//
// Amount, when HasAmount, is the NEW desired amount_total (1..100, the same
// range WorkOrderDesignation.Quantity validates at creation) -- NOT a
// delta. The plugin mirrors DFHack's own scripts/workorder.lua mutation
// shape exactly: amount_left += (new_total - old_total); amount_total =
// new_total -- so progress already made toward the order is preserved
// rather than reset. If that leaves amount_left <= 0, the order is fully
// satisfied by this edit and gets deleted outright (the same cleanup
// CancelOrderDesignation uses), matching workorder.lua's own "delete once
// amount_left <= 0" completion behavior; the plugin's ACK says so
// explicitly rather than claiming a bare "edited" when the order no longer
// exists. Amount == 0 is deliberately NOT accepted as the "infinite/
// repeating" sentinel some freshly-created orders carry (Q1 flagged that
// sentinel's edit-time interplay with amount_left as unverified) --
// EditOrderDesignation always requires 1..100, same as order creation.
//
// Frequency, when HasFrequency, is a WorkOrderFrequency* constant. Changing
// it also resets the order's finished_year/finished_year_tick checkpoint
// fields to -1 (their own struct-default init value) -- per Q1's own
// recommendation: no in-tree DFHack code mutates frequency on an
// already-active order, so whether DF's manager tolerates a stale
// checkpoint after an in-place change otherwise is UNVERIFIED.
type EditOrderDesignation struct {
	OrderID      int32
	HasAmount    bool
	Amount       uint16 // new amount_total (1..100); ignored unless HasAmount
	HasFrequency bool
	Frequency    uint8 // WorkOrderFrequency* constant; ignored unless HasFrequency
}

// StockpileDesignation represents a stockpile zone designation. The
// GroupMask is a bitfield of StockpileGroup* constants identifying which
// item categories the stockpile will accept at the top-level UI grouping.
//
// IMPORTANT (current limitation): we set the top-level group flags but
// not the per-material sub-flags. The stockpile will be created and
// visible in DF with the right category tabs, but each material may need
// to be manually accepted in the DF UI the first time. A future plugin
// extension can call dfhack stockpiles::accept_all to fill sub-params.
type StockpileDesignation struct {
	X1, Y1, Z int16
	X2, Y2    int16
	GroupMask uint32 // bitmask of StockpileGroup* constants
}

// SmoothDesignation represents a smooth/engrave designation on a single
// Z-level rectangular region. SmoothType picks 1=smooth or 2=engrave.
//
// IMPORTANT: smooth/engrave only works on natural stone walls and floors —
// not on soil/sand/gravel/dirt and not on constructed walls. The plugin
// applies the designation regardless; DF's labor system will simply skip
// tiles that aren't valid targets. For dirt/soil aquifer layers, use a
// constructed wall (BuildTypeWall) instead.
type SmoothDesignation struct {
	X1, Y1, Z  int16
	X2, Y2     int16
	SmoothType uint8 // SmoothTypeSmooth or SmoothTypeEngrave
}

// PauseControl is the payload for CommandTypePause.
// Mode: 0x00 unpause, 0x01 pause, 0x02 step (unpause, auto-pause after Ticks).
type PauseControl struct {
	Mode  uint8
	Ticks uint32 // only meaningful for Mode=0x02
}

const (
	PauseModeUnpause uint8 = 0x00
	PauseModePause   uint8 = 0x01
	PauseModeStep    uint8 = 0x02
)

// CommandMessage represents a command from server to DFHack
type CommandMessage struct {
	CommandID    uint32                    // Unique command identifier
	CommandType  uint8                     // Type of command (dig/build/cancel/zone/blueprint/unsuspend/work_order)
	DigType      uint8                     // For DIG: dig designation type (Default=1, UpDownStair=2, Channel=3, etc)
	Region       Region                    // For DIG and CANCEL commands
	Build        BuildDesignation          // For BUILD commands
	Zone         ZoneDesignation           // For ZONE commands
	Unsuspend    UnsuspendDesignation      // For UNSUSPEND commands
	Order        WorkOrderDesignation      // For WORK_ORDER commands
	Stockpile    StockpileDesignation      // For STOCKPILE commands
	Smooth       SmoothDesignation         // For SMOOTH commands
	Pause        PauseControl              // For PAUSE commands
	Remove       RemoveBuildingDesignation // For REMOVE_BUILDING commands
	QueueJob     QueueJobDesignation       // For QUEUE_JOB commands
	SetLabor     SetLaborDesignation       // For SET_LABOR commands
	AssignZone   AssignZoneDesignation     // For ASSIGN_ZONE commands
	UnassignZone UnassignZoneDesignation   // For UNASSIGN_ZONE commands
	RemoveZone   RemoveZoneDesignation     // For REMOVE_ZONE commands
	FarmPlot     FarmPlotDesignation       // For BUILD_FARM_PLOT commands
	SetFarmCrop  SetFarmCropDesignation    // For SET_FARM_CROP commands

	DesignateBurrow DesignateBurrowDesignation // For DESIGNATE_BURROW commands
	RemoveBurrow    RemoveBurrowDesignation    // For REMOVE_BURROW commands
	AssignBurrow    AssignBurrowDesignation    // For ASSIGN_BURROW commands
	SetAlert        SetAlertDesignation        // For SET_ALERT commands

	LinkBuilding LinkBuildingDesignation // For LINK_BUILDING commands
	PullLever    PullLeverDesignation    // For PULL_LEVER commands
	BuildBridge  BuildBridgeDesignation  // For BUILD_BRIDGE commands

	SetWorkshopProfile SetWorkshopProfileDesignation // For SET_WORKSHOP_PROFILE commands

	AssignWorkDetail  AssignWorkDetailDesignation  // For ASSIGN_WORK_DETAIL commands
	SetWorkDetailMode SetWorkDetailModeDesignation // For SET_WORK_DETAIL_MODE commands
	CreateWorkDetail  CreateWorkDetailDesignation  // For CREATE_WORK_DETAIL commands

	BringGoodsToDepot BringGoodsToDepotDesignation // For BRING_GOODS_TO_DEPOT commands

	AppointPosition        AppointPositionDesignation        // For APPOINT_POSITION commands
	SetBookkeeperPrecision SetBookkeeperPrecisionDesignation // For SET_BOOKKEEPER_PRECISION commands

	CreateSquad CreateSquadDesignation // For CREATE_SQUAD commands
	AssignSquad AssignSquadDesignation // For ASSIGN_SQUAD commands
	SquadOrder  SquadOrderDesignation  // For SQUAD_ORDER commands

	CancelOrder CancelOrderDesignation // For CANCEL_ORDER commands
	EditOrder   EditOrderDesignation   // For EDIT_ORDER commands

	CreateLocation  CreateLocationDesignation  // For CREATE_LOCATION commands
	AssignLodging   AssignLodgingDesignation   // For ASSIGN_LODGING commands
	UnassignLodging UnassignLodgingDesignation // For UNASSIGN_LODGING commands

	// Feature 007: Blueprint command fields
	BlueprintName string // For BLUEPRINT: blueprint filename (without .csv)
	OriginX       int16  // For BLUEPRINT: placement X coordinate
	OriginY       int16  // For BLUEPRINT: placement Y coordinate
	OriginZ       int16  // For BLUEPRINT: placement Z coordinate
}

func (m *CommandMessage) Type() uint8 { return MessageTypeCommand }

func (m *CommandMessage) Validate() error {
	// Validate CommandType
	if m.CommandType < CommandTypeDig || m.CommandType > CommandTypeEditOrder {
		return fmt.Errorf("invalid command type: 0x%02X", m.CommandType)
	}

	// Type-specific validation
	switch m.CommandType {
	case CommandTypeDig, CommandTypeCancel:
		// Validate region bounds
		if m.Region.X2 < m.Region.X1 || m.Region.Y2 < m.Region.Y1 || m.Region.Z2 < m.Region.Z1 {
			return errors.New("invalid region: end coordinates must be >= start coordinates")
		}
		// Multi-Z is allowed: stair shafts span Z natively, and the plugin
		// handles 3D rectangles tile-by-tile (DF designations are per-tile;
		// there is no "shaft" concept in DF). Cancel also accepts multi-Z
		// for the symmetric case of clearing a stair shaft.
	case CommandTypeBuild:
		// Validate BuildType — accept any defined construction, workshop,
		// furniture, door, furnace, depot, or misc/infrastructure value, or
		// the generalized by-name sentinel. Plugin will reject specific values it doesn't
		// yet implement.
		if m.Build.BuildType != BuildTypeByName &&
			!IsBuildTypeConstruction(m.Build.BuildType) &&
			!IsBuildTypeWorkshop(m.Build.BuildType) &&
			!IsBuildTypeFurniture(m.Build.BuildType) &&
			!IsBuildTypeDoor(m.Build.BuildType) &&
			!IsBuildTypeFurnace(m.Build.BuildType) &&
			!IsBuildTypeDepot(m.Build.BuildType) &&
			!IsBuildTypeInfra(m.Build.BuildType) &&
			!IsBuildTypeWaterPower(m.Build.BuildType) &&
			!IsBuildTypeTrap(m.Build.BuildType) {
			return fmt.Errorf("invalid build type: 0x%02X", m.Build.BuildType)
		}
		if m.Build.BuildType == BuildTypeByName && m.Build.BuildTypeName == "" {
			return errors.New("build by-name path (BuildTypeByName) requires a non-empty BuildTypeName")
		}
		if m.Build.Material > MaterialClassBlocks {
			return fmt.Errorf("invalid material class: 0x%02X", m.Build.Material)
		}
		if m.Build.Quality != QualityTierAny && m.Build.Quality > QualityTierArtifact {
			return fmt.Errorf("invalid quality tier: 0x%02X", m.Build.Quality)
		}
		if m.Build.Orientation != BuildOrientAny && m.Build.Orientation > BuildOrientWest {
			return fmt.Errorf("invalid orientation: 0x%02X", m.Build.Orientation)
		}
	case CommandTypePause:
		if m.Pause.Mode > PauseModeStep {
			return fmt.Errorf("invalid pause mode: 0x%02X", m.Pause.Mode)
		}
		if m.Pause.Mode == PauseModeStep && m.Pause.Ticks == 0 {
			return errors.New("pause step requires Ticks > 0")
		}
	case CommandTypeSetLabor:
		if m.SetLabor.LaborID > LaborMaxIndex {
			return fmt.Errorf("invalid labor id: %d (max %d)", m.SetLabor.LaborID, LaborMaxIndex)
		}
	case CommandTypeQueueJob:
		if m.QueueJob.OrderType == OrderTypeByName && m.QueueJob.JobTypeName == "" {
			return errors.New("queue_job by-name path (OrderTypeByName) requires a non-empty JobTypeName")
		}
		if m.QueueJob.OrderType == OrderTypeCustomReaction && m.QueueJob.ReactionCode == "" {
			return errors.New("queue_job custom-reaction path (OrderTypeCustomReaction) requires a non-empty ReactionCode")
		}
	case CommandTypeWorkOrder:
		if m.Order.OrderType == OrderTypeByName && m.Order.JobTypeName == "" {
			return errors.New("order by-name path (OrderTypeByName) requires a non-empty JobTypeName")
		}
		if m.Order.Frequency > WorkOrderFrequencyYearly {
			return fmt.Errorf("invalid work order frequency: 0x%02X", m.Order.Frequency)
		}
	case CommandTypeBuildFarmPlot:
		if m.FarmPlot.X2 < m.FarmPlot.X1 || m.FarmPlot.Y2 < m.FarmPlot.Y1 {
			return errors.New("invalid region: end coordinates must be >= start coordinates")
		}
	case CommandTypeSetFarmCrop:
		if m.SetFarmCrop.Season > 3 && m.SetFarmCrop.Season != FarmSeasonAll {
			return fmt.Errorf("invalid season: 0x%02X (want 0-3 or FarmSeasonAll)", m.SetFarmCrop.Season)
		}
		if m.SetFarmCrop.CropName == "" {
			return errors.New("set_farm_crop requires a non-empty CropName (a plant raw token/display name, or \"fallow\")")
		}
	case CommandTypeDesignateBurrow:
		if m.DesignateBurrow.Name == "" {
			return errors.New("designate_burrow requires a non-empty Name")
		}
		if m.DesignateBurrow.X2 < m.DesignateBurrow.X1 || m.DesignateBurrow.Y2 < m.DesignateBurrow.Y1 || m.DesignateBurrow.Z2 < m.DesignateBurrow.Z1 {
			return errors.New("invalid region: end coordinates must be >= start coordinates")
		}
	case CommandTypeRemoveBurrow:
		if m.RemoveBurrow.Name == "" {
			return errors.New("remove_burrow requires a non-empty Name")
		}
	case CommandTypeAssignBurrow:
		if m.AssignBurrow.Name == "" {
			return errors.New("assign_burrow requires a non-empty Name")
		}
	case CommandTypeSetAlert:
		if m.SetAlert.Name == "" {
			return errors.New("set_alert requires a non-empty Name")
		}
	case CommandTypeBuildBridge:
		if m.BuildBridge.X2 < m.BuildBridge.X1 || m.BuildBridge.Y2 < m.BuildBridge.Y1 {
			return errors.New("invalid region: end coordinates must be >= start coordinates")
		}
		if m.BuildBridge.Direction > BridgeDirectionRaiseW {
			return fmt.Errorf("invalid bridge direction: 0x%02X", m.BuildBridge.Direction)
		}
	case CommandTypeSetWorkshopProfile:
		if m.SetWorkshopProfile.MinSkillLevel < 0 || m.SetWorkshopProfile.MinSkillLevel > 20 {
			return fmt.Errorf("invalid min_skill_level: %d (want 0-20)", m.SetWorkshopProfile.MinSkillLevel)
		}
		if m.SetWorkshopProfile.MaxSkillLevel != -1 && (m.SetWorkshopProfile.MaxSkillLevel < 0 || m.SetWorkshopProfile.MaxSkillLevel > 20) {
			return fmt.Errorf("invalid max_skill_level: %d (want 0-20, or -1 for uncapped)", m.SetWorkshopProfile.MaxSkillLevel)
		}
		if m.SetWorkshopProfile.MaxSkillLevel != -1 && m.SetWorkshopProfile.MaxSkillLevel < m.SetWorkshopProfile.MinSkillLevel {
			return errors.New("set_workshop_profile: max_skill_level must be >= min_skill_level")
		}
	case CommandTypeSetWorkDetailMode:
		if m.SetWorkDetailMode.Mode > WorkDetailModeOnlySelectedDoesThis {
			return fmt.Errorf("invalid work detail mode: 0x%02X", m.SetWorkDetailMode.Mode)
		}
	case CommandTypeCreateWorkDetail:
		if m.CreateWorkDetail.Name == "" {
			return errors.New("create_work_detail requires a non-empty Name")
		}
		if m.CreateWorkDetail.Mode > WorkDetailModeOnlySelectedDoesThis {
			return fmt.Errorf("invalid work detail mode: 0x%02X", m.CreateWorkDetail.Mode)
		}
	case CommandTypeBringGoodsToDepot:
		if m.BringGoodsToDepot.MaxCount < 1 {
			return errors.New("bring_goods_to_depot requires MaxCount >= 1")
		}
	case CommandTypeAppointPosition:
		if m.AppointPosition.PositionCode == "" {
			return errors.New("appoint_position requires a non-empty PositionCode")
		}
	case CommandTypeSetBookkeeperPrecision:
		if m.SetBookkeeperPrecision.Precision > BookkeeperPrecisionAllAccurate {
			return fmt.Errorf("invalid bookkeeper precision: 0x%02X", m.SetBookkeeperPrecision.Precision)
		}
	case CommandTypeSquadOrder:
		if m.SquadOrder.Type > SquadOrderCancel {
			return fmt.Errorf("invalid squad order type: 0x%02X", m.SquadOrder.Type)
		}
		if m.SquadOrder.Type == SquadOrderDefendBurrow && m.SquadOrder.BurrowName == "" {
			return errors.New("squad_order defend_burrow requires a non-empty BurrowName")
		}
	case CommandTypeCancelOrder:
		if m.CancelOrder.OrderID < 0 {
			return errors.New("cancel_order requires OrderID >= 0")
		}
	case CommandTypeEditOrder:
		if m.EditOrder.OrderID < 0 {
			return errors.New("edit_order requires OrderID >= 0")
		}
		if !m.EditOrder.HasAmount && !m.EditOrder.HasFrequency {
			return errors.New("edit_order requires at least one of Amount or Frequency to change")
		}
		if m.EditOrder.HasAmount && (m.EditOrder.Amount == 0 || m.EditOrder.Amount > 100) {
			return fmt.Errorf("edit_order amount out of range (1-100): %d", m.EditOrder.Amount)
		}
		if m.EditOrder.HasFrequency && m.EditOrder.Frequency > WorkOrderFrequencyYearly {
			return fmt.Errorf("invalid work order frequency: 0x%02X", m.EditOrder.Frequency)
		}
	}

	return nil
}

// CommandAckMessage represents acknowledgment from DFHack to server
type CommandAckMessage struct {
	CommandID uint32 // Original command ID
	Status    uint8  // Execution status (success/partial/failure)
	ErrorMsg  string // Error description (empty if success)
}

func (m *CommandAckMessage) Type() uint8 { return MessageTypeCommandAck }

func (m *CommandAckMessage) Validate() error {
	// Validate Status
	if m.Status > AckStatusFailure {
		return fmt.Errorf("invalid ack status: 0x%02X", m.Status)
	}
	// ErrorMsg length is validated during serialization
	return nil
}

// AnnouncementInfo is one DF announcement / cancellation in the protocol
// wire format. Mirrors df::report's relevant fields.
//
// Wire layout (per entry, big-endian):
//
//	[4: ID]
//	[2: TypeID]
//	[1: Severity]      // 0=info, 1=warn, 2=critical
//	[2: X][2: Y][2: Z] // (-1,-1,-1) if non-positional
//	[4: GameYear]
//	[4: GameTick]
//	[2: TextLen]
//	[N: Text UTF-8]
//
// RepeatCount is NOT part of the per-entry layout above — it travels in a
// trailing block after all entries (see AnnouncementUpdateMessage). It
// mirrors df::report.repeat_count: DF 50+ pools a repeated identical
// announcement (e.g. "Digging designation cancelled: damp stone located."
// firing over and over) by bumping repeat_count IN PLACE on the same report
// object rather than allocating a new one, so an id-only cursor can never
// see the repeat happen — the plugin re-sends the entry (same ID) whenever
// repeat_count grows, and this field is how the caller tells "the same
// problem, again" from "a brand-new problem".
type AnnouncementInfo struct {
	ID          uint32
	TypeID      uint16
	Severity    uint8
	X, Y, Z     int16
	GameYear    uint32
	GameTick    uint32
	Text        string
	RepeatCount uint32
}

// AnnouncementUpdateMessage carries a batch of new (or repeat-count-bumped)
// DF announcements from the plugin. The plugin sends only NEW/changed
// entries (delta against the highest previously-sent ID, plus any
// already-sent entry whose repeat_count has grown), so the server's
// AlertStore can dedupe/merge by ID without storing its own cursor.
//
// Wire layout: [4: Count] [N × AnnouncementInfo per-entry fields above]
// [trailing block]. The trailing block is EITHER absent (an old plugin that
// predates RepeatCount) OR exactly N × [4: RepeatCount uint32], one per
// entry in the SAME order as the entries above — deliberately placed after
// all N entries, not interleaved into each entry's own fields, so decoding
// can tell old format from new format unambiguously: after consuming
// exactly Count entries, 0 bytes left over means old format (every
// RepeatCount defaults to its zero value), exactly 4*Count bytes left over
// means new format. See codec.go's deserializeAnnouncementUpdate for the
// decode side of this backward-compat rule.
type AnnouncementUpdateMessage struct {
	Count         uint32
	Announcements []AnnouncementInfo
}

func (m *AnnouncementUpdateMessage) Type() uint8 { return MessageTypeAnnouncementUpdate }

func (m *AnnouncementUpdateMessage) Validate() error {
	if uint32(len(m.Announcements)) != m.Count {
		return fmt.Errorf("announcement count mismatch: Count=%d, len=%d", m.Count, len(m.Announcements))
	}
	for i, a := range m.Announcements {
		if len(a.Text) > 4096 {
			return fmt.Errorf("announcement %d text too long (%d > 4096)", i, len(a.Text))
		}
	}
	return nil
}

// QueryMessage is a server → plugin request for structured data. Distinct
// from commands: queries don't mutate game state, they read. The plugin
// dispatches by Name, runs the handler on the main thread, and returns a
// QueryResponseMessage with the same QueryID.
type QueryMessage struct {
	QueryID uint32
	Name    string // tool name (e.g. "list_orders", "dwarf_detail")
	Args    string // JSON-encoded args; handler decodes
}

func (m *QueryMessage) Type() uint8 { return MessageTypeQuery }

func (m *QueryMessage) Validate() error {
	if m.QueryID == 0 {
		return errors.New("queryID must be non-zero")
	}
	if len(m.Name) == 0 || len(m.Name) > 64 {
		return fmt.Errorf("query name length out of range (1..64): %d", len(m.Name))
	}
	if len(m.Args) > 4096 {
		return fmt.Errorf("query args too long (max 4096): %d", len(m.Args))
	}
	return nil
}

// QueryResponseStatus constants.
const (
	QueryStatusSuccess uint8 = 0x00
	QueryStatusError   uint8 = 0x01
	QueryStatusUnknown uint8 = 0x02 // unknown query name
)

// QueryResponseMessage carries the plugin's structured answer to a query.
type QueryResponseMessage struct {
	QueryID uint32
	Status  uint8
	Data    string // JSON; success = result payload, error = {"error":"..."}
}

func (m *QueryResponseMessage) Type() uint8 { return MessageTypeQueryResponse }

func (m *QueryResponseMessage) Validate() error {
	if m.QueryID == 0 {
		return errors.New("queryID must be non-zero")
	}
	if m.Status > QueryStatusUnknown {
		return fmt.Errorf("invalid query response status: 0x%02X", m.Status)
	}
	return nil
}
