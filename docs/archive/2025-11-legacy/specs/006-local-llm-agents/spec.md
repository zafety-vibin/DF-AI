# Feature Specification: Local LLM with Goal-Oriented Agent Architecture

**Feature Branch**: `006-local-llm-agents`
**Created**: 2025-11-09
**Status**: Draft
**Input**: User description: "Feature 006: Local LLM with Goal-Oriented Agents Replace Claude API with local LM Studio. Use goal-oriented architecture: 5 specialized Python agents (FoodSecurity, Housing, Mining, Wealth, Defense) monitor metrics and propose actions. Single local LLM arbiter (Qwen2.5-7B/14B Q4_K_M) receives proposals, resolves conflicts, and issues final commands. Target: <2s cycles, <1k tokens, zero API costs, runs on RTX 4070 8GB"

## User Scenarios & Testing

### User Story 1 - Goal Agents Maintain Fort Health (Priority: P1)

The system continuously monitors critical fort metrics (food stocks, housing availability, mining activity, wealth generation, defense status) through specialized agents. When any metric falls below target thresholds, the responsible agent immediately proposes corrective actions to restore fort health.

**Why this priority**: Core survival functionality. Without active monitoring and response to food/housing needs, the fort fails within days. This provides the foundational autonomous behavior that keeps dwarves alive and productive.

**Independent Test**: Can be fully tested by observing an unmanaged fort for 30 in-game days and verifying: food stocks remain above 15 units per dwarf, all dwarves have bedroom assignments, no starvation deaths occur, and mining continues producing resources. Delivers immediate autonomous survival value.

**Acceptance Scenarios**:

1. **Given** fort has 7 dwarves and 80 food units (11.4 per dwarf, below 20/dwarf target), **When** FoodSecurityAgent evaluates fort state, **Then** agent generates proposal to gather surface plants or dig underground farm
2. **Given** fort has 7 dwarves but only 3 designated bedrooms, **When** HousingAgent evaluates fort state, **Then** agent generates proposal to dig 4 additional bedrooms with specific coordinates avoiding hazards
3. **Given** fort has experienced no ore discoveries in past 20 decision cycles, **When** MiningAgent evaluates mining progress, **Then** agent generates proposal for exploratory mining shaft at depth predicted to contain ores
4. **Given** fort wealth has grown less than 10% over past 10 cycles, **When** WealthAgent evaluates economic metrics, **Then** agent generates proposal to construct missing workshops or optimize production chains
5. **Given** enemy entities detected within fort boundaries, **When** DefenseAgent evaluates threat status, **Then** agent immediately escalates proposal priority to maximum and suggests defensive measures

---

### User Story 2 - LLM Arbiter Coordinates Graph-Based Proposals (Priority: P1)

When multiple goal agents simultaneously propose modification nodes (bedroom_cluster, mining_shaft, farm_plot), the local LLM arbiter receives graph proposals with dependencies and conflicts, performs topological sorting and constraint satisfaction, and produces a coordinated execution sequence that maximizes spatial synergies (shared corridors, connected areas) and multi-phase planning (dig shaft → expand bedrooms → connect with stairs).

**Why this priority**: Essential for intelligent multi-agent coordination. Graph-based proposals enable spatial reasoning that command lists cannot: agents propose "bedroom cluster" instead of "dig 5×5 rectangle", arbiter can recognize shared corridor opportunities, dependency chains enable multi-turn plans. Required for P1 agents to function effectively together.

**Independent Test**: Can be fully tested by intentionally triggering low food + low housing simultaneously, verifying arbiter recognizes bedroom_cluster and farm_plot can share an access corridor, and produces execution sequence that digs shared corridor first (dependency satisfaction). Delivers coordinated multi-goal optimization with spatial synergies.

**Acceptance Scenarios**:

1. **Given** FoodAgent proposes farm_plot node at (40,30,125) and HousingAgent proposes bedroom_cluster at (45,30,125) with both requiring corridor access, **When** arbiter evaluates graph, **Then** arbiter recognizes shared corridor opportunity and adds corridor_connector node linking both
2. **Given** three agents propose six total modification nodes with dependencies (mining_shaft → exploratory_tunnel → ore_stockpile), **When** arbiter performs topological sort, **Then** arbiter produces execution sequence respecting dependency order (dig shaft before tunnel)
3. **Given** DefenseAgent proposes emergency seal_cavern node (priority 10) while FoodAgent and MiningAgent have medium-priority proposals, **When** arbiter evaluates graph, **Then** emergency node executes first regardless of dependencies
4. **Given** proposals create dependency chain requiring 3+ turns to complete, **When** arbiter evaluates workload, **Then** arbiter produces partial execution plan for current cycle and preserves remaining nodes for future cycles
5. **Given** two agents propose spatially adjacent nodes (entrance_hall + stair_cluster in corner), **When** arbiter evaluates proposals, **Then** arbiter recognizes spatial synergy and coordinates placement for optimal connection

