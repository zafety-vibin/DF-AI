package mcpserver

import "sync"

// digRect is a normalized (min <= max on every axis) 3D dig-designation
// rectangle.
type digRect struct {
	x1, y1, z1, x2, y2, z2 int16
}

func (r digRect) contains(x, y, z int16) bool {
	return x >= r.x1 && x <= r.x2 &&
		y >= r.y1 && y <= r.y2 &&
		z >= r.z1 && z <= r.z2
}

// pendingDigs records every dig rectangle the plugin ACKed this session.
// connectorSuggestion treats tiles inside these rects as connected-in-
// progress: the dig-ahead workflow designates a stair spine and its rooms
// while both are still solid, and the topology overlay classifies a
// hidden-but-designated spine tile as Unknown — so without this record a
// room beside the not-yet-carved spine reads "not yet connected" and the
// nearest-open-tile fallback points at wild surface terrain. Session-scoped
// on purpose: it is a hint aid, not ground truth, and resets with the
// server. A later cancel_designation can leave a stale rect behind; the
// only consequence is a suppressed informational hint.
type pendingDigs struct {
	mu    sync.Mutex
	rects []digRect
}

// add records a successfully ACKed dig rectangle, corners in any order.
// Nil-safe no-op so a scaffold Bridge without the record never panics.
func (p *pendingDigs) add(x1, y1, z1, x2, y2, z2 int16) {
	if p == nil {
		return
	}
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	if z1 > z2 {
		z1, z2 = z2, z1
	}
	p.mu.Lock()
	p.rects = append(p.rects, digRect{x1, y1, z1, x2, y2, z2})
	p.mu.Unlock()
}

// contains reports whether any recorded rect covers (x,y,z). Nil-safe:
// a nil record contains nothing.
func (p *pendingDigs) contains(x, y, z int16) bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, r := range p.rects {
		if r.contains(x, y, z) {
			return true
		}
	}
	return false
}
