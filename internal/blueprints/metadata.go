package blueprints

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/df-ai/orchestrator/internal/logging"
)

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
	return fmt.Sprintf("%s: %s (%d×%d tiles, %d dwarf capacity)",
		bm.DisplayName, bm.Description, bm.Width, bm.Height, bm.DwarfCapacity)
}

// FitsInSpace checks if blueprint fits in available region
func (bm *BlueprintMetadata) FitsInSpace(availableWidth, availableHeight int) bool {
	return bm.Width <= availableWidth && bm.Height <= availableHeight
}

// LoadMetadata generates metadata for all blueprints in directory (T073)
func LoadMetadata(blueprintDir string, logger *logging.Logger) ([]*BlueprintMetadata, error) {
	metadata := make([]*BlueprintMetadata, 0)

	// Scan directory for .csv files
	entries, err := os.ReadDir(blueprintDir)
	if err != nil {
		if logger != nil {
			logger.Error("failed to read blueprint directory", err,
				logging.Field{Key: "dir", Value: blueprintDir})
		}
		return nil, fmt.Errorf("failed to read blueprint directory: %w", err)
	}

	loadedCount := 0
	skippedCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".csv") {
			continue
		}

		// Skip zone companion files
		if strings.HasSuffix(name, "_zones.csv") {
			continue
		}

		// Load blueprint and generate metadata
		blueprintPath := filepath.Join(blueprintDir, name)
		bpMetadata, err := parseBlueprint(blueprintPath)
		if err != nil {
			skippedCount++
			if logger != nil {
				logger.Warn("failed to parse blueprint, skipping",
					logging.Field{Key: "file", Value: name},
					logging.Field{Key: "error", Value: err.Error()})
			}
			continue
		}

		metadata = append(metadata, bpMetadata)
		loadedCount++

		if logger != nil {
			logger.Debug("loaded blueprint metadata",
				logging.Field{Key: "name", Value: bpMetadata.Name},
				logging.Field{Key: "dimensions", Value: fmt.Sprintf("%dx%d", bpMetadata.Width, bpMetadata.Height)},
				logging.Field{Key: "capacity", Value: bpMetadata.DwarfCapacity})
		}
	}

	if logger != nil {
		logger.Info("blueprint metadata loading complete",
			logging.Field{Key: "loaded", Value: loadedCount},
			logging.Field{Key: "skipped", Value: skippedCount},
			logging.Field{Key: "total", Value: loadedCount})
	}

	return metadata, nil
}

// parseBlueprint reads CSV, counts tiles, infers dimensions and capacity
func parseBlueprint(path string) (*BlueprintMetadata, error) {
	// Load blueprint using existing CSV loader
	bp, err := LoadFromCSV(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load blueprint: %w", err)
	}

	// Extract name (filename without .csv)
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".csv")

	// Generate display name (convert underscores to spaces, title case)
	displayName := strings.ReplaceAll(name, "_", " ")
	displayName = strings.Title(displayName)

	// Calculate dimensions and tile count
	width := int(bp.Width)
	height := int(bp.Height)
	tileCount := len(bp.Digs)

	// Infer dwarf capacity from blueprint name
	capacity := 0
	description := ""
	tags := make([]string, 0)

	if strings.Contains(name, "bedroom") {
		tags = append(tags, "bedroom")
		if strings.Contains(name, "cluster") {
			// Parse number from name (e.g., "bedroom_cluster_10" -> 10 dwarves)
			parts := strings.Split(name, "_")
			for _, part := range parts {
				if n, err := fmt.Sscanf(part, "%d", &capacity); n == 1 && err == nil {
					break
				}
			}
			description = fmt.Sprintf("Bedroom cluster for %d dwarves", capacity)
		} else if strings.Contains(name, "3x3") {
			capacity = 1
			description = "Compact single bedroom (3×3)"
			tags = append(tags, "compact")
		}
	}

	if strings.Contains(name, "nobles") {
		tags = append(tags, "nobles")
	}

	return &BlueprintMetadata{
		Name:          name,
		DisplayName:   displayName,
		Width:         width,
		Height:        height,
		Depth:         1, // Single Z-level for now
		TileCount:     tileCount,
		DwarfCapacity: capacity,
		Description:   description,
		Tags:          tags,
		SuitabilityCriteria: map[string]interface{}{
			"min_width":  width,
			"min_height": height,
		},
	}, nil
}

// GetMetadataPrompt formats all blueprint metadata for arbiter system prompt
func GetMetadataPrompt(metadata []*BlueprintMetadata) string {
	if len(metadata) == 0 {
		return ""
	}

	prompt := "Available Blueprints:\n"
	for _, bm := range metadata {
		prompt += "- " + bm.ToPromptString() + "\n"
	}

	return prompt
}
