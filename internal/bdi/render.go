package bdi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/df-ai/orchestrator/internal/plan"
	"github.com/df-ai/orchestrator/internal/predicate"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/reconcile"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// RenderInput bundles everything the renderer needs. The loop populates
// this each cycle, the renderer turns it into a markdown+JSON prompt body.
type RenderInput struct {
	WM                *worldmodel.WorldModel
	Predicates        []predicate.Result
	PlanStats         plan.Stats
	RecentNodes       []plan.Node
	RecentDivergences []reconcile.DivergenceEvent
	ToolCatalog       string // pre-rendered markdown from query.Registry.CatalogMarkdown()
	SkillCatalog      string // pre-rendered markdown from skill.Library.RenderForPrompt()
}

// RenderOptions tunes the level of detail in the rendered prompt.
type RenderOptions struct {
	IncludeDwarfList     bool
	MaxDwarfList         int
	IncludePredictedList bool
	MaxPredictedList     int
	IncludeTopology      bool
	MaxRecentNodes       int
	MaxRecentDivergences int
}

// DefaultRenderOptions is what the BDI loop uses for first-pass cycles.
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{
		IncludeDwarfList:     true,
		MaxDwarfList:         10,
		IncludePredictedList: true,
		MaxPredictedList:     10,
		IncludeTopology:      true,
		MaxRecentNodes:       8,
		MaxRecentDivergences: 5,
	}
}

// Render produces the user-message body that the deliberator sends alongside
// SystemPrompt. The output is a Markdown document with one JSON block (for
// structured fields) and short bulleted summaries for orientation.
func Render(in RenderInput, opts RenderOptions) string {
	if in.WM == nil {
		return "(world model nil)\n"
	}
	snap := in.WM.Snapshot()

	var sb strings.Builder
	writeHeader(&sb, snap)
	if in.ToolCatalog != "" {
		sb.WriteString(in.ToolCatalog)
	}
	if in.SkillCatalog != "" {
		sb.WriteString(in.SkillCatalog)
	}
	writePredicateBlock(&sb, in.Predicates)
	writeWorldBlock(&sb, in.WM, snap, opts)
	writePlanBlock(&sb, in.PlanStats, in.RecentNodes, opts)
	writePredictedBlock(&sb, snap, opts)
	// Stall events are no longer surfaced to the LLM. DF announcements
	// (snapshot's "alerts" block) are the canonical signal for "why is
	// this not progressing". The reconciler still tracks divergences for
	// debug logs and predicate confidence, but they don't reach the LLM.
	return sb.String()
}

func writeHeader(sb *strings.Builder, snap worldmodel.Snapshot) {
	sb.WriteString("# Fort State\n\n")
	fmt.Fprintf(sb, "- tick: %d\n", snap.Tick)
	fmt.Fprintf(sb, "- wall_clock: %s\n", snap.TakenAt.Format(time.RFC3339))
	if snap.Fort.Valid {
		fmt.Fprintf(sb, "- fort_age_days: %d\n", snap.Fort.DaysElapsed)
		fmt.Fprintf(sb, "- season: %s\n", seasonName(snap.Fort.Season))
		fmt.Fprintf(sb, "- year: %d\n", snap.Fort.Year)
		fmt.Fprintf(sb, "- created_wealth: %d\n", snap.Fort.CreatedWealth)
	} else {
		sb.WriteString("- fort_age_days: (not yet observed)\n")
	}
	sb.WriteString("\n")
}

func writePredicateBlock(sb *strings.Builder, results []predicate.Result) {
	sb.WriteString("## Goals\n\n")

	if len(results) == 0 {
		sb.WriteString("(no predicates registered)\n\n")
		return
	}

	for _, h := range []predicate.Horizon{predicate.HorizonNow, predicate.HorizonSoon, predicate.HorizonEventual} {
		filtered := predicate.FilterByHorizon(results, h)
		if len(filtered) == 0 {
			continue
		}
		fmt.Fprintf(sb, "### %s\n", strings.ToUpper(h.String()))
		for _, r := range filtered {
			marker := "[ ]"
			if r.Satisfied {
				marker = "[x]"
			}
			fmt.Fprintf(sb, "- %s **%s** (confidence %.2f)\n", marker, r.Name, r.Confidence)
			for _, ev := range r.Evidence {
				fmt.Fprintf(sb, "    - %s\n", ev)
			}
		}
		sb.WriteString("\n")
	}
}

