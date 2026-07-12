// Package worldmodel holds the three-layer world model that the BDI
// architecture reasons over.
//
//	+--------------------------------------------------------------+
//	|  WorldModel                                                  |
//	|                                                              |
//	|  Observed   <- mirror of DF state, populated on heartbeat    |
//	|  Predicted  <- what should be true if in-flight plans land   |
//	|  Planned    <- derived from PlanDAG (set by plan package)    |
//	+--------------------------------------------------------------+
//
// The WorldModel is the single substrate read by the deliberator (the LLM
// reasoning pass) and the executor (the trigger-driven step runner), and the
// single substrate written by perception (this package's Populator), the
// executor (which writes Predicted entries when committing actions), and the
// reconciler (which marks predictions confirmed or diverged).
//
// This package is intentionally passive: it does NOT own overlay construction
// (those still live in their respective packages: topology, hazards,
// modifications). Observed embeds pointers to those overlays so callers can
// query them through one handle. Predicted carries the only sparse state this
// package owns: per-tile expectations stamped with the plan node that produced
// them.
//
// The package is designed to compile and run alongside the legacy autonomous
// loop. Nothing in it touches existing control flow.
package worldmodel
