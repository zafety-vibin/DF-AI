package mapview

import (
	"fmt"
	"math"
	"sort"

	"github.com/df-ai/orchestrator/internal/topology"
)

// DigSiteRequest describes the room footprint to place and the anchor to
// rank candidates against.
type DigSiteRequest struct {
	W, H          int16 // room footprint
	Z             int16 // level to search
	NearX, NearY  int16 // ranking anchor (e.g. stairs bottom / wagon)
	MaxCandidates int   // default 5
}

// DigSite is one ranked candidate rectangle.
type DigSite struct {
	X1, Y1, X2, Y2 int16
	Z              int16
	Dist           float64 // distance from anchor
	Solid          int     // solid/hidden tiles inside (diggable mass)
	Open           int     // already-open tiles inside (0 is ideal)
	Rationale      string
}

// FindDigSites scans a z-level for WxH rectangles of solid (closed or
// hidden) ground, ranked by distance from the anchor. Geometry lives
// here so the model never does coordinate arithmetic (FLE failure mode).
func FindDigSites(topo *topology.TopologyOverlay, req DigSiteRequest) []DigSite {
	if req.MaxCandidates <= 0 {
		req.MaxCandidates = 5
	}
	w, h, _ := topo.GetDimensions()
	var out []DigSite
	const stride = 2 // scan on a coarse grid; candidates don't need to be exhaustive
	for x := int16(1); x+req.W < int16(w)-1; x += stride {
		for y := int16(1); y+req.H < int16(h)-1; y += stride {
			solid, open := 0, 0
			for dx := int16(0); dx < req.W; dx++ {
				for dy := int16(0); dy < req.H; dy++ {
					switch topo.GetTileState(x+dx, y+dy, req.Z) {
					case topology.StateOpen:
						open++
					default: // closed or unknown/hidden — both are diggable mass
						solid++
					}
				}
			}
			if open > 0 {
				continue // room must be carved from fully solid ground
			}
			d := math.Hypot(float64(x+req.W/2-req.NearX), float64(y+req.H/2-req.NearY))
			out = append(out, DigSite{
				X1: x, Y1: y, X2: x + req.W - 1, Y2: y + req.H - 1, Z: req.Z,
				Dist: d, Solid: solid, Open: open,
				Rationale: fmt.Sprintf("%dx%d fully solid, %.0f tiles from anchor", req.W, req.H, d),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dist < out[j].Dist })
	if len(out) > req.MaxCandidates {
		out = out[:req.MaxCandidates]
	}
	return out
}
