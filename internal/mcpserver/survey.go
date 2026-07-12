package mcpserver

import (
	"fmt"
	"strings"

	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/protocol"
)

type SurveyData struct {
	MapW, MapH, MapD uint16
	SurfaceZ         int16
	Dwarves          []protocol.EntityInfo
	Columns          []*mapview.ColumnProfile
	SurfaceSlice     *mapview.Slice
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

// renderSurvey composes the embark orientation report: dimensions, the
// surface Z, where everyone is, and what the sampled stratigraphy looks
// like. This is the model's first read of a new map.
func renderSurvey(d SurveyData) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "EMBARK SURVEY\nmap %dx%dx%d (x east, y south, z up; z=%d is map top)\n",
		d.MapW, d.MapH, d.MapD, int(d.MapD)-1)
	fmt.Fprintf(&sb, "surface z=%d (highest dwarf z on a fresh embark)\n", d.SurfaceZ)
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
	sb.WriteString("\nOpening reminders: dig a 2x2 stair shaft down into stone, carve rooms BESIDE the shaft (never on top of it), keep the surface exposure small.\n")
	return sb.String()
}
