# Feature 009, sub-project 3: Locations (Tavern/Temple/Library/Guildhall) — design

**Status**: approved design, ready for planning (superpowers:writing-plans is next) — **implementation must not
start until the in-flight Zones workflow (sub-project 2) has landed and merged**. See Global Constraints.
**Parent**: `specs/009-culture-and-learning/design-brief.md` workstream 1 ("Zones"), continued. This surfaced
organically while Zones (`design-zones.md`) was mid-implementation: DF's MeetingHall civzone can be converted
into a Location (Tavern/Temple/Library/Guildhall) via a completely separate DFHack subsystem from civzone
ownership/roster assignment, and the project lead specifically wants Locations — meeting halls are the fort's
default safe social/assembly space, and taverns are how visiting travelers/performers get housed.
**Why a separate design and a separate MCP tool, not folded into Zones**: the underlying DFHack API surface is
genuinely different (`df::abstract_building`/`df::world_site`, not `df::building_civzonest`/`Buildings::setOwner`),
and — practically — Zones was already mid-implementation via an autonomous workflow when this was scoped; keeping
the files disjoint (new `locations.cpp`, new `tools_location.go`) avoids a live edit conflict.

## Scope

In scope: creating a Location (Tavern/Temple/Library/Guildhall) from an existing MeetingHall civzone, listing
Locations, and — at a distinctly lower confidence tier, called out explicitly rather than blended in — assigning
a Bedroom civzone as guest lodging inside a Tavern.

Out of scope: per-unit Location relationships (who's a patron, who works there, deity/profession assignment for
Temples/Guildhalls) — these go through `df::occupation` records and activity-event structures neither confirmed
nor exercised by any research this project has done; a future increment if it's ever needed. Also out of scope:
the "bare label" association DFHack's quickfort supports generically (any civzone can carry its own
`site_id`/`location_id` pointing at a Location, independent of `contents.building_ids`/`room_info` membership) —
considered and rejected as this plan's mechanism for bedroom-tavern linkage, because unlike `rental_roomst` (a
real, purpose-built lodging struct, confirmed to exist even though unexercised), the bare-label approach's actual
in-game effect on lodging behavior is unconfirmed; it may be inert metadata. Founding-civzone (MeetingHall)
creation itself is NOT in scope here — that's `designate_zone` from the Zones sub-project, reused as-is.

## Architecture overview

```
┌─────────────────────────────┐
│ Existing MeetingHall civzone  │  created via Zones' designate_zone —
│ (from the Zones sub-project)   │  NOT this plan's job
└──────────────┬──────────────────┘
               │ create_location(x,y,z,type)
               ▼
┌─────────────────────────────┐
│ Plugin: locations.cpp          │  mirrors quickfort's set_location()
│ applyCreateLocation              │  exactly — allocate abstract_building_*st,
│ (queries.cpp: list_locations)      │  insert into world_site.buildings,
└──────────────┬──────────────────────┘  link civzone.site_id/location_id
               │
               ▼
┌─────────────────────────────┐        ┌──────────────────────────────┐
│ Go MCP tools                    │        │ assign_lodging / unassign_lodging │
│ create_location / list_locations │◄──────►│ (Component C — gated behind live  │
└─────────────────────────────────┘        │  verification, its own task)       │
                                            └──────────────────────────────┘
```

Component A (creation) has a working, current DFHack reference implementation to copy line-for-line —
mechanically similar risk to Zones' own creation path. Component C (lodging) has confirmed struct layout but
zero DFHack precedent anywhere — genuinely the first implementation of that mechanism, and is scoped, tasked,
and verified separately from A/B for that reason.

## Component A: `create_location(x, y, z, type, profession?)`

