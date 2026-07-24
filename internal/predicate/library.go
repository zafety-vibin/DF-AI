package predicate

import (
	"fmt"
	"time"

	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// HasMinDwarves is satisfied when the observed dwarf count meets a threshold.
// Used for population goals ("absorb the migrant wave").
type HasMinDwarves struct {
	Min int
	H   Horizon
}

func (p HasMinDwarves) Name() string {
	return fmt.Sprintf("has_min_dwarves_%d", p.Min)
}

func (p HasMinDwarves) Description() string {
	return fmt.Sprintf("at least %d dwarves are alive in the fort", p.Min)
}

func (p HasMinDwarves) Horizon() Horizon { return p.H }

func (p HasMinDwarves) Check(wm *worldmodel.WorldModel) Result {
	now := time.Now()
	snap := wm.Snapshot()
	count := len(snap.Entities.Dwarves)
	r := Result{
		Name:       p.Name(),
		Horizon:    p.H,
		Satisfied:  count >= p.Min,
		Confidence: confidenceFromFreshness(snap.Entities.UpdatedAt),
		CheckedAt:  now,
	}
	r.Evidence = []string{
		fmt.Sprintf("observed dwarf count: %d", count),
		fmt.Sprintf("threshold: >= %d", p.Min),
	}
	return r
}

// dugTileEvidence returns the best available dug/modified-tile count plus
// the evidence line(s) explaining its source. Prefers wm's live
// region_scan count (installed transiently by check_goals' handler just
// before CheckAll runs — see internal/mcpserver/tools_control.go and
// live_state.go's liveDugTileCount) since that reflects the CURRENT map
// state regardless of session/reconnect history; the session-delta
// Modifications overlay (session-scoped, and fed by a TILE_UPDATE stream
// that empirically delivers nothing) is always reported too, as
// supplementary evidence — never ripped out, just no longer the sole or
// preferred source.
func dugTileEvidence(wm *worldmodel.WorldModel) (int, []string) {
	overlayCount := 0
	if wm.Observed.Modifications != nil {
		overlayCount = int(wm.Observed.Modifications.GetCount())
	}
	if live := wm.LiveDugTiles(); live.Ok {
		return int(live.Count), []string{
			fmt.Sprintf("live dug-tile count (%s): %d", live.Source, live.Count),
			fmt.Sprintf("session-delta overlay count (supplementary): %d", overlayCount),
		}
	}
	return overlayCount, []string{
		fmt.Sprintf("session-delta overlay count (no live scan this check): %d", overlayCount),
	}
}

// HasShelter is a coarse early-game predicate: are there enough modified
// (dug-out) tiles to plausibly contain the dwarves indoors? A more precise
// check would require room detection, which we'll add later. For now this
// catches the "no fort exists yet" case and the "tiny bunker" failure mode.
type HasShelter struct {
	MinDugTilesPerDwarf int
	H                   Horizon
}

func (p HasShelter) Name() string {
	return fmt.Sprintf("has_shelter_%d_per_dwarf", p.MinDugTilesPerDwarf)
}

func (p HasShelter) Description() string {
	return fmt.Sprintf("at least %d dug tiles per dwarf exist (rough shelter capacity)", p.MinDugTilesPerDwarf)
}

func (p HasShelter) Horizon() Horizon { return p.H }

func (p HasShelter) Check(wm *worldmodel.WorldModel) Result {
	now := time.Now()
	snap := wm.Snapshot()
	dwarfCount := len(snap.Entities.Dwarves)
	if dwarfCount == 0 {
		return Result{
			Name:       p.Name(),
			Horizon:    p.H,
			Satisfied:  false,
			Confidence: 0.5,
			Evidence:   []string{"no dwarves observed yet"},
			CheckedAt:  now,
		}
	}

	dugTiles, sourceEvidence := dugTileEvidence(wm)

	required := dwarfCount * p.MinDugTilesPerDwarf
	r := Result{
		Name:       p.Name(),
		Horizon:    p.H,
		Satisfied:  dugTiles >= required,
		Confidence: confidenceFromFreshness(snap.Entities.UpdatedAt),
		CheckedAt:  now,
	}
	r.Evidence = append([]string{fmt.Sprintf("dwarves: %d", dwarfCount)}, sourceEvidence...)
	r.Evidence = append(r.Evidence, fmt.Sprintf("required (%d/dwarf): %d", p.MinDugTilesPerDwarf, required))
	return r
}

// HasModifiedAnything is the simplest possible predicate: have we dug or
// built ANY tile yet? It's the "is the fort started" gate. Useful as a
// pre-condition for everything else and easy to verify visually.
type HasModifiedAnything struct {
	H Horizon
}

func (p HasModifiedAnything) Name() string { return "has_modified_anything" }
func (p HasModifiedAnything) Description() string {
	return "the fort has at least one modified (dug or built) tile"
}
func (p HasModifiedAnything) Horizon() Horizon { return p.H }

func (p HasModifiedAnything) Check(wm *worldmodel.WorldModel) Result {
	now := time.Now()
	count, evidence := dugTileEvidence(wm)
	return Result{
		Name:       p.Name(),
		Horizon:    p.H,
		Satisfied:  count > 0,
		Confidence: 1.0,
		Evidence:   evidence,
		CheckedAt:  now,
	}
}

// NoActiveHostiles is a coarse defense predicate. The reconciler can use a
// flip from satisfied → unsatisfied to fire the engineer→commander mode
// switch trigger.
type NoActiveHostiles struct {
	H Horizon
}

func (p NoActiveHostiles) Name() string { return "no_active_hostiles" }
func (p NoActiveHostiles) Description() string {
	return "no hostile units are currently inside the map"
}
func (p NoActiveHostiles) Horizon() Horizon { return p.H }

func (p NoActiveHostiles) Check(wm *worldmodel.WorldModel) Result {
	now := time.Now()
	snap := wm.Snapshot()
	count := len(snap.Entities.Enemies)
	return Result{
		Name:       p.Name(),
		Horizon:    p.H,
		Satisfied:  count == 0,
		Confidence: confidenceFromFreshness(snap.Entities.UpdatedAt),
		Evidence:   []string{fmt.Sprintf("hostile count: %d", count)},
		CheckedAt:  now,
	}
}

// confidenceFromFreshness returns 1.0 if the observation is fresh
// (< 30 s old), decaying linearly to 0.5 at 5 minutes and 0.0 thereafter.
// Predicates inherit lower confidence when their data is stale, so the
// deliberator knows to discount them.
func confidenceFromFreshness(observedAt time.Time) float64 {
	if observedAt.IsZero() {
		return 0.0
	}
	age := time.Since(observedAt)
	switch {
	case age < 30*time.Second:
		return 1.0
	case age < 5*time.Minute:
		// Linear decay between 30 s and 5 min: 1.0 → 0.5
		t := float64(age-30*time.Second) / float64(5*time.Minute-30*time.Second)
		return 1.0 - 0.5*t
	case age < 30*time.Minute:
		// Slow decay between 5 min and 30 min: 0.5 → 0.0
		t := float64(age-5*time.Minute) / float64(30*time.Minute-5*time.Minute)
		return 0.5 * (1.0 - t)
	default:
		return 0.0
	}
}

// HasBedroomZones is satisfied when at least Min zones of type Bedroom
// exist in the world. Reads ZoneSnapshot from the WorldModel — zones
// arrive via ENTITY_UPDATE.
type HasBedroomZones struct {
	Min int
	H   Horizon
}

func (p HasBedroomZones) Name() string {
	return fmt.Sprintf("has_bedroom_zones_%d", p.Min)
}

func (p HasBedroomZones) Description() string {
	return fmt.Sprintf("at least %d bedroom zones designated", p.Min)
}

func (p HasBedroomZones) Horizon() Horizon { return p.H }

func (p HasBedroomZones) Check(wm *worldmodel.WorldModel) Result {
	now := time.Now()
	snap := wm.Snapshot()
	const zoneTypeBedroom = protocol.ZoneTypeBedroom
	count := 0
	for _, z := range snap.Zones.All {
		if z.ZoneType == zoneTypeBedroom {
			count++
		}
	}
	return Result{
		Name:       p.Name(),
		Horizon:    p.H,
		Satisfied:  count >= p.Min,
		Confidence: confidenceFromFreshness(snap.Zones.UpdatedAt),
		Evidence: []string{
			fmt.Sprintf("bedroom zones: %d", count),
			fmt.Sprintf("threshold: >= %d", p.Min),
		},
		CheckedAt: now,
	}
}

// HasDiningHall is satisfied when at least one zone of type Dining exists.
type HasDiningHall struct {
	H Horizon
}

func (p HasDiningHall) Name() string        { return "has_dining_hall" }
func (p HasDiningHall) Description() string { return "at least one dining hall designated" }
func (p HasDiningHall) Horizon() Horizon    { return p.H }

func (p HasDiningHall) Check(wm *worldmodel.WorldModel) Result {
	now := time.Now()
	snap := wm.Snapshot()
	const zoneTypeDining = protocol.ZoneTypeDiningHall
	count := 0
	for _, z := range snap.Zones.All {
		if z.ZoneType == zoneTypeDining {
			count++
		}
	}
	return Result{
		Name:       p.Name(),
		Horizon:    p.H,
		Satisfied:  count >= 1,
		Confidence: confidenceFromFreshness(snap.Zones.UpdatedAt),
		Evidence:   []string{fmt.Sprintf("dining halls: %d", count)},
		CheckedAt:  now,
	}
}

// NewStarterLibrary returns a small starter set of predicates suitable for
// the very first BDI runs. The deliberator gets immediate visibility into
// "fort started?", "has the embark crew survived?", "is shelter forming?",
// "do we have real bedrooms?", and "are we under attack?".
func NewStarterLibrary() *Library {
	lib := NewLibrary()
	// Now: survival
	lib.Register(HasModifiedAnything{H: HorizonNow})
	lib.Register(HasMinDwarves{Min: 7, H: HorizonNow})
	lib.Register(HasShelter{MinDugTilesPerDwarf: 6, H: HorizonNow})
	lib.Register(NoActiveHostiles{H: HorizonNow})
	// Soon: headroom — real housing, not just dug tiles
	lib.Register(HasBedroomZones{Min: 7, H: HorizonSoon})
	lib.Register(HasDiningHall{H: HorizonSoon})
	lib.Register(HasMinDwarves{Min: 14, H: HorizonSoon})
	lib.Register(HasShelter{MinDugTilesPerDwarf: 12, H: HorizonSoon})
	// Eventual: trajectory — more bedrooms than current population
	lib.Register(HasBedroomZones{Min: 15, H: HorizonEventual})
	return lib
}
