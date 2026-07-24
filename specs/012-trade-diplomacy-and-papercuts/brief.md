# Feature 012 — Trade/Diplomacy Tooling + Session-2 Paper Cuts

**Status:** brief only (2026-07-22, written mid-Fort-#6 by the playing
session so the experience survives compaction). Wave 7 candidate.
Everything below is grounded in live play — first caravan ever traded
(The Gray Mansion, autumn y100), waves 6a/6b deployed and live-verified
the same night. Read `fortress/memory/journal.md` Fort #6 session-2
entries + `fortress/memory/goals.md` TRADE/DIPLOMACY GAPS section for
the raw findings; `docs/decisions.md` 2026-07-22 entries for the
architecture decisions already made (live MAP-STATE over session-delta
overlays; the zone-extents refutation).

## Process notes for whoever picks this up (hard-won, don't skip)

- **Investigation-first with adversarial verify.** The zone-dance fix
  was REFUTED by its reviewer (constructAbstract already init_extents
  every civzone) and the "dance" itself proved to be folklore when
  live-tested (designate→assign instant). Plausible mechanisms die
  under review; ship nothing un-refuted. Sonnet agents per the
  model-routing memory; review gates per change.
- **Deploy cycle:** DF closed → `cmake --build build --target
  df_ai_protocol --config Release` from the sibling checkout (junction
  sanity-check first) → copy `df_ai_protocol.plug.dll` to Steam
  `hack/plugins/` → restart df-mcp → live-verify checklist as session
  first act (template: journal "NEXT-SESSION LIVE-VERIFY" entry).
- **Verification timing for trade fixes:** needs a LIVE caravan. Elven
  caravan = spring, human = summer, dwarven = autumn. Fort #6 is at
  autumn y100 — the elven caravan (~spring y101) is the nearest test
  window. Plan the wave to ship before a season boundary.
- String-named JSON queries (list_buildings pattern) need no wire
  change; prefer them. All the usual house rules (truthful ACKs,
  schema budget, /W3, no head/tail pipes) apply.

## A. Trade & diplomacy (overseer-priority)

1. **BINS BLOCK STAGING — the headline.** `bring_goods_to_depot`
   (work_orders.cpp command 0x23 + Go tools_action.go) marks loose
   items only; finished goods auto-bin in stockpiles, so craft trade
   goods are UNSTAGEABLE (live: melbil's shell crafts sat binned while
   25 worthless raw shells got marked by material substring). Fix
   direction: vanilla brings the whole BIN — mark the bin itself
   (research df.item flags: `flags.trader` semantics for containers)
   and/or recurse into containers when a class filter matches. Add an
   item-CLASS filter (finished-goods/crafts) alongside the substring
   filters — substrings conflated raw shells with shell crafts.
2. **Unmark/undo trade marks.** No inverse of bring_goods exists; 8
   rough gems got trade-reserved with no recovery path (only unsold
   return). Trivial sibling command: clear flags.trader by the same
   filters, or by depot.
3. **depot_goods ours-vs-theirs split.** Currently one aggregated list
   mixing OUR staged goods with the merchants' 22k import inventory —
   staging can't be audited. Find the discriminating flag (merchant
   ownership vs fort item marked trader) and render two sections.
4. **trader_requested toggle.** Depot flag
   (building_tradedepotst trade flags) — no tool writes it; the broker
   never gets summoned by us. One-flag command; also expose
   anyone_can_trade.
5. **Diplomacy readout (read-only first).** The liaison meeting +
   agreement happened INVISIBLY (viewscreen_meetingst). Reading the
   CONCLUDED agreement (requested imports, price agreements) should be
   data-side (entity agreement structures — research in the 53.15
   checkout). Full meeting interaction is viewscreen-bound; scope
   honestly (probably out, like the trade commit).
