// Package plan implements the BDI loop's intention layer.
//
// The deliberator commits decisions as PlanNodes added to a DAG. The
// Executor drains Ready nodes (those whose dependencies are all Done),
// sends commands to the dfhack plugin, and writes per-tile predictions
// into the world model's predicted state. The reconciler watches those
// predictions and marks nodes Done or Failed.
//
// This is intentionally minimal for the first cut: a flat DAG with simple
// status transitions and a single-action-per-node model. Compound plans
// (a single LLM emission decomposed into multiple nodes with internal
// dependencies) will come when the deliberator starts emitting structured
// plans rather than free-text commands.
package plan
