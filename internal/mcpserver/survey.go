package mcpserver

import (
	"fmt"
	"strings"

	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/protocol"
)

type SurveyData struct {
	MapW, MapH, MapD uint16
	// CrewZ is the highest dwarf Z at embark — an anchor for sampling
	// z-windows, NOT the map's surface. A single Z stamped across every
	// column as "the surface" is the exact bug this survey used to have
	// (surface height varies per column: hills, valleys, ramps down to
	// water). Use SurfaceSamples for the real, per-column surface picture.
	CrewZ int16
	// Dwarves is the embark crew's cluster (for the position summary).
	Dwarves []protocol.EntityInfo
	// Columns are close-in stratigraphy samples (crew position + two
	// offsets) — soil/stone/aquifer detail near the crew.
	Columns []*mapview.ColumnProfile
	// SurfaceSamples are a coarse grid of columns spread across the whole
	// map, sampled purely to find each column's own surface Z
	// (mapview.ColumnSurfaceZ) and report the range.
	SurfaceSamples []*mapview.ColumnProfile
	SurfaceSlice   *mapview.Slice
}

// aquiferNote summarizes aquifer layers in a sampled column: ", AQUIFER at
// z=N" for a single layer, a z range when the layers are contiguous (levels
// arrive top-to-bottom), first z otherwise. Empty when no aquifer.
func aquiferNote(levels []mapview.ColumnLevel) string {
	var zs []int16
	for _, lv := range levels {
		if lv.Aquifer {
			zs = append(zs, lv.Z)
		}
	}
	if len(zs) == 0 {
		return ""
	}
	contiguous := true
	for i := 1; i < len(zs); i++ {
		if zs[i] != zs[i-1]-1 {
			contiguous = false
			break
		}
	}
	if len(zs) > 1 && contiguous {
		return fmt.Sprintf(", AQUIFER at z=%d..%d", zs[0], zs[len(zs)-1])
	}
	return fmt.Sprintf(", AQUIFER at z=%d", zs[0])
}

// renderSurfaceRange summarizes a coarse grid of sampled columns into a
// surface-Z range line. Samples whose queried z-window never confirmed a
// surface (mapview.ColumnSurfaceZ result != SurfaceFound) are flagged,
// never folded into the range as a wrong number — a survey that quietly
// reports "surface z=0" for a column it couldn't actually see, or that
// folds a sample whose window-top was already solid terrain (a hill rising
// above the window) into the range as if that were the true surface, is
// worse than saying so plainly. The two inconclusive causes need opposite
// fixes, so they are counted and reported separately.
func renderSurfaceRange(samples []*mapview.ColumnProfile, crewZ int16) string {
	var sb strings.Builder
	minZ, maxZ, found, aboveWindow, allAir := int16(0), int16(0), 0, 0, 0
	for _, c := range samples {
		z, result := mapview.ColumnSurfaceZ(c)
		switch result {
		case mapview.SurfaceAboveWindow:
			aboveWindow++
		case mapview.SurfaceUnknownAllAir:
			allAir++
		default:
			if found == 0 || z < minZ {
				minZ = z
			}
			if found == 0 || z > maxZ {
				maxZ = z
			}
			found++
		}
	}
	switch {
	case len(samples) == 0:
		fmt.Fprintf(&sb, "surface z: no samples taken (embark crew at z=%d)\n", crewZ)
	case found == 0:
		fmt.Fprintf(&sb, "surface z: inconclusive across all %d sampled columns (embark crew at z=%d)\n", len(samples), crewZ)
	default:
		fmt.Fprintf(&sb, "surface z ranges %d..%d across %d sampled columns (embark crew at z=%d)\n",
			minZ, maxZ, found, crewZ)
	}
	if aboveWindow > 0 {
		fmt.Fprintf(&sb, "(%d of %d samples inconclusive: terrain still solid at the sample window's top — raise the window)\n", aboveWindow, len(samples))
	}
	if allAir > 0 {
		fmt.Fprintf(&sb, "(%d of %d samples inconclusive: open air throughout the queried z-window — widen it downward)\n", allAir, len(samples))
	}
	return sb.String()
}

// renderSurvey composes the embark orientation report: dimensions, the
// surface Z, where everyone is, and what the sampled stratigraphy looks
// like. This is the model's first read of a new map.
func renderSurvey(d SurveyData) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "EMBARK SURVEY\nmap %dx%dx%d (x east, y south, z up; z=%d is map top)\n",
		d.MapW, d.MapH, d.MapD, int(d.MapD)-1)
	sb.WriteString(renderSurfaceRange(d.SurfaceSamples, d.CrewZ))
	if len(d.Dwarves) > 0 {
		minX, maxX, minY, maxY := d.Dwarves[0].X, d.Dwarves[0].X, d.Dwarves[0].Y, d.Dwarves[0].Y
		for _, e := range d.Dwarves {
			if e.X < minX {
				minX = e.X
			}
			if e.X > maxX {
				maxX = e.X
			}
			if e.Y < minY {
				minY = e.Y
			}
			if e.Y > maxY {
				maxY = e.Y
			}
		}
		fmt.Fprintf(&sb, "%d dwarves clustered in (%d,%d)-(%d,%d)\n",
			len(d.Dwarves), minX, minY, maxX, maxY)
	}
	for _, c := range d.Columns {
		if c == nil || len(c.Levels) == 0 {
			continue
		}
		soil, stone, firstStone := 0, 0, int16(-1)
		firstStoneHidden := false
		for _, lv := range c.Levels {
			switch lv.Material {
			case "soil":
				soil++
			case "stone", "mineral":
				stone++
				if firstStone == -1 {
					firstStone = lv.Z
					firstStoneHidden = lv.Hidden
				}
			}
		}
		switch {
		case firstStone == -1:
			fmt.Fprintf(&sb, "column (%d,%d): %d soil layers, no stone in sampled range",
				c.X, c.Y, soil)
		case firstStoneHidden:
			fmt.Fprintf(&sb, "column (%d,%d): %d soil layers, first stone at z=%d (under fog — undug, diggable)",
				c.X, c.Y, soil, firstStone)
		default:
			fmt.Fprintf(&sb, "column (%d,%d): %d soil layers, first exposed stone at z=%d",
				c.X, c.Y, soil, firstStone)
		}
		sb.WriteString(aquiferNote(c.Levels) + "\n")
	}
	if d.SurfaceSlice != nil {
		trees, grass := 0, 0
		for _, row := range d.SurfaceSlice.Rows {
			for _, g := range row {
				if g == 'T' || g == 't' {
					trees++
				}
				if g == ',' {
					grass++
				}
			}
		}
		fmt.Fprintf(&sb, "surface sample: %d tree/sapling tiles, %d grass tiles in a %dx%d crop around the crew\n",
			trees, grass, len(d.SurfaceSlice.Rows[0]), len(d.SurfaceSlice.Rows))
	}
	sb.WriteString("\nOpening reminders: dig a 2x2 stair shaft down into stone, carve rooms BESIDE the shaft (never on top of it), keep the surface exposure small. " +
		"Sampled columns above the crew's z are normal terrain (hills/mountains), not a gap in the world — that ground is real, solid, and just as diggable; only fog ('?') means undug, never elevation.\n")
	return sb.String()
}