---

### User Story 3 - Local LLM Inference Replaces Cloud API (Priority: P1)

The system establishes connection to locally-running LM Studio instance, sends arbiter decision prompts via local HTTP endpoint, and receives strategic guidance without requiring internet connectivity or incurring API costs.

**Why this priority**: Core infrastructure change. Replaces paid Claude API ($0.02/hour) with free local inference. Enables unlimited autonomous operation and experimentation without cost concerns.

**Independent Test**: Can be fully tested by starting LM Studio with Qwen model, connecting orchestrator, issuing arbiter prompt, and measuring response time + validating zero API charges. Delivers cost-free autonomous decision-making.

**Acceptance Scenarios**:

1. **Given** LM Studio running Qwen2.5-7B-Instruct on localhost:1234, **When** orchestrator starts and attempts connection, **Then** connection establishes successfully and logs model confirmation
2. **Given** arbiter has agent proposals ready, **When** arbiter prompt sent to local endpoint, **Then** response received in under 500ms for 7B model or under 1000ms for 14B model
3. **Given** system prompt contains 800 tokens of instructions, **When** first arbiter request sent, **Then** LM Studio caches system prompt and subsequent requests only send new context
4. **Given** agent proposals formatted as 400-token JSON, **When** arbiter makes decision, **Then** total prompt size remains under 1000 tokens
5. **Given** local LM Studio crashes or becomes unresponsive, **When** connection attempt fails, **Then** system logs error, pauses autonomous loop gracefully, and maintains HTTP monitoring availability

---

### User Story 4 - Configurable Agent Behavior (Priority: P2)

Users adjust goal agent target thresholds, enable/disable specific agents, and tune agent priorities through configuration file, allowing customization for different fort strategies, hardware limitations, or experimental gameplay.

**Why this priority**: Enables optimization and experimentation. Not critical for basic autonomous function but important for adapting system to user preferences and hardware constraints.

**Independent Test**: Can be fully tested by setting custom thresholds in config (food_target: 30), restarting system, and verifying agent triggers at new threshold instead of default. Delivers customization capability.

**Acceptance Scenarios**:

1. **Given** configuration sets `enable_wealth_agent: false`, **When** system initializes agents, **Then** WealthAgent doesn't load and doesn't generate proposals during operation
2. **Given** configuration sets `food_target_per_dwarf: 30` instead of default 20, **When** FoodAgent evaluates 25 food/dwarf, **Then** agent triggers and proposes food-gathering actions (above default but below custom threshold)
3. **Given** configuration overrides agent priorities (housing:10, food:9, mining:7, wealth:6, defense:variable), **When** arbiter resolves equal-urgency proposals, **Then** arbiter uses configured priority ordering for selection
4. **Given** configuration sets `enable_goal_agents: false`, **When** system runs, **Then** system falls back to direct LLM mode (Feature 005 behavior) without agent preprocessing

---

### User Story 5 - Arbiter Handles Emergencies and Novel Events (Priority: P2)

When fort encounters situations outside normal agent coverage (forgotten beasts, strange moods, cave-ins, floods), the arbiter LLM applies reasoning to generate appropriate emergency responses without pre-programmed handlers.

**Why this priority**: Handles rare but important events. Goal agents cover routine operations (90% of gameplay), arbiter fills critical gaps for unusual situations. Important for long-term fort survival.

**Independent Test**: Can be fully tested by triggering special game events (spawn forgotten beast, breach aquifer) and verifying arbiter generates contextually appropriate emergency commands. Delivers robustness to unexpected events.

**Acceptance Scenarios**:

1. **Given** forgotten beast entity appears in cavern layers (not covered by DefenseAgent normal logic), **When** arbiter receives fort state showing beast presence, **Then** arbiter generates emergency commands (seal cavern access, mobilize military, avoid beast location)
2. **Given** aquifer breach causes rapid water spread (50+ water tiles added in one cycle), **When** arbiter detects catastrophic water increase, **Then** arbiter proposes emergency flood response (construct walls at breach point, dig drainage channels)
3. **Given** dwarf enters strange mood requiring specific materials, **When** arbiter sees mood notification in fort state, **Then** arbiter adjusts commands to prioritize material gathering for mood completion
4. **Given** all goal agent targets are met (food adequate, housing sufficient, wealth growing), **When** arbiter receives proposals showing fort is stable, **Then** arbiter either issues strategic exploration commands or waits for next cycle

