# Data Model: Spatial Validator Planner and Housing Zones

**Feature**: 007-svp-housing-zones
**Date**: 2025-11-09
**Status**: Complete

## Overview

This document defines all core entities, their attributes, relationships, validation rules, and state transitions for Feature 007. Each entity is designed to be independently testable with clear responsibilities.

---

## Entity 1: SpatialValidatorPlanner

**Purpose**: Strategic component that analyzes embark terrain to establish organized vertical fort structure, designating Z-levels for different fort functions.

**Location**: `internal/spatial/planner.go`

### Attributes

| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| embarkZ | int | Z-level where fortress embark occurred | 0-199 (DF map bounds) |
| housingZ | int | Designated Z-level for bedrooms/dormitories | 0-199, must be < embarkZ |
| workshopZ | int | Designated Z-level for workshops/stockpiles | 0-199, must be < housingZ |
| farmZ | []int | Array of Z-levels suitable for farming (has soil) | 0-3 elements, each 0-199 |
| hazardZ | []int | Z-levels to avoid (aquifer, lava, caverns) | 0-200 elements |
| fortName | string | Name of fortress (unique identifier for persistence) | Non-empty, max 100 chars |
| analyzedAt | time.Time | When terrain analysis was performed | Not zero |
| layoutVersion | string | Schema version for migration support | Semver format (e.g., "1.0") |
| persistencePath | string | File path for saving layout | Valid path ending in .json |

### Methods

```go
// AnalyzeTerrain performs initial terrain analysis using topology and entity data
func (svp *SpatialValidatorPlanner) AnalyzeTerrain(
    topology *topology.Overlay,
    entities []*protocol.Entity,
    hazards *hazards.Manager,
) error

// GetHousingZ returns designated Z-level for housing
func (svp *SpatialValidatorPlanner) GetHousingZ() int

// GetWorkshopZ returns designated Z-level for workshops
func (svp *SpatialValidatorPlanner) GetWorkshopZ() int

// GetFarmZ returns array of Z-levels suitable for farming
func (svp *SpatialValidatorPlanner) GetFarmZ() []int

// ValidateProposal checks if proposed region complies with Z-level designations
func (svp *SpatialValidatorPlanner) ValidateProposal(
    nodeType agents.NodeType,
    region modifications.Region,
) (bool, string) // Returns (valid, error_message)

// Save persists layout to disk
func (svp *SpatialValidatorPlanner) Save() error

// Load restores layout from disk
func (svp *SpatialValidatorPlanner) Load(fortName string) error
```

### Relationships

- **Depends on**: TopologyOverlay (embark terrain data), HazardManager (aquifer/lava detection), EntityCache (dwarf positions)
- **Used by**: HousingAgent (queries housing Z), FoodAgent (queries farm Z), GraphExecutor (validates proposals)

### State Transitions

```
UNINITIALIZED → ANALYZING (AnalyzeTerrain called)
ANALYZING → READY (analysis complete, designations set)
READY → PERSISTED (Save called)
PERSISTED → READY (Load called on reconnection)
```

### Validation Rules

1. **Housing Z < Embark Z**: Housing layer must be below surface (embark - 5 heuristic)
2. **Workshop Z < Housing Z**: Workshops immediately below bedrooms (housing - 1)
3. **Farm Z must have soil**: All elements in farmZ array must have is_soil_layer=true
4. **No hazard overlap**: housingZ, workshopZ, farmZ must not be in hazardZ array
5. **Z-levels in bounds**: All Z values must be 0-199 (DF map bounds)

### Invariants

- Once analyzed, designations remain stable (no dynamic re-analysis unless requested)
- Fort name is immutable after initialization
- Persistence file matches fort name (one layout per fort)

---

## Entity 2: ZLevelDesignation

**Purpose**: Represents a designated purpose for a Z-level with capacity and hazard metadata.

**Location**: `internal/spatial/layout.go`

### Attributes

| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| zLevel | int | Z-coordinate of this level | 0-199 |
| purpose | DesignationPurpose | Intended use (housing, workshop, farm, etc.) | Enum value |
| capacity | CapacityStatus | Space availability (available, partial, full) | Enum value |
| hasSoil | bool | Whether this Z-level contains soil | N/A |
| hasAquifer | bool | Whether this Z-level has aquifer | N/A |
| hasLava | bool | Whether this Z-level has lava | N/A |
| isCavern | bool | Whether this Z-level is cavern layer | N/A |
| notes | string | Human-readable rationale for designation | Max 200 chars |

