# Data Gaps Analysis: What Agents Actually Need

**Date**: 2025-11-09
**Status**: Critical gaps identified, system non-functional without real data

---

## Current State: Placeholder Hell

**In `computeFortMetrics()` (loop.go:743)**:
```go
metrics.FoodPerDwarf = 15.0      // ❌ HARDCODED - FoodAgent never triggers
metrics.DrinkPerDwarf = 15.0     // ❌ HARDCODED
metrics.BedroomCount = 0         // ❌ ALWAYS ZERO - HousingAgent always triggers (wrong)
metrics.MiningTilesPerCycle = 0  // ❌ ALWAYS ZERO - MiningAgent never triggers
metrics.WealthGrowthRate = 0.05  // ❌ HARDCODED - WealthAgent never triggers
```

**Result**:
- FoodAgent thinks food is adequate (15.0 > 15.0 critical threshold) → no proposals
- HousingAgent thinks 0 bedrooms vs 7 dwarves → ALWAYS proposes (spam)
- Other agents never trigger

**System is non-functional without real data extraction.**

---

## Agent-by-Agent Data Requirements

### FoodSecurityAgent - CRITICAL GAPS

**Needs to Know**:
1. ✅ **Dwarf count** - Have this (entity cache)
2. ❌ **Food stock count** - Need from DF world data
3. ❌ **Drink stock count** - Need from DF world data
4. ❌ **Soil locations** - Where can farms be placed?
5. ❌ **Water proximity** - For well planning (future)
6. ❌ **Existing farm zones** - Don't duplicate farms

**Why Soil Matters**:
- Can't place farm on stone (must be soil/clay/loam)
- Current code blindly proposes (40,30,125) - may not be soil!
- Need either:
  - Material type per tile (heavy - 60×60×10 = 36k tiles)
  - OR soil region hints (light - "Z=125 has soil at Y=20-40")

**Extraction Options**:
1. **Add to RESYNC**: soil_regions array [{xmin, ymin, zmin, xmax, ymax, zmax}]
2. **Add material type flag**: is_soil bool in topology_slice
3. **Agent heuristic**: Assume surface Z-level has soil (simple but fragile)

**Recommendation**: Option 1 (soil regions) - lean, sufficient for farm placement

---

### HousingAgent - CRITICAL GAPS

**Needs to Know**:
1. ✅ **Dwarf count** - Have this
2. ❌ **Bedroom zone count** - Need from DF zone list
3. ❌ **Existing dug rooms** - Chambers from modification overlay (have this)
4. ❌ **Good bedroom locations** - Near dining hall, away from danger
5. ❌ **Blueprint access** - 3×3 bedroom, 2×3 bedroom, bedroom cluster patterns

**Critical Issue**: Dug rooms ≠ bedrooms
- Current HousingAgent looks at `BedroomCount = 0` (wrong)
- Should look at DF bedroom zones (assigned to dwarves)
- Chamber count is proxy, but not accurate (chambers might be workshops)

**What DF Provides**:
- Zone list from buildings module
- Zone type (bedroom, dining hall, barracks, etc.)
- Zone assignment (which dwarf owns this bedroom)

**Extraction Needed**:
1. **Add to ENTITY_UPDATE**: zone_count_by_type {bedroom: 5, dining: 1, ...}
2. **Or new message**: ZONE_UPDATE with zone list

**Blueprint Integration**:
- Blueprints already loaded (Feature 005)
- NOT exposed to arbiter yet
- Arbiter system prompt should include available blueprints
- Example: "Available: bedroom_3x3.csv (compact), bedroom_cluster_10bed.csv (nobles)"

**Recommendation**:
- Extract zone counts from DF
- Add blueprints to arbiter context
- HousingAgent uses zone count, arbiter uses blueprints for design

---

### MiningAgent - MODERATE GAPS

**Needs to Know**:
1. ✅ **Current Z-level range** - Can infer from dwarf positions
2. ❌ **Tiles dug per cycle** - Track modification overlay growth
3. ❌ **Ore strike events** - Material type tracking
4. ❌ **Depth progression** - Have we dug stairs down?
5. ❌ **Safe dig zones** - Avoid aquifer, magma, caverns

