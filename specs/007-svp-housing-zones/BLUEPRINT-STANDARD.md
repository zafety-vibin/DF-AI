# Blueprint Naming Convention & Staircase Connectivity

**Purpose**: Define standard for blueprint naming and staircase tracking to ensure fort connectivity
**Created**: 2025-11-09
**Context**: Community quickfort format uses: `{rooms}-{area}-{Author}_{Description}.csv`

---

## Quickfort Dig Designations

From community blueprints:

| Symbol | Meaning | DF Command | Connectivity |
|--------|---------|------------|--------------|
| `d` | Default dig | Dig (floor/wall) | Horizontal only |
| `u` | Up stair | UpStair | Connects to Z+1 |
| `j` | Down stair | DownStair | Connects to Z-1 |
| `i` | Up/down stair | UpDownStair | Connects Z-1 and Z+1 |
| `h` | Channel | Channel (dig from above) | Vertical |
| `r` | Up ramp | UpRamp | Connects to Z+1 |
| `x` | Remove ramp | Ramp removal | - |
| `#` | Skip/comment | Ignore | - |

**Critical**: `i`, `j`, `u` create vertical connectivity. Without these, levels are isolated!

---

## Proposed Naming Convention

### Format
```
{purpose}_{rooms}r{median-area}t_{width}x{height}_{stairs}s{stair-type}_{descriptor}.csv
```

### Components

**Purpose** (required):
- `bedroom` - Sleeping quarters
- `dining` - Dining halls
- `industry` - Workshops + stockpiles
- `farm` - Farm plots
- `mixed` - Multi-purpose (bedrooms + dining + etc.)

**Rooms** (required):
- `10r` = 10 rooms
- `1r` = single room
- `0r` = corridors/infrastructure only

**Median Area** (required):
- `3t` = 3 tiles median
- `9t` = 9 tiles median (3×3 bedrooms)
- `25t` = 25 tiles median (5×5 rooms)

**Dimensions** (required):
- `18x12` = 18 wide × 12 tall
- `20x20` = square layout

**Stairs** (CRITICAL):
- `1s` = 1 staircase
- `3s` = 3 staircases
- `0s` = no stairs (single-level blueprint)

**Stair Type** (if stairs > 0):
- `center` = stair in center
- `edge` = stair on edge
- `multi` = multiple stair locations
- `spiral` = spiral staircase

**Descriptor** (optional):
- `compact` - Dense/efficient layout
- `noble` - Large rooms for nobles
- `radial` - Radial/circular pattern
- `linear` - Straight corridor layout

### Examples

```
bedroom_10r9t_18x12_1scenter_compact.csv
  → 10 bedrooms, 9-tile median, 18×12, 1 central stair, compact layout

dining_1r25t_10x10_1sedge_hall.csv
  → 1 dining hall, 25 tiles, 10×10, 1 edge stair

mixed_20r6t_25x20_3smulti_cluster.csv
  → 20 rooms, 6-tile median, 25×20, 3 stairs at different locations

industry_0r0t_30x15_1scenter_workshop.csv
  → Workshops only, 30×15, 1 central stair

bedroom_1r9t_3x3_0s_single.csv
  → Single 3×3 bedroom, no stairs (for stacking)
```

---

## Staircase Metadata Extension

### BlueprintMetadata Enhancement

Add to `internal/blueprints/metadata.go`:

```go
type BlueprintMetadata struct {
    Name                string
    DisplayName         string
    Width               int
    Height              int
    Depth               int  // Z-levels spanned (1 for single-level)
    TileCount           int
    DwarfCapacity       int
    Description         string
    Tags                []string
    SuitabilityCriteria map[string]interface{}

    // Staircase tracking (CRITICAL for connectivity)
    StairCount     int                `json:"stair_count"`      // Total stairs in blueprint
    StairLocations []StaircaseAnchor `json:"stair_locations"`  // Relative coordinates of stairs
}

type StaircaseAnchor struct {
    RelativeX    int    `json:"x"`          // X offset from blueprint origin
    RelativeY    int    `json:"y"`          // Y offset from blueprint origin
    RelativeZ    int    `json:"z"`          // Z offset (0 for single-level)
    StairType    string `json:"type"`       // "up", "down", "updown"
    IsEntryPoint bool   `json:"entry"`      // True if this is the main entry stair
}
```

### Example Metadata

```json
{
  "name": "bedroom_10r9t_18x12_1scenter_compact",
  "width": 18,
  "height": 12,
  "stair_count": 1,
  "stair_locations": [
    {
      "x": 9,
      "y": 6,
      "z": 0,
      "type": "updown",
      "entry": true
    }
  ]
}
```