### Enums

```go
type DesignationPurpose int
const (
    PurposeHousing DesignationPurpose = iota
    PurposeWorkshop
    PurposeFarm
    PurposeStorage
    PurposeIndustrial
    PurposeUnassigned
)

type CapacityStatus int
const (
    CapacityAvailable CapacityStatus = iota
    CapacityPartial
    CapacityFull
)
```

### Methods

```go
// IsSafe returns true if no hazards present
func (d *ZLevelDesignation) IsSafe() bool

// CanAccommodate checks if this Z-level suitable for given purpose
func (d *ZLevelDesignation) CanAccommodate(purpose DesignationPurpose) bool
```

### Relationships

- **Contained in**: SpatialValidatorPlanner (array of designations)
- **Used by**: SVP validation logic, logging, debugging

### Validation Rules

1. **Hazard consistency**: If hasAquifer/hasLava/isCavern, capacity must be Unavailable
2. **Soil consistency**: If purpose=Farm, hasSoil must be true
3. **Purpose-capacity alignment**: If purpose=Housing and capacity=Full, no more bedrooms should be proposed

---

## Entity 3: ZoneInfo

**Purpose**: Represents a DF zone extracted from game state with type, coordinates, and assignment status.

**Location**: `internal/zones/types.go`

### Attributes

| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| zoneID | uint32 | Unique identifier from DF | Non-zero |
| zoneType | ZoneType | Zone category (bedroom, dining, etc.) | Enum value |
| region | protocol.Region | Bounding box (x1,y1,z1,x2,y2,z2) | Valid region |
| assignedTo | int32 | Dwarf ID if assigned, -1 if unassigned | -1 or valid dwarf ID |
| sizeX | uint16 | Width in tiles | > 0 |
| sizeY | uint16 | Height in tiles | > 0 |
| createdAt | time.Time | When zone was first detected | Not zero |

### Enums

```go
type ZoneType int
const (
    ZoneTypeBedroom ZoneType = iota
    ZoneTypeDining
    ZoneTypeDormitory
    ZoneTypeOffice
    ZoneTypeBarracks
    ZoneTypeWorkshop
    ZoneTypeStockpile
)
```

### Methods

```go
// IsAssigned returns true if zone has owner
func (z *ZoneInfo) IsAssigned() bool

// GetArea returns tile count
func (z *ZoneInfo) GetArea() int
```

### Relationships

- **Extracted by**: ZoneExtractor (queries DF API)
- **Used by**: Fort metrics computation (counts zones by type), HousingAgent (calculates deficit)

### Validation Rules

1. **Type-assignment consistency**: Only bedroom/office/barracks can have assignedTo != -1
2. **Region validity**: x2 > x1, y2 > y1, z1 == z2 (single Z-level zones)
3. **Size consistency**: sizeX == (x2-x1+1), sizeY == (y2-y1+1)

---

## Entity 4: ZoneExtractor

**Purpose**: Component that queries DF building module to extract zone data and compute zone counts for fort metrics.

**Location**: `internal/zones/extractor.go`

### Attributes

| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| pluginClient | *protocol.DFHackClient | Client for querying DF data | Not nil |
| logger | *logging.Logger | Logging instance | Not nil |
| lastExtractTime | time.Time | Timestamp of last extraction | Updated each cycle |
| zoneCache | map[uint32]*ZoneInfo | Cached zones by ID | N/A |
| extractionEnabled | bool | Feature toggle | N/A |

### Methods

```go
// ExtractZones queries DF API for current zone list
func (ze *ZoneExtractor) ExtractZones() ([]*ZoneInfo, error)

// CountByType returns zone counts by type (bedroom: 5, dining: 2, etc.)
func (ze *ZoneExtractor) CountByType(zones []*ZoneInfo) map[ZoneType]int

// GetUnassignedCount returns count of zones without owners
func (ze *ZoneExtractor) GetUnassignedCount(zones []*ZoneInfo, zoneType ZoneType) int
```