**Key Insight**: MiningAgent should encourage vertical progression
- Early game: Dig down from surface (Z=140) to main fort level (Z=125-130)
- Propose mining_shaft or stair_cluster when no depth change in N cycles
- Use dwarf Z-level mode to detect if stuck at surface

**Extraction Needed**:
1. **Track modifications per cycle** (orchestrator-side, no plugin change)
2. **Add depth hint to ENTITY_UPDATE**: deepest_z_reached
3. **Material type** (optional, for ore tracking)

**Recommendation**:
- Track modification growth in orchestrator (cycle diff)
- Add deepest_z_reached to fort_info
- MiningAgent proposes stairs down if Z hasn't changed in 5 cycles

---

### WealthAgent - LOW PRIORITY

**Needs to Know**:
1. ❌ **Fort created wealth** - From DF world data
2. ❌ **Workshop count** - From zone list
3. ❌ **Production activity** - Item creation events

**Note**: Wealth is low priority early game (survival first)

**Recommendation**: Defer until Food/Housing/Mining functional

---

### DefenseAgent - WORKS AS-IS

**Needs to Know**:
1. ✅ **Enemy count** - Have this (entity cache)
2. ✅ **Enemy positions** - Have this
3. ❌ **Fort perimeter** - Where are entrances?

**Current Implementation**: Good enough for MVP
- Detects enemies from entity cache
- Proposes seal_entrance and defensive_wall
- Priority escalates correctly (0 → 10)

**Future Enhancement**: Track actual entrance locations from modification overlay

---

## Missing Semantic Layer: Spatial Classifier

**Your Insight**: "designator or classifier that defines entire z levels or portions as zones"

**Exactly right.** Agents need mid-level spatial understanding:

**What We Need**:
```go
type SpatialClassifier struct {
    SoilRegions []Region      // Where farms can go
    WaterSources []Coordinate // For wells
    SafeDigZones []Region     // Avoid hazards
    ResidentialLevel int16    // Main fort Z-level
    WorkshopLevel int16       // Industrial Z-level
}
```

**How to Build It**:
1. **Soil detection**: Check material types, find clay/loam/soil regions
2. **Water sources**: Find aquifer tiles, rivers, lakes
3. **Safe zones**: Inverse of hazard manager (avoid water, lava, caverns)
4. **Z-level classification**: Use modification density + dwarf positions

**Agents use it**:
- FoodAgent: `classifier.GetSoilRegions()` → propose farm in soil
- HousingAgent: `classifier.GetResidentialLevel()` → dig bedrooms at main Z
- MiningAgent: `classifier.GetSafeDigZones()` → avoid hazards

**Token impact**: SpatialClassifier is orchestrator-side (not sent to LLM)
- Agents query it, get 1-2 regions, propose nodes
- Arbiter still just sees proposals

---

## Zone Designation Support - CRITICAL

**Your Point**: "if we want to call something a 'bedroom' it should be a zone called 'bedroom'"

