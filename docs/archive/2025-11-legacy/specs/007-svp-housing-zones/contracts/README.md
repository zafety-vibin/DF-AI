# API Contracts: Spatial Validator Planner and Housing Zones

**Feature**: 007-svp-housing-zones
**Date**: 2025-11-09

## Overview

Feature 007 consists entirely of **internal Go APIs** and **DFHack plugin enhancements**. There are no external REST/GraphQL endpoints or public APIs. This document defines the internal interfaces used between components.

---

## Internal Go Interfaces

### 1. SpatialValidatorPlanner Interface

**Package**: `internal/spatial`

**Purpose**: Provides SVP functionality to agents and executor for Z-level designation and spatial validation.

```go
package spatial

type Planner interface {
    // AnalyzeTerrain performs initial terrain analysis
    AnalyzeTerrain(
        topology *topology.Overlay,
        entities []*protocol.Entity,
        hazards *hazards.Manager,
    ) error

    // GetHousingZ returns designated Z-level for housing
    GetHousingZ() int

    // GetWorkshopZ returns designated Z-level for workshops
    GetWorkshopZ() int

    // GetFarmZ returns array of Z-levels suitable for farming
    GetFarmZ() []int

    // ValidateProposal checks if proposed region complies with Z-level designations
    ValidateProposal(
        nodeType agents.NodeType,
        region modifications.Region,
    ) (valid bool, errorMsg string)

    // IsReady returns true if SVP has completed analysis
    IsReady() bool

    // Save persists layout to disk
    Save() error

    // Load restores layout from disk for given fort name
    Load(fortName string) error
}
```

**Usage Example**:
```go
// In HousingAgent.Analyze()
if al.svp != nil && al.svp.IsReady() {
    housingZ := al.svp.GetHousingZ()
    proposal := ModificationNode{
        Region: modifications.Region{
            XMin: 60, XMax: 80,
            YMin: 30, YMax: 50,
            ZMin: housingZ, ZMax: housingZ,  // Use SVP-designated Z
        },
        // ...
    }
}
```

---

### 2. ZoneExtractor Interface

**Package**: `internal/zones`

**Purpose**: Extracts zone data from DF and provides zone counts for fort metrics.

```go
package zones

type Extractor interface {
    // ExtractZones queries DF API for current zone list
    ExtractZones() ([]*ZoneInfo, error)

    // CountByType returns zone counts by type
    CountByType(zones []*ZoneInfo) map[ZoneType]int

    // GetUnassignedCount returns count of zones without owners
    GetUnassignedCount(zones []*ZoneInfo, zoneType ZoneType) int

    // IsEnabled returns true if zone extraction is configured
    IsEnabled() bool
}
```

**Usage Example**:
```go
// In AutonomousLoop.computeFortMetrics()
if al.zoneExtractor != nil && al.zoneExtractor.IsEnabled() {
    zones, err := al.zoneExtractor.ExtractZones()
    if err == nil {
        zoneCounts := al.zoneExtractor.CountByType(zones)
        metrics.BedroomZoneCount = zoneCounts[zones.ZoneTypeBedroom]
        metrics.DiningZoneCount = zoneCounts[zones.ZoneTypeDining]
    }
}
```

---

### 3. ZoneQueue Interface

**Package**: `internal/zones`

**Purpose**: Manages async execution of zone commands that cannot be placed immediately.

```go
package zones

type Queue interface {
    // Enqueue adds zone to retry queue
    Enqueue(zone *QueuedZone)

    // ProcessQueue attempts to place queued zones
    ProcessQueue(dfhackClient *protocol.DFHackClient) []QueueResult

    // GetPendingCount returns count of zones still queued
    GetPendingCount() int

    // Save persists queue to disk
    Save() error

    // Load restores queue from disk
    Load() error
}

type QueueResult struct {
    ZoneID   uint32
    Status   QueueStatus  // Completed, Failed, StillPending
    Error    string
}
```