### Relationships

- **Depends on**: DFHack plugin (zone extraction protocol), protocol.DFHackClient
- **Used by**: Autonomous loop (calls ExtractZones during metrics computation)
- **Produces**: ZoneInfo structs, zone count metrics

### Validation Rules

1. **Extraction frequency**: Should not extract more than once per second (rate limiting)
2. **Error handling**: Must gracefully handle DF API failures (return cached data with warning)
3. **Cache consistency**: zoneCache updated atomically (no partial updates)

### State Transitions

```
IDLE → EXTRACTING (ExtractZones called)
EXTRACTING → SUCCESS (zones returned)
EXTRACTING → ERROR (DF API failed, use cache)
SUCCESS/ERROR → IDLE (wait for next cycle)
```

---

## Entity 5: ZoneQueue

**Purpose**: Manages async execution of zone commands that cannot be placed immediately (space not yet dug).

**Location**: `internal/zones/queue.go`

### Attributes

| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| queuedZones | []*QueuedZone | Pending zone commands | Max 50 elements |
| modificationOverlay | *modifications.Overlay | For dig completion verification | Not nil |
| logger | *logging.Logger | Logging instance | Not nil |
| persistencePath | string | File path for queue persistence | Valid path ending in .json |

### Methods

```go
// Enqueue adds zone to retry queue
func (zq *ZoneQueue) Enqueue(zone *QueuedZone)

// ProcessQueue attempts to place queued zones
func (zq *ZoneQueue) ProcessQueue(dfhackClient *protocol.DFHackClient) []QueueResult

// Save persists queue to disk
func (zq *ZoneQueue) Save() error

// Load restores queue from disk
func (zq *ZoneQueue) Load() error

// GetPendingCount returns count of zones still queued
func (zq *ZoneQueue) GetPendingCount() int
```

### Relationships

- **Depends on**: ModificationOverlay (dig completion verification), DFHackClient (zone command execution)
- **Used by**: GraphExecutor (enqueues failed ZONE commands), autonomous loop (calls ProcessQueue each cycle)

### Validation Rules

1. **Max queue size**: Queue limited to 50 zones (prevent memory growth)
2. **Retry limit**: Each zone max 3 retry attempts
3. **Timeout**: Each zone max 10 cycles in queue
4. **Dig verification**: Zone only retried if tiles confirmed dug (modification overlay check)

### State Transitions (per QueuedZone)

```
ENQUEUED → WAITING_FOR_DIG (initial state)
WAITING_FOR_DIG → RETRYING (dig complete, retry ZONE)
RETRYING → COMPLETED (zone placed successfully)
RETRYING → WAITING_FOR_DIG (zone failed, dig still incomplete)
WAITING_FOR_DIG → FAILED (timeout or retry limit exceeded)
```

---

## Entity 6: QueuedZone

**Purpose**: Represents a single queued zone command with retry metadata.

**Location**: `internal/zones/queue.go`

### Attributes

| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| zoneType | protocol.ZoneType | Type of zone to create | Enum value |
| region | protocol.Region | Target coordinates | Valid region |
| commandID | uint32 | Associated dig command ID | Non-zero |
| retryCount | int | How many retries attempted | 0-3 |
| maxRetries | int | Retry limit | Default: 3 |
| timeoutCycles | int | Cycles remaining before timeout | 1-10 |
| status | QueueStatus | Current state | Enum value |
| createdAt | time.Time | When zone was queued | Not zero |
| lastRetryAt | time.Time | Last retry attempt timestamp | May be zero |

### Enums

```go
type QueueStatus int
const (
    QueueStatusWaitingForDig QueueStatus = iota
    QueueStatusRetrying
    QueueStatusCompleted
    QueueStatusFailed
)
```

### Methods

```go
// ShouldRetry returns true if retry conditions met
func (qz *QueuedZone) ShouldRetry() bool

// IncrementRetry updates retry counter and timestamp
func (qz *QueuedZone) IncrementRetry()

// DecrementTimeout reduces timeout counter
func (qz *QueuedZone) DecrementTimeout()

// IsTimedOut returns true if timeout reached
func (qz *QueuedZone) IsTimedOut() bool
```

