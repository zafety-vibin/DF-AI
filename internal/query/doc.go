// Package query implements the deliberator's tool-use surface.
//
// The deliberator can emit REQUEST <name>(<args>) calls in addition to
// (or instead of) action commands. The BDI loop's runCycle routes those
// calls through this package's Registry, which dispatches to the right
// Handler and returns a structured Result. The result is rendered into
// the next pass's user message so the LLM can read it and continue
// reasoning.
//
// Tier 1 handlers (this package) read only the world model, plan DAG,
// predicate library, and divergence event buffer — no plugin work
// required. Tier 2/3 handlers will add plugin-backed queries
// (dwarf_detail, list_orders, manager_orders, etc.) once the protocol
// supports them.
//
// The `request <name>(<args>)` grammar is parsed by the same regex
// extractor as action commands. A response is treated as a request batch
// if it contains REQUEST calls and NO action commands; commands always
// win when both are present in the same response (so the LLM can finish
// reasoning at any point by emitting a command).
package query
