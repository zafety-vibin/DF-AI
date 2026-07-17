# Research Decisions: Local LLM with Graph-Based Goal-Oriented Agents

**Feature**: 006-local-llm-agents
**Date**: 2025-11-09
**Phase**: 0 (Technical Research)

## Overview

This document resolves technical unknowns and design decisions for implementing graph-based goal-oriented agents with local LLM arbitration. All NEEDS CLARIFICATION items from Technical Context addressed.

---

## Decision 1: Graph-Based Proposal Architecture

**Question**: Should agents propose modification graph nodes or command lists?

**Decision**: Graph-based modification nodes with dependencies and conflicts

**Rationale**:
- **Spatial synergies**: bedroom_cluster + dining_hall can share corridor (inject corridor_connector node)
- **Temporal planning**: mining_shaft → exploratory_tunnel dependencies enable multi-phase plans without stateful agents
- **Existing infrastructure**: ModificationOverlay already provides graph substrate (sparse coordinate map)
- **Token efficiency**: Graph JSON (nodes + edges) is ~400-600 tokens vs 4000+ token fort state

**Alternatives Considered**:
1. **Command lists** (Feature 005 approach)
   - Rejected: Cannot represent spatial relationships (shared corridors)
   - Rejected: Cannot encode multi-turn dependencies
   - Rejected: Arbiter must infer synergies from coordinates (unreliable)

2. **Action queue with priorities**
   - Rejected: No spatial reasoning (which actions share infrastructure)
   - Rejected: Priority alone insufficient for conflict resolution

