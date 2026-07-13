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

**Revised during planning research (see Component 3) — read this before the rest of Scope.** The
original brainstorm approved "full civzone breadth" on the assumption that every type would cleanly
resolve into one of two DFHack assignment mechanisms (owner or roster). Direct verification against
the DFHack 53.15-r1 source during the writing-plans research phase found that assumption wrong:
DFHack's own modules only confirm an assignment mechanism for 6 of the 18 types (4 owner-type, 2
roster-type); Barracks uses a third, structurally different mechanism (squad-based, not unit-based);
and the remaining 11 types have no assignment logic anywhere in the DFHack source to copy — the
vtable slots that would settle it belong to the closed-source game engine, not DFHack. Rather than
guess at unverified DFHack internals (this project's standing rule: `CHECK_*` failures throw for a
reason, don't paper over what you haven't confirmed), creation and listing stay full-breadth (that
mechanism is genuinely type-agnostic and fully confirmed), but `assign_zone` is honest about which
types it actually supports. See Component 3 for the exact breakdown.

In scope: full fortress-relevant `civzone_type` breadth for creation and listing — Bedroom, Office,
Tomb, DiningHall, MeetingHall, Dormitory, Barracks, Pen, Pond, ArcheryRange, PlantGathering,
WaterSource, Dump, SandCollection, FishingArea, ClayCollection, Dungeon, AnimalTraining. Unit
assignment/unassignment for the 6 types with a confirmed DFHack mechanism (Bedroom, Office, Tomb,
DiningHall via owner; Pen, Pond via roster); a clear "not yet implemented" error (not a guess) for
the other 12. A `look` zones lens (the extensibility hook Perception round 2 explicitly pre-built
for this), and correcting one piece of code this workstream's data directly breaks: the two zone
predicates (`HasBedroomZones`, `HasDiningHall`) currently checking against wrong legacy values.

