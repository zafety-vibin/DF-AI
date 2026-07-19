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
// MSG_TYPE_ANNOUNCEMENT_UPDATE payload: [4:Count][N x entry][trailing block].
// The trailing block is EITHER absent (old format) OR exactly N x [4:
// RepeatCount uint32], one per entry in the same order -- never interleaved
// into the per-entry fields, so a decoder can tell the two formats apart
// unambiguously after parsing exactly Count entries: 0 bytes left over means
// old format (no repeat_count support), exactly 4*Count bytes left over
// means new format. See announcements.cpp::send_announcement_update for the
// encode side and internal/protocol/message.go's AnnouncementInfo /
// codec.go's deserializeAnnouncementUpdate for the decode side (including
// the backward-compat default-to-0 rule for old-format messages).
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
constexpr uint8_t COMMAND_TYPE_REMOVE_ZONE       = 0x15;
constexpr uint8_t COMMAND_TYPE_BUILD_FARM_PLOT   = 0x16;
constexpr uint8_t COMMAND_TYPE_SET_FARM_CROP     = 0x17;
constexpr uint8_t COMMAND_TYPE_DESIGNATE_BURROW  = 0x18;
constexpr uint8_t COMMAND_TYPE_REMOVE_BURROW     = 0x19;
constexpr uint8_t COMMAND_TYPE_ASSIGN_BURROW     = 0x1A;
constexpr uint8_t COMMAND_TYPE_SET_ALERT         = 0x1B;
constexpr uint8_t COMMAND_TYPE_LINK_BUILDING     = 0x1C;
constexpr uint8_t COMMAND_TYPE_PULL_LEVER        = 0x1D;
constexpr uint8_t COMMAND_TYPE_BUILD_BRIDGE      = 0x1E;
constexpr uint8_t COMMAND_TYPE_SET_WORKSHOP_PROFILE = 0x1F;

// Work Detail (labor-group) commands -- DF's own work_detail mechanism
// (df.plotinfo.xml: labor_infost.work_details, since v0.50.01). This is
// the AUTHORITATIVE store behind a unit's derived status.labors cache --
// see df_ai_protocol.cpp's applySetLabor doc comment for that
// cache/authoritative-source relationship. ASSIGN_WORK_DETAIL edits one
// detail's membership (assigned_units); SET_WORK_DETAIL_MODE changes a
// detail's mode (df::work_detail_mode) and recomputes derived labors for
// EVERY current citizen, not just the detail's own membership, since a
// mode change (e.g. EverybodyDoesThis) reshuffles who does what fort-wide;
// CREATE_WORK_DETAIL allocates a brand-new custom detail into a free
// CUSTOM_1..CUSTOM_8 icon slot. See dfhack-plugin/work_details.cpp and the
// work_details query (queries.cpp) for the read side.
constexpr uint8_t COMMAND_TYPE_ASSIGN_WORK_DETAIL   = 0x20;
constexpr uint8_t COMMAND_TYPE_SET_WORK_DETAIL_MODE = 0x21;
constexpr uint8_t COMMAND_TYPE_CREATE_WORK_DETAIL   = 0x22;

// COMMAND_TYPE_BRING_GOODS_TO_DEPOT marks up to MaxCount free fort items
// (filtered by ItemTypeFilter/MaterialFilter substrings) for hauling to
// the built trade depot at (X,Y,Z) -- the safe half of the 2026-07-19
// trade/caravan research pass (docs/decisions.md): DFHack's own
// scripts/internal/caravan/movegoods.lua eligibility filter and
// Items::markForTrade commit, neither of which is viewscreen-bound. See
// trade.cpp's applyBringGoodsToDepot doc comment for the eligibility rules
// and known limitations, and protocol.h's Go counterpart
// (internal/protocol/message.go BringGoodsToDepotDesignation) for the wire
// layout. Executing an actual trade (the Offer/Trade commit) has no safe
// non-viewscreen API and is deliberately NOT implemented.
constexpr uint8_t COMMAND_TYPE_BRING_GOODS_TO_DEPOT = 0x23;

// Location types -- DF-AI's own wire values for df::abstract_building_type's
// INN_TAVERN/TEMPLE/LIBRARY/GUILDHALL/HOSPITAL. A Location is created FROM an
// existing MeetingHall civzone (see designate_zone), not designated
// directly -- these values only ever appear as create_location's `type`
// param. Guildhall additionally requires a profession name (see
// applyCreateLocation) -- confirmed via DFHack's own quickfort reference
// (scripts/internal/quickfort/zone.lua's set_location(), which refuses
// to create a guildhall without one); the other four need no extra input.
constexpr uint8_t LOCATION_TYPE_TAVERN    = 0x01;
constexpr uint8_t LOCATION_TYPE_TEMPLE    = 0x02;
constexpr uint8_t LOCATION_TYPE_LIBRARY   = 0x03;
constexpr uint8_t LOCATION_TYPE_GUILDHALL = 0x04;
constexpr uint8_t LOCATION_TYPE_HOSPITAL  = 0x05;

