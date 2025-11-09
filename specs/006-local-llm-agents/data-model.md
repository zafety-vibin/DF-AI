# Data Model: Local LLM with Graph-Based Goal-Oriented Agents

**Feature**: 006-local-llm-agents
**Date**: 2025-11-09
**Phase**: 1 (Design)

## Overview

This document defines the core entities, their relationships, validation rules, and state transitions for the graph-based goal-oriented agent system.

---

## Entity: GoalAgent

**Purpose**: Interface for specialized fort monitoring agents that analyze metrics and generate graph-based modification proposals.

### Fields

```go
type GoalAgent interface {
    // Name returns agent identifier (e.g., "FoodSecurity", "Housing")
    Name() string

    // Priority returns base priority level (1-10)
    Priority() int

    // Analyze examines fort metrics and returns modification node proposals
    // Returns empty slice if all targets met
    Analyze(metrics FortMetrics) []ModificationNode

    // Enabled returns whether agent is active
    Enabled() bool
}

type FortMetrics struct {
    DwarfCount     int
    FoodPerDwarf   float64
    DrinkPerDwarf  float64
    BedroomCount   int
    MiningTilesPerCycle int
    WealthGrowthRate float64  // Percentage over last 10 cycles
    EnemyCount     int
    FortAge        int         // Days elapsed
    Phase          FortPhase   // Embark/Establish/Expand/Fortify
}
```

### Validation Rules

- Name MUST be non-empty and unique within agent registry
- Priority MUST be in range [0, 10]
- Analyze() MUST return within 20ms (contributes to 100ms budget for 5 agents)
- Analyze() MUST return deterministic results for same FortMetrics input

### Relationships

- **Produces**: 0 or more ModificationNode entities
- **Consumes**: FortMetrics (computed once per cycle)
- **Registered in**: AgentRegistry (map[string]GoalAgent)

### State Transitions

Agents are stateless - no state transitions. Each cycle:
1. Receive FortMetrics
2. Compute proposals (pure function)
3. Return ModificationNode slice

---

## Entity: ModificationNode

**Purpose**: Represents a spatial modification proposal in the proposal graph, with dependencies and conflicts.

### Fields

```go
type ModificationNode struct {
    ID          string              // Unique identifier (e.g., "food_1", "housing_2")
    AgentName   string              // Originating agent
    Type        NodeType            // bedroom_cluster, mining_shaft, etc.
    Region      Region              // Spatial bounding box
    Dependencies []DependencyType   // Requires access, water, etc.
    Conflicts   []string            // Node IDs with spatial overlap
    Priority    int                 // Agent priority (1-10)
    Urgency     float64             // Urgency score (0.0-1.0)
    Rationale   string              // Human-readable explanation
    Metadata    map[string]interface{} // Type-specific data
}

type NodeType string
const (
    NodeTypeBedroom      NodeType = "bedroom_cluster"
    NodeTypeMiningShaft  NodeType = "mining_shaft"
    NodeTypeFarmPlot     NodeType = "farm_plot"
    NodeTypeWorkshop     NodeType = "workshop_zone"
    NodeTypeCorridor     NodeType = "corridor_connector"
    NodeTypeSealEntrance NodeType = "seal_entrance"
    NodeTypeExploratory  NodeType = "exploratory_tunnel"
    NodeTypeDefensive    NodeType = "defensive_wall"
    NodeTypeStairCluster NodeType = "stair_cluster"
    NodeTypeStockpile    NodeType = "stockpile_zone"
)

type DependencyType string
const (
    DependencyAccess DependencyType = "requires_access_from"
    DependencyWater  DependencyType = "requires_water"
    DependencyStairs DependencyType = "requires_stairs"
    DependencyPower  DependencyType = "requires_power"
)

type Region struct {
    X1, Y1, Z int16
    X2, Y2    int16  // Z is single level for most nodes
}
```

### Validation Rules

- ID MUST be unique within ProposalGraph
- Type MUST be one of defined NodeType constants
- Region MUST have X1 <= X2, Y1 <= Y2, Z in range [0, 200]
- Priority MUST be in range [1, 10]
- Urgency MUST be in range [0.0, 1.0]
- Dependencies MUST NOT include own ID (no self-dependency)
- Conflicts array MAY be empty (no spatial conflicts)
- Rationale SHOULD be non-empty for logging/debugging

