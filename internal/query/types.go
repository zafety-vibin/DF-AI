package query

import (
	"context"
	"time"

	"github.com/df-ai/orchestrator/internal/plan"
	"github.com/df-ai/orchestrator/internal/predicate"
	"github.com/df-ai/orchestrator/internal/reconcile"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// PluginDispatcher is the abstraction Tier 2/3 handlers use to send
// structured queries to the DFHack plugin and receive JSON responses.
// Tier 1 handlers don't need this. main.go wires the implementation
// (a thin adapter over dfhack.Client.SendQuery).
type PluginDispatcher interface {
	Dispatch(ctx context.Context, name string, argsJSON string) (resultJSON []byte, err error)
}

// Context is the bundle of references a Handler needs to answer a query.
// The BDI loop builds one of these per turn and passes it to Dispatch.
type Context struct {
	Ctx    context.Context // for cancellation propagation into plugin queries
	WM     *worldmodel.WorldModel
	DAG    *plan.DAG
	Lib    *predicate.Library
	Events *reconcile.EventBuffer
	Plugin PluginDispatcher // nil for runs without a connected plugin
}

// ArgSpec describes one positional argument a Handler expects. Used by
// the catalog renderer so the LLM sees what each tool needs.
type ArgSpec struct {
	Name        string
	Type        string // "int", "string", "coord", "node_id", etc. (informational)
	Optional    bool
	Description string
}

// Call is a parsed `request <name>(<args>)` from the LLM's response.
type Call struct {
	Name string
	Args []string // raw token list, post-comma-split and trim. Quotes stripped.
	Raw  string   // original text, for trace fidelity
}

// Result is one Handler's response. Mirrors the Call's name + args so the
// renderer can show what was asked alongside the answer.
type Result struct {
	Name       string
	Args       []string
	Success    bool
	Error      string
	Note       string // short human-readable summary; surfaces in renderer header
	Data       any    // structured payload; serialized as JSON in the prompt
	DurationMs int64
	StartedAt  time.Time
}

// Handler is one tool. Implementations are registered with the Registry
// and dispatched by Name.
type Handler interface {
	// Name returns the lowercase, snake_case identifier used in
	// `request <name>(...)`.
	Name() string

	// Description returns a human-readable explanation of what the tool
	// does. Surfaces in the prompt's tool catalog.
	Description() string

	// ArgsSpec describes the positional arguments the handler expects.
	// Optional args may appear at the end.
	ArgsSpec() []ArgSpec

	// Execute runs the query. The returned Result must always have its
	// Name and Args fields populated; the registry handles timing and
	// success bookkeeping around the call.
	Execute(ctx Context, args []string) (Result, error)
}