// Season byte for the SET_FARM_CROP command. 0-3 target one of
// building_farmplotst::plant_id's four season slots (matches df::season:
// Spring/Summer/Autumn/Winter). 0xFF is a wire-level convenience with no DF
// equivalent -- write the same crop into all four slots in one call.
constexpr uint8_t SEASON_SPRING = 0x00;
constexpr uint8_t SEASON_SUMMER = 0x01;
constexpr uint8_t SEASON_AUTUMN = 0x02;
constexpr uint8_t SEASON_WINTER = 0x03;
constexpr uint8_t SEASON_ALL    = 0xFF;

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

// Work detail mode byte -- SET_WORK_DETAIL_MODE and CREATE_WORK_DETAIL's
// Mode parameter. Matches df::work_detail_mode directly (df.plotinfo.xml),
// no translation table: the wire byte IS the enum value. NobodyDoesThis's
// actual in-game effect beyond "not auto-assigned via this detail" is NOT
// confirmed by anything in this checkout -- see work_details.cpp's doc
// comment; acks stay truthful about what byte was written, not a guess
// about DF's job-assignment behavior.
constexpr uint8_t WORK_DETAIL_MODE_DEFAULT                 = 0x00;
constexpr uint8_t WORK_DETAIL_MODE_EVERYBODY_DOES_THIS     = 0x01;
constexpr uint8_t WORK_DETAIL_MODE_NOBODY_DOES_THIS        = 0x02;
constexpr uint8_t WORK_DETAIL_MODE_ONLY_SELECTED_DOES_THIS = 0x03;

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

// Work order frequency byte -- WORK_ORDER's trailing Frequency byte,
// appended (unconditionally, by the encoder) after the existing
// [OrderType][Quantity][optional name tail]. Matches df::workquota_
// frequency_type directly (library/xml/df.workquota.xml): OneTime=0 (also
// df::manager_order::frequency's own struct-zero default -- an absent byte
// and an explicit OneTime byte are indistinguishable on the wire and need no
// separate "unset" sentinel), Daily=1, Monthly=2, Seasonally=3, Yearly=4.
// Kept in sync with internal/protocol/message.go's WorkOrderFrequency*
// consts.
constexpr uint8_t WORK_ORDER_FREQUENCY_ONE_TIME   = 0x00;
constexpr uint8_t WORK_ORDER_FREQUENCY_DAILY      = 0x01;
constexpr uint8_t WORK_ORDER_FREQUENCY_MONTHLY    = 0x02;
constexpr uint8_t WORK_ORDER_FREQUENCY_SEASONALLY = 0x03;
constexpr uint8_t WORK_ORDER_FREQUENCY_YEARLY     = 0x04;

