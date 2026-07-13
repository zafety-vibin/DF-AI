# Feature 009, sub-project 2: Zones — design

**Status**: approved design, ready for planning (superpowers:writing-plans is next).
**Parent**: `specs/009-culture-and-learning/design-brief.md` workstream 1 ("Zones"). The brief lists
Zones first in priority order, but Perception round 2 (workstream 2) landed first per the project
lead's explicit call ("zone ARE important... but perception is so core to the experience we should
work on it first"). Perception round 2 is done (11 tasks + 1 post-review fix, merged to
`008-mcp-server`, build/test clean) — this is the next sub-project.
**Why now**: the brief calls this "the largest single capability unlock" and a hard prerequisite —
bedroom-assignment and housing-quality predicates are explicitly gated behind it, and the brief's
own success criterion #1 ("fort with assigned bedrooms and a dining hall in use") is currently
unreachable. It is also a live, concrete blocker today: First Fort session 2 built 7 beds via
`queue_job` with zero room-assignment step, because `applyZoneDesignation` in the plugin is a
deliberate stub that always fails ("zones are not implemented in this plugin build").

## Scope

In scope: full fortress-relevant `civzone_type` breadth — Bedroom, Office, Tomb, DiningHall,
MeetingHall, Dormitory, Barracks, Pen, Pond, ArcheryRange, PlantGathering, WaterSource, Dump,
SandCollection, FishingArea, ClayCollection, Dungeon, AnimalTraining. Zone creation, unit
assignment/unassignment (both DFHack mechanisms — owner-type and membership-type), zone listing, a
`look` zones lens (the extensibility hook Perception round 2 explicitly pre-built for this), and
correcting two pieces of code this workstream's data directly breaks or fixes: the two zone
predicates (`HasBedroomZones`, `HasDiningHall`) currently checking against wrong legacy values, and
deletion of the dead `internal/zones` package (Feature 007, never wired to live data, flagged for
deletion in CLAUDE.md).

Out of scope: any placement/suggestion logic (which room to make a bedroom, whether a zone is
"good enough") — the brief explicitly assigns room-quality and housing-value predicates to a
*separate* future workstream ("Verification & predicates"), gated behind this one landing. Baking
that judgment into Zones now would repeat the SVP mistake this project already paid to unlearn:
placement and quality decisions stay with the calling model, never hardcoded into Go or the plugin.
Also out of scope: military/squad/burrow tooling (goals.md already tracks this as a separate,
distinct backlog item) and farm-plot-as-zone (farm plots are a different building type entirely,
not a civzone — the old stub's "farm" zone type was simply wrong).

## Architecture overview

```
┌─────────────────────────┐
│ Wire protocol correction │  ZONE_TYPE_* replaced with real civzone_type
│ (protocol.h, message.go, │  values; ZoneData gains owner/roster fields
│  codec.go)                │
└──────────┬────────────────┘
           │
           ▼
┌─────────────────────────┐        ┌──────────────────────────┐
│ Plugin: zones.cpp         │◄──────►│ FULL_STATE zone population │
│ applyDesignateZone         │        │ (df_ai_protocol.cpp)       │
│ applyAssignZone             │        └──────────────────────────┘
│ applyUnassignZone            │                    │
│ (queries.cpp: list_zones)     │                    ▼
└──────────┬────────────────────┘        ┌──────────────────────────┐
           │                              │ worldmodel.WorldModel.Zones│
           ▼                              │ (populator.go — already    │
┌─────────────────────────┐              │  scaffolded, never fed)    │
│ Go MCP tools               │              └──────────┬───────────────┘
│ designate_zone / assign_zone│                         │
│ unassign_zone / list_zones   │                         ▼
└──────────┬────────────────────┘              ┌──────────────────────┐
           │                                    │ predicate fixes:      │
           ▼                                    │ HasBedroomZones,       │
┌─────────────────────────┐                    │ HasDiningHall          │
│ look zones lens             │                    └──────────────────────┘
│ (internal/mcpserver/lenses.go,│
│  new LensDef entry — zero      │
│  changes to RenderCrop)         │
└─────────────────────────────────┘

Deleted: internal/zones/ (legacy Feature 007 package, never wired to live data)
```

Everything downstream of the wire-protocol correction is either a thin plugin wrapper around a
DFHack API call that already has a fixed, prescribed sequence (creation, owner assignment), or a
Go-side consumer of data that already has scaffolding waiting for it (`WorldModel.Zones`, the two
predicates, the lens registry). There is no new algorithmic component here the way Perception round
2's region graph was — Zones is a capability unlock, not new machinery.

## Component 1: Wire protocol correction

**Problem**: the current `ZONE_TYPE_*` enum (`dfhack-plugin/protocol.h`) and the stub tool's type
list mix an abandoned Feature-007 numbering (BEDROOM=1, DINING=2, MEETING=3, BARRACKS=4,
DORMITORY=5) with guessed hex values (FARM=0x10 through TOMB=0x18) that don't match DFHack 53.15's
real `civzone_type` enum at all — there is no Farm or Hospital civzone, "Garbage" should be `Dump`,
"Pit" isn't a real value. Nobody could have designated a real zone with these values even if the
stub weren't hardcoded to fail.

**Fix**: replace `ZONE_TYPE_*` with a small DF-AI-owned enum (matching the pattern
`designation_kinds` used in Perception round 2 — a stable, project-owned integer mapping, not a
direct passthrough of DFHack's own numbering, which insulates the wire format from future DFHack
renumbering) covering the 18 fortress-relevant types listed in Scope above. The plugin translates
DF-AI's enum to/from `df::civzone_type` in one place (a `civzoneTypeFromWire`/`wireFromCivzoneType`
pair in `zones.cpp`).

