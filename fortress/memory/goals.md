# Goals — Fort #5 (double-aquifer map; day 232 autumn y100, PAUSED, 17 living)

## OVERSEER DIRECTION (2026-07-18, session 2): human co-playing live.
## East-side (INNER-fort, past the bridge) housing wing in progress so a
## siege lockdown still houses most dwarves. Lever stays EAST-only — a
## west lever would give besiegers bridge control (proposal withdrawn).
## Iron arc is the live thread: limonite at (63,38,125); queen's
## MAKE-ANVIL mandate (~25k ticks from day 232) is the clock. Engineering
## wave (work-details + trade tooling) queued for the next natural break
## — full gap list in the session-2 Tool state section below.

## NOW (survival)
- [x] Aquifer pierce+seal (both z=131 AND z=130 soil layers) — DONE and
      holding. Second deep stone aquifer at z=123-121 NOT yet approached
      (dry stone band z=124-128 has been enough so far).
- [x] Drink chain re-established (was 5 drinks/0 plants/no farm at
      session-2 open): FIRST FARM PLOT built (2x5 at (53-54,41-45,133),
      plump helmet all seasons, producing) + gather sweep (113 shrubs)
      + still brewing. Watch: 17 mouths now — keep brew jobs queued.
- [x] Food chain: fishery + kitchen built on surface near depot;
      PrepareRawFish clearing the ~53 raw mussels; PrepareMeal via
      manager work orders. Was 3 edible items at worst — recovering.
- [ ] IRON ARC — TOOLING-BLOCKED at the last step (day 256): industry
      quarter BUILT at z=125 (wood_furnace (75,41) + smelter (78,41) +
      metalsmith (75,45), embark anvil consumed into the forge), 7
      limonite + 2 coal bars + charcoal jobs running — but SmeltOre is
      unreachable: unwired in queue_job (material pinning) AND the
      `order` path fails silently for it (phantom completion alerts,
      no bars, order never decrements). Make-anvil mandate expired
      mid-smelt-attempt (no justice system — no observed consequence).
      UNBLOCKS via engineering wave; everything else is staged.
- [ ] `order` TRUST BOUNDARY (session-2 late finding): PrepareMeal
      verified real end-to-end (18 meals exist). MakeFigurine/SmeltOre
      emit blank-material "(N) has been completed" alerts with NO item
      produced and no order decrement; ConstructBlocks never dispatched.
      NEVER trust an order-path completion alert without a stocks
      check. The earlier "order routes around queue_job" claim is
      WRONG beyond meals — corrected in decisions.md.
- [ ] Second miner RESOLVED-manually: fort had 2 picks all along;
      overseer assigned atér via the client's Work Details tab (see Tool
      state: set_labor writes a dead layer). Woodcutting same (2
      woodcutters manual, 96 logs banked). Tool fix queued.
- [x] no_active_hostiles — 0 hostiles all fort.

## SOON (headroom)
- [ ] East housing wing z=125 (~5/8 pods dug): finish dig, then 8x
      (bed+door+cabinet+zone+assign) for the day-206 migrant wave.
      6 beds + 8 rock doors queued. Carpenter has min_skill=3 profile
      (dabbler protection).
- [x] Bedroom capacity for the OLD roster: Row-3 pods furnished+assigned
      (olon/zulban/methkat) day 207; 9 owned bedrooms total now.
- [x] Dining hall zoned+furnished (2 tables; 1 chair placed, more rock
      thrones queued as mason gets to them).
- [x] has_min_dwarves_14 — 17 living after the +8 wave (day ~206).
- [ ] Queen's suite still owed: monarch requires office/bedroom/dining/
      tomb each >=10000 value + 10 boxes/5 cabinets/5 racks/5 stands
      (real numbers via noble_demands). She's still in a 4x3 pod.
- [ ] Mandate watch: MAKE ANVIL (~25k ticks from day 232) — the iron
      arc IS the fulfillment path. Export bans standing: ANVIL +
      FIGURINE (never trade these). Fig-make mandate open question:
      a claystone figurine completed via work order did NOT credit it
      (reader verified correct vs client — DF-side subtlety).

