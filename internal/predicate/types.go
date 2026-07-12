package predicate

import (
	"time"

	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// Horizon is when a predicate is expected to be satisfied. It maps onto the
// goal-tier framing (Now/Soon/Eventual) without forcing the deliberator to
// reason about real-time deadlines explicitly.
type Horizon uint8

const (
	HorizonNow      Horizon = 1 // Survival: must be satisfied or the fort dies
	HorizonSoon     Horizon = 2 // Headroom: should be satisfied within the next milestone
	HorizonEventual Horizon = 3 // Trajectory: open-ended; gates emergence
)

func (h Horizon) String() string {
	switch h {
	case HorizonNow:
		return "now"
	case HorizonSoon:
		return "soon"
	case HorizonEventual:
		return "eventual"
	default:
		return "unknown"
	}
}

// Predicate is a checkable condition on the world model.
//
// Implementations should be cheap (microseconds, not milliseconds) and
// side-effect free — predicates run on every reconciliation tick and must
// not block. If a check needs expensive computation, it should cache against
// the worldmodel snapshot's EventSeq and only recompute when the seq advances.
type Predicate interface {
	// Name returns a stable identifier used in prompts, logs, and dataset
	// rows. Lowercase snake_case ("has_shelter", "has_food_buffer_60d").
	Name() string

	// Description returns a human-readable explanation of what success looks
	// like. The deliberator sees this in its prompt; the reconciler stamps it
	// onto divergence events.
	Description() string

	// Horizon classifies the predicate by goal tier.
	Horizon() Horizon

	// Check evaluates the predicate against the current world model and
	// returns a Result. Result.CheckedAt should be set to time.Now() by the
	// implementation.
	Check(*worldmodel.WorldModel) Result
}

// Result is one predicate evaluation.
type Result struct {
	// Name mirrors the Predicate's Name() at evaluation time.
	Name string

	// Horizon mirrors the Predicate's Horizon().
	Horizon Horizon

	// Satisfied is the headline answer.
	Satisfied bool

	// Confidence is 0.0–1.0. 1.0 means the satisfied/unsatisfied verdict is
	// derived from solid observable evidence; lower means the predicate had
	// to estimate. The deliberator uses this to decide whether to ask for
	// more observation before acting on a result.
	Confidence float64

	// Evidence is human-readable bullet points explaining the verdict. Goes
	// into the deliberator prompt verbatim so the LLM can see the reasoning
	// trail. Keep entries short — one short clause each.
	Evidence []string

	// SubResults are nested results for compound predicates (AND / OR /
	// hierarchies). Empty for atomic predicates.
	SubResults []Result

	// CheckedAt stamps when the check ran.
	CheckedAt time.Time
}

// Library is a registered set of predicates evaluated together. The BDI loop
// holds one of these and re-runs it on a slow cadence; the deliberator reads
// the most recent results when forming the next prompt.
type Library struct {
	predicates []Predicate
}

// NewLibrary returns an empty Library. Use Register to add predicates.
func NewLibrary() *Library { return &Library{} }

// Register adds a predicate to the library. Predicates are checked in
// registration order; the deliberator's prompt rendering follows that order.
func (l *Library) Register(p Predicate) {
	l.predicates = append(l.predicates, p)
}

// All returns the registered predicates.
func (l *Library) All() []Predicate { return l.predicates }

// CheckAll evaluates every registered predicate against wm and returns a
// slice of Results in registration order.
func (l *Library) CheckAll(wm *worldmodel.WorldModel) []Result {
	if wm == nil {
		return nil
	}
	out := make([]Result, 0, len(l.predicates))
	for _, p := range l.predicates {
		out = append(out, p.Check(wm))
	}
	return out
}

// SatisfiedCount returns how many of results have Satisfied=true.
func SatisfiedCount(results []Result) int {
	n := 0
	for _, r := range results {
		if r.Satisfied {
			n++
		}
	}
	return n
}

// FilterByHorizon returns only the results whose horizon equals h.
func FilterByHorizon(results []Result, h Horizon) []Result {
	out := make([]Result, 0, len(results))
	for _, r := range results {
		if r.Horizon == h {
			out = append(out, r)
		}
	}
	return out
}

// Unsatisfied returns only the results whose Satisfied flag is false.
// The deliberator typically prompts on this set: "here's what isn't done."
func Unsatisfied(results []Result) []Result {
	out := make([]Result, 0, len(results))
	for _, r := range results {
		if !r.Satisfied {
			out = append(out, r)
		}
	}
	return out
}