Out of scope: any placement/suggestion logic (which room to make a bedroom, whether a zone is
"good enough") — the brief explicitly assigns room-quality and housing-value predicates to a
*separate* future workstream ("Verification & predicates"), gated behind this one landing. Baking
that judgment into Zones now would repeat the SVP mistake this project already paid to unlearn:
placement and quality decisions stay with the calling model, never hardcoded into Go or the plugin.
Also out of scope: Barracks assignment specifically (DFHack's `Military::updateRoomAssignments`
takes a `squad_id`, not a `unit_id` — a structurally different tool shape than `assign_zone`'s;
this belongs with goals.md's already-separate "military/squad/burrow tooling" backlog item, not
bolted onto this tool), farm-plot-as-zone (farm plots are a different building type entirely, not a
civzone — the old stub's "farm" zone type was simply wrong), and deleting the legacy
`internal/zones` package. That package is NOT the confirmed-dead orphan an earlier design draft
believed — a repo-wide grep found two live importers, `cmd/df-orchestrator/main.go` and
`internal/autonomous/loop.go`. Both of those importers are themselves already on CLAUDE.md's
Task-16 legacy-deletion list (`cmd/df-orchestrator`, `internal/autonomous`) — deleting
`internal/zones` cleanly means touching two other already-condemned legacy files first, which is
Task 16's job, not this workstream's. The new wire-level zone type (Component 1) is named distinctly
enough (`protocol.ZoneKind` or similar, not `zones.ZoneType`) that no import collision blocks
proceeding without the deletion.

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
│ Plugin: zones.cpp         │◄──────►│ ENTITY_UPDATE zone payload │
│ applyDesignateZone         │        │ (entities.cpp — the        │
│ applyAssignZone             │        │  periodic per-turn refresh,│
│ applyUnassignZone            │        │  NOT the connect-time      │
│ (queries.cpp: list_zones)     │        │  tiles-only FULL_STATE)    │
└──────────┬────────────────────┘        └──────────────┬───────────┘
           │                                             ▼
           ▼                              ┌──────────────────────────┐
┌─────────────────────────┐              │ worldmodel.WorldModel.Zones│
│ Go MCP tools               │              │ (populator.go's OnEntity- │
│ designate_zone / assign_zone│              │  Update — ALREADY builds  │
│ unassign_zone / list_zones   │              │  ZoneSnapshot from        │
└──────────┬────────────────────┘              │  msg.Zones; only the wire │
           │                                    │  codec never fed it data) │
           ▼                                    └──────────┬───────────────┘
┌─────────────────────────┐                                │
│ look zones lens             │                             ▼
│ (internal/mcpserver/lenses.go,│              ┌──────────────────────┐
│  new LensDef entry — zero      │              │ predicate fixes:      │
│  changes to RenderCrop)         │              │ HasBedroomZones,       │
└─────────────────────────────────┘              │ HasDiningHall          │
                                                  └──────────────────────┘
```

Everything downstream of the wire-protocol correction is either a thin plugin wrapper around a
DFHack API call that already has a fixed, prescribed sequence (creation, owner assignment), or a
Go-side consumer of data that already has scaffolding waiting for it — `worldmodel.ZoneSnapshot` is
not just scaffolded but **already fully wired** into `ObservedState`/`Snapshot`/`OnEntityUpdate`;
the only genuinely missing piece on that side is the wire-level `codec.go` serialize/deserialize for
`ZoneData`, which today doesn't exist at all despite the Go struct being declared. There is no new
algorithmic component here the way Perception round 2's region graph was — Zones is a capability
unlock, not new machinery.

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
renumbering) covering the 18 fortress-relevant types listed in Scope above. This insulation turns
out to matter more than originally argued: DFHack 53.15-r1's real `df::civzone_type` values are
**not** a small sequential range — they're scattered from 79 to 97 (`NONE = -1`, `int32_t` base
type), confirmed directly against `library/include/df/civzone_type.h` in the checkout:

| DF-AI wire value | Type | Real `df::civzone_type` value |
|---|---|---|
| 0x01 | Bedroom | 92 |
| 0x02 | Office | 93 |
| 0x03 | Tomb | 97 |
| 0x04 | DiningHall | 80 |
| 0x05 | MeetingHall | 87 |
| 0x06 | Dormitory | 79 |
| 0x07 | Barracks | 95 |
| 0x08 | Pen | 88 |
| 0x09 | Pond | 86 |
| 0x0A | ArcheryRange | 94 |
| 0x0B | PlantGathering | 91 |
| 0x0C | WaterSource | 82 |
| 0x0D | Dump | 83 |
| 0x0E | SandCollection | 84 |
| 0x0F | FishingArea | 85 |
| 0x10 | ClayCollection | 89 |
| 0x11 | Dungeon | 96 |
| 0x12 | AnimalTraining | 90 |

The plugin translates DF-AI's enum to/from `df::civzone_type` in one place (a
`civzoneTypeFromWire`/`wireFromCivzoneType` pair in `zones.cpp`), using the table above. Note this
finally corrects the stale reasoning behind the current stub: `applyZoneDesignation`'s comment
(`df_ai_protocol.cpp:282-296`) claims DFHack 53.12's `civzone_type` "has completely different
semantics... but NO direct Bedroom/Dining/Barracks values" — that claim is simply wrong for 53.15-r1
(all three exist, at 92/80/95 respectively); the original implementer was likely looking at an
incomplete or mis-versioned reference. This design's table above was independently verified against
the actual checkout, not carried forward from that comment.

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
building).

**Validation**: a civzone claims existing floor space, it doesn't dig — `applyDesignateZone` must
reject a rectangle that isn't fully built/carved floor, with a specific error (which tiles failed
and why), matching this project's existing truthful-ACK discipline rather than DFHack's own
generic construction-failure behavior.

**Zones are identified by tile coordinate, not a synthetic ID — corrected during planning
research.** The original draft had `assign_zone`/`unassign_zone` take a `zone_id` and left open how
the plugin would resolve one back to a `df::building*`, and how `designate_zone`'s ACK (a plain
success/error string, per `sendCommandAck`'s signature — there's no numeric-return channel on
success without misusing the PARTIAL status) would even communicate a freshly-generated ID back to
the model. Both problems disappear by following this codebase's own existing convention instead:
`remove_building(x,y,z)` and `unsuspend(x,y,z)` already target a building by a tile coordinate
inside it, not a synthetic ID (`internal/mcpserver/server_test.go`'s `minToolArgs` confirms both
signatures). Zones do the same: `assign_zone`/`unassign_zone` take `(x, y, z, unit_id)`, and the
plugin resolves the civzone at that tile via `Buildings::findCivzonesAt(&results, df::coord(x,y,z))`
(confirmed present at `library/modules/Buildings.h:100` in the checkout) — no ID tracking, no ACK
return-value problem, and one less DFHack API call to verify blind. `list_zones` reports each
zone's extents (the same `x1/y1/x2/y2` shape `list_buildings` already emits) as the caller's handle
for picking a tile to target.

## Component 3: Zone assignment (`applyAssignZone` / `applyUnassignZone`)

**This component was substantially corrected during the writing-plans research phase.** The
brainstormed design assumed every zone type would resolve cleanly into "owner" or "roster." Direct
verification against the DFHack 53.15-r1 source (`library/modules/Buildings.cpp`, `plugins/
preserve-rooms.cpp`, `plugins/zone.cpp`, `library/modules/Military.cpp`) found a third mechanism
and, more importantly, found that DFHack's own modules only ever implement assignment for 6 of the
18 types — the rest have no confirmed mechanism to copy, because the deciding logic lives in the
closed-source game engine, not DFHack. One Go-facing tool (`assign_zone`, per the earlier tool-shape
decision), four behaviors:

| Mechanism | Types | Confirmed via | Behavior |
|---|---|---|---|
| **Owner** (`Buildings::setOwner`) | Bedroom, Office, Tomb, DiningHall | `plugins/preserve-rooms.cpp` tracks exactly these four via `last_known_assignments_bedroom/office/dining/tomb` and calls `setOwner` for each | Single `assigned_unit_id` (`Buildings::setOwner(civzone, unit)`). Unassign clears it (`setOwner(civzone, nullptr)` or equivalent — verify the clear-owner call signature against `Buildings.cpp:325-366` when implementing). |
| **Roster** (`assigned_units` + general-ref) | Pen, Pond | `plugins/zone.cpp`'s `assignUnitToZone()` explicitly gates on `Buildings::isPenPasture(building) \|\| Buildings::isPitPond(building)` — no other type is accepted by that function | Push a `df::general_ref_building_civzone_assignedst` (fields: inherited `building_id int32_t`) onto the unit's `general_refs`, append the unit id to the zone's `assigned_units`; unassign removes both. `plugins/zone.cpp` itself is dead/commented-out in this checkout (superseded by Lua), so this workstream calls the underlying `Buildings::`/struct-level API directly — the same way `queue_job` already bypasses DFHack's Manager-order plugin layer. |
| **Squad (out of scope)** | Barracks | `library/modules/Military.cpp`'s `Military::updateRoomAssignments(squad_id, civzone_id, flags)` links `df::squad::rooms` to the zone's `squad_room_info` — a fundamentally different tool shape (squad, not unit) | `assign_zone` on a Barracks zone returns a clear error naming the actual mechanism ("Barracks assignment uses squads, not units — not supported by this tool; see the future military/squad workstream"). Not a guess-and-hope; a named, deliberate gap. |
| **Unconfirmed (not implemented)** | MeetingHall, Dormitory, ArcheryRange, PlantGathering, WaterSource, Dump, SandCollection, FishingArea, ClayCollection, Dungeon, AnimalTraining (11 types) | Searched but found nothing — no DFHack module, plugin, or Lua script implements assignment for any of these; the vtable slots that would settle it (`canMakeRoom`/`canUseSpouseRoom`/`canBeRoom`/`isAssigned` on `df::building`) have no override bodies in the open-source checkout | `assign_zone` returns "assignment mechanism for <type> is not confirmed against the DFHack API — creation and listing work, assignment does not yet" rather than guessing at a struct write that could silently corrupt save state. Creation and `list_zones` still work fully for these types (that mechanism is type-agnostic, per Component 2). |

This is a real, deliberate scope reduction from the original "full breadth" framing — worth
restating plainly: `assign_zone` supports Bedroom, Office, Tomb, DiningHall (owner) and Pen, Pond
(roster) at launch; every other type designates and lists fine but cannot be assigned yet. The
brief's own success criterion ("assigned bedrooms and a dining hall in use") is fully satisfied by
the 4 confirmed owner-type kinds, so nothing in Scope's stated goals is blocked by this narrowing.

`list_zones` (new `queries.cpp` query) iterates civzone buildings, returning id/type/extents plus
owner-or-roster, mirroring `list_buildings`' existing shape and reusing its footprint-extent fields
from Perception round 2's Task 4.

## Component 4: Zone data on the wire (`ENTITY_UPDATE`, not `FULL_STATE`)

**Corrected during planning research**: the original component name ("FULL_STATE zone population")
was imprecise about which message actually matters. This plugin has two distinct periodic-refresh
concepts: `send_full_state` (`df_ai_protocol.cpp:1616-1679`) sends tile data once, at connect time
only — it has no per-entity list at all and is the wrong integration point. The message that
actually matters is `ENTITY_UPDATE`, sent by `push_state_refresh` on every turn boundary (manual
pause and step-completion auto-repause, `df_ai_protocol.cpp:503-524`) via `serialize_entity_update`
(`entities.cpp:140-237`) — this is the live, per-turn refresh the model's dashboard actually depends
on, and it already carries an optional `FortInfo` block using exactly the additive-flag-byte pattern
a zones block needs to follow.

**The scope here is much smaller than originally described**, because most of the target already
exists and works: `worldmodel.ZoneSnapshot` is not a stub waiting to be wired — it's already a live
field on both `ObservedState` (`types.go:58`) and the immutable `Snapshot` (`types.go:215,264`), and
`Populator.OnEntityUpdate` (`populator.go:167-217`) already builds a `ZoneSnapshot` from
`msg.Zones` and assigns it under lock, exactly mirroring how `Entities`/`FortInfo` are populated.
**Only two things are actually missing**:
1. `internal/protocol/codec.go` has **zero** serialize/deserialize code for `EntityUpdateMessage.
   Zones`/`ZoneData` today (confirmed by grep — the struct is declared in `message.go` but never
   touched in `codec.go`) — this needs adding, following the exact `HasFortInfo`-flag-byte pattern
   already used for `FortInfo` (`codec.go:619-626` write, `:671-672` read): a `[1: HasZones? or
   Count]` prefix followed by per-zone fixed-width fields.
2. `entities.cpp`'s `serialize_entity_update` needs to actually gather civzone data (the same
   `df::global::world->buildings.all` walk `handleListBuildings` already does, filtered to
   `Civzone` type) and append it to the buffer in that new wire format, mirroring how the FortInfo
   block is appended today (`entities.cpp:188-227`).

No new Go-side worldmodel code is needed at all — once the wire carries real data, the existing
`OnEntityUpdate` path picks it up automatically.

## Component 5: MCP tools

`designate_zone(type, x1,y1,z,x2,y2)`, `assign_zone(x, y, z, unit_id)`, `unassign_zone(x, y, z,
unit_id)` (both target the civzone at that tile via `Buildings::findCivzonesAt`, matching
`remove_building`/`unsuspend`'s existing by-tile-coordinate convention), `list_zones(type?, z?)` —
replacing the existing stub `zone` tool
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

## Component 8: `internal/zones` — deferred to Task 16, not this plan

**Removed from scope during planning research.** The brainstormed design assumed `internal/zones`
was orphaned; a repo-wide import grep found it isn't — `cmd/df-orchestrator/main.go` constructs a
`zones.ZoneExtractor` and wires it into the autonomous loop, and `internal/autonomous/loop.go`
holds a `zoneExtractor` field, a `SetZoneExtractor` setter, and a metrics block (`loop.go:1309+`)
that calls `ExtractZones`/`CountByType`/`GetUnassignedCount`. Both of those importers are themselves
already on CLAUDE.md's "legacy pending deletion after First Fort (Task 16)" list — deleting
`internal/zones` cleanly means editing two other already-condemned legacy files, which is squarely
Task 16's bulk-cleanup job, not a side effect of shipping civzone tooling. Left in place for this
plan; the new wire-level type (Component 1) uses a distinct name (not `zones.ZoneType`) so nothing
here blocks proceeding without the deletion.

## Error handling

Truthful-ACK discipline throughout, consistent with every other command this plugin exposes:
- `assign_zone` on Barracks — names the actual mechanism (squads, not units) and that it's out of
  scope for this tool, per Component 3.
- `assign_zone` on any of the 11 unconfirmed-mechanism types — explicit "not yet implemented,
  mechanism unconfirmed against the DFHack API" error, never a guessed struct write.
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
round 2's existing table-driven glyph test rather than duplicating its structure), `renderZones`'
summary-by-type grouping (Component 5's token-scaling correction) — a fixture with several zones of
the same type must collapse to one summary line, not one line per zone, and a `type`/`z` filter must
return full per-zone detail — and that `assign_zone` returns the correct distinct error text for
each of Component 3's four buckets (owner/roster success paths are covered by the live-verification
checkpoint below; the Barracks-squad and unconfirmed-mechanism error paths are pure Go logic and get
unit tests directly). Plugin side: a
compile-check rebuild (this project has no C++ unit harness, established precedent from Perception
round 2 Task 4). Live verification: an actual play-session checkpoint — designate a bedroom zone
over one of the fort's 7 existing unassigned beds, assign a specific dwarf to it via `assign_zone`,
confirm `HasBedroomZones` flips from false to true and `list_zones` reports the correct owner. This
mirrors the checkpoint-gating culture both First Fort and Perception round 2 used, and it's the
literal, concrete blocker this workstream exists to clear.

## Success criteria

- `designate_zone`/`assign_zone`/`unassign_zone`/`list_zones` live and passing the registration
  sweep (`TestEveryToolNilBridge`).
- At least one owner-type zone (Bedroom) and one roster-type zone (Pen or Pond) verified assigned
  live in an actual play session (not just unit tests) — bedroom assignment is the concrete target
  given goals.md's existing blocker.
- `HasBedroomZones`/`HasDiningHall` correctly flip on real zone data, no longer permanently false.
- `go build ./...` and `go test ./...` clean.
- `WorldModel.Zones` populated from the periodic `ENTITY_UPDATE` refresh without any explicit
  `list_zones` poll.

## Open questions

None blocking. The DFHack API research raised, and this document resolves, one real design change:
`assign_zone` ships for 6 of 18 zone types (owner: Bedroom/Office/Tomb/DiningHall; roster: Pen/Pond)
rather than all 18 — Component 3 explains why the other 12 have no confirmed DFHack mechanism to
implement safely. Creation and listing remain full-breadth across all 18. Room-quality/value scoring
stays explicitly assigned to the future Verification & predicates workstream, unchanged from the
original design.