func writeWorldBlock(sb *strings.Builder, wm *worldmodel.WorldModel, snap worldmodel.Snapshot, opts RenderOptions) {
	sb.WriteString("## Observed\n\n")
	sb.WriteString("```json\n")

	view := buildObservedView(wm, snap, opts)
	bytes, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		fmt.Fprintf(sb, "(render error: %v)\n", err)
	} else {
		sb.Write(bytes)
		sb.WriteString("\n")
	}
	sb.WriteString("```\n\n")
}

func writePlanBlock(sb *strings.Builder, stats plan.Stats, recent []plan.Node, opts RenderOptions) {
	if stats.Pending+stats.Ready+stats.Active+stats.Done+stats.Failed == 0 {
		return
	}

	sb.WriteString("## Plan\n\n")
	fmt.Fprintf(sb, "- pending: %d\n", stats.Pending)
	fmt.Fprintf(sb, "- ready: %d\n", stats.Ready)
	fmt.Fprintf(sb, "- active: %d\n", stats.Active)
	fmt.Fprintf(sb, "- done: %d\n", stats.Done)
	fmt.Fprintf(sb, "- failed: %d\n", stats.Failed)

	if len(recent) > 0 && opts.MaxRecentNodes > 0 {
		limit := opts.MaxRecentNodes
		if limit > len(recent) {
			limit = len(recent)
		}
		sb.WriteString("\nRecent nodes (newest first):\n")
		for i := 0; i < limit; i++ {
			n := recent[i]
			fmt.Fprintf(sb, "- %s [%s] %s — %s",
				n.ID, n.Status.String(), describeAction(n.Action), n.Result)
			if n.PredictionCount > 0 {
				fmt.Fprintf(sb, " (predictions %d/%d confirmed, %d diverged)",
					n.PredictionsConfirmed, n.PredictionCount, n.PredictionsDiverged)
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")
}

func writePredictedBlock(sb *strings.Builder, snap worldmodel.Snapshot, opts RenderOptions) {
	if !opts.IncludePredictedList {
		return
	}
	if snap.Predictions.OutstandingCount == 0 && snap.Predictions.DivergedCount == 0 {
		return
	}

	sb.WriteString("## Predicted (in-flight)\n\n")
	fmt.Fprintf(sb, "- outstanding: %d\n", snap.Predictions.OutstandingCount)
	fmt.Fprintf(sb, "- confirmed: %d\n", snap.Predictions.ConfirmedCount)
	fmt.Fprintf(sb, "- diverged: %d\n", snap.Predictions.DivergedCount)

	if len(snap.Predictions.Outstanding) > 0 {
		sb.WriteString("\nOutstanding (sample):\n")
		limit := opts.MaxPredictedList
		if limit <= 0 || limit > len(snap.Predictions.Outstanding) {
			limit = len(snap.Predictions.Outstanding)
		}
		for i := 0; i < limit; i++ {
			p := snap.Predictions.Outstanding[i]
			fmt.Fprintf(sb, "- %s @ (%d,%d,%d) age=%s plan_node=%s\n",
				p.Action, p.Coord.X, p.Coord.Y, p.Coord.Z,
				formatAge(p.PredictedAt, snap.TakenAt),
				p.PlanNodeID)
		}
	}
	sb.WriteString("\n")
}

// observedView is the structured slice of the snapshot worth handing to the
// LLM. Topology / hazards / modifications are summaries, not dumps.
type observedView struct {
	Tick          uint64            `json:"tick"`
	Orientation   *orientationView  `json:"orientation,omitempty"`
	DwarfCount    int               `json:"dwarf_count"`
	EnemyCount    int               `json:"enemy_count"`
	AnimalCount   int               `json:"animal_count"`
	Dwarves       []entityView      `json:"dwarves,omitempty"`
	Enemies       []entityView      `json:"enemies,omitempty"`
	Hazards       map[string]uint32 `json:"hazards,omitempty"`
	Modifications modificationsView `json:"modifications"`
	Topology      *topologyView     `json:"topology,omitempty"`
	Zones         zoneCountsView    `json:"zones"`
	Alerts        *alertsView       `json:"alerts,omitempty"`
}

// alertsView surfaces DF announcements (cancellations, ambushes, migrant
// arrivals, ...). These are the game's own player-facing diagnostic stream
// and the primary signal the LLM should react to when work isn't
// progressing.
type alertsView struct {
	ActiveCount    int          `json:"active_count"`
	DismissedCount int          `json:"dismissed_count"`
	Active         []alertEntry `json:"active,omitempty"`
}

type alertEntry struct {
	ID       uint32 `json:"id"`
	Severity string `json:"severity"` // "info" | "warn" | "critical"
	Text     string `json:"text"`
	Pos      string `json:"pos,omitempty"` // "(x,y,z)" if positional
	Age      string `json:"age,omitempty"` // wall-clock age, e.g. "12s"
}

// orientationView anchors the LLM in the map's coordinate system.
//
// Without this, the LLM has no way to tell whether Z=111 is "the surface"
// or "deep underground" — both are plausible without context. The Z axis
// in DF runs upward (higher Z = above sea level / sky), so the surface
// is typically a few Z below the top of the map. Dwarves on a fresh
// embark stand on that surface Z.
type orientationView struct {
	MapDims   string `json:"map_dims"`    // "WxHxD"
	ZTop      int16  `json:"z_top"`       // highest Z index in the map (sky)
	ZBottom   int16  `json:"z_bottom"`    // lowest Z (typically 0, deepest stone)
	MinDwarfZ int16  `json:"min_dwarf_z"` // lowest Z any dwarf is currently on
	MaxDwarfZ int16  `json:"max_dwarf_z"` // highest Z any dwarf is currently on
	Note      string `json:"note"`        // plain-English orientation hint
}

// zoneCountsView breaks down zones by type for predicate reasoning.
type zoneCountsView struct {
	Total    int `json:"total"`
	Bedroom  int `json:"bedroom"`
	Dining   int `json:"dining"`
	Meeting  int `json:"meeting"`
	Barracks int `json:"barracks"`
	Other    int `json:"other"`
}

type entityView struct {
	ID uint32 `json:"id"`
	X  int16  `json:"x"`
	Y  int16  `json:"y"`
	Z  int16  `json:"z"`
}

type modificationsView struct {
	Count uint32 `json:"count"`
}

type topologyView struct {
	// OpenPercentage is the fraction of KNOWN tiles that are open,
	// across the whole map. Excludes unknown / unallocated tiles.
	OpenPercentage float64     `json:"open_percentage"`
	MemoryKB       uint64      `json:"memory_kb"`
	OpenTiles      uint32      `json:"open_tiles"`
	ClosedTiles    uint32      `json:"closed_tiles"`
	UnknownTiles   uint32      `json:"unknown_tiles"`
	ByZ            []topoZView `json:"by_z,omitempty"` // band around the dwarves
}

// topoZView is one Z-level's open/closed/unknown breakdown plus a
// human-readable note ("your dwarves' Z", "above surface", etc.).
type topoZView struct {
	Z       int16   `json:"z"`
	OpenPct float64 `json:"open_pct"`
	Open    uint32  `json:"open"`
	Closed  uint32  `json:"closed"`
	Unknown uint32  `json:"unknown"`
	Note    string  `json:"note,omitempty"`
}

func buildObservedView(wm *worldmodel.WorldModel, snap worldmodel.Snapshot, opts RenderOptions) observedView {
	zoneCounts := zoneCountsView{Total: len(snap.Zones.All)}
	for _, z := range snap.Zones.All {
		switch z.ZoneType {
		case 0x01:
			zoneCounts.Bedroom++
		case 0x02:
			zoneCounts.Dining++
		case 0x03:
			zoneCounts.Meeting++
		case 0x04:
			zoneCounts.Barracks++
		default:
			zoneCounts.Other++
		}
	}

	v := observedView{
		Tick:        snap.Tick,
		DwarfCount:  len(snap.Entities.Dwarves),
		EnemyCount:  len(snap.Entities.Enemies),
		AnimalCount: len(snap.Entities.Animals),
		Zones:       zoneCounts,
	}

	if opts.IncludeDwarfList {
		v.Dwarves = sampleEntities(snap.Entities.Dwarves, opts.MaxDwarfList)
	}
	v.Enemies = sampleEntities(snap.Entities.Enemies, opts.MaxDwarfList)

	if wm.Observed.Modifications != nil {
		v.Modifications.Count = wm.Observed.Modifications.GetCount()
	}

	if wm.Observed.Hazards != nil {
		counts := wm.Observed.Hazards.GetAllCounts()
		v.Hazards = make(map[string]uint32, len(counts))
		for _, k := range sortedKeys(counts) {
			v.Hazards[k] = counts[k]
		}
	}

	if opts.IncludeTopology && wm.Observed.Topology != nil {
		open, closed, unknown := wm.Observed.Topology.Counts()
		v.Topology = &topologyView{
			OpenPercentage: wm.Observed.Topology.GetOpenPercentage(),
			MemoryKB:       wm.Observed.Topology.GetMemoryUsage() / 1024,
			OpenTiles:      open,
			ClosedTiles:    closed,
			UnknownTiles:   unknown,
			ByZ:            buildPerZTopology(wm, snap),
		}
	}

	v.Orientation = buildOrientationView(wm, snap)
	v.Alerts = buildAlertsView(wm, snap)

	return v
}

// buildPerZTopology returns a band of per-Z stats covering the dwarves'
// elevation plus a few above/below. This anchors the LLM to "your Z" vs
// "the rest of the map" — a single global open percentage was structurally
// dominated by unallocated underground tiles and led the agent to think
// it was sealed in walls when it was actually standing in a meadow.
//
// Band: 3 above max dwarf Z (treetops / sky), all dwarf Zs, 5 below min
// dwarf Z (immediate digging zone). If no dwarves yet, returns nil.
func buildPerZTopology(wm *worldmodel.WorldModel, snap worldmodel.Snapshot) []topoZView {
	if wm == nil || wm.Observed.Topology == nil {
		return nil
	}
	dwarves := snap.Entities.Dwarves
	if len(dwarves) == 0 {
		return nil
	}

	minDZ, maxDZ := dwarves[0].Z, dwarves[0].Z
	for _, e := range dwarves {
		if e.Z < minDZ {
			minDZ = e.Z
		}
		if e.Z > maxDZ {
			maxDZ = e.Z
		}
	}

	zMax := maxDZ + 3
	zMin := minDZ - 5
	stats := wm.Observed.Topology.StatsForZRange(zMin, zMax)

	out := make([]topoZView, 0, len(stats))
	for _, s := range stats {
		view := topoZView{
			Z:       s.Z,
			OpenPct: s.OpenPercent(),
			Open:    s.Open,
			Closed:  s.Closed,
			Unknown: s.Unknown,
			Note:    annotateZ(s.Z, minDZ, maxDZ),
		}
		out = append(out, view)
	}
	return out
}

// annotateZ produces a one-line hint for a Z level relative to the
// dwarves' band. Helps the LLM read the per-Z stats without needing
// to compute "is Z=111 above or below my dwarves" each time.
func annotateZ(z, minDZ, maxDZ int16) string {
	switch {
	case z > maxDZ:
		diff := z - maxDZ
		if diff == 1 {
			return "1 above dwarves (treetops / sky / above ground)"
		}
		return fmt.Sprintf("%d above dwarves (sky)", diff)
	case z >= minDZ && z <= maxDZ:
		if z == maxDZ && z == minDZ {
			return "your dwarves' Z (likely embark surface)"
		}
		return "dwarf Z (terrain elevation band)"
	default:
		diff := minDZ - z
		if diff == 1 {
			return "1 below dwarves (first soil / dig target)"
		}
		return fmt.Sprintf("%d below dwarves (deeper soil / stone)", diff)
	}
}

// buildAlertsView renders the active DF announcements + counts. Returns nil
// if there's nothing to show (no store, no alerts).
func buildAlertsView(wm *worldmodel.WorldModel, snap worldmodel.Snapshot) *alertsView {
	if wm == nil || wm.Observed.Alerts == nil {
		return nil
	}
	active, dismissed, _ := wm.Observed.Alerts.Counts()
	if active == 0 && dismissed == 0 {
		return nil
	}
	out := &alertsView{
		ActiveCount:    active,
		DismissedCount: dismissed,
	}
	now := time.Now()
	for _, a := range snap.ActiveAlerts {
		entry := alertEntry{
			ID:       a.ID,
			Severity: severityName(a.Severity),
			Text:     a.Text,
			Age:      formatAge(a.ReceivedAt, now),
		}
		if a.HasPosition() {
			entry.Pos = fmt.Sprintf("(%d,%d,%d)", a.X, a.Y, a.Z)
		}
		out.Active = append(out.Active, entry)
	}
	return out
}

func severityName(s uint8) string {
	switch s {
	case 2:
		return "critical"
	case 1:
		return "warn"
	default:
		return "info"
	}
}

// buildOrientationView anchors the LLM in DF's coordinate system. Returns
// nil if we don't have enough info to be useful (no map dims yet).
func buildOrientationView(wm *worldmodel.WorldModel, snap worldmodel.Snapshot) *orientationView {
	if wm == nil || wm.Observed.Topology == nil {
		return nil
	}
	w, h, d := wm.Observed.Topology.GetDimensions()
	if w == 0 || h == 0 || d == 0 {
		return nil
	}

	zTop := int16(d - 1)
	zBottom := int16(0)

	view := &orientationView{
		MapDims: fmt.Sprintf("%dx%dx%d", w, h, d),
		ZTop:    zTop,
		ZBottom: zBottom,
	}

	dwarves := snap.Entities.Dwarves
	if len(dwarves) == 0 {
		view.Note = fmt.Sprintf("Z axis runs upward: Z=%d is the top (sky), Z=%d is deepest stone. No dwarves observed yet.", zTop, zBottom)
		return view
	}

	minZ, maxZ := dwarves[0].Z, dwarves[0].Z
	for _, e := range dwarves {
		if e.Z < minZ {
			minZ = e.Z
		}
		if e.Z > maxZ {
			maxZ = e.Z
		}
	}
	view.MinDwarfZ = minZ
	view.MaxDwarfZ = maxZ

	// Heuristic: on a fresh embark, dwarves stand on the surface. The
	// surface Z is therefore ~= max dwarf Z. Tiles ABOVE that Z are sky
	// or treetops; tiles BELOW are unexplored deep stone (until you dig).
	view.Note = fmt.Sprintf(
		"Z axis runs upward: Z=%d is the top (sky), Z=%d is deepest stone. Your dwarves are at Z=%d (with up to %d Z above and %d Z below). On a fresh embark, max dwarf Z is the embark SURFACE — tiles below it are unexplored stone, NOT 'deep underground' just because the topology summary says they're closed. Dig downward to expose them.",
		zTop, zBottom, maxZ, zTop-maxZ, maxZ-zBottom,
	)
	return view
}

func sampleEntities(entities []protocol.EntityInfo, limit int) []entityView {
	if len(entities) == 0 {
		return nil
	}
	if limit <= 0 || limit > len(entities) {
		limit = len(entities)
	}
	out := make([]entityView, 0, limit)
	for i := 0; i < limit; i++ {
		e := entities[i]
		out = append(out, entityView{ID: e.ID, X: e.X, Y: e.Y, Z: e.Z})
	}
	return out
}

func describeAction(a plan.Action) string {
	if a.Description != "" {
		return a.Description
	}
	switch a.Type {
	case "dig":
		return fmt.Sprintf("dig %s (%d,%d,%d)→(%d,%d,%d)",
			a.DigType,
			a.Region.X1, a.Region.Y1, a.Region.Z1,
			a.Region.X2, a.Region.Y2, a.Region.Z2)
	case "chop", "gather":
		return fmt.Sprintf("%s (%d,%d,%d)→(%d,%d,%d)",
			a.Type,
			a.Region.X1, a.Region.Y1, a.Region.Z1,
			a.Region.X2, a.Region.Y2, a.Region.Z2)
	case "wait":
		return "wait"
	case "dismiss":
		if a.AlertID == 0 {
			return "dismiss all alerts"
		}
		return fmt.Sprintf("dismiss alert %d", a.AlertID)
	default:
		return a.Type
	}
}

func formatAge(start, now time.Time) string {
	if start.IsZero() {
		return "?"
	}
	age := now.Sub(start)
	if age < 0 {
		age = 0
	}
	return age.Round(time.Second).String()
}

func seasonName(season uint8) string {
	switch season {
	case 0:
		return "spring"
	case 1:
		return "summer"
	case 2:
		return "autumn"
	case 3:
		return "winter"
	default:
		return "unknown"
	}
}

func sortedKeys(m map[string]uint32) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
