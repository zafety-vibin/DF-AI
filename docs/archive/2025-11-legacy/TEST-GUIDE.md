# Testing Guide: Local LLM with HRM Architecture

**Goal**: Test DF-AI with LM Studio (local Qwen2.5-7B) using the HRM intent-based planning architecture
**Date**: 2025-11-09

---

## Prerequisites

### 1. LM Studio Setup

**Check LM Studio**:
- ✅ LM Studio running
- ✅ Model loaded (Qwen2.5-7B or similar)
- ✅ Server started (click "Start Server" in LM Studio)
- ✅ Default port: `http://localhost:1234/v1`

**Verify Model Name**:
In LM Studio, check what the model is called. Common names:
- `qwen2.5-7b-instruct`
- `qwen2.5-7b-instruct-q4_k_m`
- `Qwen/Qwen2.5-7B-Instruct`

**Test LM Studio** (optional):
```bash
curl http://localhost:1234/v1/models
```

Should return JSON with model list.

---

### 2. DFHack Plugin

**Status**: Not required for first test!

You can test the **orchestrator-side logic** without DF running:
- Agents will use placeholder metrics (0 dwarves)
- SVP won't analyze (needs topology from DF)
- But you can verify LLM calls work

**For full test**:
- Dwarf Fortress running
- DFHack plugin loaded
- Plugin connecting to orchestrator on port 5001

---

## Configuration Changes

### Edit `config/orchestrator.yaml`

Make these changes:

```yaml
# LLM Provider Configuration
llm_provider_type: openai_compatible  # CHANGE from 'claude' to use local LLM

# OpenAI-compatible endpoint settings
llm_endpoint: http://localhost:1234/v1  # LM Studio default
llm_model: qwen2.5-7b-instruct-q4_k_m   # VERIFY this matches your LM Studio model name
llm_api_key: ""                         # Empty for local

# LLM Request tuning
llm_temperature: 0.7
llm_max_tokens: 2048        # Reduce for local LLM (faster)
llm_timeout_seconds: 60s    # Increase for local LLM (slower than Claude)

# Goal Agent System (Feature 006)
enable_goal_agents: true    # MUST ENABLE for agents to work

# HRM Architecture (Feature 007 Refactor)
use_intent_planning: true   # ENABLE to test HRM architecture
                            # Set to false to test coordinate-based mode

# Feature 007: SVP
use_svp: true               # Already true
svp_persistence_dir: ./saves

# Feature 007: Zone Extraction
extract_zones: true          # Already true

# Feature 007: Blueprint Integration
blueprint_directory: ./blueprints
include_blueprint_metadata: true

# Agent Configuration
agent_housing:
  enabled: true
  priority: 9
  targets:
    bedrooms_per_dwarf: 1.0
```

**CRITICAL Changes**:
1. `llm_provider_type: openai_compatible`
2. `llm_model: <YOUR_MODEL_NAME>` (check LM Studio)
3. `enable_goal_agents: true`
4. `use_intent_planning: true` (for HRM test)

---

## System Prompts (No Action Needed)

**System prompts are built into the code**. They're sent automatically with each LLM request:

### Coordinate-Based Mode (`use_intent_planning: false`)
- Prompt: `getArbiterSystemPrompt()` in `internal/autonomous/loop.go:1240`
- Purpose: Coordinate ProposalGraph, resolve conflicts
- Input: Graph JSON with nodes + edges
- Output: Sorted execution sequence

### Intent-Based Mode (`use_intent_planning: true`)
- Prompt: `getIntentArbiterSystemPrompt()` in `internal/autonomous/loop.go:1367`
- Purpose: Map intents → blueprint placements with staircase anchoring
- Input: StrategicLayout + IntentProposals + Blueprints JSON
- Output: Blueprint+Anchor commands

**You don't need to upload anything to LM Studio!**

---

## Test Scenarios

### Test 1: Basic LLM Connection (No DF Required)

**Purpose**: Verify orchestrator can talk to LM Studio

**Steps**:
1. Edit `config/orchestrator.yaml` (changes above)
2. Start orchestrator:
   ```bash
   cd C:\Users\zmanl\Projects\DF-AI
   .\bin\df-orchestrator.exe
   ```

3. Watch logs for:
   ```
   INFO LLM provider configured provider=openai_compatible endpoint=http://localhost:1234/v1
   INFO goal agents enabled agent_count=5
   INFO intent-based planning enabled (HRM architecture)
   INFO SVP configured - waiting for topology and entity data
   INFO blueprint metadata loaded for arbiter blueprint_count=2
   ```

**Expected**: Orchestrator starts, waits for DFHack connection

---

### Test 2: Agent Analysis (Mock Data)

**Purpose**: Verify agents work with placeholder data