// Sentinel OrderType for the generalized name-based job-type path (0x00
// was never assigned to one of the hand-maintained order types above).
// Shared by BOTH COMMAND_TYPE_QUEUE_JOB and, as of a later pass,
// COMMAND_TYPE_WORK_ORDER — each parses its own payload independently, but
// both interpret this exact OrderType byte value the same way: a trailing
// [2:NameLen][N:Name] follows (QUEUE_JOB: after the X/Y/Z/OrderType
// prefix; WORK_ORDER: after OrderType/Quantity, since it has no
// coordinates) — the SAME length-prefixed-string tail pattern
// COMMAND_TYPE_BLUEPRINT already uses for its blueprint filename
// (df_ai_protocol.cpp case 0x07). The name is looked up against DFHack's
// df::job_type key_table via find_enum_item (work_orders.cpp:
// resolveJobTypeByName) instead of the protocolToJobType switch in
// work_orders.cpp — this is how a caller reaches a job_type that has no
// ORDER_TYPE_* byte above (e.g. "ConstructHatchCover" for QUEUE_JOB,
// "ProcessPlants"/"MakeCheese"/"MilkCreature"/"ShearCreature"/"SpinThread"
// for WORK_ORDER, once a manager exists to dispatch them). Existing
// callers that only ever send bytes 0x01-0x0C are completely unaffected on
// either command: the plugin only looks for a trailing name when it sees
// this exact byte.
//
// NOTE: applyWorkOrder needs no hand-maintained workshop-compatibility
// TABLE for its resolved job_type at all (unlike applyQueueJob, which still
// gates every job_type through jobTypeAllowedAtWorkshop/
// jobTypeAllowedAtFurnace) — DF's own manager picks a compatible workshop
// once a Manager noble with an office scans the queue. CORRECTED (2026-07-19
// manager-work-order fix wave): the manager does NOT fill in every
// job_item's material on its own for job types with real material
// ambiguity (a figurine's wood/stone/metal choice, an ore smelt's raw
// choice) -- those need the WorkOrderDesignation.Material field (see
// message.go) threaded to applyWorkOrder, which sets either
// manager_order::material_category (a job_material_category keyword) or
// mat_type/mat_index (via MaterialInfo::find) before the order is queued.
// A job type with no such ambiguity (PrepareMeal, ConstructBlocks with its
// generic-INORGANIC default) still needs no Material at all.
// ORDER_TYPE_CUSTOM_REACTION (below) remains QUEUE_JOB-only; applyWorkOrder
// does not accept it — ORDER_TYPE_BREW_DRINK above is still the one
// reaction-backed order type the manager-queue path supports.
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
// Ranges: 0x01-0x0F constructions, 0x10-0x2F workshops (incl.
// MetalsmithsForge AND MagmaForge — DF models both as Workshop subtypes
// (df::workshop_type::MetalsmithsForge / MagmaForge), NOT as furnace_type
// values, despite "Magma Forge" sounding like a furnace family member --
// source-confirmed against df.building.xml's workshop_type enum-type block
// (MagmaForge original-name='LAVAMILL' sits inside that same enum-type,
// NOT inside the separate furnace_type enum-type a few lines above it).
// 0x30-0x4F furniture, 0x50-0x6F doors/hatches, 0x70-0x7F furnaces
// (df::building_type::Furnace — a top-level type distinct from Workshop;
// WoodFurnace/Smelter/GlassFurnace/Kiln plus the magma-fueled
// MagmaSmelter/MagmaGlassFurnace/MagmaKiln -- all seven are real
// df::furnace_type values per df.building.xml), 0x80-0x8F trade depot
// (forced 5x5 by DF),
// 0x90-0x9F misc/infrastructure (Well, Support, the room-value
// furniture family Statue/Slab/WindowGlass/WindowGem/Bookcase/
// DisplayFurniture/OfferingPlace/Instrument, and ArcheryTarget/
// TractionBench/NestBox/Hive — each its own top-level building_type,
// forced 1x1 by DF like the doors/hatches range), 0xA0-0xAF
// water/power-transmission infrastructure (ScrewPump, GearAssembly,
// AxleHorizontal, AxleVertical, WaterWheel, Windmill, Rollers — see
// BUILD_TYPE_SCREW_PUMP et al below), 0xB0-0xBF more df::trap_type
// subtypes beyond Lever (which stays at BUILD_TYPE_LEVER in the
// doors/hatches range above, sharing placeDoor's single-mechanism-item
// shape) — PressurePlate, StoneFallTrap, WeaponTrap, TrackStop, see
// BUILD_TYPE_PRESSURE_PLATE et al below.
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
constexpr uint8_t BUILD_TYPE_WS_METALSMITH = 0x19;
constexpr uint8_t BUILD_TYPE_WS_MAGMA_FORGE = 0x1A; // df::workshop_type::MagmaForge -- NOT a furnace_type (see range comment above); needs an anvil (magma_safe, not fire_safe) + magma-safe building material (buildings.lua:229-237)