**Implementation**:
- ModificationNode struct: type, region, dependencies[], conflicts[], priority, rationale
- ProposalGraph: nodes[], dependencyEdges (directed), conflictEdges (undirected)
- Topological sort algorithm for dependency ordering (Kahn's algorithm, O(V+E))
- Conflict detection via AABB spatial intersection

---

## Decision 2: LM Studio Integration Pattern

**Question**: How to integrate local LLM endpoint with existing provider abstraction?

**Decision**: New LocalLLMProvider implementing existing Provider interface, OpenAI-compatible HTTP endpoint

**Rationale**:
- **Reuse existing code**: internal/llm/provider.go interface already designed for abstraction
- **LM Studio compatibility**: Provides OpenAI-compatible `/v1/chat/completions` endpoint
- **Prompt caching**: LM Studio supports system prompt caching (static arbiter instructions, dynamic proposals)
- **Model flexibility**: User selects model via config (7B vs 14B, Q4_K_M vs Q5_K_M)

**Alternatives Considered**:
1. **Direct HTTP calls without provider abstraction**
   - Rejected: Breaks existing abstraction
   - Rejected: Complicates testing (can't mock LLM easily)

2. **Reuse OpenAI provider with endpoint override**
   - Considered: Simpler implementation
   - Rejected: Local models may have different parameter semantics (temperature, top_p ranges)
   - Rejected: Want explicit LocalLLMProvider type for logging clarity

**Implementation**:
```go
type LocalLLMProvider struct {
    endpoint string      // http://localhost:1234/v1
    model    string      // qwen2.5-7b-instruct-q4_k_m
    client   *http.Client
    systemPromptCache string // Cached arbiter instructions
}

func (p *LocalLLMProvider) GenerateResponse(ctx context.Context, prompt string) (string, error) {
    // POST to endpoint/chat/completions
    // System prompt: cached arbiter instructions
    // User message: graph JSON + fort metrics
}
```

**Configuration**:
```yaml
llm:
  provider_type: local  # "claude" | "local"
  local_endpoint: "http://localhost:1234/v1"
  local_model: "qwen2.5-7b-instruct-q4_k_m"
  timeout_ms: 5000
```

---

## Decision 3: Topological Sort Algorithm

**Question**: Which algorithm for dependency ordering?

**Decision**: Kahn's algorithm with cycle detection

**Rationale**:
- **Simple**: 20-30 lines of code, O(V+E) complexity
- **Stable**: Returns nodes in consistent order (priority-preserving)
- **Cycle detection**: Catches invalid dependencies (A requires B, B requires A)
- **Performance**: <10ms for 5-20 nodes (well under 50ms budget)

**Alternatives Considered**:
1. **DFS-based topological sort**
   - Rejected: More complex recursion
   - Rejected: Less intuitive cycle detection

2. **Priority heap without dependency ordering**
   - Rejected: Ignores dependencies (dig shaft after tunnel = failure)

**Implementation**:
```go
func TopologicalSort(nodes []ModificationNode, deps map[string][]string) ([]ModificationNode, error) {
    // Kahn's algorithm:
    // 1. Calculate in-degree for each node
    // 2. Queue all zero in-degree nodes
    // 3. Process queue: remove node, decrement successor in-degrees
    // 4. If all nodes processed: success. Else: cycle detected.
}
```

**Edge case**: Circular dependency (mining_shaft requires tunnel, tunnel requires shaft)
- **Handling**: Return error, log cycle nodes, skip conflicting proposals
- **Prevention**: Agents generate acyclic dependencies by design (depth-based dependencies only)

---

## Decision 4: Agent Priority Defaults

**Question**: What default priority values for 5 agents?

**Decision**: Food:10, Housing:9, Defense:Variable (0-10 based on threat), Mining:7, Wealth:6

**Rationale**:
- **Survival first**: Food highest priority (fort fails without food in days)
- **Comfort second**: Housing prevents unhappy dwarves (tantrum spirals)
- **Defense dynamic**: Escalates to 10 when enemies present, drops to 0 when safe
- **Economic growth**: Mining and wealth lower priority (optimize after survival secured)

**Research basis**:
- DF gameplay: Food shortage kills fort in 20-30 days
- Housing deficit causes unhappiness after 60-90 days
- Defense only critical during active threats (forgotten beasts, sieges)

**Configuration**:
```yaml
agents:
  food:
    enabled: true
    priority: 10
    target_per_dwarf: 20  # Food units
  housing:
    enabled: true
    priority: 9
    target_per_dwarf: 1   # Bedrooms
  defense:
    enabled: true
    priority_base: 0      # When no threats
    priority_threat: 10   # When enemies detected
  mining:
    enabled: true
    priority: 7
    tiles_per_cycle: 50
  wealth:
    enabled: true
    priority: 6
    growth_threshold: 0.10  # 10% per 10 cycles
```

---

## Decision 5: Synergy Recognition Patterns

**Question**: How should arbiter recognize spatial synergies?

**Decision**: Rule-based synergy templates + LLM reasoning

**Rationale**:
- **Common patterns**: bedroom_cluster + dining_hall → shared corridor (high frequency)
- **LLM flexibility**: Handles novel synergies (farm_plot + well → water_channel)
- **Hybrid approach**: Rules catch 80% of cases (fast), LLM catches edge cases

**Synergy Templates**:
1. **Shared corridor**: Any 2+ nodes with `requires_access_from` dependency at different locations
   - Inject: corridor_connector node linking both
   - Coordinates: Shortest Manhattan path between nodes

2. **Vertical connection**: Nodes at adjacent Z-levels both requiring stairs
   - Inject: stair_cluster node connecting Z-levels
   - Coordinates: Overlap XY region + span Z-range

3. **Resource proximity**: farm_plot + stockpile_food → adjacent placement
   - Modify: Adjust node coordinates to be within 5 tiles
   - Rationale: Reduces hauling time

**LLM Prompt**:
```text
Analyze these modification nodes for spatial synergies:
- bedroom_cluster (40,30,125) requires corridor access
- dining_hall (60,30,125) requires corridor access

Can these share infrastructure? If yes, propose connector node.
```

**Performance**: Rule-based patterns <5ms, LLM synergy check <100ms (only when patterns don't match)

---

## Decision 6: Conflict Resolution Strategy

**Question**: When 2+ nodes conflict spatially, how to resolve?

**Decision**: Priority-based selection with phase-aware tie-breaking

**Rationale**:
- **Primary**: Agent priority (food > housing > mining)
- **Secondary**: Urgency score (low food = urgent, adequate food = normal)
- **Tertiary**: Fort phase (embark phase favors food, fortify phase favors wealth)
- **Fallback**: Defer lower-priority node to next cycle

**Implementation**:
```go
func ResolveConflict(nodes []ModificationNode, phase FortPhase) ModificationNode {
    // 1. Filter to highest priority
    // 2. If tie: use urgency score
    // 3. If still tie: use phase preference
    // 4. Return winner, mark others as deferred
}
```

**Synergy override**: If conflict can become synergy (bedroom + workshop → shared corridor), prefer synergy injection over rejection

---

## Decision 7: Model Selection Guidelines

**Question**: Which Qwen2.5 variant and quantization?

**Decision**: Qwen2.5-7B-Instruct Q4_K_M default, Qwen2.5-14B-Instruct Q5_K_M optional

**Rationale**:
- **7B Q4_K_M**: ~4.5GB VRAM, 300-500ms inference on RTX 4070, sufficient reasoning for graph analysis
- **14B Q5_K_M**: ~8.5GB VRAM (tight fit), 800-1000ms inference, better complex synergy recognition
- **Instruct variant**: Fine-tuned for reasoning and decision-making (vs Coder for code generation)
- **K-quant**: Preserves more model capability than pure Q4_0

**Performance targets**:
- 7B: <500ms inference, 95% synergy recognition accuracy
- 14B: <1000ms inference, 98% synergy recognition accuracy

**Fallback**: If VRAM insufficient, system logs warning and suggests 7B model

---

## Decision 8: Graph Metadata Persistence

**Question**: How to persist graph nodes between sessions?

**Decision**: Extend existing modification persistence (modifications.json) with graph_metadata field

**Rationale**:
- **Reuse infrastructure**: internal/modifications/persistence.go already saves/loads modification overlay
- **Atomic operations**: Graph metadata saved alongside modifications (consistent state)
- **Minimal changes**: Add GraphMetadata struct to ModificationInfo

**Schema extension**:
```go
type ModificationInfo struct {
    Type      ModificationType
    Timestamp time.Time
    Source    string
    GraphNode *GraphMetadata  // NEW: Optional graph node metadata
}

type GraphMetadata struct {
    NodeType     string   // "bedroom_cluster", "mining_shaft"
    Dependencies []string // Node IDs this node depends on
    Status       string   // "pending", "in_progress", "completed"
}
```

**Persistence flow**:
1. Agent proposes node → Store in ProposalGraph
2. Arbiter approves node → Add to ModificationOverlay with GraphMetadata
3. Commands execute → Update GraphMetadata.Status = "completed"
4. Save triggered → Entire graph state written to modifications.json

---

## Decision 9: Agent Analysis Performance Budget

**Question**: How to ensure <100ms for all 5 agents combined?

**Decision**: Parallel agent execution with goroutines, pre-computed metric caching

**Rationale**:
- **Parallel execution**: 5 agents run concurrently (goroutine per agent)
- **Metric caching**: Fort metrics computed once per cycle, passed to all agents
- **Early exit**: Agent returns immediately if targets met (no proposal generation)

**Implementation**:
```go
func AnalyzeAllAgents(agents []GoalAgent, metrics FortMetrics) []ModificationNode {
    var wg sync.WaitGroup
    results := make(chan []ModificationNode, len(agents))

    for _, agent := range agents {
        wg.Add(1)
        go func(a GoalAgent) {
            defer wg.Done()
            proposals := a.Analyze(metrics)
            results <- proposals
        }(agent)
    }

    wg.Wait()
    close(results)

    // Collect all proposals
}
```

**Benchmark target**: 5 agents @ 20ms each = 100ms sequential, ~30ms parallel (5-way concurrency)

---

## Decision 10: Arbiter Prompt Template

**Question**: What prompt format maximizes graph reasoning quality?

**Decision**: System prompt with graph schema, user message with JSON proposals

**System prompt** (cached, ~400 tokens):
```text
You are a Dwarf Fortress fort manager coordinating 5 specialized agents.

Agents propose modification nodes with spatial dependencies and conflicts.
Your task: Perform topological sort, detect synergies, resolve conflicts.

Node types: bedroom_cluster, mining_shaft, farm_plot, workshop_zone,
            corridor_connector, seal_entrance, exploratory_tunnel, defensive_wall

Dependencies: requires_access_from (needs corridor), requires_water (needs well)
Conflicts: overlaps_spatially (same coordinates)

Output: Execution sequence (topologically sorted node list)

Recognize synergies: bedroom_cluster + dining_hall → inject corridor_connector
```

**User message** (dynamic, ~400-600 tokens):
```json
{
  "fort_metrics": {
    "dwarves": 7,
    "food_per_dwarf": 11.4,
    "bedrooms": 3,
    "wealth_growth": 0.05,
    "threats": 0
  },
  "proposals": [
    {
      "id": "food_1",
      "agent": "FoodSecurity",
      "type": "farm_plot",
      "region": {"x1": 40, "y1": 30, "z": 125, "x2": 50, "y2": 40},
      "dependencies": ["requires_access_from"],
      "conflicts": [],
      "priority": 10,
      "rationale": "Food at 11.4/dwarf, below 20 target"
    },
    {
      "id": "housing_1",
      "agent": "Housing",
      "type": "bedroom_cluster",
      "region": {"x1": 60, "y1": 30, "z": 125, "x2": 70, "y2": 40},
      "dependencies": ["requires_access_from"],
      "conflicts": [],
      "priority": 9,
      "rationale": "7 dwarves, only 3 bedrooms"
    }
  ]
}
```

**Expected output** (150-200 tokens):
```json
{
  "sequence": [
    {"id": "corridor_1", "type": "corridor_connector", "region": {...}, "rationale": "Shared access for farm and bedrooms"},
    {"id": "food_1", "rationale": "Higher priority, depends on corridor"},
    {"id": "housing_1", "rationale": "Parallel with food, shares corridor"}
  ],
  "synergies": ["corridor_1"],
  "deferred": []
}
```

**Total tokens**: 400 (system) + 500 (user) + 200 (completion) = 1100 tokens (within 1k budget with margin)

---

## Decision 11: Error Handling and Fallback

**Question**: What happens when local LLM fails or is unavailable?

**Decision**: Graceful degradation with retry, timeout, and fallback to direct commands

**Error scenarios**:
1. **LM Studio not running**: Timeout after 5s, log error, pause autonomous loop
2. **Model loading slow**: Increase timeout to 10s for first request
3. **Invalid JSON response**: Parse error → log response, use fallback heuristic
4. **Out of VRAM**: Log error, suggest 7B model, halt system

**Fallback heuristic** (when LLM unavailable):
- Select highest-priority non-conflicting proposal
- Skip synergy recognition
- Execute single proposal per cycle
- Log: "LLM unavailable, using fallback heuristic"

**Retry logic**: 1 retry with 2s delay, then fallback

---

## Decision 12: Testing Strategy

**Question**: How to test graph operations without running DF?

**Decision**: Three-tier testing: unit (graph ops), integration (mock agents), e2e (mock LLM)

**Unit tests** (tests/agents/graph_test.go):
- TopologicalSort with various dependency graphs
- Cycle detection (A→B→A)
- Conflict detection (spatial overlap)
- Edge cases: empty graph, single node, disconnected components

**Integration tests** (tests/agents/food_test.go):
- FoodSecurityAgent.Analyze with mock FortMetrics
- Verify proposal generation when food < target
- Verify no proposal when food > target

**E2E tests** (tests/integration/arbiter_test.go):
- Mock 5 agents, generate proposals
- Mock LLM response (JSON execution sequence)
- Verify command conversion
- Measure latency: agents + graph + arbiter < 2s

**Benchmark**: Graph operations with 20 nodes, measure <50ms

---

## Research Summary

All technical unknowns resolved. Key decisions:

1. **Architecture**: Graph-based proposals with dependencies/conflicts (not command lists)
2. **LLM Integration**: LocalLLMProvider with LM Studio OpenAI-compatible endpoint
3. **Algorithm**: Kahn's topological sort for dependency ordering
4. **Priorities**: Food:10, Housing:9, Defense:Variable, Mining:7, Wealth:6
5. **Synergies**: Rule-based templates + LLM reasoning
6. **Conflicts**: Priority-based resolution with phase-aware tie-breaking
7. **Model**: Qwen2.5-7B-Instruct Q4_K_M default (4.5GB VRAM, 300-500ms)
8. **Persistence**: Extend modifications.json with GraphMetadata
9. **Performance**: Parallel agent execution, <100ms for 5 agents
10. **Prompt**: System (400 tokens) + user JSON (500 tokens) = <1k total
11. **Errors**: Retry → timeout → fallback to heuristic
12. **Testing**: Unit + integration + e2e with mocks

**Ready for Phase 1**: Data model design and API contracts
