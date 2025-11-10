# Quickstart: Spatial Validator Planner and Housing Zones

**Feature**: 007-svp-housing-zones
**Date**: 2025-11-09
**Audience**: Developers and testers

## Overview

This quickstart guide helps you set up, test, and verify the SVP and housing zones feature. Follow these steps to enable strategic fort planning and real zone data extraction.

---

## Prerequisites

Before starting, ensure you have:

- ✅ Feature 006 (graph-based agents) implemented and working
- ✅ Go 1.21+ installed
- ✅ DFHack with DF-AI plugin compiled
- ✅ Dwarf Fortress with active fort or new embark
- ✅ LM Studio running with local LLM (for arbiter)

**Required from Feature 006**:
- Autonomous loop with agent registry
- Graph-based proposal system
- Local LLM provider
- HousingAgent (will be enhanced)
- GraphExecutor (will be enhanced)

---

## Installation Steps

### 1. Update Configuration

Edit `config/orchestrator.yaml` to enable SVP and zone extraction:

```yaml
# Enable goal agents (Feature 006)
enable_goal_agents: true

# NEW: Enable SVP (Feature 007)
use_svp: true
svp_persistence_dir: "./saves"

# NEW: Enable zone extraction (Feature 007)
extract_zones: true
zone_extraction_interval_ms: 5000  # Extract zones every 5 seconds

# NEW: Blueprint library (Feature 007)
blueprint_directory: "./blueprints"
include_blueprint_metadata: true  # Add metadata to arbiter prompt

# Agent configuration (existing)
agent_housing:
  enabled: true
  priority: 9
  targets:
    bedrooms_per_dwarf: 1
```

### 2. Create Blueprint Zone Companions

For each existing blueprint, create a zone CSV companion:

**Example: blueprints/bedroom_3x3_zones.csv**
```csv
x,y,z,zone_type,width,height
0,0,0,bedroom,3,3
```

**Example: blueprints/bedroom_cluster_10_zones.csv**
```csv
x,y,z,zone_type,width,height
0,0,0,bedroom,3,3
4,0,0,bedroom,3,3
8,0,0,bedroom,3,3
0,4,0,bedroom,3,3
4,4,0,bedroom,3,3
8,4,0,bedroom,3,3
0,8,0,bedroom,3,3
4,8,0,bedroom,3,3
8,8,0,bedroom,3,3
12,4,0,dining,6,6
```

### 3. Build and Run

```bash
# Build orchestrator
cd C:\Users\zmanl\projects\df-ai
go build -o bin/df-orchestrator.exe cmd/df-orchestrator/main.go

# Build DFHack plugin (with new BLUEPRINT command and zone extraction)
cd plugin
# ... follow DFHack build instructions ...

# Run orchestrator
./bin/df-orchestrator.exe
```

---

## Testing SVP (Spatial Validator Planner)

### Test 1: Initial Terrain Analysis

**Goal**: Verify SVP analyzes embark terrain and designates Z-levels.

**Steps**:
1. Start new DF fort embark (embark at Z=125 recommended)
2. Unpause game, let 7 dwarves spawn
3. Run orchestrator, connect with `ai-connect` command in DF
4. Watch orchestrator logs for SVP analysis

**Expected Logs**:
```
[INFO] Topology received (RESYNC), waiting for entity update...
[INFO] Entities received (ENTITY_UPDATE), triggering SVP analysis...
[DEBUG] SVP: Detected embark Z=125 from 7 dwarf positions
[DEBUG] SVP: Detected soil at Z=124, Z=125 (is_soil_layer=true)
[DEBUG] SVP: Hazard manager reports aquifer at Z=115
[INFO] SVP: Designations complete - Housing: Z=120, Workshop: Z=119, Farm: [124, 125]
[INFO] SVP: Saved layout to saves/Doomfortress/svp_layout.json
```

**Verification**:
- Check `saves/Doomfortress/svp_layout.json` exists
- Verify embark_z, housing_z, workshop_z values are correct
- Confirm farm_z includes soil layers only

---

### Test 2: SVP Persistence Across Restarts

**Goal**: Verify SVP loads saved layout on reconnection.

**Steps**:
1. With fort still running, stop orchestrator (Ctrl+C)
2. Restart orchestrator
3. Reconnect with `ai-connect` in DF
4. Watch logs for SVP load

**Expected Logs**:
```
[INFO] Topology received (RESYNC), waiting for entity update...
[INFO] Entities received (ENTITY_UPDATE), checking for existing SVP layout...
[INFO] SVP: Loaded existing layout for Doomfortress from disk
[DEBUG] SVP: Housing Z=120, Workshop Z=119, Farm Z=[124, 125]
[INFO] SVP: Ready (loaded from persistence)
```

**Verification**:
- No "analyzing terrain" logs (analysis skipped, loaded from file)
- Same Z-level designations as before restart
- Fort metrics include SVP values immediately

---

## Testing Zone Extraction

### Test 3: Zone Count Extraction

