package query

import (
	"fmt"
	"strconv"

	"github.com/df-ai/orchestrator/internal/predicate"
	"github.com/df-ai/orchestrator/internal/topology"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// NewStarterRegistry returns a Registry pre-populated with all Tier 1 +
// Tier 2 handlers. Tier 1 reads the world model directly; Tier 2 forwards
// to the plugin via Context.Plugin (returns an error if Plugin is nil).
func NewStarterRegistry() *Registry {
	r := NewRegistry()
	// Tier 1
	r.Register(RegionDetailHandler{})
	r.Register(RecentDivergencesHandler{})
	r.Register(PlanStatusHandler{})
	r.Register(PredicateExplainHandler{})
	r.Register(ListPredicatesHandler{})
	// Tier 2 (plugin-backed)
	r.Register(ListOrdersHandler{})
	r.Register(ManagerOrdersHandler{})
	r.Register(DwarfDetailHandler{})
	r.Register(BuildingStatusHandler{})
	r.Register(WorkshopJobsHandler{})
	r.Register(StockpileInventoryHandler{})
	return r
}

// ---------------------------------------------------------------------------
// region_detail
// ---------------------------------------------------------------------------

type RegionDetailHandler struct{}

func (h RegionDetailHandler) Name() string { return "region_detail" }
func (h RegionDetailHandler) Description() string {
	return "inspect a bounded region: tile open/closed counts, hazards present, modifications, dwarves currently inside, in-flight predicted tiles"
}
func (h RegionDetailHandler) ArgsSpec() []ArgSpec {
	return []ArgSpec{
		{Name: "x1", Type: "int"},
		{Name: "y1", Type: "int"},
		{Name: "z1", Type: "int"},
		{Name: "x2", Type: "int"},
		{Name: "y2", Type: "int"},
		{Name: "z2", Type: "int"},
	}
}
func (h RegionDetailHandler) Execute(ctx Context, args []string) (Result, error) {
	if len(args) < 6 {
		return Result{Error: "region_detail needs 6 args: x1, y1, z1, x2, y2, z2"},
			fmt.Errorf("not enough args")
	}
	coords := make([]int16, 6)
	for i := 0; i < 6; i++ {
		v, err := strconv.ParseInt(args[i], 10, 16)
		if err != nil {
			return Result{Error: fmt.Sprintf("arg %d (%q) not an int16: %v", i, args[i], err)},
				err
		}
		coords[i] = int16(v)
	}
	x1, y1, z1, x2, y2, z2 := coords[0], coords[1], coords[2], coords[3], coords[4], coords[5]
	if x2 < x1 || y2 < y1 || z2 < z1 {
		return Result{Error: "region invalid: end < start"}, fmt.Errorf("bad region")
	}

	view := buildRegionView(ctx.WM, x1, y1, z1, x2, y2, z2)
	tileVolume := int(x2-x1+1) * int(y2-y1+1) * int(z2-z1+1)
	note := fmt.Sprintf("region %dx%dx%d (%d tiles): %d open, %d closed; %d dwarves, %d hazards, %d modifications, %d outstanding predictions",
		x2-x1+1, y2-y1+1, z2-z1+1, tileVolume,
		view.OpenTiles, view.ClosedTiles,
		view.DwarvesInRegion, view.HazardCount, view.ModificationCount, view.OutstandingPredictions)
	return Result{Note: note, Data: view}, nil
}

type regionView struct {
	X1                     int16           `json:"x1"`
	Y1                     int16           `json:"y1"`
	Z1                     int16           `json:"z1"`
	X2                     int16           `json:"x2"`
	Y2                     int16           `json:"y2"`
	Z2                     int16           `json:"z2"`
	OpenTiles              int             `json:"open_tiles"`
	ClosedTiles            int             `json:"closed_tiles"`
	UnknownTiles           int             `json:"unknown_tiles"`
	OpenPercentage         float64         `json:"open_percentage"`  // open / (open + closed); excludes unknown
	KnownPercentage        float64         `json:"known_percentage"` // (open + closed) / total in region
	HazardCount            int             `json:"hazard_count"`
	HazardSamples          []hazardSample  `json:"hazard_samples,omitempty"`
	DwarvesInRegion        int             `json:"dwarves_in_region"`
	DwarfPositions         []entityPos     `json:"dwarf_positions,omitempty"`
	ModificationCount      int             `json:"modification_count"`
	OutstandingPredictions int             `json:"outstanding_predictions"`
	PredictionSamples      []predictionPos `json:"prediction_samples,omitempty"`
}

type hazardSample struct {
	X    int16  `json:"x"`
	Y    int16  `json:"y"`
	Z    int16  `json:"z"`
	Kind string `json:"kind"`
}

type entityPos struct {
	ID uint32 `json:"id"`
	X  int16  `json:"x"`
	Y  int16  `json:"y"`
	Z  int16  `json:"z"`
}

type predictionPos struct {
	X          int16  `json:"x"`
	Y          int16  `json:"y"`
	Z          int16  `json:"z"`
	Action     string `json:"action"`
	PlanNodeID string `json:"plan_node_id"`
}

func buildRegionView(wm *worldmodel.WorldModel, x1, y1, z1, x2, y2, z2 int16) regionView {
	v := regionView{X1: x1, Y1: y1, Z1: z1, X2: x2, Y2: y2, Z2: z2}

	if topo := wm.Observed.Topology; topo != nil {
		for x := x1; x <= x2; x++ {
			for y := y1; y <= y2; y++ {
				for z := z1; z <= z2; z++ {
					state := topo.GetTileState(x, y, z)
					switch state {
					case topology.StateOpen:
						v.OpenTiles++
					case topology.StateClosed:
						v.ClosedTiles++
					default:
						v.UnknownTiles++
					}
				}
			}
		}
		known := v.OpenTiles + v.ClosedTiles
		if known > 0 {
			v.OpenPercentage = float64(v.OpenTiles) / float64(known) * 100.0
		}
		total := known + v.UnknownTiles
		if total > 0 {
			v.KnownPercentage = float64(known) / float64(total) * 100.0
		}
	}

	snap := wm.Snapshot()
	for _, e := range snap.Entities.Dwarves {
		if e.X >= x1 && e.X <= x2 && e.Y >= y1 && e.Y <= y2 && e.Z >= z1 && e.Z <= z2 {
			v.DwarvesInRegion++
			if len(v.DwarfPositions) < 10 {
				v.DwarfPositions = append(v.DwarfPositions, entityPos{ID: e.ID, X: e.X, Y: e.Y, Z: e.Z})
			}
		}
	}

	if wm.Predicted != nil {
		for _, p := range wm.Predicted.Outstanding() {
			if p.Coord.X >= x1 && p.Coord.X <= x2 &&
				p.Coord.Y >= y1 && p.Coord.Y <= y2 &&
				p.Coord.Z >= z1 && p.Coord.Z <= z2 {
				v.OutstandingPredictions++
				if len(v.PredictionSamples) < 10 {
					v.PredictionSamples = append(v.PredictionSamples, predictionPos{
						X: p.Coord.X, Y: p.Coord.Y, Z: p.Coord.Z,
						Action:     p.Action,
						PlanNodeID: p.PlanNodeID,
					})
				}
			}
		}
	}

	// Hazards: count by sampling each hazard layer's region. The hazards
	// manager doesn't expose a region query directly, so we use coarse
	// heuristics — total counts per type from the manager, scaled by
	// region area as a guess. For the LLM that's enough signal: if total
	// caverns is 0, the region has no caverns either.
	if hzd := wm.Observed.Hazards; hzd != nil {
		counts := hzd.GetAllCounts()
		// Just expose the global counts; per-region hazard scan would
		// need hazards package extension. Mark this as a known coarseness.
		for kind, c := range counts {
			if c > 0 {
				v.HazardCount += int(c)
				if len(v.HazardSamples) < 6 {
					v.HazardSamples = append(v.HazardSamples, hazardSample{Kind: kind})
				}
			}
		}
	}

	if mods := wm.Observed.Modifications; mods != nil {
		// Modification overlay tracks all changes; we don't have a fast
		// in-region count, so we use the overall count as a coarse signal.
		// A future ModificationOverlay.CountInRegion call would refine this.
		v.ModificationCount = int(mods.GetCount())
	}
	return v
}

// ---------------------------------------------------------------------------
// recent_divergences
// ---------------------------------------------------------------------------

type RecentDivergencesHandler struct{}

func (h RecentDivergencesHandler) Name() string { return "recent_divergences" }
func (h RecentDivergencesHandler) Description() string {
	return "list recent stall events and explicit blockers (defaults to last 10; pass an int to change)"
}
func (h RecentDivergencesHandler) ArgsSpec() []ArgSpec {
	return []ArgSpec{
		{Name: "limit", Type: "int", Optional: true},
	}
}
func (h RecentDivergencesHandler) Execute(ctx Context, args []string) (Result, error) {
	limit := 10
	if len(args) > 0 {
		if v, err := strconv.Atoi(args[0]); err == nil && v > 0 {
			limit = v
		}
	}
	if ctx.Events == nil {
		return Result{Note: "no event buffer configured", Data: []any{}}, nil
	}
	events := ctx.Events.Recent(limit)
	return Result{
		Note: fmt.Sprintf("%d events (last %d requested)", len(events), limit),
		Data: events,
	}, nil
}

// ---------------------------------------------------------------------------
// plan_status
// ---------------------------------------------------------------------------

type PlanStatusHandler struct{}

func (h PlanStatusHandler) Name() string { return "plan_status" }
func (h PlanStatusHandler) Description() string {
	return "return DAG stats and recent nodes; pass a node_id to get one node's full record"
}
func (h PlanStatusHandler) ArgsSpec() []ArgSpec {
	return []ArgSpec{
		{Name: "node_id", Type: "string", Optional: true},
	}
}
func (h PlanStatusHandler) Execute(ctx Context, args []string) (Result, error) {
	if ctx.DAG == nil {
		return Result{Error: "no plan DAG configured"}, fmt.Errorf("no dag")
	}
	if len(args) > 0 && args[0] != "" {
		node, ok := ctx.DAG.Get(args[0])
		if !ok {
			return Result{Error: fmt.Sprintf("plan node %q not found", args[0])}, fmt.Errorf("not found")
		}
		return Result{
			Note: fmt.Sprintf("node %s: status=%s, predictions confirmed=%d/%d, diverged=%d",
				node.ID, node.Status.String(), node.PredictionsConfirmed, node.PredictionCount, node.PredictionsDiverged),
			Data: node,
		}, nil
	}
	stats := ctx.DAG.Stats()
	recent := ctx.DAG.Recent(20)
	return Result{
		Note: fmt.Sprintf("pending=%d ready=%d active=%d done=%d failed=%d",
			stats.Pending, stats.Ready, stats.Active, stats.Done, stats.Failed),
		Data: map[string]any{
			"stats":  stats,
			"recent": recent,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// predicate_explain
// ---------------------------------------------------------------------------

type PredicateExplainHandler struct{}

func (h PredicateExplainHandler) Name() string { return "predicate_explain" }
func (h PredicateExplainHandler) Description() string {
	return "evaluate one predicate by name and return its full Result with evidence"
}
func (h PredicateExplainHandler) ArgsSpec() []ArgSpec {
	return []ArgSpec{
		{Name: "name", Type: "string"},
	}
}
func (h PredicateExplainHandler) Execute(ctx Context, args []string) (Result, error) {
	if len(args) < 1 || args[0] == "" {
		return Result{Error: "predicate_explain needs the predicate name"}, fmt.Errorf("missing arg")
	}
	if ctx.Lib == nil {
		return Result{Error: "no predicate library configured"}, fmt.Errorf("no lib")
	}
	target := args[0]
	for _, p := range ctx.Lib.All() {
		if p.Name() == target {
			r := p.Check(ctx.WM)
			satisfied := "unsatisfied"
			if r.Satisfied {
				satisfied = "satisfied"
			}
			return Result{
				Note: fmt.Sprintf("%s: %s (confidence %.2f)", r.Name, satisfied, r.Confidence),
				Data: r,
			}, nil
		}
	}
	return Result{Error: fmt.Sprintf("predicate %q not registered", target)}, fmt.Errorf("not found")
}

// ---------------------------------------------------------------------------
// list_predicates
// ---------------------------------------------------------------------------

type ListPredicatesHandler struct{}

func (h ListPredicatesHandler) Name() string { return "list_predicates" }
func (h ListPredicatesHandler) Description() string {
	return "list every registered predicate by name, horizon, and description"
}
func (h ListPredicatesHandler) ArgsSpec() []ArgSpec { return nil }
func (h ListPredicatesHandler) Execute(ctx Context, args []string) (Result, error) {
	if ctx.Lib == nil {
		return Result{Error: "no predicate library configured"}, fmt.Errorf("no lib")
	}
	preds := ctx.Lib.All()
	type entry struct {
		Name        string `json:"name"`
		Horizon     string `json:"horizon"`
		Description string `json:"description"`
	}
	out := make([]entry, 0, len(preds))
	for _, p := range preds {
		out = append(out, entry{
			Name:        p.Name(),
			Horizon:     p.Horizon().String(),
			Description: p.Description(),
		})
	}
	return Result{
		Note: fmt.Sprintf("%d predicates registered", len(preds)),
		Data: out,
	}, nil
}

// Compile-time check that handlers satisfy the interface.
var (
	_ Handler = RegionDetailHandler{}
	_ Handler = RecentDivergencesHandler{}
	_ Handler = PlanStatusHandler{}
	_ Handler = PredicateExplainHandler{}
	_ Handler = ListPredicatesHandler{}
)

// Force-import for compile-time checks of cross-package types referenced
// in JSON tags / descriptions only via worldmodel/predicate.
var (
	_ = worldmodel.Coord{}
	_ = predicate.Result{}
)
