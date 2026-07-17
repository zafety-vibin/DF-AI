# Quickstart: LLM Integration with Modification Tracking and Command Execution

**Feature**: 005-llm-integration | **Date**: 2025-11-07

## What Is This Feature?

Enables an AI to autonomously manage a Dwarf Fortress fort by:
1. **Observing** what's been built (modification overlay tracks player/AI changes)
2. **Understanding** fort state (viewport system sends compact context to LLM)
3. **Deciding** what to do next (LLM reasons about state and proposes commands)
4. **Acting** on decisions (bidirectional protocol sends commands to DFHack)
5. **Learning** from outcomes (feedback loop describes results including hazard encounters)

**Key Innovation**: AI sees "fort signature" (only modified tiles) not entire natural map, reducing context from 850 KB → 10 KB typical.

---

## 10-Minute Integration

### Step 1: Configure LLM Provider

Edit `config/orchestrator.yaml`:

```yaml
# LLM Provider Configuration
llm_provider_type: "claude"  # or "openai_compatible"
llm_enabled: true

# Claude API settings
claude_api_key: "sk-ant-..."  # Your Anthropic API key
claude_model: "claude-sonnet-4"

# OR: Local model settings
# llm_provider_type: "openai_compatible"
# llm_endpoint: "http://localhost:8000"
# llm_model: "meta-llama/Llama-3-70b-hf"
# llm_api_key: ""  # Empty for local servers

# Context settings
context_budget_kb: 200
context_update_frequency_seconds: 100
viewport_active_z_margin: 3  # ±3 Z-levels from highest modified tile
viewport_hazard_margin_tiles: 10

# Model parameters
llm_temperature: 0.7
llm_max_tokens: 4096
llm_timeout_seconds: 30
```

---

### Step 2: Initialize Overlays

```go
package main

import (
    "github.com/df-ai/orchestrator/internal/modifications"
    "github.com/df-ai/orchestrator/internal/context"
    "github.com/df-ai/orchestrator/internal/llm"
    "github.com/df-ai/orchestrator/internal/commands"
    "github.com/df-ai/orchestrator/internal/autonomous"
)

var (
    modificationOverlay *modifications.ModificationOverlay
    contextAssembler    *context.Assembler
    llmProvider         llm.Provider
    commandExecutor     *commands.Executor
    autonomousLoop      *autonomous.Loop
)

func main() {
    // ... existing setup ...

    // Initialize modification overlay on FULL_STATE
    client.SetOnFullState(func(state *protocol.FullStateMessage) {
        // ... existing topology/hazard builds ...

        // Build modification overlay (starts empty - tracks changes from now)
        modificationOverlay = modifications.NewModificationOverlay(
            modifications.Bounds{
                Width: state.Width,
                Height: state.Height,
                Depth: state.Depth,
            },
        )

        log.Info("modification overlay initialized - tracking changes from embark state")
    })

    // Update modification overlay on TILE_UPDATE
    updates := client.SubscribeTileUpdates()
    go func() {
        for update := range updates {
            if modificationOverlay != nil {
                modificationOverlay.DetectModifications(update.Tiles)
            }
        }
    }()
}
```

---

### Step 3: Initialize LLM Client

```go
// Create LLM provider based on config
providerFactory := llm.NewProviderFactory(cfg)
llmProvider, err := providerFactory.CreateProvider()
if err != nil {
    log.Fatal("failed to create LLM provider", err)
}

log.Infof("LLM provider initialized: %s (%s)",
    llmProvider.GetModelName(),
    llmProvider.GetProviderType())
```

---

### Step 4: Set Up Context Assembler

```go
contextAssembler = context.NewAssembler(
    cfg.ContextBudgetKB,
    cfg.ViewportActiveZMargin,
    cfg.ViewportHazardMarginTiles,
)
```

---

### Step 5: Initialize Command Executor

```go
commandExecutor = commands.NewExecutor(client)  // Uses existing DFHack client

// Subscribe to command acknowledgments
acks := client.SubscribeCommandAcks()  // New subscription method
go func() {
    for ack := range acks {
        commandExecutor.HandleAck(ack)
        log.Infof("command ack received: id=%d, status=%s", ack.CommandID, ack.Status)
    }
}()
```