### Relationships

- **Created by**: GoalAgent via Analyze()
- **Contained in**: ProposalGraph
- **Depends on**: 0 or more other ModificationNode entities (dependency edges)
- **Conflicts with**: 0 or more other ModificationNode entities (conflict edges)
- **Persisted as**: ModificationInfo.GraphMetadata when approved

### State Transitions

```
[proposed] --> Arbiter analysis --> [approved] --> Command execution --> [completed]
                                 |
                                 +--> [deferred] (conflicts or low priority)
                                 |
                                 +--> [rejected] (invalid or superseded)
```

**State field** (added during arbiter processing):
```go
type NodeStatus string
const (
    StatusProposed NodeStatus = "proposed"  // Initial state from agent
    StatusApproved NodeStatus = "approved"  // Arbiter selected for execution
    StatusDeferred NodeStatus = "deferred"  // Delayed to next cycle
    StatusRejected NodeStatus = "rejected"  // Arbiter discarded
    StatusInProgress NodeStatus = "in_progress" // Commands executing
    StatusCompleted NodeStatus = "completed"    // Commands finished
)
```

---

## Entity: ProposalGraph

**Purpose**: Collection of modification nodes with dependency and conflict edges, representing the current cycle's proposals from all agents.

### Fields

```go
type ProposalGraph struct {
    Nodes           []ModificationNode
    DependencyEdges map[string][]string  // NodeID -> []DependsOnNodeID
    ConflictEdges   map[string][]string  // NodeID -> []ConflictsWithNodeID
    AgentMetadata   map[string]string    // NodeID -> AgentName
    Timestamp       time.Time
}
```

### Validation Rules

- Nodes MUST have unique IDs
- DependencyEdges keys MUST exist in Nodes (valid NodeID)
- DependencyEdges values MUST exist in Nodes (valid dependencies)
- DependencyEdges MUST NOT contain cycles (detected by topological sort)
- ConflictEdges MUST be symmetric (if A conflicts with B, B conflicts with A)
- AgentMetadata keys MUST exist in Nodes

### Relationships

- **Contains**: 0 or more ModificationNode entities
- **Consumed by**: Arbiter (topological sort + synergy recognition)
- **Produces**: ArbitrationDecision

### Operations

```go
// AddNode adds a node and validates ID uniqueness
func (pg *ProposalGraph) AddNode(node ModificationNode) error

// AddDependency creates directed edge A depends on B
func (pg *ProposalGraph) AddDependency(nodeID, dependsOnID string) error

// AddConflict creates undirected edge A conflicts with B
func (pg *ProposalGraph) AddConflict(nodeA, nodeB string) error

// TopologicalSort returns nodes in dependency order
// Returns error if cycles detected
func (pg *ProposalGraph) TopologicalSort() ([]ModificationNode, error)

// DetectSpatialConflicts computes conflict edges via AABB intersection
func (pg *ProposalGraph) DetectSpatialConflicts()

// ToJSON serializes graph for arbiter prompt
func (pg *ProposalGraph) ToJSON() (string, error)
```

---

## Entity: ArbitrationDecision

**Purpose**: Represents the arbiter's final coordinated decision after analyzing the proposal graph, including execution sequence, recognized synergies, and rejections.

### Fields

```go
type ArbitrationDecision struct {
    ExecutionSequence []ModificationNode  // Topologically sorted, approved nodes
    InjectedNodes     []ModificationNode  // Synergy connectors added by arbiter
    DeferredNodes     []ModificationNode  // Low priority, delayed to next cycle
    RejectedNodes     []RejectedNode      // Invalid or superseded
    SpatialAllocation map[Region]string   // Region -> NodeID (allocated coordinates)
    LaborAllocation   int                 // Estimated dwarf-tasks
    Rationale         string              // Arbiter's reasoning
    Timestamp         time.Time
    Latency           time.Duration       // LLM inference time
    TokensUsed        int                 // Prompt + completion tokens
}

type RejectedNode struct {
    Node   ModificationNode
    Reason string  // "spatial_conflict", "cycle_detected", "invalid_region"
}
```