// Cloth/leather-industry and misc remaining df::workshop_type values --
// closes out "the entire cloth/leather industry is absent end to end"
// (a prior research pass's finding). Recipes per buildings.lua
// workshop_inputs (dfhack-build library/lua/dfhack/buildings.lua:214-294);
// see buildings.cpp placeWorkshop for the exact filter shape of each.
// Deliberately NOT included: df::workshop_type::Tool and ::Custom --
// buildings.lua's workshop_inputs table has NO entry for either (a Lua
// table lookup miss, confirmed by reading the table directly), and
// DFHack's own gui/buildings.lua BuildingDialog:initWorkshopMode excludes
// Tool from its default workshop list the same way it excludes Custom
// (both gated behind opt-in flags, buildings.lua:146) -- i.e. DFHack's own
// canonical consumers treat Tool as belonging to the same "no universal
// recipe" class as Custom (Custom is raws-defined per-mod via
// df::building_def; Tool has no raws-lookup path at all and simply has no
// known reagent set). Both remain resolvable via the BUILD_TYPE_BY_NAME
// path (they are real df::workshop_type keys) but resolveCuratedBuildTypeByte
// has no case for either, so building one yields a truthful "no placement
// recipe yet" error rather than a guessed filter -- see the building_types
// tool for this documented as a limitation, not a silent gap.
constexpr uint8_t BUILD_TYPE_WS_JEWELERS    = 0x1B; // df::workshop_type::Jewelers -- 1x generic building material (buildings.lua:219)
constexpr uint8_t BUILD_TYPE_WS_BOWYERS     = 0x1C; // df::workshop_type::Bowyers -- 1x generic building material (buildings.lua:238)
constexpr uint8_t BUILD_TYPE_WS_SIEGE       = 0x1D; // df::workshop_type::Siege -- 3x generic building material (buildings.lua:240, quantity=3) -- NOT df::building_type::SiegeEngine (the catapult/ballista building, deliberately out of scope); this is the Siege Workshop that preps ammunition
constexpr uint8_t BUILD_TYPE_WS_LEATHERWORKS = 0x1E; // df::workshop_type::Leatherworks -- 1x generic building material (buildings.lua:242)
constexpr uint8_t BUILD_TYPE_WS_TANNERS     = 0x1F; // df::workshop_type::Tanners -- 1x generic building material (buildings.lua:243)
constexpr uint8_t BUILD_TYPE_WS_CLOTHIERS   = 0x20; // df::workshop_type::Clothiers -- 1x generic building material (buildings.lua:244)
constexpr uint8_t BUILD_TYPE_WS_LOOM        = 0x21; // df::workshop_type::Loom -- 1x generic building material (buildings.lua:247)
constexpr uint8_t BUILD_TYPE_WS_KENNELS     = 0x22; // df::workshop_type::Kennels -- 1x generic building material (buildings.lua:249)
constexpr uint8_t BUILD_TYPE_WS_ASHERY      = 0x23; // df::workshop_type::Ashery -- 3 SPECIFIC-item reagents, no generic building-material reagent at all: BLOCKS/BLOCKS (no flags), an EMPTY BARREL/BARREL, and a lye_milk_free BUCKET/BUCKET (buildings.lua:251-268) -- materialClass is rejected, same reasoning as placeWell
constexpr uint8_t BUILD_TYPE_WS_DYERS       = 0x24; // df::workshop_type::Dyers -- 2 SPECIFIC-item reagents, no generic building-material reagent: an EMPTY BARREL/BARREL and a lye_milk_free BUCKET/BUCKET (buildings.lua:269-282) -- materialClass is rejected, same reasoning as placeWell

constexpr uint8_t BUILD_TYPE_BED      = 0x30;
constexpr uint8_t BUILD_TYPE_TABLE    = 0x31;
constexpr uint8_t BUILD_TYPE_CHAIR    = 0x32;
constexpr uint8_t BUILD_TYPE_CABINET  = 0x33;
constexpr uint8_t BUILD_TYPE_COFFER   = 0x34;
constexpr uint8_t BUILD_TYPE_COFFIN   = 0x35; // df::building_type::Coffin -- item_type::COFFIN/vector_id::COFFIN (buildings.lua:38)

constexpr uint8_t BUILD_TYPE_DOOR      = 0x50;
constexpr uint8_t BUILD_TYPE_HATCH     = 0x51;
constexpr uint8_t BUILD_TYPE_LEVER     = 0x52; // df::building_type::Trap, trap_type::Lever -- needs 1 mechanism (TRAPPARTS)
constexpr uint8_t BUILD_TYPE_FLOODGATE = 0x53; // df::building_type::Floodgate -- needs 1 FLOODGATE item, no mechanism at build time

constexpr uint8_t BUILD_TYPE_FURNACE_SMELTER      = 0x70;
constexpr uint8_t BUILD_TYPE_FURNACE_WOOD         = 0x71;
constexpr uint8_t BUILD_TYPE_FURNACE_KILN         = 0x72; // df::furnace_type::Kiln -- fire-safe building material (buildings.lua:206), same shape as Smelter/WoodFurnace
constexpr uint8_t BUILD_TYPE_FURNACE_GLASS        = 0x73; // df::furnace_type::GlassFurnace -- fire-safe building material (buildings.lua:205), same shape
constexpr uint8_t BUILD_TYPE_FURNACE_MAGMA_SMELTER = 0x74; // df::furnace_type::MagmaSmelter -- magma-safe (NOT fire-safe) building material (buildings.lua:207); see placeFurnace's magma-placement-validation doc comment for what DFHack does and does NOT check at build time
constexpr uint8_t BUILD_TYPE_FURNACE_MAGMA_GLASS   = 0x75; // df::furnace_type::MagmaGlassFurnace -- magma-safe building material (buildings.lua:208)
constexpr uint8_t BUILD_TYPE_FURNACE_MAGMA_KILN    = 0x76; // df::furnace_type::MagmaKiln -- magma-safe building material (buildings.lua:209)