`ZoneData` (`internal/protocol/message.go`) gains fields the current stub-era struct never needed:
`OwnerUnitID int32` (owner-type zones; -1/absent when unowned or membership-type), `AssignedUnits
[]int32` (membership-type roster), and keeps the existing extents/type fields. Additive to the
existing struct — no wire compatibility concern since the field has never carried real data.

## Component 2: Zone creation (`applyDesignateZone`)

Replaces the current hard-fail stub. Sequence, mirroring `dfhack-plugin/buildings.cpp`'s existing
stockpile creation exactly (confirmed via the DFHack 53.15-r1 checkout as the canonical path for
*any* abstract building — stockpiles and civzones use the identical mechanism):
`Buildings::allocInstance(pos, building_type::Civzone, subtype)` → set `room` extents on the
allocated building → `Buildings::setSize(bld, size)` → `Buildings::constructAbstract(bld)`. No
materials/filters (abstract buildings skip that path entirely — this is the same reason
`buildings.cpp` warns `constructAbstract` must never be called for a real, materials-backed
building). Returns the new zone's DFHack building ID in the ACK.

**Validation**: a civzone claims existing floor space, it doesn't dig — `applyDesignateZone` must
reject a rectangle that isn't fully built/carved floor, with a specific error (which tiles failed
and why), matching this project's existing truthful-ACK discipline rather than DFHack's own
generic construction-failure behavior.

## Component 3: Zone assignment (`applyAssignZone` / `applyUnassignZone`)

One Go-facing tool (`assign_zone`, per the earlier tool-shape decision), three behaviors selected
internally by the zone's `civzone_type` — every one of the 18 in-scope types is placed into exactly
one bucket below so the implementer never has to guess:

| Mechanism | Types | Behavior |
|---|---|---|
| **Owner-type** (`Buildings::setOwner`) | Bedroom, Office, Tomb | Single `assigned_unit_id`. Unassign clears it. |
| **Membership-type** (roster push) | Dormitory, Barracks, Pen, Pond, ArcheryRange, PlantGathering, WaterSource, Dump, SandCollection, FishingArea, ClayCollection, Dungeon, AnimalTraining | Push a `df::general_ref_building_civzone_assignedst` onto the unit's `general_refs` and append to the zone's `assigned_units`; unassign is the exact inverse (remove both). Dormitory and Barracks take the roster path rather than `setOwner` because DF allows multiple simultaneous occupants/squad-members per zone, which a single `assigned_unit_id` field cannot represent. |
| **Ambient (no assignment)** | DiningHall, MeetingHall | `assign_zone`/`unassign_zone` on either returns a clear "this zone type has no owner/roster concept, it's ambient — dwarves use it automatically when in range" error rather than silently doing nothing. |

