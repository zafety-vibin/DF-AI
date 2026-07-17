# Quickstart: Local LLM with Graph-Based Goal-Oriented Agents

**Feature**: 006-local-llm-agents
**Estimated Setup Time**: 20-30 minutes
**Prerequisites**: Windows 10/11, RTX 4070 8GB (or equivalent), 16GB+ RAM

## Overview

This quickstart guide walks you through setting up local LLM inference with LM Studio and enabling graph-based goal-oriented agents for autonomous Dwarf Fortress fort management.

---

## Step 1: Install LM Studio (5 minutes)

1. **Download LM Studio**:
   - Visit: https://lmstudio.ai/
   - Download Windows installer
   - Install to default location

2. **Launch LM Studio**:
   - Open LM Studio application
   - Accept terms and complete initial setup

---

## Step 2: Download Qwen2.5 Model (10-15 minutes)

1. **Search for model**:
   - Click "Search" tab in LM Studio
   - Search: `Qwen2.5-7B-Instruct`
   - Filter by quantization: Q4_K_M or Q5_K_M

2. **Download recommended model**:
   - **Option 1** (Default): `bartowski/Qwen2.5-7B-Instruct-GGUF` → `Qwen2.5-7B-Instruct-Q4_K_M.gguf`
     - Size: ~4.5GB
     - VRAM: ~4.5GB
     - Inference: 300-500ms

   - **Option 2** (Better quality): `bartowski/Qwen2.5-14B-Instruct-GGUF` → `Qwen2.5-14B-Instruct-Q5_K_M.gguf`
     - Size: ~8.5GB
     - VRAM: ~8.5GB (tight fit on 8GB card)
     - Inference: 800-1000ms

3. **Wait for download** to complete

**Why Qwen2.5-Instruct?**
- Optimized for reasoning and decision-making (not code generation)
- Excellent spatial reasoning for fort layout planning
- Handles JSON input/output reliably
- Performs well at Q4_K_M quantization

---

## Step 3: Start Local LLM Server (2 minutes)

1. **Load model**:
   - Click "Local Server" tab in LM Studio
   - Select downloaded model from dropdown
   - Click "Load Model"
   - Wait for model to load (10-30 seconds)

2. **Start server**:
   - Click "Start Server" button
   - Default endpoint: `http://localhost:1234/v1`
   - Verify server status shows "Running"

3. **Test connection** (optional):
   - Open browser: `http://localhost:1234/v1/models`
   - Should return JSON with loaded model info

**Server settings**:
- Context length: 8192 (default)
- GPU layers: Max (offload entire model to GPU)
- Temperature: 0.7 (default, arbiter prompt will override)

---

## Step 4: Configure DF AI Orchestrator (3 minutes)

1. **Edit config file**:
   - Open: `C:\Users\zmanl\Projects\DF-AI\config\orchestrator.yaml`

2. **Update LLM settings**:
```yaml
# LLM Provider Configuration
llm_provider_type: local  # Change from "claude" to "local"
local_llm_endpoint: http://localhost:1234/v1
local_llm_model: qwen2.5-7b-instruct-q4_k_m  # Or your chosen model
llm_timeout_ms: 5000  # 5 seconds (increase to 10000 if using 14B model)
```

3. **Enable goal agents**:
```yaml
# Goal Agent System
enable_goal_agents: true

agent_food:
  enabled: true
  priority: 10
  targets:
    food_per_dwarf: 20
    drink_per_dwarf: 15

agent_housing:
  enabled: true
  priority: 9
  targets:
    bedrooms_per_dwarf: 1

agent_mining:
  enabled: true
  priority: 7
  targets:
    tiles_per_cycle: 50
    no_strike_cycles: 20

agent_wealth:
  enabled: true
  priority: 6
  targets:
    growth_threshold: 0.10  # 10% growth per 10 cycles

agent_defense:
  enabled: true
  priority_base: 0       # Low priority when no threats
  priority_threat: 10    # Max priority when enemies detected
```

4. **Save file**

---

## Step 5: Build and Run Orchestrator (2 minutes)

1. **Build**:
```bash
cd C:\Users\zmanl\Projects\DF-AI
go build -o bin/df-orchestrator.exe cmd/df-orchestrator/main.go
```

