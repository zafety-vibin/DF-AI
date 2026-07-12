package plan

import (
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// DAG is a thread-safe directed-acyclic graph of plan nodes. The executor
// drains Ready nodes; the reconciler updates per-node prediction counts and
// status; the deliberator reads stats and active nodes when forming the
// next prompt.
type DAG struct {
	mu          sync.RWMutex
	nodes       map[string]*Node
	insertOrder []string
}

// New returns an empty DAG.
func New() *DAG {
	return &DAG{nodes: make(map[string]*Node)}
}

// Add stores n. If a node with the same ID already exists, it is replaced
// (the deliberator should generate fresh IDs per turn). Initial Status is
// computed from dependencies: Ready if no deps; Pending otherwise.
func (d *DAG) Add(n *Node) {
	if n == nil || n.ID == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, existed := d.nodes[n.ID]; !existed {
		d.insertOrder = append(d.insertOrder, n.ID)
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	if n.Status == StatusPending && d.allDepsDoneLocked(n.Dependencies) {
		n.Status = StatusReady
	}
	d.nodes[n.ID] = n
}

// Get returns a copy of the node and a found flag.
func (d *DAG) Get(id string) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if n, ok := d.nodes[id]; ok {
		return *n, true
	}
	return Node{}, false
}

// Update mutates a node under the DAG lock. Returns true if the node existed
// and the mutator was invoked. Use this for status transitions.
func (d *DAG) Update(id string, fn func(*Node)) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.nodes[id]
	if !ok {
		return false
	}
	fn(n)
	d.promoteNewlyReadyLocked()
	return true
}

// MarkActive transitions a node from Ready to Active and stamps StartedAt.
// LastProgressAt is set to StartedAt so stall detection has a grace
// period from execution start (not from node creation, which may include
// queue time). Returns true if the transition happened.
func (d *DAG) MarkActive(id string) bool {
	return d.Update(id, func(n *Node) {
		if n.Status == StatusReady {
			n.Status = StatusActive
			now := time.Now()
			n.StartedAt = now
			n.LastProgressAt = now
		}
	})
}

// NoteProgress stamps LastProgressAt = now and clears StallNotedAt. The
// reconciler calls this whenever it confirms a prediction belonging to
// the node, so stall detection resets every time we see the work moving.
// Returns true if the node existed and is currently Active.
func (d *DAG) NoteProgress(id string) bool {
	return d.Update(id, func(n *Node) {
		if n.Status != StatusActive {
			return
		}
		n.LastProgressAt = time.Now()
		n.StallNotedAt = time.Time{}
	})
}

// NoteStall stamps StallNotedAt = now. Used by the reconciler to dedupe
// stall events: it only emits a fresh stall divergence when StallNotedAt
// is zero (or older than a re-notify interval).
func (d *DAG) NoteStall(id string) bool {
	return d.Update(id, func(n *Node) {
		if n.Status != StatusActive {
			return
		}
		n.StallNotedAt = time.Now()
	})
}

// MarkDone transitions a node to Done and stamps DoneAt. Successors that
// become Ready as a result are promoted automatically.
func (d *DAG) MarkDone(id string) bool {
	return d.Update(id, func(n *Node) {
		if n.IsTerminal() {
			return
		}
		n.Status = StatusDone
		n.DoneAt = time.Now()
	})
}

// MarkFailed transitions a node to Failed with a reason. Successors stay
// Pending forever (or until manually re-added) — the deliberator decides
// whether to retry by emitting a new node next turn.
func (d *DAG) MarkFailed(id, reason string) bool {
	return d.Update(id, func(n *Node) {
		if n.IsTerminal() {
			return
		}
		n.Status = StatusFailed
		n.Result = reason
		n.DoneAt = time.Now()
	})
}

// Ready returns copies of all nodes whose Status is Ready, in insertion
// order. The executor drains this list each pass.
func (d *DAG) Ready() []Node {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.filterByStatusLocked(StatusReady)
}

// Active returns copies of all Active nodes.
func (d *DAG) Active() []Node {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.filterByStatusLocked(StatusActive)
}

// Pending returns copies of all Pending nodes.
func (d *DAG) Pending() []Node {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.filterByStatusLocked(StatusPending)
}

// Recent returns the most recently inserted n nodes (any status), newest
// first. Used by the renderer to show what the deliberator just committed.
func (d *DAG) Recent(n int) []Node {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if n <= 0 || len(d.insertOrder) == 0 {
		return nil
	}
	if n > len(d.insertOrder) {
		n = len(d.insertOrder)
	}
	out := make([]Node, 0, n)
	for i := len(d.insertOrder) - 1; i >= len(d.insertOrder)-n; i-- {
		if node, ok := d.nodes[d.insertOrder[i]]; ok {
			out = append(out, *node)
		}
	}
	return out
}

// Stats returns a count-by-status snapshot.
func (d *DAG) Stats() Stats {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var s Stats
	for _, n := range d.nodes {
		switch n.Status {
		case StatusPending:
			s.Pending++
		case StatusReady:
			s.Ready++
		case StatusActive:
			s.Active++
		case StatusDone:
			s.Done++
		case StatusFailed:
			s.Failed++
		}
	}
	return s
}

// PendingNodes / ActiveNodes implement worldmodel.PlanReader so the
// WorldModel snapshot can include them when the deliberator renders.
func (d *DAG) PendingNodes() []worldmodel.PlanNodeView {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.viewByStatusLocked(StatusPending, StatusReady)
}

func (d *DAG) ActiveNodes() []worldmodel.PlanNodeView {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.viewByStatusLocked(StatusActive)
}

// allDepsDoneLocked reports whether every dependency ID in deps refers to a
// node currently in StatusDone. Caller must hold the lock.
func (d *DAG) allDepsDoneLocked(deps []string) bool {
	for _, depID := range deps {
		dep, ok := d.nodes[depID]
		if !ok || dep.Status != StatusDone {
			return false
		}
	}
	return true
}

// promoteNewlyReadyLocked walks Pending nodes and flips any whose deps are
// all Done. Caller must hold the lock.
func (d *DAG) promoteNewlyReadyLocked() {
	for _, n := range d.nodes {
		if n.Status == StatusPending && d.allDepsDoneLocked(n.Dependencies) {
			n.Status = StatusReady
		}
	}
}

func (d *DAG) filterByStatusLocked(statuses ...Status) []Node {
	out := make([]Node, 0)
	for _, id := range d.insertOrder {
		n, ok := d.nodes[id]
		if !ok {
			continue
		}
		for _, s := range statuses {
			if n.Status == s {
				out = append(out, *n)
				break
			}
		}
	}
	return out
}

func (d *DAG) viewByStatusLocked(statuses ...Status) []worldmodel.PlanNodeView {
	out := make([]worldmodel.PlanNodeView, 0)
	for _, id := range d.insertOrder {
		n, ok := d.nodes[id]
		if !ok {
			continue
		}
		match := false
		for _, s := range statuses {
			if n.Status == s {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		view := worldmodel.PlanNodeView{
			ID:       n.ID,
			Action:   n.Action.Type,
			Priority: 0,
			Status:   n.Status.String(),
		}
		if n.Action.Type == "dig" {
			c := worldmodel.Coord{X: n.Action.Region.X1, Y: n.Action.Region.Y1, Z: n.Action.Region.Z1}
			view.TargetTile = &c
		}
		out = append(out, view)
	}
	return out
}