**Goal**: Verify orchestrator extracts real bedroom zone counts from DF.

**Steps**:
1. In DF, manually create 3 bedroom zones (z menu → bedroom)
2. Wait 5-10 seconds for entity update
3. Check orchestrator logs for zone extraction

**Expected Logs**:
```
[DEBUG] ZoneExtractor: Querying DF building module for zones...
[INFO] ZoneExtractor: Extracted 3 zones (3 bedrooms, 0 dining, 0 dormitories)
[DEBUG] Fort metrics: DwarfCount=7, BedroomZoneCount=3, HousingDeficit=4
```

**Verification**:
- BedroomZoneCount matches DF zone count (z menu shows 3)
- HousingDeficit calculated correctly (7 dwarves - 3 bedrooms = 4 deficit)
- No "fallback to chamber count" warnings

---

### Test 4: HousingAgent Triggers on Deficit

**Goal**: Verify HousingAgent uses real zone data to detect deficit and propose bedrooms.

**Steps**:
1. With 7 dwarves and 3 bedrooms (deficit = 4), wait for autonomous cycle
2. Watch logs for HousingAgent analysis

**Expected Logs**:
```
[INFO] HousingAgent: Analyzing metrics - 7 dwarves, 3 bedrooms, deficit = 4
[DEBUG] HousingAgent: Querying SVP for housing Z-level
[INFO] HousingAgent: Proposing bedroom_cluster for 4 dwarves at Z=120 (SVP housing layer)
[DEBUG] HousingAgent: Urgency = 0.57 (4/7 dwarves unhoused)
```

**Verification**:
- HousingAgent detects real deficit (not placeholder 0)
- Proposal targets SVP housing Z-level (120, not arbitrary)
- Urgency calculated from percentage of dwarves unhoused

---

## Testing Blueprint Integration

### Test 5: Arbiter Selects Blueprint

**Goal**: Verify arbiter has access to blueprint metadata and selects appropriate blueprint.

**Steps**:
1. Ensure HousingAgent proposal exists (from Test 4)
2. Watch arbiter logs for blueprint selection

**Expected Logs**:
```
[DEBUG] Arbiter: System prompt includes 2 blueprints (bedroom_3x3, bedroom_cluster_10)
[INFO] Arbiter: Analyzing proposal graph with 1 node (bedroom_cluster)
[INFO] Arbiter: Selected blueprint 'bedroom_3x3' for 4 dwarves (4 instances)
[DEBUG] Arbiter: Rationale - "Compact design fits available space at housing Z=120"
```

**Verification**:
- Arbiter response includes blueprint name
- Blueprint choice makes sense (4 dwarves → 4 instances of bedroom_3x3)
- Rationale explains selection reasoning

---

### Test 6: Executor Sends BLUEPRINT Command

**Goal**: Verify executor sends BLUEPRINT command to plugin when arbiter selects blueprint.

**Steps**:
1. Arbiter decision includes blueprint (from Test 5)
2. Watch executor logs for command execution

**Expected Logs**:
```
[INFO] GraphExecutor: Processing arbiter decision (1 node, blueprint: bedroom_3x3)
[DEBUG] GraphExecutor: Sending BLUEPRINT command - name: bedroom_3x3, origin: (60,30,120), instances: 4
[INFO] DFHack client: Sent BLUEPRINT command (ID: 1234567890)
```

**Verification**:
- Executor sends BLUEPRINT command (not individual DIG+ZONE commands)
- Command includes blueprint name and placement coordinates
- Command ID logged for async tracking

---

## Testing Zone Queue

### Test 7: Zone Queued When Space Not Dug

**Goal**: Verify zone queue handles async execution (space not yet dug).

**Steps**:
1. Trigger bedroom proposal (HousingAgent with deficit)
2. Watch executor attempt zone placement

**Expected Logs**:
```
[DEBUG] GraphExecutor: Converting bedroom_cluster node to commands
[INFO] GraphExecutor: Sent DIG command for region (60,30,120) to (63,33,120)
[DEBUG] GraphExecutor: Attempting ZONE command for bedroom at (60,30,120)
[WARN] DFHack: ZONE command failed - space not yet dug
[INFO] ZoneQueue: Enqueued zone (bedroom, 60,30,120) - waiting for dig completion
```

**Verification**:
- DIG command sent successfully
- ZONE command fails (space not dug yet)
- Zone queued with status "waiting_for_dig"

---

### Test 8: Zone Placed After Dig Completion

**Goal**: Verify zone queue retries and succeeds after dwarves dig space.

**Steps**:
1. Wait for dwarves to dig bedroom (watch DF, tiles change from wall to floor)
2. Watch autonomous cycle logs for zone retry

**Expected Logs**:
```
[DEBUG] ZoneQueue: Processing queue (1 pending zones)
[DEBUG] ZoneQueue: Checking dig status for zone at (60,30,120)
[INFO] ModificationOverlay: Tiles (60,30,120) to (63,33,120) confirmed dug
[DEBUG] ZoneQueue: Retrying ZONE command (retry 1/3)
[INFO] DFHack: ZONE command success - bedroom created at (60,30,120)
[INFO] ZoneQueue: Zone completed, removed from queue (0 pending)
```