2. **Start orchestrator**:
```bash
.\bin\df-orchestrator.exe --config config\orchestrator.yaml
```

3. **Verify startup**:
   - Check logs for: `Local LLM provider initialized: http://localhost:1234/v1`
   - Check logs for: `Goal agents enabled: 5 agents registered`
   - Verify no connection errors

---

## Step 6: Connect to Dwarf Fortress (1 minute)

1. **Launch Dwarf Fortress**
   - Start DF with DFHack
   - Load existing fort or embark new game

2. **Enable AI protocol plugin**:
```
DFHack console: enable df_ai_protocol
```

3. **Verify connection**:
   - Orchestrator logs: `DFHack client connected`
   - DFHack logs: `df_ai_protocol: Connected to orchestrator`

4. **Trigger first cycle**:
   - Wait for autonomous loop cycle (100 seconds default)
   - Or trigger manually: DFHack command `ai-resync`

---

## Step 7: Monitor Agent Activity (Ongoing)

### Watch logs for agent proposals

**Example log output**:
```json
{
  "timestamp": "2025-11-09T12:00:00Z",
  "level": "INFO",
  "message": "Agent analysis complete",
  "agents": 5,
  "proposals": 2,
  "latency_ms": 45
}

{
  "timestamp": "2025-11-09T12:00:00Z",
  "level": "DEBUG",
  "agent": "FoodSecurity",
  "proposal": {
    "id": "food_1",
    "type": "farm_plot",
    "region": {"x1": 40, "y1": 30, "z": 125, "x2": 50, "y2": 40},
    "priority": 10,
    "rationale": "Food at 11.4/dwarf, below 20 target"
  }
}

{
  "timestamp": "2025-11-09T12:00:01Z",
  "level": "INFO",
  "message": "Arbiter decision complete",
  "execution_sequence": ["corridor_1", "food_1", "housing_1"],
  "synergies": 1,
  "deferred": 0,
  "latency_ms": 420,
  "tokens_used": 890
}
```

### Key metrics to watch

- **Agent latency**: Should be <100ms for all 5 agents combined
- **Graph assembly**: Should be <50ms
- **Arbiter latency**: Should be <500ms (7B) or <1000ms (14B)
- **Total cycle time**: Should be <2 seconds
- **Token usage**: Should be <1000 tokens per cycle
- **Synergy recognition**: Count of shared corridors, multi-phase plans

### In-game observation

- Check designations menu (d) for dig commands from agents
- Verify corridors connect bedroom clusters to farms
- Observe multi-phase plans: stairs → tunnels → rooms
- Monitor fort survival: food stocks, bedroom count, dwarf happiness

---

## Step 8: Experiment with Agent Settings (Optional)

### Increase food priority for challenging embarks

```yaml
agent_food:
  targets:
    food_per_dwarf: 30  # Higher threshold = more aggressive food gathering
```

### Disable wealth agent to focus on survival

```yaml
agent_wealth:
  enabled: false  # No workshop/production optimization
```

### Adjust mining aggressiveness

```yaml
agent_mining:
  targets:
    tiles_per_cycle: 100  # Double the mining activity
    no_strike_cycles: 10  # Shorter patience before exploratory digging
```

### Change model for better quality

```yaml
local_llm_model: qwen2.5-14b-instruct-q5_k_m
llm_timeout_ms: 10000  # Allow more time for larger model
```

---

## Troubleshooting

### Error: "Local LLM connection failed"

**Symptoms**: Orchestrator logs show timeout or connection refused

**Solutions**:
1. Verify LM Studio server is running (green "Running" status)
2. Check endpoint matches config: `http://localhost:1234/v1`
3. Test endpoint in browser: should return JSON
4. Increase timeout: `llm_timeout_ms: 10000`

### Error: "Out of VRAM"

**Symptoms**: LM Studio fails to load model, or crashes during inference

**Solutions**:
1. Close other GPU applications (games, rendering software)
2. Switch to smaller model: Qwen2.5-7B Q4_K_M instead of 14B
3. Reduce GPU layers in LM Studio (offload some to RAM)
4. Check Task Manager → Performance → GPU → Dedicated GPU memory

### Error: "Agent analysis timeout"

**Symptoms**: Logs show agents taking >100ms

