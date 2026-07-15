#pragma once

#include <cstdint>

// Protocol version
constexpr uint8_t PROTOCOL_VERSION = 1;

// Message type constants
constexpr uint8_t MSG_TYPE_HANDSHAKE = 0x01;
constexpr uint8_t MSG_TYPE_FULL_STATE = 0x02;
constexpr uint8_t MSG_TYPE_TILE_UPDATE = 0x03;
constexpr uint8_t MSG_TYPE_RESYNC_REQUEST = 0x04;
constexpr uint8_t MSG_TYPE_HEARTBEAT = 0x05;
constexpr uint8_t MSG_TYPE_ERROR = 0x06;
constexpr uint8_t MSG_TYPE_DISCONNECT = 0x07;
constexpr uint8_t MSG_TYPE_ENTITY_UPDATE = 0x08;
constexpr uint8_t MSG_TYPE_COMMAND = 0x09;
constexpr uint8_t MSG_TYPE_COMMAND_ACK = 0x0A;
constexpr uint8_t MSG_TYPE_QUERY = 0x0B;
constexpr uint8_t MSG_TYPE_QUERY_RESPONSE = 0x0C;
constexpr uint8_t MSG_TYPE_ANNOUNCEMENT_UPDATE = 0x0D;

// Query response status codes
constexpr uint8_t QUERY_STATUS_SUCCESS = 0x00;
constexpr uint8_t QUERY_STATUS_ERROR   = 0x01;
constexpr uint8_t QUERY_STATUS_UNKNOWN = 0x02;

// Tile flags — kept in sync with internal/protocol/message.go (Flag*).
constexpr uint8_t FLAG_HIDDEN = 0x01;
constexpr uint8_t FLAG_DISCOVERED = 0x02;
constexpr uint8_t FLAG_DESIGNATED = 0x04;
constexpr uint8_t FLAG_CONSTRUCT = 0x08;
constexpr uint8_t FLAG_WALL = 0x10;        // Solid wall/rock (diggable)
constexpr uint8_t FLAG_FLOOR = 0x20;       // Walkable floor/ramp/stair
constexpr uint8_t FLAG_VOID = 0x40;        // Open air / missing floor (fall hazard)
constexpr uint8_t FLAG_LIQUID_7_7 = 0x80;  // Water/magma at 7/7 depth

// Error codes
constexpr uint16_t ERR_UNKNOWN_TYPE = 0x0001;
constexpr uint16_t ERR_INVALID_PAYLOAD = 0x0002;
constexpr uint16_t ERR_VERSION_MISMATCH = 0x0003;
constexpr uint16_t ERR_INTERNAL = 0x0004;

// Disconnect reasons
constexpr uint8_t REASON_NORMAL_SHUTDOWN = 0x00;
constexpr uint8_t REASON_PLUGIN_UNLOAD = 0x01;
constexpr uint8_t REASON_SERVER_SHUTDOWN = 0x02;
constexpr uint8_t REASON_RESTART = 0x03;

// Resync reasons
constexpr uint8_t RESYNC_MANUAL = 0x00;
constexpr uint8_t RESYNC_INCONSISTENCY = 0x01;
constexpr uint8_t RESYNC_RECONNECT = 0x02;
constexpr uint8_t RESYNC_PERIODIC = 0x03;

// Heartbeat timing (milliseconds)
constexpr uint32_t HEARTBEAT_INTERVAL_MS = 10000;
constexpr uint32_t HEARTBEAT_TIMEOUT_MS = 15000;