---

### User Story 6 - Fast Decision Cycles Enable Responsive Management (Priority: P2)

The system completes full autonomous decision cycles (agent analysis + arbiter decision + command execution) in under 2 seconds, enabling frequent fort adjustments and rapid response to changing conditions.

**Why this priority**: Quality of life improvement. Fast decisions mean AI responds quickly to problems and adapts strategies fluidly. Not critical for survival but significantly improves gameplay experience.

**Independent Test**: Can be measured by running 100 consecutive decision cycles, timing each, and verifying 95%+ complete within 2-second target. Delivers responsive AI behavior.

**Acceptance Scenarios**:

1. **Given** all 5 goal agents active, **When** agents analyze current fort state, **Then** complete analysis finishes in under 100ms total
2. **Given** agent proposals ready for arbitration, **When** arbiter LLM processes proposals and generates commands, **Then** inference completes in under 500ms
3. **Given** arbiter issues command list, **When** full cycle measured from start to command acknowledgment, **Then** total elapsed time is under 2 seconds
4. **Given** system using 14B model instead of 7B, **When** arbiter inference runs, **Then** latency remains under 1000ms (acceptable degradation for better quality)

---

### Edge Cases

- **What happens when all 5 agents propose conflicting actions in same spatial area?**
  - Arbiter receives 5 nodes with conflict edges marked
  - Arbiter selects highest-priority node based on agent priority + urgency
  - Rejected nodes deferred to next cycle with conflict metadata preserved
  - Arbiter may recognize synergy opportunity: "bedroom_cluster at (40,30) conflicts with workshop_zone at (42,30), but both need corridor access → inject shared corridor at (45,30)"
  - Logs conflict resolution decision and synergy recognition for debugging

- **What happens when agents propose complementary nodes that should share infrastructure?**
  - Example: bedroom_cluster at (40,30) and dining_hall at (60,30) both need access
  - Arbiter recognizes spatial proximity and common dependency (requires_corridor_access)
  - Arbiter injects corridor_connector node linking both with coordinates calculated from node positions
  - Topological sort ensures corridor digs first, then bedroom and dining nodes execute in parallel
  - Demonstrates spatial reasoning that command-based proposals cannot achieve

