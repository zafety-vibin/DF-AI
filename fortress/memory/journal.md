# Fort Journal

(newest entries on top)

## 2026-07-18 — Fort #5, session 2 (Autumn y100, day 178 → day 232, ongoing)

LIVE-VERIFICATION SESSION for all three 2026-07-17/18 fix waves, then
straight back into play. Overseer co-played in the client throughout
(manual trade, manual work-detail fixes, one manual rescue). Fort grew
9 → 17 living citizens (+8 wave, day ~206). DÖBAR IS BURIED.

**Verification: every headline fix confirmed live.** `[DEAD]` tag caught
döbar first try; `'u'` pending-building glyph painted the 4 queued wall
segments in plain `look` (and built ones correctly flipped to `#`);
mid-dig `'d'` persisted through claim-and-dig; `mandates`/`moods`/
`noble_demands`/`fort_wealth`/`wellbeing`/stocks-quality all work
(punishment populates at ISSUE time — the open question is answered);
`ConstructCoffin` accepted at Masons; `build` reshape fully proven:
curated+material byte (craftsdwarf [stone], tradedepot [wood], walls
[blocks]), quality-tier placement ("placed existing COFFIN quality
WellCrafted material jet" — names the item it picked), and UNCURATED
name-passthrough (`Statue` resolved and placed). `set_workshop_profile`
min-skill gate applied to the carpenter (dabbler-migrant protection).
ONE regression: `zone_value` fails "no zone at coordinates" on every
real zone tested — the only wave-2 tool that failed live.

**Döbar's burial (the session's heart):** mason ConstructCoffin x2 →
WellCrafted jet coffin → dug a cross-shaped MAUSOLEUM west of the guard
room at z=126 (5x5 chamber, 3-wide alcove wings N/S, morion gems struck
in its walls) → tomb zone over the north alcove → coffin placed by
quality → `assign_zone` accepted the DEAD unit → bridge raised to
uncover the pit ramps (the lowered bridge is a LID — that's the real
reason she couldn't leave) → "found dead, dehydrated" → carried up and
entombed (overseer confirmed in client; our unit record still shows her
at the pit coords — burial state is invisible to tooling, logged).
Claystone statue placed in the chamber via the passthrough. Queen
grieved (cat 5/6 held).

**Bridge design flaw, now understood as a SYSTEM:** the pit under the
raise-bridge collects items → haulers descend whenever the deck is open
→ every close risks entombing someone. It trapped 2 more dwarves this
session (overseer freed them by excavating a pit exit + flooring it
over) and "comically reliably" traps the lever-puller. RULES BANKED:
census the pit (z=125) AND deck before any close; the puller must stay
on the lever side; overseer's bypass was walled back up with 7 block
walls (49-55,42,126) after use. Lever stays EAST-only by design (east =
inner fort; a west lever would hand bridge control to besiegers — my
proposal to add one was wrong and was withdrawn).

**Work-details discovery (project-significant):** fort owned TWO picks
all along; `set_labor` writes `unit.status.labors` which v50 treats as
a DERIVED CACHE of the work-details layer. Overseer manually assigned
miner #2 + 2 woodcutters in the client → wood went 2 → 96 logs (the
"no axe" theory was wrong — woodcutting was detail-gated exactly like
mining). Full root-cause research report (Sonnet agent, source-cited):
authority is `plotinfo->labor_info.work_details`; fix = edit
`assigned_units` + call `Units::setAutomaticProfessions`; 4-tool
surface proposed. See decisions.md 2026-07-18 + cross-session memory.

**Economy/survival arc:** drink hit 5-for-9 with ZERO plants and no
farm — built the fort's FIRST farm plot (2x5, z=133 soil, plump
helmets all seasons; producing by day ~217), brewing re-established.
Autumn caravan came with NO tools to buy; overseer traded manually
(bought plump helmets, sold spare chairs/cabinet); wagon deconstructed
→ 5x5 tradedepot [wood] on the surface. Food crisis at 17 mouths →
113-shrub gather sweep + FISHERY + KITCHEN built on the surface;
manager work orders (olon appointed by overseer + office we built at
(46-49,34-38,133): throne, table, zone, assign) VERIFIED: PrepareMeal
and ConstructBlocks orders dispatched and completed. `order` accepts
ANY job_type (manager fills reagents) — it made a FIGURINE, routing
around queue_job's crafts-class rejection. (Figurine did NOT credit the
queen's make-mandate though — reader verified correct vs client; DF
crediting subtlety, open question.) Queen's mandate churn all session:
figurine 3/3 expired unfulfilled (no justice system = no beating),
coffin-make fulfilled by the burial coffins, anvil make/export + fig
make/export bans active — ANVIL MAKE (~25k ticks) is the live clock.

**Housing:** Row-3 pods furnished + assigned (olon/zulban/methkat, rock
doors, pass-through doors at (37,29)/(37,39) placed — overseer confirmed
those gaps were intentional symmetry). EAST WING under way at z=125
(siege-resilient housing per overseer direction): 2x2 stairs off the
checkpoint corridor — first shaft placement FAILED into the mechanic
workshop's invisible footprint ("Inappropriate dig square" root cause:
buildings block digs and terrain view doesn't show them; same
explanation as the old (40,40,133) cabinet mystery) — relocated to
(61-62,43-44); 3-wide hall (56-73,43-45) + 8 3x3 pods, ~5 dug so far.
STRUCK LIMONITE (real iron ore, (63,38,125)) and saltpeter in the pod
walls — first actual ore of the fort. 6 beds queued, 8 rock doors
queued, blocks work order producing.

