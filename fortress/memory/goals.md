# Goals

## NOW (survival) — ALL SATISFIED per check_goals as of day 92 summer
- [x] Underground shelter: 112 dug tiles, well past the per-dwarf minimum
- [x] Aquifer at z136 pierced AND sealed (8 constructed walls, all 4 sides;
      residual pocket drained/dispersed harmlessly through z132 by day 90)
- [x] Wood chain: carpenter workshop built; chop/gather real (multi-species
      wood flowing)
- [x] ALL 7 BEDS BUILT — full coverage for the fort's 7 real dwarves
- [x] Fishery built, auto-processing raw fish (no job-queue needed for this
      chain — confirmed automatic once the workshop exists)
- [x] Food/general stockpiles placed (z134 food-only + z132 "everything"
      per overseer's centralize-underground-storage tip)
- [x] no_active_hostiles

## SOON (headroom)
- [ ] Drink: 9 drinks (draining slowly). Still BUILT at (45,49,134) but
      BLOCKED — BrewDrink has no job_type mapping in order/queue_job
      (protocolToJobType returns -1); needs a DFHack reaction-based
      lookup, not a plain enum value. Water is a fallback (unhappy
      thought, not fatal) if this runs dry before the fix lands.
- [ ] Dining hall (has_dining_hall predicate) — table+chair buildable
      today, needs the same room-assignment gap as bedroom zones below.
- [ ] Bedroom zone assignment (has_bedroom_zones_7/15) — beds exist and
      are usable, but formal zone/room assignment needs either the zone
      civzone fix or a new room-assignment tool; neither exists yet.
- [ ] Survive first migrant wave
- [ ] Office (chair+table+door, all buildable today) + eventually a
      Manager noble once that tooling exists — unlocks bulk `order` orders
- [x] GENERALIZE item construction — SHIPPED 2026-07-12 in the pre-009
      blocking-fixes wave. queue_job's wire payload gained an optional
      job-type NAME string (ORDER_TYPE_BY_NAME sentinel), resolved via
      DFHack's find_enum_item<df::job_type> — no C++ addition needed for
      any job_type DFHack already knows. New `job_types` discovery tool
      (filterable, NOT baked into orders/queue_job's hot path). NOT
      generalized (confirmed underivable from DFHack enum metadata, see
      decisions.md): jobTypeAllowedAtWorkshop's workshop-compatibility
      table and the WOOD/BOULDER material filter stay hand-maintained —
      a name-resolved job_type can still be rejected by that whitelist.
- [ ] BrewDrink job_type mapping (engineering) — unblocks the still. The
      name-based path can now RESOLVE "BrewDrink" but per
      constructionResearch BrewDrink isn't its own job_type at all — it's
      a CustomReaction dispatched via job->reaction_name, so name
      resolution alone won't be enough; still needs the reaction-based
      lookup originally scoped.
- [ ] Hatch cover job_type mapping — SMALLER NOW than before generalized
      construction shipped: "ConstructHatchCover" resolves cleanly via
      the new name path, the ONLY remaining gap is one entry in
      jobTypeAllowedAtWorkshop's hand-maintained whitelist (Carpenters,
      matching the door/furniture group). 4 hatches at the main shaft's
      surface opening (46-47,50-51,140) are dead plans until this lands.
      Bedroom hall's west wall (48,50/51,139) can't be retrofitted with a
      door there without relocating 2 already-built beds — minor design
      debt, resolve when doors land.
- [ ] stocks category filter is case-SENSITIVE against the plugin
      (needs "DRINK" not "drink") — minor, normalize with strings.ToUpper
      in Go before querying; tool description's lowercase examples are
      wrong until fixed.
- [x] set_labor tool — SHIPPED + LIVE-VERIFIED 2026-07-12. Write confirmed
      via read-back (Solon gained MINE). ROOT CAUSE turned out sharper
      than first diagnosed: this is a DEFAULT-EMBARK DESIGN fact, not a
      quirk — every dwarf starts with every labor EXCEPT mine except
      Doren, who alone has it. Applied the fix live: enabled MINE on
      Solon (idle, stone-working skills) as a second miner so digging
      doesn't fully gate on Doren's other ~80 labors (incl. CLEAN_FISH,
      the actual collision — not a narrow specialization as first
      assumed, just one of many generalist labors that happened to grab
      him). General lesson for future forts: check for this exact
      single-miner pattern early and spread MINE to a second dwarf
      pre-emptively, don't wait for a stall.
- [x] Dwarf-vs-animal census filter — SHIPPED + LIVE-VERIFIED 2026-07-12.
      `dwarves` now reports 7, exactly matching the manual census.
      Root cause: entities.cpp classified via Units::isFortControlled()
      (true for ANY tame unit) instead of Units::isCitizen(); fixed with
      a Units::isAnimal() check inserted before the fort-controlled
      branch.
- [x] find_dig_site stale-solidity bug (engineering) — CLOSED 2026-07-12.
      Root cause: `tile_updates.cpp` hardcoded TILE_UPDATE delta flags to
      FLAG_DISCOVERED instead of computing them; fixed via
      `compute_tile_flags()` in commit 062455f. See docs/decisions.md
      2026-07-12 entry and fortress/memory/learnings.md's
      "find_dig_site reliability" follow-up for detail.
- [ ] Named gaps from the 2026-07-12 skills-proposal research (not yet
      scoped, filed for future engineering — none block current play):
      trade-depot build type + caravan/broker tooling (blocks liquidating
      wealth — production works, selling doesn't); created-wealth
      query/predicate (no way to measure fort value, fits 009 workstream
      4); lever/mechanism/drawbridge build types (blocks a full
      mechanical entrance seal beyond door+hatch); military/squad/burrow
      tooling (blocks any active defense response — architecture is the
      only lever today); farm-plot build type (blocks farming
      independently of the zone civzone stub — even a fixed zone tool
      needs somewhere to designate a farm plot onto).

## EVENTUAL (trajectory)
- [ ] Survive year 1 (zero starvation/dehydration deaths)
- [ ] Defensible single entrance
- [ ] Cistern from the sealed wet column (46-50,50-51 area, z133-136)
