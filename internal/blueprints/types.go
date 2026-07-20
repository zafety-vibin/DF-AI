package blueprints

import (
	"github.com/df-ai/orchestrator/internal/modifications"
)

// DigBlueprint represents a saved dig pattern
type DigBlueprint struct {
	Name        string
	Description string
	Author      string // "ai" or "human"
	CreatedAt   string

	// Dig commands (relative coordinates)
	Digs []DigEntry

	// Metadata
	Width  int16
	Height int16
	Depth  int16
	Tags   []string // "entrance", "bedroom_block", "stairwell"
}

// DigEntry represents a single dig designation
type DigEntry struct {
	X       int16 // Relative to blueprint origin
	Y       int16
	Z       int16
	DigType string // "default", "stairs", "channel", "ramp", "upstair", "downstair"
}

// BlueprintLibrary manages available blueprints
type BlueprintLibrary struct {
	blueprints map[string]*DigBlueprint
	basePath   string // "blueprints/"
}

// NewBlueprintLibrary creates a new library and loads all blueprints
func NewBlueprintLibrary(basePath string) *BlueprintLibrary {
	bl := &BlueprintLibrary{
		blueprints: make(map[string]*DigBlueprint),
		basePath:   basePath,
	}
	bl.LoadAll() // Implementation in csv.go
	return bl
}

// AddBlueprint adds a blueprint to the library
func (bl *BlueprintLibrary) AddBlueprint(name string, bp *DigBlueprint) {
	bl.blueprints[name] = bp
}

// GetBlueprint retrieves a blueprint by name
func (bl *BlueprintLibrary) GetBlueprint(name string) *DigBlueprint {
	return bl.blueprints[name]
}

// ListBlueprints returns all blueprint names
func (bl *BlueprintLibrary) ListBlueprints() []string {
	names := make([]string, 0, len(bl.blueprints))
	for name := range bl.blueprints {
		names = append(names, name)
	}
	return names
}

// ApplyBlueprint calculates absolute coordinates for blueprint placement
func (bp *DigBlueprint) ApplyAt(origin modifications.Coordinate) []DigCommand {
	commands := make([]DigCommand, len(bp.Digs))

	for i, dig := range bp.Digs {
		commands[i] = DigCommand{
			X:       origin.X + dig.X,
			Y:       origin.Y + dig.Y,
			Z:       origin.Z + dig.Z,
			DigType: dig.DigType,
		}
	}

	return commands
}

// DigCommand represents an executable dig command
type DigCommand struct {
	X, Y, Z int16
	DigType string
}
