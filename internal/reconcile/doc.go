// Package reconcile compares predicted tile state against observed tile
// state, confirms predictions when observation catches up, and emits
// stall events when a plan node has gone too long without observed
// progress.
//
// There are intentionally no deadlines. Predictions stay Outstanding
// until observation catches up (Confirmed) or the plugin reports an
// explicit blocker (Diverged with a specific reason like
// ConstructionSuspended — placeholder until that protocol message
// exists). Work that takes hours is fine if it's progressing.
//
// The Reconciler runs as a single goroutine on a slow ticker. On each
// tick:
//
//  1. For every Outstanding prediction whose target tile now matches the
//     prediction (e.g., predicted "becomes open" and topology now reports
//     open) → call Confirm and stamp LastProgressAt on the parent plan
//     node.
//
//  2. For each Active plan node, derive confirmed = total - outstanding -
//     diverged. If confirmed reaches the total, mark the node Done.
//     Failure-by-divergence is NOT applied here; nodes only fail on ACK
//     rejection at execute time, or (future) when the plugin reports an
//     explicit blocker.
//
//  3. For each Active plan node whose LastProgressAt is older than
//     StallThreshold (default 90s), emit a Stalled divergence event.
//     The plan node stays Active. A node that has been stalled is
//     re-notified at most every StallReNotifyInterval (default 5min) so
//     the divergence buffer doesn't fill with the same node.
//
// On a slower cadence the reconciler also retires Confirmed predictions
// older than RetainConfirmed to bound memory.
//
// The dataset signal that the BDI architecture is supposed to produce
// emerges here: every DivergenceEvent records (plan node, action, idle
// seconds, reason) — a row of "command language failed to produce
// observed progress under this state." Stall events are the cheapest
// signal; explicit-blocker events from the plugin (when they exist) are
// the highest-fidelity signal.
package reconcile
