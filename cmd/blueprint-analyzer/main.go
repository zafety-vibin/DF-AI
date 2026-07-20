package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StaircaseAnchor represents a staircase location within a blueprint
type StaircaseAnchor struct {
	RelativeX    int    `json:"x"`
	RelativeY    int    `json:"y"`
	RelativeZ    int    `json:"z"`
	StairType    string `json:"type"`
	IsEntryPoint bool   `json:"entry"`
}

// BlueprintAnalysis is the output of scanning a blueprint
type BlueprintAnalysis struct {
	Filename       string            `json:"filename"`
	Width          int               `json:"width"`
	Height         int               `json:"height"`
	TotalTiles     int               `json:"total_tiles"`
	DigTiles       int               `json:"dig_tiles"`
	StairCount     int               `json:"stair_count"`
	StairLocations []StaircaseAnchor `json:"stair_locations"`
	RoomCount      int               `json:"room_count_estimate"` // Estimated from connected regions
}

func main() {
	inputFile := flag.String("file", "", "Blueprint CSV file to analyze")
	inputDir := flag.String("dir", "", "Directory of blueprint CSVs to analyze")
	outputJSON := flag.Bool("json", false, "Output as JSON")
	flag.Parse()

	if *inputFile != "" {
		// Analyze single file
		analysis, err := analyzeBlueprint(*inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		printAnalysis(analysis, *outputJSON)
	} else if *inputDir != "" {
		// Analyze directory
		entries, err := os.ReadDir(*inputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading directory: %v\n", err)
			os.Exit(1)
		}

		analyses := make([]BlueprintAnalysis, 0)
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".csv") {
				continue
			}

			path := filepath.Join(*inputDir, entry.Name())
			analysis, err := analyzeBlueprint(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Skipping %s: %v\n", entry.Name(), err)
				continue
			}

			analyses = append(analyses, *analysis)
		}

		if *outputJSON {
			data, _ := json.MarshalIndent(analyses, "", "  ")
			fmt.Println(string(data))
		} else {
			for _, a := range analyses {
				printAnalysis(&a, false)
				fmt.Println("---")
			}
		}
	} else {
		flag.Usage()
		os.Exit(1)
	}
}

func analyzeBlueprint(path string) (*BlueprintAnalysis, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	analysis := &BlueprintAnalysis{
		Filename:       filepath.Base(path),
		StairLocations: make([]StaircaseAnchor, 0),
	}

	// Skip header row (starts with #dig or #build)
	startRow := 0
	if len(rows) > 0 && len(rows[0]) > 0 && strings.HasPrefix(rows[0][0], "#") {
		startRow = 1
	}

	// Calculate dimensions
	analysis.Height = len(rows) - startRow
	if analysis.Height > 0 {
		analysis.Width = len(rows[startRow])
	}

	// Scan for dig designations and stairs
	for y := startRow; y < len(rows); y++ {
		row := rows[y]
		for x := 0; x < len(row); x++ {
			cell := strings.TrimSpace(row[x])

			switch cell {
			case "d", "D":
				// Default dig
				analysis.DigTiles++
				analysis.TotalTiles++

			case "i", "I":
				// Up/down stair
				analysis.DigTiles++
				analysis.TotalTiles++
				analysis.StairLocations = append(analysis.StairLocations, StaircaseAnchor{
					RelativeX:    x,
					RelativeY:    y - startRow,
					RelativeZ:    0,
					StairType:    "updown",
					IsEntryPoint: analysis.StairCount == 0, // First stair is entry
				})
				analysis.StairCount++

			case "j", "J":
				// Down stair
				analysis.DigTiles++
				analysis.TotalTiles++
				analysis.StairLocations = append(analysis.StairLocations, StaircaseAnchor{
					RelativeX:    x,
					RelativeY:    y - startRow,
					RelativeZ:    0,
					StairType:    "down",
					IsEntryPoint: false,
				})
				analysis.StairCount++

			case "u", "U":
				// Up stair
				analysis.DigTiles++
				analysis.TotalTiles++
				analysis.StairLocations = append(analysis.StairLocations, StaircaseAnchor{
					RelativeX:    x,
					RelativeY:    y - startRow,
					RelativeZ:    0,
					StairType:    "up",
					IsEntryPoint: false,
				})
				analysis.StairCount++

			case "h", "H":
				// Channel
				analysis.DigTiles++
				analysis.TotalTiles++

			case "r", "R":
				// Ramp
				analysis.DigTiles++
				analysis.TotalTiles++
			}
		}
	}

	// Estimate room count (simple heuristic: dig_tiles / 9 for 3×3 rooms)
	if analysis.DigTiles > 0 {
		analysis.RoomCount = (analysis.DigTiles - analysis.StairCount) / 9
		if analysis.RoomCount < 1 {
			analysis.RoomCount = 1
		}
	}

	return analysis, nil
}

func printAnalysis(a *BlueprintAnalysis, jsonOutput bool) {
	if jsonOutput {
		data, _ := json.MarshalIndent(a, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Blueprint: %s\n", a.Filename)
		fmt.Printf("  Dimensions: %d×%d\n", a.Width, a.Height)
		fmt.Printf("  Total tiles: %d\n", a.TotalTiles)
		fmt.Printf("  Dig tiles: %d\n", a.DigTiles)
		fmt.Printf("  Estimated rooms: %d\n", a.RoomCount)
		fmt.Printf("  Stairs: %d\n", a.StairCount)
		if len(a.StairLocations) > 0 {
			fmt.Printf("  Stair locations:\n")
			for i, stair := range a.StairLocations {
				entry := ""
				if stair.IsEntryPoint {
					entry = " [ENTRY]"
				}
				fmt.Printf("    %d. (%d,%d) type=%s%s\n", i+1, stair.RelativeX, stair.RelativeY, stair.StairType, entry)
			}
		}
	}
}