---

## Staircase Connectivity System

### 1. Track Existing Staircases in StrategicLayout

Extend `ZoneLayer`:

```go
type ZoneLayer struct {
    ZLevel             int
    Purpose            string
    Regions            []*SpatialRegion
    ExistingZones      map[string]int

    // Staircase tracking for connectivity (CRITICAL)
    ExistingStairs     []*StaircaseLocation  // All stairs on this Z-level
    ConnectsToZAbove   bool                  // Has up-stairs to Z+1
    ConnectsToZBelow   bool                  // Has down-stairs to Z-1
    OrphanedStairs     []*StaircaseLocation  // Stairs leading nowhere

    Constraints        []string
}

type StaircaseLocation struct {
    AbsoluteX    int    `json:"x"`         // Absolute map coordinates
    AbsoluteY    int    `json:"y"`
    AbsoluteZ    int    `json:"z"`
    StairType    string `json:"type"`      // "up", "down", "updown"
    ConnectedTo  *StaircaseLocation `json:"-"` // Link to stair on adjacent Z
    IsOrphaned   bool   `json:"orphaned"`  // True if no matching stair above/below
}
```

### 2. Arbiter Staircase Anchoring Rules

Update arbiter prompt with connectivity logic:

```
Staircase Connectivity Rules (CRITICAL):
1. ALWAYS check blueprint.stair_locations before placement
2. If blueprint has stairs, you MUST either:
   a) Anchor to existing staircase (align blueprint.stair[0] with existing stair)
   b) Create new vertical shaft (only if no existing stairs on layer)
3. NEVER place blueprints that create orphaned stairs (stairs leading to solid rock)
4. Prefer blueprints with 1 central stair for easy connectivity
5. When placing multiple blueprints on same layer, align their stairs

Anchoring Algorithm:
1. Get existing stairs on target Z-level from fort_layout.layers[purpose].existing_stairs
2. If existing stairs exist:
   - Calculate anchor that aligns blueprint stair with existing stair
   - anchor = existing_stair_pos - blueprint_stair_relative_pos
3. If no existing stairs:
   - Place blueprint freely, its stair becomes the new anchor point
   - Log: "Creating new vertical shaft at (x,y)"
4. Verify: After placement, check fort remains connected (no orphaned levels)
```

### 3. Example Arbiter Decision

**Scenario**: Housing layer (Z=95) has 1 existing stair at (50, 50)

**Agent Proposal**:
```json
{
  "intent": "provide_housing",
  "quantity": 10,
  "blueprint_hint": "bedroom_10r9t_18x12_1scenter_compact"
}
```

**Blueprint Metadata**:
```json
{
  "name": "bedroom_10r9t_18x12_1scenter_compact",
  "width": 18,
  "height": 12,
  "stair_locations": [{"x": 9, "y": 6, "type": "updown", "entry": true}]
}
```

**Arbiter Calculation**:
```
Existing stair: (50, 50, 95)
Blueprint stair offset: (9, 6)

Anchor = Existing - Offset
Anchor = (50, 50) - (9, 6)
Anchor = (41, 44, 95)

Verification:
  Blueprint stair will be at: (41+9, 44+6, 95) = (50, 50, 95) ✓
  Aligns with existing stair ✓
```

**Arbiter Output**:
```json
{
  "type": "apply_blueprint",
  "blueprint": "bedroom_10r9t_18x12_1scenter_compact",
  "anchor": [41, 44, 95],
  "reasoning": "Anchored to existing stair at (50,50,95). Blueprint stair aligns perfectly. Provides 10 bedrooms."
}
```

---

## Zone CSV Companion Format

### Simplified Zone Format (For Our System)

Since community blueprints don't have zone companions, we create them:

**File**: `bedroom_10r9t_18x12_1scenter_compact_zones.csv`

```csv
x,y,z,zone_type,width,height,is_stair
0,0,0,bedroom,3,3,false
4,0,0,bedroom,3,3,false
8,0,0,bedroom,3,3,false
9,6,0,staircase,1,1,true
12,0,0,bedroom,3,3,false
...
```

**New Field**: `is_stair` (boolean)
- Marks which zone is the staircase
- Enables staircase extraction during blueprint placement

---

## Staircase Scanning from Quickfort CSV

### Parse Stairs During LoadFromCSV

Update `internal/blueprints/loader.go` (or create):

