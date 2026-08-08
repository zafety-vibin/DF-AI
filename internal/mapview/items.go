package mapview

import (
	"fmt"
	"sort"
	"strings"
)

// Loose-item reporting for rendered views.
//
// The plugin has always sent per-tile loose-item POSITIONS
// (map_slice's "floor_items": [[x,y,count],...]); the renderer used to
// throw them away and print only len() — "tiles with loose items on
// floor: 200". That line cannot distinguish a well-organized full
// stockpile from a floor buried in junk, which is exactly the confusion
// that made a live session blind to a stockpile shortage until a human
// said "we need more stockpile room" out loud.
//
// Everything here is spatial arithmetic done in Go, never handed to the
// model: the model gets totals and a handful of cluster bounding boxes it
// can act on directly (the follow-up `stockpile` call needs no further
// perception round trip in the common case).

// maxItemClustersShown bounds the always-on footer: the two biggest
// homeless piles carry the actionable signal, and everything past them
// collapses into a "+N tiles elsewhere" tail. This footer prints on EVERY
// look, so its worst case has to stay a fixed couple of lines.
const maxItemClustersShown = 2

// ItemCluster is one 8-connected blob of item-bearing tiles: its bounding
// box, how many tiles it spans, and how many loose items sit in it. The
// bbox is what a caller feeds straight back into `stockpile`.
type ItemCluster struct {
	X1, Y1, X2, Y2 int16
	Tiles          int
	Items          int
}

// String renders a cluster for the footer, collapsing a single-tile
// cluster to a bare coordinate instead of a degenerate 1x1 rectangle.
func (c ItemCluster) String() string {
	if c.X1 == c.X2 && c.Y1 == c.Y2 {
		return fmt.Sprintf("(%d,%d) %d items", c.X1, c.Y1, c.Items)
	}
	return fmt.Sprintf("(%d,%d)-(%d,%d) %d items", c.X1, c.Y1, c.X2, c.Y2, c.Items)
}

// ClusterItemTiles groups [x,y,count] item tiles into 8-connected
// components, sorted by item count descending (ties break by X1 then Y1,
// so output is stable for a given input regardless of map iteration
// order). Diagonal adjacency counts: a dwarf sees one pile there, and
// splitting it into two diagonal fragments would report two half-sized
// piles that no follow-up rectangle covers cleanly.
//
// Repeated coordinates are summed rather than double-counted as tiles —
// a stitched multi-block slice unions its blocks' arrays, and a defensive
// duplicate must not inflate the tile count.
func ClusterItemTiles(tiles [][3]int16) []ItemCluster {
	if len(tiles) == 0 {
		return nil
	}
	counts := make(map[[2]int16]int, len(tiles))
	order := make([][2]int16, 0, len(tiles))
	for _, t := range tiles {
		p := [2]int16{t[0], t[1]}
		if _, seen := counts[p]; !seen {
			order = append(order, p)
		}
		counts[p] += int(t[2])
	}

	visited := make(map[[2]int16]bool, len(order))
	var out []ItemCluster
	for _, seed := range order {
		if visited[seed] {
			continue
		}
		visited[seed] = true
		stack := [][2]int16{seed}
		c := ItemCluster{X1: seed[0], Y1: seed[1], X2: seed[0], Y2: seed[1]}
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			c.Tiles++
			c.Items += counts[p]
			if p[0] < c.X1 {
				c.X1 = p[0]
			}
			if p[0] > c.X2 {
				c.X2 = p[0]
			}
			if p[1] < c.Y1 {
				c.Y1 = p[1]
			}
			if p[1] > c.Y2 {
				c.Y2 = p[1]
			}
			for dy := int16(-1); dy <= 1; dy++ {
				for dx := int16(-1); dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					n := [2]int16{p[0] + dx, p[1] + dy}
					if _, has := counts[n]; !has || visited[n] {
						continue
					}
					visited[n] = true
					stack = append(stack, n)
				}
			}
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Items != out[j].Items {
			return out[i].Items > out[j].Items
		}
		if out[i].X1 != out[j].X1 {
			return out[i].X1 < out[j].X1
		}
		return out[i].Y1 < out[j].Y1
	})
	return out
}

// sumItemCounts totals the per-tile item counts of an [x,y,count] array.
func sumItemCounts(tiles [][3]int16) int {
	n := 0
	for _, t := range tiles {
		n += int(t[2])
	}
	return n
}

// clusterPhrase renders "clusters: <top N>, +M tiles elsewhere" for an
// [x,y,count] array, or "" when there is nothing to cluster.
func clusterPhrase(tiles [][3]int16) string {
	clusters := ClusterItemTiles(tiles)
	if len(clusters) == 0 {
		return ""
	}
	allTiles := 0
	for _, c := range clusters {
		allTiles += c.Tiles
	}
	shown := clusters
	if len(shown) > maxItemClustersShown {
		shown = shown[:maxItemClustersShown]
	}
	parts := make([]string, 0, len(shown))
	shownTiles := 0
	for _, c := range shown {
		parts = append(parts, c.String())
		shownTiles += c.Tiles
	}
	phrase := "clusters: " + strings.Join(parts, ", ")
	if rest := allTiles - shownTiles; rest > 0 {
		phrase += fmt.Sprintf(", +%d tiles elsewhere", rest)
	}
	return phrase
}

