package modifications

import "time"

// Coordinate represents a 3D spatial position (reused from hazards pattern)
type Coordinate struct {
	X int16
	Y int16
	Z int16
}

// Bounds represents map dimensions
type Bounds struct {
	Width  uint16
	Height uint16
	Depth  uint16
}

// Region represents a 3D bounding box for spatial queries
type Region struct {
	XMin, XMax int16
	YMin, YMax int16
	ZMin, ZMax int16
}

// ModificationType represents the type of tile modification detected
type ModificationType uint8

const (
	ModificationUnknown    ModificationType = iota
	ModificationDug                         // Rock/Wall -> Floor (player dug)
	ModificationBuiltWall                   // Floor -> Wall (player built)
	ModificationBuiltFloor                  // Space -> Floor (player built)
	ModificationBuiltRamp                   // Floor -> Ramp (player built)
	ModificationChanneled                   // Floor -> Space (player channeled)
	ModificationSmoothed                    // Rough -> Smooth (player smoothed)
	ModificationEngraved                    // Smooth -> Engraved (player engraved)
	ModificationDestroyed                   // Built -> Natural (destroyed by player/enemy)
)

// String returns human-readable modification type name
func (m ModificationType) String() string {
	switch m {
	case ModificationDug:
		return "DUG"
	case ModificationBuiltWall:
		return "BUILT_WALL"
	case ModificationBuiltFloor:
		return "BUILT_FLOOR"
	case ModificationBuiltRamp:
		return "BUILT_RAMP"
	case ModificationChanneled:
		return "CHANNELED"
	case ModificationSmoothed:
		return "SMOOTHED"
	case ModificationEngraved:
		return "ENGRAVED"
	case ModificationDestroyed:
		return "DESTROYED"
	default:
		return "UNKNOWN"
	}
}

// ModificationInfo stores metadata for a modification at a specific coordinate
type ModificationInfo struct {
	Type         ModificationType // Type of modification
	OldTileType  uint16           // Original tile type before modification
	NewTileType  uint16           // New tile type after modification
	DetectedAt   time.Time        // When modification was first detected
	CommandID    uint32           // ID of command that caused this (0 if unknown)
	LastVerified time.Time        // Last time this modification was verified to still exist
	GraphNode    *GraphMetadata   `json:"graph_node,omitempty"` // Optional graph node metadata (Feature 006)
}

// GraphMetadata stores graph-based planning metadata for modifications
type GraphMetadata struct {
	NodeID       string   `json:"node_id"`
	NodeType     string   `json:"node_type"`
	AgentName    string   `json:"agent_name"`
	Dependencies []string `json:"dependencies"`
	Status       string   `json:"status"` // "pending", "in_progress", "completed"
	Rationale    string   `json:"rationale"`
}