- **What happens when local LLM becomes unresponsive or crashes?**
  - System detects timeout after 5 seconds
  - Logs error with connection details
  - Pauses autonomous loop (doesn't spam failed requests)
  - HTTP monitoring remains available
  - Resumes automatically when LM Studio reconnects

- **What happens when agent target thresholds conflict with immediate survival?**
  - Example: WealthAgent wants workshops but food critically low (<5 per dwarf)
  - Arbiter detects critical survival alerts
  - Overrides configured priorities to prioritize survival
  - Food/housing always supersede wealth/optimization in crisis

- **What happens when fort is thriving (all agent targets exceeded)?**
  - Agents return empty or low-priority proposals
  - Arbiter generates strategic/exploratory commands (breach caverns, establish trade depot)
  - Or arbiter issues "wait" to conserve resources
  - System doesn't create busywork when fort is stable

- **What happens when RTX 4070's 8GB VRAM is insufficient for 14B model?**
  - System logs VRAM allocation warnings
  - Falls back to 7B model automatically if configured
  - If no fallback configured, logs error and suggests 7B alternative
  - System doesn't crash, gracefully reports hardware limitations

- **What happens when agent proposals total 20+ commands?**
  - Arbiter evaluates available dwarf labor (typically 7 dwarves)
  - Limits command output to realistic capacity (5-7 commands per cycle)
  - Prioritizes by agent priority and urgency
  - Defers lower-priority commands to future cycles
  - Prevents dwarf over-allocation and task queue overflow

- **What happens when two agents need same resource (e.g., stone for building vs trade)?**
  - Arbiter evaluates which use provides more value to fort
  - Prioritizes based on fort phase and current deficits
  - May split resource if sufficient quantity available
  - Logs resource allocation decision

## Requirements

### Functional Requirements

**Goal Agent System**:

- **FR-001**: System MUST implement five specialized goal agents (FoodSecurity, Housing, Mining, Wealth, Defense)
- **FR-002**: FoodSecurityAgent MUST monitor food stocks and drink stocks, proposing farm_plot or gather_zone modification nodes when stocks fall below target (20 food, 15 drink per dwarf)
- **FR-003**: HousingAgent MUST monitor bedroom count versus dwarf count, proposing bedroom_cluster modification nodes when deficit exists (target: 1 bedroom per dwarf minimum)
- **FR-004**: MiningAgent MUST monitor mining activity rate and ore discovery frequency, proposing mining_shaft or exploratory_tunnel nodes when activity drops below 50 tiles/cycle or no strikes in 20 cycles
- **FR-005**: WealthAgent MUST monitor fort value growth rate, proposing workshop_zone or production_area nodes when growth stagnates (<10% per 10 cycles)
- **FR-006**: DefenseAgent MUST monitor enemy presence, escalating priority to maximum when threats detected and proposing seal_entrance or defensive_wall nodes
- **FR-007**: Each agent MUST output graph-based modification proposals containing: node type (bedroom_cluster, mining_shaft, etc.), spatial region, dependencies (requires_access_from, requires_water), conflicts (overlaps_spatially), priority level (1-10), and rationale
- **FR-008**: Agents MUST use existing data overlays (modifications, entities, hazards, topology, phase) without requiring new data sources
- **FR-009**: Agents MUST be stateless, recalculating graph proposals from current fort state each cycle without persistent memory

**LLM Arbiter Integration**:

- **FR-010**: System MUST connect to local LM Studio instance via configurable HTTP endpoint (default: localhost:1234/v1)
- **FR-011**: Arbiter MUST receive graph-based modification proposals from all enabled goal agents each decision cycle
- **FR-012**: Arbiter MUST perform topological sort on proposal graph to identify dependency chains and execution order
- **FR-013**: Arbiter MUST detect spatial conflicts where multiple nodes overlap coordinates and resolve via priority or synergy recognition
- **FR-014**: Arbiter MUST detect resource conflicts where proposals would over-allocate available dwarf labor
- **FR-015**: Arbiter MUST generate final execution sequence by converting prioritized graph nodes to DFHack commands (dig, zone, build)
- **FR-016**: Arbiter MUST recognize spatial synergies (bedroom_cluster + dining_hall can share corridor) and inject connector nodes when beneficial
- **FR-017**: Arbiter MUST handle novel situations not covered by goal agents by generating ad-hoc modification nodes with appropriate dependencies
- **FR-018**: Arbiter prompts MUST remain under 1000 tokens by sending graph structure (nodes, edges, priorities) instead of complete fort state
- **FR-019**: Arbiter MUST complete inference in under 500ms for 7B models or under 1000ms for 14B models

**Configuration and Flexibility**:

- **FR-020**: Users MUST be able to enable or disable individual goal agents via configuration
- **FR-021**: Users MUST be able to configure target thresholds for each agent (food_target_per_dwarf, mining_tiles_per_cycle, etc.)
- **FR-022**: Users MUST be able to select LLM model (7B vs 14B variant, quantization level)
- **FR-023**: Users MUST be able to override default agent priorities through configuration
- **FR-024**: System MUST support fallback to direct-LLM mode (Feature 005 behavior) when goal_agents disabled

**Performance and Resource Management**:

- **FR-025**: Full decision cycle MUST complete in under 2 seconds (agent analysis + graph assembly + LLM arbitration + command execution)
- **FR-026**: System MUST operate within 8GB VRAM budget when using recommended 7B model configuration
- **FR-027**: System MUST generate zero API costs (all inference performed locally)
- **FR-028**: Agent analysis phase MUST complete in under 100ms for all 5 agents combined
- **FR-029**: Graph assembly and topological sort MUST complete in under 50ms for typical proposal sets (5-20 nodes)

**Integration with Existing Systems**:

- **FR-030**: System MUST reuse existing DFHack protocol without modification
- **FR-031**: System MUST utilize existing ModificationOverlay as graph substrate (nodes stored as modification metadata)
- **FR-032**: System MUST utilize existing data overlays (topology, hazards, entities) without creating new collection systems
- **FR-033**: System MUST incorporate phase context (embark/establish/expand/fortify) into agent behavior and arbiter decisions
- **FR-034**: HousingAgent MUST utilize existing room type detection for intelligent bedroom_cluster node sizing
- **FR-035**: Agents MUST be able to propose blueprint_application nodes when templates match current needs
- **FR-036**: System MUST continue using existing modification persistence for save/load functionality (including graph metadata)

### Key Entities

- **GoalAgent**: Represents a specialized monitoring component that tracks specific fort metrics and generates graph-based modification proposals when targets not met. Attributes: agent name, priority rating (1-10), monitored metrics list, target threshold values, enabled/disabled status, proposal generation logic.

- **ModificationNode**: Represents a spatial modification proposal in the graph. Attributes: node type (bedroom_cluster, mining_shaft, farm_plot, workshop_zone, corridor_connector, seal_entrance, exploratory_tunnel, defensive_wall), spatial region (bounding box), dependencies (array of node IDs that must complete first), conflicts (array of node IDs with spatial overlap), priority level, originating agent, rationale text, estimated tile count.

- **ProposalGraph**: Represents collection of modification nodes with dependency and conflict edges. Attributes: nodes (array of ModificationNode), dependency edges (directed, represents "requires" relationships), conflict edges (undirected, represents spatial overlap), topological ordering (cached sort result), agent metadata (which agent proposed which nodes).

- **ArbitrationDecision**: Represents final coordinated decision from LLM arbiter after graph analysis. Attributes: execution sequence (topologically sorted node list), command list (converted DFHack commands), rejected nodes with rejection reasons, recognized synergies (injected connector nodes), spatial allocation map, dwarf labor allocation, decision timestamp.

- **LocalLLMProvider**: Represents connection to local inference server. Attributes: endpoint URL, model identifier, context window capacity, quantization level, connection health status, average latency, system prompt cache status.

- **GraphExecutor**: Represents component that converts graph nodes to DFHack commands. Attributes: node type to command mapping, current execution phase (which nodes in progress), completion tracking, error handling state.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Fort survives 100+ in-game days without human intervention when managed solely by goal-agent system
- **SC-002**: Food stocks maintained above 15 units per dwarf for 95%+ of decision cycles across multiple test forts
- **SC-003**: All dwarves receive bedroom assignments within 30 in-game days of fort establishment
- **SC-004**: Fort wealth demonstrates positive growth trend over 50+ day observation period
- **SC-005**: Zero catastrophic failures (total fort wipeout, mass starvation) occur in 10 independent test games of 100+ days each
- **SC-006**: Decision cycle latency remains under 2 seconds for 95%+ of all cycles
- **SC-007**: Arbiter prompt token usage averages below 1000 tokens per cycle (5x reduction from Feature 005's 4000+ tokens)
- **SC-008**: System incurs zero API costs over unlimited runtime (all inference local)
- **SC-009**: System operates continuously for 8+ hours on RTX 4070 8GB hardware without out-of-memory errors
- **SC-010**: Agent proposals are accepted by arbiter at 80%+ rate (low rejection indicates good proposal quality)
- **SC-011**: Spatial conflicts detected and resolved correctly in 100% of cases (zero overlapping dig commands executed)
- **SC-012**: Multi-agent cycles produce 3-5 coordinated commands on average (demonstrates effective multi-goal coordination)
- **SC-013**: Arbiter recognizes spatial synergies in 50%+ of cases where bedroom_cluster and dining_hall/workshop nodes are proposed simultaneously (shared corridor injection)
- **SC-014**: Dependency chains execute in correct topological order in 100% of multi-phase plans (mining_shaft completes before exploratory_tunnel)
- **SC-015**: Graph assembly and topological sort complete in under 50ms for typical proposal sets (contributes to 2s cycle budget)

## Assumptions

- User has LM Studio installed and configured locally
- User has downloaded Qwen2.5-7B-Instruct or Qwen2.5-14B-Instruct model with Q4_K_M or Q5_K_M quantization
- User's hardware provides 8GB+ VRAM (RTX 4070, RTX 3080, or equivalent)
- User has 16GB+ system RAM available
- Feature 005 (LLM integration + architectural improvements) is fully implemented and operational
- DFHack plugin and binary protocol remain unchanged
- LM Studio provides OpenAI-compatible API endpoint
- Existing OpenAI provider (llm/openai.go) can be reused for local endpoint with minimal modification
- LM Studio supports system prompt caching
- Goal agents implemented in Go to match existing codebase (not Python as initially suggested)
- Agent decision logic uses deterministic rules, not machine learning
- Arbiter prompt engineering sufficient for coordination (no fine-tuning required initially)

## Dependencies

**From Feature 005**:
- Modification overlay system (sparse storage of fort changes)
- Entity cache (dwarf, enemy, animal position tracking)
- Hazard overlay system (aquifer, water, lava, cavern, enemy detection)
- Phase management system (embark/establish/expand/fortify lifecycle)
- Room type detection (bedroom, corridor, workshop, dining hall inference)
- Blueprint system (CSV dig pattern templates)
- Autonomous loop infrastructure (100-second decision cycles)
- Command execution system (dig with 6 types, chop, gather)
- Multi-command parsing and execution
- Persistence system (modification save/load to disk)

**External Dependencies**:
- LM Studio application installed and running
- Qwen2.5-Instruct model (7B or 14B variant) downloaded and loaded
- HTTP endpoint accessible at configured address (typically localhost:1234/v1)
- Sufficient VRAM for model inference

## Out of Scope

**Not Included in This Feature**:
- Claude API hybrid mode implementation (config schema support only, no actual hybrid decision logic)
- Multi-model pipeline architecture (separate analyzer and executor models)
- Fine-tuning or LoRA adaptation of local models
- Agent learning or adaptive threshholds (agents use fixed configured targets)
- Reinforcement learning from fort outcomes
- Reward signal calculation and optimization
- Heatmap visualization of agent decisions
- Blueprint auto-generation from successful forts
- Agent memory or historical pattern recognition
- DFHack plugin modifications
- New binary protocol messages
- Additional data collection beyond Feature 005 overlays
- Python implementation (Go implementation to match codebase)

## Open Questions

None. Architectural approach (goal-oriented agents + single arbiter) selected based on:
- Hardware constraints (8GB VRAM limits model size)
- DF's metric-driven gameplay (food stocks, wealth, population are measurable goals)
- Token efficiency needs (agents compress analysis, arbiter just coordinates)
- Robustness requirements (agents continue functioning if LLM slow/unavailable)
- User preference expressed: goal-oriented over dual-model architecture

## Notes

**Architectural Rationale**:

Goal-oriented agent architecture with graph-based proposals chosen over dual-model (analyzer + executor) or command-based approaches because:

1. **Efficiency**: Agents perform deterministic metric checks in <1ms, LLM only needed for graph analysis and coordination decisions
2. **Token Reduction**: Agents compress fort analysis to compact graph structure (nodes + edges), arbiter receives ~400-600 tokens instead of 4000+ token fort state
3. **Spatial Synergies**: Graph structure enables recognition of shared corridors, connected areas, and multi-room complexes that command lists cannot represent (bedroom_cluster + dining_hall sharing access corridor)
4. **Temporal Planning**: Dependency edges enable multi-phase plans (dig_shaft → expand_bedrooms → add_well) without requiring agents to maintain state across cycles
5. **Existing Infrastructure**: ModificationOverlay already provides graph substrate, agents speak the language the system understands instead of inventing new command formats
6. **Robustness**: Agents continue monitoring even if LLM temporarily unavailable, graph nodes queue for next arbitration
7. **DF Alignment**: Dwarf Fortress spatial design is inherently graph-structured (rooms connected by corridors, zones with access requirements), graph proposals match fort planning mental model
8. **Hardware Fit**: Single 7B model for graph analysis fits comfortably in 8GB VRAM, dual-model would risk OOM

**Model Selection Rationale**:

- **Qwen2.5-Instruct over Qwen2.5-Coder**: Instruct variant optimized for reasoning and decision-making, Coder variant for code generation
- **7B as primary, 14B as optional**: 7B model (Q4_K_M ~4.5GB) provides safety margin in 8GB VRAM, 14B (Q4_K_M ~8.5GB) possible but tight
- **Q4_K_M quantization**: Balances inference speed with response quality, K-quant methods preserve more capabilities than pure Q4_0
- **Instruct variants only**: Base models require extensive prompting, instruct-tuned models follow instructions reliably

**Integration Philosophy**:

Minimal changes to Feature 005 codebase:
- Goal agents added as new package (internal/agents/)
- Arbiter replaces direct LLM calls in autonomous loop
- All existing overlays, protocol, and infrastructure unchanged
- Config toggle allows switching between goal-agent mode and direct-LLM mode
- Backward compatible: disabling goal agents reverts to Feature 005 behavior

**From Session Context**:
- User explicitly chose goal-oriented over dual-model after analysis
- User wants zero Claude integration initially (pure local operation)
- User emphasized blueprints as learning tools, not rigid templates
- User validated Feature 005 works (dig commands execute, multi-command works, token efficiency improved)
- User satisfied with current progress, ready for local LLM transition
