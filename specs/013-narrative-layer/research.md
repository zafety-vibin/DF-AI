# Feature 013 — The Narrative Layer: research report

**Status:** research only (2026-07-24). No code written. This document is the
feasibility verdict + struct inventory + proposed tool surface for letting the
AI overseer READ THE STORY the simulation writes — commissioned by the playing
session itself after a full fort year of personifying dwarves from job strings
alone.

**Method:** all claims verified against the DFHack **53.15-r2** source checkout
at `C:/Users/zmanl/Projects/dfhack-build` (`CMakeLists.txt:10` sets
`DF_VERSION "53.15"` — the same tag the plugin builds against). Citations below
are relative to that checkout root unless prefixed `repo:` (this repository).
Three parallel structure surveys (personality/prose, combat/reports,
social/art) were spot-checked line-by-line for every load-bearing claim.
Where something does NOT exist, it is flagged **NOT FOUND** — those negatives
were verified by exhaustive grep, not absence of memory.

---

## 1. Executive summary — feasibility verdicts

| # | Desire | Verdict | One-line basis |
|---|--------|---------|----------------|
| 1 | **Portrait card** | **COMPOSABLE** (structs open; vanilla's exact prose is a closed-source gap with a known, acceptable fidelity loss) | Every semantic ingredient (facets, values, needs, emotions+causes, preferences, deity, links, history hooks) is readable from structs; DFHack ships the exact threshold tables to band them into prose tiers. Vanilla's own composed sentences exist in memory only as lazily-rendered UI buffers harvestable via simulated tab-clicks — off-limits by house rule, and not needed. |
| 2 | **Story pulse** | **EXISTS-IN-STRUCTS** — cheap, cursor-friendly | Each emotion record carries its own `year`/`year_tick`; a fort-wide since-cursor scan with strength thresholds is exactly the shape the announcement cursor + wildlife remembered-set already prove out. Bonus: emotion records carry `facet_change`/`value_change` flags — arcs *forming* are literally flagged. |
| 3 | **Combat narrator** | **EXISTS-IN-STRUCTS** + one opportunistic live probe | Per-unit combat report-id vectors exist (`unit.reports.log`); momentum is computable from wound deltas with per-wound attacker attribution and NO text parsing; a real engagement struct (`activity_event_conflictst`, with per-side `peak_strength`/`current_strength`) exists — whether fort combat reports link to it (`report.activity_id`) needs one live check, with a proven fallback (unit-intersection + 500-tick window, DFHack's own constant). |
| 4 | **Relationship edges** | **EXISTS-IN-STRUCTS** (bond CATEGORY is computed, not stored — but the thresholds are documented in the XML itself) | Family/lover/deity edges are histfig links (cheap). Friend/grudge lives in `relationship_profiles.hf_visual` → `core.love` (-100..100) with DF's own UI banding documented in a struct comment, plus an explicit `rank` enum containing `grudge`, `war_buddy`, `childhood_friend`, etc. |
| 5 | **The fort's own art** | **EXISTS-IN-STRUCTS** with one join caveat | Forms/written works are fully resolvable to readable strings server-side (proven in-tree). No struct carries "composed at site X" — fort-locality comes from four history-event types that DO carry `site`, matched against `plotinfo.site_id`, append-only and tail-cursorable; author-∈-citizens is the complementary filter. |

**Honest gaps, stated up front:**

- **Vanilla's exact prose phrasing is unreachable without the viewscreen.** The
  client's "He is assertive. He needs alcohol to get through the working day."
  strings live in `view_sheets_interfacest.personality_raw_str`
  (`library/xml/df.d_interface.xml:1793`) and siblings — filled lazily by the
  renderer, and the only in-tree harvester (`scripts/markdown.lua:95-125`)
  **synthesizes mouse clicks on the unit-sheet tabs** with hard-coded pixel
  offsets. That path violates this project's never-touch-the-viewscreen record
  and is fragile besides. We compose our own prose from the same underlying
  numbers using DFHack's own threshold tables (§2.1). Fidelity loss: our
  wording will not match the client's word-for-word (vanilla even varies its
  phrasing per-dwarf via `unit_preference.prefstring_seed`,
  `library/xml/df.unit.xml:1529`); the *semantic content* — which facets, which
  tier, which needs — is identical. For a model reading 300 tokens, semantics
  are what matter.
- **`subthought` (the emotion's cause-detail id) has NO resolver anywhere in
  DFHack** — exhaustive grep confirmed. Partial inline maps exist in the XML
  (§2.1c). We ship a curated best-effort resolver for the highest-frequency
  thought types and emit the raw int with an honest `(unresolved)` otherwise.
- **The mandate ← preference link is unproven in this checkout.** Strange moods
  provably consume `LikeMaterial` preferences (`plugins/strangemood.cpp`);
  no code ties preferences to noble mandates (that logic is vanilla-side,
  closed). Portrait says "drives strange moods" where proven and stays silent
  on mandates until observed live.
- **~40 of 282 thought types have no `caption` attribute** (e.g.
  `AcquireArtifact`); those render as the enum name, same as today.

---

## 2. Per-desire findings

### 2.1 Portrait card — "WHO IS THIS DWARF"

**What the repo already has** (`repo:dfhack-plugin/queries.cpp:1335-1626`,
`handleDwarfDetail`): psyche block with stress + category, full needs list,
20-cap recency-sorted emotions **already decoding thought captions via
`ENUM_ATTR(unit_thought_type, caption, …)`** (queries.cpp:1538), top-8 facets
by |deviation from 50|, full values, dreams. The portrait is a *salience +
prose + relationships + preferences + history* layer over this foundation, not
a new foundation.

#### (a) Personality prose — compose, don't harvest

`scripts/gui/unit-info-viewer.lua` (read in full, 559 lines) does **NOT**
compose personality prose — it renders caste description, age, size, death
info only. Its one threshold table: size vs median at `:364-373` (`>=110`
larger / `<=90` smaller than average). Vanilla's own personality prose is the
UI-buffer gap described in §1.

The threshold tables to compose equivalent prose ourselves — all shipped by
DFHack, all verbatim-portable to C++:

| Table | Source | Content |
|---|---|---|
| Facet tiers (7 bands) | `scripts/modtools/set-personality.lua:28` (comment `:27`: "bounds at which the description for a personality trait changes") | `0-9 / 10-24 / 25-39 / 40-60 / 61-75 / 76-90 / 91-100` |
| Tier labels | `scripts/assign-facets.lua:13-22` | Lowest / Very Low / Low / Neutral / High / Very High / Highest |
| Value (belief) tiers | `scripts/modtools/set-belief.lua:31` (`getBeliefTier` `:226-236`) | `-50..-41 / -40..-26 / -25..-11 / -10..10 / 11..25 / 26..40 / 41..50` |
| Need fulfillment tiers + labels | `scripts/modtools/set-need.lua:409-432` | ≤-100000 "Badly distracted" … ≥300 "Unfettered" |
| Need strength labels | `set-need.lua:435-454` | Slight / Moderate / Strong / Intense (need_level 1/2-4/5-9/≥10) |
| **Trait→need derivation** | `set-need.lua:216-247` (`needDefaultsInfo`) | e.g. `:218` `DrinkAlcohol ← IMMODERATION`, `:217` `Socialize ← GREGARIOUSNESS`, `:220` `StayOccupied ← ACTIVITY_LEVEL+HARD_WORK` — literally the mechanics behind "He needs alcohol to get through the working day" |
| Stress tiers | `library/modules/Units.cpp:2116-2117` (`stress_cutoffs {50000, 25000, 10000, -10000, -25000, -50000, -100000}`, category 0=worst) | labels "Miserable"…"Ecstatic" at `plugins/lua/spectate.lua:65-71` |

Underlying structs (all verified): `unit_personality.traits` — fixed 50-wide
`int16_t` array indexed by `personality_facet_type`
(`library/xml/df.personality.xml:1483`; enum at
`library/xml/df.d_basics.xml:1296-1348`, `ASSERTIVENESS` at `:1342`). Values:
`personality_valuest {type, strength}` (`df.personality.xml:98-101`; enum
`value_type` `df.d_basics.xml:524-559`). Needs: `personality_needst {id,
deity_id, focus_level, need_level}` (`df.personality.xml:1408-1413`; enum
`:1374-1406`, 30 values). Dreams: already surfaced.

**Salience selection (the design problem):** the client shows all 50 facets;
the portrait shows facets whose tier ≠ Neutral (outside 40-60), top 3-5 by
|deviation|; values at |strength| ≥ 26 (Very High/Low bands), top 2-3; needs
split into *starved* (focus_level ≤ -1000, i.e. "Unfocused" or worse) and the
single best-fed; the trait→need table turns IMMODERATION+DrinkAlcohol into one
composed line instead of two raw facts.

#### (b) Strongest emotion and its cause

`personality_moodst` (`library/xml/df.personality.xml:1326-1337`): `type`
(emotion_type), `strength`, `relative_strength`, `thought`
(unit_thought_type), `subthought` (raw int32 "for certain thoughts"),
`severity`, `flags`, `year`/`year_tick` (last-used stamps). The repo already
sorts by recency and decodes captions. `emotion_type` carries a `divider`
attr (negative = positive emotion — `df.d_basics.xml:687`), already emitted.

`unit_thought_type` is defined at `library/xml/df.personality.xml:119-1312`
(NOT d_basics; there is no df.unit_thoughts.xml) with enum-attrs `caption` and
`xml_caption` (`:120-121`). 242 of 282 items carry a caption; captions embed
placeholder tokens — verbatim examples:
`:138` `'at the unexpected death of [somebody]'`, `:171`
`'upon mastering [skill]'`, `:243` `'near a [quality] [building]'`. Token
vocabulary observed: `[somebody] [skill] [building] [deity] [animal] [book]
[poetic form] [musical form] [dance form] [relative] [HF relative] [value]
[syndrome] …` and the `[multiple]` sentinel (`:228`, `Complained`) meaning
"caption depends entirely on subthought."

**Subthought resolution — NOT FOUND anywhere in DFHack** (modules, plugins,
scripts — every grep hit inventoried; the only writer is
`scripts/add-thought.lua`, the only semantic map there is Syndrome→syndrome id
at `:17-23`). What exists to build a partial resolver from:

- XML inline maps: `Complained`/`UnableComplain` subthought codes 0x19-0x1D
  (`df.personality.xml:230-234`, `:326-330`), `ReceivedComplaint` `:239-240`,
  `GhostNightmare`/`GhostHaunt` relative-type codes 0x00-0x12 (`:294-317`),
  `RelativeExpelled` = `histfig_relationship_type` (`:1129`).
- The unwired `circumstance_id` union (`df.personality.xml:109-117`): `Death`,
  `Prayer`, `DreamAbout`, `Defeated`, `Murdered` → `historical_figure`,
  `HistEventCollection` → collection, `AfterAbducting` → histfig. It is used
  by history-event structs but never by `personality_moodst` — still, it
  documents Bay12's intent: for death/prayer/dream/defeat-class thoughts,
  subthought is very likely a histfig id. **Resolve-and-verify**: treat
  subthought as a histfig id for that curated class, resolve via
  `df::historical_figure::find()`, emit the name ONLY when the lookup
  succeeds and the figure is plausible (known to the fort); otherwise emit the
  raw int. Truthful, incremental, and the resolver is shared with the story
  pulse (§2.2).

#### (c) Preferences that matter mechanically

**Two-struct trap:** `unit_personality.preferences`
(`df.personality.xml:1481`) is a worldgen-side generator structure. The real
list is **`soul.preferences`** (`library/xml/df.soul.xml:67`) →
`unit_preference` (`library/xml/df.unit.xml:1504-1530`): `type`
(`unitpref_type`, `:1484-1498`: LikeMaterial / LikeCreature / LikeFood /
HateCreature / LikeItem / LikePlant / LikeTree / LikeColor / LikeShape /
LikePoeticForm / LikeMusicalForm / LikeDanceForm), a not-really-a-union int32
(`item_type` / `creature_id` / `color_id` / `shape_id` / `plant_id` /
`poetic_form_id` / `musical_form_id` / `dance_form_id`, `:1509-1518`),
`item_subtype`, `mattype`/`matindex`, `mat_state`, and the gate flag
`flags.bits.visible` (`:1500-1502`) — every consumer filters on it.

Render template: `scripts/assign-preferences.lua` `format_preference()`
`:29-59` (matinfo token for materials/food/plants, raw registry lookups for
creature/color/shape, `translateName` for the three form types).

**Mechanical relevance, proven:** strange moods consume `LikeMaterial`
exclusively — `plugins/strangemood.cpp:656-670` (stone), `:734-754` (cloth),
`:817-827` (metal bars), `:847-861` (glass), `:879-903` (bone/shell), all
gated on `flags.bits.visible && type == LikeMaterial`. Portrait therefore
ranks: LikeMaterial first (mood insurance), LikeFood/LikeItem next (feasts,
and the *suspected* mandate driver — unproven, §1), one flavor pick (form/
color/creature) last. Cap 3.

#### (d) Relationships (portrait subset) — see §2.4 for the full graph

Per-dwarf: walk `historical_figure.histfig_links`
(`library/xml/df.history_figure.xml:1070`; 17 link classes `:963-996`, type
enum `df.d_basics.xml:4670-4687`) for spouse/lover/deity(+`link_strength`,
`:945`)/children/master/apprentice/companion; then scan
`info.relationships.hf_visual` (`:810`, `:774`) filtered to the fort's
citizen-hfid set for best friend (max `core.love`) and worst grudge (min
`core.love` ≤ -50, or `rank` in the grudge family) — thresholds in §2.4.
Resolution histfig→living dwarf: `historical_figure.unit_id`
(`df.history_figure.xml:1062`) or a prebuilt hfid→unit map from the citizen
pass.

#### (e) Physical description

`Units::getPhysicalDescription` **was removed from DFHack** — the surviving
evidence is remotefortressreader's TODO
(`plugins/remotefortressreader/remotefortressreader.cpp:1713`). Cheap truthful
substitute, all verified: `Units::getReadableName(unit)`
(`library/modules/Units.cpp:1196-1216` — native name, english name,
profession, ghost/corpse/tame tags), the caste description string ("A short,
sturdy creature fond of drink and industry…" — `library/xml/df.creature.xml:1021`,
accessed as `getCasteRaw(unit)->description`), and the size banding
(`unit.appearance.size_modifier`, `df.unit.xml:2850`; thresholds 110/90 from
`unit-info-viewer.lua:364-373`). Composing full appearance prose ("his very
long beard…") from `appearance_modifierst.desc_range`
(`df.creature.xml:28-47`) + `unit.appearance` (`df.unit.xml:2844-2869`) is
possible but is a phrasing-table project of its own — **deferred; not in any
proposed wave.**

#### (f) Pre-fort history hook

Cheap, per-dwarf, no world-event scan:

- **Bands/guilds/faiths:** `histfig.entity_links` (`df.history_figure.xml:1068`)
  → `MEMBER`/`FORMER_MEMBER` links (`:839-841`) →
  `historical_entity.type == PerformanceTroupe` (`library/xml/df.entity.xml:1401`,
  enum `:1391-1403` — Religion/MerchantCompany/Guild/MilitaryUnit come free).
  Reverse direction (troupe→members): `entity.histfig_ids` (`df.entity.xml:1436`).
- **What art they carry:** `info.known_info` (`knowledge_profilest`,
  `df.history_figure.xml:351-374`): `known_poetic_forms` (`:368`),
  `known_musical_forms` (`:369`), `known_dance_forms` (`:370`),
  `known_written_contents` (`:355`) — sorted id vectors, O(1) per dwarf.
  "edóm knows 4 musical forms and carries 2 books' worth of writing" is a
  two-line read.
- **Masterworks/kills:** `info.masterpieces` (`artistic_profilest`,
  `:93-102` — `events` vector + a 100-slot `top_related_heid` array) and
  `info.kills` (`:41-44`). **Caveat:** nothing in DFHack reads
  `artistic_profile`; whether form-creation events land in it is undocumented
  — live probe (§5), portrait uses counts only until verified.
- **NOT FOUND:** a general per-figure event index. Full biography = scan of
  `world.history.events` — explicitly out of the portrait's budget; the art
  registry (§2.5) does that scan once, fort-scoped, for everyone.

**Portrait verdict: COMPOSABLE.** Plugin emits one salient structured JSON
(~1KB); Go renders 15-25 prose-ish lines. 250-450 tokens/call.

### 2.2 Story pulse — the fort's emotional weather

**Structures support it cheaply — confirmed.** Every `personality_moodst`
carries `year` + `year_tick` (last-used stamps, `df.personality.xml:1335-1336`)
— the repo's own `handleDwarfDetail` already recency-sorts on exactly these
fields (queries.cpp:1519-1522). A pulse is: one pass over citizens
(`Units::isCitizen`, the `handleWellbeing` precedent, queries.cpp:1294-1333),
inner filter `(year, year_tick) > cursor`, salience rank, hard cap.

Design mirrors the two proven repo patterns:

- **Cursor:** plugin-global `(year, year_tick)` watermark + mutex, advanced on
  read, reset on reconnect (announcements.cpp `g_last_sent_report_id` +
  `reset_announcement_cursor` semantics). Re-felt emotions bump their stamps
  in place — the same-object-updated problem the repeat_count tail window
  solved; here the stamp IS the signal (a re-felt grudge emotion is news).
- **Dedup:** within-window dedup by `(unit, emotion, thought, subthought)`,
  remembered-set style (wildlife tripwire `g_known_dangerous_wildlife_ids`,
  queries.cpp:1902-1955), bounded by clearing entries older than the cursor
  window.

**Salience ranking, in order:** (1) emotions whose `flags` carry
`facet_change`/`value_change` (`personality_mood_flag`,
`df.personality.xml:1314-1324`) — the simulation literally flags
personality-altering moments; (2) `MadeFriend` / `FormedGrudge` thoughts
(`:656`, `:660`) — edges forming (§2.4 cross-check); (3) |strength| above
threshold, negative (`divider > 0`) weighted above positive; (4) recency.
Cause text via the shared caption+subthought resolver (§2.1b). Cap 12-15
lines + truthful `N others below threshold` clamp line.

**Cost:** O(total emotion entries across citizens) compares per call — for 50
citizens this is thousands of int compares, negligible. **Open question:** how
large emotions vectors grow over fort-years (DF may or may not prune; §5). If
they balloon, per-unit early-out on max-stamp is trivial insurance.

**Verdict: EXISTS-IN-STRUCTS.** ~200-350 tokens/call.

### 2.3 Combat narrator with momentum

The deepest survey; full pipeline below. Headline correction first:
**`df::report` lives in `library/xml/df.announcement.xml:39-70`** (the file
`df.report.xml` is something unrelated), and the vector to read is
**`world.status.reports`** (`df.announcement.xml:180`) — the authoritative
append-only log — NOT `world.status.announcements` (`:181`), which is the
display *subset* holding the same pointers (`library/modules/Gui.cpp:1894`
always appends to reports; `:1922-1927` inserts into announcements only for
display-routed types and sets `flags.bits.announcement`). Combat strike lines
are routed per-type by `d_init.announcements.flags[type]`
(`library/xml/df.d_init.xml:169-182`): the `UNIT_COMBAT_REPORT` bit (`:175`)
marks combat-log lines; `D_DISPLAY` (`:174`) marks ticker lines. **Our
existing announcements.cpp pipeline reads the ticker; the narrator reads the
log. They are different feeds and the 53.15 pooling gotcha applies to both**
(never delete `df::report`; max-id cursoring; `repeat_count` bumps in place,
`df.announcement.xml:54`).

Key structures (all verified):

- **Report fields** (`df.announcement.xml:41-69`): `type`, `text`, `flags`
  (uint8: `continuation` "set on all but the first" / `unconscious`
  (adventurer-only — do NOT read as victim-KO, written from
  `units.active[0]` at Gui.cpp:1850-1852) / `announcement` / `high_prio_removal`,
  `:31-37`), `repeat_count`, `zoom_type`+`pos` (`zoom_type==Unit` ⇒ pos is a
  creature), `id`, `year`, `time`, and — since v0.40 — `activity_id`,
  `activity_event_id` (default -1).
- **Per-unit combat logs**: `unit.reports.log` — static array of report-id
  vectors indexed by `unit_report_type` (`NONE/-1, Combat/0, Sparring/1,
  Hunting/2`) — `library/xml/df.unit.xml:3038-3046`, enum `:2162-2167`.
  Sibling `last_year`/`last_year_tick` per category (garbage when the vector
  is empty — XML's own warning `:3043`).
- **Engagement grouping**: a real conflict object exists —
  `activity_event_conflictst` (`library/xml/df.activity.xml:660-668`) with
  `sides` → `conflict_sidest` (`:651-658`): `unit_ids`, `histfig_ids`,
  `enemies`, **`peak_strength`, `current_strength`** — DF already tracks
  per-side attrition (a ready-made momentum verdict), plus
  `inactivity_timer`/`attack_inactivity_timer` (DF's own "is this fight over"
  clocks). Reachable from a report IF `report.activity_id` is populated for
  fort combat — **undeterminable from the checkout** (DFHack's own injected
  reports leave it -1; nothing in-tree reads it back). Live probe #1 (§5).
  **Proven fallback**: group by unit-id intersection + time proximity with
  DFHack's own `RECENT_REPORT_TICKS = 500` (`library/modules/Gui.cpp:107`),
  exactly what `EventManager::updateReportToRelevantUnits` does
  (`library/modules/EventManager.cpp:1081-1100`, memoised per frame).
- **Momentum without text parsing** — the cheapest correct per-unit diff set:

  | Signal | Field | Cite (df.unit.xml) |
  |---|---|---|
  | casualty | `flags2.bits.killed` | :1375 |
  | knocked out | `counters.unconscious` | :2886 |
  | stunned/winded | `counters.stunned` / `.winded` | :2885 / :2884 |
  | suffering | `counters.pain` | :2898 |
  | bleeding out | `body.blood_count` vs `blood_max` | :2836 / :2835 |
  | gore | `flags2.bits.gutted` | :1385 |
  | hits taken | `body.wound_next_id` high-water mark | :2816 |

  Snapshot at step start for units in fights; on read, walk only wounds with
  `id >= watermark`. **Attribution is free**: `unit_wound.attacker_unit_id`
  (`:2097`) — "hits landed by X" = new wounds anywhere with attacker X
  (freshness idiom `wound->age <= 1 && attacker_unit_id == id`,
  EventManager.cpp:1124-1131). Severity classes from structured flags, no
  English: sever = `unit_wound.flags.severed_part` (`:2082`); fracture =
  `parts[].flags1.broken`/`.compound_fracture` (`:1949`/`:1961`); artery =
  `.major_artery`/`.artery` (`:1943`/`:1963`); bruise =
  `parts[].effect_type == Bruise` (`df.d_basics.xml:11990`) or strain-only
  (`df.unit.xml:1990`); mutilation = `.guts_spilled` (`:1944`). **NOT FOUND:**
  a `mortal_wound` flag — lethality is read from death state only. Avoid
  `unit.health` (nullable, lazily materialized, garbage high bits per the
  XML's own warning `df.unit.xml:2227`).
- **Casualty attribution**: `flags2.killed` → `unit.counters.death_id`
  (`:2882`) → `df::incident` (`library/xml/df.incident.xml:99-156`:
  `victim`, `criminal`, `death_cause`, `activity_id`) — same struct the repo's
  burial detection already uses.
- **Telling-line selection** (the reader's ask): rank stitched report groups
  by (1) structured severity of the wound that landed in the same window
  (sever > artery > KO-transition > fracture > the rest), (2) status
  transitions (`COMBAT_EVENT_KNOCKED_OUT`, `_STRANGLE_KO` — the combat
  announcement_type block is `library/xml/df.g_src.basics.xml:47-181`,
  `COMBAT_STRIKE_DETAILS` `:147` + `_2` `:150`), (3) rarity: type frequency
  within the engagement's own line distribution (a once-per-fight
  `CHARGE_COLLISION` outranks the fortieth `DODGE`). Quote 2-3 lines verbatim
  (`report.text`, continuation-stitched). Everything else becomes counts.
- **Bonus probe:** `world.status.slots` (`combat_event_listst`,
  `df.announcement.xml:141-147, :200`) is a 100-slot scratch buffer of
  `combat_report_event_type` enums (`SeveredPart`, `MajorArtery`, `Pulped`,
  `Unconscious`… `:90-130`) DF fills while composing strike lines. Nothing in
  DFHack reads it; if it survives observation windows it is a locale-free
  severity oracle. Live probe #2 (§5) — the design does not depend on it.
- **Precedent defects — do not copy** (all in
  `library/modules/EventManager.cpp`): unguarded `reports[idx]` deref before
  bounds check (`:1140`; the guarded loop at `:1107-1111` is the correct
  template); reciprocal-wound dedup bug (`:1199` tests the same key `:1192`
  inserted — defender counter-hits silently dropped); `reportStr.find("severed
  part")` used as a boolean (`:1238` — npos is truthy). Also: EventManager
  keys only on `COMBAT_STRIKE_DETAILS` and misses `_2`; we handle both.
  Sparring exclusion: skip units whose report ids sit in `reports.log[Sparring]`
  (EventManager skips Sparring at `:1090-1091`); `unit.flags2.sparring` is
  unreliable per the XML's own comment (`df.unit.xml:1367`).

**Verdict: EXISTS-IN-STRUCTS**, one opportunistic probe. Summary ~250-450
tokens; drill-down capped slice on demand.

### 2.4 Relationship edges — the social graph

Three tiers, cheapest first (all current-state unless noted):

1. **`unit.relationship_ids`** — fixed 9-slot int array indexed by
   `unit_relationship_type` (`library/xml/df.unit.xml:2732`, enum
   `:1572-1583`), holding **unit ids** (proof: `Buildings.cpp:355`,
   `family-affairs.lua:143-144`). Useful slots: Spouse, Mother, Father,
   PetOwner. **The enum values past `NUM` (Lover, Friend, Grudge, Worship,
   Sibling…, `:1584-1623`) are vocabulary for other structures, NOT array
   slots** — a key trap the schema comment spells out (`:1583` "end of simple
   types, rest used elsewhere").
2. **`histfig.histfig_links`** (`df.history_figure.xml:1070`) — 17 subtypes
   (`:963-996`): mother/father/spouse (+former/deceased — historical),
   child, **deity** (`histfig_hf_link_deityst` `:975`, devotion =
   `link_strength` `:945`; scale ~0-100, "1 = casual worshipper" per
   `scripts/armoks-blessing.lua:112`, prayer-need bands 24/49/69/89 in
   `set-need.lua:318-328` — exact banding is live probe #5), **lover**
   (`:977`), master/apprentice, companion, pet owner. ~50 hash lookups +
   a few hundred vtable checks for a full fort. **NOT FOUND: friend or grudge
   link types** — this is the decisive negative.
3. **`histfig.info.relationships.hf_visual`**
   (`df.history_figure.xml:810, 773-794`) → `relationship_profile_hf_visualst`
   (`:494-517`: `histfig_id`, `attitude`/`counter` parallel vectors, `rank`,
   `core`, `meet_count`, `last_meet_year`) — the ONLY friend/grudge source.
   **The bond category is computed, not stored**: `core_hf_relationshipst`
   (`:482-487`) holds `loyalty/respect/fear/love/trust`, and the XML comment
   on `love` (`:486`) documents DF's own UI banding verbatim: *"-100: Pure
   Hate, LE -75: Hated, LE -50: Disliked, LE 49: Acquaintance, LE 74: Friend,
   LE 99: Close Friend, 100: Kindred Spirit"* (and `:483` confirms only Love +
   familiarity drive the description). `rank` (`vague_relationship_type`,
   `df.history_figure.xml:2-30`) adds flavor categories with explicit
   `grudge`/`jealous_relationship_grudge`/`supernatural_grudge` members plus
   `childhood_friend`, `war_buddy`, `athletic_rival`, `lover`,
   `shared_entity` ("Religion/PerformanceTroupe/MerchantCompany/Guild",
   `:29`). Scan cost: O(sum of profile counts), filtered against the citizen
   hfid set — thousands of entries for a mature fort, still cheap plugin-side.
   The `:775-788` comment block mixing "Buddy/Grudge" into `attitude` is
   **internally inconsistent with the enums** (reputation_type has no such
   members) — treat as stale; trust `core.love` + `rank`.

Cheap shortcut worth probing: `historical_figure.vague_relationships`
(`relationship_quick_infost`, `:767-770, :1073`) — a top-6
(hfid, vague_relationship_type) summary per figure. Nothing in DFHack reads
it and its fort-mode freshness is unknown (probe #4); if live, it IS the
edge list for free.

Worship edges: deity links from tier 2, target resolved via
`historical_figure::find` → name translation (deities are histfigs).

**Verdict: EXISTS-IN-STRUCTS.** ~1 line/edge, dedup A<B, cap ~40 edges with
kind filters and a truthful clamp note. Tantrum-spiral early warning = the
pulse's `FormedGrudge` events (§2.2) cross-referenced with these edges.

### 2.5 The fort's own art

Registries (all `df.global.world.*`, `library/xml/df.world.xml:545-557`):
`written_contents`, `poetic_forms`, `musical_forms`, `dance_forms` — each
handler has `all` + `order_load` only (**no `bad` vector**; `order_load` is
marked `has-bad-pointers` — never touch it; iterate `.all`, proven by every
in-tree consumer).

Readable content per struct (key fields):

- **`written_content`** (`library/xml/df.written_content.xml:2-24`): `title`
  is a **raw stl-string** (`:5` — no translation needed), `author` histfig
  (`:22`), `author_roll` quality (`:23`), `type` (27-value
  `written_content_type`, `df.d_basics.xml:2250-2277`: Poem, MusicalComposition,
  Choreography, Chronicle, Autobiography, Essay, Atlas…), `styles` +
  `style_strength` (`written_content_style` `:2280-2299`: Cheerful, Depressing,
  Vicious, Witty, Ranting…), `refs`+`ref_aux` (roles: Subject, Narrator,
  MoralLesson… `:2309-2320`).
- **`poetic_form`** (`library/xml/df.poetic_form.xml:218-250`): `name` is a
  `language_name` (`:221`), `originating_entity` (`:222`), `original_author`
  (`:223`), `mood` (Narrative/Dramatic/Riddle/Solemn… `:39-47`), **`subject`**
  (`:60-83`: Past, SomeoneRecentlyDeceased, Nature, Lover, AlcoholicBeverages,
  War, Hunt, Mining, Death, Immortality…) with `subject_target` union (`:51-58`:
  histfig OR sphere concept), `action` (Describe…Beseech `:85-109`).
- **`musical_form`** (`library/xml/df.musical_form.xml:250-281`): `name`,
  `originating_entity`, `original_author` ("the composer"), `purpose`,
  `devotion_target` histfig, links to poetic form/written content.
- **`dance_form`** (`library/xml/df.dance_form.xml:223-260`): `name`,
  `originating_entity`, `original_author`, and — pure narrative gold —
  `event` "Event the dance acts out" (`:247`) + `hfid` "Character whose story
  the dance acts out" (`:248`) + `race` "Creature whose movements are
  imitated" (`:249`). *A dance imitating the carnotaurus is a struct read.*

**Name resolution — proven server-side**: `Translation::translateName`
(`library/modules/Translation.cpp:172-259`) is type-agnostic and used on form
names in-tree (`scripts/assign-preferences.lua:49-53`). (The capitalized
`Translation::TranslateName` is dead code surviving only in two
commented-out-of-the-build plugins — do not reference it.)

**Fort-local filter — the one caveat**: **NOT FOUND: any site field on any of
the four structs** (each read in full). The authoritative "composed HERE"
signal is the four history-event classes, which all carry `site`:
`history_event_poetic_form_createdst` (`library/xml/df.history_event.xml:1456-1469`),
`musical_form_createdst` (`:1472-1485`), `dance_form_createdst` (`:1488-1501`),
`written_content_composedst` (`:1504-1517`); type enum members `:99-102`.
Match `event.site == df.global.plotinfo.site_id`
(`library/xml/df.plotinfo.xml:894`; usage pattern `scripts/list-waves.lua:106`).
`world.history.events` (`library/xml/df.history.xml:236`) is append-only —
one full scan at first call, then tail-cursor (the announcement-cursor pattern
at world-history scale). Complementary filter that needs no events at all:
`author/original_author ∈ citizen-hfid set` — catches fort-composed works
even if the event linkage surprises us, and pre-fort works BY current
citizens (also interesting: "olon brought this song here"). Entity-level
proxies (`historical_entity.performed_poetic_forms` etc.,
`library/xml/df.entity.xml:1653-1658`) conflate "performed here" with
"composed here" — usable as a labeled fallback only.

**Verdict: EXISTS-IN-STRUCTS** with the event-join caveat handled.
~150-300 tokens/call.

---

## 3. Proposed tool surface

Respecting the schema budget (repo:internal/mcpserver/schema_budget_test.go —
3KB/tool ceiling, 1600-byte name tax; ~84 tools live): **two new tool names,
one parameter added to an existing tool, six string-named JSON plugin queries,
zero wire-protocol changes.** All queries dispatch through the existing
`executeQuery` exception barrier (repo:dfhack-plugin/queries.cpp:3785-3872).

### 3.1 `dwarf_detail` + `portrait=true` (no new tool name)

- **Plugin query** `dwarf_portrait {id}`: single-unit read emitting salient
  structured JSON — identity (`getReadableName`, caste description line, size
  band), top non-neutral facets with tier indices, extreme values, starved/fed
  needs (+trait→need composition hints), strongest recent emotion with
  caption + resolved-or-raw subthought, preferences (visible-flag, ranked
  LikeMaterial → LikeFood/LikeItem → flavor, cap 3), deity + devotion band,
  relationship subset (spouse/lover/best-friend/grudge/children resolved to
  living citizens by name), history hook (troupe/religion memberships current
  + former, known-forms counts, masterpiece/kill counts), stress tier.
  Salience thresholds live plugin-side as named constants (the
  `classifyWildlifeDanger` pattern).
- **Go side**: `portrait=true` on the existing tool renders prose-ish lines
  from tier tables (the phrasing lives in Go code — legal under the house
  rules; skills/schemas stay clean of specifics). Existing `include_*`
  mechanical views unchanged.
- **Budget**: 250-450 tokens. One dwarf, one call.

### 3.2 `fort_story` (new tool, modes)

One name, three fort-wide narrative surfaces (the `look` scope/lens
precedent):

- **`mode=pulse`** (default) → plugin query `story_pulse {max?}`: emotions
  fort-wide since the plugin-side cursor, salience-ranked (§2.2), dedup by
  remembered set, hard cap 12-15 lines each "who felt what, why", truthful
  clamp + `N below threshold` note. Cursor advances on read; reconnect resets
  with a bounded re-window (announce the re-window honestly, the
  reconnect-resend precedent).
- **`mode=social`** → plugin query `social_graph {unit?, kind?}`: edge list,
  citizens-only both ends (worship edges excepted), 1 line/edge
  (`A — B: Close Friend (love 82, met 31×)`), family collapsed to typed edges,
  cap ~40 with kind filters (family/friend/grudge/worship) and clamp note.
- **`mode=art`** → plugin query `fort_art {}`: fort-composed works (event-join
  + author-filter union, each labeled with its provenance path), title/form/
  creator/subject/mood per line, plus a one-line census (works known in fort
  vs composed in fort). Tail-cursored event scan; first call pays the full
  scan once.
- **Budget**: pulse 200-350; social 300-450; art 150-300 tokens.

### 3.3 `combat_report` (new tool, modes)

- **`mode=summary`** (default) → plugin query `combat_summary {}`: per
  engagement (cap 3, most recent first): sides by name (roster from
  `unit.reports.log` reverse index; conflict-activity sides if probe #1
  lands), computed momentum (hits landed/taken by side, severity-class
  tallies, `peak/current_strength` when reachable, bleeding/KO/casualty
  lists), the 2-3 most telling lines quoted verbatim (§2.3 ranking), and a
  drill-down pointer. Wound watermarks snapshotted at step start for
  fight-involved units.
- **`mode=log`** → plugin query `combat_log {engagement?|unit?, last?}`:
  windowed raw slice, continuation-stitched, cap ~30 lines with clamp note.
  Explicit-request-only by design.
- **Cursors**: independent max-id cursor over `world.status.reports`
  (binsearch + guarded advance, the EventManager `:1107-1111` template);
  never deletes reports; coexists with announcements.cpp untouched.
- **Budget**: summary 250-450; log ≤ ~500 tokens.

### 3.4 What stays out (deliberately)

- No step-report integration in the first cut — `step` already trips on
  critical combat announcements; the narrator is the *next call* after a trip,
  not more step payload. Revisit a 1-line pulse teaser only after live
  experience shows it earns its recurring cost.
- No full-appearance prose composer (§2.1e), no per-figure biography scan, no
  memories/`personality_memory_handlerst` surface, no world-scale legends
  reader — all future candidates, none needed for the five desires.
- No new alwaysLoad tools; both new names are deferral-friendly.

**Schema cost estimate**: 2 names ≈ +25 name-tax bytes (ceiling 1600, measured
932); each tool well under 1KB serialized (modes + 2-3 params each) vs the
3KB ceiling. No exceptions needed.

---

## 4. Suggested wave scope — first slice

**Endorse the overseer's vote: portrait card + combat narrator skeleton**, and
the struct research backs it concretely: a siege can arrive any season and the
combat pipeline has the one live probe (`activity_id`) we want answered before
we lean on it; the portrait exercises the caption+subthought resolver and
citizen-hfid machinery that pulse/social/art all reuse.

**Wave 013-A (minimal, ship-before-a-season-boundary):**

1. Plugin: shared helpers — citizen-hfid set + hfid→unit map; thought-caption
   + curated subthought resolver (portrait & pulse both consume it).
2. Plugin: `dwarf_portrait` query (§3.1). Go: `portrait=true` render.
3. Plugin: combat ingest skeleton — reports cursor (guarded binsearch),
   continuation stitch, `UNIT_COMBAT_REPORT` classification, per-unit roster
   index, wound watermarks; `combat_summary` with wound-delta momentum v1 +
   telling-line ranking; `combat_log` slice. Conflict-activity join and
   `world.status.slots` behind live probes — landing them is a follow-up
   commit, not a blocker.
4. Go: `combat_report` tool (both modes). Schema-budget test green.

**Wave 013-B:** `fort_story` pulse + social (cursor + remembered set + edge
walk). **Wave 013-C:** `fort_story` art (event tail-scan + registries) +
portrait history-hook enrichment (known-forms counts, troupe lines) if any of
it slipped from A.

Verification timing: portrait/pulse/social/art are live-verifiable any tick
on Fort #6's 19 citizens (edóm/olon/zefon/deduk are the test cases the
overseer already named). Combat verification needs a fight — wildlife
skirmish or the first siege; the summary must degrade truthfully ("no combat
reports since cursor") until then.

---

## 5. Open questions needing live verification

1. **`report.activity_id` population** on fort-mode combat lines (writes -1
   from DFHack's own paths; vanilla behavior unobservable in source). One
   log-line probe during the first real fight decides conflict-join vs
   intersection-window grouping.
2. **`world.status.slots` lifetime** (`combat_event_listst`): does the
   structured severity scratch survive until our poll, or is it overwritten
   per strike? Nothing in DFHack reads it; we'd be first.
3. **Emotions-vector growth**: do `personality.emotions` get pruned over
   fort-years, or grow monotonically? Bounds the pulse scan and the portrait's
   `emotions_total`.
4. **`vague_relationships` freshness** in fort mode (top-6 quick info,
   `df.history_figure.xml:1073`): populated and current for ordinary citizens,
   or legends/worldgen-only? If live, social edges get a free fast path.
5. **Deity `link_strength` scale**: confirm the 0-100 reading and pick honest
   band labels (armoks-blessing's "1 = casual" vs set-need's 24/49/69/89
   bands).
6. **Subthought semantics probe**: for the highest-frequency thoughts observed
   live (Argument/Complained/deaths/prayer), log raw subthought + context and
   grow the curated resolver only from confirmed pairs.
7. **`artistic_profilest.events` / `top_related_heid` contents**: do
   form-creation events land there (making per-dwarf art history O(1)), or is
   the world-event tail-scan the only path?
8. **`originating_entity` for fort-composed forms**: site government vs
   parent civ — decides whether the entity proxy is a usable art filter or
   stays labeled "performed here."
9. **`MadeFriend`/`FormedGrudge` subthought**: does it hold the other party's
   hfid? If yes, the pulse names both ends of a forming edge for free.
10. **World-event scan cost on this DINO world**: measure the one-time
    `world.history.events` walk (likely milliseconds; confirm before making
    `fort_art`'s first call pay it silently — if slow, note it in the ACK).

---

*Repo patterns this design deliberately reuses: max-id cursor + bounded
repeat/remembered windows (repo:dfhack-plugin/announcements.cpp,
queries.cpp:1902-1968); salience classifier with truthful reason strings
(queries.cpp:1816-1843); caps with truthful clamp fields
(queries.cpp:1523-1524); one-pass citizen scans (queries.cpp:1294-1333);
Go-side typed parse → compact gated render (repo:internal/mcpserver/
tools_state.go:2035-2160); the executeQuery exception barrier
(queries.cpp:3790-3870). Nothing here touches the wire protocol, the
viewscreen stack, or the announcement ticker pipeline.*