---

### Step 6: Start Autonomous Loop

```go
autonomousLoop = autonomous.NewLoop(
    modificationOverlay,
    topologyOverlay,
    hazardManager,
    contextAssembler,
    llmProvider,
    commandExecutor,
    cfg.ContextUpdateFrequencySeconds,
)

// Start loop in background
go autonomousLoop.Start(ctx)

log.Info("autonomous AI loop started - will make decisions every 100s")
```

---

## Testing the System

### Test 1: Modification Tracking

**In DF**:
1. Load fort or start new embark
2. Connect DFHack plugin: `load df_ai_protocol` → `ai-connect`
3. Mine a small room (5×5 area)

**Check logs**:
```
"modification detected: action=DUG, count=25, bounds=(10,20,95)-(15,25,95)"
"chambers extracted: 1 chamber (25 tiles)"
```

**Verify**:
```
curl http://localhost:8081/metrics
```

Should show:
```json
{
  "modifications": {
    "total_count": 25,
    "bounds": {"x_min": 10, "x_max": 15, "y_min": 20, "y_max": 25, "z_min": 95, "z_max": 95},
    "chambers": 1
  }
}
```

---

### Test 2: Context Assembly

**Manual context generation** (before autonomous loop):

```go
ctx, err := contextAssembler.AssembleContext(
    context.Level1_ActiveArea,
    modificationOverlay,
    hazardManager,
    entityData,
)

log.Infof("Level 1 context: %d bytes", ctx.SizeBytes)
log.Info("Chambers: ", ctx.Chambers)
log.Info("Hazards: ", ctx.HazardsNearby)
```

**Expected output**:
```
Level 1 context: 8,523 bytes
Chambers: [{"bounds": "(10,20,95) to (15,25,95)", "tiles": 25, "description": "entrance hall 5×5"}]
Hazards: [{"type": "aquifer", "location": "(12,30,94)", "distance": 8}]
```

---

### Test 3: First LLM Interaction

**Trigger manually** (before loop):

```go
prompt := &llm.Prompt{
    SystemPrompt: "You are a Dwarf Fortress AI managing an early embark fort. Observe the fort state and suggest safe actions.",
    UserMessage:  contextAssembler.FormatAsJSON(ctx),
    MaxTokens:    4096,
    Temperature:  0.7,
}

response, err := llmProvider.SendPrompt(context.Background(), prompt)
if err != nil {
    log.Fatal("LLM call failed", err)
}

log.Infof("AI response (%d tokens): %s", response.TokensCompletion, response.Text)
```

**Expected AI response**:
```
"The fort is in early embark stage at surface level Z=95. There is one entrance hall (5×5, 25 tiles).
An aquifer is detected 8 tiles away to the north - mark as caution area. Seven dwarves are present.

Recommendation: Dig bedrooms for dwarves. Suggest expanding east from entrance hall to create 3 bedroom
chambers (3×3 each). Avoid digging north toward the aquifer until better understanding of geology.

Command: Dig region (16,20,95) to (25,23,95) for bedroom area."
```

---

### Test 4: Command Execution

**Parse command** from AI response:

```go
reasoning, cmdSpecs, err := llm.ParseResponse(response.Text)

for _, spec := range cmdSpecs {
    cmd := commands.NewCommand(spec.Type, spec.Region)

    // Send to DFHack
    err := commandExecutor.SendCommand(cmd)
    if err != nil {
        log.Error("failed to send command", err)
        continue
    }

    // Wait for acknowledgment
    ack, err := commandExecutor.WaitForAck(cmd.ID, 5*time.Second)
    if err != nil {
        log.Error("command ack timeout", err)
        continue
    }

    log.Infof("Command executed: id=%d, status=%s", cmd.ID, ack.Status)
}
```

**In DF**: Check that new designation appears (blue 'd' markers in the suggested region)

**Wait for dwarves to mine**: Tiles change from Rock → Floor

