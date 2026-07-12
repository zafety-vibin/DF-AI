// Package skill is the deliberator's library of procedural-knowledge
// templates — "to build a bedroom, dig a room, then place a door, then
// place a bed, then zone it as bedroom."
//
// Skills are deliberately under-specified. They describe ORDER and
// DEPENDENCIES but not dimensions, materials, locations, or quality. The
// LLM still chooses all the specifics; the skill just keeps it from
// inventing the procedure each time. Emergence comes from:
//
//   - Composition: stitching multiple skill instances into a fort layout
//   - Parameter choice: peasant room vs noble suite uses the same skill
//     with different material/scale decisions
//   - Adaptation: skipping or reordering steps when context demands it
//
// Phase 1 (current) renders skills as markdown into the deliberator
// prompt. The LLM reads them as guidance and emits its own concrete
// actions. No control-flow changes; just smarter priors.
//
// Phase 2 (later) will let the LLM emit `invoke <skill>(<args>)` calls
// that the executor expands into a sub-DAG with prefilled dependency
// edges. The reconciler will track the whole skill instance as a unit.
package skill