constexpr uint8_t BUILD_TYPE_TRADE_DEPOT = 0x80;

constexpr uint8_t BUILD_TYPE_WELL    = 0x90; // df::building_type::Well -- BLOCKS + bucket + chain + mechanism (buildings.lua:79-100)
constexpr uint8_t BUILD_TYPE_SUPPORT = 0x91; // df::building_type::Support -- 1x generic building material (buildings.lua:111); cave-in/collapse trigger, not justice-related

// Room-value furniture family -- df::building_type values with no further
// subtype, each forced 1x1 by DF (getCorrectSize has no case for any of
// them). Recipes per buildings.lua building_inputs (dfhack-build
// library/lua/dfhack/buildings.lua); see buildings.cpp placeRoomValueFurniture
// for the exact filter shape of each. Closes the "craftable but not
// placeable" gap for Statue/Slab specifically (ConstructStatue/ConstructSlab
// were already whitelisted for Carpenters/Masons in a prior wave) the same
// way an earlier wave closed it for Coffin.
constexpr uint8_t BUILD_TYPE_STATUE            = 0x92; // df::building_type::Statue -- 1x STATUE item (buildings.lua:70)
constexpr uint8_t BUILD_TYPE_SLAB              = 0x93; // df::building_type::Slab -- 1x SLAB item, no vector_id narrowing (buildings.lua:178)
constexpr uint8_t BUILD_TYPE_WINDOW_GLASS      = 0x94; // df::building_type::WindowGlass -- 1x WINDOW item (buildings.lua:71)
constexpr uint8_t BUILD_TYPE_WINDOW_GEM        = 0x95; // df::building_type::WindowGem -- 3x SMALLGEM items (buildings.lua:72-78)
constexpr uint8_t BUILD_TYPE_BOOKCASE          = 0x96; // df::building_type::Bookcase -- 1x TOOL item with has_tool_use=BOOKCASE (buildings.lua:183)
constexpr uint8_t BUILD_TYPE_DISPLAY_FURNITURE = 0x97; // df::building_type::DisplayFurniture -- 1x TOOL item with has_tool_use=DISPLAY_OBJECT (buildings.lua:184)
constexpr uint8_t BUILD_TYPE_OFFERING_PLACE    = 0x98; // df::building_type::OfferingPlace -- 1x TOOL item with has_tool_use=PLACE_OFFERING (buildings.lua:181)
constexpr uint8_t BUILD_TYPE_INSTRUMENT        = 0x99; // df::building_type::Instrument -- 1x INSTRUMENT item, vector_id=INSTRUMENT_STATIONARY (buildings.lua:182)

// Additional 1x1-forced-footprint building types (getCorrectSize has no case
// for any of the four below either, same default branch as Well/Support/the
// room-value furniture family above). TractionBench/NestBox/Hive dispatch
// through buildings.cpp's placeRoomValueFurniture purely as CODE REUSE (same
// "one specific-item or tool-use filter, materialClass rejected" shape) --
// they are NOT members of DF's own "room value" furniture family (none of
// the three contributes to a bedroom's furnishing-value score). ArcheryTarget
// instead takes a generic building-material filter like Support/Construction,
// so it gets its own placer (placeArcheryTarget) and accepts materialClass.
constexpr uint8_t BUILD_TYPE_ARCHERY_TARGET = 0x9A; // df::building_type::ArcheryTarget -- 1x generic building material (buildings.lua:112); marksman-dwarf training target, not justice-related
constexpr uint8_t BUILD_TYPE_TRACTION_BENCH = 0x9B; // df::building_type::TractionBench -- 1x TRACTION_BENCH item, vector_id=TRACTION_BENCH (buildings.lua:172-177); hospital traction-splint furniture -- the item itself is crafted via ConstructTractionBench at a Mechanic's workshop (see queue_job / work_orders.cpp)
constexpr uint8_t BUILD_TYPE_NEST_BOX       = 0x9C; // df::building_type::NestBox -- 1x TOOL item with has_tool_use=NEST_BOX (buildings.lua:179); egg-laying animal nesting -- crafting the NEST_BOX tool item itself needs a job_item item_subtype queue_job cannot express yet (pre-existing, out of scope here)
constexpr uint8_t BUILD_TYPE_HIVE           = 0x9D; // df::building_type::Hive -- 1x TOOL item with has_tool_use=HIVE (buildings.lua:180); beekeeping -- same TOOL item_subtype gap as NestBox