**Check TILE_UPDATE logs**: Modification overlay updated with DUG entries

---

### Test 5: Autonomous Loop (End-to-End)

**Start loop** (already running from step 6 above)

**Observe logs** every 100 seconds:

```
[Turn 1]
"assembling context: level=1, modifications=25, chambers=1"
"sending prompt to LLM: size=9423 bytes, model=claude-sonnet-4"
"LLM response received: tokens=380, latency=3.2s"
"parsed command: type=DIG, region=(16,20,95)-(25,23,95), reasoning=expand bedrooms"
"sending command to DFHack: id=1001"
"command ack received: id=1001, status=success"
"waiting for tile updates to confirm modifications..."
"tile updates received: 30 new DUG modifications"
"feedback generated: Successfully dug 30 tiles, created bedroom area as planned"
"turn complete: added to history"

[Turn 2 - 100s later]
"assembling context: level=1, modifications=55, chambers=2"
"sending prompt with history (1 previous turn)"
"LLM response: Continue expanding bedrooms..."
...
```

**After 10 minutes (6 turns)**: Check `logs/llm-interactions.jsonl` contains 6 entries with full decision chains.

---

## Common Use Cases

### Use Case 1: Observe Modifications Without AI

```go
// Just track what player does (no LLM calls)
client.SetOnFullState(func(state *protocol.FullStateMessage) {
    modificationOverlay = modifications.NewModificationOverlay(bounds)
})

go func() {
    for update := range updates {
        modificationOverlay.DetectModifications(update.Tiles)

        count := modificationOverlay.GetCount()
        bounds := modificationOverlay.GetBounds()
        chambers := modificationOverlay.ExtractChambers()

        log.Infof("Player activity: %d mods, %d chambers, bounds=%v",
            count, len(chambers), bounds)
    }
}()
```

**Use**: Validate modification detection before adding LLM complexity

---

### Use Case 2: One-Shot LLM Query (No Loop)

```go
// Single LLM call to get fort assessment
ctx, _ := contextAssembler.AssembleContext(context.Level1_ActiveArea, ...)

prompt := &llm.Prompt{
    SystemPrompt: "You are a Dwarf Fortress expert. Analyze this fort.",
    UserMessage:  contextAssembler.FormatAsJSON(ctx),
}

response, _ := llmProvider.SendPrompt(context.Background(), prompt)
fmt.Println("AI Assessment:", response.Text)
```

**Use**: Manual fort analysis without autonomous operation

---

### Use Case 3: Test Command Protocol (No LLM)

```go
// Manually create command without LLM
cmd := &commands.Command{
    Type: commands.CommandTypeDig,
    Region: modifications.Region{
        XMin: 10, YMin: 20, ZMin: 95,
        XMax: 15, YMax: 25, ZMax: 95,
    },
}

// Send to DFHack
commandExecutor.SendCommand(cmd)
ack, _ := commandExecutor.WaitForAck(cmd.ID, 5*time.Second)

fmt.Printf("Designation result: %s\n", ack.Status)
```

**Use**: Validate protocol messages before integrating LLM

---

## Debugging

### Enable Debug Logging

```yaml
# config/orchestrator.yaml
log_level: debug
```

**Debug logs show**:
- Every modification detected (action, coordinates)
- Context assembly details (size at each level)
- Full LLM prompts and responses
- Command serialization (hex dump)
- ACK parsing details

---

### Check Modification Overlay State

```go
func logModificationStats() {
    count := modificationOverlay.GetCount()
    bounds := modificationOverlay.GetBounds()
    chambers := modificationOverlay.ExtractChambers()

    log.WithFields(log.Fields{
        "mod_count": count,
        "bounds": bounds,
        "chambers": len(chambers),
        "memory_kb": count * 32 / 1024,
    }).Debug("modification overlay state")
}
```

---

### Validate Context Size

```go
for level := context.Level0_Overview; level <= context.Level3_FullContext; level++ {
    ctx, _ := contextAssembler.AssembleContext(level, ...)
    log.Infof("Level %d context: %d bytes (%.1f KB)", level, ctx.SizeBytes, float64(ctx.SizeBytes)/1024)
}
```

