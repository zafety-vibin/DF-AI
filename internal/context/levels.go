package context

import (
	"fmt"

	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/modifications"
)

// GenerateTextOverview creates Level 0 text summary (500 bytes target)
// Example: "Embark year 1, surface Z=95-98, 7 dwarves, 3 chambers (48 tiles), water detected, no enemies"
func GenerateTextOverview(
	mods *modifications.ModificationOverlay,
	dwarfCount int,
	hazardMgr *hazards.HazardManager,
) string {
	// Start with basic info
	overview := fmt.Sprintf("%d dwarves", dwarfCount)

	// Add modification statistics if available
	if mods != nil && mods.GetCount() > 0 {
		bounds := mods.GetBounds()

		// Extract chambers for summary
		extractor := modifications.NewChamberExtractor(mods)
		chambers := extractor.ExtractChambers()

		// Calculate total tiles in chambers
		totalTiles := uint32(0)
		for _, chamber := range chambers {
			totalTiles += chamber.TileCount
		}

		overview += fmt.Sprintf(", %d chamber(s) (%d tiles modified), Z=%d to %d",
			len(chambers), totalTiles, bounds.ZMin, bounds.ZMax)
	} else {
		overview += ", no modifications yet"
	}

	// Add hazard warnings if present
	if hazardMgr != nil {
		counts := hazardMgr.GetAllCounts()
		hazardWarnings := []string{}

		if counts["aquifer"] > 0 {
			hazardWarnings = append(hazardWarnings, fmt.Sprintf("%d aquifer tile(s)", counts["aquifer"]))
		}
		if counts["water"] > 0 {
			hazardWarnings = append(hazardWarnings, fmt.Sprintf("%d water tile(s)", counts["water"]))
		}
		if counts["lava"] > 0 {
			hazardWarnings = append(hazardWarnings, fmt.Sprintf("%d lava tile(s)", counts["lava"]))
		}
		if counts["enemies"] > 0 {
			hazardWarnings = append(hazardWarnings, fmt.Sprintf("%d enem(y/ies)", counts["enemies"]))
		}
		if counts["caverns"] > 0 {
			hazardWarnings = append(hazardWarnings, fmt.Sprintf("%d cavern tile(s)", counts["caverns"]))
		}

		if len(hazardWarnings) > 0 {
			overview += ", hazards: "
			for i, warning := range hazardWarnings {
				if i > 0 {
					overview += ", "
				}
				overview += warning
			}
		} else {
			overview += ", no hazards detected"
		}
	}

	return overview
}

// AssembleActiveArea generates Level 1 active area context (10 KB target)
// Includes chambers + hazards + dwarves in modification region
// This is handled in assembly.go assembleLevel1()

// AssembleDeepPlanning generates Level 2 deep planning context (50 KB target)
// Includes caverns + water/lava + modification area expanded
// This is handled in assembly.go assembleLevel2()

// AssembleFullContext generates Level 3 full context (200 KB target)
// Includes compressed topology + all overlays + complete state
// This is handled in assembly.go assembleLevel3()
