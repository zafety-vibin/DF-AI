package agents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/df-ai/orchestrator/internal/modifications"
)

// NodeType represents the type of modification node
type NodeType string

const (
	NodeTypeBedroom        NodeType = "bedroom_cluster"
	NodeTypeMiningShaft    NodeType = "mining_shaft"
	NodeTypeFarmPlot       NodeType = "farm_plot"
	NodeTypeWorkshop       NodeType = "workshop_zone"
	NodeTypeCorridor       NodeType = "corridor_connector"
	NodeTypeSealEntrance   NodeType = "seal_entrance"
	NodeTypeExploratory    NodeType = "exploratory_tunnel"
	NodeTypeDefensive      NodeType = "defensive_wall"
	NodeTypeStairCluster   NodeType = "stair_cluster"
	NodeTypeStockpile      NodeType = "stockpile_zone"
	NodeTypeGatherZone     NodeType = "gather_zone"
	NodeTypeProductionArea NodeType = "production_area"
)

// DependencyType represents a dependency relationship
type DependencyType string

const (
	DependencyAccess DependencyType = "requires_access_from"
	DependencyWater  DependencyType = "requires_water"
	DependencyStairs DependencyType = "requires_stairs"
	DependencyPower  DependencyType = "requires_power"
)

// NodeStatus represents the execution status of a node
type NodeStatus string

const (
	StatusProposed   NodeStatus = "proposed"
	StatusApproved   NodeStatus = "approved"
	StatusDeferred   NodeStatus = "deferred"
	StatusRejected   NodeStatus = "rejected"
	StatusInProgress NodeStatus = "in_progress"
	StatusCompleted  NodeStatus = "completed"
)