**Solutions**:
1. This is rare - agents are deterministic Go code
2. Check CPU usage - may be thermal throttling
3. Reduce agent count: disable wealth or mining agents temporarily
4. Report as bug with logs

### Warning: "Token usage exceeded budget"

**Symptoms**: Logs show >1000 tokens per cycle

**Solutions**:
1. This is expected occasionally (complex proposals)
2. If consistent: reduce proposal detail in agent code
3. Check for excessive node metadata
4. May indicate too many simultaneous proposals (5+ nodes from single agent)

### Issue: "No commands executed"

**Symptoms**: Agents propose nodes, arbiter decides, but no DF designations appear

**Solutions**:
1. Check DFHack plugin loaded: `ls` command, look for df_ai_protocol
2. Verify connection: orchestrator logs show "DFHack client connected"
3. Check modification overlay persistence: may be deferring commands
4. Trigger resync: DFHack command `ai-resync`

### Issue: "Arbiter keeps rejecting proposals"

**Symptoms**: All agent proposals marked as "deferred" or "rejected"

**Solutions**:
1. Check logs for rejection reasons: "spatial_conflict", "invalid_region"
2. May indicate hazard detection (agents proposing in water/lava)
3. Verify fort has valid diggable area (not all open space)
4. Reduce proposal region sizes in agent code

---

## Performance Benchmarks

### Expected Performance (RTX 4070 8GB, Qwen2.5-7B Q4_K_M)

- Agent analysis: 30-50ms (5 agents parallel)
- Graph assembly: 15-30ms
- Topological sort: 5-10ms
- Arbiter inference: 300-500ms
- Total cycle: 1.2-1.8 seconds

### Token Usage Breakdown

- System prompt (cached): 400 tokens
- User message (proposals): 400-600 tokens
- Completion (execution sequence): 150-250 tokens
- Total: 950-1250 tokens (within 1k budget with margin)

### VRAM Usage

- Qwen2.5-7B Q4_K_M: ~4.5GB model + ~1GB overhead = 5.5GB total
- Qwen2.5-14B Q5_K_M: ~8.5GB model + ~1GB overhead = 9.5GB total (may swap)

---

## Next Steps

### Observe autonomous fort management

- Let system run for 100+ in-game days
- Watch how agents prioritize food → housing → mining → wealth
- Observe synergy recognition (shared corridors between bedrooms and dining halls)
- Check survival rate compared to manual play or Feature 005 direct-LLM mode

### Compare graph-based vs command-based

- Disable graph agents: `enable_goal_agents: false`
- Run same map seed with Feature 005 direct-LLM mode
- Compare:
  - Survival time
  - Fort efficiency (food per dwarf, bedroom coverage)
  - Token usage (should be 4-5x higher in direct-LLM mode)
  - Spatial quality (are bedrooms clustered? corridors efficient?)

### Experiment with model variants

- Try Qwen2.5-14B for better synergy recognition
- Test other Instruct models (Mistral, Llama-3)
- Measure quality vs latency tradeoff

### Contribute findings

- Share logs and metrics in project discussions
- Report bugs or unexpected behaviors
- Suggest new node types or synergy patterns

---

## Success Criteria Checklist

After setup, verify these targets are met:

- [ ] Full decision cycle completes in <2 seconds (check logs)
- [ ] Token usage averages <1000 per cycle (check llm-interactions.jsonl)
- [ ] Food stocks maintained above 15 units/dwarf (check DF UI)
- [ ] All dwarves have bedroom assignments within 30 days (check zones menu)
- [ ] No out-of-memory crashes during 8+ hour run
- [ ] Synergy recognition occurs in 50%+ of multi-proposal cycles (check logs for "corridor_connector" injections)
- [ ] Dependency chains execute in correct order (check modification logs: shaft before tunnel)

---

## Additional Resources

- **LM Studio Docs**: https://lmstudio.ai/docs
- **Qwen2.5 Model Card**: https://huggingface.co/Qwen/Qwen2.5-7B-Instruct
- **Feature Spec**: [spec.md](spec.md)
- **Data Model**: [data-model.md](data-model.md)
- **Research Decisions**: [research.md](research.md)

---

**Setup complete!** You should now have local LLM-powered autonomous fort management with graph-based spatial reasoning.

Monitor logs for the first few cycles to verify all components working correctly. Report issues or observations to project maintainers.
