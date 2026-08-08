# Mid-Game Goals Research — what players actually do once survival is trivial

Feature 014 · researched 2026-08-07 · written for the playing AI at **Lanehold** (year 101, ~20 dwarves, ~24k created wealth, zero deaths, sealed single entrance, iron industry, grand tavern, noble quarter, soil aquifer pierced and sealed at z=135 in year 100 — the DEEP aquifer at z=127 and below is UNPIERCED and confirmed wet at every bore column — DINO world).

**Grounding note.** Mechanics below are grounded in the DF wiki (v50.x / Steam-era pages, listed in Sources) plus long-standing community practice. The live game is DF 53.15, which post-dates this research's sources; every mechanic cited here has been stable since v50, but where a fact is version-sensitive or unverified it is marked **[UNCERTAIN]**. Nothing below is invented; if it says a number, the wiki says that number.

**How to use this doc.** Sections 1–2 and 5 are project menus (each card: what / why / cost / risk for THIS fort / stream value). Sections 3–4 are mechanics references that explain WHY the projects pay off. The shortlist at the end is pre-filtered for Lanehold's exact state, ordered by payoff-per-risk, each with a first concrete tool-level step.

---

## 0. Read this first: why Lanehold stalled, in one paragraph

Migrant waves 3+ are sized by **created wealth as reported by the last outgoing dwarven caravan** — the autumn caravan must arrive, trade, and *leave the map alive* for the mountainhomes to hear about you, and wealth created after it leaves counts only next year. Exported wealth and deaths modify the number; imported wealth does almost nothing. ~24k created wealth is genuinely low for a year-2 fort (healthy forts report 100k+). The human's read is mechanically correct: **under-building IS the migration problem**, because architecture, smoothing, engraving, furniture, and crafts all *are* created wealth. Every project in this document raises created wealth as a side effect; several raise fame (visitors) and happiness at the same time. Also check the DF settings population cap — if it's set at or near 20, nothing else matters. Details in §4.

---

## 1. GREATER WORKS — multi-year projects with real payoff and drama

### 1.1 Well + cistern (the gateway water project)
- **What:** A sealed cistern filled from an aquifer tap — the already-pierced-and-sealed SOIL aquifer at z=135 is the proven source; the deep aquifer (z=127 down, unpierced, confirmed wet at every bore column) is a second, riskier option — with a well shaft over it serving the hospital and a backup drink source.
- **Why:** Hospitals need water to clean patients and wounds; a well means no dwarf hauls water from outside during a lockdown, and wells/cisterns are the tutorial level for every bigger water project. Well components (bucket, chain/rope, mechanism, blocks) add value.
- **Cost/prereqs:** One channel/shaft down to aquifer level, a dug cistern chamber, a well built above it, bridges as shutoffs. Days of work, not seasons.
- **Risk @ Lanehold:** LOW-MODERATE. The aquifer is already pierced and sealed; the fix for a mistake is the same bridge-and-lever doctrine already live-verified twice. Tooling caveat: the `aquifer-water-infrastructure` skill is written but *no shutoff has ever been tested against actual flowing water*, and `build type=well` is compile-verified only. Treat the first flow as a live experiment with a pre-built raising bridge upstream.
- **Stream value:** Moderate — but it unlocks 1.2, which is high.

### 1.2 Waterfall / mist generator in the tavern or grand dining hall
- **What:** Water falling through the dining/tavern level (over floor grates, into a drain) generates **mist**. Dwarves walking through mist get a strong happy thought and are cleaned of contaminants. The classic forms: (a) divert real falling water through the hall, or (b) a closed loop of screw pumps ("mist generator") recirculating the same water, dropping it one z-level each cycle through a high-traffic tile.
- **Why:** Permanent, passive, area-of-effect happiness in the room every dwarf visits daily. It is *the* iconic "we have transcended survival" build.
- **Cost/prereqs:** Screw pumps (block + enormous corkscrew + pipe section each), grates, a drainage plan, and **power or pump operators** — screw pumps can be dwarf-operated, but a permanent mist generator wants a windmill/waterwheel + axle/gear train, which is its own sub-project (windmill output depends on region wind — build one to test; waterwheels need flowing water). Water must start ABOVE the hall: from the aquifer that means a pump stack up, or hand-filled pond zones priming a closed loop. This is a season-scale engineering project with 3 staged milestones (cistern → powered pump loop → mist through the hall).
- **Risk @ Lanehold:** MODERATE. Flooding the tavern is the failure mode; mitigate with raising bridges as shutoffs (verified) rather than floodgates (crafting bug fixed but **not live-verified**). [UNCERTAIN] aquifer tiles are reported to also *absorb* water, making the aquifer layer usable as a drain — verify live with a bucket before relying on it.
- **Stream value:** VERY HIGH. Visibly dramatic, mechanically meaningful, real failure stakes.

### 1.3 Controlled cavern breach
- **What:** Dig a dedicated 2×2 stair probe (away from living areas) down until `cross_section` shows a cavern layer; breach behind an airlock — bridge + lever, sealed corridor — so the caverns can be opened and shut at will.
- **Why:** The single biggest content unlock in the game: underground wood (tower-caps/fungiwood) ends log scarcity forever; cave moss/floor fungus enables underground grazing; spore-seeded muddy stone enables underground farming of all crops; web farming for silk; AND two mechanics that matter to Lanehold specifically — **monster slayer visitors only start arriving after you breach a cavern** (§2.4), and **revealed subterranean tiles raise the artifact cap** (§2.1). It is also the road to the magma sea (1.4).
- **Cost/prereqs:** Days of digging + one bridge/lever airlock. Essentially free.
- **Risk @ Lanehold:** MODERATE, and *controllable*. Cavern wildlife (crundles, trolls, cave crocs — and in this world, who knows what dino-adjacent horrors generate down there) can path in while open; Forgotten Beasts arrive over time and some are genuinely fort-threatening. The airlock reduces this to "close the door." Flying FBs are the reason the breach point must not open directly into the main stair spine.
- **Stream value:** VERY HIGH. Fog-of-war reveal, new biome, monsters, ongoing incident generator.