The membership-type roster mechanism itself: push a `df::general_ref_building_civzone_assignedst`
onto the unit's `general_refs` and append to the zone's `assigned_units`; unassign is the exact
inverse (remove both). DFHack's own C++ zone plugin (`plugins/zone.cpp`) that historically
implemented this is entirely dead/commented-out in the 53.15-r1 checkout — this workstream calls
the underlying `Buildings::`/struct-level API directly, the same way the rest of this plugin
already bypasses DFHack's higher-level plugins in favor of direct API calls (`queue_job` bypassing
the Manager system is the precedent). **Verify against source during implementation**: the
Dormitory/Barracks roster-vs-owner classification above is this design's best-confidence read of
DF's mechanics, not a value directly confirmed field-by-field in the DFHack checkout the way
Bedroom/Office/Tomb's `setOwner` path was — the implementing task should re-confirm both against
`library/modules/Buildings.cpp` before writing the branch, the same "read the real source before
editing" discipline Perception round 2's Task 4 already established for this codebase.

`list_zones` (new `queries.cpp` query) iterates civzone buildings, returning id/type/extents plus
owner-or-roster, mirroring `list_buildings`' existing shape and reusing its footprint-extent fields
from Perception round 2's Task 4.

## Component 4: FULL_STATE zone population

The wire field for zone data already exists (`ZoneSnapshot`/`ZoneData` in
`internal/worldmodel/{types,populator}.go`) but nothing on the plugin side has ever populated it —
`FULL_STATE` messages ship with an empty zone list today, permanently, regardless of what zones
exist in-game. This workstream wires the plugin's periodic full-state builder to include real
civzone data (same id/type/extents/owner/roster shape as `list_zones`), so `WorldModel.Zones` stays
live without the model needing to poll `list_zones` every turn — the same passive-update pattern
every other predicate-backing data source already uses.

## Component 5: MCP tools

`designate_zone(type, x1,y1,z,x2,y2)`, `assign_zone(zone_id, unit_id)`, `unassign_zone(zone_id,
unit_id)`, `list_zones(type?, z?)` — replacing the existing stub `zone` tool
(`internal/mcpserver/tools_action.go`) entirely rather than layering on top of it, since the stub's
tool name, param shape, and type list are all being superseded. Tool descriptions state plainly
that a zone claims existing floor, is not a dig designation, and that assignment mechanism (owner
vs roster) depends on zone type — mirroring how `designate_dig`'s own description was tightened
during Perception round 2.