**Usage Example**:
```go
// In GraphExecutor.Execute()
err := dfhackClient.SendZoneCommand(zoneCmd)
if err != nil && err.IsSpaceNotAvailable() {
    queuedZone := &QueuedZone{
        ZoneType: zoneCmd.Type,
        Region:   zoneCmd.Region,
        CommandID: zoneCmd.ID,
        // ...
    }
    al.zoneQueue.Enqueue(queuedZone)
}

// In AutonomousLoop.runCycle()
if al.zoneQueue != nil {
    results := al.zoneQueue.ProcessQueue(al.dfhackClient)
    for _, result := range results {
        if result.Status == QueueStatusCompleted {
            al.logger.Info("Zone placed successfully", "zone_id", result.ZoneID)
        }
    }
}
```

---

### 4. BlueprintMetadata Interface

**Package**: `internal/blueprints`

**Purpose**: Provides blueprint metadata for arbiter system prompt.

```go
package blueprints

type MetadataProvider interface {
    // LoadMetadata generates metadata for all blueprints in directory
    LoadMetadata(blueprintDir string) ([]*BlueprintMetadata, error)

    // GetMetadataPrompt formats blueprint metadata for arbiter system prompt
    GetMetadataPrompt(metadata []*BlueprintMetadata) string

    // FindBlueprint returns metadata for specific blueprint by name
    FindBlueprint(name string) (*BlueprintMetadata, error)
}
```

**Usage Example**:
```go
// In arbiter system prompt builder
blueprintMetadata, _ := metadataProvider.LoadMetadata("./blueprints")
promptAddition := metadataProvider.GetMetadataPrompt(blueprintMetadata)

systemPrompt := basePrompt + "\n\nAvailable Blueprints:\n" + promptAddition
```

---

## DFHack Plugin Protocol Extensions

### 1. BLUEPRINT Command (New)

**Protocol Buffer Definition** (extends existing CommandMessage):

```protobuf
message CommandMessage {
  enum CommandType {
    DIG = 0;
    ZONE = 1;
    BLUEPRINT = 2;  // NEW
  }

  CommandType type = 1;
  uint32 command_id = 2;

  // Existing fields for DIG, ZONE...

  // NEW: For BLUEPRINT type
  string blueprint_name = 10;
  uint32 origin_x = 11;
  uint32 origin_y = 12;
  uint32 origin_z = 13;
}
```

**Plugin Handler**:
```cpp
// In plugin command handler
if (cmd.type == CommandType::BLUEPRINT) {
    std::string blueprintPath = "blueprints/" + cmd.blueprint_name + ".csv";
    std::string zonesPath = "blueprints/" + cmd.blueprint_name + "_zones.csv";

    auto digPattern = ParseBlueprintCSV(blueprintPath);
    auto zones = ParseZoneCSV(zonesPath);

    ApplyDigPattern(digPattern, cmd.origin_x, cmd.origin_y, cmd.origin_z);
    QueueZones(zones, cmd.origin_x, cmd.origin_y, cmd.origin_z);

    return {success: true, message: "Blueprint applied"};
}
```

---

### 2. Zone Extraction (Enhanced ENTITY_UPDATE)

**Protocol Buffer Definition** (extends existing EntityUpdate):

```protobuf
message EntityUpdate {
  repeated Entity entities = 1;

  // NEW: Zone data
  repeated ZoneData zones = 2;
}

message ZoneData {
  uint32 zone_id = 1;
  ZoneType zone_type = 2;
  uint32 x1 = 3;
  uint32 y1 = 4;
  uint32 z1 = 5;
  uint32 x2 = 6;
  uint32 y2 = 7;
  uint32 z2 = 8;
  int32 assigned_to = 9;  // Dwarf ID or -1
}

enum ZoneType {
  ZONE_BEDROOM = 0;
  ZONE_DINING = 1;
  ZONE_DORMITORY = 2;
  ZONE_OFFICE = 3;
  ZONE_BARRACKS = 4;
  ZONE_WORKSHOP = 5;
  ZONE_STOCKPILE = 6;
}
```

