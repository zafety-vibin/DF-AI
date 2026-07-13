# Skills for the early fort — proposal for Feature 009 workstream 3

**Date**: 2026-07-12 (dated snapshot; frozen history per the stateless-docs policy — verify tool
names and gap status against `internal/mcpserver/tools_*.go` and `fortress/memory/goals.md` before
acting on them)
**Scope**: a concrete elaboration of `specs/009-culture-and-learning/design-brief.md` workstream 3
("Skills as culture"), distilled from the two live First Fort sessions
(`fortress/memory/journal.md`, `fortress/memory/learnings.md`, ~103 game-days), the goals hierarchy
(`fortress/memory/goals.md`), the MCP tool surface as registered today, and the token-scaling
analysis (`docs/archive/2026-07-12-token-scaling-research.md`). Answers the overseer's three asks:
early survival, wealth generation, early defense preparedness.
**This document designs skills; it does not write them.** Target artifact per the brief:
`fortress/.claude/skills/<name>/SKILL.md`.

---

## Summary

**14 skills proposed: 7 Early Survival, 4 Wealth Generation, 3 Early Defense Preparedness.**
Eleven are experience-derived (grounded in a specific live incident from sessions 1–2); three carry
curated-reference sections that must be marked unverified until live play confirms them
(Voyager's verified-only rule, per the brief). Each skill lists the exact MCP tools it composes,
the NOW/SOON/EVENTUAL tiers it advances, and any named engineering gap it is blocked on.

**The single most load-bearing finding**: across ~103 game-days, every costly mistake was a
*silent* failure — damp-cancelled designations that quietly vanish, building plans that die
without an announcement, completions that announce nothing, a designation gap invisible in a
count-only readout (4,800 ticks burned), sentinel data after reconnect, `find_dig_site` offering
already-carved floor as "solid". Meanwhile the pre-play skill library (`internal/skill/builtins.go`)
was **falsified exactly where it was specific** (smooth-first aquifer sealing fails on sand;
manager-first production never dispatches on a fresh fort) and **validated where it was purely
procedural** (dig down first, one everything-stockpile, chokepoint entrance). Two consequences
shape everything below:

1. **Every skill ends in a mandatory observe-and-verify step naming a real tool.** Procedure
   without verification was worth nothing in practice. This also makes skills compose with
   `check_goals`-as-critic (brief workstream 4) for free.
2. **The conversion of the 11 builtins is a rewrite, not a transcription.** The builtins contain
   house-rule violations (room dimensions: "3×3 to 4×4", "5×5+", "10×10+"), dead tool names
   (`region_detail`, freeform `dig`/`order` grammar), a falsified aquifer protocol, and an
   inverted production doctrine. A sonnet "transcription-grade" pass (per CLAUDE.md delegation
   rules) would faithfully transcribe the bugs.

---

## Design rules applied throughout

### The house rule, operationalized

Per `docs/decisions.md` 2026-05-03 and `CLAUDE.md` "House rules": skills carry **order,
dependencies, tradeoffs — never dimensions, materials, or coordinates**. Every skill below passes
the scale test: *the same recipe serves a 2-dwarf hovel and a 50-dwarf mountainhome; only the
model's parameter choices differ.*

One distinction the authoring pass (and the brief's proposed skill linter) needs to make
explicit — **mechanical invariants are not "specifics"**:

- ALLOWED (game mechanics / project law): stairwells default 2×2 spanning many z in one
  designation (decision 2026-05-03); aquifer seepage is orthogonal, so the seal ring is "the
  orthogonal neighbors of the shaft"; hatches go over stair openings, doors between walls;
  1200 ticks = 1 game day; workshops are 3×3 with a center coordinate.
- BANNED (parameter choices): room sizes, stockpile sizes, wall/door materials, absolute
  coordinates, item counts ("order 10 beds"), day-number deadlines ("produce food by day 30").
  All of these appear in the current builtins and must be stripped.

### Progressive disclosure (token budget)

Per the token-scaling research, anything loaded every turn is a standing tax. Claude Code skills
already have the right shape: the frontmatter `description` is the always-loaded part; the body
loads only on invocation. Applied here:

- **Descriptions are the trigger lines below** — one sentence each, ~15–25 words. 14 skills cost
  roughly 400–600 tokens of always-loaded surface, comparable to a single `look` call.
- **Bodies carry the procedure** — the ordered steps and decision points, still compact.
- **Reference depth goes in auxiliary files** inside the skill directory (e.g. the full
  aquifer protocol narrative, DF door-mechanics background), read only when the model asks.
  This mirrors the established crop-overview → detail-query perception pattern
  (`docs/guides/world-model.md`).
- **The charter (`fortress/CLAUDE.md`) keeps the always-in-context 6-step turn loop**; the
  turn-discipline skill holds the extended depth (pacing tables, crisis posture, resync). Do not
  duplicate the charter into a skill body verbatim — split by load frequency.

### Verified-only entry

Skills marked **[experience-derived]** cite the live incident that produced them. Sections marked
**[curated reference — unverified]** encode standard DF knowledge the fort has not yet exercised
(crafts production, hostile response); per the brief they ship as clearly-labeled reference
material and get promoted (or corrected) by the learning loop after first live contact.

---

## Section 1 — Early Survival (7 skills)

### 1.1 `turn-discipline`

- **Trigger**: use every session — pacing, patience, and resync rules for the pause→observe→act→step loop; consult when unsure how long to step or whether to re-issue a command.
- **Procedure** [experience-derived]:
  1. Confirm PAUSED via the dashboard header before reasoning; trust the header over memory.
  2. Observe alerts FIRST, then look/cross_section at the active worksite. After reading and
     acting on an alert, `dismiss_alerts` — nothing auto-clears, and an undismissed alert set
     decays into wallpaper.
  3. Scale step length to stability, not curiosity: short steps while fluids/threats are active;
     multi-day steps when the fort is stable. During heavy fluid simulation a step can exceed the
     poll deadline ("DID NOT complete") — call `status`/`pause` to resync, don't panic and don't
     re-issue.
  4. Never re-issue a command that simply hasn't finished — dwarves complete work over game-days.
     Judge progress by alerts and step deltas, never by re-checking tiles every turn.
  5. Reconnect protocol: after `/mcp` + `ai-connect`, "Connected" can front-run real data —
     sentinel values (year=0, dwarves=0) may persist for several queries. Call `pause` and
     re-query until the calendar reads sane; don't mistake this for a broken connection.
  6. Verify: end each turn by reading the step report's deltas; end each milestone with
     `check_goals` before claiming it.
- **Tools**: pause, alerts, dismiss_alerts, look, cross_section, step, status, check_goals.
- **Tiers**: meta — protects all NOW/SOON/EVENTUAL work.
- **Status**: fully usable today.
- **Worked example**: learnings "Pacing" (400–800 ticks during the aquifer crisis, 1200–2400
  stable; step-poll timeout during heavy fluid sim) and "Reconnecting mid-session" (both
  session-2 reconnects returned sentinel data for 3–4 calls). The 4,800-tick burn in learnings
  "Designation overlays" is the canonical cost of stepping without verifying first.
- **Scale test**: pacing-by-stability and verify-before-claiming are population-independent.

### 1.2 `fresh-embark-opening`

- **Trigger**: use on any new fort or fresh embark — the ordered opening sequence from wagon to first sheltered, provisioned room.
- **Procedure** [experience-derived]:
  1. `survey_site` first (embark orientation), then `cross_section` at the intended shaft site to
     read soil depth, stone start, and any DAMP/AQUIFER bands before committing to a location.
  2. Choose the shaft site with `find_dig_site` near the dwarf cluster — but cross-check the
     candidate with `look` (see `designation-hygiene`).
  3. Designate the 2×2 stair shaft in ONE `designate_dig type=stairs` call spanning many z
     (project law). Rooms branch BESIDE the spine on each level, never on top of it.
  4. In parallel (designations are free while miners work): `chop` a stand of trees and `gather`
     surface plants — both are pure designations that start the wood and food inflow without
     occupying the miner.
  5. Know the embark material reality before planning any build: a default embark has ZERO
     construction boulders ("ROCK" stock items are NOT boulders), and only a few wagon logs.
     Therefore the first workshop must be one that accepts wood, and stone-dependent plans wait
     for quarry output.
  6. Carve the first storage room off the shaft and designate one everything-stockpile in it
     (see `stockpile-architecture`), so wagon goods move underground early.
  7. Verify: `look` at the carved levels, `stocks` for the wood/food inflow, `check_goals` for
     the shelter predicate — noting the census caveat in `labor-and-census-triage`.
- **Tools**: survey_site, cross_section, find_dig_site, look, designate_dig, chop, gather,
  stockpile, build, stocks, step, check_goals.
- **Tiers**: NOW (shelter, wood chain, food inflow).
- **Status**: fully usable today.
- **Worked example**: journal session 1 — the whole opening (shaft (46,50)-(47,51) z140 down,
  storage at z139, wagon reality of 3 logs + 3 non-boulder "ROCK" items, fisherdwarf producing
  unprompted); learnings "Construction materials".
- **Scale test**: same sequence at any embark size; the model picks depth, room count, and
  which chains to start first.

### 1.3 `designation-hygiene`

- **Trigger**: use whenever designating digs — connectivity checks, silent-cancel detection, stair continuation, and when not to trust find_dig_site.
- **Procedure** [experience-derived]:
  1. Before stepping on any NEW designation, `look` at the BOUNDARY between existing floor and
     the new designation — not just the new room's interior. A gap means the work is unreachable
     and zero progress will occur, with no error anywhere. (This is the workaround until 009's
     designation-overlays-in-`look` lands; retire this step when it does.)
  2. Treat `find_dig_site` candidates as hypotheses near recently-developed space: it has
     re-offered already-carved floor as "fully solid". Cross-check with `look` before
     designating. (Named gap: find_dig_site stale-solidity bug, filed in decisions 2026-07-12.)
  3. Stair continuation: the tool now joins a stairs range whose top adjoins existing carved
     stairs (008 fix wave), but the underlying mechanic still bites in edge cases — a stairs
     designation starting at an undug level below existing stairs carves a down-stair with no up
     component, stranding the level. If a shaft top is already a dead down-stair, don't fight it:
     dig a side corridor at the deepest reachable level and sink a fresh shaft beside it.
  4. Damp/aquifer warning-cancels are LOUD ONCE then silent (announcement dedup). A designation
     that quietly vanishes = damp-cancelled. Designating the same tiles a SECOND time digs them
     for real — this is deliberate, not a retry loop.
  5. Mis-designations are cheap to fix while undug: `cancel_designation` immediately rather than
     letting a wrong dig complete.
  6. Verify: after each step, compare dug-tile progress in `look`; zero progress over a full step
     means a connectivity or cancellation problem, not slow dwarves.
- **Tools**: designate_dig, look, cross_section, find_dig_site, cancel_designation, alerts, step.
- **Tiers**: NOW (all digging), protects every later tier.
- **Status**: usable today; step 1 is explicitly a stopgap for the 009 overlay workstream.
- **Worked example**: journal session 1 "STAIR QUIRK INCIDENT" (permanently unreachable level,
  abandoned flooding column); learnings "Designation overlays in `look`" (second occurrence —
  6×6 room designated 4 tiles from floor, 4,800 ticks burned); learnings "find_dig_site
  reliability".
- **Scale test**: connectivity and cancellation mechanics are identical at any fort size.

### 1.4 `aquifer-piercing`

- **Trigger**: use when a dig designation vanishes with a damp warning, or cross_section shows DAMP/AQUIFER bands across the descent path.
- **Procedure** [experience-derived — the field-verified protocol, learnings.md "Aquifer
  piercing", which SUPERSEDES the smooth-first builtin (sand aquifers are unsmoothable)]:
  1. Confirm the wet layer's extent with `cross_section` (DAMP/AQUIFER annotations render
     honestly, including on hidden tiles — that's deliberate fog-honest design).
  2. Pierce the layer with the 2×2 stair shaft — the second designation after the warning-cancel
     digs for real. Light-aquifer seepage is slow: days per tile of water, so this is a project,
     not an emergency.
  3. Freeze the descent a level or two below the aquifer (`cancel_designation` on deeper levels)
     so seepage pools in a small catch basin instead of the lower fort.
  4. Mine the orthogonal neighbors of the shaft at the aquifer level (corners can stay natural —
     seepage is orthogonal). Expect one silent damp-cancel round; re-designate.
  5. Immediately queue constructed walls in every mined ring tile (`build` wall; builders work in
     shallow water). ALL FOUR SIDES must end as constructed wall. Material reality: constructions
     bind non-economic boulders only — check `stocks` for real BOULDER items before starting, and
     stage spares for the last side.
  6. Adjacent standing water (an old flooded pocket) is NOT a reason to skip a side: mine
     through, let the small volume dump and drain down the shaft (it spreads thin and evaporates
     once inflow stops), and wall the gap.
  7. NEVER build a wall ON a stair tile to blind a wet face — it destroys the only descent.
  8. After the seal: resume descent from a DRY offset (a room a level below) rather than through
     the puddle at the old bottom; the wet column becomes a future cistern (EVENTUAL goal).
  9. Crisis pacing throughout: short steps, one structural project at a time, verify inflow via
     `cross_section` water digits each turn.
- **Tools**: cross_section, designate_dig, cancel_designation, build, look, stocks, alerts, step,
  unsuspend.
- **Tiers**: NOW (shelter/descent), EVENTUAL (cistern from the sealed column).
- **Status**: fully usable today; live-verified end to end.
- **Worked example**: journal session 1 (the entire aquifer arc: pierce, ring, 8 shale walls,
  pocket entombed west) and session 2 verification (seal holding, ~1/7 residual puddle). This is
  the fort's single largest earned asset — the skill IS the record of it.
- **Scale test**: the protocol is per-shaft; a mountainhome with three shafts runs it three
  times. No dimension in it beyond the 2×2 law and the mechanical orthogonal ring.
- **Authoring note**: the skill body carries steps 1–9 compactly; the full narrative (why each
  step, the session-1 recovery story) belongs in an auxiliary reference file in the skill dir.

### 1.5 `provisioning`

- **Trigger**: use at fort start and whenever food or drink stocks trend down — which chains run themselves, which need designations, which need queued jobs, and what is still blocked.
- **Procedure** [experience-derived]:
  1. Know the three chain classes (this distinction cost a session to learn):
     - AUTOMATIC once the building exists: fishery (auto-processes raw fish, zero queue calls).
     - DESIGNATION-driven: `chop` (wood), `gather` (surface plants) — fire-and-forget rectangles.
     - QUEUE-driven: meals need `queue_job` meal at a kitchen; items need queue_job at the
       matching workshop.
  2. Order of establishment: designation chains first (free while mining proceeds), then the
     automatic buildings, then queue-driven refinement (cooking) once inputs accumulate.
  3. Monitor with `stocks` category filters rather than the full dump (token cost + truncation);
     note the category argument is currently case-SENSITIVE against the plugin — pass uppercase
     (named gap, goals.md SOON).
  4. Drink is BLOCKED end-to-end today: the still can be BUILT but BrewDrink has no job_type
     mapping in order or queue_job (protocolToJobType returns -1; needs a DFHack reaction-based
     lookup). Track drink stock decay explicitly; water is a fallback (unhappy thought, not
     fatal) if stores run dry before the fix lands.
  5. Farming is BLOCKED: farm zones sit behind the zone civzone stub, and no farm-plot build type
     exists. Gathering + fishing are the sustainable chains until then.
  6. Verify: `stocks` deltas across steps (e.g. raw fish rising confirms the automatic chain);
     `check_goals` food predicates at each season boundary.
- **Tools**: stocks, gather, chop, build (fishery/kitchen/still/butcher), queue_job (meal), jobs,
  stockpile (food category), check_goals, step.
- **Tiers**: NOW (food chain — satisfied), SOON (drink — blocked).
- **Status**: food side fully usable; drink blocked on the **BrewDrink job_type mapping** (goals
  SOON); farming blocked on the **zone civzone stub** (009 workstream 1) plus a missing farm-plot
  build type.
- **Worked example**: goals NOW "Fishery built, auto-processing raw fish (no job-queue needed —
  confirmed automatic)"; journal session 1 fish 19→26 unprompted; session 2 gather bringing in
  cotton/lettuce/bitter-melon; goals SOON "Drink: 9 drinks (draining slowly)... BLOCKED".
- **Scale test**: chain classification and establishment order are identical at any population;
  quantity targets are the model's per-fort choice.

### 1.6 `direct-production`

- **Trigger**: use when the fort needs items made (beds, doors, barrels, blocks) — the workshop-first, queue_job-first pattern, and how building plans silently die.
- **Procedure** [experience-derived]:
  1. Dependency order is absolute: material exists → workshop BUILT (not just placed) →
     `queue_job` at the workshop → item lands in a stockpile → `build` places it.
  2. Use `queue_job` for everything early: it queues a job directly at a workshop (vanilla
     right-click equivalent) with NO manager/noble/office requirement. It queues one job per
     call — deliberate, keeps ACKs truthful. Save `order` for bulk/standing production later
     (see `bulk-production-upgrade` — currently a dead end on a fresh fort).
  3. Material reality per item class: wood items need real logs at a carpenter; constructions
     bind non-economic boulders only; "ROCK" stock items are not boulders. Check `stocks` before
     queueing, not after the cancel spam.
  4. Building plans SILENTLY DIE after repeated needs-material cancels ("The dwarves were unable
     to complete the X" = plan removed; later cancels may be silent). After new materials arrive,
     RE-PLACE any building that hasn't visibly completed. Completions are also silent — verify
     with `look` / `buildings`, never assume.
  5. If a placed plan sits idle with materials on hand, check `buildings` for a suspension and
     `unsuspend` after fixing the blocker; if it died, re-place.
  6. Verify: `jobs` at the workshop (queue depth), `buildings` (construction progress — the only
     way to see whether a build order is being worked or died silently), then `look`.
- **Tools**: build, queue_job, jobs, buildings, stocks, look, unsuspend, step; order (flagged
  dead-end for now).
- **Tiers**: NOW (beds — satisfied via exactly this pattern), SOON (office furniture, barrels).
- **Status**: fully usable today.
- **Worked example**: journal session 2 — queue_job ×7 ConstructBed → all 7 beds built (first
  headroom milestone), including the (52,51,139) plan that died silently and was re-placed per
  the existing learning; learnings "Production without a manager" and "Construction materials".
- **Scale test**: the dependency chain is identical for 1 bed or 40; counts and materials are
  play-time choices.

### 1.7 `labor-and-census-triage`

- **Trigger**: use when work stalls despite valid designations and materials, or before any per-dwarf math (beds, food, shelter predicates).
- **Procedure** [experience-derived]:
  1. When work stalls: `alerts` first (DF usually says why), then `jobs` at the relevant
     workshop, then identify WHO can do the stalled labor class via `dwarves` + `dwarf_detail`.
  2. Build the TRUE census before any per-dwarf planning: the `dwarves` count includes pack
     animals and pets (18 listed vs 7 real at First Fort). Real dwarves have a first_name and
     dwarf-typical labor skills; animals show empty names and zero-or-one animal skill. Sample
     `dwarf_detail` across the id list once per session and cache the roster in the journal.
  3. Distrust per-dwarf predicate output until the census filter lands upstream: check_goals'
     has_shelter_N_per_dwarf divides by the inflated count — a false negative on the model's own
     self-critic (concrete impact documented in learnings).
  4. Recognize single-point-of-failure labors: with a handful of dwarves, one specialist covering
     two labor classes serializes the whole fort (the sole miner also cleaning fish halts ALL
     digging fortress-wide). Today's only mitigation is sequencing — time big digs away from
     workload spikes in the specialist's other labor, and keep the specialist's job pipeline
     clear. There is NO tool to reassign labors.
  5. Verify: after triage, one short step and re-read `jobs`/`alerts` to confirm the stall
     cleared rather than assuming the diagnosis.
- **Tools**: alerts, dwarves, dwarf_detail, jobs, check_goals, step.
- **Tiers**: NOW (keeps all NOW work unblocked), SOON (survive first migrant wave — triage
  scales with arrivals).
- **Status**: diagnosis fully usable; remediation blocked on the **set_labor tool** (goals SOON,
  field-confirmed critical day 102) and the **dwarf-vs-animal census filter** (goals SOON).
- **Worked example**: goals SOON set_labor entry — Doren, the fort's ONLY miner (skill 6, no
  other dwarf has mining), also has fish-cleaning enabled; every raw-fish backlog halts all
  digging. Journal session 2 "DWARF CENSUS CORRECTED" (user-caught 18-vs-7).
- **Scale test**: bottleneck diagnosis by roster sampling works at 7 or 70 dwarves; at larger
  populations the same skill says to sample, not enumerate.

---

## Section 2 — Wealth Generation (4 skills)

What this fort actually produced with value, ~103 days in: 7 beds, a multi-species wood stock
(3→13 logs from one chop designation), a growing fish larder, gathered plant materials, 8 shale
constructed walls, and carved living space. Wealth at this stage IS surplus production plus the
storage that protects it — the skills below encode that, and flag precisely where trade itself
is blocked.

### 2.1 `stockpile-architecture`

- **Trigger**: use when placing any stockpile — the centralize-first doctrine and when to specialize.
- **Procedure** [experience-derived — overseer-taught]:
  1. Early default: ONE large carved room with an "everything" stockpile, underground and
     defendable — not several small category piles scattered where things are made. This builds
     the hauling habit: goods flow to one protected spot instead of littering the surface.
  2. Sequence: carve the room (off the shaft spine) → `stockpile` category 'all' → step and
     verify hauling actually starts (`look` shows goods accumulating; `stocks` unchanged totals
     but relocated).
  3. Specialize only when a chain demands it: a food-category pile near the kitchen/fishery came
     second at First Fort, and input piles beside workshops come as industry grows. Tradeoff:
     adjacency multiplies workshop throughput; scattering multiplies surface exposure and haul
     distance. Resolve toward centralization until throughput visibly suffers.
  4. Wealth angle: everything the fort will one day trade or defend sits in this room — its
     location choice is simultaneously an economic and a defensive decision (deep beats near).
- **Tools**: stockpile, designate_dig, find_dig_site, look, stocks, step.
- **Tiers**: NOW (food/general stockpiles — satisfied), SOON/EVENTUAL (protects accumulating
  wealth; feeds the defensible-entrance calculus).
- **Status**: fully usable today.
- **Worked example**: learnings "Storage strategy (overseer tip)"; goals NOW — z134 food-only +
  z132 "everything" placed per that tip.
- **Scale test**: centralize-then-specialize is the same doctrine at any size; room size,
  location, and category split are the model's choices.

### 2.2 `resource-flows`

- **Trigger**: use when planning where materials will come from — quarry doctrine, economic-stone awareness, and reading stocks correctly.
- **Procedure** [experience-derived]:
  1. Stone comes from QUARRY ROOMS, not shafts: carving stair tiles drops boulders at a much
     lower rate than mining rooms. Plan rooms you need anyway (storage, halls) as your quarries —
     every room is dual-purpose: space plus material.
  2. Check what a quarry actually yields before relying on it: economic stone (bauxite, lignite —
     mineral-rich sites are full of it) is INVISIBLE to construction jobs by default. `stocks`
     carries material names and the economic flag — read it, don't assume "struck stone" means
     "usable stone". If a quarry level is all economic stone, quarry a different layer.
  3. Item-type literacy: "ROCK" is not "BOULDER"; only BOULDER items feed constructions. Logs
     feed wood items only.
  4. Wood renews by designation: one `chop` rectangle over a tree stand yielded a 4× multi-species
     return at First Fort. `gather` similarly converts surface flora into food/thread inputs.
  5. Match workshop placement to flows (input pile beside the workshop, output flowing to the
     central stockpile) — order: source → adjacent input pile → workshop → central storage.
  6. Verify: `stocks` category deltas after each production step; `buildings` for idle workshops
     (idle + materials present = a labor or hauling problem, see `labor-and-census-triage`).
- **Tools**: stocks, designate_dig, chop, gather, find_dig_site, cross_section, stockpile,
  buildings, jobs.
- **Tiers**: SOON (headroom industries), EVENTUAL (surplus for trade).
- **Status**: fully usable today.
- **Worked example**: journal session 1 — east quarry struck BAUXITE (economic → unusable),
  west quarry re-targeted hunting layer stone; learnings "Stairs & digging" (8 carved stair
  tiles ≈ 1 boulder) and "Construction materials" (ROCK≠BOULDER, economic-stone invisibility).
- **Scale test**: dual-purpose quarrying and flow-matching hold from first workshop to full
  industry district.

### 2.3 `trade-goods`

- **Trigger**: use when material surplus exists and the goal is convertible wealth — what to make, where value densifies, and exactly what trade is blocked on.
- **Procedure**:
  1. [experience-derived] Wealth begins as surplus from chains already running: extra beds,
     barrels, and blocks are value the fort can hold. Furniture made and BUILT is fort value;
     furniture made and stockpiled is tradeable value — the same production step feeds either.
  2. [curated reference — unverified] The classic convertible-wealth chain is crafts: a
     craftsdwarf workshop + `queue_job` crafts turns surplus stone/wood into value-dense,
     light goods. The vocabulary exists today (crafts is in the queue_job/order schemas) but has
     NOT been exercised live — first live craft production should verify material binding
     (does the economic-stone restriction apply?) and then promote this section to
     experience-derived.
  3. Keep trade goods in the central stockpile (see `stockpile-architecture`) — accumulating
     value on the surface is a raid subsidy.
  4. TRADE ITSELF IS BLOCKED, precisely: there is no trade-depot build type in `build`'s
     vocabulary, and no caravan/broker/negotiation tools exist at all. Until those land, this
     skill produces and stores value; it cannot liquidate it.
  5. There is also NO wealth measurement: `stocks` carries counts and materials but no item
     value, and check_goals has no created-wealth predicate — a natural 009 workstream-4
     predicate candidate (the data exists in DF; the query doesn't).
  6. Verify: `stocks` (crafts/finished-goods counts rising), `jobs` (queue draining).
- **Tools**: queue_job (crafts, blocks, barrel), build (craftsdwarf workshop), stocks, jobs,
  stockpile.
- **Tiers**: EVENTUAL (trajectory — wealth for the first caravan), SOON (barrels feed the
  drink/food chain when unblocked).
- **Status**: production usable today (crafts path unverified live); trade blocked on **missing
  trade-depot build type + caravan tooling** (new named gap — not currently in goals.md);
  measurement blocked on **no wealth query/predicate**.
- **Worked example**: motivated by the surplus play actually created — 13 logs from one chop,
  quarry boulders, 7 beds (journal session 2) — and by the "first barrel" one-off queue_job
  use-case named in learnings "Production without a manager". Flagged honestly: no crafts have
  been made yet; per the verified-only rule this ships as a reference skill until a session
  proves it.
- **Scale test**: "convert surplus into dense storable value near storage" applies to any fort;
  what to make and from what is the model's read of its own surplus.

### 2.4 `bulk-production-upgrade`

- **Trigger**: use when repeated queue_job calls for the same item become routine — the manager/office upgrade path and what it's blocked on.
- **Procedure** [experience-derived]:
  1. Recognize the scaling smell: queue_job is one job per call by design; when the same item is
     queued again and again, the fort has outgrown direct queueing.
  2. The upgrade is the manager pipeline: `order` dispatches standing/bulk work orders — but ONLY
     once a Manager noble occupies an office. Orders issued before that validate and never
     dispatch (validated=true, active=false, forever) with no error.
  3. Everything physical for an office is buildable today: chair + table via queue_job, a
     door-partitioned room (see `portal-placement`). Build it as soon as furniture surplus
     allows — it's the cheapest standing prerequisite to bank.
  4. The blocking step is assignment: NO tool exists to appoint a noble or claim a room as an
     office. This is a distinct gap from the zone stub — both were discovered separately in live
     play.
  5. Until then: keep standing needs on a journal checklist and re-queue via queue_job at
     turn cadence; do NOT park work in `orders` where it rots invisibly.
  6. Verify: `orders` shows dispatch state honestly (validated/active) — read it after any
     order call rather than trusting the ACK alone.
- **Tools**: build, queue_job, order, orders, jobs, buildings.
- **Tiers**: SOON (office is a goals.md SOON item; unlocks bulk order flow).
- **Status**: office construction usable today; dispatch blocked on **noble-assignment +
  room-claim tooling** (journal session 2: "a NEW gap, distinct from the already-known
  zone/civzone stub").
- **Worked example**: journal session 2 "DISCOVERED: manager work orders... need a Manager noble
  + office... No tool exists to assign a noble or claim a room" — plus the mat_type=-1 sentinel
  bug found while probing it. The whole skill exists because `order` looked available and was a
  dead end.
- **Scale test**: the queue→manager transition point is population/throughput-relative — the
  skill names the smell, the model decides when it applies.

---

## Section 3 — Early Defense Preparedness (3 skills)

Defense at First Fort's stage is 100% architectural: there are no squad, military, burrow, lever,
mechanism, or drawbridge tools. What exists — and is partially proven — is the
walls/doors/hatches/chokepoint vocabulary plus the tripwire-guarded step. The goals.md EVENTUAL
"defensible single entrance" decomposes cleanly onto it.

### 3.1 `single-entrance-doctrine`

- **Trigger**: use once shelter exists and before the first migrant wave or threat — auditing surface openings and reducing them to one sealable chokepoint.
- **Procedure** [experience-derived audit; curated sealing tradeoffs]:
  1. AUDIT first: enumerate every fort-to-surface opening. `look` at the surface z across the
     fort's footprint; `cross_section` each shaft column to confirm where it daylights;
     `buildings` for surface structures. The audit, not the wall-building, is the actual skill —
     openings accrete silently as the fort grows (every new shaft top is a new hole).
  2. Reduce: every opening except one gets sealed with constructed walls (permanent, needs
     non-economic boulders — check `stocks` first) or covered. Order matters: seal extras BEFORE
     investing in the chosen entrance's defenses.
  3. The chosen entrance gets, in escalating order of investment: a door (producible + placeable
     today — pathing-blocks hostiles when locked, though building-destroyers can break doors), a
     hatch over the stair top (placement works; cover production BLOCKED — see status), and
     eventually a drawbridge+lever full seal (no tools exist yet).
  4. Tradeoff to hold onto (from the pre-play library, still sound): a fully sealed fort with one
     entrance can be starved by a long siege — keep a deep secondary exit reachable only from
     inside, sealed from outside. Where and whether is the model's call per site.
  5. Interaction with `stockpile-architecture`: the entrance choice and the central storage
     location should be decided together — the chokepoint protects the wealth room and the
     dormitories behind it.
  6. Verify: re-run the audit after sealing; `check_goals` no_active_hostiles stays the standing
     NOW predicate; a defensible-entrance predicate is a natural 009 workstream-4 addition
     (the brief already names "defense posture (entrance sealed?)").
- **Tools**: look, cross_section, buildings, build (wall/door/hatch), queue_job (door), stocks,
  step, check_goals.
- **Tiers**: EVENTUAL (defensible single entrance), NOW (no_active_hostiles maintenance).
- **Status**: audit + wall-sealing + door choke usable today. Hatch covers BLOCKED on the
  **hatch-cover job_type mapping** (goals SOON, explicitly HELD per overseer direction
  2026-07-12, superseded by the generalize-item-construction item); full mechanical seal blocked
  on **missing lever/mechanism/drawbridge build types**; garrison response blocked on **missing
  military/squad/burrow tooling entirely**.
- **Worked example**: goals SOON — "4 hatches placed at the main shaft's surface opening
  (46-47,50-51,140) are dead plans until this lands": the fort already ATTEMPTED exactly this
  doctrine and hit the named gap; learnings "Doors & hatches" — "hatch the top of every shaft
  that opens to the surface (defensible entrance is a core NOW/EVENTUAL goal)".
- **Scale test**: audit→reduce→harden is the same loop for a hovel with one shaft or a
  mountainhome with a trade gate; the number and nature of seals is site-driven.

### 3.2 `portal-placement`

- **Trigger**: use when placing any door or hatch — which goes where, the build-order constraint on wide entryways, and why partitioning precedes furnishing.
- **Procedure** [experience-derived — overseer-taught mid-session]:
  1. HATCHES go over stairwell openings (any tile where a stair meets open air/surface — e.g. a
     shaft top). DOORS go between two walls (corridor and room entrances) — never over a stair
     tile.
  2. Doors require an adjacent wall OR an already-built door on at least one side. For a wide
     multi-tile entryway, middle doors are REJECTED until a flanking door nearer the wall edge is
     built and connects — build OUTSIDE-IN, not all at once.
  3. Production chain: queue_job door at a carpenter/mason → item in stockpile → build door at
     the target tile. Verify placement via `buildings` (plans die silently — see
     `direct-production`).
  4. ORDERING LESSON: partition BEFORE furnishing. Furniture built first can make a wall
     un-retrofittable with a door without tearing the room up. When carving a room, decide its
     door tile at dig time.
  5. Priority: a single stairwell chokepoint is served by a hatch at top; doors matter more for
     interior partitioning (offices, bedrooms, storage) than for the shaft itself.
- **Tools**: build (door/hatch), queue_job (door), look, buildings, remove_building (for
  retrofit tear-ups), step.
- **Tiers**: SOON (office needs a door; interior partitioning), EVENTUAL (entrance hardening).
- **Status**: doors fully usable today; hatches placeable but covers unproducible (same
  **hatch-cover job_type mapping** gap as above).
- **Worked example**: learnings "Doors & hatches (overseer tip)" verbatim; goals SOON — the
  bedroom hall's west wall "can't be retrofitted with a door there without relocating 2
  already-built beds — minor design debt" — the live incident behind the partition-first rule.
- **Scale test**: placement mechanics and build ordering are invariant; how many portals and
  where is layout-dependent.

### 3.3 `crisis-posture`

- **Trigger**: use when a step tripwire fires or alerts show an active threat/flood — how the turn rhythm and the fort's plans change under crisis.
- **Procedure** [experience-derived template from the aquifer crisis; hostile-specific parts
  curated — the fort has met water, not war]:
  1. The tripwire is the entry point: a critical-severity announcement auto-repauses a running
     step and the report says why. Read it before anything else; `dismiss_alerts` only after
     acting.
  2. Drop to crisis pacing: short steps, verify after every one. Stability earns back long steps
     later (see `turn-discipline`).
  3. One structural project at a time: freeze expansion (`cancel_designation` on now-risky digs),
     redirect all effort at the crisis boundary. This is exactly the aquifer playbook
     generalized: contain (freeze descent) → ring (control the boundary) → seal → verify.
  4. [curated — unverified] For hostiles specifically, today's only levers are architectural:
     verify citizen positions via `dwarves` (underground = safer), seal the entrance (locked
     door/wall — see `single-entrance-doctrine`), and wait threats out on short steps. There are
     NO burrow/civilian-alert/squad tools — do not plan on mobilization, plan on the door.
  5. Aftermath: verify with `check_goals` (no_active_hostiles), dismiss handled alerts so the
     signal channel stays meaningful, and write the incident to learnings while it's fresh —
     crises are where skills are born (this proposal's best skill is a crisis record).
- **Tools**: step (tripwire reports), alerts, dismiss_alerts, look, cross_section, dwarves,
  cancel_designation, build, check_goals, pause.
- **Tiers**: NOW (no_active_hostiles), EVENTUAL (survive year 1 — zero deaths).
- **Status**: tripwires + pacing + sealing usable today; any active-defense response blocked on
  **missing military/burrow tooling** (no goals.md entry yet — worth filing when a threat first
  materializes).
- **Worked example**: the aquifer arc as crisis template (journal session 1: freeze descent,
  ring, seal on 400–800-tick steps); decisions 2026-07-12 — step tripwires pulled forward and
  landed because multi-day steps needed to be crisis-safe.
- **Scale test**: contain→boundary→seal→verify and pacing-by-stability apply to floods, sieges,
  and tantrums alike, at any population.

---

## Conversion lineage — the 11 builtins vs this proposal

Workstream 3 says "convert the 11 Go skills". This table is the honest version of that task —
what converts, what must be rewritten, what waits:

| Builtin (`internal/skill/builtins.go`) | Fate | Lands in |
|---|---|---|
| `carve_initial_shelter` | convert; strip depth numbers, retarget dead `region_detail`→survey/cross_section | 1.2 fresh-embark-opening |
| `dig_safe_stairwell` | REWRITE — its single-column-only advice is stale (designate_dig does 2×2-in-one-call + continuation now); keep corner-vs-center tip | 1.2 + 1.3 designation-hygiene |
| `build_bedroom` | DEFER — blocked on zone civzone stub; strip all room dimensions when it converts; keep partition-before-furnish (already extracted) | future skill post-workstream-1; 3.2 carries its live lesson |
| `setup_workshop_chain` | convert; strip "minimum 5×5" | 2.2 resource-flows + 1.6 direct-production |
| `setup_stockpile` | REWRITE — its "many small piles beats one giant one" default is CONTRADICTED by the overseer-taught centralize-first doctrine; keep the adjacency throughput point as the later specialization stage | 2.1 stockpile-architecture |
| `produce_furniture` | REWRITE — manager-first doctrine inverted by live play (order is a dead end on a fresh fort; queue_job is the path); strip item counts | 1.6 direct-production + 2.4 bulk-production-upgrade |
| `establish_food_supply` | REWRITE — farm-centric plan is blocked (zone stub, no farm-plot build type); live-verified chains are gather/fish/cook; strip day-number deadlines and farm dimensions | 1.5 provisioning |
| `create_dining_hall` | DEFER — blocked on zone civzone stub; strip "10×10+/20×20" | future skill; goals SOON already tracks it |
| `secure_entrance` | convert; drawbridge step becomes a named-gap flag; keep the starvation-under-siege tradeoff | 3.1 single-entrance-doctrine |
| `secure_water_access` | DEFER — wells unbuildable (no well/mechanism build types); the live-relevant fragment (the sealed wet column as future cistern) is in 1.4 and goals EVENTUAL | future skill |
| `handle_aquifer_layer` | REPLACE OUTRIGHT — smooth-first protocol falsified on sand aquifers; the field-verified wall-ring protocol supersedes it | 1.4 aquifer-piercing |

Two builtins survive roughly intact; five need rewrites where live play falsified them; three
defer behind named gaps; one is replaced outright. **A transcription pass would preserve five
known-wrong doctrines** — the conversion must be done against learnings.md, with learnings.md
winning every conflict.

## Engineering gap register (every blocked edge above, precisely named)

Already filed (goals.md SOON / decisions 2026-07-12): zone civzone stub (bedrooms, dining,
farming, meeting — 009 workstream 1); noble/manager assignment + room claim (distinct from the
zone stub); BrewDrink job_type mapping (reaction-based lookup needed); hatch-cover job_type
mapping (HELD; superseded by generalize-item-construction); set_labor (field-confirmed critical);
dwarf-vs-animal census filter (also corrupts check_goals per-dwarf math); find_dig_site
stale-solidity; stocks category case-sensitivity; designation+building overlays in `look`
(two live incidents).

Newly named by this proposal (not yet in goals.md): trade-depot build type + caravan/broker
tooling (blocks liquidating wealth — 2.3); created-wealth query/predicate (blocks measuring it —
2.3, fits workstream 4); lever/mechanism/drawbridge build types (blocks full mechanical seal —
3.1); military/squad/burrow tooling (blocks any active defense — 3.3); farm-plot build type
(blocks farming independently of the zone stub — 1.5).

## Fit to 009 success criteria

- Criterion 3 ("model authors ≥3 skills from experience") is already retroactively met in
  substance: 1.4 aquifer-piercing, 1.6 direct-production, and 3.2 portal-placement are pure
  distillations of session 1–2 experience — authoring them as SKILL.md files is the mechanical
  step. The learning loop's job is to keep doing what learnings.md already does, in skill form.
- Criterion 5 (learnings survive compaction and change behavior) is strengthened by moving
  stable, procedural learnings into skills (loaded on demand, no compaction exposure) while
  learnings.md keeps fort-specific and still-settling observations.
- The skill-linter open question in the brief gets a concrete spec from this document: ban
  numerals attached to room/stockpile dimensions, material names, absolute coordinates, item
  counts, and day deadlines; allow the sanctioned mechanical invariants (2×2 stair law,
  orthogonal ring, 3×3 workshop footprint, tick/day conversion); require a final verify step
  naming a real tool; require either a worked-example citation or an explicit
  [curated reference — unverified] marker.