**`list_zones` render — token-scaling correction.** The 2026-07-12 token/scaling research
(`docs/archive/2026-07-12-token-scaling-research.md`) flags `list_buildings`'s flat
one-line-per-entry render as the pattern to *stop* using at scale (recommendation #5: "the model
needs the shape of the fort far more often than N coordinates... default: summary-by-type, roster
on request"). An earlier draft of this component said `list_zones` should mirror `list_buildings`'
existing shape verbatim — that would import the exact defect the research already identified rather
than the pattern it recommends replacing it with. Corrected default render: grouped by type, one
line per type — `"Zones: 5 Bedroom (3 owned, 2 unowned), 1 DiningHall, 2 Pen (7 animals total),
1 Tomb (unowned)"` — with full per-zone detail (id, extents, owner/roster) returned only when the
caller passes a `type` or `z` filter, the same progressive-disclosure shape `stocks`' own fix
(recommendation #1) and `dwarves`' own fix (recommendation #6) both converge on. The plugin-side
`list_zones` query still carries a cap + `truncated` flag from day one (the established "truthful
truncation with a refine hint" pattern, `queries.cpp`'s existing 200-building cap), applied
proactively here rather than reactively the way `buildings`/`stocks` had to be patched after the
fact — zone counts scale roughly with fort population (bedrooms) plus a handful of resource zones,
so the risk is lower than `stocks`' combinatorial item×material growth, but the pattern costs
nothing to build in now.

## Component 6: `look` zones lens

Perception round 2's design explicitly reserved this: *"the `zones` lens is designed when that
workstream lands... zero changes to the rendering pipeline."* One new `LensDef` entry in
`internal/mcpserver/lenses.go` (`gatherZonesLens`), following the exact `buildings`/`designations`
lens pattern — glyph per zone-type category (reusing the same category-glyph philosophy: a handful
of glyphs, not one per civzone_type, per the project's house rule against per-type ASCII budget
blowout), painted over the zone's extents, with the dwarf-collision footnote mechanism (fixed in
Perception round 2's post-review pass) applying automatically since it's implemented at the `look`
handler level, not per-lens.

## Component 7: Predicate fixes

`HasBedroomZones` and `HasDiningHall` (`internal/predicate/library.go`) currently check
`ZoneType==0x01`/`0x02` — the old Feature-007 numbering, not any value a real zone will ever carry
post-fix. Update both to the new DF-AI-owned enum values from Component 1. These predicates were
never wrong on their own terms (correct field access, correct horizon registration) — they were
just checking data that could never arrive. No new predicates in scope (that's the gated future
workstream).

## Component 8: Delete `internal/zones`

Confirmed dead (Explore agent survey, this session): `types.go`, `extractor.go`, `queue.go`, none
wired to any live code path, built for a defunct "FortLayout" feature. Its `ZoneType` enum shares a
name with the new wire-level type this workstream introduces, which is exactly the kind of
same-name confusion CLAUDE.md's legacy-deletion note exists to prevent. Delete outright rather than
deprecate — nothing imports it.

## Error handling

Truthful-ACK discipline throughout, consistent with every other command this plugin exposes:
- `assign_zone` on a mismatched mechanism (owner call on a roster-type zone, or either on an
  ambient DiningHall/MeetingHall) — specific error naming the zone's actual type and what mechanism
  it does use, never a silent no-op.
- `designate_zone` over non-floor tiles — names which tiles failed.
- `unassign_zone` on a unit that isn't currently assigned to that zone — explicit "not assigned"
  ACK, not a silent success (a silent success here could mask the model acting on stale state).
- `list_zones` with zero zones — "No zones." (matches `list_buildings`' existing empty-state text),
  never an error.
- All new DF-touching plugin paths stay inside `executeCommand`/`executeQuery`'s existing
  try/catch, per this project's standing `CHECK_*`-throws rule.

## Testing

Go-side: unit tests for the wire-enum translation functions, `ZoneData` decode (including the
absent-owner/absent-roster cases), both corrected predicates (each against a fixture `WorldModel`
with the new type values), the zones lens's glyph mapping + disjointness (extending Perception
round 2's existing table-driven glyph test rather than duplicating its structure), and
`renderZones`' summary-by-type grouping (Component 5's token-scaling correction) — a fixture with
several zones of the same type must collapse to one summary line, not one line per zone, and a
`type`/`z` filter must return full per-zone detail. Plugin side: a
compile-check rebuild (this project has no C++ unit harness, established precedent from Perception
round 2 Task 4). Live verification: an actual play-session checkpoint — designate a bedroom zone
over one of the fort's 7 existing unassigned beds, assign a specific dwarf to it via `assign_zone`,
confirm `HasBedroomZones` flips from false to true and `list_zones` reports the correct owner. This
mirrors the checkpoint-gating culture both First Fort and Perception round 2 used, and it's the
literal, concrete blocker this workstream exists to clear.

## Success criteria

- `designate_zone`/`assign_zone`/`unassign_zone`/`list_zones` live and passing the registration
  sweep (`TestEveryToolNilBridge`).
- At least one owner-type and one membership-type zone kind verified live in an actual play
  session (not just unit tests) — bedroom assignment is the concrete target given goals.md's
  existing blocker.
- `HasBedroomZones`/`HasDiningHall` correctly flip on real zone data, no longer permanently false.
- `internal/zones` deleted; `go build ./...` and `go test ./...` clean afterward.
- `WorldModel.Zones` populated from `FULL_STATE` without any explicit `list_zones` poll.

## Open questions

None blocking — the DFHack API research (this session) resolved every structural unknown the
brief flagged ("civzone API shape in 53.x — research first, then scope"). The one deferred
question is room-quality/value scoring, explicitly assigned to the future Verification & predicates
workstream per Scope above, not this document.
