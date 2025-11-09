# Implementation Plan: Local LLM with Graph-Based Goal-Oriented Agents

**Branch**: `006-local-llm-agents` | **Date**: 2025-11-09 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/006-local-llm-agents/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Replace Claude API with local LM Studio inference, implementing goal-oriented architecture where five specialized agents (FoodSecurity, Housing, Mining, Wealth, Defense) propose graph-based modification nodes with spatial dependencies and conflicts. Local LLM arbiter performs topological sort and constraint satisfaction to generate coordinated execution sequences that maximize spatial synergies (shared corridors, multi-phase plans) and resource efficiency. Target: <2s cycles, <1k tokens, zero API costs, runs on RTX 4070 8GB.

## Technical Context

**Language/Version**: Go 1.21+ (server orchestrator)
**Primary Dependencies**:
- LM Studio HTTP client (OpenAI-compatible API endpoint)
- Existing packages: internal/modifications (ModificationOverlay), internal/llm (provider abstraction), internal/protocol, internal/autonomous
- No new external dependencies (Go stdlib only)

**Storage**:
- In-memory graph structures (ProposalGraph, ModificationNode)
- Existing modification persistence (saves/{fortname}/modifications.json)
- Graph metadata persisted alongside modification data

**Testing**: go test (unit tests for graph operations, topological sort, conflict detection)

**Target Platform**: Windows 10/11 (development), cross-platform Go binary

**Project Type**: Single project (orchestrator server)

**Performance Goals**:
- Full decision cycle: <2 seconds (agent analysis + graph assembly + LLM arbitration + command execution)
- Agent analysis phase: <100ms for all 5 agents combined
- Graph assembly + topological sort: <50ms for 5-20 nodes
- LLM inference: <500ms for 7B model, <1000ms for 14B model
- Token usage: <1000 tokens per arbiter prompt (5x reduction from Feature 005)

**Constraints**:
- VRAM budget: 8GB (RTX 4070) - single 7B Q4_K_M model (~4.5GB)
- Zero API costs (all inference local)
- Stateless agents (no persistent memory between cycles)
- Existing ModificationOverlay as graph substrate (no new data structures)

**Scale/Scope**:
- 5 specialized agents
- 5-20 graph nodes per cycle typical
- 10+ node types (bedroom_cluster, mining_shaft, farm_plot, etc.)
- 100-second autonomous decision cycles
- Target: 100+ in-game days autonomous survival

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principle I: Research-First Development ✅
- **Hypothesis**: Graph-based proposals enable spatial synergies (shared corridors) and temporal planning (multi-phase dependencies) that command lists cannot achieve
- **Experiment**: A/B test graph-based vs command-based coordination, measure synergy recognition rate (target: 50%+)
- **Rollback plan**: Config toggle enables fallback to Feature 005 direct-LLM mode (enable_goal_agents: false)
- **Research artifacts**: LLM interaction logs for fine-tuning, graph decision logs, synergy recognition metrics

### Principle II: Comprehensive Logging & Observability ✅
- **LLM interactions**: Full arbiter prompts (graph JSON), responses, timing, token usage logged to logs/llm-interactions.jsonl
- **Graph operations**: Node creation, dependency edges, conflict edges, topological sort results logged at DEBUG
- **Performance metrics**: Graph assembly latency, topological sort time, synergy recognition count
- **Game state snapshots**: Fort metrics, agent proposals, execution sequences at decision points
- **Structured format**: JSON logs for graph structures, agent proposals, arbiter decisions

### Principle III: Modularity & Pluggability ✅
- **Clear interfaces**: GoalAgent interface (Analyze() → []ModificationNode), GraphExecutor interface (Execute() → []Command)
- **Pluggable agents**: Registry-based agent system, enable/disable via config
- **LLM provider**: Reuse internal/llm/provider abstraction, local endpoint as new implementation
- **Mock testing**: Agent proposals can be generated without LLM, graph operations testable independently
- **Extension points**: New agent types, new node types, new conflict resolution strategies

### Principle IV: Lean & Efficient (Local LLM Compatible) ✅
- **Token budget**: <1000 tokens (graph JSON: nodes + edges vs 4000+ token fort state)
- **Memory efficient**: Graph stored as modification metadata in existing sparse ModificationOverlay
- **Minimal LLM calls**: 1 arbiter call per 100-second cycle (agents are deterministic Go code)
- **No dependencies**: Go stdlib only, reuse existing packages
- **Profile targets**: Graph assembly <50ms, topological sort <10ms

