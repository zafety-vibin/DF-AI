# HRM Architecture: Intent-Based Planning

**Inspired by**: [Hierarchical Reasoning Model (arxiv.org/abs/2506.21734v3)](https://arxiv.org/abs/2506.21734v3)
**Implementation**: Feature 007 Refactor (R001-R015)
**Status**: Active (toggle with `use_intent_planning` config flag)

---

## Overview

The HRM (Hierarchical Reasoning Model) architecture separates **strategic planning** (H-module) from **tactical execution** (L-module), enabling more intelligent spatial reasoning with less data.

### Key Innovation: Intent-First, Not Coordinates-First

**Before (Coordinate-Based)**:
```
HousingAgent → ModificationNode{Region: (60,30,125)} → Arbiter → Executor
              ↑ hardcoded coordinates
```

**After (Intent-Based)**:
```
HousingAgent → IntentProposal{Quantity: 7, Purpose: "housing"}
                                      ↓
SVP → StrategicLayout{Layers: housing, workshop, farm}
                                      ↓
                            Arbiter (H-module)
                                      ↓
              ArbiterCommand{Blueprint: "cluster_10", Anchor: (42,31,95)}
                                      ↓
                                  Executor
```

---

## Architecture Components

### 1. **Agents (L-Module)** - Fast, Focused Analysis

Agents analyze fort metrics and propose **high-level needs** without spatial reasoning.

**Example: HousingAgent.AnalyzeIntent()**
```go
func (a *HousingAgent) AnalyzeIntent(metrics *FortMetrics) []IntentProposal {
    deficit := metrics.DwarfCount - metrics.BedroomZoneCount

    return []IntentProposal{{
        Agent:         "Housing",
        Intent:        "provide_housing",  // WHAT is needed
        Purpose:       "housing",           // WHERE type (not coordinates!)
        Quantity:      deficit,             // HOW MANY
        Constraints:   []string{"safe_layer", "avoid_aquifer"},
        BlueprintHint: "bedroom_cluster_10",
        Rationale:     "7 dwarves, 0 bedrooms, 7 deficit",
    }}
}
```

**Key Change**: Agent doesn't decide coordinates. Just states the need.

---

### 2. **SVP (L-Module)** - Fast Spatial Analysis

SVP analyzes terrain and generates **StrategicLayout** with available regions and infrastructure counts.

**Output: StrategicLayout JSON**
```json
{
  "fort_name": "Mountainhomes",
  "layers": {
    "housing": {
      "z_level": 95,
      "purpose": "housing",
      "regions": [
        {
          "id": "housing_1",
          "bbox": [10, 10, 95, 190, 190, 95],
          "status": "available",
          "area": 32400,
          "features": ["open_space"]
        }
      ],
      "existing_zones": {
        "bedroom": 5,
        "dining": 1
      },
      "constraints": []
    },
    "workshop": {
      "z_level": 94,
      "regions": [...],
      "existing_zones": {}
    }
  }
}
```

**What SVP Provides**:
- Available construction regions per Z-level (bboxes + area)
- Existing infrastructure counts (bedroom zones, workshops, stockpiles)
- Constraints (hazards, soil requirements)

**When Updated**:
- Analysis: Once at startup (or fort load)
- Zone counts: Every cycle via `UpdateStrategicLayoutWithZones()`

---

### 3. **Arbiter (H-Module)** - Slow, Strategic Planning

Arbiter receives **StrategicLayout + IntentProposals + Blueprints** and performs spatial reasoning.

**Arbiter Input**:
```json
{
  "fort_layout": { /* StrategicLayout from SVP */ },
  "proposals": [
    {
      "agent": "Housing",
      "intent": "provide_housing",
      "quantity": 7,
      "blueprint_hint": "bedroom_cluster_10"
    }
  ],
  "blueprints": [
    {"name": "bedroom_3x3", "width": 3, "height": 3, "capacity": 1},
    {"name": "bedroom_cluster_10", "width": 18, "height": 12, "capacity": 10}
  ]
}
```

**Arbiter Output**:
```json
{
  "commands": [
    {
      "type": "apply_blueprint",
      "blueprint": "bedroom_cluster_10",
      "anchor": [42, 31, 95],
      "rotation": 0,
      "reasoning": "Housing layer has 32400 tiles in housing_1. Cluster needs 216 tiles. Placed at anchor with margin."
    }
  ],
  "deferred": []
}
```

**Arbiter Decision Rules**:
1. Check `region.area >= (blueprint.width * blueprint.height)` before placement
2. Respect layer purposes (housing blueprints on housing layers only)
3. Avoid overlapping `existing_zones`
4. Use `blueprint_hint` when provided
5. Prefer blueprints over geometric patterns
6. Defer proposals if blueprint doesn't fit

---

## Data Flow

```
┌─────────────┐
│   Reality   │ (DF game state)
└──────┬──────┘
       │
       ├─→ Topology → SVP.AnalyzeTerrain() → StrategicLayout
       │                                       ↓
       └─→ Zones ────→ UpdateStrategicLayoutWithZones()
                                              │
┌─────────────────────────────────────────────┴─────────────┐
│ StrategicLayout (Available Regions + Infrastructure)      │
└────────────────────────────┬───────────────────────────────┘
                             │
┌─────────────┐              │              ┌──────────────┐
│   Agents    │ → Intents ───┼─→ Arbiter ←──┤  Blueprints  │
└─────────────┘              │   (H-mod)    └──────────────┘
                             ↓
                    ArbiterCommands
                  (Blueprint + Anchor)
                             ↓
                         Executor
                             ↓
                    Protocol BLUEPRINT
```

---

## Configuration

### Enable HRM Architecture

In `config/orchestrator.yaml`:

```yaml
# Enable HRM architecture (intent-based planning)
use_intent_planning: true

# Prerequisites
enable_goal_agents: true           # Must enable agents
use_svp: true                      # Must enable SVP
extract_zones: true                # Must enable zone extraction
include_blueprint_metadata: true   # Must enable blueprints
```

### Disable (Legacy Mode)

```yaml
use_intent_planning: false  # Use coordinate-based planning
```

---

## Implementation Status

### ✅ Completed (R001-R011)

**Types**:
- `StrategicLayout`, `ZoneLayer`, `SpatialRegion` (`internal/spatial/layout.go`)
- `IntentProposal`, `ArbiterCommand`, `ArbiterIntentResponse` (`internal/agents/intent.go`)

**SVP**:
- `GetStrategicLayout()` accessor
- `buildStrategicLayout()` layer constructor
- `analyzeLayer()` per-layer analysis with region scanning
- `findAvailableRegions()` bbox generation (placeholder - needs flood-fill)
- `UpdateStrategicLayoutWithZones()` zone count updates
- Integrated with `AnalyzeTerrain()` - builds layout at end of analysis

**ZoneExtractor**:
- `GetZonesByZLevel()` groups zones by Z-level and type
- `TypeString()` converts ZoneType to string for JSON

**HousingAgent**:
- `AnalyzeIntent()` intent-based analysis
- Backward compatible (`Analyze()` still exists for legacy mode)

**AutonomousLoop**:
- `collectIntentProposals()` gathers intents from agents
- `buildArbiterIntentInput()` formats JSON for arbiter
- `getIntentArbiterSystemPrompt()` specialized prompt
- `parseArbiterIntentResponse()` parses arbiter JSON
- `convertArbiterCommandsToProtocol()` maps to BLUEPRINT commands
- `SetUseIntentPlanning()` mode toggle
- Branching logic in `runCycle()` (line 236)

**Config**:
- `UseIntentPlanning` flag added to `config.Config`
- `use_intent_planning` in `orchestrator.yaml`

### ⚠️ Remaining (R012-R015)

**R012-R014: Testing**
- Create test scenario with `use_intent_planning: true`
- Verify StrategicLayout JSON generation
- Verify intent proposals from HousingAgent
- Verify arbiter blueprint selection
- Compare intent vs coordinate flow outputs

**R015: Deprecation** (Future)
- Remove coordinate-based flow after validation
- Remove `Analyze()` methods (keep only `AnalyzeIntent()`)
- Remove `ModificationNode` coordinate proposals
- Remove `ProposalGraph` assembly

---

## Advantages of HRM Architecture

### 1. **Agent Simplification**
Agents no longer need spatial reasoning:
```go
// BEFORE: Agent must hardcode coordinates
Region: modifications.Region{XMin: 60, YMin: 30, ZMin: 125, ...}

// AFTER: Agent just states need
Quantity: 7, Purpose: "housing"
```

### 2. **Arbiter Intelligence**
Arbiter makes informed spatial decisions:
- Knows available regions per layer (from StrategicLayout)
- Knows existing infrastructure (bedroom count: 5)
- Can validate blueprint fits (region.area >= blueprint.area)
- Can reason about placement (margins, overlaps)

### 3. **Data Efficiency** (HRM Paper Insight)
With structured inputs (StrategicLayout + Blueprints), arbiter can learn spatial planning from few examples:
- HRM paper: 27M params solve Sudoku with 1000 examples
- DF-AI: Qwen2.5-7B learns blueprint placement with StrategicLayout structure

### 4. **Future: Trainable Arbiter**
Once we collect enough (Intent, StrategicLayout) → Blueprint+Anchor examples:
- Fine-tune local LLM on placement task
- Or train small HRM model (27M params) specifically for fort planning
- Remove dependency on large LLMs for spatial reasoning

---

## Comparison: Intent vs Coordinate Flow

| Aspect | Coordinate-Based (Legacy) | Intent-Based (HRM) |
|--------|---------------------------|-------------------|
| **Agent Output** | `ModificationNode{Region: (x,y,z)}` | `IntentProposal{Quantity: 7}` |
| **Spatial Reasoning** | In agents (hardcoded) | In arbiter (dynamic) |
| **Arbiter Input** | ProposalGraph (nodes + edges) | StrategicLayout + Intents |
| **Arbiter Output** | Sorted node sequence | Blueprint + Anchor commands |
| **Executor** | GraphExecutor converts nodes | Direct BLUEPRINT commands |
| **Blueprint Selection** | GraphExecutor checks metadata | Arbiter selects with reasoning |
| **Infrastructure Awareness** | None (agents don't know zones) | Full (arbiter sees existing zones) |
| **Region Fitting** | Not validated | Validated (area check) |

---

## Future Enhancements

### Region Scanner (R006 Full Implementation)
Replace placeholder `findAvailableRegions()` with flood-fill:
```go
// 1. Scan all tiles at Z-level from TopologyOverlay
// 2. Connected-components algorithm (flood-fill)
// 3. Generate bbox for each contiguous region
// 4. Filter: area >= 9 tiles, no hazards
```

### Multi-Agent Intent Support
Add `AnalyzeIntent()` to other agents:
- `FoodAgent`: Intent "secure_food", Purpose "farm", Quantity (farm plots)
- `MiningAgent`: Intent "extract_resources", Purpose "mining", Quantity (shafts)
- `DefenseAgent`: Intent "fortify_entrance", Purpose "defense", Constraints ["near_entrance"]

### Arbiter Training Data Collection
Log (Intent, StrategicLayout, BlueprintPlacement) tuples:
```json
{
  "input": {"layout": {...}, "intent": {...}},
  "output": {"blueprint": "cluster_10", "anchor": [42,31,95]},
  "outcome": "success"  // from post-execution validation
}
```

Collect 1000+ examples → fine-tune Qwen2.5-7B or train HRM model.

---

## How to Test

### Enable Intent Planning
1. Edit `config/orchestrator.yaml`:
   ```yaml
   enable_goal_agents: true
   use_intent_planning: true
   use_svp: true
   extract_zones: true
   include_blueprint_metadata: true
   ```

2. Start orchestrator:
   ```bash
   ./bin/df-orchestrator.exe
   ```

3. Watch logs for:
   ```
   INFO SVP: Terrain analysis complete layout_layers=3
   INFO intent-based planning enabled (HRM architecture)
   DEBUG SVP: Analyzed layer z=95 purpose=housing region_count=1
   INFO arbiter intent response received
   INFO arbiter blueprint command blueprint=bedroom_cluster_10 anchor=[42,31,95]
   ```

### Compare with Legacy Mode
1. Set `use_intent_planning: false`
2. Observe coordinate-based flow:
   ```
   DEBUG agent proposal type=bedroom_cluster region=(60,30,125)-(70,40,125)
   INFO arbiter decision parsed execution_count=1
   ```

---

## Performance Targets

From HRM paper insights:
- **SVP Analysis**: <200ms (L-module, fast spatial scan)
- **Intent Collection**: <10ms (L-module, simple deficit calculation)
- **Arbiter Planning**: <500ms (H-module, strategic blueprint placement)
- **Total Cycle**: <1000ms (within 5s cycle budget)

---

## Key Files

### Core Types
- `internal/spatial/layout.go` - StrategicLayout, ZoneLayer, SpatialRegion (lines 61-119)
- `internal/agents/intent.go` - IntentProposal, ArbiterCommand, ArbiterIntentResponse

### Implementation
- `internal/spatial/planner.go` - SVP StrategicLayout generation (lines 376-568)
- `internal/zones/extractor.go` - GetZonesByZLevel() (lines 114-155)
- `internal/agents/housing.go` - AnalyzeIntent() (lines 101-156)
- `internal/autonomous/loop.go` - Intent flow integration (lines 236-428, 1256-1383)

### Configuration
- `config/orchestrator.yaml` - use_intent_planning flag (line 99)
- `internal/config/config.go` - UseIntentPlanning field (line 96)

---

## Migration Path

### Phase 1: Coexistence (Current)
- Both flows implemented
- Toggle with config flag
- Test parity between modes

### Phase 2: Validation
- Run A/B tests (intent vs coordinate)
- Verify blueprint placement quality
- Measure arbiter decision latency

### Phase 3: Deprecation
- Set `use_intent_planning: true` as default
- Remove coordinate-based flow
- Clean up legacy code (R015)

---

## Why This Matters

### From HRM Paper
> "With only 27 million parameters, HRM achieves exceptional performance on complex reasoning tasks using only 1000 training samples."

**Applied to DF-AI**:
- **Structured inputs** (StrategicLayout, Blueprints) act as "curriculum"
- **Hierarchical processing** (agents → SVP → arbiter) enables deep reasoning
- **Small local LLM** (Qwen2.5-7B) can solve spatial planning with structure
- **Few examples needed** to fine-tune arbiter on blueprint placement

### Long-Term Vision
1. Collect 1000+ placement examples from human/arbiter decisions
2. Fine-tune Qwen2.5-7B on (StrategicLayout, Intent) → (Blueprint, Anchor) task
3. Or train small HRM model (27M params) specifically for fort spatial planning
4. Achieve near-perfect blueprint placement without large LLMs
5. Run entirely locally, efficiently

---

## Current Limitations

### 1. **Placeholder Region Scanner**
`findAvailableRegions()` creates one large region per layer.

**Fix**: Implement flood-fill to find actual contiguous dug areas.

### 2. **Single Agent Support**
Only `HousingAgent` has `AnalyzeIntent()`.

**Fix**: Add intent methods to FoodAgent, MiningAgent, etc.

### 3. **No Arbiter Training**
Currently using Qwen2.5-7B zero-shot (no fine-tuning).

**Fix**: Collect training data, fine-tune on placement task.

---

## Conclusion

The HRM architecture transforms DF-AI from a **coordinate-based** system (agents hardcode regions) to an **intent-based** system (agents state needs, arbiter reasons spatially).

This enables:
- **Simpler agents**: No spatial logic required
- **Smarter arbiter**: Informed decisions with StrategicLayout context
- **Better placements**: Validates fit, checks existing zones, respects constraints
- **Future trainability**: Structured data for fine-tuning

Set `use_intent_planning: true` to enable.
