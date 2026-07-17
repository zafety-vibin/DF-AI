# User Setup Tasks: Feature 006

**Your Role**: Manual setup and testing tasks that cannot be automated by the AI

**AI's Role**: Implementation tasks in [tasks.md](tasks.md) (T001-T093)

---

## Your Tasks (In Order)

### ⏰ BEFORE IMPLEMENTATION STARTS

#### Task U001: Install LM Studio (10 minutes)

**When**: Do this NOW, before AI starts implementation

**Steps**:
1. Visit https://lmstudio.ai/
2. Download Windows installer
3. Run installer, accept defaults
4. Launch LM Studio application
5. Complete initial setup wizard

**How to verify**:
- LM Studio opens without errors
- You can see "Search", "Local Server", and "Chat" tabs

---

#### Task U002: Download Qwen2.5-7B-Instruct Model (15 minutes)

**When**: After LM Studio installed, before AI finishes implementation

**Steps**:
1. In LM Studio, click "Search" tab
2. Search: `Qwen2.5-7B-Instruct`
3. Find: `bartowski/Qwen2.5-7B-Instruct-GGUF`
4. Download: `Qwen2.5-7B-Instruct-Q4_K_M.gguf` (~4.5GB)
5. Wait for download to complete (10-15 minutes depending on connection)

**Alternative** (if you have 8GB+ VRAM and want better quality):
- Download: `Qwen2.5-14B-Instruct-Q5_K_M.gguf` (~8.5GB)
- Note: This is tighter fit on 8GB GPU, may swap to system RAM

**How to verify**:
- Model appears in "Local Server" dropdown
- Size shows ~4.5GB (7B) or ~8.5GB (14B)

---

#### Task U003: Start LM Studio Server (2 minutes)

**When**: Before running orchestrator for first time

**Steps**:
1. In LM Studio, click "Local Server" tab
2. Select your downloaded model from dropdown
3. Click "Load Model" button
4. Wait 10-30 seconds for model to load
5. Click "Start Server" button
6. Verify status shows "Running"
7. Note endpoint: `http://localhost:1234/v1` (should be displayed)

**Server Settings** (defaults are fine):
- Context length: 8192
- GPU layers: Max (offload entire model to GPU)
- Temperature: 0.7 (arbiter prompt will override this)

**How to verify**:
- Status shows "Running" in green
- Open browser: `http://localhost:1234/v1/models`
- Should return JSON with model info

**IMPORTANT**: Keep LM Studio running during entire orchestrator session!

---

### ⏰ DURING IMPLEMENTATION (AI is working on tasks.md)

#### Task U004: Update orchestrator.yaml for Local LLM (3 minutes)

**When**: After AI completes T016-T017 (config schema updates)

**Steps**:
1. Open: `C:\Users\zmanl\Projects\DF-AI\config\orchestrator.yaml`
2. Find LLM Provider section
3. Change `llm_provider_type: claude` → `llm_provider_type: local`
4. Add these lines:
   ```yaml
   llm_provider_type: local
   local_llm_endpoint: http://localhost:1234/v1
   local_llm_model: qwen2.5-7b-instruct-q4_k_m
   llm_timeout_ms: 5000
   ```