// Command types
constexpr uint8_t COMMAND_TYPE_DIG        = 0x01;
constexpr uint8_t COMMAND_TYPE_BUILD      = 0x02;
constexpr uint8_t COMMAND_TYPE_CANCEL     = 0x03;
constexpr uint8_t COMMAND_TYPE_CHOP       = 0x04;
constexpr uint8_t COMMAND_TYPE_GATHER     = 0x05;
constexpr uint8_t COMMAND_TYPE_ZONE       = 0x06;
constexpr uint8_t COMMAND_TYPE_BLUEPRINT  = 0x07;
constexpr uint8_t COMMAND_TYPE_UNSUSPEND  = 0x08;
constexpr uint8_t COMMAND_TYPE_WORK_ORDER = 0x09;
constexpr uint8_t COMMAND_TYPE_STOCKPILE  = 0x0A;
constexpr uint8_t COMMAND_TYPE_SMOOTH     = 0x0B;
constexpr uint8_t COMMAND_TYPE_PAUSE      = 0x0C;
constexpr uint8_t COMMAND_TYPE_REMOVE_BUILDING = 0x0D;
constexpr uint8_t COMMAND_TYPE_QUEUE_JOB  = 0x0E;
constexpr uint8_t COMMAND_TYPE_SET_LABOR  = 0x0F;
constexpr uint8_t COMMAND_TYPE_ASSIGN_ZONE   = 0x10;
constexpr uint8_t COMMAND_TYPE_UNASSIGN_ZONE = 0x11;
constexpr uint8_t COMMAND_TYPE_CREATE_LOCATION   = 0x12;
constexpr uint8_t COMMAND_TYPE_ASSIGN_LODGING    = 0x13;
constexpr uint8_t COMMAND_TYPE_UNASSIGN_LODGING  = 0x14;

// Location types -- DF-AI's own wire values for df::abstract_building_type's
// INN_TAVERN/TEMPLE/LIBRARY/GUILDHALL. A Location is created FROM an
// existing MeetingHall civzone (see designate_zone), not designated
// directly -- these values only ever appear as create_location's `type`
// param. Guildhall additionally requires a profession name (see
// applyCreateLocation) -- confirmed via DFHack's own quickfort reference
// (scripts/internal/quickfort/zone.lua's set_location(), which refuses
// to create a guildhall without one); the other three need no extra input.
constexpr uint8_t LOCATION_TYPE_TAVERN    = 0x01;
constexpr uint8_t LOCATION_TYPE_TEMPLE    = 0x02;
constexpr uint8_t LOCATION_TYPE_LIBRARY   = 0x03;
constexpr uint8_t LOCATION_TYPE_GUILDHALL = 0x04;

// Pause modes (payload byte after cmdType): matches internal/protocol/message.go.
constexpr uint8_t PAUSE_MODE_UNPAUSE = 0x00;
constexpr uint8_t PAUSE_MODE_PAUSE   = 0x01;
constexpr uint8_t PAUSE_MODE_STEP    = 0x02;

// Smooth subtype: matches df::tile_designation::smooth bitfield (1=smooth, 2=engrave).
constexpr uint8_t SMOOTH_TYPE_SMOOTH  = 0x01;
constexpr uint8_t SMOOTH_TYPE_ENGRAVE = 0x02;