### Validation Rules

- ExecutionSequence MUST be in valid topological order (dependencies before dependents)
- ExecutionSequence + InjectedNodes MUST NOT have spatial conflicts (overlapping regions)
- LaborAllocation SHOULD be <= DwarfCount * 3 (realistic dwarf capacity)
- Latency SHOULD be <500ms for 7B model, <1000ms for 14B model
- TokensUsed SHOULD be <1000 (within budget)

### Relationships

- **Produced by**: Arbiter (LLM or fallback heuristic)
- **Consumes**: ProposalGraph
- **Produces**: Command list (via GraphExecutor)

### State Transitions

```
[created] --> Graph executor conversion --> [commands_generated] --> Execution --> [completed]
                                         |
                                         +--> [execution_failed] (DFHack error)
```

---

## Entity: LocalLLMProvider

**Purpose**: Connection to local LM Studio inference server, implementing Provider interface for local model arbitration.

### Fields

```go
type LocalLLMProvider struct {
    endpoint          string      // http://localhost:1234/v1
    model             string      // qwen2.5-7b-instruct-q4_k_m
    client            *http.Client
    systemPromptCache string      // Cached arbiter instructions
    timeout           time.Duration
    maxRetries        int
    healthStatus      HealthStatus
    avgLatency        time.Duration
    callCount         int
}

type HealthStatus string
const (
    HealthUnknown     HealthStatus = "unknown"
    HealthConnected   HealthStatus = "connected"
    HealthDisconnected HealthStatus = "disconnected"
    HealthTimeout     HealthStatus = "timeout"
)
```

### Validation Rules

- endpoint MUST be valid HTTP/HTTPS URL
- model MUST be non-empty string
- timeout MUST be >= 1000ms, <= 10000ms
- maxRetries MUST be >= 0, <= 3
- systemPromptCache updated on initialization (arbiter instructions)

### Relationships

- **Implements**: Provider interface (internal/llm/provider.go)
- **Consumes**: ProposalGraph JSON (arbiter user message)
- **Produces**: ArbitrationDecision JSON (arbiter response)

### Operations

```go
// GenerateResponse sends graph JSON to local LLM, returns execution sequence JSON
func (p *LocalLLMProvider) GenerateResponse(ctx context.Context, prompt string) (string, error)

// HealthCheck pings endpoint, updates healthStatus
func (p *LocalLLMProvider) HealthCheck() error

// UpdateSystemPrompt replaces cached arbiter instructions
func (p *LocalLLMProvider) UpdateSystemPrompt(prompt string)
```

---

## Entity: GraphExecutor

**Purpose**: Converts approved modification nodes from arbiter into executable DFHack commands.

### Fields

```go
type GraphExecutor struct {
    nodeTypeHandlers map[NodeType]NodeHandler
    logger           logging.Logger
    executionStatus  map[string]ExecutionStatus
}

type NodeHandler func(node ModificationNode) ([]protocol.CommandMessage, error)

type ExecutionStatus struct {
    NodeID    string
    Status    NodeStatus  // in_progress, completed, failed
    CommandIDs []uint32   // DFHack command IDs
    Error     string      // If failed
    StartTime time.Time
    EndTime   time.Time
}
```

### Operations

```go
// Execute converts nodes to commands and sends to DFHack
func (ge *GraphExecutor) Execute(decision ArbitrationDecision) ([]uint32, error)

// RegisterHandler adds handler for specific node type
func (ge *GraphExecutor) RegisterHandler(nodeType NodeType, handler NodeHandler)

// GetStatus returns execution status for node
func (ge *GraphExecutor) GetStatus(nodeID string) ExecutionStatus
```

### Node Type Handlers

**bedroom_cluster**:
- Extract region bounds
- Generate DIG commands for 3×3 bedroom grid
- Add corridor access if needed
- Return CommandTypeDig messages

**mining_shaft**:
- Generate DIG commands with DigTypeDownStair for vertical shaft
- Calculate depth from metadata
- Return CommandTypeDig messages

**farm_plot**:
- Generate ZONE commands for farm area
- Set zone type to farming
- Return CommandTypeZone messages

