package mapview

import (
	"errors"
	"fmt"
	"strings"
)

// StitchSlices concatenates a row-major grid of previously-fetched Slice
// blocks into one Slice covering their combined extent. grid[tileRow][col]
// — each block is at most 48 tiles wide/tall (the plugin's map_slice region
// cap, queries.cpp), and the last block on either axis may be a narrower/
// shorter remainder (e.g. width 100 = 48+48+4).
//
// Rows concatenate horizontally within a tile-row and the resulting
// tile-row lines stack vertically across tile-rows. Designated/Water/
// Aquifer/DesignationKinds/Smoothed/FloorItems entries in every block are
// already ABSOLUTE-coordinate (map_slice reports them in map space, not
// block-relative space) so they are simply unioned — no re-basing needed.
func StitchSlices(grid [][]*Slice) (*Slice, error) {
	if len(grid) == 0 || len(grid[0]) == 0 {
		return nil, errors.New("stitch: empty grid")
	}
	cols := len(grid[0])
	for r, row := range grid {
		if len(row) != cols {
			return nil, fmt.Errorf("stitch: ragged grid: tile-row %d has %d blocks, want %d", r, len(row), cols)
		}
	}
	first := grid[0][0]
	if first == nil {
		return nil, errors.New("stitch: nil block at (0,0)")
	}
	z, x1, y1 := first.Z, first.X1, first.Y1

	var rows []string
	wantWidth := -1
	for r, tileRow := range grid {
		height := -1
		for c, blk := range tileRow {
			if blk == nil {
				return nil, fmt.Errorf("stitch: nil block at tile-row %d col %d", r, c)
			}
			if blk.Z != z {
				return nil, fmt.Errorf("stitch: mismatched z at tile-row %d col %d: %d != %d", r, c, blk.Z, z)
			}
			if len(blk.Rows) == 0 {
				return nil, fmt.Errorf("stitch: empty block at tile-row %d col %d", r, c)
			}
			if height == -1 {
				height = len(blk.Rows)
			} else if len(blk.Rows) != height {
				return nil, fmt.Errorf("stitch: mismatched height within tile-row %d: col %d has %d rows, want %d", r, c, len(blk.Rows), height)
			}
		}
		for gr := 0; gr < height; gr++ {
			var sb strings.Builder
			for _, blk := range tileRow {
				sb.WriteString(blk.Rows[gr])
			}
			line := sb.String()
			if wantWidth == -1 {
				wantWidth = len(line)
			} else if len(line) != wantWidth {
				return nil, fmt.Errorf("stitch: tile-row %d width %d does not match earlier tile-row width %d", r, len(line), wantWidth)
			}
			rows = append(rows, line)
		}
	}

	out := &Slice{Z: z, X1: x1, Y1: y1, Rows: rows}
	// Stockpile coverage sums across blocks — the blocks' windows are
	// disjoint, so no tile is counted twice. Promoted to non-nil ONLY when
	// every block reported it: a mixed grid means part of the region was
	// never measured, and a partial sum presented as a whole would be a
	// quieter version of exactly the lie the pointer sentinel exists to
	// prevent.
	allMeasured := true
	spInView, spOccupied := 0, 0
	// Each block carries its OWN first-seen class-name table, so indices
	// are only meaningful within their block and must be re-keyed against
	// a merged table before the arrays can be unioned.
	classIndex := map[string]int{}
	for _, tileRow := range grid {
		for _, blk := range tileRow {
			out.Designated = append(out.Designated, blk.Designated...)
			out.Water = append(out.Water, blk.Water...)
			out.Aquifer = append(out.Aquifer, blk.Aquifer...)
			out.DesignationKinds = append(out.DesignationKinds, blk.DesignationKinds...)
			out.Smoothed = append(out.Smoothed, blk.Smoothed...)
			out.FloorItems = append(out.FloorItems, blk.FloorItems...)
			out.FloorItemsNoStock = append(out.FloorItemsNoStock, blk.FloorItemsNoStock...)
			if blk.FloorItemsNoStockCapped {
				out.FloorItemsNoStockCapped = true
			}
			if blk.StockpileTilesInView == nil {
				allMeasured = false
			} else {
				spInView += *blk.StockpileTilesInView
				if blk.StockpileTilesOccupied != nil {
					spOccupied += *blk.StockpileTilesOccupied
				}
			}
			remap := make([]int, len(blk.FloorItemClassNames))
			for i, name := range blk.FloorItemClassNames {
				idx, ok := classIndex[name]
				if !ok {
					idx = len(out.FloorItemClassNames)
					classIndex[name] = idx
					out.FloorItemClassNames = append(out.FloorItemClassNames, name)
				}
				remap[i] = idx
			}
			for _, fc := range blk.FloorItemClasses {
				i := int(fc[2])
				if i < 0 || i >= len(remap) {
					continue
				}
				out.FloorItemClasses = append(out.FloorItemClasses, [3]int16{fc[0], fc[1], int16(remap[i])})
			}
		}
	}
	if allMeasured {
		out.StockpileTilesInView = &spInView
		out.StockpileTilesOccupied = &spOccupied
	}
	return out, nil
}