### Relationships

- **Contained in**: ZoneQueue (array of queued zones)
- **References**: CommandID links to dig command in modification overlay

### Validation Rules

1. **Retry limit**: retryCount must not exceed maxRetries (3)
2. **Timeout enforcement**: timeoutCycles decremented each cycle, zone removed at 0
3. **Status consistency**: If status=Completed or Failed, zone should be removed from queue

---

## Entity 7: BlueprintMetadata

**Purpose**: Metadata about blueprints for arbiter context (name, dimensions, capacity).

**Location**: `internal/blueprints/metadata.go`

### Attributes

| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| name | string | Blueprint filename (without .csv) | Non-empty, max 50 chars |
| displayName | string | Human-readable name | Non-empty, max 100 chars |
| width | int | Horizontal dimension (X) | > 0 |
| height | int | Depth dimension (Y) | > 0 |
| depth | int | Vertical dimension (Z) | Default: 1 (single level) |
| tileCount | int | Total tiles in blueprint | > 0 |
| dwarfCapacity | int | How many dwarves this blueprint houses | >= 0 (0 for non-housing) |
| description | string | Purpose and features | Max 200 chars |
| tags | []string | Categories (bedroom, compact, nobles, etc.) | 0-5 elements |
| suitabilityCriteria | map[string]interface{} | Min space, constraints | N/A |

### Methods

```go
// ToPromptString formats metadata for arbiter system prompt
func (bm *BlueprintMetadata) ToPromptString() string

// FitsInSpace checks if blueprint fits in available region
func (bm *BlueprintMetadata) FitsInSpace(availableWidth, availableHeight int) bool
```

### Relationships

- **Generated by**: Blueprint loader (parses CSV, counts tiles, infers capacity)
- **Used by**: Arbiter system prompt builder (formats metadata for LLM)

### Validation Rules

1. **Dimensions match tile count**: width × height × depth should approximately equal tileCount (allowing for walls)
2. **Capacity consistency**: If tags include "bedroom", dwarfCapacity > 0
3. **Prompt token budget**: ToPromptString output must be < 100 tokens per blueprint

---

## Entity 8: ZoneCSVParser

**Purpose**: Parses zone CSV companion files to extract zone designations for blueprints.

**Location**: Plugin (`plugin/df-ai-plugin.cpp`) - documented here for completeness

### Attributes (C++ struct)

```cpp
struct ZoneDesignation {
    int x;           // Zone origin X (relative to blueprint)
    int y;           // Zone origin Y (relative to blueprint)
    int z;           // Zone origin Z (relative to blueprint)
    ZoneType type;   // bedroom, dining, etc.
    int width;       // Zone width
    int height;      // Zone height
};
```

### Methods (C++ functions)

```cpp
// Parse zone CSV file
std::vector<ZoneDesignation> ParseZoneCSV(const std::string& filepath);

// Validate zone fits within blueprint bounds
bool ValidateZone(const ZoneDesignation& zone, int blueprintWidth, int blueprintHeight);

// Apply zone designation via DF API
bool ApplyZone(const ZoneDesignation& zone, int originX, int originY, int originZ);
```

### Relationships

- **Used by**: Plugin BLUEPRINT command handler
- **Produces**: ZoneDesignation structs (plugin-side, not sent to orchestrator)

### Validation Rules

1. **CSV format**: Must have 6 columns (x,y,z,zone_type,width,height)
2. **Zone bounds**: x+width and y+height must fit within blueprint dimensions
3. **Zone type mapping**: zone_type string must map to DF ZoneType enum

---

## Metrics Enhancement

### Fort Metrics (Enhanced in Feature 007)

**Existing Metrics** (from Feature 006):
- DwarfCount (int)
- FoodPerDwarf (float64) - still placeholder
- DrinkPerDwarf (float64) - still placeholder
- BedroomCount (int) - **REPLACED** with BedroomZoneCount
- MiningTilesPerCycle (int) - still placeholder
- WealthGrowthRate (float64) - still placeholder
- EnemyCount (int)
- FortAge (int)
- Phase (phases.FortPhase)