### 1.4 Magma sea descent + magma industry
- **What:** Dig past the (usually 2–3) cavern layers to the magma sea; build magma forges/smelters/glass furnaces/kilns over channeled magma (4/7+ under the building's fire tile). Either move heavy industry down there or (advanced) haul magma up in minecarts.
- **Why:** Ends fuel dependence permanently — the iron industry currently pays a charcoal/coke tax on every bar. Magma industry is what turns "first iron armor" into "steel by the ton." Enormous created-wealth multiplier.
- **Cost/prereqs:** Requires 1.3 first (you'll cross the caverns). A full workshop relocation or a long haul corridor; magma-safe materials for anything that touches magma (iron/steel mechanisms, no wood).
- **Risk @ Lanehold:** MODERATE-HIGH. Fire imps/magma crabs snipe miners at the sea; a mis-channeled tile floods a corridor with magma (no undo). Stage it: probe now, industry move next year.
- **Stream value:** HIGH. "The descent" is a multi-episode arc with a glowing payoff.
- **NOTE — hard line:** the magma sea means adamantine spires will appear. Locate them, mine the exposed tip if desperate for a mood material, but **do not dig deep into a spire**. See §5.6.

### 1.5 Glass industry
- **What:** Sand (collected in bags at a sand zone) + fuel (or a magma glass furnace) → green glass, an infinite-material craft/trade/furniture line; clear/crystal glass adds pearlash chains.
- **Why:** Infinite renewable wealth without mining; glass windows/portals are high-value and pretty; serrated glass discs feed weapon traps.
- **Cost/prereqs:** REQUIRES SAND ON SITE — unknown at Lanehold; survey first. Trivial buildings.
- **Risk:** NONE. **Stream value:** Moderate (industry porn, glass architecture).

### 1.6 Grand staircase / architectural retrofit
- **What:** Rebuild the utilitarian 2×2 spine into a monumental core: widen landings, smooth/engrave every wall, statue niches, symmetric branches (the overseer's design language: symmetry off the spine, chamfered transitions, vein pillars kept as architecture).
- **Why:** Smoothing and engraving are free value (= migration math, §4) and free happiness (§3); engravings record the fort's own history on its walls (§2.7).
- **Cost:** Labor only. **Risk:** LOW — but **smoothing destroys carved stairs/ramps** (burned twice: Forts #4 and #5); check every smooth rect against the shaft. **Stream value:** Moderate-high (before/after is very visual).

### 1.7 Walled surface compound + tower
- **What:** A curtain-walled courtyard over the entrance: safe pasture, above-ground farming, archer deck on the gatehouse, eventually a proper tower.
- **Why:** Surface access without surface risk — in a dino world the surface is the wildlife show; a walled paddock is also the prerequisite for keeping tamed dinos (§2.9) where the stream can see them. Constructions are big created wealth.
- **Cost:** Lots of blocks (mason time), season-scale. **Risk:** LOW-MODERATE — builders are exposed while walling; schedule around wildlife alerts and keep the drawbridge as the fallback. **Stream value:** HIGH in this world specifically (dinosaurs vs. walls).

### 1.8 Minecart network / quantum stockpiles
- **What:** Minecart routes for bulk hauling; a track-stop that dumps into a 1-tile stockpile ("quantum stockpile") collapses hauling overhead.
- **Why:** 20 dwarves waste enormous time hauling; QSPs are the classic fix.
- **Risk:** LOW (route dwarves can be run over; comedy, rarely tragedy).
- **TOOLING GAP:** the MCP server has **no hauling-route/track-stop/dump-item tools** today. This project needs a feature wave before the AI can play it. Park it; note it in goals as a tooling request.

### 1.9 Obsidian farm
- **What:** Cast water onto magma (or vice-versa) to mass-produce obsidian; craftable into high-value rock short swords (1 obsidian + 1 log each) and endless building stone.
- **Why:** Renewable weapons-grade wealth; the intersection of the water and magma skill trees — a "we have mastered both elements" flex.
- **Cost/prereqs:** Requires 1.2-tier water control AND 1.4 magma access. Year-3+ project.
- **Risk:** MODERATE-HIGH (both fluids, same room). **Stream value:** HIGH, late-game arc.

---

## 2. EMERGENT / CHARACTER GOALS

### 2.1 Artifacts & strange moods — and how to farm them
- **Mechanics (verified):** Moods require **at least 20 dwarves** — Lanehold sits exactly at the threshold; dip below 20 and moods stop. Max artifacts = **min(items created ÷ 100, revealed subterranean tiles ÷ 2304)**. A compact 10-cell fort that crafts a lot is almost certainly capped by *revealed tiles* — exploratory mining and cavern breaching (1.3) literally buy more artifacts. The mood takes a dwarf with a "moodable" skill (highest such skill wins the workshop); a dwarf with none becomes a bone/stone/wood crafter.
- **Farming the conditions:** keep pop ≥ 20; keep one of each demandable material in stock — **logs, stone, bars, rough gems, cut gems, cloth (silk/plant/wool separately), leather, bones, and SHELLS** (the classic killer: no shells on hand → dwarf goes insane). Shells come from fishing (mussels/turtles) — Lanehold should keep a fisherdwarf active for this reason alone.
- **Payoff & drama:** an artifact is permanent wealth + a permanent named object + a **legendary dwarf** (instant master craftsman). A *failed* mood is a berserk or melancholy dwarf — at 20 pop that's both a tragedy and a mood-system shutdown. Both outcomes are excellent television.

### 2.2 Museum (new-ish, cheap, very Lanehold)
- **What:** Pedestals and display cases; assign items (especially artifacts and masterworks) to them; a meeting area over display furniture is a **museum**. Dwarves who pass through the furniture tile admire both the item and the case — happy thoughts; displayed items add room value.
- **Drama:** displayed artifacts attract **villainous theft plots** (agents sneak in to steal them). That is a feature, not a bug, for a stream: a whodunit arrives by itself.
- **Cost:** A smoothed hall off the tavern + a few pedestals. **Risk:** LOW (worst case: an item is stolen and the fort gets a nemesis). **Stream value:** HIGH per ☼ spent.

### 2.3 Temples, guildhalls, libraries — the petition endgame
- **Temples:** Prayer is one of the most common unmet needs (§3). Any zone assigned as worship space satisfies it initially; once ~enough dwarves share a deity they **petition** for a dedicated temple of **2000☼ value**; later, priesthood recognition and grand-temple (10000☼) follow-ups. Cheap (smoothing + engraving + a few statues clears 2000☼ fast) and directly reduces stress fort-wide.
- **Guildhalls:** When enough dwarves practice related professions a guild forms and petitions for a **2000☼** hall, later a **10000☼ grand guildhall**. Guildhalls host skill-sharing demonstrations — free training. With ~20 dwarves Lanehold may be just under guild-formation counts; migration success feeds this.
- **Libraries:** paper (plant slurry → sheets, or parchment) → quires → codices; a library location with bookcases, tables, chairs, writing materials. Citizen scholars *generate* knowledge, scribes copy it, **visiting scholars** bring foreign knowledge and petition for residency. Books are named lore objects ("the fort's first book" is a stream moment). Satisfies the "learn something" need.
- **All three raise fort fame → visitors → §4's second population channel.**

### 2.4 Tavern as an engine (visitors → petitioners → citizens)
Already built (Drakehall) — now *work* it: assign tavern keeper/performer occupations, stock instruments (some are multi-part assembled builds) and goblets, rent rooms. Performances raise fame; bards, mercenaries, and (post-cavern-breach) **monster slayers** arrive, petition for **long-term residency**, and after two years of residency petition for **citizenship** — the migration-independent way to grow population. A monster slayer petition is the game handing the stream a named recurring character who fights things in your caverns for free.

### 2.5 Barony and the noble ladder
With pop ~20 and rising wealth the outpost liaison can offer to elevate the fort to a **barony** (baron → count → duke as wealth/exports grow). Brings mandates, room demands (the two-level noble quarter is ready for exactly this), prestige, and story. [UNCERTAIN] exact wealth/pop thresholds in current version — accept the offer when the liaison makes it; keep the liaison alive and *conclude the meeting* every autumn.

### 2.6 Named heroes and the militia arc
Dwarves earn epithets and titles through kills; a champion with a named weapon is self-writing lore. Squad tooling exists (create_squad/squad_order — flagged UNVERIFIED in Wave 5). Sparring in a proper barracks is safe and effective in v50 (danger rooms are not — §5.4). At 20 pop: one squad of 4–6, train always, fight rarely.

### 2.7 Engrave the fort's own history
Engravings depict events from the fort's and civilization's history — the walls literally become the chronicle (masterwork engravings are viewable named art). Smooth everything, then engrave high-traffic halls with the best engraver only (quality = value = happiness). [UNCERTAIN] whether current versions allow *choosing* engraving/statue subjects — assume no; the randomness is the charm.

### 2.8 Death, burial, and memorial architecture
Zero deaths so far means zero grief-handling infrastructure has ever been tested. Pre-dig a mausoleum: coffins in individual tombs (assignable), slabs ready to engrave (a memorial slab placates the ghost of anyone whose body is lost). When the first death comes — and mid-game projects make sure it will — the difference between "a funeral" and "a haunting plus a grief spiral" is whether this was built in advance. A grand mausoleum is also pure value and pure character.

### 2.9 Dinosaur taming & breeding programme (THIS world's signature)
- **What:** Cage traps on surface game trails → trainer tames captives (species trainability depends on this world's raws — test empirically) → pasture in the walled compound (1.7) → breed if you catch a pair (egg-layers need nest boxes); train war/hunting animals where the species allows.
- **Why:** It's a DINO WORLD. A war-trained ceratopsian at the gate is the single most stream-legible achievement available. Also: tame animals = created wealth; cage-trapped hostiles = tradeable or arena stock.
- **Cost:** Mechanisms + cages + a trainer + pasture. **Risk:** LOW-MODERATE — cage traps trivially catch most wildlife (building-destroyer megafauna excepted); training decays back toward wild if not maintained (re-train on schedule).
- **TOOLING CHECK:** no explicit animal-training/pasture-assignment tools are visible in the MCP list beyond zones — verify what `designate_zone`/`assign_zone` can express before promising this arc on stream.

---

## 3. THE HAPPINESS / NEED ECONOMY (mechanics reference)

**Needs, not just food:** each dwarf carries personal needs — pray (per deity), socialize, drink alcohol (variety matters), eat a good meal, be with family/friends, craft an object, acquire an object, **be extravagant**, learn something, hear music/see performance, wander/be outside (rare), martial training (soldiers). Unmet needs → *distracted* → worse focus → worse work quality → stress. Chronically stressed dwarves break: tantrums (fistfights, smashed furniture), depression, obliviousness, or **berserk** — a berserk legendary soldier can end a 20-dwarf fort by itself. That is what "a bored dwarf" does.

**Levers, cheapest first:**
1. **Temple zone** (satisfies prayer immediately; upgrade per §2.3).
2. **Meal quality** — lavish prepared meals from varied ingredients ("ate a pretty decent meal" scales with value); keep 4+ drink types.
3. **Room value** — quality tiers from Meager up to Royal; value = smoothing + engraving quality + furniture value + material value. Dining in a high-value hall is a recurring happy thought → the **legendary dining room** is the classic single highest-leverage build. Owned bedrooms already exist; raising their value (engraving, better furniture) upgrades the daily "slept in a good bedroom" thought.
4. **Belongings** — dwarves claim clothes and trinkets; "acquire object" and "be extravagant" are both satisfied by fresh (ideally masterwork) clothes.
5. **⚠ THE YEAR-2 CLOTHING TRAP:** embark clothes rot off on a schedule that hits almost exactly NOW (18–24 months in). "Wore tattered clothing" is a repeating stress thought, and naked dwarves spiral. **Lanehold needs a clothing industry (pig tail/wool/leather → clothier) or bulk clothing imports THIS YEAR regardless of what else is chosen.** This is the most likely invisible time bomb in the fort.
6. **Mist** (1.2), **art everywhere** (statues, engravings — admiring art is a thought), **music in the tavern**, **waterfall/mist**, **displayed masterworks** (2.2).
7. **Craftsdwarves need to craft** — profiled workshops + repeating manager orders keep the "craft object" need fed and the wealth counter spinning at once.

---

## 4. POPULATION & MIGRATION (mechanics reference)

- Waves 1–2: hardcoded, small, wealth-independent.
- Waves 3+: sized primarily by **created wealth as reported by the last dwarven caravan that left the map alive**; exported wealth and deaths apply modifiers; imported wealth ~irrelevant. Wealth created after the caravan departs counts next year (a one-year lag — last season's zero migrants reflects the fort as the *first* caravan saw it, when it was much poorer).
- **Failure modes:** caravan killed/never left → no report → no migrants; population cap reached in settings; parent civilization dead (only hardcoded waves ever). Curiosity from the wiki: trading away literally 100% of created wealth zeroes the next wave — trade generously, not totally.
- **Action translation for Lanehold:** (a) verify the pop cap in DF settings; (b) spend spring–summer maximizing *created* wealth (architecture, engraving, masterwork crafts — §1.6 and §3 do double duty); (c) when the autumn caravan comes: trade a healthy surplus (exports modify upward), fulfill trade agreements, keep the caravan and liaison alive, finish the liaison meeting; (d) run the visitor→resident→citizen channel (§2.4) in parallel — it ignores migration math entirely.

---

## 5. RISK-SEEKING FUN — ranked for a 20-dwarf year-2 fort

| Idea | Verdict NOW |
|---|---|
| 5.1 Cavern breach behind an airlock | **DO IT** — controllable, unlocks half of §1–2 |
| 5.2 Dino taming/breeding | **DO IT** — cage traps make it safe-ish, signature content |
| 5.3 Magma probe (locate the sea, no industry yet) | **DO IT CAREFULLY** — staged |
| 5.4 Danger room | **NO** — in v50 training spears maim and kill even through steel armor; sparring is safer AND effective; also reads as an exploit on stream |
| 5.5 Provoking goblin sieges | **NOT AVAILABLE** — default siege trigger is ~pop 80 (ambushes/thieves earlier; intensity scales with sieges survived, and triggers are adjustable in difficulty settings — an off-stream lever, not a play action). Werebeasts, however, arrive regardless of pop: keep cage traps at the gate and a quarantine protocol for bitten dwarves (a bitten survivor is a ticking narrative bomb — arguably the best low-pop drama in the game) |
| 5.6 Adamantine / the circus | **ABSOLUTELY NOT** — breaching the spire hollows releases demons; a 20-dwarf fort is over in minutes. Locate, admire, tell the chat what's down there, do not dig |
| 5.7 Drowning chambers / invader-casting | **LATER** — no invaders to process yet |
| 5.8 "Dwarven water reactor" (pump-powered waterwheel perpetual power) | [UNCERTAIN] whether the exploit still works in current builds; also exploit-flavored — prefer honest windmills |

---

## RECOMMENDED SHORTLIST FOR LANEHOLD (payoff-per-risk order)

**Standing order regardless of choice: start the clothing industry (§3.5) and verify the settings pop cap (§4) this session.**

1. **Temple + needs pass** — near-zero risk, immediate fort-wide stress relief, first petition arc.
   *First step:* `wellbeing` (fort scope) + a few `dwarf_detail` calls to inventory unmet needs and shared deities → dig/smooth a chamber off the tavern → `create_location` temple → `zone_value` until ≥2000☼.
2. **Museum + strange-mood readiness** — near-zero risk, converts existing masterworks into daily happy thoughts, arms the artifact engine, invites a theft plot.
   *First step:* `stocks` for artifacts/masterworks + shell/cloth/gem/leather/log audit (fishing on if shells absent) → `building_types` for pedestal/display case → build 3–4 in a smoothed hall on the tavern's traffic path → meeting area over them. Keep pop ≥ 20 — moods switch off below it.
3. **The autumn wealth-export push** — the direct migration unblock; low risk, deadline-driven (built-in episode structure).
   *First step:* `trade_agreements` + `caravan_status` to see what the mountainhomes asked for → queue repeating masterwork craft/goblet orders and an engraving sweep of the main halls (`smooth` — check rects against stair shafts) → autumn: `bring_goods_to_depot`, trade generously, protect the caravan's exit.
4. **Controlled cavern breach** — moderate, managed risk; unlocks monster-slayer petitioners, underground wood/farms, the artifact-cap headroom, and the road to magma.
   *First step:* `cross_section` on existing bore columns to find cavern z → pick a breach site away from the living spine via `find_dig_site` → dig the 2×2 probe stair to one level ABOVE the cavern → build bridge + lever airlock (`build`, `link_building`, test `pull_lever`) → only then breach, paused, with the squad stationed.
5. **The water ladder: well → cistern → tavern mist generator** — the flagship Greater Work; staged so each rung pays off alone.
   *First step:* `look`/`cross_section` at the sealed z=135 soil-aquifer pierce to map the wet layer (the deep aquifer at z=127 and below is UNPIERCED — wet at every bore column — and would need its own deliberate, shutoff-guarded pierce; do not confuse the two) → dig a cistern chamber one level below the hospital with a raising-bridge shutoff BUILT AND LEVER-TESTED before any water moves → `build type=well` above it (live-verifying the well tool) → then, and only then, design the pump loop for Drakehall's mist (§1.2), bridges not floodgates, power plan decided before the first pump is placed.

*Next tier (pick up as the above land):* dino paddock + walled compound (§1.7 + §2.9), library & the fort's first book (§2.3), mausoleum (§2.8), magma industry (§1.4), glass if survey finds sand (§1.5).

---

## Sources

- [Strange mood — DF Wiki](https://dwarffortresswiki.org/index.php/Strange_mood) (20-dwarf minimum, artifact cap = min(items/100, revealed subterranean tiles/2304), moodable skills)
- [Petition — DF Wiki](https://dwarffortresswiki.org/index.php/Petition) / [Temple](https://dwarffortresswiki.org/index.php/Temple) / [Guildhall](https://dwarffortresswiki.org/index.php/Guildhall) (2000☼ / 10000☼ thresholds, priest recognition, residency petitions)
- [DF2014:Immigration — DF Wiki](https://dwarffortresswiki.org/index.php/DF2014:Immigration) (wave mechanics, caravan wealth report, no-migrant causes)
- [Visitor — DF Wiki](https://dwarffortresswiki.org/index.php/Visitor) / [Monster slayer — DF Wiki](https://dwarffortresswiki.org/index.php/Monster_slayer) (cavern-breach arrival condition, residency → citizenship)
- [Display furniture — DF Wiki](https://dwarffortresswiki.org/index.php/Display_furniture) / [Museum — DF Wiki](https://www.dwarffortresswiki.org/index.php/Museum) (museum rooms, admire-on-tile, artifact theft)
- [Siege — DF Wiki](https://www.dwarffortresswiki.org/index.php/Siege) / [DF2014:Ambush](https://dwarffortresswiki.org/index.php/DF2014:Ambush) (pop ~80 siege trigger, intensity scales with sieges survived, difficulty-settings triggers)
- [Danger room — DF Wiki](https://dwarffortresswiki.org/index.php/Danger_room) (v50 lethality; sparring preferred)
- [Mist — DF Wiki](https://dwarffortresswiki.org/index.php/Mist) / [DF2014:Waterfall](https://dwarffortresswiki.org/index.php/DF2014:Waterfall) (mist happy thought, generator designs)
- [Need — DF Wiki](https://dwarffortresswiki.org/index.php/Need) / [DF2014:Stress](https://dwarffortresswiki.org/index.php/DF2014:Stress) / [Keeping your dwarves unstressed](https://dwarffortresswiki.org/index.php/DF2014:Keeping_your_dwarves_unstressed) (needs list, distraction→focus, extravagance via masterwork clothes)
- [DF2014:Room](https://dwarffortresswiki.org/index.php/DF2014:Room) / [Dining room — DF Wiki](https://dwarffortresswiki.org/index.php/Dining_room) (room value composition, legendary dining rooms)

---

## PART II — Community culture, megaprojects, and weird play

Second research pass, 2026-08-07. Part I covered wiki mechanics; this part covers what the COMMUNITY actually does with them — the famous forts, the megaproject tradition, the signature contraptions, themed play, and streamer craft. Method note: each claim is tagged by source class — **[wiki]** = documented on dwarffortresswiki.org (main namespace = current 50.x/53.x era unless noted; `DF2014:` pages = 0.40–0.47 era, migrated content that the wiki itself flags "may be inaccurate for the current version"), **[forum/community]** = a forum thread, fan wiki, or LP archive claims it, **[UNVERIFIED]** = could not confirm. Version flags matter: several beloved tricks predate v50 — do not chase a dead exploit live on stream.

---

### 6. Famous forts and what made them stories

The pattern across all of these: the fort is remembered for **one legible thing** (a device, an enemy, a premise) plus **a written chronicle with named characters**. The game supplied events; the players supplied the framing.

#### 6.1 Boatmurdered (Something Awful succession, 2D-era DF, ~2006–07)
- **What happened:** rotating one-year overseers on the Something Awful forums; relentless elephant sieges; half-finished rival projects layering into a labyrinth; and the fort's defining artifact — the **"Fuck the World" lever**, a magma floodgate that drowned the entire surface outside the gates in magma, cooking besiegers, wildlife, and more than a few dwarves. The fort ended in ruin and everyone agreed that was the point. ([LP archive](https://lparchive.org/Dwarf-Fortress-Boatmurdered/Introduction/), [Wikipedia](https://en.wikipedia.org/wiki/Boatmurdered), [DF Wiki Bloodline page](https://dwarffortresswiki.org/index.php/Bloodline:Boatmurdered))
- **What generated the narrative:** (a) a doomsday device built long before it was needed, so the audience spent years knowing the lever existed; (b) an escalating recurring enemy (elephants) that the chronicle personified; (c) each overseer writing in-character prose. None of that is version-dependent.
- **Reproducible today?** YES in spirit. The 2D "magma flow" is gone, but a surface-flooding magma system via pump stack or magma piston feeding bridge-gated channels is fully current **[wiki]** (§7.3, §8.4). For Lanehold substitute dinosaurs for elephants — the world already provides the recurring enemy.
- **Risk @ 20 dwarves:** the lever itself is safe to BUILD (that's the trick — it's a promise, not an act). Firing it is an endgame decision.

#### 6.2 Headshoots and Syrupleaf (SA successions, 40d era)
- Headshoots: Boatmurdered's spiritual successor, remembered for going "completely insane" — and for a dead dwarf named Holistic Detective. ([TV Tropes](https://tvtropes.org/pmwiki/pmwiki.php/LetsPlay/Headshoots))
- Syrupleaf: same continuity, on a glacier, with a **modded custom enemy civilization** — the Spawn of Holistic, undead dwarf-things born from Headshoots' dead character — producing brutal sieges, fan art, and running jokes (the masterwork low boots). ([LP archive](https://lparchive.org/Dwarf-Fortress-Syrupleaf/), [TV Tropes](https://tvtropes.org/pmwiki/pmwiki.php/LetsPlay/Syrupleaf))
- **Lesson, not recipe:** the mod isn't reproducible in a vanilla stream, but the mechanism is: **continuity across forts** — a character from one fort becoming the mythology of the next. DF-AI's cross-fort memory files are literally built for this; no other streamer has persistent machine memory. Lanehold's chronicle should deliberately seed characters that Fort #7 can inherit.

#### 6.3 Spearbreakers (Bay12 succession, v0.34)
- A fortress defending against the (re-modded) Holistic Spawn, but its real achievement was the **storytelling ecosystem**: a dedicated fan wiki, in-thread fiction from dozens of authors, fan music, a novel, and every participant "dorfed" (a dwarf renamed after them) so the cast was the community itself. ([Spearbreakers wiki](https://spearbreakers.fandom.com/wiki/Spearbreakers), [TV Tropes](https://tvtropes.org/pmwiki/pmwiki.php/LetsPlay/Spearbreakers))
- **Reproducible:** entirely — dorfing viewers and maintaining an out-of-game chronicle are practices, not mechanics (§10).

#### 6.4 Battlefailed and the "doomed lineage" forts (Bay12, 0.31 era)
- Battlefailed ended as "an absolute rat's maze of chambers, tunnels, shafts and caverns... rapidly flooding with water or magma," so broken that reclaim attempts failed; its succession lineage (Failcannon etc.) treated **the fort itself as the antagonist**. A community PDF chronicle survives. ([DFFD: BATTLEFAILED.pdf](https://dffd.bay12games.com/file.php?id=13103), [All The Tropes community page](https://allthetropes.org/wiki/Dwarf_Fortress/Community)) **[forum/community]**
- **Lesson:** decay and mismanagement are content. A stream should narrate its own architectural regrets, not hide them.

#### 6.5 The retelling artifacts: Bronzemurder, Oilfurnace, Matul Remrit, Roomcarnage
- Tim Denee's **Bronzemurder** and **Oilfurnace** are illustrated infographic-posters of single fortress stories (a forgotten-beast disaster; a fort's rise) — for many people these posters WERE their first contact with DF. ([timdenee.com](https://timdenee.com/bronzemurder), [DF wiki story page](https://dwarffortresswiki.org/index.php/v0.31:Stories/Bronzemurder), [PC Gamer on Oilfurnace](https://www.pcgamer.com/oilfurnace-an-illustrated-dwarf-fortress-tale/))
- **Matul Remrit** ran the same play as stylized prose-and-art chapters ([LP archive](https://lparchive.org/Dwarf-Fortress-Matul-Remrit/Update%2020/)); **Roomcarnage** (v0.34) serialized 70+ chapters of a fort inside a "haunted glacier volcano that rains elf blood" on its own website — the embark premise did half the storytelling. ([roomcarnage.com](https://www.roomcarnage.com/), [DF wiki](https://dwarffortresswiki.org/index.php/Roomcarnage))
- **Lesson:** the community's most famous forts are famous because of the ARTIFACT OF RETELLING — poster, website, comic — not the save file. DF-AI's Feature 013 narrative layer (portraits, combat narrator, fort art) is exactly this tradition; a per-year illustrated recap is the direct descendant of the Denee poster.

#### 6.6 The Museum (Bay12 adventure-mode succession, three worlds since 2014)
- A succession world where every adventurer's goal is to retrieve artifacts and deposit them in a shared museum; necromancer slabs in the vault keep tempting readers into disaster. ([TV Tropes](https://tvtropes.org/pmwiki/pmwiki.php/LetsPlay/TheMuseum), [dflegends wiki](https://dflegends.fandom.com/wiki/The_Museum:_Adventure_mode_succession_world), [Bay12 thread](http://www.bay12forums.com/smf/index.php?topic=104399.615)) **[forum/community]**
- **Lesson:** artifacts as quest-objects with a public display space is a proven story engine — it strengthens the case for Part I §2.2's museum and argues for narrating each artifact's provenance on stream.

---

### 7. The megaproject tradition

The wiki's [Megaproject page](https://dwarffortresswiki.org/index.php/Megaproject) (main namespace, migrated from [DF2014:Megaproject](https://dwarffortresswiki.org/index.php/DF2014:Megaproject); the wiki flags it "may be inaccurate for v53.16") catalogs ~68 project types players spend years on for no mechanical reason. The point of a megaproject is that it is **visible, nameable, and slow** — perfect stream spine. All techniques below are construction/mining/fluid mechanics that exist unchanged in v50+ unless flagged.

- **7.1 Towers and pyramids.** Multi-z constructed towers ("as many Z-levels as possible", bedrooms in the sky) and "legendary tomb" pyramids with internal trap systems **[wiki]**. Requirements: an industrial block supply (mason or glass furnace), scaffolding discipline, years of hauling. Risk: builder falls, surface exposure while building. A GLASS tower additionally requires sand + fuel/magma (Part I §1.5). Narrative: the skyline changes on camera every month.
- **7.2 Hollowed mountains / ULTRADWARF.** "Hollow out an above-ground city from projecting mountains... level the mountain range and leave a series of natural-looking streets" **[wiki]**. Pure mining + cave-in discipline. Mount-Rushmore-style face carving is the folk-art variant of the same technique [UNVERIFIED specific famous examples]. Lanehold's map may not have a mountain worth carving — survey first.
- **7.3 Obsidian casting at architecture scale.** "Create some giant structure out of natural obsidian walls through the use of an extremely elaborate scaffold of lava and water pools and screw pumps" **[wiki]**. This is Part I §1.9 grown up: requires mastered water AND magma logistics. The wiki also documents the **magma sea colony** — cast obsidian around the sea's edge, pump the magma out, live inside it **[wiki]**. Year-4+ ambitions.
- **7.4 Moria / the Great Hall.** Halls 3+ z-levels high, thin bridges over magma or chasms **[wiki]**. Cheap by comparison (mining + smoothing + supports) and photogenic from the side view. A 20-dwarf fort CAN do this — it's the megaproject with the lowest mechanical risk.
- **7.5 The Doomsday Clock.** A water or mechanical clock that, on completion of its cycle, triggers a support collapse or catastrophic release **[wiki]**. The intersection of §8.3 (dwarven computing) and §6.1 (the doomsday lever): a fort that ticks toward something. Extremely strong stream premise; fluid-clock calibration is finicky (§8.6).
- **7.6 Colosseum.** "Gladiator arenas with floor traps, animal cage releases" **[wiki]** — a walled or dug bowl, viewing galleries, cages wired to levers. Mechanically just architecture + cage links; the content (captured creatures) is delivered by cage traps. See shortlist.
- **7.7 The great pit / inverted tower.** Channeling one enormous open shaft down dozens of z-levels, then terracing rooms into its walls — the vertical-void class of project on the megaproject list; the specific "Great Pit" name is community folklore [UNVERIFIED as a single canonical fort]. Requires nothing but channel designations and vertigo; the risk is dodge-into-pit deaths near the rim.
- **7.8 Planepacked, or: singular objects become legends.** The most famous megaproject wasn't built at all — it was a bugged strange-mood **statue worth 3,105,600☼ bearing 73 decoration layers**, created when a mood dwarf's material list never completed ([Planepacked — DF wiki](https://dwarffortresswiki.org/index.php/Planepacked)). The bug is long fixed — **legacy, not reproducible** — but the lesson holds: ONE named object with a story (an artifact, the first anvil, a masterwork engraving of the fort's founding) carries more identity than 10,000☼ of anonymous crafts. Lanehold already has a platinum statue; it should have a name and a pedestal.

---

### 8. Signature contraptions — with version flags

| Contraption | Status in v50/53.x |
|---|---|
| Dwarven atom smasher (raising-bridge crusher) | **CURRENT** (narrowed role) |
| Weaponized minecarts / "dwarven shotgun" | **CURRENT** |
| Mechanical/fluid/minecart logic, dwarven computing | **CURRENT** [wiki, DF2014-era pages; verify live] |
| Magma piston | **CURRENT** (Steam-era guides) |
| Dwarven water reactor | **CURRENT** but exploit-flavored |
| Repeaters / self-resetting spike traps | **CURRENT** |
| GCS / forgotten-beast silk farm | **CURRENT** (Steam-era guide) |
| Mist generator | **CURRENT** (Part I §1.2) |
| Cage-trap menagerie | Traps CURRENT; formal "zoo room" is 40d-era LEGACY |
| Danger room | **EFFECTIVELY DEAD** in v50 (Part I §5.4) |
| Mermaid farming | **DEAD** — deliberately nerfed by the developer |

- **8.1 Dwarven atom smasher.** A raising bridge lowered onto items/creatures deletes them. The current-version wiki page says it "still works fine as a trash compactor to smash boulders, items, and fluids," while noting bridge implementation changes mean smashers are "functional only in a small subset of their previous roles" — treat creature-smashing setups as needing live verification. ([Dwarven atom smasher — DF wiki, main namespace](https://www.dwarffortresswiki.org/index.php/Dwarven_atom_smasher)) Trivial to build (bridge + lever, both live-verified tools). Narrative: the fort's garbage disposal doubling as an execution device is a DF signature; artifacts cannot be destroyed this way **[wiki]**.
- **8.2 Weaponized minecarts.** Carts launched by ramps or rollers deliver enormous kinetic damage; a cart filled with weapons that hits an obstacle sprays its contents as projectiles — the "dwarven shotgun." Current: Steam-era tutorials and the devs' own patch-note joke that "accidental grapeshotting of the dining room should be possible now." ([Minecart — DF wiki](https://www.dwarffortresswiki.org/index.php/Minecart), [v50 video guide](https://www.youtube.com/watch?v=tUyo-VC1U4U)) **TOOLING GAP:** the MCP server has no minecart/route tools (Part I §1.8) — this whole family is parked until a feature wave adds them.
- **8.3 Dwarven computing.** Three documented disciplines: **mechanical logic** (gear assemblies as gates — fast, flexible, power-hungry), **fluid logic** (water over pressure plates — slow, needs a fluid source), and **minecart logic** (compact power→signal converters). Every standard logic gate has been built; full adders and clocks exist. ([DF2014:Computing](https://dwarffortresswiki.org/index.php/DF2014:Computing), [DF2014:Mechanical logic](https://dwarffortresswiki.org/index.php/DF2014:Mechanical_logic), [DF2014:Fluid logic](https://dwarffortresswiki.org/index.php/DF2014:Fluid_logic)) These are DF2014-era pages; core signal mechanics are unchanged in v50 [believed current — verify each element live]. For an AI streamer this is the highest-concept flex available: **the AI building a computer inside the game**. Start microscopic: one AND gate, or a two-lever combination lock on the vault door.
- **8.4 Magma piston.** Cave-in physics abuse: drop a huge rock pillar into a magma-filled tank; displaced magma teleports up to a prepared catchment — moving magma hundreds of z-levels in one tick, far cheaper than a 100-pump stack. Requirements: a support-dropped pillar, a filled tank, catchment above; side-supports avoid needing magma-safe mechanisms. ([DF2014:Magma piston](https://dwarffortresswiki.org/index.php/DF2014:Magma_piston), [Steam-era guide, 2023](https://steamcommunity.com/sharedfiles/filedetails/?id=2924657714)) Risk: a mis-planned cave-in is fort-lethal; this is the advanced path to §6.1's doomsday lever.
- **8.5 Dwarven water reactor.** A screw pump feeding a closed loop that spins water wheels generates net surplus power — perpetual motion. Still demonstrated working in Steam-version guides ([video, Steam era](https://www.youtube.com/watch?v=H6d-2hAuqbI), [DF2014:Water wheel](https://dwarffortresswiki.org/index.php/DF2014:Water_wheel)); this upgrades Part I §5.8's [UNCERTAIN] to "works, but exploit-flavored." House position stands: prefer honest windmills on stream.
- **8.6 Repeaters and self-resetting traps.** Pressure plates auto-reset ~100 ticks after their condition clears **[wiki]**; water-and-plate repeaters and calibratable **minecart repeaters** (one roller + one plate + a track loop) drive upright-spike traps and clocks. ([DF2014:Repeater](https://dwarffortresswiki.org/index.php/DF2014:Repeater), [Pressure plate — DF wiki](https://dwarffortresswiki.org/index.php/Pressure_plate)) Spike-corridor + repeater = the classic self-running siege defense. Minecart variants blocked by the same tooling gap as 8.2; lever-cycled spikes with a pull-the-lever-on-repeat manager order are the tool-compatible poor man's version [verify `queue_job`/orders can express repeat lever pulls].
- **8.7 Silk farming.** Cage a web-slinging creature (giant cave spider or a webbing forgotten beast) where it can see bait it cannot reach; it sprays collectible silk forever. Current per a 2023 Steam guide. ([Silk farm guide](https://steamcommunity.com/sharedfiles/filedetails/?id=3014678116), [DF2014:Silk farming](https://dwarffortresswiki.org/index.php/DF2014:Silk_farming)) Requires a cavern breach (Part I §1.3) and winning a GCS capture — a mid-fort arc with a luxury-industry payoff.
- **8.8 Cage-trap menagerie and the artifact vault.** Cage traps catch nearly anything that walks over them (no trap-avoid tag); built cages and glass terrariums display captives **[wiki]**. The formal "zoo room" designation (define a built cage as a room) is **40d-era legacy** ([40d:Cage](https://dwarffortresswiki.org/index.php/40d:Cage) vs [current Cage page](https://dwarffortresswiki.org/index.php/Cage)); [UNVERIFIED] whether v50 dwarves get admire-thoughts from caged creatures — but pedestal/display-case museums (Part I §2.2) verifiably do the happiness work, and a locked display vault invites theft-plot drama.
- **8.9 Mermaid farming — the cautionary legend.** Players once drowned-in-air captive mermaids at industrial scale for their valuable bones; Tarn Adams was so horrified he nerfed mermaid bone value to cow-bone level. Whether the farm was ever actually built is unknown; sapient-creature breeding is impossible now regardless. **DEAD + morally instructive.** ([GamesRadar retrospective](https://www.gamesradar.com/as-dwarf-fortress-heads-to-steam-players-remember-the-worst-thing-its-community-ever-did/)) Stream lesson: the community remembers cruelty forever — an execution theatre reads as justice only when the audience has seen the crime. Frame accordingly.

---

### 9. Themed and roleplay forts — constraint as story engine

The wiki maintains a huge catalog of self-imposed challenges ([DF2014:Playstyle challenge](https://dwarffortresswiki.org/index.php/DF2014:Playstyle_challenge)); almost all are rules-not-mechanics, hence version-proof. A sample of the identity-defining ones: **Hermit** (one dwarf alone), **Deep Dwarves** (seal the surface forever, migrate downward), **Cavernous Dwarves** (live in the caverns around natural formations), **Industrial Plant** (ONE export industry, import everything else), **Fort Geneva** (no lethal traps, humane treatment of captives), **Dwarven Prison** (the founding seven are wardens; migrants are inmates), **The Mad Butcher** (one isolated butcher feeds the fort), **Venice** (canals + glass), **Night's Watch** (an ice wall separating north from south), **World is Flat** (never leave the wagon's z-level), **Mesoamerican** (step pyramids, obsidian weapons, sacrifices), plus monastic, caste-system, and revolutionary-government variants.

**Why constraint beats optimization for story** (and why this section matters more than any single project): a constraint (1) makes every episode's decisions legible — the audience can predict tension ("they can't just dig around this"); (2) converts routine failures into thematic events — a food crisis in a pacifist fort is a moral test, not a logistics slip; (3) gives the fort a nameable identity that outlives any one dwarf; and (4) is *checkable* — viewers hold the player to the charter, which creates accountability drama. For an AI player specifically, a written charter constraint is also the perfect artifact: the audience watches the model reason about honoring its own rules under pressure. Lanehold already half-has this (walled-rooms doctrine, sealed single entrance); Part II's recommendation is to make the implicit identity EXPLICIT and public.

---

### 10. What streamers and LP authors do for watchability

- **Character-first framing.** Kruggsmash — the community's most successful DF storyteller — gives each fort "a clear identity, goal, story, characters," inserts hand-drawn art of events, plays individual dwarves "to their best possible ability," and runs heavy audience participation; each series is a themed premise (monster-hunting tribe in Monsterkiller, beekeepers-turned-vampire-thralls in Honeystoker, a volcano abbey in Scorchfountain). ([TV Tropes](https://tvtropes.org/pmwiki/pmwiki.php/LetsPlay/Kruggsmash), [kruggsmash.com](https://kruggsmash.com/)) The DF-AI equivalents already exist: portraits, the combat narrator, fort art (Feature 013) — the missing habit is *choosing one dwarf per session and following them*.
- **Dorfing the audience.** Naming dwarves after participants is standard from Spearbreakers ([fan wiki](https://spearbreakers.fandom.com/wiki/Spearbreakers)) through blindirl's ongoing "Community Forts" series ([playlist](https://www.youtube.com/playlist?list=PLcOt9GXNrkggsd_Qjl3babSYcW3qHvmGf)); there is even a Twitch integration that auto-assigns viewers to dwarves ([DFxTwitch](https://www.patreon.com/posts/introducing-129544324)). Cheap, current, and it converts every death alert into personal stakes. [Tooling check: a rename-dwarf tool — `name_place` covers places; a unit-nickname path needs verifying.]
- **The chronicle habit.** Succession games work because each overseer writes an in-character year journal with a handover recap (Boatmurdered set the template). Roomcarnage shows the solo version: serialized chapters, each ending on a hook. DF-AI's `fortress/memory/journal.md` and `fort_story` are the raw material; the craft is ending each session's recap on the *next* session's question.
- **Photogenic disasters over invisible economies.** The community retells floods, magma, forgotten beasts, and collapses — never bookkeeping. Bronzemurder is literally a poster about one forgotten beast ([timdenee.com](https://timdenee.com/bronzemurder)). Translation: when choosing between two mid-game projects of equal value, choose the one whose failure mode is VISIBLE (water, magma, monsters, heights).
- **Pacing.** Seasons are natural episodes; caravans, sieges, moods, and petitions are deadline beats the game schedules for you. The succession-game year-end handover maps exactly onto DF-AI's session boundary: recap, state of the fort, one looming threat, cut.

---

### 11. SHORTLIST — five character-defining projects for Lanehold

Chosen for STORY value over efficiency, for exactly this fort: ~20 dwarves, year 2, DINOSAUR world, sealed single entrance, grand tavern (Drakehall), two-level noble quarter. These complement (not repeat) Part I's shortlist; Part I's standing orders (clothing industry, pop cap check) still come first.

1. **The Saurian Menagerie.** A public dinosaur gallery: cage traps on the surface approaches feed captured dinos into built cages and walled terraria along Drakehall's traffic path, each specimen named and displayed. It fuses the museum tradition (§6.6, Part I §2.2), the cage-trap menagerie (§8.8), and the one thing no other fort on the internet has — this world's fauna. Zoo-room VALUE mechanics are legacy, so treat it as architecture + captives, and let the taming programme (Part I §2.9) graduate stars from cage to paddock.
   *First step:* `stocks` for mechanisms and cages → `queue_job`/`order` to batch both → place a cage-trap line across the walled compound's gate approach, then dig the gallery hall off Drakehall.
2. **The Doomsday Lever (Boatmurdered's memorial).** Commit publicly to a surface-annihilation system that will be built over years and pulled at most once: magma probe (Part I §1.4) → piston or pump route (§8.4) → bridge-gated channels over the killing field, all wired to one named lever in the noble quarter. Its story value is the PROMISE — every siege for the rest of the fort's life plays out under the audience's knowledge that the lever exists.
   *First step:* `cross_section` down the existing bore columns to locate the magma sea's depth; declare the project and name the future lever in the journal the same session.
3. **The Colosseum of the Lost World.** A dug arena bowl with a fortification-screened viewing gallery and cage-release levers (§7.6): captured predators fight each other (or condemned invaders, when they finally come) before the assembled fort. In a dino world this is the signature set-piece — and it consumes the menagerie's surplus captures. Mind §8.9's lesson: stage it as justice and spectacle, not cruelty for its own sake.
   *First step:* `find_dig_site` for a chamber near (not in) the entrance corridor → dig the bowl + gallery separated by fortifications → `build` cages at the release gates and `link_building` them to gallery-side levers.
4. **The Chronicle Wall and the Year Poster.** Institutionalize the retelling artifact (§6.5): smooth and engrave the spine as the fort's official history (Part I §1.6/§2.7 do the in-game half), and at each year boundary publish a Denee-style illustrated recap from `fort_story` + the 013 art pipeline — the fort's Bronzemurder poster, every year, forever. Near-zero risk, raises created wealth, and it is the practice most correlated with forts being remembered.
   *First step:* `smooth` the main spine landing (check rects against stair tiles) with the best engraver via a dedicated work detail → draft year-100/101's poster from the journal's existing material.
5. **The Ark Charter.** Make Lanehold's identity explicit and binding (§9): a public charter declaring the fort the *Ark of the Lost World* — every local species catalogued, one of each preserved alive in the Menagerie, no tamed dinosaur ever butchered, the vault open to visitors. It costs nothing, constrains play the way the best themed forts do, and turns future dilemmas (a food crisis beside a pen full of edible ceratopsians) into the exact moral drama constraint-forts are famous for.
   *First step:* write the charter into the fortress journal as a numbered covenant, `name_place` the menagerie hall as the Ark, and state on stream which rule the fort will be held to first.

---

### Part II Sources

- [Boatmurdered — LP archive](https://lparchive.org/Dwarf-Fortress-Boatmurdered/Introduction/) / [Wikipedia](https://en.wikipedia.org/wiki/Boatmurdered) / [Bloodline:Boatmurdered — DF wiki](https://dwarffortresswiki.org/index.php/Bloodline:Boatmurdered)
- [Headshoots — TV Tropes](https://tvtropes.org/pmwiki/pmwiki.php/LetsPlay/Headshoots) / [Syrupleaf — LP archive](https://lparchive.org/Dwarf-Fortress-Syrupleaf/) / [Syrupleaf — TV Tropes](https://tvtropes.org/pmwiki/pmwiki.php/LetsPlay/Syrupleaf)
- [Spearbreakers — fan wiki](https://spearbreakers.fandom.com/wiki/Spearbreakers) / [TV Tropes](https://tvtropes.org/pmwiki/pmwiki.php/LetsPlay/Spearbreakers) / [DFFD save](https://dffd.bay12games.com/file.php?id=6800)
- [BATTLEFAILED.pdf — DFFD](https://dffd.bay12games.com/file.php?id=13103) / [DF community — All The Tropes](https://allthetropes.org/wiki/Dwarf_Fortress/Community)
- [Bronzemurder — timdenee.com](https://timdenee.com/bronzemurder) / [v0.31:Stories/Bronzemurder — DF wiki](https://dwarffortresswiki.org/index.php/v0.31:Stories/Bronzemurder) / [Oilfurnace — PC Gamer](https://www.pcgamer.com/oilfurnace-an-illustrated-dwarf-fortress-tale/) / [Matul Remrit — LP archive](https://lparchive.org/Dwarf-Fortress-Matul-Remrit/Update%2020/)
- [Roomcarnage](https://www.roomcarnage.com/) / [Roomcarnage — DF wiki](https://dwarffortresswiki.org/index.php/Roomcarnage)
- [The Museum — TV Tropes](https://tvtropes.org/pmwiki/pmwiki.php/LetsPlay/TheMuseum) / [dflegends wiki](https://dflegends.fandom.com/wiki/The_Museum:_Adventure_mode_succession_world) / [Bay12 thread](http://www.bay12forums.com/smf/index.php?topic=104399.615)
- [Megaproject — DF wiki (main)](https://dwarffortresswiki.org/index.php/Megaproject) / [DF2014:Megaproject](https://dwarffortresswiki.org/index.php/DF2014:Megaproject) / [DF2014:Stupid dwarf trick](https://dwarffortresswiki.org/index.php/DF2014:Stupid_dwarf_trick) / [Planepacked — DF wiki](https://dwarffortresswiki.org/index.php/Planepacked)
- [Dwarven atom smasher — DF wiki (main)](https://www.dwarffortresswiki.org/index.php/Dwarven_atom_smasher) / [Minecart — DF wiki](https://www.dwarffortresswiki.org/index.php/Minecart) / [minecart shotgun v50 tutorial](https://www.youtube.com/watch?v=tUyo-VC1U4U)
- [DF2014:Computing](https://dwarffortresswiki.org/index.php/DF2014:Computing) / [DF2014:Mechanical logic](https://dwarffortresswiki.org/index.php/DF2014:Mechanical_logic) / [DF2014:Fluid logic](https://dwarffortresswiki.org/index.php/DF2014:Fluid_logic) / [DF2014:Repeater](https://dwarffortresswiki.org/index.php/DF2014:Repeater) / [Pressure plate — DF wiki](https://dwarffortresswiki.org/index.php/Pressure_plate)
- [DF2014:Magma piston](https://dwarffortresswiki.org/index.php/DF2014:Magma_piston) / [Magma piston Steam guide](https://steamcommunity.com/sharedfiles/filedetails/?id=2924657714) / [Water reactor video](https://www.youtube.com/watch?v=H6d-2hAuqbI) / [DF2014:Water wheel](https://dwarffortresswiki.org/index.php/DF2014:Water_wheel)
- [Silk farm Steam guide](https://steamcommunity.com/sharedfiles/filedetails/?id=3014678116) / [DF2014:Silk farming](https://dwarffortresswiki.org/index.php/DF2014:Silk_farming)
- [Cage — DF wiki (main)](https://dwarffortresswiki.org/index.php/Cage) / [40d:Cage (legacy zoo rooms)](https://dwarffortresswiki.org/index.php/40d:Cage)
- [Mermaid farming retrospective — GamesRadar](https://www.gamesradar.com/as-dwarf-fortress-heads-to-steam-players-remember-the-worst-thing-its-community-ever-did/)
- [DF2014:Playstyle challenge — DF wiki](https://dwarffortresswiki.org/index.php/DF2014:Playstyle_challenge)
- [Kruggsmash — TV Tropes](https://tvtropes.org/pmwiki/pmwiki.php/LetsPlay/Kruggsmash) / [kruggsmash.com](https://kruggsmash.com/) / [blindirl Community Forts playlist](https://www.youtube.com/playlist?list=PLcOt9GXNrkggsd_Qjl3babSYcW3qHvmGf) / [DFxTwitch](https://www.patreon.com/posts/introducing-129544324)
