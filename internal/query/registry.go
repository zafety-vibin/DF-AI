package query

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Registry holds the set of registered Handlers and dispatches Calls.
// Safe to construct with NewRegistry, then Register handlers, then
// Dispatch from the loop. Not safe for concurrent registration after
// startup — register all handlers in main, then read-only.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register adds a Handler. Panics if a handler with the same Name is
// already registered — that's a programming error, not a runtime
// condition.
func (r *Registry) Register(h Handler) {
	if h == nil {
		return
	}
	name := h.Name()
	if _, exists := r.handlers[name]; exists {
		panic(fmt.Sprintf("query.Registry: duplicate handler name %q", name))
	}
	r.handlers[name] = h
}

// Get returns a handler by name and a found flag.
func (r *Registry) Get(name string) (Handler, bool) {
	h, ok := r.handlers[strings.ToLower(name)]
	return h, ok
}

// All returns every registered handler in stable (sorted-by-name) order.
// Used by the catalog renderer.
func (r *Registry) All() []Handler {
	out := make([]Handler, 0, len(r.handlers))
	for _, h := range r.handlers {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Dispatch runs a single Call through its handler and returns the
// resulting Result. Times the execution and stamps Success/Error on the
// Result so handlers don't have to repeat that bookkeeping.
func (r *Registry) Dispatch(ctx Context, call Call) Result {
	started := time.Now()
	h, ok := r.handlers[call.Name]
	if !ok {
		return Result{
			Name:       call.Name,
			Args:       call.Args,
			Success:    false,
			Error:      fmt.Sprintf("unknown query %q", call.Name),
			StartedAt:  started,
			DurationMs: time.Since(started).Milliseconds(),
		}
	}

	result, err := h.Execute(ctx, call.Args)
	result.Name = call.Name
	result.Args = call.Args
	result.StartedAt = started
	result.DurationMs = time.Since(started).Milliseconds()
	if err != nil {
		result.Success = false
		if result.Error == "" {
			result.Error = err.Error()
		}
	} else {
		result.Success = true
	}
	return result
}

// DispatchAll runs each Call sequentially. Errors don't stop the batch —
// each Call gets its own Result. Returns results in input order.
func (r *Registry) DispatchAll(ctx Context, calls []Call) []Result {
	out := make([]Result, 0, len(calls))
	for _, c := range calls {
		out = append(out, r.Dispatch(ctx, c))
	}
	return out
}

// CatalogMarkdown renders the registered handlers as a markdown section
// suitable for the deliberator's prompt. Lists each handler's name,
// description, and arg signature.
func (r *Registry) CatalogMarkdown() string {
	var sb strings.Builder
	sb.WriteString("## Tool Catalog\n\n")
	sb.WriteString("Emit `request <tool_name>(<args>)` to call any of these. Multiple REQUESTs per response are allowed; results come back in the next pass. Action commands (dig/build/etc) and REQUESTs are mutually exclusive — if both are emitted, commands win and REQUESTs are dropped.\n\n")
	for _, h := range r.All() {
		fmt.Fprintf(&sb, "- **%s(", h.Name())
		for i, a := range h.ArgsSpec() {
			if i > 0 {
				sb.WriteString(", ")
			}
			if a.Optional {
				fmt.Fprintf(&sb, "%s?", a.Name)
			} else {
				sb.WriteString(a.Name)
			}
		}
		fmt.Fprintf(&sb, ")** — %s\n", h.Description())
	}
	sb.WriteString("\n")
	return sb.String()
}

// RenderResults turns a batch of Results into the markdown block fed
// into the next pass's user message.
func RenderResults(results []Result) string {
	var sb strings.Builder
	sb.WriteString("## Tool Results\n\n")
	for _, r := range results {
		argsStr := strings.Join(r.Args, ", ")
		fmt.Fprintf(&sb, "### request %s(%s)\n", r.Name, argsStr)
		fmt.Fprintf(&sb, "_duration: %d ms_\n\n", r.DurationMs)
		if r.Error != "" {
			fmt.Fprintf(&sb, "**ERROR:** %s\n\n", r.Error)
			continue
		}
		if r.Note != "" {
			fmt.Fprintf(&sb, "%s\n\n", r.Note)
		}
		if r.Data != nil {
			sb.WriteString("```json\n")
			b, err := json.MarshalIndent(r.Data, "", "  ")
			if err != nil {
				fmt.Fprintf(&sb, "(marshal error: %v)\n", err)
			} else {
				sb.Write(b)
				sb.WriteString("\n")
			}
			sb.WriteString("```\n\n")
		}
	}
	return sb.String()
}