**Expected**:
```
Level 0 context: 482 bytes (0.5 KB)
Level 1 context: 9,234 bytes (9.0 KB)
Level 2 context: 48,192 bytes (47.1 KB)
Level 3 context: 198,456 bytes (193.8 KB)
```

---

### Test LLM Provider Connectivity

```go
// Simple ping test
testPrompt := &llm.Prompt{
    SystemPrompt: "You are a helpful assistant.",
    UserMessage:  "Say hello.",
    MaxTokens:    50,
}

response, err := llmProvider.SendPrompt(context.Background(), testPrompt)
if err != nil {
    log.Fatal("LLM provider not reachable", err)
}

log.Info("LLM test successful:", response.Text)
// Expected: "Hello! How can I assist you today?"
```

---

### Monitor Command Execution

```go
// Check pending commands
pending := commandExecutor.GetPendingCommands()
for _, p := range pending {
    elapsed := time.Since(p.SentAt)
    log.Infof("Command %d: type=%s, sent %s ago, ack=%v",
        p.ID, p.Type, elapsed, p.AckReceived)
}
```

---

### Inspect Conversation History

```go
history := autonomousLoop.GetHistory()
for i, turn := range history {
    log.Infof("Turn %d: context=%d bytes, response=%d tokens, outcome=%s",
        i+1, len(turn.Context), turn.Response.TokensCompletion, turn.Outcome)
}
```

---

## FAQ

**Q: How does AI know what's been built vs natural terrain?**

A: Modification overlay tracks ONLY player/AI changes. Natural terrain never appears in modification data. AI sees "fort signature" - just the work performed.

**Q: What if I load a save mid-game with existing buildings?**

A: Modification tracking starts from load point. Pre-existing buildings not tracked unless changed after load. Accept limitation for research phase.

**Q: Why 4 detail levels instead of one adaptive context?**

A: Staged levels are predictable and debuggable. Early game AI uses Level 1 (10 KB), doesn't waste tokens on irrelevant cavern data. When AI needs caverns, explicitly request Level 2.

**Q: How does AI learn from hazard failures?**

A: Feedback loop describes outcomes. "Dug into aquifer → water flooding" becomes part of conversation history. Next turn, AI sees previous failure and (hopefully) avoids similar mistake.

**Q: Can AI issue multiple commands per turn?**

A: Yes. LLM response can contain multiple CommandSpecs. All executed sequentially with separate ACKs.

**Q: What if DFHack crashes mid-command?**

A: ACK timeout (5s) marks command as failed. Feedback to AI: "Command 1005 failed: no acknowledgment received". AI can retry or adjust strategy.

**Q: How expensive is this with Claude API?**

A: Level 1 context (~10 KB) ≈ 2500 tokens input. At 100s cycles, 36 calls/hour. Claude Sonnet pricing: ~$0.90/hour. Acceptable for research.

**Q: Can I use local Llama model instead?**

A: Yes! Configure `llm_provider_type: "openai_compatible"` and `llm_endpoint: "http://localhost:8000"`. Compatible with vLLM, llama.cpp, Ollama.

**Q: What's the multi-model pipeline?**

A: Two-stage: Fast local model extracts features (50 KB → 5 KB), capable model plans strategy. Reduces cost and improves quality. Configure both providers in pipeline mode.

---

## Next Steps After Integration

1. **Run first autonomous cycle** - Let AI observe empty embark and suggest first action
2. **Verify command execution** - Check DF UI for designations applied
3. **Test hazard encounter** - Let AI dig into aquifer, observe feedback
4. **Analyze decision logs** - Review `llm-interactions.jsonl` for patterns
5. **Experiment with prompts** - Adjust system prompt to guide AI behavior
6. **Try local model** - Compare quality with Claude vs Llama-70B
7. **Optimize context** - Tune viewport margins to balance detail vs token usage

See [tasks.md](./tasks.md) (after running `/speckit.tasks`) for detailed implementation checklist.