**Verification**:
- Zone queue detects dig completion
- ZONE command succeeds on retry
- Zone removed from queue
- In DF, bedroom zone appears (z menu shows new bedroom)

---

## Troubleshooting

### Issue: SVP Not Analyzing Terrain

**Symptoms**: No SVP logs, housing_z always 0

**Diagnosis**:
- Check `use_svp: true` in config
- Verify both RESYNC and ENTITY_UPDATE received (check logs)
- Ensure topology has embark data (not all zeros)

**Fix**:
```bash
# Enable debug logging
export LOG_LEVEL=DEBUG

# Re-run, check for topology and entity messages
./bin/df-orchestrator.exe
```

---

### Issue: Zone Extraction Returns Zero Zones

**Symptoms**: BedroomZoneCount always 0, even with zones in DF

**Diagnosis**:
- Check `extract_zones: true` in config
- Verify plugin has zone extraction code (new in Feature 007)
- Check DF has zones (z menu shows bedrooms)

**Fix**:
```bash
# Check plugin version (should include zone extraction)
# In DF console:
ls

# Look for "df-ai-plugin v0.7" or later
```

---

### Issue: Arbiter Never Selects Blueprints

**Symptoms**: Arbiter always uses geometric patterns, never references blueprints

**Diagnosis**:
- Check `include_blueprint_metadata: true` in config
- Verify blueprint CSV files exist in blueprints/ directory
- Check arbiter system prompt includes blueprint list (logs)

**Fix**:
```bash
# Verify blueprint metadata generation
# Look for startup logs:
grep "Blueprint metadata" orchestrator.log

# Expected:
# [INFO] Blueprint loader: Generated metadata for 2 blueprints
# [DEBUG] Arbiter prompt includes blueprints: bedroom_3x3, bedroom_cluster_10
```

---

### Issue: Zone Queue Never Completes

**Symptoms**: Zones queued indefinitely, never placed

**Diagnosis**:
- Check if dwarves actually digging (DF paused? dwarves idle?)
- Verify modification overlay detecting dig completion
- Check zone queue timeout (may have expired)

**Fix**:
```bash
# Check zone queue status
grep "ZoneQueue" orchestrator.log

# If zones timing out:
# Increase timeout in config
zone_queue_timeout_cycles: 20  # Was 10
```

---

## Performance Benchmarks

**Expected Performance** (from research.md Decision 12):

| Operation | Target | Acceptable | Warning Sign |
|-----------|--------|------------|--------------|
| SVP analysis | <200ms | <500ms | >1000ms |
| Zone extraction | <50ms | <100ms | >200ms |
| Zone queue processing | <10ms | <20ms | >50ms |
| Blueprint metadata generation | <5ms | <10ms | >20ms |

**How to Measure**:
- Enable DEBUG logging
- Look for timing logs: `[DEBUG] SVP analysis took 152ms`
- If performance poor, check fort size (huge forts = more zones)

---

## Success Checklist

Before considering Feature 007 complete, verify:

- [ ] SVP analyzes terrain on first connection (housing, workshop, farm Z designated)
- [ ] SVP persists to disk (saves/{fortname}/svp_layout.json created)
- [ ] SVP loads on reconnection (no re-analysis, designations restored)
- [ ] Zone extraction returns real bedroom counts (matches DF z menu)
- [ ] HousingAgent uses real zone data (proposes when deficit exists, silent when met)
- [ ] HousingAgent queries SVP for housing Z (proposals target correct Z-level)
- [ ] Arbiter receives blueprint metadata in system prompt
- [ ] Arbiter selects blueprints when appropriate (70% target)
- [ ] Executor sends BLUEPRINT command (not individual DIG+ZONE)
- [ ] Zone queue handles async execution (queues, retries, completes)
- [ ] All logs present (SVP, zone extraction, blueprint selection, queue)
- [ ] Performance acceptable (SVP <200ms, zones <50ms, queue <10ms)

---

## Next Steps

Once Feature 007 is working:

1. **Collect Metrics**: Run experiments to measure success criteria (SC-001 to SC-010)
2. **Compare with Feature 006**: A/B test (with vs without SVP)
3. **Create Additional Blueprints**: Design more bedroom layouts (nobles, dormitories)
4. **Enhance Other Agents**: Apply same pattern to FoodAgent (use real food stocks)
5. **Implement Feature 008**: Next priority (camera repositioning, or other agent enhancements)

---

## Reference

- **Spec**: [spec.md](./spec.md)
- **Planning**: [plan.md](./plan.md)
- **Research**: [research.md](./research.md)
- **Data Model**: [data-model.md](./data-model.md)
- **Tasks**: [tasks.md](./tasks.md) (generated by `/speckit.tasks`)