// Water/power-transmission infrastructure family -- df::building_type
// values with no further subtype. Recipes per buildings.lua building_inputs
// (dfhack-build library/lua/dfhack/buildings.lua); see buildings.cpp
// placeWaterPowerBuilding for the exact filter shape of each and for the
// Orientation-byte handling shared by ScrewPump/AxleHorizontal/WaterWheel/
// Rollers (see BUILD_ORIENT_* below).
//
// KNOWN LIMITATION -- adjacency is NOT modeled: DF links two touching
// machine buildings (an axle end abutting a gear assembly's tile, a gear
// abutting a water wheel, etc.) automatically at the ENGINE level purely
// from tile adjacency once both exist and the fort is unpaused -- there is
// no separate "connect A to B" parameter to set at placement time, so
// placement here is exactly as automatable as vanilla DF's own build UI:
// place each piece touching its intended neighbor and DF's machine-network
// code (not this plugin) does the rest. Neither this plugin nor its Go
// peer can verify two placed pieces actually formed one working machine
// short of a live in-game check.
constexpr uint8_t BUILD_TYPE_SCREW_PUMP      = 0xA0; // df::building_type::ScrewPump -- BLOCKS + screw (TRAPCOMP) + pipe (PIPE_SECTION) (buildings.lua:116-132); orientation = intake side
constexpr uint8_t BUILD_TYPE_GEAR_ASSEMBLY   = 0xA1; // df::building_type::GearAssembly -- 1x mechanism (TRAPPARTS) (buildings.lua:147-153); no orientation
constexpr uint8_t BUILD_TYPE_AXLE_HORIZONTAL = 0xA2; // df::building_type::AxleHorizontal -- WOOD (buildings.lua:154-156); orientation = axis (horizontal vs vertical); ALWAYS 1 tile long today (see buildings.cpp placeWaterPowerBuilding KNOWN LIMITATION 2 -- no caller-chosen length yet)
constexpr uint8_t BUILD_TYPE_AXLE_VERTICAL   = 0xA3; // df::building_type::AxleVertical -- 1x WOOD (buildings.lua:157); no orientation (single-tile Z-shaft)
constexpr uint8_t BUILD_TYPE_WATER_WHEEL     = 0xA4; // df::building_type::WaterWheel -- 3x WOOD (buildings.lua:158-164); orientation = axis (horizontal vs vertical)
constexpr uint8_t BUILD_TYPE_WINDMILL        = 0xA5; // df::building_type::Windmill -- 4x WOOD (buildings.lua:165-171); no orientation, forced 3x3
constexpr uint8_t BUILD_TYPE_ROLLERS         = 0xA6; // df::building_type::Rollers -- mechanism (TRAPPARTS) + CHAIN (buildings.lua:185-197); orientation = push direction; ALWAYS 1 tile long today (same limitation as AxleHorizontal)

// More df::trap_type subtypes (building_type::Trap) beyond Lever, which
// stays at BUILD_TYPE_LEVER (0x52, doors/hatches range) sharing placeDoor's
// single-mechanism-item shape. All four below are 1x1 ACTUAL buildings
// (getCorrectSize has no case for building_type::Trap, same default branch
// as Lever/Well/Support) placed via buildings.cpp's placeTrap. Recipes per
// buildings.lua trap_inputs (dfhack-build library/lua/dfhack/buildings.lua:
// 298-338) -- see placeTrap's doc comment for the exact filter shape and
// known limitations of each.
constexpr uint8_t BUILD_TYPE_PRESSURE_PLATE  = 0xB0; // df::trap_type::PressurePlate -- 1x mechanism (TRAPPARTS), same shape as Lever (buildings.lua:324-330); can itself be a link_building SOURCE (see mechanisms.cpp applyLinkBuilding) -- trigger CONDITIONS (plate_info: creature size/water/magma/track thresholds and the detect-category flags) are left at DF's raw constructor defaults, not configured by this command
constexpr uint8_t BUILD_TYPE_STONE_FALL_TRAP = 0xB1; // df::trap_type::StoneFallTrap -- 1x mechanism (TRAPPARTS), same shape as Lever (buildings.lua:299-305); builds UNARMED -- arming with a boulder is DF's own separate post-construction Load Stone Trap job, not queued by this command
constexpr uint8_t BUILD_TYPE_WEAPON_TRAP     = 0xB2; // df::trap_type::WeaponTrap -- 2x reagents: mechanism (TRAPPARTS) + weapon/trap-component (vector_id=ANY_WEAPON, buildings.lua:306-316) -- UNLIKE StoneFallTrap this one IS armed at construction time, matching DF's own build-menu behavior
constexpr uint8_t BUILD_TYPE_TRACK_STOP      = 0xB3; // df::trap_type::TrackStop -- 1x generic building material (buildings.lua:338, same shape as Support/ArcheryTarget) -- basic placement only, no minecart track-piece linkage/friction/dump-menu configuration

