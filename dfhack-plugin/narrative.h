// narrative.h -- shared helpers for the narrative-layer query handlers
// (Feature 013). Query handlers themselves (dwarf_portrait today;
// story_pulse/social_graph/fort_art/combat_* in later waves) live in
// narrative.cpp and are forward-declared at their executeQuery dispatch
// site in queries.cpp, matching this plugin's existing convention (see
// handleListSquads in military.cpp). This header exists only for the two
// pieces every narrative query is expected to reuse -- a citizen-hfid
// index and the thought caption/subthought resolver -- so sibling query
// handlers in future waves can `#include "narrative.h"` instead of
// re-deriving either from scratch.

#pragma once

#include "df/unit_thought_type.h"

#include <cstdint>
#include <string>
#include <unordered_map>
#include <unordered_set>

namespace df { struct unit; }

// CitizenIndex is the one-pass-over-units.active scan every narrative
// query needs before it can answer "is this histfig one of ours" or
// "resolve this histfig back to a living, named citizen": hfids is every
// current citizen's historical-figure id (Units::isCitizen, the
// handleWellbeing/handleListWildlife precedent); hfidToUnit resolves a
// citizen's hfid straight back to their live df::unit* for a follow-up
// getReadableName lookup, valid only for the duration of the query call
// that built it (main-thread-only, synchronous use -- same lifetime
// assumption df::unit pointers get everywhere else in this plugin). A
// citizen with hist_figure_id < 0 (should not happen for a real dwarf,
// but df::unit doesn't forbid it) is simply absent from both -- never
// inserted as a garbage/negative key.
struct CitizenIndex {
    std::unordered_set<int32_t> hfids;
    std::unordered_map<int32_t, df::unit *> hfidToUnit;
};

// build_citizen_index walks df::global::world->units.active once. Caller
// must have already confirmed df::global::world is non-null (the same
// precondition every other handler in this plugin checks up front) --
// this function does not re-check it and returns an empty index if world
// is null, so a caller skipping the check fails soft rather than crashing.
CitizenIndex build_citizen_index();

// ThoughtDetail is the shared decode of one personality_moodst's
// thought+subthought pair: caption is DFHack's own ENUM_ATTR text
// (placeholder tokens like [somebody]/[deity] left intact -- this project
// does not attempt to fill them in), subthoughtText is populated ONLY
// when subthoughtResolved is true. Nothing here ever guesses: an
// unresolved subthought means the curated resolver (below) has no
// confirmed mapping for this thought type or this particular code, not
// that resolution was skipped.
struct ThoughtDetail {
    std::string caption;
    bool subthoughtResolved = false;
    std::string subthoughtText;
};

// resolve_thought is the caption+subthought decoder dwarf_portrait and
// (in a later wave) story_pulse both consume. Three resolution paths,
// all research-confirmed against this checkout's df.personality.xml
// (see specs/013-narrative-layer/research.md 2.1b) -- everything else
// reports unresolved rather than guessing:
//   1. Death/Prayer/DreamAbout/Defeated/Murdered: subthought is a
//      historical_figure id (the union member names in circumstance_id
//      literally match these five thought-type names) -- resolve via
//      df::historical_figure::find() and Translation::translateName();
//      unresolved if the figure isn't found or translates to an empty
//      name (the "plausible" gate -- never surfaces a garbage lookup as
//      a real name).
//   2. Complained/UnableComplain/ReceivedComplaint (meeting-topic codes)
//      and GhostNightmare/GhostHaunt (relative-type codes): the curated
//      hex-code tables transcribed from df.personality.xml's own inline
//      comments (DFHack ships no structured resolver for these -- the
//      comments are prose, not enum-attrs).
//   3. RelativeExpelled: subthought is a histfig_relationship_type enum
//      value, decoded via DFHack's own bounds-checked enum-key lookup.
ThoughtDetail resolve_thought(df::unit_thought_type thought, int32_t subthought);