5. Find Goal Agent section (should already exist from AI's work)
6. Verify `enable_goal_agents: true`
7. Optionally adjust agent thresholds (see examples below)
8. Save file

**Optional Agent Tuning**:
```yaml
agent_food:
  enabled: true
  priority: 10
  targets:
    food_per_dwarf: 30  # Higher = more aggressive food gathering

agent_housing:
  enabled: true
  priority: 9
  targets:
    bedrooms_per_dwarf: 1

agent_defense:
  enabled: true
  priority_base: 0
  priority_threat: 10
```

**How to verify**:
- File saves without errors
- YAML syntax valid (no tabs, proper indentation)

---

### ⏰ AFTER IMPLEMENTATION COMPLETE (AI finished tasks.md)

#### Task U005: Build Updated Orchestrator (2 minutes)

**When**: After AI completes all implementation tasks (T001-T055 for MVP)

**Steps**:
1. Open PowerShell in project root
2. Run:
   ```bash
   cd C:\Users\zmanl\Projects\DF-AI
   go build -o bin/df-orchestrator.exe cmd/df-orchestrator/main.go
   ```
3. Wait for build to complete (30-60 seconds)
4. Check for errors

**How to verify**:
- Build completes with "success" or no errors
- File exists: `bin/df-orchestrator.exe`
- File size reasonable (~15-25 MB)

---

#### Task U006: First Test Run (5 minutes)

**When**: After successful build

**Prerequisites**:
- ✅ LM Studio running with model loaded
- ✅ Server status shows "Running"
- ✅ orchestrator.yaml updated with local settings

**Steps**:
1. In PowerShell:
   ```bash
   cd C:\Users\zmanl\Projects\DF-AI
   .\bin\df-orchestrator.exe --config config\orchestrator.yaml
   ```
2. Watch startup logs carefully

**What to look for** (success indicators):
```
INFO: Local LLM provider initialized: http://localhost:1234/v1
INFO: Goal agents enabled: 5 agents registered
INFO: Agent registered: FoodSecurity (priority: 10)
INFO: Agent registered: Housing (priority: 9)
INFO: Agent registered: Mining (priority: 7)
INFO: Agent registered: Wealth (priority: 6)
INFO: Agent registered: Defense (priority: 0-10)
INFO: HTTP server listening on :8080
```

**Errors to watch for**:
- ❌ "Local LLM connection failed" → Check LM Studio is running, server started
- ❌ "Out of VRAM" → Switch to 7B model if using 14B
- ❌ "Config validation error" → Check orchestrator.yaml syntax

**How to verify**:
- Orchestrator starts without errors
- Logs show local LLM connection successful
- Logs show 5 agents registered
- HTTP server listening

---

#### Task U007: Connect Dwarf Fortress (2 minutes)

**When**: After orchestrator running successfully

**Prerequisites**:
- ✅ Orchestrator running
- ✅ No connection errors in logs

**Steps**:
1. Launch Dwarf Fortress with DFHack
2. Load existing fort or embark new game
3. In DFHack console, type:
   ```
   enable df_ai_protocol
   ```
4. Press Enter

**What to look for**:
- DFHack console: "df_ai_protocol: Connected to orchestrator"
- Orchestrator logs: "DFHack client connected"

**How to verify**:
- Orchestrator logs show entity updates arriving
- DFHack console shows connection message
- No timeout errors

---

#### Task U008: Trigger First AI Cycle (3 minutes)

**When**: After DFHack connected

**Steps**:
1. Wait 100 seconds (default autonomous cycle time)
   - OR trigger manually: DFHack command `ai-resync`

2. Watch orchestrator logs for:
   ```
   INFO: Agent analysis complete (agents: 5, proposals: 2, latency_ms: 45)
   DEBUG: Agent proposal: FoodSecurity -> farm_plot at (40,30,125)
   DEBUG: Agent proposal: Housing -> bedroom_cluster at (60,30,125)
   INFO: Arbiter decision complete (synergies: 1, latency_ms: 420, tokens: 890)
   INFO: Executing 3 commands: corridor_connector, farm_plot, bedroom_cluster
   ```

3. In Dwarf Fortress, press `d` (designations menu)
4. Look for dig designations appearing on map

**Success indicators**:
- Logs show agent proposals
- Logs show arbiter decision with synergies recognized
- Logs show commands executed
- DF shows dig designations on map
- Latency < 2 seconds total
- Tokens < 1000

**How to verify**:
- At least 1 agent proposal in logs
- Arbiter responded (no timeout)
- Commands sent to DFHack
- Dig designations visible in DF

---

### ⏰ VALIDATION & TESTING

#### Task U009: Run 100-Day Survival Test (2-3 hours game time)

**When**: After successful first cycle

**Purpose**: Validate all success criteria from spec.md

**Steps**:
1. Start new fort (or continue existing)
2. Let orchestrator run autonomously for 100 in-game days
3. Monitor logs periodically (every 15 minutes real-time)
4. Check DF periodically to see fort progress

**What to monitor**:

**In Logs**:
- Agent proposals appear regularly
- Arbiter recognizes synergies (corridor_connector injections)
- Cycle latency stays < 2s
- Token usage stays < 1000
- No out-of-memory errors

**In Dwarf Fortress**:
- Food stocks (z → stocks): Should stay > 15 per dwarf
- Bedrooms (z → zones): All dwarves assigned within 30 days
- Dig patterns: Corridors connect rooms (not isolated rectangles)
- Dwarves: No starvation deaths

**Success Criteria** (from spec.md):
- [ ] Fort survives 100+ days
- [ ] Food > 15/dwarf for 95%+ of cycles
- [ ] All dwarves have bedrooms within 30 days
- [ ] Cycle latency < 2s for 95%+ of cycles
- [ ] Token usage < 1000 average
- [ ] Synergy recognition occurs (50%+ of multi-proposal cycles)
- [ ] Zero API costs (check: no Claude API calls in logs)
- [ ] No out-of-memory crashes

**How to verify**:
- All success criteria checkboxes above can be checked
- Fort is thriving, not failing
- Logs show healthy metrics throughout

---

#### Task U010: Test Configuration Changes (Optional, 30 minutes)

**When**: After successful 100-day test

**Purpose**: Verify agent behavior is configurable

**Test 1: Higher Food Threshold**:
1. Stop orchestrator (Ctrl+C)
2. Edit config: `food_per_dwarf: 30` (was 20)
3. Restart orchestrator
4. Verify: FoodAgent proposes at 25 food/dwarf (above old threshold)

**Test 2: Disable Wealth Agent**:
1. Edit config: `agent_wealth.enabled: false`
2. Restart orchestrator
3. Verify: No workshop/production proposals in logs

**Test 3: Priority Override**:
1. Edit config: `agent_housing.priority: 10`, `agent_food.priority: 9`
2. Trigger low food + low housing
3. Verify: Housing proposal wins conflicts (logs show priority order)

**How to verify**:
- Config changes affect agent behavior
- Logs show different thresholds triggering
- Disabled agents don't appear in proposals

---

#### Task U011: Test Emergency Handling (Optional, 15 minutes)

**When**: After core functionality validated

**Purpose**: Verify arbiter handles novel events

**Test 1: Forgotten Beast**:
1. In DFHack console: `modtools/spawn CREATURE:FORGOTTEN_BEAST`
2. Wait for next AI cycle
3. Check logs: Should show emergency seal_cavern or defensive_wall proposal
4. Verify: Emergency node has high priority, executes first

**Test 2: Aquifer Breach** (advanced):
1. Dig into aquifer layer (if embark has one)
2. Wait for water to spread (50+ tiles)
3. Check logs: Should show emergency flood response
4. Verify: Arbiter generates ad-hoc emergency nodes

**How to verify**:
- Emergency events detected
- Arbiter generates appropriate emergency nodes
- Emergency priorities override normal priorities

---

## Troubleshooting

### "Local LLM connection failed"

**Symptoms**: Orchestrator logs show timeout or connection refused

**Fix**:
1. Check LM Studio is running (green "Running" status)
2. Check server endpoint: should be `http://localhost:1234/v1`
3. Test in browser: `http://localhost:1234/v1/models` should return JSON
4. Check firewall: may be blocking localhost connections
5. Increase timeout in config: `llm_timeout_ms: 10000`

---

### "Out of VRAM"

**Symptoms**: LM Studio fails to load model or crashes

**Fix**:
1. Close other GPU applications (games, browsers with GPU acceleration)
2. Switch to smaller model: Qwen2.5-7B Q4_K_M instead of 14B
3. In LM Studio: Reduce "GPU Layers" slider (offload some to RAM)
4. Check Task Manager → Performance → GPU → Dedicated memory

---

### "No agent proposals appearing"

**Symptoms**: Logs show agent analysis but no proposals

**Fix**:
1. Check fort metrics: Food may be above threshold, bedrooms may be adequate
2. Lower thresholds in config to trigger more proposals
3. Start new embark with challenging conditions (scarce food)
4. Check agent enabled flags: all should be `true`

---

### "Arbiter timeout"

**Symptoms**: Logs show "LLM request timeout" after 5 seconds

**Fix**:
1. Check LM Studio server is running and responsive
2. Increase timeout: `llm_timeout_ms: 10000` (10 seconds)
3. If using 14B model, this is expected first request (model loading)
4. Check system resources: CPU/RAM may be maxed out

---

### "Dig designations not appearing in DF"

**Symptoms**: Logs show commands executed but DF has no designations

**Fix**:
1. Verify DFHack plugin loaded: `ls` command, look for df_ai_protocol
2. Check connection: orchestrator logs should show "DFHack client connected"
3. Try manual resync: DFHack command `ai-resync`
4. Check DF is unpaused (space bar)
5. Verify dwarves have picks (mining tools)

---

## Summary: Your Checklist

**Before Implementation**:
- [ ] U001: Install LM Studio
- [ ] U002: Download Qwen2.5-7B-Instruct Q4_K_M model

**During Implementation** (while AI works):
- [ ] U003: Start LM Studio server (keep running)
- [ ] U004: Update orchestrator.yaml with local LLM settings

**After Implementation**:
- [ ] U005: Build updated orchestrator binary
- [ ] U006: First test run (verify connection)
- [ ] U007: Connect Dwarf Fortress
- [ ] U008: Trigger first AI cycle
- [ ] U009: Run 100-day survival test
- [ ] U010: Test configuration changes (optional)
- [ ] U011: Test emergency handling (optional)

**Total Time Estimate**:
- Setup (U001-U004): 30 minutes
- Testing (U005-U008): 15 minutes
- Validation (U009): 2-3 hours (mostly game running autonomously)
- Optional tests (U010-U011): 45 minutes

---

## When to Do What

| Your Task | AI Task | Timing |
|-----------|---------|--------|
| U001-U002: Install LM Studio, download model | - | Do NOW, before AI starts |
| U003: Start LM Studio server | - | Before testing |
| - | T001-T055: Implement MVP | AI works autonomously |
| U004: Update config | After AI completes T016-T017 | During AI work |
| U005: Build binary | After AI completes T055 | After AI done |
| U006-U009: Test and validate | - | After build |
| U010-U011: Advanced testing | T056-T093: P2 features + polish | Optional |

---

**Key Takeaway**: You do ~1 hour of manual setup/testing. AI does ~14-20 hours of code implementation. Then you validate together.