// Sentinel BuildType for COMMAND_TYPE_BUILD's generalized name-based path
// (0x00 was never assigned to one of the curated BuildType values above) --
// mirrors ORDER_TYPE_BY_NAME's exact shape (queue_job) one section up. When
// a BUILD payload's BuildType byte equals this, a trailing
// [2:NameLen][N:Name] follows the MaterialClass/QualityTier bytes -- the
// SAME length-prefixed-string tail pattern QUEUE_JOB already uses for its
// job-type-by-name path. The name is resolved plugin-side against FOUR
// DFHack enums in turn (df::building_type, then df::workshop_type, then
// df::furnace_type, then df::trap_type -- see buildings.cpp
// resolveBuildTypeByName) instead of a single hand-maintained BuildType
// switch -- this is how a caller reaches a building type that has no
// BUILD_TYPE_* byte above (e.g. "Statue", "Jewelers", "Quern").
// Existing callers that only ever send bytes 0x01-0x9F are completely
// unaffected: the plugin only looks for a trailing name when it sees this
// exact byte.
//
// KNOWN LIMITATION (mirrors ORDER_TYPE_BY_NAME): resolving a name to a real
// DFHack building_type/workshop_type/furnace_type/trap_type does NOT by
// itself mean this plugin knows how to PLACE it -- job_item filter recipes
// (buildings.lua workshop_inputs/furnace_inputs/trap_inputs) are hand-ported
// domain knowledge, one building type at a time. Only names matching one of
// the curated BUILD_TYPE_* bytes above have a recipe today; every other
// resolvable name fails with a truthful "resolved but no placement recipe"
// error naming the resolved building_type/subtype. Discover what's actually
// buildable today via the building_types tool.
constexpr uint8_t BUILD_TYPE_BY_NAME = 0x00;

// Material class byte -- COMMAND_TYPE_BUILD's first trailing byte (payload
// offset 12, right after BuildType). Constrains which item CLASS DF's job
// system may claim for a generic building-material job_item filter
// (constructions, workshops, furnaces, trade depot -- makeBuildMatFilter/
// makeFireSafeBuildMatFilter in buildings.cpp); DF still picks the
// specific item within that class. Does NOT apply to furniture or
// doors/hatches/levers/floodgates, which already filter on one specific
// finished item type (makeItemFilter) rather than a raw material class --
// applyBuildDesignation rejects a non-ANY class for those build types.
// Kept in sync with internal/protocol/message.go's MaterialClass* consts.
constexpr uint8_t MATERIAL_CLASS_ANY    = 0x00;
constexpr uint8_t MATERIAL_CLASS_WOOD   = 0x01;
constexpr uint8_t MATERIAL_CLASS_STONE  = 0x02;
constexpr uint8_t MATERIAL_CLASS_BLOCKS = 0x03;

// Quality tier byte -- COMMAND_TYPE_BUILD's second trailing byte (payload
// offset 13, right after the material class byte). Matches df::item_quality
// (DFHack-only enum, confirmed against df.dfhack.xml: Ordinary/WellCrafted/
// FinelyCrafted/Superior/Exceptional/Masterful/Artifact -- "Masterful", NOT
// "Masterwork"). Only meaningful for furniture build types (Bed/Table/
// Chair/Cabinet/Coffer/Coffin): selects an EXISTING item of at least this
// quality at PLACEMENT time (Buildings::constructWithItems) instead of
// accepting any matching item (Buildings::constructWithFilters) -- quality
// cannot be requested at craft time in vanilla DF, only chosen among what
// already exists. applyBuildDesignation rejects a non-ANY tier for any
// non-furniture build type. 0xFF is the backward-compatible "no
// constraint" sentinel (payload absent, or explicitly requested). Kept in
// sync with internal/protocol/message.go's QualityTier* consts.
constexpr uint8_t QUALITY_TIER_ORDINARY       = 0x00;
constexpr uint8_t QUALITY_TIER_WELL_CRAFTED   = 0x01;
constexpr uint8_t QUALITY_TIER_FINELY_CRAFTED = 0x02;
constexpr uint8_t QUALITY_TIER_SUPERIOR       = 0x03;
constexpr uint8_t QUALITY_TIER_EXCEPTIONAL    = 0x04;
constexpr uint8_t QUALITY_TIER_MASTERFUL      = 0x05;
constexpr uint8_t QUALITY_TIER_ARTIFACT       = 0x06;
constexpr uint8_t QUALITY_TIER_ANY            = 0xFF;

