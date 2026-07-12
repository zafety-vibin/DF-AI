// Package predicate defines checkable conditions over the world model.
//
// A Predicate names what success looks like for a sub-goal. The deliberator
// reads predicate results to know what's satisfied and what isn't; the
// reconciler watches predicate results to detect when a satisfied predicate
// flips back to unsatisfied (a regression).
//
// Predicates are intentionally not procedures. They describe the END STATE,
// not the steps to get there. The deliberator decides how to satisfy them.
//
// A starter library lives in library.go. The expected progression is:
//
//   - Phase 1: hand-written Now/Soon/Eventual predicates (this package).
//   - Phase 2: predicate composition — AND / OR / NOT combinators.
//   - Phase 3: predicate templates with parameters extracted from successful
//     traces (Voyager-style skill harvesting at the predicate level).
package predicate