**What Happens**:
1. Autonomous loop runs every 5 seconds
2. `computeFortMetrics()` creates metrics with DwarfCount=0 (placeholder)
3. Agents analyze (HousingAgent won't propose with 0 dwarves)

**Watch logs**:
```
INFO starting autonomous decision cycle
DEBUG computed fort metrics dwarves=0 enemies=0
INFO no intent proposals this cycle (all targets met)
```

**This is expected** - agents need dwarves to propose housing!

---

### Test 3: Connect to Dwarf Fortress

**Prerequisites**:
- Dwarf Fortress running
- DFHack loaded
- Fort with dwarves

**Steps**:
1. In DF console, run:
   ```
   lua require('df-ai-plugin').connect()
   ```

2. Watch orchestrator logs for:
   ```
   INFO client connected
   INFO processing RESYNC message
   INFO topology overlay built tiles=2000000 memory_kb=250
   INFO SVP: Topology received, waiting for entity data
   INFO processing ENTITY_UPDATE message
   INFO SVP: Both topology and entities received, triggering terrain analysis
   INFO SVP: Terrain analysis complete embark_z=100 housing_z=95 workshop_z=94
   INFO SVP: Saved layout to disk path=./saves/unknown_fort/svp_layout.json
   DEBUG SVP: Analyzed layer z=95 purpose=housing region_count=1
   ```

3. After few cycles (5-10 seconds):
   ```
   DEBUG computed fort metrics dwarves=7 enemies=0 bedroom_zones=0 housing_deficit=7
   INFO agent analysis metrics proposal_count=1
   ```

**If intent-based planning enabled**:
```
INFO sending intent proposals to arbiter proposal_count=1 input_size=1500
DEBUG arbiter intent response tokens_prompt=500 tokens_completion=150
INFO arbiter intent decision parsed command_count=1
INFO arbiter blueprint command blueprint=bedroom_cluster_10 anchor=[42,31,95]
INFO blueprint command sent command_id=123456 blueprint=bedroom_cluster_10
```

**If coordinate-based planning**:
```
INFO arbiter decision received latency_ms=250
INFO arbiter selected blueprint node_id=housing_123 blueprint=bedroom_cluster_10
INFO sending BLUEPRINT command blueprint=bedroom_cluster_10 origin=(42,31,95)
```

---

## What to Test

### Scenario A: Intent-Based Planning (HRM Architecture)

**Config**:
```yaml
enable_goal_agents: true
use_intent_planning: true
```

**What Happens**:
1. HousingAgent detects deficit (7 dwarves, 0 bedrooms)
2. Proposes: `IntentProposal{Quantity: 7, Purpose: "housing", BlueprintHint: "bedroom_cluster_10"}`
3. SVP provides: `StrategicLayout{Layers: {"housing": {z: 95, regions: [...], existing_zones: {}}}}`
4. Arbiter receives JSON with layout + intent + blueprints
5. Arbiter outputs: `{blueprint: "bedroom_cluster_10", anchor: [42,31,95]}`
6. Executor sends BLUEPRINT command to DF

**Watch for** in logs:
- `collectIntentProposals()` finds 1 proposal
- `buildArbiterIntentInput()` creates JSON
- `getIntentArbiterSystemPrompt()` used
- Arbiter response parsed successfully
- BLUEPRINT command sent

---

### Scenario B: Coordinate-Based Planning (Legacy)

**Config**:
```yaml
enable_goal_agents: true
use_intent_planning: false
```

**What Happens**:
1. HousingAgent.Analyze() detects deficit
2. Proposes: `ModificationNode{Region: (60,30,95)-(70,40,95)}`
3. ProposalGraph assembled
4. Arbiter coordinates graph
5. GraphExecutor checks for blueprint in metadata
6. Sends BLUEPRINT command

**Watch for**:
- `analyzeAgents()` called
- `assembleProposalGraph()` creates graph
- `getArbiterSystemPrompt()` used (different prompt)
- Blueprint selected via node metadata

---

## Troubleshooting

### Issue: LLM Connection Fails

**Symptoms**:
```
ERROR arbiter intent request failed err="Post http://localhost:1234/v1/chat/completions: connection refused"
```

**Fix**:
1. Check LM Studio server is running (green "Running" indicator)
2. Verify port: LM Studio → Settings → Server → Port (default 1234)
3. Test with curl:
   ```bash
   curl http://localhost:1234/v1/models
   ```

---

### Issue: Wrong Model Name

**Symptoms**:
```
ERROR arbiter request failed err="model not found"
```

**Fix**:
1. In LM Studio, check loaded model name (top of chat window)
2. Update `llm_model` in config to exact match
3. Restart orchestrator

---

### Issue: No Agent Proposals

**Symptoms**:
```
INFO no intent proposals this cycle (all targets met)
```

**Reason**: Agents need actual deficit to propose

**Fix**:
- Connect to DF with dwarves but no bedrooms
- Or manually set deficit in code for testing

---

### Issue: SVP Not Ready

**Symptoms**:
```
WARN SVP layout not available for intent planning
```

**Reason**: SVP needs both RESYNC (topology) and ENTITY_UPDATE (entities)

**Fix**:
- Wait for both messages from DF
- Check logs for "SVP: Terrain analysis complete"

---

## Expected First Test Output

**With Intent Planning + LM Studio + DF Connected**:

```
[INFO] LLM provider configured provider=openai_compatible
[INFO] goal agents enabled agent_count=5
[INFO] intent-based planning enabled (HRM architecture)
[INFO] SVP enabled
[INFO] blueprint metadata loaded for arbiter blueprint_count=2
[INFO] autonomous loop started

... (waiting for DF connection) ...

[INFO] client connected
[INFO] processing RESYNC message
[INFO] SVP: Topology received
[INFO] processing ENTITY_UPDATE message
[INFO] SVP: Both topology and entities received, triggering terrain analysis
[INFO] SVP: Detected embark Z z=100
[INFO] SVP: Designated housing Z z=95
[INFO] SVP: Designated workshop Z z=94
[INFO] SVP: Terrain analysis complete embark_z=100 housing_z=95 workshop_z=94 layout_layers=3
[INFO] SVP: Saved layout to disk

... (5 seconds later) ...

[INFO] starting autonomous decision cycle
[DEBUG] computed fort metrics dwarves=7 bedroom_zones=0 housing_deficit=7
[DEBUG] sending intent proposals to arbiter proposal_count=1
[DEBUG] arbiter intent response tokens_prompt=450 tokens_completion=120
[INFO] arbiter intent decision parsed command_count=1
[INFO] arbiter blueprint command blueprint=bedroom_cluster_10 anchor=[42,31,95] reasoning="..."
[INFO] blueprint command sent command_id=1699564893000 blueprint=bedroom_cluster_10
```

---

## What You DON'T Need

- ❌ Upload system prompts to LM Studio
- ❌ Configure LM Studio system prompt
- ❌ Install anything in LM Studio

**Why**: System prompts are sent with each API request in the `system` field. LM Studio receives them automatically.

---

## What You DO Need

1. ✅ LM Studio running with model loaded
2. ✅ Edit `config/orchestrator.yaml` (4 line changes)
3. ✅ Run `.\bin\df-orchestrator.exe`
4. ✅ (Optional) Connect to DF for full test

---

## Quick Start Commands

```bash
# 1. Edit config (see changes above)
notepad config\orchestrator.yaml

# 2. Start orchestrator
cd C:\Users\zmanl\Projects\DF-AI
.\bin\df-orchestrator.exe

# 3. In another terminal, watch logs
# (orchestrator outputs to console)

# 4. (Optional) In DF console
lua require('df-ai-plugin').connect()
```

---

## Testing Checklist

- [ ] LM Studio running on port 1234
- [ ] Model loaded in LM Studio
- [ ] Config updated:
  - [ ] `llm_provider_type: openai_compatible`
  - [ ] `llm_model: <MATCH_LM_STUDIO>`
  - [ ] `enable_goal_agents: true`
  - [ ] `use_intent_planning: true` (for HRM test)
- [ ] Orchestrator starts without errors
- [ ] Log shows "intent-based planning enabled"
- [ ] (Optional) DF connects successfully
- [ ] (Optional) SVP analyzes terrain
- [ ] (Optional) Agent proposals sent to arbiter
- [ ] (Optional) Arbiter responds with blueprint commands

---

## First Test Recommendation

**Test Mode**: Intent-based planning WITHOUT DF connection

**Why**: Isolate LLM integration testing from DF complexity

**Config**:
```yaml
llm_provider_type: openai_compatible
llm_endpoint: http://localhost:1234/v1
llm_model: qwen2.5-7b-instruct-q4_k_m  # Adjust to your model
enable_goal_agents: true
use_intent_planning: true
```

**Expected Result**:
- Orchestrator starts ✓
- LLM configured ✓
- Waits for DF connection ✓
- No errors ✓

**If this works**, proceed to connect DF for full agent testing.

---

## Comparing Intent vs Coordinate Modes

Run **two tests** to compare:

### Test A: Intent-Based (HRM)
```yaml
use_intent_planning: true
```

**Watch for**:
- `sending intent proposals to arbiter`
- `buildArbiterIntentInput()` creates JSON with StrategicLayout
- Arbiter receives structured spatial context
- Outputs blueprint + anchor

### Test B: Coordinate-Based (Legacy)
```yaml
use_intent_planning: false
```

**Watch for**:
- `agent proposal` with coordinates
- `assembleProposalGraph()`
- Arbiter receives graph JSON
- Outputs node sequence

**Compare**: Both should send same BLUEPRINT command (just via different paths)

---

## Success Criteria

✅ **Basic Success**:
- Orchestrator starts without crashes
- LM Studio receives requests
- No connection errors

✅ **Integration Success** (with DF):
- SVP analyzes terrain
- StrategicLayout JSON generated
- HousingAgent proposes intent
- Arbiter responds with valid JSON
- Blueprint command sent to DF

✅ **Full Success**:
- Arbiter selects appropriate blueprint
- Staircase anchoring calculated correctly
- Blueprint command executes in DF (when plugin ready)

---

## Next Steps After Basic Test

1. **If LLM works**: Connect to DF, test full cycle
2. **If issues**: Debug connection, check model name, review logs
3. **After working**: Implement plugin BLUEPRINT handler (Phase 6)

Ready to start? Let me know if you need help with the config changes!