```go
// ParseStaircases scans blueprint CSV for stair designations
func ParseStaircases(path string) ([]StaircaseAnchor, error) {
    stairs := make([]StaircaseAnchor, 0)

    // Read CSV file
    data, _ := os.ReadFile(path)
    lines := strings.Split(string(data), "\n")

    // Skip header line (#dig ...)
    startRow := 1
    if strings.HasPrefix(lines[0], "#") {
        startRow = 1
    }

    // Scan each row
    for y, line := range lines[startRow:] {
        cells := strings.Split(line, ",")

        for x, cell := range cells {
            cell = strings.TrimSpace(cell)

            switch cell {
            case "j":  // Down stair
                stairs = append(stairs, StaircaseAnchor{
                    RelativeX: x,
                    RelativeY: y,
                    RelativeZ: 0,
                    StairType: "down",
                    IsEntryPoint: false,
                })
            case "u":  // Up stair
                stairs = append(stairs, StaircaseAnchor{
                    RelativeX: x,
                    RelativeY: y,
                    RelativeZ: 0,
                    StairType: "up",
                    IsEntryPoint: false,
                })
            case "i":  // Up/down stair
                stairs = append(stairs, StaircaseAnchor{
                    RelativeX: x,
                    RelativeY: y,
                    RelativeZ: 0,
                    StairType: "updown",
                    IsEntryPoint: true,  // Assume first i is main entry
                })
            }
        }
    }

    return stairs, nil
}
```

### Update parseBlueprint to Extract Stairs

```go
func parseBlueprint(path string) (*BlueprintMetadata, error) {
    bp, _ := LoadFromCSV(path)

    // Extract stairs from CSV
    stairs, _ := ParseStaircases(path)

    return &BlueprintMetadata{
        Name:           extractName(path),
        Width:          int(bp.Width),
        Height:         int(bp.Height),
        StairCount:     len(stairs),
        StairLocations: stairs,
        // ... other fields
    }, nil
}
```

---

## Orphaned Staircase Detection

### Algorithm

```go
// DetectOrphanedStairs finds stairs that lead nowhere (R-addition)
func (svp *SpatialValidatorPlanner) DetectOrphanedStairs(
    layout *StrategicLayout,
    topologyOverlay *topology.TopologyOverlay,
) {
    for _, layer := range layout.Layers {
        orphaned := make([]*StaircaseLocation, 0)

        for _, stair := range layer.ExistingStairs {
            // Check if stair has matching counterpart on adjacent Z
            hasConnection := false

            if stair.StairType == "up" || stair.StairType == "updown" {
                // Check Z+1 for down/updown stair at same (x,y)
                if aboveLayer := layout.Layers[getPurposeForZ(stair.AbsoluteZ+1)]; aboveLayer != nil {
                    for _, aboveStair := range aboveLayer.ExistingStairs {
                        if aboveStair.AbsoluteX == stair.AbsoluteX &&
                           aboveStair.AbsoluteY == stair.AbsoluteY &&
                           (aboveStair.StairType == "down" || aboveStair.StairType == "updown") {
                            hasConnection = true
                            stair.ConnectedTo = aboveStair
                            break
                        }
                    }
                }
            }

            if stair.StairType == "down" || stair.StairType == "updown" {
                // Check Z-1 for up/updown stair at same (x,y)
                if belowLayer := layout.Layers[getPurposeForZ(stair.AbsoluteZ-1)]; belowLayer != nil {
                    for _, belowStair := range belowLayer.ExistingStairs {
                        if belowStair.AbsoluteX == stair.AbsoluteX &&
                           belowStair.AbsoluteY == stair.AbsoluteY &&
                           (belowStair.StairType == "up" || belowStair.StairType == "updown") {
                            hasConnection = true
                            stair.ConnectedTo = belowStair
                            break
                        }
                    }
                }
            }

            if !hasConnection {
                stair.IsOrphaned = true
                orphaned = append(orphaned, stair)
            }
        }

        layer.OrphanedStairs = orphaned

        if len(orphaned) > 0 {
            svp.logger.Warn("Orphaned stairs detected",
                logging.Field{Key: "z", Value: layer.ZLevel},
                logging.Field{Key: "count", Value: len(orphaned)})
        }
    }
}
```

---

## Recommended Naming Conversion

### Community Blueprints → Our Standard

| Community Name | Our Standard | Notes |
|----------------|--------------|-------|
| `100-4-Vherid.csv` | `bedroom_100r4t_50x50_1scenter_vherid.csv` | Count stairs, measure dims |
| `108-6-Raynard1_dig.csv` | `bedroom_108r6t_36x36_1scenter_raynard.csv` | Extract from CSV |
| `Simple Spiral Staircase.csv` | `stair_0r0t_3x3_1sspiral_simple.csv` | Pure staircase |

### For Your Custom Blueprints