// percentOf returns n as a whole-number percentage of total, 0 when total
// is 0 (a view with no stockpile tiles never reaches the caller anyway).
func percentOf(n, total int) int {
	if total <= 0 {
		return 0
	}
	return n * 100 / total
}

// FloorItemFooter renders the always-on loose-item lines for a crop, at
// most two of them, and NOTHING at all for a tidy view — a clean level
// pays zero tokens for this feature.
//
// Version skew is the load-bearing case. StockpileTilesInView == nil
// means the connected plugin never measured stockpile coverage: the
// footer then reports only what that plugin actually sent (totals plus
// cluster positions, both derived from the floor_items array every build
// since the smoothing wave already ships) and says outright that the
// in/out-of-stockpile split is unavailable. It must never render an
// absent split as "0 items outside stockpiles", which would read as an
// all-clear the plugin never asserted.
func FloorItemFooter(s *Slice) []string {
	if s == nil {
		return nil
	}
	totalItems := sumItemCounts(s.FloorItems)
	totalTiles := len(s.FloorItems)

	if s.StockpileTilesInView == nil {
		if totalTiles == 0 {
			return nil
		}
		line := fmt.Sprintf("loose items in view: %d on %d tiles (stockpile coverage not reported by this plugin build)",
			totalItems, totalTiles)
		if cl := clusterPhrase(s.FloorItems); cl != "" {
			line += " — " + cl
		}
		return []string{line}
	}

	inView := *s.StockpileTilesInView
	occupied := 0
	if s.StockpileTilesOccupied != nil {
		occupied = *s.StockpileTilesOccupied
	}

	// The "nothing is wrong" case is the one a fort pays most often, so it
	// collapses to ONE line. Two lines there would spend tokens saying
	// overlapping things: with no homeless pile to point at, "everything is
	// stored" and "here is how taken the stockpile tiles are" are the same
	// subject. The two-line form below is reserved for the homeless case,
	// where the lines carry genuinely different subjects.
	if totalTiles > 0 && len(s.FloorItemsNoStock) == 0 {
		line := fmt.Sprintf("loose items in view: %d on %d tiles, all inside stockpiles", totalItems, totalTiles)
		if inView > 0 {
			line += fmt.Sprintf(" (%d stockpile tiles, %d taken)", inView, occupied)
		}
		return []string{line}
	}

	var out []string
	if totalTiles > 0 {
		line := fmt.Sprintf("loose items OUTSIDE stockpiles: %d items on %d tiles (view total %d on %d tiles)",
			sumItemCounts(s.FloorItemsNoStock), len(s.FloorItemsNoStock), totalItems, totalTiles)
		if cl := clusterPhrase(s.FloorItemsNoStock); cl != "" {
			line += " — " + cl
		}
		if s.FloorItemsNoStockCapped {
			// "per map block", not a flat 200: the cap is applied inside the
			// plugin per map_slice block, and a stitched scope=elevation/fort
			// render (mcpserver's fetchStitchedRegion) unions many blocks into
			// one Slice, so the real bound is 200 x blocks — a flat "capped at
			// 200" would understate it by more than an order of magnitude.
			// FloorItemSummaryLine already says the same thing.
			line += " [outside-stockpile tile list capped at 200 per map block — treat these as lower bounds]"
		}
		line += " (lens=items for classes)"
		out = append(out, line)
	}
	if inView > 0 {
		// Deliberately tile-scoped wording: this is NOT a capacity figure.
		// A pile whose every tile holds one half-empty bin reads as 100%
		// here, and "84% full" would be a lie the caller would act on. The
		// container caveat that qualifies it lives on `buildings`; this
		// always-on line pays for its honesty in wording, not a second line.
		out = append(out, fmt.Sprintf("stockpile tiles in view: %d, %d with items on them (%d%% of tiles taken)",
			inView, occupied, percentOf(occupied, inView)))
	}
	return out
}

// FloorItemSummaryLine is FloorItemFooter's one-line sibling for
// downsampled renders (look scope=elevation/fort past the render budget).
// Cluster bounding boxes are deliberately dropped there: those views
// report block-granular cells, so a tile-exact bbox would invite a
// follow-up rectangle finer than the picture it came from. Returns "" when
// there is nothing to report.
func FloorItemSummaryLine(s *Slice) string {
	if s == nil || len(s.FloorItems) == 0 {
		return ""
	}
	totalItems := sumItemCounts(s.FloorItems)
	totalTiles := len(s.FloorItems)
	if s.StockpileTilesInView == nil {
		return fmt.Sprintf("loose items in source view: %d on %d tiles (stockpile coverage not reported by this plugin build)",
			totalItems, totalTiles)
	}
	homelessTiles := len(s.FloorItemsNoStock)
	if homelessTiles == 0 {
		return fmt.Sprintf("loose items in source view: %d on %d tiles, all inside stockpiles", totalItems, totalTiles)
	}
	line := fmt.Sprintf("loose items in source view: %d on %d tiles — %d items on %d tiles OUTSIDE stockpiles",
		totalItems, totalTiles, sumItemCounts(s.FloorItemsNoStock), homelessTiles)
	if s.FloorItemsNoStockCapped {
		line += " [outside-stockpile tile list capped at 200 per block — lower bound]"
	}
	return line
}