**Session decision (day 232):** overseer left continue-vs-engineering
to me. Chose to CONTINUE PLAYING the iron arc (wood_furnace + smelter +
metalsmith, dig limonite, charcoal → smelt → forge the queen's anvil)
— it's the most alive thread and it live-verifies the one untested
wave-2 surface (furnace jobs). Engineering wave (work-details + trade
tooling headliners; full gap list in goals.md Tool state) starts at the
next natural break.

**Iron arc outcome (day 232-256):** industry quarter fully built at
z=125 (wood_furnace/smelter/metalsmith ringed in limonite+lignite
veins), 7 ore + coal banked, charcoal running — and the arc ended
TOOLING-BLOCKED one job short: SmeltOre unreachable (unwired in
queue_job; order path failed silently — the session's biggest finding:
manager-order "completed" alerts can be PHANTOM, see goals.md order
trust boundary). Make-anvil mandate expired mid-attempt, no observed
consequence. Fort entered winter fed (18 real meals), 17 citizens,
east housing wing 6/8 pods + doors/beds queued, stockpile-hall east
expansion digging. DF saved+closed by overseer at ~day 256.

**WAVE 4 shipped same night (2026-07-19): see goals.md Tool state for
the full list** — work-details labor tools, the orders material-pinning
root-cause fix + SmeltOre direct wiring (the iron arc unblock), trade
safe-subset (caravan_status/depot_goods/bring_goods_to_depot; the
trade COMMIT itself honestly remains human-only), zone_value fix,
lens=minerals, and the whole perception paper-cut list. 20 agents,
survived 2 more mid-run disconnects (resume + prompt-amended stitch of
two dead agents' partial work — both had left MORE correct work in the
tree than their reviews could see). All gates green. DLL (564KB)
DEPLOYED to Steam hack/plugins with DF closed. Next session opens on
live-verify: assign atér to Mining via work_details tooling, then
finally smelt that iron.

## 2026-07-17 — Fort #5, session 1 (Spring y100, day 14 → Summer day 147, ongoing)

FRESH EMBARK, human co-playing live in the DF client alongside MCP tool calls
(first session with real-time human course-correction, not just post-hoc
review). Site: 96x96, surface z=134, soil aquifer z=131-130, DEEP STONE
aquifer z=123-121 (only 5 clean levels between them, z=124-128) — tighter
double-aquifer sandwich than any prior fort. Second miner (atér, masonry)
labor-enabled pre-emptively per fort-opening's Gate 1, before first dig.

**Aquifer: BOTH layers pierced and sealed same-session**, standard
protocol (shaft to one-above, pierce+re-designate through silent cancels,
quarry-before-ring, orthogonal ring, wall-in-shallow-water) — zero
deviations from the skill. First REAL bedrooms this project has opened
with (walls+door+cabinet, corridor-branch, not 1x1 zones) built cleanly
on the first attempt: 3 pods off a west corridor, doors placed as soon as
carpenter output allowed. User confirmed live: "great job on creating
real bedrooms... this is proper fort design basics."

**Brewing worked on the second try**: first BREW_DRINK_FROM_PLANT batch
died silently (dedup swallowed the "needs empty food storage item" cancel
after the first alert) — starting barrels were apparently already full of
embark cargo. Fix: `queue_job item=barrel` for fresh ones, re-queue the
reaction. Recurred once more mid-session (ran through the fresh barrels)
same fix. Pattern now well-established across 3+ forts.

**NEW GROUND: first defensible checkpoint architecture, built at user's
direct request** mid-session ("below the aquifer layer we should create
a checkpoint... possibly with a drawbridge to hole up within"). Shaft
continued 2 levels past the aquifer seal into the dry stone band (z=126);
guard room carved directly around the landing (first thing anyone
descending the stairs reaches); corridor east to a moat — **user
explicitly asked to widen it from a cautious 1x3 slot to a real 4x3
moat** mid-designation, redesignated cleanly since only 1 of 3 tiles had
dug. Bridge built (`build type=bridge width=4 height=3`) — **first
attempt was misaligned by exactly one tile** (center x=52 mapped to
footprint x50-53, not the actual x51-54 gap, leaving one gap tile
uncovered); caught via `look`, fixed with `remove_building` +
rebuild at center x=53 (instant, no deconstruction wait since still
unbuilt). Mechanic workshop + lever built on the protected (east) side
per design intent (operate from inside, not from the exposed side).
`link_building` needed 2 free mechanisms, not 1 (the lever build itself
consumes one) — queued a 3rd, link succeeded. `pull_lever` raise/lower
both completed cleanly (empty job queue = success, per the established
no-visual-feedback pattern).

**LIVE INCIDENT: raising the bridge locked every dwarf out of the lever
that controls it.** All 7 (at the time) dwarves lived on the guard-room
side; the ONLY path to the lever was across the bridge itself. Once
raised, the `PullLever` "lower" job sat queued forever — unreachable, no
error, dwarves just went idle. User diagnosed it correctly in real time
("raising the bridge may have cut off access... the lever is on the
other side"). **Root lesson: a single-bridge chokepoint with no
independent access to its own lever is a self-lock, not a chokepoint** —
recovery required digging an emergency bypass tunnel (hit an
"Inappropriate dig square" cancel on the first thin-corridor attempt;
widening to a full room block resolved it, cause not fully diagnosed).
Once the bypass reached the lever, the queued job finally executed.
**Then deliberately walled the bypass shut** (14-tile wall across the
guard room's south side) to restore the chokepoint's actual purpose —
user's idea, also gave 3 idle non-miner dwarves something to do. Lesson
banked in learnings.md: any lever controlling the only crossing needs
either a second permanent access path that never crosses the bridge, or
acceptance that raising it stops being reversible from the outside.

**Large underground stockpile** (12x14, `stockpile category=all`) placed
in a big hall dug beyond the checkpoint, safely in the dry band between
both aquifers (z=126) — user's explicit ask ("large stockpile room...
below the aquifer layer"). Single-miner-equivalent pace (see below) made
this the session's slowest dig by far (~200 tiles).

**Second-miner-in-name-only, root-caused this session**: atér has had
MINE labor enabled since before the first designation, but dwarf census
after the fact shows her idle throughout the big-hall dig while logem
(sole `PickupEquipment` recipient at embark) did visibly all the mining
(MINING Lvl5→Lvl9 over the session). Root cause: DF requires a physical
pick, not just the labor flag — embark almost certainly shipped exactly
one. `stocks category=pick` confirmed **zero free picks anywhere**.
Checked the smelt→forge→pick path at user's request: `SmeltOre` and
`MakeWeapon` job_types both exist and are real, `build type=metalsmith`
is available (anvil already in stock) — but **no ore has been struck
yet** (only coal/jet/jade/tiger iron, all fuel or decorative), AND
`queue_job`'s `item` param has no way to specify a weapon SUBTYPE (pick
vs. sword vs axe) — a real tooling gap, not just a resource wait. Flagged
for engineering; second-miner-via-forged-pick stays unverified.

**Migrant wave, day 143**: 7→10. Two strong pickups (olon Leatherwork
Lvl13, zulban Engrave Stone Lvl10), one social skill (methkat Judging
Intent Lvl4). None arrived with mining skill.

**Rare event: one of our OWN starting 7 became civ monarch** (Monom,
PERSUASION/NEGOTIATION/CONVERSATION skill profile — plausibly not
coincidental). Confirmed via alert + dwarf_detail; she still lives in an
ordinary 4x3 pod for now, a real noble suite is future work. Also got an
expedition-leader assignment (oddom) shortly after — both are the
civ/fort's own automatic early-noble succession, not anything we
triggered.

**Tooling gap flagged by user — TWO related but distinct bugs, BOTH now
fixed** (Bug B via the workflow, Bug A by hand afterward at the user's
explicit follow-up ask — "that's important for your understanding of
what's going away vs is staying"). Bug B (FIXED, compile-verified NOT
live-verified, workflow wf_6ea04eea-68b): a pending BUILDING construction
(e.g. `build type=wall` before the job completes) was completely
invisible in a plain `look` call without `lens=buildings` — the direct
cause of this session's repeated wall-breach confusion. Fix: DF's
`tile_occupancy.bits.building == Planned` now paints an always-on `'u'`
glyph, same treatment as `'d'` for dig designations.

Bug A (FIXED by hand, root-caused via source, compile-verified NOT
live-verified): `look`/`map_slice` only read the raw `des.bits.dig` tile
designation bit — but DF CLEARS that bit the instant a unit CLAIMS the
dig job, well before the tile is actually dug. A tile a miner is actively
walking to or working therefore rendered as plain undesignated rock,
genuinely indistinguishable from "nobody has ever touched this" — which
is exactly the "already visible wall, not showing 'd'" case, not a
render-priority bug at all (render.go's paint order was already correct;
'd' unconditionally wins over any base terrain glyph once a tile is IN
`s.Designated` — the C++ side just wasn't putting mid-dig tiles in that
set to begin with). The fix already existed elsewhere in this exact
codebase and was never adopted here: `tile_extractor.cpp`'s
`collect_dig_job_targets()` (used by the entity/topology wire path,
comment there literally documents this same DF behavior) builds the set
of tiles with an in-flight dig job; `queryMapSlice` in `queries.cpp` now
ORs that set into its designated-tile check too, mirroring
`compute_tile_flags`'s identical `FLAG_DESIGNATED` treatment. Zero Go/
protocol changes needed — same JSON field, just correctly populated now.
Rebuilt clean (`queries.cpp` zero warnings, fresh `.plug.dll`, confirmed
mtime), `go build`/`go test` independently re-confirmed unaffected (264
tests, no protocol touched).

**DEATH: döbar (Planter) starved to death in the checkpoint moat pit,
day ~153.** The exact standing risk flagged above (lever's only access
path crosses the bridge it controls) went from hypothetical to real —
she was apparently in the pit at z=125 (one level below the moat) when
the bridge was raised/lowered during the earlier lockout-and-recovery,
got stranded despite visible ramps out (`look` showed '^' ramp tiles on
all sides), and simply never had a job/reason to path out before she
starved. `dwarves`/`dwarf_detail` gave NO indication of death — same
position, "idle", no distress flag, position unchanged across multiple
polls — the "missing for a week" alert was the only signal; user caught
the actual death by watching the live client, tooling never surfaced it.
Real gap: dwarf_detail should flag dead/missing status, not silently
report a stale-but-plausible-looking idle record.

**Follow-up: tried to properly entomb her, testing tomb/coffin tooling
live (user's suggestion — "good testing scenario").** `designate_zone
type=tomb` works cleanly. `ConstructCoffin` is a real DFHack job_type but
REJECTED at both Carpenter's and Mason's ("job type not supported at
this workshop type") — a fresh instance of the exact same bug class as
the ConstructFloodgate gap fixed earlier this project (missing from
`jobTypeAllowedAtWorkshop`'s whitelist). Full chain blocked: can zone a
tomb, can't manufacture anything to put in it. Logged in goals.md for
the next engineering pass. Proper burial deferred until the fix lands.

**Session close (day 178, PAUSED, DF closed by overseer):** finished the
3rd bedroom pod row (öton/doren/oddom, dig complete, furniture built) —
6→9 real bedroom pods total, has_bedroom_zones_7 MET. Dining hall zoned
and furnished (2 tables, chairs queued) in the corridor between pod rows
1+2 — has_dining_hall MET. Fixed two live self-inflicted architecture
bugs mid-session, both caught by the overseer watching the actual DF
client (our own tools couldn't see either): (1) the dining-hall-to-Row1
connector landed one tile off, punching straight into atér's bedroom
instead of a neutral corridor — left as a door-gated pass-through rather
than a full reroute, overseer's call; (2) the Row3-to-Row2 connector dig
was designated one tile too wide (y=29 instead of y=28), shearing off
Row2's ENTIRE north wall in one pass — same failure shape as the
guard-room breach earlier this session, same fix (rebuild the wall,
this time leaving zero unintended openings after 3 rounds of "wait,
there's still a gap here" corrections). Root cause of the repeated
back-and-forth, per the overseer: `look` shows a pending
Construction-type building (e.g. a queued `build type=wall` not yet
complete) as an indistinguishable plain wall glyph unless
`lens=buildings` is explicitly requested — there's no way to tell "solid
rock, never touched" apart from "floor, wall queued but not finished"
from a plain `look` call, which is exactly backwards from what's
decision-relevant. Queen's construction mandate also went unfulfilled
past its deadline (screenshot showed "...tion mandate near deadline" in
the DFHack log strip) — we have no tooling to see what it actually
required, so no way to act on it deliberately.

**OVERSEER OFFERED A CHOICE at this point: keep playing (e.g. a manager's
office) or pivot to fixing what this session surfaced.** Chose the
latter — the glyph-priority bug was actively degrading play (three
separate rounds of the same "wait, that's still open" correction), and
we'd already source-confirmed two other real gaps (mandate inspection
missing entirely, dwarf death/missing status invisible to
dwarves/dwarf_detail). Fixed `ConstructCoffin` in the whitelist directly
(trivial, same shape as the already-shipped `ConstructFloodgate` fix,
no research needed). Dispatched a 3-lane research→implement→verify
workflow for the other three (glyph-priority is Go-only, safe to
implement+test live even with DF closed; mandate inspection and
dwarf-death-status both need real DFHack struct research from the
sibling source checkout before implementing — instructed to stop short
of a risky implementation and hand back a scoped plan if the DF data
model turns out more tangled than expected, rather than force something
that might silently misreport). **Also by-hand fixed a second, genuinely
distinct glyph bug** the user caught live (`Bug A`): a mid-dig wall
tile's `des.bits.dig` designation bit goes cold the instant a unit
CLAIMS the job (well before the tile is dug), so `map_slice` never put
such tiles in the always-on designated set at all — not a render-
priority bug (render.go already lets 'd' win unconditionally), a data-
population bug. Fix reused `collect_dig_job_targets()`, which already
existed in this codebase for the entity/topology wire path and had just
never been adopted by `map_slice`.

**SECOND MAJOR RESEARCH+IMPLEMENTATION WAVE, same session, day 178
onward**: user asked for deep research into what DFHack system to
integrate next for mid/late-game management (explicitly excluding
noble-professional appointment, which stays deferred). Four parallel
Fable research agents (moods/personality/stress, quality/wealth,
justice/nobility-overview, and a proactive gap-audit of the plugin's own
hand-maintained whitelists) all landed with deep, source-grounded
reports — full detail in Claude Code cross-session memory
(`project_fort5_session1_checkpoint_and_fixes.md`). Two independent
agents converged on the same live bug from different angles: `build`'s
`material` parameter (wood/stone/blocks) has been a **silent no-op**
since it was added — Go encodes the byte and the ACK text even claims
`[stone]` succeeded, but the C++ side never reads past byte 11. The gap-
audit agent additionally found the `ConstructCoffin`/`ConstructFloodgate`
whitelist gap was a PATTERN, not a one-off: 12 more legitimate furniture
job types were missing the same way, plus a structural bug blocking
**every furnace job/reaction project-wide** (charcoal, smelting) since
`applyQueueJob`/`applyQueueReactionJob` only ever accepted workshop
buildings, never furnaces.

Dispatched a 14-agent implementation workflow (audit fixes + wealth/
quality system + moods/psychology system; justice and noble appointment
explicitly held back per user instruction until testable) — see the
Claude Code memory file for the complete shipped-feature list. Headline
results: the material-byte bug is fixed for real; furnace jobs/reactions
now reachable (charcoal/ash confirmed reachable via the simple filter
path — `SmeltOre`/`MeltMetalObject` deliberately NOT force-fit, they need
per-job material pinning the current filter shape can't express, flagged
honestly rather than faked); `build` gained a `quality` parameter that
can target one specific existing item by quality tier (the "masterwork
bed in the queen's room" capability); a `set_workshop_profile` action;
new `moods`, `noble_demands`, `fort_wealth`, `zone_value`, and
`wellbeing` queries; `stocks`/`mandates`/`dwarf_detail` all gained new
dimensions (quality breakdown, resolved item-subtype names, a full
`psyche` section). Both consolidated build checkpoints and a final
review pass all passed clean (0 compiler warnings) — two agents hit
transient "stream idle timeout" disconnects (once from the user's own
laptop closing) and were cleanly resumed from cached workflow state
rather than restarted from scratch. Two minor, non-blocking findings
from final review: `renderMoods`/`renderNobleDemands` have no unit tests
(every sibling new-tool render function in this wave got one — a real
gap in an otherwise-consistent pattern); `set_workshop_profile`'s
`worker_unit_id` uses a bare `int` with an `if > 0` presence check
instead of this codebase's established `*int` pattern (`tools_defense.go`)
for "optional single unit id," meaning unit id 0 specifically could
never be targeted (vanishingly unlikely to matter, real fort unit ids
essentially never land on exactly 0, but inconsistent with precedent).
**Everything from this whole wave is compile-verified only — DF stayed
closed the entire time.** Next session's first real task, alongside
döbar's burial: live-verify every one of these in order, same discipline
as prior sessions' fix-waves.

A FOLLOW-UP Fable agent researched the tool-schema scaling problem the
user flagged (tool descriptions risk exploding as DF coverage grows,
loaded into every session's context regardless of relevance) — briefed
on this wave's new tool surface as settled fact. Landed with real
measured numbers (58 tools, ~38KB/~10k tokens today; `build` is the
single worst offender) and a concrete recommendation: the `queue_job`/
`job_types` pattern already in this codebase (curated short vocabulary +
full-generality name-passthrough + a separate on-demand discovery tool)
is the right template, and `build`'s `type` parameter should be reshaped
into that same three-part shape before the audit's ~30 still-deferred
building/trap/furnace/workshop types land on it and make the problem
materially worse. Also recommended a new CLAUDE.md house rule (tool
schemas carry shape, never guidance — mirroring the existing skills
rule), and found `fort-planning`'s defense-staging skill is now stale
(claims no bridge/lever/mechanism tooling exists — shipped and verified
this very session).

**THIRD MAJOR WAVE, same session (2026-07-18): the build-tool reshape +
near-total building/workshop/furnace/trap coverage.** User's framing:
"opening the oyster so we can see the pearl completely" — go broad, not
minimal. 17-agent workflow (survived TWO mid-run disconnects — a
transient stream timeout and a wifi drop that lost the local session's
tracking entirely — both cleanly resumed via `resumeFromRunId` with zero
rework lost, cached agents replayed instantly). Shipped:

- **`build`'s type-selection architecture reshaped** to the exact
  `queue_job`/`job_types` three-part pattern: the ~29 existing curated
  names keep their old wire bytes unchanged; a new name-passthrough
  (`BUILD_TYPE_BY_NAME`, mirrors `ORDER_TYPE_BY_NAME`) resolves ANY name
  plugin-side via `find_enum_item` tried against `building_type` →
  `workshop_type` → `furnace_type` → `trap_type` in turn
  (`resolveBuildTypeByName`), then a second function
  (`resolveCuratedBuildTypeByte`) checks whether a real placement recipe
  exists for that resolved type and falls through to the SAME verified
  placer functions the curated path already uses — no parallel
  implementation. A new `building_types` discovery tool (58 entries: 30
  original wave-2 baseline extended to includes all of this wave's
  additions) replaced `build`'s ~1400-char prerequisite-prose
  description with per-value facts, on demand, matching `job_types`
  exactly. `build`'s own description is now a 4-sentence pointer.
- **~30+ new building types actually placeable now**: Well (+ MakeChain,
  the deliberate justice-adjacent EXCEPTION — chain is genuine
  construction material for Well/TractionBench/Rollers, not jail-
  specific, reasoned through explicitly not just assumed), Support, the
  full room-value furniture family (Statue/Slab/WindowGlass/WindowGem/
  Bookcase/DisplayFurniture/OfferingPlace/Instrument), water/power
  infrastructure (ScrewPump/GearAssembly/AxleHorizontal/AxleVertical/
  WaterWheel/Windmill/Rollers), ArcheryTarget/TractionBench/NestBox/Hive,
  4 trap subtypes (PressurePlate/StoneFallTrap/WeaponTrap/TrackStop —
  CageTrap deliberately excluded, its ammo is the excluded MakeCage
  item, would repeat the exact "craftable but unarmable" bug this
  project already fixed once with coffins), new furnace types (Kiln/
  GlassFurnace/magma variants/MagmaForge), and 10 new workshop types
  (Jewelers/Bowyers/Siege/Leatherworks/Tanners/Clothiers/Loom/Kennels/
  Ashery/Dyers) — closing "the entire cloth/leather industry is absent"
  gap from wave 2's audit. Every known limitation is DOCUMENTED, not
  hidden: magma buildings can't verify magma-adjacency at placement
  time (DFHack has no check); stone_fall_trap builds unarmed (no "Load
  Stone Trap" job exposed yet); pressure_plate builds with trigger
  detection off; axle/rollers are single-tile only (no length param on
  the wire); several furniture types need a pre-existing tool item this
  project can't craft with the right subtype yet (a separate, pre-
  existing gap); Tool/Custom workshops stay uncurated (no universal
  DFHack recipe exists, matches DF's own build-menu gating).
- **Job wiring**: MakeChain, ConstructTractionBench (bespoke 3-item
  TABLE+TRAPPARTS+CHAIN filter), plus baseline queue_job reachability for
  the new workshops (ForgeAnvil, CutGems, EncrustWithGems, PrepareRawFish,
  ExtractFromRawFish, WeaveCloth, CollectWebs, DyeThread, DyeCloth,
  MakeBackpack, MakeQuiver, MakeCharcoal, MakeAsh).
- **Hygiene**: the new CLAUDE.md house rule shipped for real ("Tool
  schemas carry shape, never guidance" — sibling to the skills rule),
  fort-planning/fort-opening skills refreshed to reflect real
  capabilities (procedural-status only, no new dimensions), a REAL CI
  test (`TestToolSchemaBudget`, runs a live in-memory MCP session, not a
  throwaway probe) now enforces the budget going forward — measured 65
  tools / 43,666 bytes total against a 60KB ceiling, `build` itself down
  to 2,523 bytes, `look` given a named exception (real cost-table
  complexity, not enum bloat). Two small loose ends from the PRIOR
  wave's review also closed: `renderMoods`/`renderNobleDemands` unit
  tests added, `set_workshop_profile`'s `worker_unit_id` fixed to the
  established `*int` pattern.
- Final review: full PASS, C++ rebuilt with /W3 /WX (warnings as
  errors) for a genuinely clean-not-just-passing signal, all 71
  BUILD_TYPE_* constants verified byte-identical between protocol.h and
  message.go, scope boundary grepped clean (zero Cage/Chain/MakeCage/
  CageTrap implementations, only exclusion-documenting comments).
  Honest non-blocking observation: the legacy `build` type enum itself
  still lists ~32 names (reformatted, not shrunk this wave — deliberate,
  matches what was asked) — trimming it toward the ~15 guideline is a
  clean follow-up, not a defect.
**Everything in this third wave is ALSO compile-verified only — DF
stayed closed the entire session.** Next session's live-verification
queue is now three waves deep; do it in order (glyph/mandate/dead-status
fixes first, since they're foundational to observing everything else
correctly).

Session ends at day 178, paused, 0 hostiles, drink/food stable, 9 living
dwarves (döbar dead, unburied — coffin fix now shipped but not yet
deployed to a running DF; entombment is next session's task once DLL is
redeployed). NEXT: deploy the fix-wave DLL (needs DF closed, which it
currently is — good opportunity), verify ConstructCoffin live and
properly bury döbar, check the tooling-fixes workflow's output and
integrate whatever it produced, scout a different elevation for actual
metal ore (deep-stone band here proved ore-free), manager's office
(mentioned but not started), second miner still blocked on pick supply.

**WORKFLOW RESULT (post-session, DF closed): all 3 dispatched gaps came
back IMPLEMENTED, not just designed** — the research agents judged all
three shapes clean enough to implement directly rather than stopping at a
plan (they were explicitly told they could, and to stop short if the DF
data model turned out too tangled — none did). All confirmed against the
real DFHack 53.15-r2 source in the sibling checkout, not guessed:
- Mandate inspection: new `mandates` MCP tool, zero wire-protocol changes
  needed (the plugin's query dispatch was already generic string+JSON).
- Dwarf death status: `dwarves`/`dwarf_detail` now carry `[DEAD]`/`dead`
  via `Units::isDead()`; required one new additive wire block
  (`DeadUnits`, same backward-compatible pattern as the existing `Zones`
  block) — found and fixed a real latent test-encoder bug along the way
  (asymmetric `HasZones` byte that would've desynced anything appended
  after Zones, never hit in production since Zones was always last until
  now). Logged as its own `docs/decisions.md` entry.
- Glyph-priority (Bug B specifically, see above — NOT Bug A): pending
  buildings now paint an always-on `'u'` glyph in plain `look`, sourced
  from `tile_occupancy.bits.building == Planned`.
`go build ./...` + `go test ./...` independently re-confirmed clean
afterward (264 tests, 34 packages, 0 failures) — verified myself, not
just trusted the workflow's own report. C++ side is compile-verified only
(DF was closed all workflow); every plugin-side change (coffin, mandates,
dead-status, pending-building glyph) still needs an actual live pass next
session before being trusted operationally. Full detail:
`docs/decisions.md`'s new entry (dead-status) and this session's
`goals.md` Tool State section (all four).

## 2026-07-17 — Fort #4, session 3: tool verification (Summer y100, day 121→140)

Narrow-scope session: live-verify the 4 fixes from the 2026-07-16 wave
plus a floor-item check, then (added mid-session by the overseer) prove
out the mechanism/lever/bridge chain end-to-end. Plugin redeploy
confirmed live against DFHack 53.15-r2. All 5 original checks VERIFIED;
full detail in goals.md's Tool State section. Two things worth
remembering: (1) the `remove_building` disambiguation test destroyed the
fort's actual Still (stockpile+workshop overlap tile resolved cleanly to
the workshop, no ambiguity fired) — rebuilt immediately, but ~10 days of
drink production were lost mid-test; (2) built a small standalone
mechanism test rig (mechanic workshop + lever + 3x1 bridge over a
purpose-dug channel) at (36-38,40-42,119) — a flat-ground bridge attempt
failed cleanly first (bridges need a real gap), the channel fixed that,
and lever→link→pull worked cleanly both directions. This rig is real
infrastructure, not scaffolding — left standing for a future session to
extend toward the real brook crossing near the west bend (51-58,42-47,
118), which needs a dug corridor through ~7 tiles of soil and was out of
scope here. See [[project_fort4_session2_tool_bugs]] for the bugs this
session closed out.

Follow-up (still same session, overseer asked "what else can levers
control besides bridges?"): checked the protocol source directly —
`link_building` supports exactly 4 target types (bridge/door/hatch/
floodgate), nothing else exists (no well, no traps, no restraints).
Built and linked a standalone door and hatch to the SAME lever already
driving the bridge — both VERIFIED, and confirms one lever can drive
multiple mechanisms at once. Floodgate turned out to be a real bug, not
a test gap: `ConstructFloodgate` is missing from the plugin's
workshop-compatibility table entirely, rejected at every workshop type.
Also stumbled into (and partly explained) the willow-wood "needs
non-economic logs" mystery from session 2 — a door job kept failing
against 12 free willow logs but succeeded instantly once a single fresh
non-willow tree was chopped. Full writeup in
[[project_fort4_session3_verification]].

## 2026-07-16 — Fort #4, session 2 continued (Summer y100, day 67→121)

Continued past the day-67 checkpoint per overseer direction ("keep going,
work on the dining hall next"), with a standing to redo bedrooms properly
(walls+door+cabinet, non-rectangular layouts, reserve a noble suite) —
see [[feedback_room_design_standards]]. Built an OCTAGONAL HUB at z=113
(one level below the sealed aquifer, in the dry stone reached this
session) as a combined dining hall + stairwell landing, with bedroom
spokes radiating N/NE/S/SE/SW. This is the fort's first genuinely
non-rectangular architecture.

**LIVE INCIDENT: the octagon dig destroyed the shaft's up-stair
component.** The hub's mine designation overlapped the 2x2 core and
silently converted `UpDownStair` tiles to plain `DownStair` — severing
the connection to everything above, with dwarves briefly stranded below.
`designate_dig type=stairs` re-issued on the same tiles refused to fix it
("already carved", 0 designated — the plugin only checks carved-or-not,
not stair sub-type). Root-caused and logged for engineering
(docs/decisions.md). Confirmed `build type=updownstair` (NOT "stairs")
DOES work as a real repair path — tested clean on virgin floor first,
then applied to the actually-broken tiles; full connectivity restored
and verified via cross_section top-to-bottom. Overseer initially planned
to fix this manually in-game, then handed it back once the repair path
was demonstrated working.

**A second near-miss, self-inflicted this time**: while the octagon/
spokes were mid-dig, a south-west bedroom pod turned out to sit right on
a real aquifer pocket — confirmed via cross_section (AQUIFER DAMP down
through z=110), matching the west-side hazard already known from earlier
this session but missed because the original bore-check sampled one
"representative" point per direction rather than the actual room
footprints. Cancelled that room outright rather than fight it. Real
water (not just residual DAMP) later appeared inside the NW room too
(one tile, then two) despite that room reading clean on bore-check —
overseer suggested `smooth mode=wall` on the exposed damp walls as a
stone-aquifer seal instead of constructed walls; applied to NW+NE rooms'
north walls, seepage stopped spreading (confirmed no growth after,
though `look` never visually reflects a successful smooth — logged as a
tooling gap). NW room ultimately abandoned as too wet to be worth
finishing; NE (built as a deliberately larger, extra-furnished NOBLE
SUITE reserve) and SE rooms furnished properly: door + bed + cabinet
each, NE additionally getting a chair, all zoned as WHOLE-ROOM bedroom
zones (not 1x1 bed-only zones) and assigned to specific dwarves.
**Verdict on z=113 as a housing level: workable but spotty** — dig the
NEXT housing cluster several levels further from the sealed aquifer
(e.g. z=109-110) for a firmer buffer, per overseer's direct suggestion.

**Furniture-supply lesson, learned the expensive way**: repeatedly hit
"needs bed"/"needs door"/"needs table" cancels on plans that LOOKED like
they had material in `stocks` — the count was including beds/doors/
tables already incorporated into OTHER buildings, not just free loose
stock (logged as a stocks bug). Real fix each time was to queue fresh
`queue_job` crafting and only then re-place the building. Overseer's
standing advice: stay ahead on furniture production before placing, or
expect instant silent cancels — a real DF pattern, not just us.

**MIGRANT WAVE ARRIVED — population 7 -> 16.** First migrant wave of
this fort. One new arrival, aban, already carries MINING skill (Lvl2) —
enabled the MINE labor immediately, a real chance at finally breaking
the single-miner ceiling that shaped this whole session (next session:
confirm aban actually mines — needs a pick, unverified whether one's
available). check_goals now shows has_min_dwarves_14 MET alongside the
earlier has_bedroom_zones_7 (now 9 zones, all real rooms or upgraded
zones — the old ad-hoc 1x1s for the two dwarves who got real rooms were
explicitly unassigned so they's not double-counted/wasted).

**Dining hall**: 2 of 3 tables + 1 chair built in the octagon hub by
session's end (3rd table + throne batch still mid-craft, blocked briefly
on a "needs non-economic logs" cancel despite 25 wood on hand — cause
not fully diagnosed, flagged as a follow-up). has_dining_hall predicate
still reads false; likely needs more chairs/tables actually complete
before it flips, or possibly a dedicated zone type this project doesn't
have yet (unconfirmed).

**Fort state at pause (day 121, summer)**: 33 buildings (was 3 at start
of the whole session, 21 at the day-67 checkpoint). Octagon hub (dining/
gathering space) + 2 furnished real bedrooms (NE noble suite, SE
standard) + 5 original single-tile ad-hoc bedrooms still standing in the
main hall + 2 in the old quarry room = 9 total bedroom zones for 16
dwarves. Second miner (aban) labor-enabled, unverified live. Wedding
(Kel + özum) and a promotion (äshrir -> full Carpenter) both occurred —
first fort life-cycle events observed. NEXT: verify second miner mines
(pick permitting), finish dining hall furniture, plan the deeper (z~109)
housing cluster properly bore-checked room-by-room this time, migrant
labor triage (several arrived with no skills/`labors=14` — check who's
actually a citizen vs a pet before assigning anything).

## 2026-07-16 — Fort #4, session 2 (Spring y100, day 34→67)

RESUME per overseer direction: expand the fort in its current state, get
valuables/goods into a protected underground stockpile, and rebuild the
removed surface workshops (carpenter, still) underground instead. Both
achieved, plus the fort's first DOUBLE-LAYER aquifer pierce+seal.

**Expansion**: merged the original farm hall (44-48,42-46)@118 with a new
west excavation (30-43,42-46) by mining through the old dividing wall —
one continuous hall, carpenter + still rebuilt inside it (33,44) and
(38,44), an "all" stockpile over the remaining floor. First underground
industry+storage space of the fort.

**AQUIFER: BOTH LAYERS PIERCED AND SEALED** (z=116 and z=115, each its own
full ring+wall cycle — first stacked/multi-layer aquifer this project has
executed, confirmed same protocol as single-layer, just repeated). Shaft
descended to z=112 (2 levels below the aquifer) where a small quarry
yielded 12 mudstone boulders — banked BEFORE opening either ring, per
protocol. All 16 ring tiles (8/layer) sealed with constructed walls. One
core tile (46,44,114) was silently skipped by an earlier designation
("already carved" false positive) and had to be re-designated by hand
after the fact — the residual puddle at that column didn't fully drain
until it was filled in. Seal verified dry end-to-end after ~5 game days;
industry level (z=112-114, dry stone) now open. Mason workshop placed
(50,44,112).

**MID-SESSION CORRECTION (overseer-caught)**: while the aquifer ring was
mid-dig, a concurrently-designated bedroom-cluster dig (south of the main
hall) was pulling the fort's ONLY miner away from the aquifer-sealing
work AND was one row from breaching a separate 7/7-depth standing-water
pocket at y=55+ — a second near-miss of the same class as session 1's
creek incident, this time self-inflicted via dig-ahead with no second
miner to spare. Cancelled the competing designation immediately; the
aquifer finished cleanly right after. Lesson written up in learnings.md.

**Tool friction found live** (see learnings.md for full detail): a
`stockpile` designation and a workshop's footprint can't coexist — build
workshops FIRST, stockpile second (it correctly skips occupied tiles).
`remove_building` targeting by coordinate picked the WRONG building when
a stockpile rectangle and a workshop's 3x3 footprint shared a tile — it
deconstructed the Still instead of the stockpile; rebuilt it, moved on.
Loose items sitting on stockpile tiles are invisible to `look` and still
block new `build` placement even after the stockpile itself is gone —
had to place beds on virgin, never-stockpiled floor instead.

**Fort state at pause (day 67)**: 21 buildings (was 3 at session start).
7/7 dwarves have OWNED bedrooms (bedroom zones goal now MET) — 5 in the
main hall (44,42-46,118), 2 in the new quarry room (52,43/45,112).
Carpenter + Still rebuilt underground and both active (drinks 14, seeds
46 climbing, boulders 12 banked). check_goals: has_bedroom_zones_7 ✓,
has_min_dwarves_7 ✓, no_active_hostiles ✓. Still open: dining hall (0),
has_shelter predicates (dug-tile modification tracker still reports 0 —
the regression flagged 2026-07-15 is NOT actually fixed, re-flag for
engineering). NEXT: dining hall + tables/chairs, deeper industry
(smelter/forge once ore is found), watch the 2 quarry-room beds — a
starter placement mixing housing with industry, revisit when a real
bedroom cluster gets planned somewhere hazard-checked.

## 2026-07-15 — Fort #4, session 1 (Spring y100, day 14→34; SAVED for later)

FRESH EMBARK on a RIVER/BROOK map (96x96, surface z≈119, brook channel at
z=118 snaking N-S down the east side with a WEST BEND at y=44-47 reaching
x=51, plus a NW lake complex; soil aquifer z=116-115 map-wide; dry stone
z=114-108; deep stone aquifer z=107-105). Session focus: live-verify the
2026-07-15 fix wave. Mid-session: DFHack had auto-updated to 53.15-r2 —
plugin refused to load until the checkout was retargeted + rebuilt (twice:
once for r2, once more for the brewing hotfix below, hot-swapped via
unload/copy/load with DF open).

**🍺 FIRST DRINK IN PROJECT HISTORY.** list_reactions was still empty on
the wave's DLL — root-caused live (permitted_reaction_STR is raw-load
staging, empty at runtime; DFHack's stockflow uses permitted_reaction_ID)
— hotfixed, redeployed, and then: still built → BREW_DRINK_FROM_PLANT
queued → first cancel taught "needs empty food storage item" → carpenter
+ 3 barrels → re-queued → dwarven wine +1 barrel, plump helmet seeds 5→10.
Drink chain closed after 3 forts. Also observed a dwarf DrinkItem after.

**THE CREEK NEAR-MISS (twice).** Sited the first shaft at (51-52,46-47)
and a farm hall east of it — both intersect the hidden brook channel at
z=118. Saved twice: once by DF's "Dangerous terrain" miner refusal, once
by the overseer asking "do you realize you're digging the creek into the
fort?". Root cause was a tool gap: cross_section shows NO water on hidden
tiles (bore at (51,46) called the 7/7 channel "dry hidden soil") despite
the world-model doc contract. Recovery: cancelled everything near water,
re-sited shaft (45,43)-(46,44) z119→117 INSIDE the west hall
(44,42)-(48,46)@118, 2-tile bank buffer respected. Fix agent dispatched.

**Fix-wave verification (Fort #4 live results):**
- Brewing ✓ (after hotfix; see above) — end-to-end with seeds returned
- Stockpile ✓ — overseer confirmed in-UI: real "all" preset, items
  hauling in, settings screen no longer crashes (Fort #3's killer)
- assign_crop ✓ — truthful "FAILED: under construction (stage 0/3)"
  pre-build, SUCCESS post-build, PlantSeeds jobs observed
- remove_zone severity ✓ (SUCCESS, was PARTIAL); case-insensitive
  filters ✓ ("weapon"/"plump"); connector-hint false positives GONE ✓
- TradeDepot ✓ BUILT (5x5 center→NW conversion works) — first depot
  ever; autumn caravan = pick source
- scope=fort / save_blueprint ✗ REGRESSION — the shared modification
  tracker now records NOTHING (dug hall reported "no modifications");
  the ambient-filter fix over-corrected. Fix agent dispatched.
- Forge/smelter untested (blocked on stone → blocked on aquifer pierce);
  repeat-warnings xN untested (no damp-cancel yet); lodging still pending

**Fort state at save (day 34):** surface: still (44,50), carpenter
(41,51), tradedepot (55,54), stockpile-all (50-52,54-56), wagon (47,47),
2 orphan down-stairs at (51-52,46-47) — cap or ignore. Underground: hall
(44,42)-(48,46)@118 with 2x5 plump-helmet plot (47-48,42-46) being
planted, spine stub (45,43)-(46,44) z119→117. Stocks: 13 drinks, 19+
logs, 15+ barrels, plump seeds x10. Second miner zasit labor-set (pick
count unknown — depot changes that equation). NEXT: aquifer-piercing
(z=116-115 soil, 2 layers, WEST of the fort, far from the brook), then
stone → forge/smelter tests + industry.

## 2026-07-15 — Fort #3, session 1 (Spring y100, day 14→49; ended by DF CRASH)

FRESH EMBARK, new map. Session goals: live-verify the 2026-07-14 tooling wave
(first time any of it touched a live game) and open the first fort built from a
whole-map plan (fort-planning skill). Ended early: DF crashed when the overseer
opened our stockpile's custom-settings screen in the UI — see gaps below.

**Site**: 96x96, surface z≈138 (range 138..145), 4 soil layers, stone from
z=133. TWO aquifers: a soil aquifer z=135-134 wetting only the NORTH half
(boundary ≈ y=46), and a map-wide THICK STONE aquifer z=126-122 (5 layers).
Ravine at x≈13-19 with a 7/7 stream at z=128 (bank access undug). Big SW
valley, surface down to ~z=128. Wagon (48,47,138).

**The plan worked**: sited the 2x2 spine at (50-51,54-55) in the dry south —
full descent surface→z=127 with ZERO aquifer piercing. Level map assigned at
embark: 137 farming / 136 food stockpile / 135-134 reserve / 133 industry
(loop topology, 9x9 halls E+W, satellite 2x2 stairs in each) / 132 industry
stockpile / 131 services / 130-129 housing / 128 crypt / 127 frontier stop.
Bilateral symmetry off the spine throughout; odd-width (7-wide) rooms.

**Fort state at crash (day 49)**: spine carved 138→127 (hatch built over
(50,54,138)); z=137 west = 2 dug 7x7 farm halls — hall A: 4 built 3x3 plots
(plump helmet/pig tail/cave wheat/quarry bush, all seasons, planted!), hall B:
7 built beds + 7 assigned 1x1 bedroom zones (has_bedroom_zones_7 ✓); z=137
east starter dorm/dining still designated-undug; z=133 industry ~70% dug, W
satellite stairs done, E in progress, 'all' stockpile (40,50)-(44,52) placed
(see CRASH bug); 19+ chert boulders, 57 logs, 7 beds+1 hatch+3 tables+3 chairs
made (mosus hit Carpenter Lvl6, one masterpiece bed); drinks 12 (flat all
session, odd), food thin but fisher active. Deep-aquifer pierce designated
(spine 127→120) but never reached by the miner — z=132/136/south-133
designations were cancelled to pull it forward; single-miner throughput was
the wall.

**Wave verification results** (the point of the session):
- list_crops DLL probe ✓ (157 crops, seeds-on-hand correct)
- BREWING ✗✗ FAILED — list_reactions returns EMPTY (even unfiltered);
  queue_job reaction=BREW_DRINK_FROM_PLANT → "reaction not enabled in
  fortress mode". Drink chain blocked a THIRD fort. Top engineering item.
- Farming ✓ end-to-end (plots→assign→PlantSeeds jobs observed) with TWO bugs:
  (1) assign_crop vs under-construction plot returns SUCCESS but does NOT
  stick in-game — must re-assign after the plot is BUILT (overseer confirmed
  in-game); (2) a plot tile that was undug wall at stamp time never registers
  with the building — assign_crop at that tile says "no building" forever
  while buildings lists the plot AT that exact coord (self-contradiction).
- Hatch ✓ (queue_job ConstructHatchCover → stock → build hatch at shaft top)
- remove_zone ✓ (designate→remove→gone; ACK severity mislabeled PARTIAL)
- Overlay honesty ✓ 'd' persists on in-flight digs; caveats: revealed
  SURFACE tiles and just-revealed wall faces can render plain while queued
- look scope=elevation ✓ (full 96x96, ~9.6k tok) — but aquifer count with NO
  spatial overlay ("4608 in view" = which half??); scope=fort ✓ at z=137 but
  BUGGED at z=133/136: bbox polluted by ambient tile updates (SW valley
  water/grass), rendered the wrong region entirely
- dwarves verbose ✓; survey_site surface RANGE ✓
- save_blueprint PARTIAL — captured only 28/49 tiles of a fully-dug 7x7 hall
  (suspect farm-plot-occupied tiles excluded); apply dry_run mechanics ✓
- step repeat-warnings line renders ("no repeated warnings") but no
  damp-cancel occurred; the xN path is still unexercised live
- STOCKPILE BUG (CRASH): stockpile category=all placed OK but shows in-game
  as "custom", and opening its custom settings CRASHED DF, ending the
  session. Plugin likely sets category flags without populating the
  per-category item vectors v50's UI expects. Under investigation.
- set_labor: flags verifiably stick (read-back after 30+ days) but the 2nd
  MINE dwarf never mined. Hypotheses: single embark pick (v50 needs a pick
  in hand) vs v50 work-details overriding raw unit labor flags. Under
  investigation (DFHack source agents).

**Process notes**: turn cadence solid; connector suggestions now emit proper
L-legs (fix confirmed live) but still miss adjacency to carved spine stairs
(false "not connected, nearest open tile at map edge"). stocks/list filters
are CASE-SENSITIVE ("weapon"=nothing, "WEAPON"=works). No wagon glyph exists
in look renders (wagon visible only via buildings).

## 2026-07-14 — Fort #2 ("First Fort round 2"), session 1 (Spring y100, day 14→45)

FRESH EMBARK, new world. Session goals: live-verify the new Zones+Locations
tools, and pierce/seal the aquifer UNSUPERVISED using only these memory files.

- **AQUIFER PIERCED AND SEALED UNSUPERVISED — the protocol worked.** Site:
  surface z=131, single-layer sand aquifer at z=128 (thinner than Fort #1's),
  stone from z=126. Timeline: shaft z131→129 (day 14-17), pierce designation
  day 17, loud damp-cancel day 18, re-designate → pierced to z=126 by day 19,
  quarry z=126 for mudstone (non-economic layer stone — no bauxite trap this
  time), 8-tile orthogonal ring mined day 27, ALL 8 walls built by day 29,
  residual 2-tile puddle fully evaporated by day 38. Zero inflow since.
  ~11 game-days pierce→seal, zero human coaching. Protocol deviations:
  ring dig took THREE designation rounds (protocol said expect two) with NO
  alert feedback (announcement dedup swallows repeat damp-cancels); improved
  order-of-operations: quarry stone BEFORE mining the ring so walls start
  instantly.
- **New tools all live-verified**: designate_zone (bedroom/dormitory/barracks/
  animal_training/meeting_hall/water_source all placed), assign_zone (coord-
  addressed; ownership shows in list_zones), unassign_zone (roster cleared),
  create_location tavern (list_locations correct), assign_lodging (ACK
  SUCCESS, list_locations shows "1 lodging room(s)"). check_goals
  has_bedroom_zones went 0→1 (needs a step() after zone creation — stale
  snapshot reads 0 at first, NOT a broken predicate).
- **Lodging in-game effect: UNVERIFIED (pending, not null).** Wire path fully
  works; no visitor has arrived by day 45 (expected — 7-dwarf fort, low
  wealth, visitors take seasons). Keep watching next session.
- Error-text quality confirmed excellent: Barracks assign → "uses squads,
  not units — see future military/squad workstream"; AnimalTraining assign →
  "labor-driven, no roster". Both accurate and educational.
- **BrewDrink CONFIRMED still blocked** (live re-test this fort): no job_type
  named Brew*; brewing is CustomReaction (needs job->reaction_name);
  queue_job CustomReaction cleanly rejected by workshop whitelist. Drink
  chain dead until the reaction-based path lands.
- DISCOVERY: designation overlays now painted in look ('d' glyph) — landed
  from the 009 wishlist. Caveat: UNDER-REPORTS — tiles with in-flight dig
  jobs render as plain wall; do not diagnose "cancelled" from a missing 'd'.
- Fort state at pause (day 45): shaft z131→126 sealed through aquifer;
  z=130 base room (food stockpile + meeting hall/TAVERN + 2 junk test zones,
  no zone-delete tool exists) + dorm room (4 beds built: 1 owned by etur,
  1 tavern-lodging; 3 more beds in stock, unplaced); z=126 quarry (carpenter
  + still built) + W annex mostly dug; 13 boulders, 22 logs, 12 drinks,
  food THIN (~12 units + 8 raw fish), farming still tool-blocked (no farm
  plot build type). Water: 7/7 stream found far W in gully (z=141), water_
  source zone painted on bank (4-9,71,142). Second miner (meng) enabled
  pre-emptively at embark — single-miner default-embark pattern confirmed
  again.
- Next session: place 3 remaining beds + bedroom zones (goal 7), dining
  hall (tables/chairs via carpenter queue_job), fishery for the raw fish,
  watch for migrants/visitors (lodging verification), food pressure watch.

## 2026-07-12 — First Fort, session 2 continued: pre-009 verification pass (day 103-104)

- Reconnected post pre-009-blocking-fixes wave. Census fix LIVE-VERIFIED
  immediately: `dwarves` now reports 7, not 18.
- set_labor LIVE-VERIFIED: dwarf_detail now shows full labor lists.
  Revealed the real shape of the labor collision — Doren isn't a
  specialist blocked by ONE competing labor, he's the fort's only
  MINE-enabled dwarf among ~80 labors each dwarf carries by default
  (confirmed: Solon has every labor except MINE). Applied the practical
  fix: enabled MINE on Solon (idle, stone-working skills) as a second
  miner. Verified via read-back, stepped once to confirm no errors.
- job_types filter confirmed live: "hatch" correctly resolves
  ConstructHatchCover by name — generalized construction's name
  resolution works end-to-end; the remaining hatch-cover gap is now just
  the one-line jobTypeAllowedAtWorkshop whitelist entry, not a rebuild.
- Session pauses here per overseer direction — pivoting to 009
  (Culture & Learning) design work. First Fort's core loop is proven;
  remaining fort progress (drink chain, dining hall, migrant wave,
  year-1 survival) continues opportunistically rather than as the
  primary focus.

## 2026-07-12 — First Fort, session 2 (Spring, year 100, day 62→69)

- Model handoff: Fable 5 -> Sonnet 5 mid-project. Picked up right where
  session 1 left off (aquifer sealed, descent to z130 designated).
- Fix-wave verification LIVE: real calendar date confirmed (year=100
  season=spring day=62, was year=0/season=?/day=0), chop/gather now work
  (3 logs -> 13 across 4 species from one chop designation; gather brought
  in cotton/lettuce/bitter-melon plant materials), aquifer seal holding
  (only a ~1/7 residual puddle, DAMP/AQUIFER annotations render correctly
  in cross_section), carpenter workshop completed once shale boulders
  existed, `buildings`/`stocks` material+economic fields all live-correct.
- DISCOVERED: manager work orders (the `order` tool) need a Manager noble
  + office (chair+table+door — all buildable today) to ever leave
  validated=true/active=false. No tool exists to assign a noble or claim a
  room — a NEW gap, distinct from the already-known zone/civzone stub.
  Separately confirmed (before it could bite): work_orders.cpp was setting
  manager_order.mat_type=-1, which is DFHack's INVALID-material sentinel,
  not "any material" (mat_index=-1 is correct; mat_type's default is 0) —
  fixed alongside.
- ENGINEERING DETOUR (mid-session): implemented `queue_job`, a new plugin
  command (Job::linkIntoWorld + Job::assignToWorkshop) that queues a job
  directly at a workshop, bypassing the manager entirely — mirrors
  right-clicking a workshop in vanilla play. Researched via DFHack source,
  adversarially reviewed, plugin rebuilt + redeployed (2nd DF restart this
  session), Go server reconnected to pick up the new tool.
- LIVE-VERIFIED: `queue_job` x2 bed at the carpenter workshop -> both
  ConstructBed jobs completed (~1 day each, consuming a wood log each) ->
  `build bed` x2 at (48,48,139) and (48,52,139) -> BOTH BUILT. First beds
  of the fort exist. Material-filter guess (item_type=WOOD at Carpenters)
  was correct on the first live try.
- Reconnect quirk (both times this session): a fresh `/mcp` + `ai-connect`
  cycle reports "Connected" immediately but returns sentinel data
  (year=0, dwarves=0) for one or more queries until a `pause` call — not
  reliably the FIRST query afterward, took 3-4 calls the second time.
- DWARF CENSUS CORRECTED (user caught this): the `dwarves`/`dwarf_detail`
  tools return 18 units, but sampling all 18 via dwarf_detail showed only
  7 have first_name + dwarf-typical labor skills (Doren/Miner, Dumed/
  Carpentry, Kulet/Metalcraft, Fath/Fish, Solon/Masonry, Mafol/social
  skills, one garbled-name Plant/RecordKeeping) — the other 11 are unnamed
  with either zero skills or just CLIMBING 15 (pack animals/pets). True
  fort population is 7, matching the default embark. Dashboard's
  `dwarves=18` count is misleading — needs a citizen/race filter upstream.
- ALL 7 BEDS BUILT (queue_job x7, build bed x7) — full sleeping coverage,
  first real headroom milestone of the fort. One retry needed: a bed plan
  at (52,51,139) died silently after the first attempt (tile was fine,
  loose bed stock was plentiful) — re-placed clean per the existing
  "RE-PLACE any dead plan" learning.
- Still workshop built at (45,49,134) but CANNOT be given a job yet:
  BrewDrink has no job_type mapping in protocolToJobType (returns -1) in
  either order or queue_job — needs a DFHack reaction-based lookup, not a
  plain job_type enum value. Drink chain stays blocked until that lands.
- Design note (user): building/furniture glyphs in `look` need to be
  CATEGORY-level (workshop/furniture/door/stockpile/trap, ~5-8 glyphs),
  not one-per-type — DF has 50+ building/furniture/construction types,
  far more than the remaining ASCII budget after terrain glyphs. Exact
  type stays a `buildings`/`building_status` detail-query, matching the
  existing progressive-disclosure pattern. Belongs in 009's already-scoped
  "designation + building overlays painted into look" workstream.

## 2026-07-12 — First Fort, session 1 (Spring, year 100)

- OUTCOME: **AQUIFER SEALED.** All 8 constructed shale walls stand around the
  2x2 staircase at z136; pocket entombed west; zero inflow remaining. ~28
  game days elapsed (frame 5350→32237). Descent resumed via dry offset shaft
  from the east quarry hall (54-55,50-51) z134→130. Carpenter workshop
  re-placed. Session ended at a clean pause with the overseer (human)
  coaching the aquifer protocol — full tool-gap list filed in the session
  report for engineering.

- First MCP-driven session on a fresh default embark. Connection chain worked
  first try (df-mcp listener → ai-connect).
- Dug 2x2 stair shaft (46,50)-(47,51) from surface z140 down; 5x5 storage
  room at z139 east of shaft; carpenter workshop placed at (50,50,139) but
  NOT yet built (material problem, see below).
- STAIR QUIRK INCIDENT: continuing a damp-cancelled shaft by designating
  stairs starting at the undug level (z136) carved a down-stair with no up
  component — the level below became permanently unreachable and the old
  column below 137 is abandoned (now a sealed, slowly-filling water pocket
  at (46-47,50-51,136)). Recovered by digging a side corridor at z137 and
  sinking a parallel 2x2 shaft at (49-50,50-51), z137→134.
- SAND AQUIFER at z136 (whole layer per overseer). Pierced it on the new
  shaft (2nd designation digs after DF's warning-cancel). Seal in progress:
  6-tile orthogonal ring mined on N/E/S sides, ONE stone wall built at
  (49,49,136); remaining 5 walls blocked — no usable non-economic boulders.
  West side (48,50-51) still natural sand, pocket-adjacent; plan is to mine
  + wall it LAST, with spare boulders staged, accepting the pocket dump
  (drains to 134 and evaporates).
- Quarries at z134: east room struck BAUXITE (economic → unusable for
  constructions by default). West quarry (44-46,48-52) in progress hunting
  layer stone.
- Embark stocks reality: 3 logs (untouched by wall jobs), 3 "ROCK" items
  that are NOT construction boulders, 0 real boulders at start. 12 drinks,
  ~30 food. Fisherdwarf is producing (raw fish 19→26).
- Time elapsed: ~14 game days (sim frame 5350 → 22868). No deaths, no
  enemies. Dig/step/alert loop feels solid.
