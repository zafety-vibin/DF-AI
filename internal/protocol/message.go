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
	CommandTypeDig            uint8 = 0x01
	CommandTypeBuild          uint8 = 0x02
	CommandTypeCancel         uint8 = 0x03
	CommandTypeChop           uint8 = 0x04
	CommandTypeGather         uint8 = 0x05
	CommandTypeZone           uint8 = 0x06 // Designate zones (bedroom, dining, etc.)
	CommandTypeBlueprint      uint8 = 0x07 // Feature 007: Apply blueprint pattern with zones
	CommandTypeUnsuspend      uint8 = 0x08 // Clear suspend flag on a building's jobs
	CommandTypeWorkOrder      uint8 = 0x09 // Add a manager work order
	CommandTypeStockpile      uint8 = 0x0A // Designate a stockpile zone with category flags
	CommandTypeSmooth         uint8 = 0x0B // Designate stone tiles for smoothing or engraving
	CommandTypePause          uint8 = 0x0C // pause/unpause/step simulation control
	CommandTypeRemoveBuilding uint8 = 0x0D // Mark the building at a tile for deconstruction
	CommandTypeQueueJob       uint8 = 0x0E // Queue a job directly at an existing workshop (no manager/office needed)
	CommandTypeSetLabor       uint8 = 0x0F // Enable/disable one labor on a unit
	CommandTypeAssignZone     uint8 = 0x10 // Assign a unit to the zone at a tile (owner or roster, depending on zone type)
	CommandTypeUnassignZone   uint8 = 0x11 // Remove a unit's zone assignment
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

// MaterialClass constants for the BUILD command's trailing material_class
// byte. Constrains which item CLASS DF's job system may claim for the
// build; DF still picks the specific item within that class. The plugin
// treats a BUILD payload without the byte as MaterialClassAny (backward
// compatible); new Go always appends it.
const (
	MaterialClassAny    uint8 = 0x00 // no constraint (DF picks anything suitable)
	MaterialClassWood   uint8 = 0x01 // logs (job_item item_type=WOOD)
	MaterialClassStone  uint8 = 0x02 // boulders (item_type=BOULDER)
	MaterialClassBlocks uint8 = 0x03 // blocks (item_type=BLOCKS)
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

	// OrderTypeByName is not one of the hand-maintained order types above —
	// it's the sentinel for QueueJobDesignation's generalized name-based
	// path (see that type's doc comment). Only CommandTypeQueueJob
	// (SendQueueJob) understands it; CommandTypeWorkOrder
	// (SendWorkOrderCommand) does not and still only accepts the bytes
	// above. Matches dfhack-plugin/protocol.h ORDER_TYPE_BY_NAME.
	OrderTypeByName uint8 = 0x00
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
//	              over a multi-tile footprint.
//	0x30 – 0x4F : Furniture — single-tile placed buildings using one item
//	              from a stockpile.
//	0x50 – 0x6F : Doors / Hatches — single-tile portal buildings.
//	0x70 – 0x7F : Reserved for stockpiles and zones (future).
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

	// Furniture (single-tile, requires an item from stockpile).
	BuildTypeBed     uint8 = 0x30
	BuildTypeTable   uint8 = 0x31
	BuildTypeChair   uint8 = 0x32
	BuildTypeCabinet uint8 = 0x33
	BuildTypeCoffer  uint8 = 0x34

	// Doors / hatches.
	BuildTypeDoor  uint8 = 0x50
	BuildTypeHatch uint8 = 0x51
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

// Region represents a 3D bounding box for designations
type Region struct {
	X1, Y1, Z1 int16 // Start coordinates
	X2, Y2, Z2 int16 // End coordinates (inclusive)
}

// BuildDesignation represents a single build command
type BuildDesignation struct {
	X, Y, Z   int16 // Build location
	BuildType uint8 // Type of construction
	Material  uint8 // MaterialClass* constraint (MaterialClassAny = no preference)
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
// bay12 token) and is ignored unless OrderType == OrderTypeByName. Only
// QueueJobDesignation supports this; WorkOrderDesignation does not.
type QueueJobDesignation struct {
	X, Y, Z     int16
	OrderType   uint8  // What to produce (OrderTypeMakeBed etc.), or OrderTypeByName
	JobTypeName string // DFHack job_type enum key name; only used when OrderType == OrderTypeByName
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

// WorkOrderDesignation represents a manager work order: produce N items of
// the given type. The manager dispatches to whichever workshop can fulfill
// the order, drawing reagents from stockpiles automatically.
type WorkOrderDesignation struct {
	OrderType uint8  // What to produce (OrderTypeMakeBed etc.)
	Quantity  uint16 // How many to produce (1..100)
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

	// Feature 007: Blueprint command fields
	BlueprintName string // For BLUEPRINT: blueprint filename (without .csv)
	OriginX       int16  // For BLUEPRINT: placement X coordinate
	OriginY       int16  // For BLUEPRINT: placement Y coordinate
	OriginZ       int16  // For BLUEPRINT: placement Z coordinate
}

func (m *CommandMessage) Type() uint8 { return MessageTypeCommand }

func (m *CommandMessage) Validate() error {
	// Validate CommandType
	if m.CommandType < CommandTypeDig || m.CommandType > CommandTypeUnassignZone {
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
		// furniture, or door value. Plugin will reject specific values it
		// doesn't yet implement.
		if !IsBuildTypeConstruction(m.Build.BuildType) &&
			!IsBuildTypeWorkshop(m.Build.BuildType) &&
			!IsBuildTypeFurniture(m.Build.BuildType) &&
			!IsBuildTypeDoor(m.Build.BuildType) {
			return fmt.Errorf("invalid build type: 0x%02X", m.Build.BuildType)
		}
		if m.Build.Material > MaterialClassBlocks {
			return fmt.Errorf("invalid material class: 0x%02X", m.Build.Material)
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
type AnnouncementInfo struct {
	ID       uint32
	TypeID   uint16
	Severity uint8
	X, Y, Z  int16
	GameYear uint32
	GameTick uint32
	Text     string
}

// AnnouncementUpdateMessage carries a batch of new DF announcements from
// the plugin. The plugin sends only NEW entries (delta against the highest
// previously-sent ID), so the server's AlertStore can dedupe by ID without
// storing a "last seen" cursor itself.
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