// Zone types — kept in sync with internal/protocol/message.go. These are
// DF-AI's OWN wire values, not DFHack's df::civzone_type values directly
// (those are scattered 79-97, not a small sequential range — see
// zones.cpp's civzoneTypeFromWire/wireFromCivzoneType for the real
// translation table, confirmed against library/include/df/civzone_type.h
// in the DFHack 53.15-r1 checkout). This insulates the wire format from
// DFHack renumbering and keeps command payloads compact.
//
// Assignment support (see zones.cpp::applyAssignZone): Owner-type
// (Bedroom/Office/Tomb/DiningHall) and Roster-type (Pen/Pond) are the
// only 6 with a confirmed DFHack assignment mechanism. Barracks uses a
// separate squad-based mechanism, out of scope for assign_zone. The rest
// have no confirmed mechanism in the DFHack source at all — assign_zone
// returns an explicit "not implemented" error for them, never a guess.
constexpr uint8_t ZONE_TYPE_BEDROOM         = 0x01; // Owner
constexpr uint8_t ZONE_TYPE_OFFICE          = 0x02; // Owner
constexpr uint8_t ZONE_TYPE_TOMB            = 0x03; // Owner
constexpr uint8_t ZONE_TYPE_DINING_HALL     = 0x04; // Owner
constexpr uint8_t ZONE_TYPE_MEETING_HALL    = 0x05; // Unconfirmed
constexpr uint8_t ZONE_TYPE_DORMITORY       = 0x06; // Unconfirmed
constexpr uint8_t ZONE_TYPE_BARRACKS        = 0x07; // Squad (out of scope)
constexpr uint8_t ZONE_TYPE_PEN             = 0x08; // Roster
constexpr uint8_t ZONE_TYPE_POND            = 0x09; // Roster
constexpr uint8_t ZONE_TYPE_ARCHERY_RANGE   = 0x0A; // Unconfirmed
constexpr uint8_t ZONE_TYPE_PLANT_GATHERING = 0x0B; // Unconfirmed
constexpr uint8_t ZONE_TYPE_WATER_SOURCE    = 0x0C; // Unconfirmed
constexpr uint8_t ZONE_TYPE_DUMP            = 0x0D; // Unconfirmed
constexpr uint8_t ZONE_TYPE_SAND_COLLECTION = 0x0E; // Unconfirmed
constexpr uint8_t ZONE_TYPE_FISHING_AREA    = 0x0F; // Unconfirmed
constexpr uint8_t ZONE_TYPE_CLAY_COLLECTION = 0x10; // Unconfirmed
constexpr uint8_t ZONE_TYPE_DUNGEON         = 0x11; // Unconfirmed
constexpr uint8_t ZONE_TYPE_ANIMAL_TRAINING = 0x12; // Unconfirmed

// Labor IDs for the SET_LABOR command. The value IS the real
// df::unit_labor enum index (library/include/df/unit_labor.h in the
// DFHack 53.15-r1 checkout, base-type int32_t, valid range 0-93) —
// transmitted directly as the wire byte so the plugin can index
// status.labors[] with no translation table. This is a curated subset
// relevant to a fresh 7-dwarf fort, not the full 94-entry DF list; the
// plugin bounds-checks any byte against LABOR_MAX_INDEX regardless of
// whether it appears in this subset. Kept in sync with
// internal/protocol/message.go (Labor* constants).
constexpr uint8_t LABOR_MINE            = 0;
constexpr uint8_t LABOR_HAUL_STONE      = 1;
constexpr uint8_t LABOR_HAUL_WOOD       = 2;
constexpr uint8_t LABOR_HAUL_FOOD       = 4;
constexpr uint8_t LABOR_HAUL_ITEM       = 6;
constexpr uint8_t LABOR_HAUL_FURNITURE  = 7;
constexpr uint8_t LABOR_CUTWOOD         = 10;
constexpr uint8_t LABOR_CARPENTER       = 11;
constexpr uint8_t LABOR_STONECUTTER     = 12;
constexpr uint8_t LABOR_STONE_CARVER    = 13;
constexpr uint8_t LABOR_ENGRAVER        = 14; // caption "Stone Engraving" (df.d_basics.xml:11215-11218) — not DETAIL
constexpr uint8_t LABOR_MASON           = 15;
constexpr uint8_t LABOR_BREWER          = 30;
constexpr uint8_t LABOR_COOK            = 38;
constexpr uint8_t LABOR_PLANT           = 39;
constexpr uint8_t LABOR_HERBALIST       = 40;
constexpr uint8_t LABOR_FISH            = 41;
constexpr uint8_t LABOR_SMELT           = 45;
constexpr uint8_t LABOR_FORGE_WEAPON    = 46;
constexpr uint8_t LABOR_FORGE_ARMOR     = 47;
constexpr uint8_t LABOR_FORGE_FURNITURE = 48;
constexpr uint8_t LABOR_METAL_CRAFT     = 49;
constexpr uint8_t LABOR_MECHANIC        = 60;

