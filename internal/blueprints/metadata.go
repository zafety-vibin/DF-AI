package blueprints

// BlueprintMetadata represents metadata about blueprints for arbiter context
type BlueprintMetadata struct {
	Name                string                 // Blueprint filename (without .csv)
	DisplayName         string                 // Human-readable name
	Width               int                    // Horizontal dimension (X)
	Height              int                    // Depth dimension (Y)
	Depth               int                    // Vertical dimension (Z) - default: 1 (single level)
	TileCount           int                    // Total tiles in blueprint
	DwarfCapacity       int                    // How many dwarves this blueprint houses (>= 0, 0 for non-housing)
	Description         string                 // Purpose and features (max 200 chars)
	Tags                []string               // Categories (bedroom, compact, nobles, etc.) - 0-5 elements
	SuitabilityCriteria map[string]interface{} // Min space, constraints
}

// ToPromptString formats metadata for arbiter system prompt
func (bm *BlueprintMetadata) ToPromptString() string {
	return bm.DisplayName + ": " + bm.Description +
		" (" + string(rune(bm.Width)) + "×" + string(rune(bm.Height)) + " tiles, " +
		string(rune(bm.DwarfCapacity)) + " dwarf capacity)"
}

// FitsInSpace checks if blueprint fits in available region
func (bm *BlueprintMetadata) FitsInSpace(availableWidth, availableHeight int) bool {
	return bm.Width <= availableWidth && bm.Height <= availableHeight
}