**New Metrics** (Feature 007):
- BedroomZoneCount (int) - Real count from zone extraction
- DiningZoneCount (int)
- DormitoryZoneCount (int)
- OfficeZoneCount (int)
- UnassignedBedroomCount (int) - Bedrooms without owners
- HousingDeficit (int) - DwarfCount - BedroomZoneCount
- SVPHousingZ (int) - Designated housing Z-level
- SVPWorkshopZ (int) - Designated workshop Z-level
- SVPFarmZ ([]int) - Designated farm Z-levels

**Computation** (in autonomous/loop.go):
```go
func (al *AutonomousLoop) computeFortMetrics() *agents.FortMetrics {
    metrics := &agents.FortMetrics{}

    // Existing entity counts
    entities := al.getEntities()
    for _, e := range entities {
        switch e.Type {
        case protocol.EntityTypeDwarf:
            metrics.DwarfCount++
        case protocol.EntityTypeEnemy:
            metrics.EnemyCount++
        }
    }

    // NEW: Zone extraction
    if al.zoneExtractor != nil && al.zoneExtractor.IsEnabled() {
        zones, err := al.zoneExtractor.ExtractZones()
        if err == nil {
            zoneCounts := al.zoneExtractor.CountByType(zones)
            metrics.BedroomZoneCount = zoneCounts[zones.ZoneTypeBedroom]
            metrics.DiningZoneCount = zoneCounts[zones.ZoneTypeDining]
            metrics.DormitoryZoneCount = zoneCounts[zones.ZoneTypeDormitory]
            metrics.OfficeZoneCount = zoneCounts[zones.ZoneTypeOffice]
            metrics.UnassignedBedroomCount = al.zoneExtractor.GetUnassignedCount(zones, zones.ZoneTypeBedroom)
            metrics.HousingDeficit = metrics.DwarfCount - metrics.BedroomZoneCount
        } else {
            al.logger.Warn("Zone extraction failed, using fallback")
            // Fall back to chamber count estimation (Feature 005)
        }
    }

    // NEW: SVP designations
    if al.svp != nil && al.svp.IsReady() {
        metrics.SVPHousingZ = al.svp.GetHousingZ()
        metrics.SVPWorkshopZ = al.svp.GetWorkshopZ()
        metrics.SVPFarmZ = al.svp.GetFarmZ()
    }

    return metrics
}
```

---

## Persistence Schema

### SVP Layout (saves/{fortname}/svp_layout.json)

```json
{
  "fort_name": "Doomfortress",
  "embark_z": 125,
  "housing_z": 120,
  "workshop_z": 119,
  "farm_z": [124, 125],
  "hazard_z": [115, 100],
  "analyzed_at": "2025-11-09T14:32:00Z",
  "version": "1.0"
}
```

### Zone Queue (saves/{fortname}/zone_queue.json)

```json
{
  "queued_zones": [
    {
      "zone_type": "bedroom",
      "region": {"x1": 60, "y1": 30, "z1": 120, "x2": 63, "y2": 33, "z2": 120},
      "command_id": 1234567890,
      "retry_count": 1,
      "max_retries": 3,
      "timeout_cycles": 8,
      "status": "waiting_for_dig",
      "created_at": "2025-11-09T14:35:00Z",
      "last_retry_at": "2025-11-09T14:35:30Z"
    }
  ],
  "version": "1.0"
}
```

---

## Summary

**Core Entities**: 8 total
- **3 new packages**: SpatialValidatorPlanner, ZoneExtractor, ZoneQueue
- **5 supporting types**: ZLevelDesignation, ZoneInfo, QueuedZone, BlueprintMetadata, ZoneCSVParser (plugin)
- **1 enhanced entity**: Fort Metrics (10 new fields)

**Key Relationships**:
- SVP → HousingAgent (housing Z query)
- ZoneExtractor → Fort Metrics (zone counts)
- ZoneQueue → Executor (ZONE retry)
- BlueprintMetadata → Arbiter (system prompt)

**Persistence**: 2 JSON files per fort (svp_layout.json, zone_queue.json)

**Testing Surface**:
- SVP: Mock topology, mock hazards, test designations
- ZoneExtractor: Mock DF zone data, test counting
- ZoneQueue: Simulated async timing, test retry logic
- BlueprintMetadata: Parse test CSVs, validate prompt format