## EVENTUAL (trajectory)
- [ ] Second deep aquifer (z=123-121, stone) — not yet approached; will
      need its own full pierce/ring/seal cycle per aquifer-piercing §8
      if the fort ever needs to descend past z=124.
- [ ] Queen's real noble suite — Monom currently lives in a standard 4x3
      bedroom pod like everyone else; a proper suite is unbuilt.
- [ ] Floodgate ConstructFloodgate fix (shipped 2026-07-17 per git log,
      commit 18729ec) — still not live-verified against an actual build
      in THIS fort; the lever/door/hatch/bridge chain all re-verified
      fresh this session, floodgate wasn't touched.
- [x] Tomb/coffin chain — FIXED in-session (not via workflow, done by hand
      immediately after confirming root cause): `ConstructCoffin` added to
      `jobTypeAllowedAtWorkshop`'s Carpenters/Masons case group in
      `work_orders.cpp`, identical shape to the ConstructFloodgate fix
      (commit 18729ec). Compile-verified only — NOT yet deployed to a
      running DF or live-tested. `designate_zone type=tomb` already
      confirmed working standalone. NEXT SESSION FIRST ACT: redeploy the
      DLL, build+place a real coffin, properly bury döbar (still in the
      checkpoint moat pit).

## OVERSEER DIRECTION (2026-07-19, post-wave-4): SIEGE PREP + QUEEN'S FLOOR
- Siege doctrine: first sieges = HOLE UP (burrow + civilian alert +
  bridge up), never surface; they leave in time. Not expected until
  wealth rises or an artifact mood fires. NEXT WAVE = nobles + military
  tooling (explicitly reopened by overseer).
- Stockpile shuffle: SOFTENED (overseer realized the big z=126 all-pile
  already sits east of the moat, inside the lockdown zone). No grand
  migration needed — keep growing specialized z=125 piles organically
  (bars_blocks/food/stone already placed) and let the expansion add
  capacity.
- Equal priority: (a) general expansion east+down past the checkpoint
  (playable area; NOT super deep), (b) QUEEN'S SUITE on her own
  elevation — z=124 chosen (deepest dry-band level, east side, own
  noble floor; stub future noble rooms there). Her standards are HIGH
  (4 rooms >=10k value each): this is the quality experiment —
  smoothing, engraving (zulban Lvl10), masterwork furniture via gated
  workshops, possibly gem windows/encrusting (Jewelers + CutGems/
  EncrustWithGems are wired but unbuilt/untested). Expect tooling gaps
  (engraving designation? encrust targeting?) — hit naturally.
- More miners wanted but CAPPED AT 2 PICKS: pick-subtype forging still
  impossible (MakeWeapon has no subtype param) — top next-wave item
  alongside nobles/military and cancel/edit-work-order tooling
  (overseer cleared the stale orders by hand this time).
- Automation: use RECURRING work orders (wave-4 frequency param) so
  drink/meals/fuel/blocks run without per-workshop tool calls.
## Checkpoint architecture (2026-07-17, NEW — built this session)
Layout, surface to protected fort: 2x2 spine shaft (45-46,44-45) surface
z=134 → aquifer-sealed z=131-130 → guard room z=126 (42-48,41-47, first
thing anyone descending reaches) → corridor east → 4x3 MOAT (51-54,43-45)
→ BRIDGE (built type=bridge width=4 height=3 center=(53,44,126)
direction=raise_e) → mechanic workshop (57,44) + lever (60,44), both on
the protected/east side → connector → large stockpile hall (64-75,37-50,
`stockpile category=all`).