### Principle V: Iterative Experimentation ✅
- **Simplest version first**: 5 basic agents with fixed thresholds, single 7B model
- **Phase 0 (this feature)**: Graph-based coordination with deterministic agents
- **Phase 1 (future)**: Strategic slow-loop planner (14B model every 10 cycles)
- **Phase 2 (future)**: Fine-tuning agents based on logs
- **A/B testing**: Keep Feature 005 direct-LLM mode available via config toggle
- **Version tracking**: Git branches, experiment IDs in logs

### Principle VI: Metrics-Driven Evaluation ✅
- **Feature metrics**: Synergy recognition rate (50%+ target), dependency ordering correctness (100% target)
- **Game metrics**: Survival time, food stocks (15+ per dwarf), bedroom coverage (100% within 30 days)
- **Performance metrics**: Cycle latency (2s target), token usage (1000 target), VRAM usage (8GB budget)
- **Comparison**: Graph-based vs Feature 005 direct-LLM on same map seed
- **Reward signals**: Fort survival time, dwarf deaths, wealth growth for future RL

### Principle VII: Simplicity & Clarity ✅
- **Explicit types**: GoalAgent, ModificationNode, ProposalGraph, ArbitrationDecision structs
- **Small functions**: Analyze(), AssembleGraph(), TopologicalSort(), RecognizeSynergies(), Execute()
- **Clear naming**: bedroom_cluster (not "type A"), requires_access_from (not "dep1")
- **Minimal abstraction**: Graph is slice of nodes + edge maps, no complex graph library
- **Documentation**: Module-level docs explain graph substrate, topological sort algorithm, synergy patterns

**GATE STATUS**: ✅ PASSED - All principles satisfied, research-focused approach maintained

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/
├── agents/                    # NEW: Goal-oriented agent system
│   ├── agent.go              # GoalAgent interface, registry
│   ├── food.go               # FoodSecurityAgent implementation
│   ├── housing.go            # HousingAgent implementation
│   ├── mining.go             # MiningAgent implementation
│   ├── wealth.go             # WealthAgent implementation
│   ├── defense.go            # DefenseAgent implementation
│   └── graph.go              # ModificationNode, ProposalGraph, topological sort
├── llm/
│   ├── provider.go           # EXISTING: Provider interface
│   ├── openai.go             # EXISTING: OpenAI implementation
│   └── local.go              # NEW: LocalLLMProvider (LM Studio endpoint)
├── autonomous/
│   └── loop.go               # MODIFIED: Integration with agent system + arbiter
├── modifications/            # EXISTING: ModificationOverlay (graph substrate)
│   ├── overlay.go
│   └── persistence.go
├── config/
│   └── config.go             # MODIFIED: Agent config, LLM endpoint, model selection
└── [existing packages unchanged: protocol, dfhack, http, context, phases, spatial]

tests/
├── agents/
│   ├── food_test.go          # Unit tests for agent analysis logic
│   ├── housing_test.go
│   └── graph_test.go         # Topological sort, conflict detection tests
└── integration/
    └── arbiter_test.go       # End-to-end: agents → graph → arbiter → commands
