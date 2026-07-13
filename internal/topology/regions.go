package topology

// Coord is a 3D map coordinate, local to the topology package (a
// package-level Coord here would create an import cycle with
// internal/worldmodel, which already imports topology).
type Coord struct{ X, Y, Z int16 }

// Region is one 3D-connected component of open (walkable) tiles.
type Region struct {
	ID    int
	Tiles []Coord
	BBox  [2]Coord // [0] = min corner, [1] = max corner
}

// RegionGraph is the full set of regions for one topology snapshot, plus
// a fast tile->region lookup.
type RegionGraph struct {
	Regions []Region
	index   map[Coord]int // tile -> index into Regions
}

// BuildRegionGraph computes 3D-connected components of open tiles: full
// recompute on every call, no incremental maintenance (see design doc
// "Component 1" for the rationale — this is cheap enough in practice
// that incremental maintenance is deferred until measurement says
// otherwise).
//
// Adjacency: horizontal 4-connectivity within a Z-level, plus vertical
// connectivity between vertically stacked open tiles — see neighbors()
// below for why this is permissive rather than stair-only, and the
// design doc's "documented deviation" note in Component 1 for the
// false-positive risk that trade-off accepts.
func BuildRegionGraph(topo *TopologyOverlay) RegionGraph {
	w, h, d := topo.GetDimensions()
	visited := make(map[Coord]bool)
	var regions []Region
	index := make(map[Coord]int)

	for z := int16(0); z < int16(d); z++ {
		for y := int16(0); y < int16(h); y++ {
			for x := int16(0); x < int16(w); x++ {
				start := Coord{x, y, z}
				if visited[start] || topo.GetTileState(x, y, z) != StateOpen {
					continue
				}
				region := floodFill(topo, start, visited)
				region.ID = len(regions)
				for _, t := range region.Tiles {
					index[t] = region.ID
				}
				regions = append(regions, region)
			}
		}
	}
	return RegionGraph{Regions: regions, index: index}
}

func floodFill(topo *TopologyOverlay, start Coord, visited map[Coord]bool) Region {
	queue := []Coord{start}
	visited[start] = true
	tiles := []Coord{start}
	minC, maxC := start, start

	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		for _, n := range neighbors(topo, c) {
			if visited[n] || topo.GetTileState(n.X, n.Y, n.Z) != StateOpen {
				continue
			}
			visited[n] = true
			tiles = append(tiles, n)
			queue = append(queue, n)
			minC, maxC = minCoord(minC, n), maxCoord(maxC, n)
		}
	}
	return Region{Tiles: tiles, BBox: [2]Coord{minC, maxC}}
}

// Adjacency: horizontal 4-connectivity within a Z-level, plus vertical
// connectivity straight up/down.
//
// This is a DOCUMENTED DEVIATION from the design of record
// (specs/009-culture-and-learning/design-perception-round2.md, Component
// 1), which specifies vertical connectivity only "through matching stair
// tiles". The permissive rule here (any open tile directly above/below
// counts, not just stair-shaped ones) is used instead because
// TopologyOverlay.GetTileState only exposes the coarse three-state
// open/closed/unknown classification (state.go's ClassifyState) — the
// wire protocol's FLAG_FLOOR bit is set for floor/ramp/stair alike, with
// no sub-type carried over the wire and no spare bits in TileState.Flags
// to add one without a plugin protocol change. See the design doc for
// the accepted false-positive risk (ordinary floor-over-floor stacks
// report as one region) and why it's an acceptable trade-off for now
// (false positives only, never false negatives — see TestRegionGraph_
// VerticalAdjacencyIsPermissive in regions_test.go, which pins this
// behavior explicitly rather than leaving it as an undocumented
// incidental effect).
func neighbors(topo *TopologyOverlay, c Coord) []Coord {
	return []Coord{
		{c.X - 1, c.Y, c.Z}, {c.X + 1, c.Y, c.Z},
		{c.X, c.Y - 1, c.Z}, {c.X, c.Y + 1, c.Z},
		{c.X, c.Y, c.Z - 1}, {c.X, c.Y, c.Z + 1},
	}
}

func minCoord(a, b Coord) Coord {
	m := a
	if b.X < m.X {
		m.X = b.X
	}
	if b.Y < m.Y {
		m.Y = b.Y
	}
	if b.Z < m.Z {
		m.Z = b.Z
	}
	return m
}

func maxCoord(a, b Coord) Coord {
	m := a
	if b.X > m.X {
		m.X = b.X
	}
	if b.Y > m.Y {
		m.Y = b.Y
	}
	if b.Z > m.Z {
		m.Z = b.Z
	}
	return m
}

// SameRegion reports whether a and b are in the same connected component.
// Two tiles that are both unknown/closed (neither is open) are never the
// same region, even if equal.
func (rg RegionGraph) SameRegion(a, b Coord) bool {
	ia, ok1 := rg.index[a]
	ib, ok2 := rg.index[b]
	return ok1 && ok2 && ia == ib
}

// RegionAt returns the region containing c, if any.
func (rg RegionGraph) RegionAt(c Coord) (Region, bool) {
	i, ok := rg.index[c]
	if !ok {
		return Region{}, false
	}
	return rg.Regions[i], true
}

// NearestRegion returns the region whose closest tile to c has the
// smallest Chebyshev distance, and that distance. ok is false when the
// graph has no regions at all (e.g. nothing dug yet).
func (rg RegionGraph) NearestRegion(c Coord) (region Region, dist int, ok bool) {
	best := -1
	for _, r := range rg.Regions {
		for _, t := range r.Tiles {
			dx, dy, dz := abs16(t.X-c.X), abs16(t.Y-c.Y), abs16(t.Z-c.Z)
			d := max3(dx, dy, dz)
			if best == -1 || d < best {
				best, region, ok = d, r, true
			}
		}
	}
	dist = best
	return
}

func abs16(v int16) int {
	if v < 0 {
		return int(-v)
	}
	return int(v)
}

func max3(a, b, c int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}