- Bridge/lever/link_building/pull_lever all VERIFIED working (raise AND
  lower both completed cleanly, job-queue-empty = success per the
  established no-visual-feedback pattern from Fort #4).
- **Known failure mode, now walled off**: the guard room's south wall got
  breached across its FULL WIDTH (14 tiles) during a panic emergency-
  bypass dig (see journal) — this has been re-sealed with constructed
  walls. If anyone ever "just needs a quick side tunnel" near this
  checkpoint again: DON'T — it defeats the entire point, and building a
  wide mine designation instead of a single connector tile is exactly
  how it got 14 tiles wide instead of 1 last time.
- **Standing risk, not yet mitigated**: the lever is the ONLY way to
  operate the bridge, and the ONLY path to the lever is across the
  bridge. If it's ever raised while defenders are on the wrong side
  again, the same lockout recurs. No second lever, no redundant safe
  path exists yet — worth designing before this is ever used in a real
  emergency rather than as a demo.

## Tool state (2026-07-17, Fort #5 session 1)
- CONFIRMED WORKING LIVE (repeat of Fort #4 findings, unchanged):
  multi-layer aquifer pierce+seal, `build type=updownstair` stair repair
  (not needed this fort, no incident), whole-room zones, brewing via
  reaction path + fresh-barrel workaround, mechanism chain (workshop→
  mechanism→lever→link_building→pull_lever, bridge target specifically).
- STILL BROKEN: `has_modified_anything`/dug-tile counts read 0 all
  session despite dozens of real buildings and hundreds of dug tiles —
  same regression as every prior fort, use elevation/cross_section/look
  as the substitute (unchanged guidance).
- FIXED post-session, compile-verified NOT live-verified (workflow
  wf_6ea04eea-68b, 2026-07-17): a `mandates` MCP tool now exists —
  `df::global::world->mandates.all`, one JSON object per mandate (mode,
  item_type/subtype, material, amount remaining/total, ticks_remaining,
  issuer, full punishment struct). Confirmed against the DFHack 53.15-r2
  source (`df.mandate.xml`/`df.crime.xml`), not guessed. Open question
  the source can't answer: whether `punishment` fields are populated at
  ISSUE time or only once broken — live-verify against this fort's own
  real mandates (export ban x2, "construction of certain goods") next
  session, first call after DLL redeploy.
- FIXED post-session, compile-verified NOT live-verified (same workflow):
  `dwarves`/`dwarf_detail` now surface a `dead` field / `[DEAD]` tag via
  DFHack's `Units::isDead()` (`flags2.bits.killed || flags3.bits.ghostly`).
  Root cause was real: `entities.cpp`'s `isFortControlled()` fallback
  (reached once `isCitizen()`'s sanity check excludes a dead unit) never
  checked death, so a corpse stayed classified as a living dwarf forever.
  Live-verify next session against döbar's own corpse (still in the moat
  pit, unburied) as the first real test case.
- FIXED post-session, compile-verified NOT live-verified (by hand, root-
  caused via source after the user's explicit follow-up ask): `look`'s
  "already visible wall, not showing 'd'" bug. Root cause was NOT a
  render-priority issue (render.go already lets 'd' win over any base
  glyph unconditionally) — `queryMapSlice` only read the raw
  `des.bits.dig` bit, which DF clears the instant a unit CLAIMS the dig
  job, well before the tile is dug. A mid-dig wall tile was therefore
  never even IN the designated set, genuinely indistinguishable from
  untouched rock. Fix: OR in `collect_dig_job_targets()` (already existed
  in this codebase for the entity/topology path, just never adopted
  here) — see journal for full detail. Same JSON field, no protocol
  change.
- NEW THIS SESSION:
  - `build type=bridge` center-tile math is NOT simply "gap midpoint" —
    an even-width footprint can land offset by one tile from the
    intended gap (confirmed: width=4 centered at the visual left-middle
    landed one tile short). ALWAYS `look` the built/planned footprint
    against the actual gap before letting construction proceed; fixing
    post-hoc is cheap (`remove_building` is instant pre-completion) but
    cheaper to just check first next time.
  - `designate_dig type=mine` can throw a REAL (not damp-related)
    "Inappropriate dig square" cancel on some tiles for reasons not
    fully diagnosed this session — a thin 1-tile-wide corridor line hit
    it repeatedly on the same tiles; widening to a full rectangular
    block resolved it. Possibly related to diagonal-only adjacency to
    revealed space, unconfirmed.
  - `stocks category=pick` and `category=mechan` both correctly return
    empty/no-match — picks and mechanisms are real stocked item types,
    just need `category=mechan` → nothing (mechanisms show under
    **TRAPPARTS**, not a "mechanism" category name — check unfiltered
    `stocks` output if a filtered query comes back empty and you expect
    otherwise).
  - `remove_building` on a building that's still only PLANNED (stage
    0, not yet built) removes it instantly with no deconstruction wait —
    useful for immediate do-over of a misplaced building order.

## SECOND WAVE shipped (2026-07-18, 14-agent workflow, compile-verified
## ONLY — DF stayed closed all session, nothing below has run live yet)
- Audit fixes: 12 more job types added to the workshop whitelist
  (Statue/WeaponRack/ArmorStand/Grate at both Carpenters+Masons;
  Slab/Quern/Millstone Masons-only; Bin/Splint/Crutch/AnimalTrap/
  PipeSection Carpenters-only), `ConstructBed` removed from Masons
  (DF has no stone beds — was a real latent bug), furnace jobs/reactions
  now reachable at all (`MakeCharcoal`/`MakeAsh` direct; furnace-hosted
  reactions like pig iron/steel via the generalized reaction-compat
  loop) — `SmeltOre`/`MeltMetalObject` deliberately NOT wired, they need
  per-job material pinning this command has no parameter for, honestly
  flagged not faked. `order item=drink` now builds a real
  CustomReaction+reaction_name order. 12 announcement severity gaps
  fixed (moods/deaths/mandates/attacks no longer default to info).
  `create_location type=hospital` added end-to-end.
- `build` material-byte bug FIXED FOR REAL (was a silent no-op the whole
  project). New `quality` param (Ordinary..Artifact, "Masterful" not
  "Masterwork") lets a call target one specific existing item —
  the actual "masterwork bed in the queen's room" mechanism, via
  `constructWithItems` instead of `constructWithFilters`. `coffin` is
  now placeable (closes the loop on last session's ConstructCoffin fix
  — döbar can actually be buried once this is live-verified).
  New `set_workshop_profile` action (min/max skill level + optional
  worker whitelist) — the skill-gating lever.
- New queries: `moods` (ported from DFHack's own showmood.cpp — dwarf,
  mood type/stage, claimed workshop, demand list w/ progress, raw
  mood_timeout countdown), `noble_demands` (position requirements +
  ACTIVE unit_demand entries — confirmed a wholly separate structure
  from `mandates`), `fort_wealth`, `zone_value` (component-level, keeps
  DF-formula-confirmed value separate from an explicitly-labeled
  experimental `getPersonalValue` number), `wellbeing` (roster-scale
  stress/needs). `stocks` gained a quality-tier breakdown + a
  `mechanism`→TRAPPARTS alias. `mandates` resolves item_subtype to a
  real name now. `dwarf_detail` gained a full `psyche` section (stress+
  DFHack's own 0-6 band, needs, emotions/thoughts, personality) and its
  `mood` field is a real enum name now, not a bare int.
- Two non-blocking findings from final review: `renderMoods`/
  `renderNobleDemands` have no unit tests (every sibling render func in
  this wave got one); `set_workshop_profile`'s `worker_unit_id` is a
  bare int with an `>0` presence check (can't target unit id 0) instead
  of this codebase's established `*int` pattern — low real-world risk,
  worth a quick consistency fix.
- NEXT: live-verify every item above (deploy DLL first — DF still
  closed is the opportunity), starting with rebuilding brew stock and
  finally burying döbar via the new coffin+tomb-zone path.
## THIRD WAVE shipped (2026-07-18, 17-agent workflow, compile-verified
## ONLY — survived 2 mid-run disconnects, cleanly resumed both times)
- `build` reshaped: curated ~29 names keep old wire bytes; a new
  name-passthrough (BUILD_TYPE_BY_NAME) resolves ANY building/workshop/
  furnace/trap_type name plugin-side via find_enum_item; new
  `building_types` discovery tool (58 entries) carries all per-type
  facts that used to bloat `build`'s description. `build` itself is now
  a 4-sentence pointer + a terse enum, down to 2,523 bytes.
- ~30+ new building types placeable: Well (+MakeChain), Support, full
  room-value furniture family (Statue/Slab/WindowGlass/WindowGem/
  Bookcase/DisplayFurniture/OfferingPlace/Instrument), water/power
  infra (ScrewPump/GearAssembly/Axles/WaterWheel/Windmill/Rollers),
  ArcheryTarget/TractionBench/NestBox/Hive, 4 trap subtypes
  (PressurePlate/StoneFallTrap/WeaponTrap/TrackStop — CageTrap still
  excluded, needs the still-excluded MakeCage item), new furnaces
  (Kiln/GlassFurnace/magma variants/MagmaForge), 10 new workshops
  (Jewelers/Bowyers/Siege/Leatherworks/Tanners/Clothiers/Loom/Kennels/
  Ashery/Dyers). Every real limitation documented, not hidden: magma
  buildings can't verify magma-adjacency at placement; stone_fall_trap
  builds unarmed (no "load trap" job exposed yet); pressure_plate has
  trigger-detection off; axle/rollers are single-tile only; several
  furniture types need a tool-item subtype this project still can't
  craft on demand (pre-existing gap, unrelated to this wave).
- New CLAUDE.md house rule live: "Tool schemas carry shape, never
  guidance." Real CI test (`TestToolSchemaBudget`) now enforces it —
  currently 65 tools / 43,666 bytes vs a 61,440-byte ceiling.
- Job-wiring: MakeChain, ConstructTractionBench, baseline queue_job
  reachability for all new workshops.
- Final review: C++ rebuilt with /W3 /WX (warnings-as-errors) for a
  genuinely-clean signal, not just a pass. All 71 BUILD_TYPE_* constants
  verified byte-identical protocol.h<->message.go. Scope boundary clean
  (zero Cage/Chain/MakeCage/CageTrap — only exclusion comments).
- Known non-blocking follow-up: `build`'s legacy type enum still lists
  ~32 names (reformatted not shrunk this wave, deliberately) — trimming
  toward the ~15 house-rule guideline is a clean future pass.
- DLL ALREADY DEPLOYED (2026-07-18, post-workflow): the fully-rebuilt
  `df_ai_protocol.plug.dll` (508KB, all three waves included) is already
  copied into the Steam DF `hack/plugins/` folder. NO redeploy step
  needed next session — just launch DF, `load df_ai_protocol` (or it
  auto-loads), `ai-connect`, and go straight to live-verification.
- NEXT: live-verify THREE waves' worth of compile-only work, in order —
  glyph/mandate/dead-status fixes first (they're foundational to
  observing everything else correctly), then wealth/moods/quality, then
  this build-reshape + new building types.

## Tool state (2026-07-18, Fort #5 session 2 — LIVE VERIFICATION DONE)
- VERIFIED LIVE this session: dead field/[DEAD] tag; 'u' pending-building
  glyph (+ correct flip to '#' when built); mid-dig 'd' persistence;
  mandates (punishment populated at issue; reader matches client);
  moods (clean empty state); noble_demands (live-updates on overseer's
  manual appointments); fort_wealth; wellbeing (excludes dead);
  stocks quality tiers; ConstructCoffin at Masons; build curated path w/
  REAL material byte (stone/wood/blocks all claimed correctly); build
  quality-tier placement (constructWithItems names the item picked);
  build NAME-PASSTHROUGH (uncurated `Statue` placed); building_types;
  set_workshop_profile (min_skill=3 on carpenter); farm plot+assign_crop
  (incl. truthful stage-0 refusal); gather; chop; remove_zone;
  remove_building (wagon); cancel_designation; assign_zone accepts a
  DEAD unit for tomb burial; manager `order` path end-to-end (meal,
  blocks, MakeFigurine — manager fills reagents for ANY job_type,
  routing around queue_job's crafts rejection).
- BROKEN LIVE: `zone_value` — "no zone at coordinates" on every real
  zone tested (dining hall tile, owned-bedroom tile). Only wave-2 tool
  that failed. Needs root-cause vs Buildings::findCivzonesAt or
  whatever lookup the C++ uses.
- ENGINEERING QUEUE (next wave, rough priority):
  1. WORK-DETAILS LABOR TOOLS — research DONE (Sonnet agent, source-
     cited vs 53.15-r2): set_labor writes unit.status.labors, which v50
     derives FROM plotinfo->labor_info.work_details; correct write =
     edit work_detail.assigned_units (sorted insert/erase) + call
     Units::setAutomaticProfessions(unit). Proposed tools:
     work_details (read), assign_work_detail, set_work_detail_mode,
     create_work_detail (8 CUSTOM slots). Avoid the autolabor global
     bypass flag (kills the client UI the overseer uses). Beware
     NobodyDoesThis-mode semantics (UNVERIFIED). Full report in the
     session transcript / cross-session memory.
  2. TRADE TOOLING — depot exists; caravan window was lost to manual
     play this autumn; next caravan ~spring. Needs research (bring-
     goods + trade-screen interaction via DFHack).
  3. zone_value fix (above).
  4. look lens=minerals (overseer request): name the '=' veins —
     limonite vs saltpeter vs morion matters now that ore exists. Also
     consider vein-extent visibility.
  5. Paper-cuts: bridge raised/lowered state invisible (buildings tool
     shows only "built"); buried/dead units still paint '@' at stale
     positions in look + dwarves keeps last-position record (burial
     state invisible); pooled-announcement repeats resurface with
     FIRST-occurrence coords (misleading @(40,40,133)-style positions)
     and recycled ids; order-completion alerts render blank item names
     ("(3) has been completed"); `order` has no repeat/cyclical flag
     (overseer wants recurring blocks orders); stocks can't show weapon
     subtypes (pick vs axe invisible — drove a wrong "no axe" theory);
     designate_dig's not-connected hint points at map corner (5,0,134);
     no warning when a dig designation overlaps a BUILDING footprint
     (cabinet at (40,40,133) + mechanic workshop vs stair shaft — both
     canceled "Inappropriate dig square" with no tool-visible cause).
  6. queue_job crafts-class (MakeFigurine etc.) at Craftsdwarfs —
     LOWER priority now that `order` routes around it; wire properly
     or document order as the canonical path.
  7. Fig-make mandate not credited by a completed claystone figurine
     (order path) — DF-side crediting subtlety, reader verified
     correct; test queue_job-path crediting when crafts get wired.
- WAVE 4 SHIPPED + DLL DEPLOYED (2026-07-19, 20-agent workflow, survived
  2 mid-run disconnects via resume + prompt-amended stitch; all gates
  green: clean /W3 /WX rebuild, full go test, protocol audit table all
  OK, schema budget 72 tools / 49,885B vs 61,440 ceiling). Compile-
  verified only — NOTHING below is live-verified yet. Shipped:
  - Work-details labor tools: work_details (query), assign_work_detail,
    set_work_detail_mode, create_work_detail (commands 0x20-0x22) —
    membership edit + Units::setAutomaticProfessions, per the research.
    set_labor kept but documented as the derived layer.
  - Orders root-cause FIXED: applyWorkOrder never set mat_type/mat_index
    or material_category — the phantom-order cause (meals worked only
    because PrepareMeal has no material ambiguity). order gained
    material + frequency (recurring!) params; SmeltOre now wired in
    queue_job DIRECT path with pinned ore material (the iron-arc
    unblock). "unknown material" text was DF's truthful rendering of
    our unset materials, not a render bug.
  - zone_value FIXED; look lens=minerals (dynamic per-view legend);
    bridge raised/lowered in buildings; stocks weapon/tool subtype
    names; alert repeats annotated (count + first-occurrence position);
    dead units no longer paint '@' (+ buried-in-coffin detection and
    death-tick in dwarf_detail); designate_dig warns on building-
    footprint overlap + connector-hint fixed.
  - Trade (safe subset): bring_goods_to_depot command (0x23) +
    caravan_status + depot_goods queries. Actual trade-screen COMMIT is
    honestly scoped out (viewscreen-bound, no safe headless path) — a
    human still executes the exchange; we can now see caravans and
    stage goods. Live-test at the SPRING caravan.
  - 2 known non-blocking nits: build enum still lists Coffin/Well/
    Support (bloat holdover); handleDepotGoods missing z validation
    (currently unreachable).
  - WAVE 4 LIVE-VERIFIED (same night, 2026-07-19, game days 256-274):
    work_details read (full 11-detail list incl. the overseer's manual
    Miners/Woodcutters memberships — first tooling visibility ever);
    create_work_detail ("Metalworkers" only_selected, CUSTOM_1) +
    assign_work_detail (doren) — NOTE: creating an only_selected detail
    does NOT strip cached labors from non-members (méthkat kept
    smelting; DF recompute semantics, as research flagged); SmeltOre w/
    pinned INORGANIC:LIMONITE worked FIRST TRY (announcement text now
    says "limonite ore" — 4 iron bars); set_workshop_profile worker
    whitelist routed ForgeAnvil to doren → WELLCRAFTED IRON ANVIL
    day ~270, full ore→charcoal→smelt→forge chain via own tooling; the
    material-pinned order path made a REAL rock figurine (mandate
    FULFILLED — the phantom-order theory 100% confirmed: material
    ambiguity was the whole story); zone_value lookup FIXED (but
    valuation counted 0 components in a 2-table dining hall —
    follow-up); lens=minerals verified (named the whole vein field:
    jet/lignite/blue jade/limonite/indigo tourmaline — north field is
    limonite-RICH) with one nit (vein letter 'd' collides with dig-
    designation 'd', should join t/u on the reserved list); bridge
    "built [lowered]" state live; dwarf_detail burial detection
    verified on döbar herself ("buried: yes, coffin @(37,40,126)" +
    death tick). ALL make-mandates fulfilled (coffins, figurine,
    anvil); only export bans remain (figurine/coffin — never sell).
    STILL UNTESTED: caravan_status/depot_goods/bring_goods_to_depot
    (spring caravan), dig footprint-overlap warning (no natural case
    yet), recurring order frequency param.
  - NEW SMALL GAPS from live verify: no cancel-order tool AND no
    order-edit (overseer cleared stale orders by hand; also wants
    recurring-order tweaking when industry taxes a needed resource);
    designate_dig's connector hint still points at map corner
    post-reconnect (suspect: computed against sparse world model before
    resync fills in — not fixed by wave 4's attempt); zone_value
    component enumeration; minerals-lens 'd' collision; stocks should
    render STACK-UNIT totals for food/drink (8 wine stacks = ~80
    servings — client vs tool discrepancy confused both of us, day
    289); pick/weapon SUBTYPE forging param (overseer hand-queued 2
    iron picks); SMOOTH DESTROYS CARVED STAIRCASES with no warning
    (live incident day 299: queen's-floor 2x2 shaft half-lost to my
    own smooth rects — smooth needs the same overlap warning
    designate_dig got, for stairs/carved features); cancel_designation
    is BLIND to smooth/engrave designations (only clears dig bits —
    overseer had to cancel the remaining smooth marks in-client);
    look scope=fort ROOT CAUSE confirmed on the fort tour: the Go
    footprint derives from session-observed tile deltas, so digs from
    before the connection (old saves/prior sessions) are invisible —
    fix = derive footprint from MAP STATE (carved/constructed/smoothed
    tiletypes via plugin-side scan/floodfill), not the session journal;
    ALSO world-switch staleness: entity cache served the PREVIOUS
    world's roster verbatim after a save-swap until a step forced
    resync — the sentinel should detect world identity change.
  is a LID over its own pit — items lure haulers in whenever open;
  closing entombs them (killed döbar, trapped 2 more). Pit census
  (z=125) + deck census are MANDATORY before every close. Puller must
  be east-side. Overseer's emergency pit-exit is floored over at
  (52,42,126ish); bypass at (49-55,42,126) re-walled with blocks.