```

**Structure Decision**: Single project (Go orchestrator server). New `internal/agents/` package contains goal agent implementations and graph operations. Existing packages minimally modified (autonomous loop integration, config schema, LLM provider addition). ModificationOverlay serves as graph substrate without structural changes.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

**No violations** - All constitution principles satisfied. Graph-based architecture adds complexity but justified by:
- Spatial synergies (shared corridors) impossible with command lists
- Temporal planning (multi-phase dependencies) without stateful agents
- Reuses existing ModificationOverlay infrastructure

Topological sort algorithm adds algorithmic complexity but:
- Standard graph algorithm (well-understood, 20 lines of code)
- Enables dependency ordering essential for multi-phase plans
- <10ms performance target ensures no impact on 2s cycle budget

---

## Phase 0 & 1 Completion Summary

**Phase 0: Research** ✅ COMPLETE
- Created: [research.md](research.md)
- Resolved: 12 technical decisions
- Key outputs:
  - Graph-based proposal architecture validated
  - LM Studio integration pattern defined
  - Topological sort algorithm selected (Kahn's)
  - Agent priorities established (Food:10, Housing:9, Defense:Variable, Mining:7, Wealth:6)
  - Synergy recognition patterns documented
  - Model selection: Qwen2.5-7B-Instruct Q4_K_M default
  - Arbiter prompt template designed (<1k tokens)

**Phase 1: Design** ✅ COMPLETE
- Created: [data-model.md](data-model.md)
- Entities defined: 6 core entities
  - GoalAgent (interface + 5 implementations)
  - ModificationNode (graph nodes with dependencies/conflicts)
  - ProposalGraph (node collection + edges)
  - ArbitrationDecision (arbiter output)
  - LocalLLMProvider (LM Studio client)
  - GraphExecutor (node → command converter)
- Configuration schema extended with 15+ agent settings
- Persistence schema extended with GraphMetadata

**Phase 1: Quickstart** ✅ COMPLETE
- Created: [quickstart.md](quickstart.md)
- Setup steps: 8 steps, 20-30 minute estimate
- Troubleshooting guide: 8 common issues
- Performance benchmarks documented

**Phase 1: Contracts** ✅ COMPLETE (N/A)
- Created: [contracts/README.md](contracts/README.md)
- Note: No new HTTP API contracts (internal Go interfaces only)
- LM Studio external API documented

**Phase 1: Agent Context** ✅ COMPLETE
- Updated: CLAUDE.md
- Added: Go 1.21+ (server orchestrator) technology

---

## Constitution Re-Check (Post-Design)

*Re-evaluating compliance after Phase 1 design decisions*

### Principle I: Research-First Development ✅
- Hypothesis clearly defined: graph-based enables synergies + temporal planning
- A/B testing planned: graph vs command-based on same map seed
- Rollback mechanism: config toggle to Feature 005 direct-LLM mode
- Research artifacts: 12 decisions documented in research.md

### Principle II: Comprehensive Logging & Observability ✅
- LLM interactions: Full prompts/responses in logs/llm-interactions.jsonl
- Graph operations: Node creation, edges, topological sort at DEBUG level
- Performance metrics: Agent latency, graph assembly time, token usage
- Structured JSON logs for all graph structures

### Principle III: Modularity & Pluggability ✅
- GoalAgent interface enables new agent types
- Registry-based agent system (enable/disable via config)
- LocalLLMProvider implements existing Provider interface
- GraphExecutor with pluggable node handlers

### Principle IV: Lean & Efficient (Local LLM Compatible) ✅
- Token budget: <1000 (system:400 + user:500 + completion:200)
- Memory efficient: Graph metadata in existing ModificationOverlay
- No new dependencies: Go stdlib only
- Performance targets: <2s total cycle, <100ms agents, <50ms graph ops

### Principle V: Iterative Experimentation ✅
- Simplest version: Deterministic agents, rule-based synergies
- Future experiments: Strategic planner, fine-tuning, concern clustering
- A/B testing: Config toggle for graph vs direct-LLM comparison

### Principle VI: Metrics-Driven Evaluation ✅
- Feature metrics: Synergy rate (50%+ target), dependency ordering (100%)
- Game metrics: Survival time, food stocks, bedroom coverage
- Performance metrics: Cycle latency, token usage, VRAM
- Comparison framework: Same map seed, different strategies

### Principle VII: Simplicity & Clarity ✅
- Explicit types: GoalAgent, ModificationNode, ProposalGraph
- Small functions: Analyze(), TopologicalSort(), RecognizeSynergies()
- Clear naming: bedroom_cluster, requires_access_from
- Minimal abstraction: Graph is slices + maps, no external library

**POST-DESIGN GATE STATUS**: ✅ PASSED - All principles maintained through design phase

---

## Next Steps

**Ready for**: `/speckit.tasks` to generate task breakdown

**Implementation sequence**:
1. Create internal/agents/ package with GoalAgent interface
2. Implement 5 agent types (food, housing, mining, wealth, defense)
3. Implement graph operations (ModificationNode, ProposalGraph, TopologicalSort)
4. Create internal/llm/local.go (LocalLLMProvider)
5. Update internal/config/config.go with agent settings
6. Integrate agents into internal/autonomous/loop.go
7. Implement GraphExecutor (node → command conversion)
8. Add unit tests (graph ops, topological sort, conflict detection)
9. Add integration tests (agents → graph → arbiter → commands)
10. Update CLAUDE.md usage examples

**Estimated effort**: 8-12 hours implementation + 4-6 hours testing

---

**Planning Phase Complete**: 2025-11-09

All Phase 0 and Phase 1 artifacts generated. Constitution compliance verified. Ready for task generation and implementation.