inline bool isBuildTypeWorkshop(uint8_t t)     { return t >= 0x10 && t < 0x30; }
inline bool isBuildTypeFurniture(uint8_t t)    { return t >= 0x30 && t < 0x50; }
inline bool isBuildTypeConstruction(uint8_t t) { return t >= 0x01 && t < 0x10; }
inline bool isBuildTypeDoor(uint8_t t)         { return t >= 0x50 && t < 0x70; }
inline bool isBuildTypeFurnace(uint8_t t)      { return t >= 0x70 && t < 0x80; }
inline bool isBuildTypeDepot(uint8_t t)        { return t >= 0x80 && t < 0x90; }
inline bool isBuildTypeInfra(uint8_t t)        { return t >= 0x90 && t < 0xA0; }
inline bool isBuildTypeWaterPower(uint8_t t)   { return t >= 0xA0 && t < 0xB0; }
inline bool isBuildTypeTrap(uint8_t t)         { return t >= 0xB0 && t < 0xC0; }

// Bridge direction byte -- COMMAND_TYPE_BUILD_BRIDGE's trailing byte.
// DF-AI's own wire values, translated by the plugin (buildings.cpp:
// placeBridge) to df::building_bridgest::T_direction (Retracting=-1,
// Left=0, Right=1, Up=2, Down=3 -- dfhack-build/library/include/df/
// building_bridgest.h). Names describe the visible effect confirmed via
// dfhack-build/scripts/internal/quickfort/build.lua:468-478: Up raises to
// North, Right raises to East, Down raises to South, Left raises to West;
// Retracting slides the bridge away instead of raising it vertically.
constexpr uint8_t BRIDGE_DIR_RETRACT = 0x00;
constexpr uint8_t BRIDGE_DIR_RAISE_N = 0x01;
constexpr uint8_t BRIDGE_DIR_RAISE_S = 0x02;
constexpr uint8_t BRIDGE_DIR_RAISE_E = 0x03;
constexpr uint8_t BRIDGE_DIR_RAISE_W = 0x04;

// Orientation byte -- COMMAND_TYPE_BUILD's THIRD trailing byte (payload
// offset 14, right after QualityTier), used only by the water/power-
// transmission building family (BUILD_TYPE_SCREW_PUMP/AXLE_HORIZONTAL/
// WATER_WHEEL/ROLLERS). This is deliberately NOT a new command type or a
// bridge-style rectangle+direction shape: unlike Bridge, the caller never
// chooses this footprint (Buildings::getCorrectSize computes it from the
// building type + this same direction value, dfhack-build library/modules/
// Buildings.cpp:634-651,710-734) -- these are single-tile-anchor BUILD
// commands, exactly like Well/Support above, plus one more optional
// trailing byte.
//
// Passed straight through as the raw `direction` int to
// Buildings::setSize's 3-arg overload (buildings.cpp: placeWaterPowerBuilding)
// -- DF itself casts that ONE int two different ways depending on building
// type (Buildings.cpp:950-973):
//   AxleHorizontal, WaterWheel -> bool via `!!direction` (0 = horizontal,
//                                 an E-W line; nonzero = vertical, an N-S
//                                 line).
//   ScrewPump, Rollers         -> df::screw_pump_direction (FromNorth=0,
//                                 FromEast=1, FromSouth=2, FromWest=3) --
//                                 for ScrewPump, which side draws water
//                                 FROM; for Rollers, which direction items
//                                 are pushed.
//   GearAssembly, AxleVertical -> ignored (always 1x1 -- getCorrectSize
//                                 has no case for either).
// BUILD_ORIENT_HORIZONTAL/BUILD_ORIENT_NORTH (and BUILD_ORIENT_VERTICAL/
// BUILD_ORIENT_EAST) are the SAME wire value on purpose -- one int, two
// meanings, exactly as DF itself treats it. BUILD_ORIENT_ANY (0xFF) is the
// backward-compatible "unspecified" sentinel: a payload without this byte
// decodes as BUILD_ORIENT_ANY, which the plugin maps to 0 -- a legal
// default for every one of these types.
constexpr uint8_t BUILD_ORIENT_HORIZONTAL = 0x00;
constexpr uint8_t BUILD_ORIENT_VERTICAL   = 0x01;
constexpr uint8_t BUILD_ORIENT_NORTH      = 0x00;
constexpr uint8_t BUILD_ORIENT_EAST       = 0x01;
constexpr uint8_t BUILD_ORIENT_SOUTH      = 0x02;
constexpr uint8_t BUILD_ORIENT_WEST       = 0x03;
constexpr uint8_t BUILD_ORIENT_ANY        = 0xFF;

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