**Plugin Handler**:
```cpp
// In ENTITY_UPDATE handler
EntityUpdate update;

// Existing entity extraction...

// NEW: Zone extraction
for (auto zone : df::global::world->buildings.other.ZONE) {
    ZoneData* zoneData = update.add_zones();
    zoneData->set_zone_id(zone->id);
    zoneData->set_zone_type(MapZoneType(zone->type));
    zoneData->set_x1(zone->x1);
    zoneData->set_y1(zone->y1);
    zoneData->set_z1(zone->z1);
    zoneData->set_x2(zone->x2);
    zoneData->set_y2(zone->y2);
    zoneData->set_z2(zone->z2);
    zoneData->set_assigned_to(zone->getAssignedDwarfID());
}

SendToOrchestrator(update);
```

---

### 3. Soil Layer Detection (Enhanced RESYNC)

**Protocol Buffer Definition** (extends existing TopologyMessage):

```protobuf
message TopologyMessage {
  // Existing fields: tile types, dimensions, etc.

  // NEW: Soil layer flags per Z-level
  repeated bool is_soil_layer = 10;  // Array of 200 bools (one per Z-level)
}
```

**Plugin Handler**:
```cpp
// In RESYNC handler
TopologyMessage topology;

// Existing topology extraction...

// NEW: Soil layer detection
for (int z = 0; z < 200; z++) {
    bool hasSoil = CheckZLevelForSoil(z);
    topology.add_is_soil_layer(hasSoil);
}

SendToOrchestrator(topology);
```

---

## Configuration Schema

**Config Structure** (config/orchestrator.yaml):

```yaml
# SVP Configuration
use_svp: true
svp_persistence_dir: "./saves"

# Zone Extraction Configuration
extract_zones: true
zone_extraction_interval_ms: 5000

# Blueprint Configuration
blueprint_directory: "./blueprints"
include_blueprint_metadata: true

# Zone Queue Configuration
zone_queue_max_size: 50
zone_queue_max_retries: 3
zone_queue_timeout_cycles: 10
```

**Go Config Struct**:

```go
type Config struct {
    // Existing fields...

    // NEW: SVP Configuration
    UseSVP              bool   `yaml:"use_svp"`
    SVPPersistenceDir   string `yaml:"svp_persistence_dir"`

    // NEW: Zone Extraction Configuration
    ExtractZones             bool `yaml:"extract_zones"`
    ZoneExtractionIntervalMS int  `yaml:"zone_extraction_interval_ms"`

    // NEW: Blueprint Configuration
    BlueprintDirectory         string `yaml:"blueprint_directory"`
    IncludeBlueprintMetadata   bool   `yaml:"include_blueprint_metadata"`

    // NEW: Zone Queue Configuration
    ZoneQueueMaxSize      int `yaml:"zone_queue_max_size"`
    ZoneQueueMaxRetries   int `yaml:"zone_queue_max_retries"`
    ZoneQueueTimeoutCycles int `yaml:"zone_queue_timeout_cycles"`
}
```

---

## Testing Contracts

### Mock Interfaces

For unit testing, provide mock implementations:

**MockSVP**:
```go
type MockSVP struct {
    housingZ  int
    workshopZ int
    farmZ     []int
    ready     bool
}

func (m *MockSVP) GetHousingZ() int { return m.housingZ }
func (m *MockSVP) GetWorkshopZ() int { return m.workshopZ }
func (m *MockSVP) GetFarmZ() []int { return m.farmZ }
func (m *MockSVP) IsReady() bool { return m.ready }
// ... other methods ...
```

**MockZoneExtractor**:
```go
type MockZoneExtractor struct {
    zones []*ZoneInfo
    err   error
}

func (m *MockZoneExtractor) ExtractZones() ([]*ZoneInfo, error) {
    return m.zones, m.err
}
// ... other methods ...
```

---

## Summary

**Internal APIs**: 4 Go interfaces (Planner, Extractor, Queue, MetadataProvider)

**Protocol Extensions**: 3 enhancements (BLUEPRINT command, zone extraction in ENTITY_UPDATE, soil flags in RESYNC)

**No External APIs**: All communication is internal (Go ↔ Go) or plugin protocol (Go ↔ C++ DFHack)

**Testing**: Mock implementations provided for all interfaces to enable unit testing without DF runtime