// Highest valid df::unit_labor array index (unit_labor.h:122,
// last_item_value=93; status.labors is a fixed C array of that size+1,
// df/unit.h:374-386). Bounds-check any wire LaborID against this before
// indexing status.labors[] — mirrors the defensive guard at
// autolabor/labormanager.cpp:662. NONE(-1) is excluded by the uint8_t
// wire type itself (it cannot represent a negative value).
constexpr uint8_t LABOR_MAX_INDEX = 93;

// Work order types — what the manager queue should produce
constexpr uint8_t ORDER_TYPE_MAKE_BED      = 0x01;
constexpr uint8_t ORDER_TYPE_MAKE_TABLE    = 0x02;
constexpr uint8_t ORDER_TYPE_MAKE_CHAIR    = 0x03;
constexpr uint8_t ORDER_TYPE_MAKE_DOOR     = 0x04;
constexpr uint8_t ORDER_TYPE_MAKE_BARREL   = 0x05;
constexpr uint8_t ORDER_TYPE_MAKE_BUCKET   = 0x06;
constexpr uint8_t ORDER_TYPE_MAKE_CABINET  = 0x07;
constexpr uint8_t ORDER_TYPE_MAKE_COFFER   = 0x08;
constexpr uint8_t ORDER_TYPE_BREW_DRINK    = 0x09;
constexpr uint8_t ORDER_TYPE_PREPARE_MEAL  = 0x0A;
constexpr uint8_t ORDER_TYPE_MAKE_BLOCKS   = 0x0B;
constexpr uint8_t ORDER_TYPE_MAKE_CRAFTS   = 0x0C;

// Sentinel OrderType for COMMAND_TYPE_QUEUE_JOB's generalized name-based
// path (0x00 was never assigned to one of the hand-maintained order types
// above). When a QUEUE_JOB payload's OrderType byte equals this, a
// trailing [2:NameLen][N:Name] follows the OrderType byte — the SAME
// length-prefixed-string tail pattern COMMAND_TYPE_BLUEPRINT already uses
// for its blueprint filename (df_ai_protocol.cpp case 0x07). The name is
// looked up against DFHack's df::job_type key_table via find_enum_item
// (work_orders.cpp: resolveJobTypeByName) instead of the protocolToJobType
// switch in work_orders.cpp — this is how a caller reaches a job_type that
// has no ORDER_TYPE_* byte above (e.g. "ConstructHatchCover"). Existing
// callers that only ever send bytes 0x01-0x0C are completely unaffected:
// the plugin only looks for a trailing name when it sees this exact byte.
//
// NOTE: COMMAND_TYPE_WORK_ORDER (the manager-queue path, applyWorkOrder)
// does NOT support this sentinel — only QUEUE_JOB (the direct-to-workshop
// path, applyQueueJob) does. See work_orders.cpp for the scope boundary
// (job-type resolution is generalized; workshop-compatibility and
// material-class filtering are not, this pass).
constexpr uint8_t ORDER_TYPE_BY_NAME       = 0x00;

// Sentinel OrderType for COMMAND_TYPE_QUEUE_JOB's reaction-based path:
// reach a raw-defined df::reaction directly by its reaction CODE (e.g.
// "BREW_DRINK_FROM_PLANT" — df::reaction.code, NOT the display name)
// instead of by df::job_type. This is how queue_job reaches reactions that
// have no job_type mapping at all -- BrewDrink is the motivating case
// (protocolToJobType(ORDER_TYPE_BREW_DRINK) returns -1 in work_orders.cpp).
// Shares ORDER_TYPE_BY_NAME's trailing [2:NameLen][N:Name] wire shape (see
// that constant's comment above) — when a QUEUE_JOB payload's OrderType
// byte equals THIS constant, the trailing name is a reaction code, resolved
// via a linear scan of df::global::world->raws.reactions.reactions
// (work_orders.cpp: applyQueueReactionJob) instead of find_enum_item.
// Workshop/reaction compatibility is derived from the reaction's own
// building.type/subtype/custom parallel arrays (df/reaction.h) — no
// hand-maintained workshop table needed for this path, unlike
// jobTypeAllowedAtWorkshop. Matches internal/protocol/message.go
// OrderTypeCustomReaction. Discover valid codes via the list_reactions
// query/tool.
constexpr uint8_t ORDER_TYPE_CUSTOM_REACTION = 0x0D;