// ModificationNode represents a spatial modification proposal in the graph
type ModificationNode struct {
	ID           string                 `json:"id"`
	AgentName    string                 `json:"agent"`
	Type         NodeType               `json:"type"`
	Region       modifications.Region   `json:"region"`
	Dependencies []DependencyType       `json:"dependencies"`
	Conflicts    []string               `json:"conflicts"` // Node IDs with spatial overlap
	Priority     int                    `json:"priority"`
	Urgency      float64                `json:"urgency"`
	Rationale    string                 `json:"rationale"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Status       NodeStatus             `json:"status,omitempty"`
}

// ProposalGraph represents a collection of modification nodes with edges
type ProposalGraph struct {
	Nodes           []ModificationNode  `json:"nodes"`
	DependencyEdges map[string][]string `json:"dependency_edges"` // NodeID -> []DependsOnNodeID
	ConflictEdges   map[string][]string `json:"conflict_edges"`   // NodeID -> []ConflictsWithNodeID
	AgentMetadata   map[string]string   `json:"agent_metadata"`   // NodeID -> AgentName
	Timestamp       time.Time           `json:"timestamp"`
}

// NewProposalGraph creates a new empty proposal graph
func NewProposalGraph() *ProposalGraph {
	return &ProposalGraph{
		Nodes:           make([]ModificationNode, 0),
		DependencyEdges: make(map[string][]string),
		ConflictEdges:   make(map[string][]string),
		AgentMetadata:   make(map[string]string),
		Timestamp:       time.Now(),
	}
}

// AddNode adds a node to the graph
func (pg *ProposalGraph) AddNode(node ModificationNode) error {
	// Check for duplicate IDs
	for _, existing := range pg.Nodes {
		if existing.ID == node.ID {
			return fmt.Errorf("node ID %s already exists", node.ID)
		}
	}

	pg.Nodes = append(pg.Nodes, node)
	pg.AgentMetadata[node.ID] = node.AgentName
	return nil
}

// AddDependency creates a directed edge: nodeID depends on dependsOnID
func (pg *ProposalGraph) AddDependency(nodeID, dependsOnID string) error {
	// Validate both nodes exist
	if !pg.nodeExists(nodeID) {
		return fmt.Errorf("node %s not found", nodeID)
	}
	if !pg.nodeExists(dependsOnID) {
		return fmt.Errorf("dependency node %s not found", dependsOnID)
	}

	// Add dependency edge
	if pg.DependencyEdges[nodeID] == nil {
		pg.DependencyEdges[nodeID] = make([]string, 0)
	}
	pg.DependencyEdges[nodeID] = append(pg.DependencyEdges[nodeID], dependsOnID)
	return nil
}

// AddConflict creates an undirected edge: nodeA conflicts with nodeB
func (pg *ProposalGraph) AddConflict(nodeA, nodeB string) error {
	// Validate both nodes exist
	if !pg.nodeExists(nodeA) {
		return fmt.Errorf("node %s not found", nodeA)
	}
	if !pg.nodeExists(nodeB) {
		return fmt.Errorf("node %s not found", nodeB)
	}

	// Add conflict edges (both directions for undirected edge)
	if pg.ConflictEdges[nodeA] == nil {
		pg.ConflictEdges[nodeA] = make([]string, 0)
	}
	pg.ConflictEdges[nodeA] = append(pg.ConflictEdges[nodeA], nodeB)

	if pg.ConflictEdges[nodeB] == nil {
		pg.ConflictEdges[nodeB] = make([]string, 0)
	}
	pg.ConflictEdges[nodeB] = append(pg.ConflictEdges[nodeB], nodeA)

	return nil
}

// nodeExists checks if a node with given ID exists
func (pg *ProposalGraph) nodeExists(id string) bool {
	for _, node := range pg.Nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

// GetNode retrieves a node by ID
func (pg *ProposalGraph) GetNode(id string) (*ModificationNode, bool) {
	for i := range pg.Nodes {
		if pg.Nodes[i].ID == id {
			return &pg.Nodes[i], true
		}
	}
	return nil, false
}

// TopologicalSort performs topological sort using Kahn's algorithm
// Returns nodes in dependency order, or error if cycle detected
func (pg *ProposalGraph) TopologicalSort() ([]ModificationNode, error) {
	// Calculate in-degree for each node
	inDegree := make(map[string]int)
	for _, node := range pg.Nodes {
		inDegree[node.ID] = 0
	}

	// Count incoming edges
	for _, deps := range pg.DependencyEdges {
		for _, dep := range deps {
			inDegree[dep]++
		}
	}

	// Queue nodes with zero in-degree
	queue := make([]string, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// Process queue
	sorted := make([]ModificationNode, 0, len(pg.Nodes))
	for len(queue) > 0 {
		// Dequeue
		id := queue[0]
		queue = queue[1:]

		// Add to sorted list
		node, ok := pg.GetNode(id)
		if !ok {
			continue // Skip if node not found (shouldn't happen)
		}
		sorted = append(sorted, *node)

		// Decrement in-degree of dependent nodes
		if deps, ok := pg.DependencyEdges[id]; ok {
			for _, depID := range deps {
				inDegree[depID]--
				if inDegree[depID] == 0 {
					queue = append(queue, depID)
				}
			}
		}
	}

	// Check for cycle
	if len(sorted) != len(pg.Nodes) {
		return nil, fmt.Errorf("cycle detected in dependency graph")
	}

	return sorted, nil
}

// DetectSpatialConflicts computes conflict edges via AABB intersection
func (pg *ProposalGraph) DetectSpatialConflicts() {
	// Clear existing conflict edges
	pg.ConflictEdges = make(map[string][]string)

	// Check all pairs for spatial overlap
	for i := 0; i < len(pg.Nodes); i++ {
		for j := i + 1; j < len(pg.Nodes); j++ {
			if regionsOverlap(pg.Nodes[i].Region, pg.Nodes[j].Region) {
				// Add conflict edge (both directions)
				pg.AddConflict(pg.Nodes[i].ID, pg.Nodes[j].ID)
			}
		}
	}
}

// regionsOverlap checks if two regions have spatial overlap (AABB intersection)
func regionsOverlap(a, b modifications.Region) bool {
	// No overlap if one region is completely to the left/right/above/below the other
	if a.XMax < b.XMin || b.XMax < a.XMin {
		return false // No X overlap
	}
	if a.YMax < b.YMin || b.YMax < a.YMin {
		return false // No Y overlap
	}
	if a.ZMax < b.ZMin || b.ZMax < a.ZMin {
		return false // No Z overlap
	}
	return true // All axes overlap
}

// ToJSON serializes graph for arbiter prompt
func (pg *ProposalGraph) ToJSON() (string, error) {
	data, err := json.MarshalIndent(pg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal graph: %w", err)
	}
	return string(data), nil
}

// ArbitrationDecision represents arbiter's final decision
type ArbitrationDecision struct {
	ExecutionSequence []ModificationNode              `json:"execution_sequence"`
	InjectedNodes     []ModificationNode              `json:"injected_nodes"`
	DeferredNodes     []ModificationNode              `json:"deferred_nodes"`
	RejectedNodes     []RejectedNode                  `json:"rejected_nodes"`
	SpatialAllocation map[string]modifications.Region `json:"spatial_allocation"`
	LaborAllocation   int                             `json:"labor_allocation"`
	Rationale         string                          `json:"rationale"`
	Timestamp         time.Time                       `json:"timestamp"`
	Latency           time.Duration                   `json:"latency"`
	TokensUsed        int                             `json:"tokens_used"`
}

// RejectedNode represents a rejected proposal with reason
type RejectedNode struct {
	Node   ModificationNode `json:"node"`
	Reason string           `json:"reason"`
}