6. **Trade-exchange automation research — VERDICT: stays human, no safe
   path found (wave 7).** There is no C++ `Trade` module and no `caravan`
   C++ plugin anywhere in the 53.15 DFHack checkout — the newer trade UI
   is entirely `scripts/internal/caravan/{trade,movegoods,pedestal,
   tradeagreement,common,predicates}.lua` + `scripts/caravan.lua`, a
   `plugins.overlay` selection-assist layer built directly on the vanilla
   `df.global.game.main_interface.trade` struct and
   `viewscreen_tradegoodsst`. A fresh grep across all six lua files (and
   `scripts/caravan.lua`) found zero calls to `gui.simulateInput` or any
   `feed()`-style key injection — unlike other DFHack scripts that do use
   it for other screens — leaving Bay12's own closed-source input handler
   to perform the actual exchange. `dfhack-plugin/trade.cpp`'s prior
   (2026-07-19) conclusion holds and is independently reconfirmed: the
   commit action (the final Offer/Trade confirm) has no safe non-viewscreen
   API, df_ai_protocol has never touched the DF viewscreen stack (zero
   real references anywhere in `dfhack-plugin/*.cpp` besides explanatory
   comments), and pushing a viewscreen from a headless background context
   is untested territory this project has no evidence either way on — so
   this stays deliberately unautomated, not merely deferred. One safe,
   read-only piece IS worth adding later: a `propose_trade_offer`-style
   recommender using the caravan-aware `Items::getValue(item, car)`
   overload (which our existing `bring_goods_to_depot`/`depot_goods` value
   tallies don't yet use) to compute a near-balanced selection — pure
   computation, no mutation, and structurally immune to the overseer's
   observed broker-Appraisal client-side price noise since it never reads
   any displayed/fuzzed number. Not implemented this wave; a future pick-up
   item, not a blocker.
7. **Depot accessible=false despite pack-animal arrival.** Suspect
   saplings regrown on the old chop field break the wagon-path check.
   Investigate what the flag actually computes; surface WHY (blocking
   tile coordinates) if cheap.

## B. Perception/safety paper cuts (all live burns this session)

8. **Pending-designation overwrite warning.** A flat dig rect silently
   REPLACED a pending stairs designation (dig-over-stairs warning
   guards CARVED stairs only) — overseer caught it in-client; learnings
   entry banked (rooms-first-stairs-last rule). Warn in
   designations.cpp/Go pre-check when a designation overwrites a
   pending stair/ramp/channel of a different type.
9. **Connectivity hint cross-z false negatives.** Post-6b the hint no
   longer points at map corners (coverage gate works) but still
   reported "not connected, nearest open tile at (x,y,138)" for rects
   directly adjacent to same-z open street/stair tiles (live: Iron
   Quarter street touching the spine's west face). connectorSuggestion
   should check same-z orthogonal adjacency to open/stair tiles before
   consulting the region graph.
10. **Phantom tavern location.** create_location tavern ACKed SUCCESS
    and list_locations shows "Tavern at (95,94)-(98,98) z=137" but the
    client shows a plain meeting hall. LIVE REPRO PRESERVED in Fort #6
    (deliberately not cleaned up): overlapping DiningHall zone
    (94,95)-(98,98) + MeetingHall (95,94)-(98,98); the first
    create_location attempt anchored at an overlap tile and was
    truthfully refused, the second (at a meeting-hall-only tile)
    "succeeded". Suspect locations.cpp misses a linkage step the
    client UI performs (occupation/location_id wiring?). Compare
    against a client-created tavern (Fort #5's world has none — the
    overseer can make one to diff against).
11. **Dangerous-wildlife tripwire** (deferred from 6a, overseer calls
    protection important). The danger classifier already exists
    (queries.cpp handleListWildlife: LARGE_PREDATOR/AMBUSHPREDATOR/
    big-CARNIVORE). Wire a step tripwire: auto-pause + truthful reason
    when a dangerous WILD unit newly appears on the map. 6a skipped it
    honestly because trip_step_tripwire/push_state_refresh call sites
    span thread contexts — needs its own careful pass, not a bolt-on.
12. **Instrument placeability pre-check (minor).** build type=Instrument
    on a handheld instrument fails at construction time ("unable to
    complete") after a SUCCESS placement ACK. building_types or the
    build pre-check could distinguish placeable (large) vs handheld
    instrument items.

## C. Carry-overs still open (pre-Fort-#6)

13. assign_squad fails systemically (Military::addToSquad returns
    false; squad healthy) — client-staffing discriminator then source
    debug. list_squads needs a fort-entity filter (renders all world
    squads).
14. crafts-class direct queue at Craftsdwarfs — manager-order path is
    canonical and proven; either wire it or document the canon in the
    queue_job schema pointer (probably just document).
15. TILE_UPDATE why-dead mystery: deltas flow fine THIS session
    (10-119/step via the new step counter). If any future session sees
    sustained 0s while digging, that's the repro the 6a investigation
    never had. The counter is the instrument; no action until it fires.

## D. Wired-but-unverified surfaces to exercise in play (no eng work)

- CutGems/EncrustWithGems at the built Jewelers (lorbam ENCRUSTGEM 10
  is in the fort; rough gems accumulate from Iron Quarter veins).
- check_goals predicates on the live region_scan path — VERIFIED
  2026-07-22 late session with a finding: has_modified_anything flips
  [x] correctly (first time in project history), and the degraded
  no-connection path labels itself honestly, BUT the live count
  UNDERCOUNTS badly — "region_scan z=131 bbox(90,101)-(117,102)"
  returned 8 dug tiles inside a bbox whose street alone holds ~70,
  across a fort with thousands of dug tiles on 9 levels. The 6b
  implementer's single-z max-bbox sampling is too narrow AND the count
  within its own bbox looks wrong — so has_shelter_N_per_dwarf stays
  falsely red at any population. NEW ITEM: fix the predicate scan to
  cover the fort's full footprint (fort_footprint bbox per z, summed)
  and root-cause the within-bbox undercount (kind filter? clamp?).
- Pet burial flow (coffin + tomb zone at the crypt; dog corpse
  somewhere on the map — dead units are invisible to tools, another
  small gap if burial never happens).
- steel anvil → second forge when a weaponsmith migrant lands.