**corridor_connector**:
- Calculate shortest Manhattan path between regions
- Generate DIG commands for 1-tile-wide corridor
- Return CommandTypeDig messages

---

## Relationships Diagram

```text
FortMetrics
    |
    v
GoalAgent (5 instances) --produces--> ModificationNode
    |                                      |
    +--------------------------------------+
                    |
                    v
              ProposalGraph --consumed_by--> Arbiter (LocalLLMProvider)
                                                |
                                                v
                                        ArbitrationDecision
                                                |
                                                v
                                          GraphExecutor
                                                |
                                                v
                                      CommandMessage[] ---> DFHack Plugin
```

---

## Persistence Schema

**Extend existing** `internal/modifications/persistence.go`:

```go
type ModificationInfo struct {
    Type      ModificationType
    Timestamp time.Time
    Source    string
    GraphNode *GraphMetadata  // NEW
}

type GraphMetadata struct {
    NodeID       string
    NodeType     string
    AgentName    string
    Dependencies []string
    Status       string  // "pending", "in_progress", "completed"
    Rationale    string
}
```

**Save location**: `saves/{fortname}/modifications.json`

**Format**:
```json
{
  "modifications": {
    "40,30,125": {
      "type": "dug",
      "timestamp": "2025-11-09T12:00:00Z",
      "source": "agent",
      "graph_node": {
        "node_id": "food_1",
        "node_type": "farm_plot",
        "agent_name": "FoodSecurity",
        "dependencies": [],
        "status": "completed",
        "rationale": "Food at 11.4/dwarf, below 20 target"
      }
    }
  }
}
```

---

## Configuration Schema

**Add to** `internal/config/config.go`:

```go
type Config struct {
    // ... existing fields ...

    // LLM Provider
    LLMProviderType  string `yaml:"llm_provider_type"`  // "claude" | "local"
    LocalLLMEndpoint string `yaml:"local_llm_endpoint"` // "http://localhost:1234/v1"
    LocalLLMModel    string `yaml:"local_llm_model"`    // "qwen2.5-7b-instruct-q4_k_m"
    LLMTimeoutMS     int    `yaml:"llm_timeout_ms"`     // 5000

    // Goal Agents
    EnableGoalAgents bool              `yaml:"enable_goal_agents"` // true = graph mode, false = Feature 005 mode
    AgentFood        AgentConfig       `yaml:"agent_food"`
    AgentHousing     AgentConfig       `yaml:"agent_housing"`
    AgentMining      AgentConfig       `yaml:"agent_mining"`
    AgentWealth      AgentConfig       `yaml:"agent_wealth"`
    AgentDefense     DefenseAgentConfig `yaml:"agent_defense"`
}

type AgentConfig struct {
    Enabled  bool                   `yaml:"enabled"`
    Priority int                    `yaml:"priority"`
    Targets  map[string]interface{} `yaml:"targets"` // Agent-specific thresholds
}

type DefenseAgentConfig struct {
    Enabled      bool `yaml:"enabled"`
    PriorityBase int  `yaml:"priority_base"`   // When no threats
    PriorityThreat int `yaml:"priority_threat"` // When enemies present
}
```

**YAML example**:
```yaml
llm_provider_type: local
local_llm_endpoint: http://localhost:1234/v1
local_llm_model: qwen2.5-7b-instruct-q4_k_m
llm_timeout_ms: 5000

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
    growth_threshold: 0.10

agent_defense:
  enabled: true
  priority_base: 0
  priority_threat: 10
```

---

## Summary

**Core Entities**: 6
- GoalAgent (interface, 5 implementations)
- ModificationNode (graph node)
- ProposalGraph (node collection + edges)
- ArbitrationDecision (arbiter output)
- LocalLLMProvider (LM Studio client)
- GraphExecutor (node → command converter)

**Key Relationships**:
- Agents produce nodes
- Nodes form graph
- Arbiter consumes graph, produces decision
- Executor converts decision to commands

**Persistence**: Graph metadata extends existing modifications.json

**Configuration**: 15+ new settings for agents, LLM endpoint, model selection

**Ready for**: Contracts generation (if applicable) and quickstart documentation
