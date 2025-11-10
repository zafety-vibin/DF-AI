package blueprints

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/df-ai/orchestrator/internal/modifications"
)

// LoadFromCSV loads a dig blueprint from CSV file (supports quickfort grid format)
// Quickfort format (community standard):
//   #dig start(26;25)
//   ,,,d,d,,d,d
//   ,,,d,d,,d,d
// Each cell = dig designation (d, i, j, u, h, r, #, or empty)
func LoadFromCSV(path string) (*DigBlueprint, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open blueprint: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("empty blueprint file")
	}

	bp := &DigBlueprint{
		Name: filepath.Base(path),
		Digs: make([]DigEntry, 0),
	}

	// Detect format: quickfort (grid) vs coordinate list
	startRow := 0
	if len(records) > 0 && len(records[0]) > 0 {
		firstCell := strings.TrimSpace(records[0][0])
		if strings.HasPrefix(firstCell, "#") {
			// Quickfort format - parse as grid
			return parseQuickfortGrid(records, filepath.Base(path))
		} else if firstCell == "x" {
			// Coordinate list format - skip header
			startRow = 1
		}
	}

	var minX, maxX, minY, maxY, minZ, maxZ int16

	for i, record := range records[startRow:] {
		if len(record) < 4 {
			return nil, fmt.Errorf("row %d: expected 4 columns, got %d", i+startRow, len(record))
		}

		x, err := strconv.ParseInt(record[0], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid x coordinate: %w", i+startRow, err)
		}

		y, err := strconv.ParseInt(record[1], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid y coordinate: %w", i+startRow, err)
		}

		z, err := strconv.ParseInt(record[2], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid z coordinate: %w", i+startRow, err)
		}

		digType := record[3]

		dig := DigEntry{
			X:       int16(x),
			Y:       int16(y),
			Z:       int16(z),
			DigType: digType,
		}

		bp.Digs = append(bp.Digs, dig)

		// Track bounds
		if i == 0 {
			minX, maxX = dig.X, dig.X
			minY, maxY = dig.Y, dig.Y
			minZ, maxZ = dig.Z, dig.Z
		} else {
			if dig.X < minX {
				minX = dig.X
			}
			if dig.X > maxX {
				maxX = dig.X
			}
			if dig.Y < minY {
				minY = dig.Y
			}
			if dig.Y > maxY {
				maxY = dig.Y
			}
			if dig.Z < minZ {
				minZ = dig.Z
			}
			if dig.Z > maxZ {
				maxZ = dig.Z
			}
		}
	}

	bp.Width = maxX - minX + 1
	bp.Height = maxY - minY + 1
	bp.Depth = maxZ - minZ + 1

	return bp, nil
}

// LoadAll scans directory and loads all .csv blueprints into library
func (bl *BlueprintLibrary) LoadAll() error {
	// Check if directory exists
	if _, err := os.Stat(bl.basePath); os.IsNotExist(err) {
		return nil // No blueprints directory
	}

	pattern := filepath.Join(bl.basePath, "*.csv")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, file := range files {
		bp, err := LoadFromCSV(file)
		if err != nil {
			continue // Skip invalid blueprints
		}
		name := filepath.Base(file)
		name = name[:len(name)-4] // Remove .csv extension
		bl.blueprints[name] = bp
	}

	return nil
}

// SaveToCSV saves a dig blueprint to CSV file
func SaveToCSV(bp *DigBlueprint, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create blueprint file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"x", "y", "z", "dig_type"}); err != nil {
		return err
	}

	// Write dig entries
	for _, dig := range bp.Digs {
		record := []string{
			fmt.Sprintf("%d", dig.X),
			fmt.Sprintf("%d", dig.Y),
			fmt.Sprintf("%d", dig.Z),
			dig.DigType,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// CreateBlueprintFromModifications extracts dig patterns from modifications
// Useful for AI to save successful designs
func CreateBlueprintFromModifications(
	mods *modifications.ModificationOverlay,
	region modifications.Region,
	name string,
) (*DigBlueprint, error) {

	// Get modifications in region
	modsInRegion := mods.GetModificationsInRegion(region, time.Time{})

	if len(modsInRegion) == 0 {
		return nil, fmt.Errorf("no modifications in region")
	}

	bp := &DigBlueprint{
		Name:   name,
		Author: "ai",
		Digs:   make([]DigEntry, 0, len(modsInRegion)),
	}

	// Find origin (minimum coordinates)
	originX, originY, originZ := region.XMin, region.YMin, region.ZMin

	for coord, modInfo := range modsInRegion {
		// Only save DUG tiles (skip built walls, etc.)
		if modInfo.Type != modifications.ModificationDug {
			continue
		}

		// Convert to relative coordinates
		dig := DigEntry{
			X:       coord.X - originX,
			Y:       coord.Y - originY,
			Z:       coord.Z - originZ,
			DigType: "default", // TODO: Infer dig type from tile shape
		}

		bp.Digs = append(bp.Digs, dig)
	}

	bp.Width = region.XMax - region.XMin + 1
	bp.Height = region.YMax - region.YMin + 1
	bp.Depth = region.ZMax - region.ZMin + 1

	return bp, nil
}

// parseQuickfortGrid parses community quickfort grid format
// Format: #dig start(x;y) followed by grid of cells with dig designations
func parseQuickfortGrid(records [][]string, filename string) (*DigBlueprint, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("empty quickfort blueprint")
	}

	bp := &DigBlueprint{
		Name: filename,
		Digs: make([]DigEntry, 0),
	}

	// Skip header row (starts with #)
	startRow := 0
	if len(records[0]) > 0 && strings.HasPrefix(strings.TrimSpace(records[0][0]), "#") {
		startRow = 1
	}

	var minX, maxX, minY, maxY int16
	firstEntry := true

	// Parse grid: each cell is a designation at (column, row) position
	for y := startRow; y < len(records); y++ {
		row := records[y]
		for x := 0; x < len(row); x++ {
			cell := strings.TrimSpace(strings.ToLower(row[x]))
			if cell == "" || cell == "#" {
				continue // Skip empty cells and comments
			}

			// Map quickfort designation to dig type
			digType := ""
			switch cell {
			case "d":
				digType = "default"
			case "i":
				digType = "updown_stair"
			case "j":
				digType = "down_stair"
			case "u":
				digType = "up_stair"
			case "h":
				digType = "channel"
			case "r":
				digType = "ramp"
			case "x":
				digType = "remove_ramp"
			default:
				// Unknown designation, skip
				continue
			}

			entry := DigEntry{
				X:       int16(x),
				Y:       int16(y - startRow),
				Z:       0, // Single-level blueprints default to Z=0
				DigType: digType,
			}

			bp.Digs = append(bp.Digs, entry)

			// Track bounds
			if firstEntry {
				minX, maxX = entry.X, entry.X
				minY, maxY = entry.Y, entry.Y
				firstEntry = false
			} else {
				if entry.X < minX {
					minX = entry.X
				}
				if entry.X > maxX {
					maxX = entry.X
				}
				if entry.Y < minY {
					minY = entry.Y
				}
				if entry.Y > maxY {
					maxY = entry.Y
				}
			}
		}
	}

	if len(bp.Digs) == 0 {
		return nil, fmt.Errorf("no dig entries found in quickfort blueprint")
	}

	bp.Width = maxX - minX + 1
	bp.Height = maxY - minY + 1
	bp.Depth = 1 // Single Z-level

	return bp, nil
}