When you add/rename blueprints, use:
```
bedroom_10r9t_18x12_1scenter_compact.csv
bedroom_3r9t_9x9_1sedge_trio.csv
dining_1r36t_6x6_1scenter_hall.csv
industry_0r0t_20x15_1scenter_workshops.csv
```

### Parsing Convention from Filename

```go
// ParseBlueprintName extracts metadata from filename
func ParseBlueprintName(filename string) (*BlueprintMetadata, error) {
    // bedroom_10r9t_18x12_1scenter_compact.csv
    name := strings.TrimSuffix(filename, ".csv")
    parts := strings.Split(name, "_")

    if len(parts) < 4 {
        return nil, fmt.Errorf("invalid name format")
    }

    meta := &BlueprintMetadata{Name: name}

    // Part 0: Purpose
    meta.Tags = append(meta.Tags, parts[0])

    // Part 1: Rooms + Area (e.g., "10r9t")
    if matches := regexp.MustCompile(`(\d+)r(\d+)t`).FindStringSubmatch(parts[1]); len(matches) == 3 {
        meta.DwarfCapacity, _ = strconv.Atoi(matches[1])  // Room count
        // median area in matches[2]
    }

    // Part 2: Dimensions (e.g., "18x12")
    if matches := regexp.MustCompile(`(\d+)x(\d+)`).FindStringSubmatch(parts[2]); len(matches) == 3 {
        meta.Width, _ = strconv.Atoi(matches[1])
        meta.Height, _ = strconv.Atoi(matches[2])
    }

    // Part 3: Stairs (e.g., "1scenter")
    if matches := regexp.MustCompile(`(\d+)s(\w+)`).FindStringSubmatch(parts[3]); len(matches) == 3 {
        meta.StairCount, _ = strconv.Atoi(matches[1])
        meta.Description = "Stair type: " + matches[2]
    }

    // Part 4+: Descriptors
    if len(parts) > 4 {
        meta.Tags = append(meta.Tags, parts[4:]...)
    }

    return meta, nil
}
```

---

## Arbiter Staircase Anchoring Prompt

### Enhanced Arbiter System Prompt

Add to `getIntentArbiterSystemPrompt()`:

```
CRITICAL: Staircase Connectivity

Staircases connect Z-levels vertically. Without proper anchoring, levels become isolated!

Staircase Anchoring Algorithm:
1. Check fort_layout.layers[purpose].existing_stairs for current staircase positions
2. If existing stairs found on target layer:
   - Calculate anchor that ALIGNS blueprint stair with existing stair
   - anchor_x = existing_stair.x - blueprint.stair_locations[0].x
   - anchor_y = existing_stair.y - blueprint.stair_locations[0].y
   - This ensures stairs stack vertically (perfect alignment)
3. If no existing stairs on layer:
   - Place blueprint freely (its stair becomes the vertical shaft anchor)
   - NOTE: All future blueprints on adjacent Z-levels MUST align to this stair
4. Verify connectivity:
   - If blueprint has up_stair (type="up" or "updown"), Z+1 must have down_stair at same (x,y)
   - If blueprint has down_stair (type="down" or "updown"), Z-1 must have up_stair at same (x,y)
   - If mismatch, DEFER the proposal or adjust placement

Priority Rules:
- Connectivity > Optimal placement
- If aligning to stair forces blueprint outside region bounds, DEFER proposal
- If no stair alignment possible, use blueprint with 0s (no stairs) and connect manually later

Example Decision:
  Existing stair: (50, 50, 95) type="updown"
  Proposal: housing blueprint "bedroom_10r9t_18x12_1scenter_compact"
  Blueprint stair offset: (9, 6)

  Calculated anchor: (50-9, 50-6, 95) = (41, 44, 95)

  Output:
  {
    "type": "apply_blueprint",
    "blueprint": "bedroom_10r9t_18x12_1scenter_compact",
    "anchor": [41, 44, 95],
    "reasoning": "Anchored to existing staircase at (50,50,95). Stair alignment ensures vertical connectivity."
  }
```

---

## Implementation Tasks

### Phase 1: Blueprint Metadata Enhancement

1. **Add StaircaseAnchor type** to `internal/blueprints/metadata.go`
2. **Add StairCount, StairLocations fields** to BlueprintMetadata
3. **Implement ParseStaircases()** to scan CSV for 'i', 'j', 'u' designations
4. **Update parseBlueprint()** to extract stair metadata
5. **Update ToPromptString()** to include stair info:
   ```
   "bedroom_10r9t (18×12, 10 rooms, 1 stair at center)"
   ```

### Phase 2: StrategicLayout Staircase Tracking

