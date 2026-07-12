// Package bdi implements the Beliefs/Desires/Intentions loop that replaces
// the legacy autonomous loop.
//
// Three goroutines share a worldmodel.WorldModel:
//
//   - Perception: reads tile/entity updates from the dfhack client and
//     feeds them into the populator. Bumps the world model's tick on
//     every event.
//   - Deliberator: cadence- (and eventually trigger-) driven. Takes a
//     snapshot, evaluates the predicate library, renders a prompt, calls
//     the LLM, parses, and dispatches commands.
//   - Executor (folded into Deliberator for the first cut): sends parsed
//     commands via the existing commands.CommandExecutor.
//
// The reconciler and divergence triggers are deferred to a later cut. The
// first runnable version of this loop is cadence-only — the deliberator
// wakes on a fixed interval, looks at the world, and emits one decision.
// Once the loop is producing usable behavior, the trigger bus and
// reconciler bolt onto the same goroutines without restructuring.
package bdi