**Absolutely correct.** Current system:
- Agents propose bedroom_cluster
- Executor sends DIG command
- Creates dug room
- ❌ NOT a DF bedroom zone (dwarves can't be assigned)

**What's Needed**:
1. **Add ZONE command to executor**:
   ```go
   case NodeTypeBedroom:
       commands = [
           DIG at region (create room),
           ZONE at region (designate bedroom)
       ]
   ```

2. **DFHack plugin support** (already exists from Feature 005):
   - CommandTypeZone (0x06) defined in protocol
   - Plugin has zone designation stubs
   - Just need to wire it up

3. **Track zone counts**:
   - Extract from DF building list
   - Add to ENTITY_UPDATE: `zone_counts: {bedroom: 5, dining: 1}`

**Implementation**:
- Executor bedroom handler: DIG + ZONE (2 commands per bedroom)
- HousingAgent checks zone count, not chamber count
- Arbiter sees "bedroom zones: 5, dwarves: 7, deficit: 2"

---

## Blueprint Integration - HIGH VALUE

**Current State**:
- Blueprints loaded from blueprints/*.csv
- Passed to context assembler
- ❌ NOT in arbiter prompt (graph mode bypasses context assembly)

**What Arbiter Needs**:
```json
{
  "available_blueprints": [
    {
      "name": "bedroom_3x3",
      "size": "3×3×1",
      "description": "Compact single bedroom",
      "tags": ["bedroom", "compact"]
    },
    {
      "name": "bedroom_cluster_10",
      "size": "20×15×1",
      "description": "10-bedroom noble block with central dining",
      "tags": ["bedroom", "cluster", "nobles"]
    }
  ],
  "proposals": [...]
}
```

**How Arbiter Uses It**:
- HousingAgent proposes: "need 4 bedrooms at (60,30,125)"
- Arbiter sees blueprint: bedroom_cluster_10.csv
- Arbiter decides: "Use bedroom_cluster_10 blueprint at (60,30,125) - covers 10 bedrooms"
- Executor: Load blueprint, convert to dig pattern

**Benefit**: AI learns good fort layouts from human-designed blueprints

---

## Proposed Next Steps

### Option A: Comprehensive Agent Specs (Your Suggestion)

Create detailed spec for each agent:
- **Feature 006a: HousingAgent** (with zone support, blueprints, spatial awareness)
- **Feature 006b: FoodAgent** (with soil detection, water proximity)
- **Feature 006c: MiningAgent** (with depth progression, ore tracking)

Each spec defines:
- Exact data requirements
- DF API extractions needed
- Spatial classifier queries
- Blueprint usage
- Test scenarios

**Pro**: Very thorough, properly planned
**Con**: 3 separate specs, more planning overhead

### Option B: Unified Data Enhancement Spec

Create **Feature 006-Data**: "Agent Data Layer & Spatial Awareness"
- Spec all missing data extractions
- Implement spatial classifier
- Add zone tracking
- Integrate blueprints
- Then enable all agents at once

**Pro**: Holistic, all agents get data together
**Con**: Big bang integration

### Option C: Iterative - Start with HousingAgent

1. Create mini-spec: "HousingAgent with Zone Support & Blueprints"
2. Implement:
   - Zone extraction from DF
   - Zone count in metrics
   - Blueprint integration to arbiter
   - Executor sends DIG + ZONE commands
3. Test just housing (disable other agents)
4. Then add FoodAgent, MiningAgent incrementally

**Pro**: Fast iteration, tangible progress, focused testing
**Con**: Other agents remain non-functional during iteration

---

## My Recommendation: Option C (Iterative, Housing First)

**Why**:
- Housing is most critical early game (unhappy dwarves = tantrum spiral)
- Zone support benefits ALL agents (dining zones, workshop zones, farm zones)
- Blueprints benefit ALL agents (not just housing)
- Once housing works, FoodAgent is simpler (just add soil regions)

**Concrete Plan**:

**Phase 1: Housing Foundation** (~4-6 hours)
- Extract zone list from DF (bedroom zones)
- Add zone_counts to ENTITY_UPDATE or new message
- Update HousingAgent to check bedroom zone count
- Add ZONE command execution to executor
- Test: Verify bedroom zones created, dwarves assigned

**Phase 2: Blueprint Integration** (~2-3 hours)
- Add blueprints to arbiter system prompt
- Arbiter can choose blueprint vs custom design
- Test: Verify arbiter uses bedroom_3x3.csv when appropriate

**Phase 3: Spatial Awareness** (~3-4 hours)
- Create SpatialClassifier (soil regions, safe zones)
- Update FoodAgent to query soil locations
- Add soil region extraction to plugin
- Test: Verify farms proposed in actual soil

**Phase 4: Mining Progression** (~2-3 hours)
- Track depth progression (deepest Z reached)
- MiningAgent proposes stairs down when Z stagnant
- Test: Verify AI digs from surface to main fort level

**Total**: ~12-16 hours for fully functional agent system

---

## Immediate Next Action?

**Option 1**: Create detailed spec for HousingAgent enhancement
- Define exact zone tracking requirements
- Specify blueprint integration
- Write acceptance criteria
- Use `/speckit.specify` to generate proper spec

**Option 2**: Quick prototype - just fix HousingAgent data
- Extract zone counts from DF (add to plugin)
- Update computeFortMetrics() to use real zone count
- Test with one agent working properly

**Option 3**: Create unified "Agent Data Layer" spec
- All missing data sources
- Spatial classifier design
- Blueprint integration
- Zone support

**What's your preference?** I lean toward Option 1 (detailed HousingAgent spec) as a focused pilot that proves the pattern, then we replicate for other agents.