// BuildType constants — kept in sync with internal/protocol/message.go.
// Ranges: 0x01-0x0F constructions, 0x10-0x2F workshops, 0x30-0x4F furniture,
// 0x50-0x6F doors/hatches.
constexpr uint8_t BUILD_TYPE_WALL          = 0x01;
constexpr uint8_t BUILD_TYPE_FLOOR         = 0x02;
constexpr uint8_t BUILD_TYPE_UP_STAIR      = 0x03;
constexpr uint8_t BUILD_TYPE_DOWN_STAIR    = 0x04;
constexpr uint8_t BUILD_TYPE_UPDOWN_STAIR  = 0x05;
constexpr uint8_t BUILD_TYPE_RAMP          = 0x06;

constexpr uint8_t BUILD_TYPE_WS_CARPENTER  = 0x10;
constexpr uint8_t BUILD_TYPE_WS_MASON      = 0x11;
constexpr uint8_t BUILD_TYPE_WS_STILL      = 0x12;
constexpr uint8_t BUILD_TYPE_WS_FARMER     = 0x13;
constexpr uint8_t BUILD_TYPE_WS_CRAFTSDWARF= 0x14;
constexpr uint8_t BUILD_TYPE_WS_MECHANIC   = 0x15;
constexpr uint8_t BUILD_TYPE_WS_BUTCHER    = 0x16;
constexpr uint8_t BUILD_TYPE_WS_KITCHEN    = 0x17;
constexpr uint8_t BUILD_TYPE_WS_FISHERY    = 0x18;

constexpr uint8_t BUILD_TYPE_BED      = 0x30;
constexpr uint8_t BUILD_TYPE_TABLE    = 0x31;
constexpr uint8_t BUILD_TYPE_CHAIR    = 0x32;
constexpr uint8_t BUILD_TYPE_CABINET  = 0x33;
constexpr uint8_t BUILD_TYPE_COFFER   = 0x34;

constexpr uint8_t BUILD_TYPE_DOOR     = 0x50;
constexpr uint8_t BUILD_TYPE_HATCH    = 0x51;

inline bool isBuildTypeWorkshop(uint8_t t)     { return t >= 0x10 && t < 0x30; }
inline bool isBuildTypeFurniture(uint8_t t)    { return t >= 0x30 && t < 0x50; }
inline bool isBuildTypeConstruction(uint8_t t) { return t >= 0x01 && t < 0x10; }
inline bool isBuildTypeDoor(uint8_t t)         { return t >= 0x50 && t < 0x70; }

// ACK status codes
constexpr uint8_t ACK_STATUS_SUCCESS = 0x00;
constexpr uint8_t ACK_STATUS_PARTIAL = 0x01;
constexpr uint8_t ACK_STATUS_FAILURE = 0x02;

// Shared helper functions (implemented in tile_extractor.cpp)
#include <vector>
void write_uint16_be(std::vector<uint8_t> &buf, uint16_t value);
void write_int16_be(std::vector<uint8_t> &buf, int16_t value);
void write_uint32_be(std::vector<uint8_t> &buf, uint32_t value);
void write_uint64_be(std::vector<uint8_t> &buf, uint64_t value);

// Big-endian payload reader (implemented in designations.cpp).
uint32_t read_uint32_be(const std::vector<uint8_t> &data, size_t offset);

// Thread-safe socket send (implemented in df_ai_protocol.cpp). Serializes
// all frame writes to the shared socket under one mutex — the socket
// thread (heartbeat echo, auto-update) and the main thread (acks, query
// responses, announcements) both send frames, and interleaved partial
// writes would corrupt the stream. Returns bytes sent, or -1 if the
// socket is gone. ALL g_socket->Send calls must go through this.
int32_t socket_send_locked(const uint8_t *data, size_t len);
