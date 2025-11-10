package agents

// ===== HRM Architecture: Intent-Based Proposals (R002) =====

// IntentProposal represents an agent's high-level need WITHOUT spatial coordinates
// Agents propose WHAT they need, arbiter decides WHERE to place it
type IntentProposal struct {
	Agent       string                 `json:"agent"`       // "HousingAgent", "FoodAgent"
	Intent      string                 `json:"intent"`      // "provide_housing", "secure_food"
	Purpose     string                 `json:"purpose"`     // "housing", "workshop", "farm"
	Quantity    int                    `json:"quantity"`    // How many units (bedrooms, farm plots)
	Priority    int                    `json:"priority"`    // Agent priority (1-10)
	Urgency     float64                `json:"urgency"`     // 0.0-1.0, how critical this is
	Constraints []string               `json:"constraints"` // "near_stairs", "safe_layer", "requires_soil"

	// Optional hint from agent (arbiter can override)
	BlueprintHint string `json:"blueprint_hint"` // "bedroom_cluster_10", "farm_5x5"

	Rationale string                 `json:"rationale"` // Why this is needed
	Metadata  map[string]interface{} `json:"metadata"`  // Additional context
}

// ArbiterCommand is the output from arbiter (blueprint + anchor point)
type ArbiterCommand struct {
	Type      string   `json:"type"`      // "apply_blueprint", "dig_region"
	Blueprint string   `json:"blueprint"` // Blueprint name (if type=apply_blueprint)
	Anchor    [3]int   `json:"anchor"`    // [x, y, z] origin point
	Rotation  int      `json:"rotation"`  // 0, 90, 180, 270 degrees
	Reasoning string   `json:"reasoning"` // Arbiter's rationale
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // Additional data
}

// ArbiterIntentResponse is the full arbiter output for intent-based planning
type ArbiterIntentResponse struct {
	Commands []ArbiterCommand `json:"commands"` // Ordered list of blueprint placements
	Deferred []string         `json:"deferred"` // Proposal IDs that couldn't be placed
}