**Fully resolved during planning research** — every step below is now confirmed against `zone.lua`'s literal
source, not inferred. Targets the civzone at that tile via `Buildings::findCivzonesAt` (matching `assign_zone`'s
existing convention). Rejects if the civzone isn't `MeetingHall`, or if it already carries a non-zero
`location_id` (no silent double-create). `type` is one of a small DF-AI-owned wire enum: `Tavern=0x01,
Temple=0x02, Library=0x03, Guildhall=0x04` (matching `df::abstract_building_type`'s
`INN_TAVERN/TEMPLE/LIBRARY/GUILDHALL`). `profession` is an optional string, **required only for Guildhall** —
DFHack's own reference implementation refuses to create a guildhall without one (`zone.lua:301`); it's a real
`df::profession` enum value (the same enum every unit's job/profession uses, `library/include/df/profession.h`),
resolved by name the same way this plugin's `queue_job` tool already resolves job-type names generically
(`df::find_enum_item<T>`) — no new name-resolution pattern needed. Tavern/Temple/Library need no extra input;
DFHack's own defaults (goblet/instrument/paper procurement targets, a `Religion=-1`/no-deity default for Temple)
are copied verbatim from `zone.lua`'s `valid_locations` table rather than exposed as tool parameters — they're
DF's own suggested starting values, not a placement/quality decision this project's model needs to make.

Sequence, mirroring `set_location()` (`zone.lua:300-354`) exactly:
1. Get the current site: `int32_t siteID = df::global::plotinfo->site_id; df::world_site *site =
   df::world_site::find(siteID);` (confirmed C++ chain — `dfhack.world.getCurrentSite()` is pure Lua wrapping
   the DFHack-exported `World::GetCurrentSiteId()`, which in fortress mode is exactly `plotinfo->site_id`).
2. Allocate the right subtype — **corrected during implementation**: `abstract_building_*st` subtypes are
   `virtual_class` types with a PROTECTED constructor (only `df::allocator_fn<T>` is a friend), so a literal
   `new df::abstract_building_inn_tavernst()` does not compile. DFHack's own built-in `plugins/zone.cpp` hits the
   identical problem for a different `virtual_class` type and documents the fix inline ("calling new() doesn't
   work, need `_identity.instantiate()` instead", `plugins/zone.cpp:240`); allocate via each type's own
   `virtual_identity::instantiate()` and cast the returned `virtual_ptr` instead, e.g. `(df::abstract_building_inn_tavernst*)
   df::abstract_building_inn_tavernst::_identity.instantiate()`. Then set `id = site->next_building_id`, `site_id
   = site->id`, `pos = site->pos`, apply the type's static defaults from the table above (including, for
   Guildhall, `contents.profession` from the resolved `profession` param), then `site->buildings.push_back(bld);
   site->next_building_id++;`. No DFHack helper exists for this — `df::world_site` is a plain struct with no
   `AddNewBuilding`-style method, confirmed by reading its full field list.
3. Append the founding civzone's id to the new Location's building list — **corrected during implementation**:
   `contents` (holding `building_ids`) is a field on the derived `abstract_building_*st` types only, not on the
   base `df::abstract_building` pointer the code holds `bld` as; link via the base class's own `getContents()`
   virtual accessor instead of a `bld->contents` field access, which would not compile against the base pointer:
   `bld->getContents()->building_ids.push_back(zone->id)`.
4. Set `site_id`/`location_id` (base `df::building` fields) on the civzone itself.
5. Call `zone->uncategorize(); zone->categorize(true);` directly — **corrected during planning research**: these
   are `df::building`'s own game virtual methods (add/remove-from-arrays), NOT a DFHack module function.
   `Buildings::notifyCivzoneModified` (the design's earlier guess) does something unrelated — it only rewires
   "civzone contains furniture" spatial relations, confirmed by reading its implementation. No DFHack wrapper
   exists for `categorize`/`uncategorize`; call the virtual methods on the `df::building*` directly.

## Component B: `list_locations(type?)`

Pure query, no DF mutation. Reports each Location's id/type/founding-civzone-extents, and for Taverns
specifically, its `room_info` roster (Component C) if any. Mirrors `list_zones`' shape and cap+truncated pattern
established in the Zones sub-project.

## Component C: `assign_lodging` / `unassign_lodging` — lower confidence, gated behind live verification

Per explicit project-lead decision: ship this, but as its own clearly-tagged task with its own live-verification
requirement, not blended into Component A/B's confidence level. `assign_lodging(tavern_x, tavern_y, tavern_z,
bedroom_x, bedroom_y, bedroom_z)` (both tiles explicit — rejected the auto-nearest-tavern alternative since it
silently breaks once a fort has two taverns) resolves both civzones by tile, validates the first is a
Tavern-linked MeetingHall and the second is a Bedroom, then constructs a `df::rental_roomst{id:
tavern_location->next_room_info_id++, civzone: bedroom.id, world_x/y/z: bedroom's position}` and pushes it into
the Tavern's `room_info` vector. `unassign_lodging(bedroom_x, bedroom_y, bedroom_z)` takes only the bedroom's
tile (scans every Tavern's `room_info` for that civzone id and removes it — simpler than requiring the caller to
also name the tavern, and unambiguous since a bedroom can only be one tavern's lodging at a time).

**This is genuinely first-of-its-kind work**: the struct layout (`abstract_building_inn_tavernst.room_info` →
`rental_roomst.civzone`, confirmed via `library/include/df/rental_roomst.h` and its XML `ref-target='building'`
annotation) is real, but a thorough search across `scripts/`, `plugins/`, and `library/modules/` in the DFHack
53.15-r1 checkout found zero code anywhere that reads or writes it. There is no reference sequence to copy the
way Component A has one — this task is reconstructing the write path from struct layout alone, which is exactly
the situation the Zones design's "don't guess at unverified DFHack internals" principle exists for. The
difference here is the struct is unambiguous (a plain vector-of-pointers push, no complex invariants visible in
the field list) and the risk is scoped (worst case: a `rental_roomst` entry that does nothing, not a crash) —
which is why this ships rather than being deferred outright, but it does NOT ship as "done" until an actual live
DF session shows a bedroom behaving as tavern lodging (a wandering unit/visitor using it, or the room appearing
correctly in DF's own Locations UI), not just a successful ACK from the plugin.

## Error handling

Truthful-ACK discipline throughout: `create_location` on a non-MeetingHall civzone — names the actual type.
`create_location` on an already-linked civzone — "already has a location" error, not a silent re-create.
`assign_lodging` where the tavern tile isn't a Tavern Location (e.g. it's a Temple) — names the actual type.
`assign_lodging` where the bedroom tile isn't a Bedroom civzone, or is already lodging for a different tavern —
clear error, never a silent overwrite. `unassign_lodging` on a bedroom that isn't currently lodging anywhere —
explicit "not assigned" ACK. `list_locations` with zero locations — "No locations." never an error. All new
DF-touching plugin paths stay inside `executeCommand`/`executeQuery`'s existing try/catch.

## Testing

Go-side: unit tests for the wire-type enum and render functions, matching the Zones sub-project's established
pattern. Plugin side: compile-check only (no C++ unit harness, established precedent). Live verification, two
separate checkpoints matching the two confidence tiers: (1) Component A/B — create a Tavern from an existing
MeetingHall, confirm via `list_locations`; this is the basic completion bar. (2) Component C — `assign_lodging`
a bedroom to that tavern, then confirm in an actual live DF session (not just a successful ACK) that the
lodging assignment has a real, observable effect — this checkpoint gates Component C specifically and the plan
should not claim it "done" without it.

## Success criteria

- `create_location`/`list_locations` live and passing the registration sweep (`TestEveryToolNilBridge`).
- At least one Location (a Tavern, given project priority) created and verified live from an existing MeetingHall.
- `assign_lodging`/`unassign_lodging` live and passing the registration sweep, AND independently verified live to
  have an observable in-game effect (Component C's own, stricter completion bar — see Testing).
- `go build ./...` and `go test ./...` clean.

## Open questions

None blocking for Components A/B (the DFHack reference implementation resolves the mechanism completely).
Component C carries one honest, stated risk forward rather than resolving it here: whether a manually-constructed
`rental_roomst` actually produces the in-game lodging behavior DF's own UI produces when a player does this
through vanilla means — that's exactly what the live-verification checkpoint exists to answer, and the plan
should treat a negative result (compiles and ACKs fine, but no observable effect) as a real, reportable outcome,
not a task-completion blocker to paper over.