1. **Add StaircaseLocation type** to `internal/spatial/layout.go`
2. **Add ExistingStairs, ConnectsToZAbove/Below, OrphanedStairs** to ZoneLayer
3. **Implement DetectOrphanedStairs()** in SVP
4. **Update analyzeLayer()** to populate ExistingStairs from zone data
   - Zones with type "staircase" (from zone CSV)
   - Or extract from topology (tiles with stair designation)

### Phase 3: Arbiter Prompt Update

1. **Update getIntentArbiterSystemPrompt()** with staircase anchoring rules
2. **Add stair alignment example** to prompt
3. **Add orphaned stair warning** to prompt

### Phase 4: Zone CSV Creation Helper

Create tool to generate zone CSVs with stair marking:

```go
// GenerateZoneCSV creates zone companion from blueprint CSV
func GenerateZoneCSV(blueprintPath string) error {
    // 1. Parse blueprint CSV
    // 2. Detect room boundaries (contiguous 'd' regions)
    // 3. Detect stairs ('i', 'j', 'u')
    // 4. Generate zones CSV with is_stair column
    // 5. Save as {blueprint}_zones.csv
}
```

---

## Migration Plan

### Step 1: Your Manual Work
1. Select best community blueprints (10-20 to start)
2. Rename using our convention: `{purpose}_{rooms}r{area}t_{WxH}_{stairs}s{type}_{desc}.csv`
3. Copy to `blueprints/` directory

### Step 2: Automated Metadata Generation
1. Script scans all blueprints in `blueprints/`
2. Parses CSV to detect stairs (scan for 'i', 'j', 'u')
3. Generates BlueprintMetadata with StairLocations
4. Generates zone CSV companions with is_stair column

### Step 3: SVP Staircase Tracking
1. Detect existing stairs from zone extraction
2. Populate layer.ExistingStairs
3. Run DetectOrphanedStairs() after each cycle
4. Warn if orphaned stairs found

### Step 4: Arbiter Enforcement
1. Arbiter receives fort_layout with existing_stairs
2. Arbiter calculates anchor to align blueprint stairs with existing
3. Validates connectivity before placement
4. Defers proposals if connectivity impossible

---

## Example Complete Workflow

**Cycle 1**: Housing layer (Z=95) empty
```
Agent: "provide_housing, quantity=10"
SVP:   "housing layer Z=95, region: 32400 tiles, existing_stairs: []"
Arbiter: "No existing stairs. Place bedroom_10r9t freely at (40,30,95)"
         "Blueprint stair at (49,36,95) becomes vertical shaft anchor"
```

**Cycle 2**: Workshop layer (Z=94) empty, housing has stair at (49,36,95)
```
Agent: "provide_workshops, quantity=5"
SVP:   "workshop layer Z=94, region: 32400 tiles, existing_stairs: []"
SVP:   "housing layer above has stair at (49,36,95) type=updown"
Arbiter: "Must align workshop blueprint stair to (49,36,94) to connect"
         "Using industry_5w_20x15_1scenter"
         "Blueprint stair offset: (10,7)"
         "Anchor: (49-10, 36-7, 94) = (39, 29, 94)"
         "Stair alignment: (39+10, 29+7, 94) = (49,36,94) ✓"
```

**Result**: Housing (Z=95) stair at (49,36,95) connects to Workshop (Z=94) stair at (49,36,94). Fort has continuous vertical shaft!

---

## Fallback: No-Stair Blueprints

For situations where stair alignment impossible:
- Use blueprints with `0s` (no stairs)
- Place on layer
- Manually create staircase blueprint later
- Or let mining/stair agent propose dedicated staircase

Example:
```
bedroom_10r9t_18x12_0s_compact.csv  → No stairs, just bedrooms
stair_0r0t_3x3_1scenter_shaft.csv   → Pure staircase (3×3 up/down shaft)
```

Arbiter can then:
1. Place bedroom blueprint (no stairs)
2. Place staircase blueprint at convenient location
3. Ensures connectivity

---

## Next Steps

**Immediate**:
1. Design helper script to scan community blueprints and extract stair metadata
2. Implement BlueprintMetadata staircase fields
3. Implement ParseStaircases() function
4. Update arbiter prompt with staircase anchoring rules

**After You Select Blueprints**:
1. Run metadata generator on your curated blueprint set
2. Generate zone CSV companions
3. Test arbiter staircase anchoring with mock data

**Plugin Work** (Phase 6):
1. BLUEPRINT command handler reads CSV
2. Parses 'd', 'i', 'j', 'u' designations
3. Creates dig commands
4. Creates zones from companion CSV

Want me to implement the metadata enhancement with staircase tracking first?
