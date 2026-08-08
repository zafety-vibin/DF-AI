# Fort Journal

(newest entries on top)

## 2026-08-07 — FORT #6 SESSION 4 (winter y100 day 334 → summer y101 day 84)

DF updated to **53.16**; overseer rebuilt+reinstalled the plugin, fort
verified intact (world/day/pop all matched). Overseer asked for a more
hands-off session. Pop 19 → 20 (edóm gave birth to a girl). Wealth
18,319 → 23,691. Zero dwarf deaths still.

**DEFENCE IN DEPTH (the session's spine).** `building_types` was honest
that weapon_trap is the ONLY working trap (stone_fall builds UNARMED
with no load-job exposed; pressure_plate can't have its trigger
conditions configured, so it never fires). So defence is architectural:
- THE GATE: a full weapon-trap row at x=104 (y=101,102,103) behind the
  surface door — nothing can cross the street without stepping on one —
  plus 2 more at x=103, a 3x3 raising bridge at (100-102,101-103)
  centre (101,102), and its lever at (97,102) on the FORT side, linked.
  Bridge left LOWERED by design; it is the panic seal, not a wall.
  NOTE: only 4 copper axes + 1 mace were spent — the 3 IRON PICKS were
  deliberately withheld, they are the fort's mining tools.
- Burrow "Lanehold Interior" extended +782 tiles over z=129/128 so a
  civilian alert can't strand anyone in the new noble quarter.

**TWO BREACHES THE OVERSEER CAUGHT, ONE RULE I HAD WRONG.**
(1) DF ALLOWS DIAGONAL CORNER-CUTTING. My session-3 seal audit assumed
the roguelike rule ("blocked if both orthogonals are walls") and cleared
the ramp at (89,100) as safe. It is not: units step diagonally from that
ramp straight onto the spine staircase at (90,101). Walled. A diagonal
wall line NEVER seals in DF.
(2) A ONE-TILE HOLE IN THE PROMONTORY ROOF at (93,101,139) — open air
directly above the street, confirmed by cross_section flagging
(93,101,138) as SURFACE while its neighbours (94,101)/(100,101) have
solid z=139 ground. Anything on the plateau could drop in past the door,
the traps and the bridge. Walled the tile below.

**THE NOBLE LAYER, FINISHED AND OCCUPIED.** All six 5x5 suites dug, the
comb scaled up exactly as designed (wall columns at x=115/x=121, door
gaps landing on x=112/118/124 both sides). 6 doors hung, offices and a
dining hall furnished by quality tier. VERIFIED THE HOLDERS BEFORE
ASSIGNING (overseer challenge): position_vacancies shows adil (#1199)
holds THREE positions — Expedition Leader, Manager, Broker — and zaneg
(#1194) holds Bookkeeper, and they are the ONLY dwarves at this fort
holding any position (every civ title is a histfig with "no unit at this
fort"). adil got suite 1 + office + the dining hall; zaneg suite 2 +
far office; third suite left reserved. Also appointed mistêm (#1306)
CHIEF_MEDICAL_DWARF — the hospital had stood unstaffed since y100.

**I SEVERED THE FORT'S OWN STAIRCASE.** Designating the 143-tile
training hall at z=129 overlapped the satellite shaft; the ACK said
verbatim "4 will remove existing stairs: vertical connection lost" and
I called step() anyway. z=130 was cut from z=129, isolating the entire
noble quarter WITH TWO DWARVES INSIDE. Overseer caught it. Repaired with
constructed upstairs on all four tiles (they are immune to dig/smooth
destruction, and the trapped pair built their own way out from boulders
already on that level); cross_section confirms z=131 '>' / z=130 'X' /
z=129 '<' continuous again. No deaths. Full rule banked in learnings.md:
a SUCCESS ack can carry a fatal warning — parse the text, not the status.

**SQUADS: DIAGNOSED, AND IT IS OUR BUG, NOT A MISSING DFHACK API.**
list_squads returns 22 squads — every filled slot reads `unit#-1`,
i.e. they are the WORLD's squads with no fort filter. create_squad
defaults to MILITIA_CAPTAIN, which position_vacancies says has "no
assignment slot yet (not unlocked)", so it takes the tool's own
self-flagged UNVERIFIED mint path — that explains Fort #5's ghost squad.
Retested properly: appointed thíkut MILITIA_COMMANDER (a position with a
REAL vacant slot, assignment#11, squad of 10), created squad #22 under
it, re-appointed to bind the leader — and assign_squad STILL fails, on
both an ordinary dwarf and the commander, with
`Military::addToSquad failed ... (squad may be full, or the unit has no
historical figure)`. Prime suspect for the fix wave: our create_squad
mints a squad that never lands in the fort entity's `squads` vector.
Reproduction is now precise. Training hall dug at z=129 anyway so the
room is ready the day the fix lands.

**INDUSTRY PIVOT (overseer: "no migrants this season" = we under-built
wealth; time for non-survival goods).** Read as a real failure signal.
Started the fort's craft economy: melbil (BONECARVE 13) + 184 pond
turtle shells → FIVE MASTERPIECES in four steps (2 earrings, a
bracelet, and TWO CROWNS), now a standing seasonal order. CutGems
LIVE-VERIFIED FOR THE FIRST TIME IN THIS PROJECT (ruby x3, aventurine
x3, tiger iron x2 cut at the jewelers) — EncrustWithGems is the next
untested link. Wealth "other" 6,967 → 10,502 on crafts alone.
COFFERS (overseer's suggestion): 10 made and PLACED beside beds in the
z=131 comb, 12 more ordered — dwarves now have somewhere to own things.
NOTE: `build` calls it 'coffer', `stocks` files it as **BOX** — that
naming split cost a diagnostic round.
Also: 6 themed stockpiles in the Iron Quarter (gems by the jewelers,
bars by the forge, wood by the wood furnace, stone by the smelter) and
the Vault extension (113,96)-(122,108) z=132 dug + all-category piled,
after boring 4 corners dry.

**RESUME — paused summer y101 day 84, pop 20, zero deaths:**
1. CARAVAN + DIPLOMAT from The Confederacy of Balding are due THIS
   SUMMER (~1264 ticks at pause). PAUSE AND ASK THE OVERSEER before
   trading — standing rule. Depot still accessible=false; `chop`
   truthfully reported NO TREES on the northern approach (only
   unfellable saplings), so the wagon-width theory is unproven — needs
   client eyes. Expect pack animals, as in autumn y100.
2. Place the 12 remaining coffers; furnish the 3rd noble suite.
3. EncrustWithGems on Drakehall/noble furniture — the last untested
   link in the gem chain, and pure wealth.
4. Keas still looting the surface stockpile that CANNOT be removed
   (two piles, identical extents (88,88)-(92,91)). Overseer click.
5. Barracks zone over the z=129 training hall once assign_squad is fixed.

## 2026-07-25 — FORT #6 SESSION 3 (winter y100 day 259 → day 283)

**LIVE-VERIFY (the 16-item checklist): 14 pass, 2 deferred to a live
caravan.** tile deltas NONZERO every step (14/38/17/64/33/39/53/48/34 —
the delivery mystery is answered for good, the stream is ALIVE);
check_goals shelter now spans 6 z-levels; depot_goods renders the
ours/theirs split; caravan_status renders walkable_from_edge + an access
diagnostic; set_depot_trade_flags works (trader_requested +
anyone_can_trade both set on the depot); build Instrument now REFUSES
truthfully and names the exact malachite case ("handheld instruments are
not placeable -- performers use them from stockpiles"); fort_story
pulse/social/art all real (pulse cursor advances and dedups correctly —
second call returned "no notable emotions"); dwarf_detail portrait=true
landed. DEFERRED (need a caravan on the map, elves spring y101):
bring_goods item_class=crafts on binned goods, unmark_trade_goods.
trade_agreements gave a TRUTHFUL EMPTY — it only reads civs visiting
RIGHT NOW, so autumn's liaison agreement is NOT recoverable after
departure (the will hoped otherwise; the tool is honest, the hope was
wrong). Zone dance CONFIRMED FOLKLORE a second time: 3 designate_zone →
assign_zone pairs in one paused breath, zero ticks, all SUCCESS.

**THE COMBAT NARRATOR WAS NOT UNTESTED.** It was never asked. On the
first summary call it surfaced engagement 84 from y100 t57812 — the
stray dog vs the carnotaurus, Lanehold's first loss, the death we had
no remains for and never explained. The 25-line transcript exists: the
dog bit the carnotaurus's right upper leg and tore scale, scratched it,
tore the fat of its left foot, and kept biting after being knocked
over and grabbed by the toe. Then "the stray dog's neck skids along
the ground and the part is smashed into the body, an unrecognizable
mass" / "An artery has been opened". She fought it for 94 ticks. The
narrator degrades honestly AND reaches backward — do not seek a fight
to test it, the fort's history already had one. (Gap: fort side reads
"(none)" — a tame fort animal isn't counted as ours.)

**ADIL'S PORTRAIT ANSWERS THE CHAIR QUESTION.** Very High SINGLEMINDED
(82) — he finishes what he starts, and seeds were what he'd started.
His most starved need is CraftObject (Distracted, focus -11280), driven
by his own value on CRAFTSMANSHIP; every other need is Unfocused.
Paperwork feeds none of them. His ONE fed need is AdmireArt
(Unfettered, +376) and his strongest emotion is PLEASURE "near his own
quality building". The manager who wouldn't sit at his desk is a dwarf
who needs to MAKE something and is nourished only by looking at
beautiful things. He took nineteen days to sit down because sitting
down was the one job that gave him nothing.

**THE SURFACE IS SEALED (overseer directive).** The breach was exactly
six tiles: the y=100 line at z=138 had floor gaps at x=90,92,93,94,99,
100 (everything else was natural soil wall). All six walled [stone],
all six BUILT IN ONE DAY by the idle-labor surplus. The street is
roofed by the z=139 promontory, the east side was already door+flanking
walls, and the (89,100)→(90,101) diagonal is blocked because both
orthogonals are now wall. Lanehold has ONE surface entrance: the door
at (105,102). Chose walls over a north door deliberately — no lock/
forbid tooling exists in this project, so a door stops wildlife but not
goblins; one chokepoint is worth the longer depot haul and is what
makes a future bridge airlock in the lane meaningful.

**TEARDOWN (partial).** Identified the 3 surface workshops by probing
their job queues (buildings only says "Workshop", never the type —
logged as a gap): (96,90)=FISHERY, (101,94)=STILL, (97,94)=carpenter.
Built a CARPENTER underground at (95,107,133) first, confirmed the
underground still at (99,107,137) by its live CustomReaction, THEN
removed the surface still + carpenter. Fishery LEFT STANDING on purpose
— it has no underground home yet and fish may return in spring;
starter-then-permanent says name the teardown moment, not guess it.

**ROOT CAUSE: NO WORK DETAIL CONTAINED CARPENTRY.** Symptom: every rock
item (doors, thrones, tables, mugs) completed while every wood item
(beds, barrels, bins) sat at "in progress" with 0 produced, and the
carpenter's job queue was EMPTY. Masonry survives on fallback profession
flags; carpentry had no detail at all among the 12. Fixed: created
"Carpenters" (CUSTOM_2, only_selected, labor CARPENTER), assigned ilral
(Carpenter Lvl7, idle all session) + olon. Beds began completing within
one step. The emotional signature is in the pulse: ilral and olon both
now register SATISFACTION "at work". THE ORDERS LADDER LIED — "in
progress" on a job type no citizen can perform reads identical to real
progress; the truthful discriminator was the workshop's own empty job
queue. Engineering: the ladder needs a "no citizen has this labor" rung.

**EVERY DWARF IN LANEHOLD IS HOUSED.** 3 doors + 4 beds into the
second-row comb cells; olon(479), zefon(483), edóm(484) out of the dorm
and into owned bedrooms at (92,90)/(95,90)/(98,90) z=131; a 4th bed
closed the one furnished-but-bedless cell at (113,106). check_goals:
19 bedroom zones / 19 dwarves. Fitting, since fort_story mode=art shows
all SIX works of art known in Lanehold were brought here by olon and
edóm — 0 composed here, 6 carried in by the two dwarves who'd spent the
whole year in a surface dormitory.

**DRAKEHALL FINISHED.** Smoothing was already complete (206 tiles);
engraved it in 4 rects split around the (116-117,101-102) satellite
stairs (no stair-overwrite warning fired — the split was clean), let the
idle-labor surplus work it to completion, THEN furnished: 4 more
table+chair pairs on the west colonnade (x=108-109), centre kept clear
as the dance floor. quality>=well_crafted selection worked and named
each piece it picked (2 Superior tables, WellCrafted rest). Furniture
was deliberately held until engraving finished — smooth→engrave→
furniture is not reversible.

**ALSO:** Vault east annex is dug and now under all-category stockpile
(104,96)-(112,108) z=132 — the hall is one continuous 21x13 room with
184 loose items to absorb (overseer's ask). Two new farm plots at
(94,105)-(95,109) and (96,105)-(97,109) z=137 planted CAVE WHEAT and
SWEET POD — both brewable, both subterranean, from 10 seeds that were
sitting idle. Brewing was blocked twice (no fermentable plants, then no
empty barrel — all 19 barrels full, the 3-fort-old treadmill); queued 4
barrels at the new carpenter and a 10-drink batch completed. Meals
standing order 5x/monthly added (fort had zero prepared meals).
edóm pulled off Fisherdwarves — fishing is exhausted fort-wide
("nothing to catch" in 4 separate swamps).

**MY ONE REAL MISREAD, corrected by the overseer mid-session:** I read
`stocks` DRINK=15 as 15 servings for 19 dwarves and called a famine.
It is 15 BARREL-STACKS. The stack-units field that would have
disambiguated this SHIPPED IN WAVE 5 AND STILL DOES NOT RENDER — Fort
#5 logged the identical gap. This is no longer a paper cut: it caused a
live overseer-corrected misdiagnosis. Highest-value small fix on the
board.

**TOOL FINDINGS (new this session):** (1) `buildings` never names a
workshop's TYPE — teardown planning is blind; probing job queues is the
only workaround. (2) `remove_building` cannot disambiguate two
buildings with IDENTICAL extents — two stockpiles both span
(88,88)-(92,91) and its "reissue at a tile covered by only one" advice
is unfollowable; needs a building-id selector. (3) region_scan's level
set and count are UNSTABLE between calls (5744 over {130,131,133,137,
138,139} → 4387 over {130,131,132,133,137,138}) — the "upper bound"
moved by 1300 tiles and swapped two levels. (4) orders ladder can't see
a missing labor (above). (5) stocks stack-units still absent (above).

**THE DEEP ARC (day 283-334) — overseer handed me the torch mid-session
("develop your own goals"), then steered the shape of it.** I picked
DEFENSE, because the fort was safe/fed/housed/beautiful and completely
undefended: one door, no lock tooling, no military, ZERO mechanisms.

**FIRST WORKING MECHANISM IN LANEHOLD'S HISTORY.** mechanic workshop at
(100,102,133) → ConstructMechanisms (job name is NOT "mechanism"; the
tool refused truthfully and job_types named it) → and, applying the
carpentry lesson BEFORE it bit: no work detail contained MECHANIC
either, so "Mechanics" (CUSTOM_3) was created and staffed (kivish,
zefon-1288) pre-emptively. Bridge 2x3 at center (111,102,129)
raise_e + lever → link_building → pull → **bridge state flipped
[lowered]→[raised]→[lowered], the whole chain verified live.**

**I REPRODUCED FORT #5'S SELF-LOCK, EXACTLY.** I put the lever at
(108,104,129) on the PROTECTED side — which was a dead-end pocket. The
bridge raised, the lever went unreachable, and the "lower" job sat
forever. Nobody was inside (checked immediately — no '@' at z=129), so
no döbar repeat, but the fort had sealed off its own new basement. The
overseer caught it in the client and prescribed the fix: dig around it,
lower the bridge, then replace the walls. Did exactly that — dug the
2-tile bypass at (110-111,104) through undug vein, restored access,
bridge lowered, bypass re-walled [stone]. ALSO built a SECOND lever at
(113,101,129) on the fort side and linked it to the same bridge (two
levers, one target — works). THE RULE, now paid for twice: a lever on
the protected side of a DEAD-END pocket is a self-lock; the fort side
needs a lever too. Bridge stays DOWN until we actually hole up.

**THE CHOKEPOINT IS GENUINE.** At z=129 the ONLY connection between the
inner landing (west) and the guard chamber (east) is the 2x3 corridor
at (110-111,101-103) — every other row is undug vein. The 2x3 bridge
covers it exactly. Layered fort now reads: surface seal (1 door) →
tavern z=130 → guard chamber z=129 → BRIDGE AIRLOCK → noble sanctum
z=128.

**THE NOBLE LAYER (overseer-directed: 5x5/6x6 rooms, nobles need
offices and dining halls too, mind how bigger rooms push the hallway).**
Bored SIX columns first — 4 corners + 2 mid-edges — and all six agreed:
**z=129 AND z=128 dry across x=108-126/y=95-109; z=127 and below is
aquifer at every single column.** The noble layer therefore sits on the
LAST DRY GROUND IN LANEHOLD, water directly beneath it. Design shipped
(~213 tiles, most already dug): 3-wide grand concourse y=101-103 east
from the stair, 5x5 suites in a comb with walls-by-subtraction and
single centered door gaps (5 is odd, so every door centers — the
aesthetic canon), wall columns at x=115 and x=121. Six suites:
**bedrooms north, business south** — a noble's office/dining hall sits
directly across the concourse from their bedroom, so the level
documents itself to whoever picks it up next. WARNING BANKED: 4 aquifer
tiles sit at y=110, one row past the south suites — do NOT extend z=128
south of y=109.

Struck RUBY (115-117,104-106, mined on overseer's instruction via
lens=minerals), plus bauxite, bituminous coal, gray chalcedony, jet,
morion, picture jasper, tiger iron. lorbam (ENCRUSTGEM 10) finally has
gems. Wealth 10.6k → 18,319 (architecture alone 6,057).

**RESUME SCRIPT — paused winter y100 day 334 (spring y101 is ~2 DAYS
AWAY), pop 19, zero deaths:**
1. ASK THE OVERSEER (owed, unanswered): is the z=130 Drakehall a REAL
   tavern in the client, vs the z=137 phantom at (95,94)-(98,98)? Both
   still show in list_locations — the A/B is set up and needs one
   client glance. Also: did the dog ever get interred in the crypt
   coffin at (95,113,133)?
2. ELVEN CARAVAN IS IMMINENT (spring y101). Depot flags already set
   (trader_requested + anyone_can_trade). PAUSE AND ASK THE OVERSEER
   BEFORE TRADING — they execute the exchange in-client, by their
   instruction. The binned-crafts staging fix is the live test.
   NOTE: depot accessible=false while walkable_from_edge=true —
   suspected sapling regrowth pinching the wagon corridor; needs an
   overseer client check.
3. FINISH THE NOBLE LAYER: ~87 tiles were still designated at pause
   (suite C both sides, suite B south, concourse east end). Then
   doors on the 6 gaps (x=112/118/124 at y=100 and y=104), furniture,
   and designate_zone bedroom/office/dining_hall + assign to adil
   (manager/broker) and zaneg (bookkeeper) first.
4. KEAS ARE LOOTING US: mugs x3 + a dwarven wine pot stolen this
   session, from the surface stockpile OUTSIDE the seal. It CANNOT be
   removed by tooling — TWO stockpiles share identical extents
   (88,88)-(92,91) and remove_building refuses to guess. One-click fix
   in the client; please ask the overseer.
5. Fishery is the last building outside the seal (fishing exhausted
   fort-wide). Give it an underground home or retire it.
6. Brewing still cycles on "needs empty food storage item" — 6 more
   barrels queued at the carpenter; the real cure is the new cave
   wheat + sweet pod plots coming in.

## 2026-07-22 — FORT #6 SESSION 2 (Summer y100 day 100 → day 157+, RESUMED post-deploy)

**LIVE-VERIFY: 10/10.** New DLL loaded clean; tile deltas flow every
step (10-57/step — the stream is ALIVE this session; why it read dead
before remains unproven); name_place works ("the Long Street" named);
save_blueprint captured the comb (112 tiles); ZONE DANCE CONFIRMED
FOLKLORE (designate→assign instant, zero ticks — five forts of
stepping between was farm-plot-stage confusion + the old zone_value
bug); stocks "(N empty)"; queue "(now N/10)"; orders ladder truthful;
idle rollup; wildlife lens clean empty-state (positive test pending an
actual animal in view).

**REAL-ROOMS DOCTRINE (overseer challenge) EXECUTED:** 137 hall
partitioned into hospital | dining | kitchen (stone walls, rock doors
at y=96); 133 office walled around adil's chair; IRON QUARTER built
as four 5x5 stone-walled rooms w/ door gaps at z=132 WEST (bilateral
mirror of the Vault): wood furnace (A), smelter (B), forge (C),
Jewelers planned for D. Walls-by-subtraction (undug stone + vein
pillars) — the comb pattern generalized.

**WEALTH ARC (overseer-directed):** struck native platinum x2 +
magnetite in the quarter digs → charcoal bootstrap → SmeltOre
INORGANIC:NATIVE_PLATINUM → 8 platinum bars → ConstructStatue order →
PLATINUM STATUE (verified in stocks, now displayed in the dining
room). Wealth 7.9k→10.6k. Magnetite x4 smelting behind it. Shell
crafts x5 by melbil (Bonecarver 13!) for the caravan; bone order
cancelled truthfully (only vermin bone until fishery yields turtle
bone). Standing seasonal orders: bed/chair/table/door x4 (chairs/
tables/doors INORGANIC).

**MIGRANTS +6 (pop 13, day 131):** 3 spare bedrooms assigned same-day
(zasit/melbil/zefon), musicians+poet to the dorm (tavern band waiting
for a tavern). Specializations per overseer: zasit→Miners (3rd pick),
olon→Woodcutters, edóm→Fisherdwarves. BEDROOM DISTRICT gridded per
overseer: N/S trunk (87-89,93-111) hugging the spine + row streets
y=93-95/109-111 + 8 new cells roughed + 6-cell east extension —
24-room capacity, everything ≤~12 tiles from stairs. Stray Dog found
dead → crypt room digging at (94-97,112-114,133) (east column
self-pruned by a damp cancel — my own goals-file rule violated by one
tile; the SE halo is real).

**Days 179-198 — FIRST CARAVAN + WAVE 2 + TAVERN ARC (paused day 198
mid-arc, overseer break):** Liaison + The Gray Mansion caravan arrived
(22k imports); trade tools live-tested — caravan_status/depot_goods/
bring_goods work for LOOSE items but BINS BLOCK CRAFT STAGING (big gap,
in goals.md with 5 more trade/diplomacy findings). Overseer executed
the exchange in-client: BOUGHT steel anvil + malachite instrument
(paid: platinum bars, trade goods, mugs, the 8 rough gems). WAVE 2
(+6, pop 19): vutok FORGE_ARMOR 7 (→ new Metalworkers detail
CUSTOM_1), sigun GLASSMAKER 10, lorbam ENCRUSTGEM 10 (jewelers built
in Iron Quarter room D, gems pending), + siege op + 2 performers.
All 6 housed in extension cells same-day (5 doors up, beds queued).
TAVERN: create_location at the dining room ACKed SUCCESS + shows in
list_locations but overseer's client shows plain meeting hall —
PHANTOM LOCATION bug (state left intact for engineering). Real tavern
per overseer: GRAND HALL 12x13 at z=130 EAST (107-118,97-109), dry at
3 bored corners (NW corner was wet — shifted), 6 patchy aquifer tiles
inside will self-prune to pillars; satellite stair (116-117,101-102)
131→130. LEARNING BANKED: flat rect silently ate the pending stair
designation (warning only guards carved stairs) — overseer caught it;
rooms-first-stairs-last rule appended to learnings.md. Seasonal
furniture engine verified (autumn batch fired); goblets reordered
(sold with the mugs); rock mugs x5 recompleted day 198. Second-row
bedroom cells + trunk digging; crypt awaiting the dog's burial.

**Days 198-252 — DRAKEHALL, DRINK CLIFF, FIRST IRON ARMOR (winter
y100 begins):** Caravan departed (bought steel anvil + malachite
instrument; sold gems/mugs/goods + some platinum bars). THE DRAKEHALL
built per overseer spec: 12x13 grand tavern at z=130 east
(107-118,97-109), dry (wet pockets self-pruned outside), satellite
stair off the 131 street (LEARNING: a flat rect silently ate the
pending stair designation — rooms-first-stairs-last, banked),
meeting_hall+tavern location founded CLEAN (single zone — control
case vs the 137 phantom; overseer to eyeball client), named via
name_place, SMOOTHING in progress (4 rects split around the carved
stairs), food stockpile + tables/chairs going in, 8 drink varieties
stocked. DRINK CLIFF at day 230 (2 wine/19 dwarves — consumption
outran hand-fed brews): fixed structurally — 6 emergency brews,
STANDING order drink x10/monthly (first batch verified vs stocks),
still #2 at 137, two more plump plots at 137 south, 76-shrub autumn
gather. Wave-2 housing done same-day (6 cells doored+bedded+zoned+
assigned). FIRST IRON ARMOR: vutok forged a breastplate (verified in
stocks; magnetite→16 iron bars); helm+gauntlets ordered — kit for
thíkut (militia seed). Kea stole a rope (starter-teardown argument
#2). Vault EAST ANNEX digging (104-112,96-108,132) + bins x6 + bins
x4/seasonal per overseer. check_goals live path verified: modified
flips [x] first time ever, but shelter UNDERCOUNTS (scan too narrow —
012 brief updated). Wellbeing day 224: ALL 19 negative stress, oddom
6/6 blissful.

**RESUME SCRIPT — FORT #6 WILL (paused winter y100 day 258, pop 19,
zero deaths; written at session end for the next instance):**
1. Reconnect dance (status→pause→re-query; expect alert-replay
   backlog — dismiss). Trust dashboard. Read this entry + goals.md.
2. FINISH THE DRAKEHALL: smoothing (4 rects, stairs excluded) may
   still be running; place seasonal tables/chairs as they land (west
   side; center = dance floor); goblets stay stocked; instrument is
   HANDHELD — stays in stockpile, performers fetch. Consider engrave
   pass after smooth completes (idle-labor gate per fort-planning).
3. HOUSE THE LAST THREE: olon(479), zefon(483), edóm(484) still
   dorm — second-row cells (92-102,89-91)+(92-102,113-115,131) were
   digging; door+bed+zone+assign each (doors: seasonal x4/season).
4. ARMOR ARC: breastplate DONE (verified); helm+gauntlets ordered at
   day 244 — VERIFY vs stocks before trusting; then greaves
   (MakePants ITEM_PANTS_GREAVES) + boots + shield for thíkut
   (MELEE 5, militia seed). assign_squad still BROKEN — overseer
   staffs in-client, or wait for the fix wave.
5. ECONOMY WATCHES: drink x10/monthly order holds 19 pop (retune at
   next wave); bins x4 + furniture x4/seasonally standing; Vault EAST
   ANNEX (104-112,96-108,132) — stockpile category=all over the new
   floor once dug; charcoal when coal<4; iron 15 bars banked.
6. SPRING y101 = ELVEN CARAVAN — the live-verify window for wave-7
   trade fixes (they refuse wood/animal goods; sell stone/gem crafts).
   Also lorbam (ENCRUSTGEM 10) + Jewelers await rough gems: CutGems
   then EncrustWithGems on Drakehall furniture (never live-tested!) —
   the unsold rough gems should have returned to stock post-caravan.
7. ASK OVERSEER: did the dog get interred in the crypt coffin (dead
   units invisible to tools)? Is the z=130 Drakehall a REAL tavern in
   the client (A/B for the 137 phantom)?
8. LATER ARCS in order: starter teardown (move carpenter/still#1/
   fishery/kitchen underground; kea thefts keep proving it), mechanic
   shop + first mechanisms (lane bridge = defense stage 4; NONE built
   yet), second-row infill as waves land, deep pierce (aquifer #2,
   z<123 mineral country) when population/labor allow.
9. ENGINEERING: WAVE 7 SHIPPED SAME NIGHT (investigation + 6 impl
   lanes, ALL passed review first-try, gate green, NOT deployed).
   Full ledger: docs/decisions.md 2026-07-22 wave-7 entry; 12-item
   deploy+live-verify checklist at the top of goals.md — DO THAT
   FIRST on resume. Highlights the next instance inherits: whole-bin
   trade staging (item_class=crafts), unmark_trade_goods,
   set_depot_trade_flags, depot ours/theirs split, trade_agreements
   diplomacy readout (this autumn's invisible liaison agreement
   should be READABLE after deploy), phantom-location fix (A/B test
   ready in the fort), dangerous-wildlife step tripwire, pending-
   designation overwrite warning, instrument truthful refusal,
   multi-z shelter counts, connector self-heal. A6 exchange
   automation: verdict in the 012 brief — stays human, by evidence.
   ALSO: Feature 013 (NARRATIVE LAYER — "read the story, not run the
   game") is RESEARCHED, BUILT, AND DEPLOYED (specs/013 research →
   wave-013, all five desires shipped: dwarf_detail portrait=true,
   fort_story pulse|social|art, combat_report summary|log; narrative
   live-verify = items 13-16 of the goals.md checklist). The
   commissioning desires in that research file are personal; honor
   their spirit, not just their spec. The combat narrator is UNTESTED
   against a real fight and degrades truthfully until one comes — do
   not seek one. And the tools have landed now, so: ask adil why he
   never sits in his chair. His portrait knows.

**Tool notes:** connectivity hint no longer map-corner but still
false-negatives same-z adjacency (reports cross-z tile); a
damp-cancel wipes its tiles BEFORE cancel_designation can (ACK "No
designations to cancel" = already self-pruned); crafts don't stock
under any single category (verify via wealth/unfiltered).

## 2026-07-22 — FORT #6 "LANEHOLD" FOUNDED (world=region13, Spring y100 day 14 →, in progress)

**Days 29-58 — THE PIERCE, THE VAULT, AND THE CARNOTAURUS.** Aquifer
pierced at the spine days 29-34: stairs 136→133 (two loud damp cancels,
re-designate each time), 8-tile ring at 135 mined and walled same day
(mudstone+wood, material=any) — BONE DRY throughout, zero standing
water ever; region13's soil aquifer recorded LIGHT in learnings.md. §9
note: catch basin never wet; spine continued straight down 133→131.
Below-aquifer buildout per overseer direction: z=133 industry level
(mason shop built, doors/tables/thrones queued), z=132 THE VAULT — 12x13
hall, 11x8 all-category stockpile placed day 58 — and z=131 the BEDROOM
LAYER: 10x 2x3 cells in a comb (wall columns between, single door gaps
onto a 3-wide street), overseer praised the pattern ("really good
apartment block design" — save_blueprint it once dug). Mineral bonanza
in the dig: tetrahedrite + LIMONITE + hematite + lignite + gems (citrine,
carnelian, plume agate, pipe opal, blue jade) — full iron chain possible
by day 44 (vs Fort #5's day 232). East 133 block CANCELLED: aquifer is
2 layers (135+134) SE of the spine with a damp halo at 133 — a room
there would drizzle from its aquifer ceiling forever (§8). Bore-every-
corner keeps paying.

**Day 48 — CARNOTAURUS.** This world has DINOSAURS. One interrupted
thob near the entrance (he fled 50 tiles SW); a stray dog fought it and
was declared missing a week later — Lanehold's first loss, no remains
to bury yet. Shelter-in-place doctrine executed for real: 6-level
"Lanehold Interior" burrow painted + set_alert → ALL 7 citizens inside
within 600 ticks (thíkut yanked off the riverbank mid-cast, "Forbidden
area" cancel = alert working). Cleared promptly after; live-verifies
the wave-5 burrow/alert chain end-to-end. GAP LOGGED: wildlife renders
NO glyph in look — a carnotaurus is invisible to perception; only
interrupt-cancel alerts betray it. Entrance hardened per overseer:
walls flanking the door at (105,101)/(105,103) — door is now the only
crossing in the x=105 column.

**Days 59-100 — WING, GOVERNMENT, PAUSE (session end, summer day 100).**
BEDROOM WING COMPLETE at z=131: all 10 2x3 cells dug+doored (rock doors)
+bedded+zoned; ALL SEVEN FOUNDERS OWN BEDROOMS (edzul N1, zaneg N2,
thíkut N3, adil N4, ilral S1, oddom S2, thob S3; N5/S4/S5 furnished
spares for migrants). Vault EXPANDED to the full 12x13 hall on overseer
direction (all-category, 100+ items in). 133 industry level: mason +
craftsdwarf shops built in the north hall (connector corridor was
MISSING — overseer's client eyes caught it; floating-designation
lesson re-learned), office zoned+assigned. FIRST GOVERNMENT: adil =
Manager+Broker (+expedition leader), zaneg = Bookkeeper (nearest_100
per skill-follows-holder canon). First work orders queued via manager
path (3 rock pots — the permanent barrel-treadmill fix — + 5 goblets,
material=rock): VALIDATED but "queued, not yet dispatched" at pause —
adil buried in Vault-expansion hauling; the office session will come.
Fishery + kitchen built (137 food quarter forming); refuse pile on
the surface; second harvest banked (16 plump helmets, 24 plants);
drinks 16; WOOD 193 logs; hematite 9 + lignite 13 + iron anvil = iron
chain ready whenever furnaces go up. Wave 1 migrants NOT yet arrived
by day 100 (overseer: "soon"). Fort paused clean, zero dwarf deaths,
one dog lost to the carnotaurus.

**Session tool findings (add to engineering queue):** (6)
save_blueprint "no modifications in region" on a region this session
demonstrably dug — reads the same broken session tile-delta journal as
has_modified_anything/dug-tile counters (scope=fort was fixed by
switching to MAP STATE; save_blueprint needs the same treatment). (7)
queue_job crafts-class STILL unwired at Craftsdwarfs (MakeTool/
MakeGoblet refused "not supported at this workshop type") — manager
order path is canonical, confirmed again. (8) Wildlife invisible in
look (no creature glyph except '@' dwarves) — carnotaurus threat
assessment ran blind; only interrupt alerts betray large predators.
(9) Manager order validation can starve behind hauling for game-WEEKS
at 7 pop — consider surfacing "manager has not validated yet" state in
orders output (it reads "queued, not yet dispatched" indefinitely).

**SAME NIGHT — WAVES 6a+6b SHIPPED AND DEPLOYED (ultracode, all
Sonnet agents, review-gated; investigation adversarially verified).**
Root causes CONFIRMED and fixed: (1) name_place/save_blueprint/
check_goals all read session-delta overlays that are reconnect-wiped
and fed by a TILE_UPDATE stream that empirically delivers nothing —
migrated to live MAP-STATE (name_place seeds topology from live
map_slice; save_blueprint + predicates consume a new region_scan
plugin query); WHY the delta stream is dead remains open — step now
prints "tile deltas this step: N" to settle it live. (2) ZONE-DANCE
THEORY REFUTED BY REVIEW (the gate working as designed): the
implementer's room.extents-at-creation fix was proven a NO-OP by its
reviewer — constructAbstract already calls checkBuildingTiles(force_
extents=true) → init_extents, populating identical extents for every
civzone at creation (Buildings.cpp:842-846; civzones never take the
checkFreeTiles exclusion branch) — and the fixer verified the trace
independently and reverted zones.cpp byte-identical. The REAL cause
of any "no zone at coordinates" friction is OPEN, and the trace
suggests fresh zones should resolve immediately — next session's
designate→assign-with-NO-step test is the deciding experiment (the
dance may be part folklore, part the farm-plot stage-0 case). Also shipped: lens=wildlife ('V' dangerous/'v' tame-
or-harmless + species/danger footnote — the carnotaurus becomes
visible; tripwire honestly deferred), flat-dig z-range clamp, orders
status ladder (in progress / workshop assigned / awaiting dispatch),
connector-hint honesty gate, stocks empty-container counts, dwarves
verbose census + idle rollup, queue_job "(queue now N/10)" ACKs, vet
IPv6 fix. DLL 652,800B DEPLOYED to Steam hack/plugins (DF verified
closed first). One agent (idle-rollup) died to a network reset after
finishing its edits — hand-reviewed, test added. All builds/tests/vet
exit 0 at session close.

**NEXT-SESSION LIVE-VERIFY CHECKLIST (do these before trusting the
new surfaces):** (1) restart df-mcp fresh (old binary in memory
otherwise), ai-connect; (2) FIRST STEP: read "tile deltas this step:
N" — nonzero answers the delivery mystery, zero indicts the plugin
push path; (3) name_place the Long Street + Lanehold anchor tiles;
(4) save_blueprint the apartment comb (92,96,131)-(105,108,131);
(5) designate a throwaway zone then assign_zone IMMEDIATELY, no step;
(6) look lens=wildlife on the surface — the carnotaurus (if it still
roams) should paint 'V' with a named footnote; (7) stocks
category=barrel — expect "(N empty)" split; (8) queue_job anything —
expect "(queue now N/10)"; (9) orders — pots/goblets should show the
new ladder (and if adil ever sat, verify pot/goblet counts vs stocks
per the trust boundary); (10) dwarves verbose=true — census + idle
line. NOTE: zones created BEFORE this deploy still lack extents (DF
has likely back-filled the old ones by now, but if an old zone
misbehaves, that's why).

**IF RESUMING FORT #6:** watch orders (pots/goblets dispatch → verify
vs stocks per the order trust boundary), migrants (3 spare rooms ready,
dorm = overflow), autumn caravan (depot built, adil is broker;
bring_goods_to_depot/caravan_status still never live-tested), pig tail
plot B growing over summer. NEXT ARCS, in rough order: (a) iron chain
— wood_furnace + smelter + forge at 133 east street, hematite
INORGANIC:HEMATITE pinned SmeltOre (proven pattern from Fort #5); (b)
starter teardown — move still+kitchen+carpenter down into 137/133 and
seal the surface shops (starter-then-permanent doctrine); (c) defense
stage 3/4 at the lane mouth (mechanisms need a mechanic workshop —
none built yet); (d) satellite stair east at z=131→128 for the 5-level
dry band when housing wave 2 needs it; (e) name places when name_place
is fixed. Aquifer facts: spine pierce is the ONLY safe soil crossing
bored so far (1 wet level); SE of spine is 2-layer wet with a damp
halo at 133 — never dig 133 east of x≈98 south of y≈105.

**Economy day 58:** brewing VERIFIED (wine 4→6, fresh barrels fixed the
empty-container cancel); 2 farm plots programmed (A: plump all seasons;
B: pig tail summer/autumn + dimple cup spring/winter — pig tail refuses
spring, truthful ACK); dining pair at 137 + dining_hall zone; HOSPITAL
location founded at 137 west (meeting_hall→create_location, 2 beds in;
NOTE: DF gives animals no hospital care — the dog question answered);
trade depot built on the north plain; dorm zoned (7 beds); 7 more beds
in stock for the wing; adil auto-appointed expedition leader.

FIRST PROACTIVE FOUNDING. Fresh 192x192 embark, 7 dwarves, wagon at
(97,97,138). Terrain read FIRST per doctrine: NW plain (z=138) with a
RIVER (7/7, surface z=137) running from the north edge, bending west
across y≈44-63 and exiting west — ~40 tiles NW of the crew at its
closest (bend ~(70,58)). SE half is highlands. Between them: a z=139
grass promontory (~x=88-107) split from the east plain by a natural
SUNKEN LANE at z=138 (floor x=107, ramps both sides, y=100-106). Site
verdict: geometry language underground, terrain-hugging at the
threshold — fort dug INTO the promontory via the lane's west wall.
Named LANEHOLD (name_place tool is broken, see findings — name lives
here for now).

**Stratigraphy (bore fan, 9 columns incl. all 4 spine corners):** soil
to ~134; SOIL AQUIFER z=135 only ONE wet level at the spine site (134
merely damp) — a 1-level pierce, cheaper than Fort #5's two. Dry soil
138-136. Dry stone 133-131 map-wide; the EAST band (x≈108-112) is dry
132-128 (5 levels, mineral-rich) — reachable laterally at z=131, no
second pierce needed. DEEP STONE AQUIFER ~130/127-123 (patchy, 5-6
levels), dry mineral-rich stone below to 118+. Plan: farming 138 /
kitchens 137 / staging+future cistern 136 / pierce 135 / industry 133 /
halls 132 / housing 131 / east cluster 130-128 via satellite stair /
deep fort 122-118 someday.

**Day 14-18 execution:** spine 2x2 stairs (90-91,101-102) z138→136
(stopped one above aquifer); Long Street 3-wide (92-105,101-103,z138)
from lane wall to spine — DUG, fort has its doorway; south cross
street (95-97,104-110); starter hall W block (88-94,105-109) + first
farm E block (98-101,105-109) flanking it symmetrically. Miners:
edzul + thob + adil (3 iron picks in stock; work-details layer used,
not set_labor). Chop x12 trees → 26 sand pear logs day 15; wagon
deconstructed. Carpenter + still BUILT on the north grass (starter
buildings, disposable). Queued: 7 beds, 3 doors (queue full at 10 —
tables/chairs wait), 2x BREW_DRINK_FROM_PLANT. water_source zone on
the river bank (64-70,54-56,z138). Embark food is LEAN: 12 drink
stacks, 6 plump helmets, 30 seeds, meat/fish 3+3.

**Findings/incidents:** (1) name_place returns "no dug/open region"
on genuinely dug street tiles — BROKEN this fort; overseer says log
and play on (engineering queue). (2) My own typo passed z2=109 to a
type=default dig and the tool silently designated the 4x5 block on
EVERY level 109-138 — punched through both aquifers on paper; caught
via lens=designations, cleaned with 29 cancel_designation calls.
Tool-hardening candidate: flat dig types accepting z-ranges is a
footgun. (3) queue_job reaction path: first call timed out with the
still's queue empty (transport, not refusal) — verify-then-retry
worked, brews queued. (4) Brew #1 cancelled "Needs empty food storage
item" — all 15 embark barrels are full; queue fresh barrels when the
carpenter's bed run drains. Same as Fort #5's fresh-barrel workaround.
(5) Plateau is narrow: west slope is ~2 tiles of wall from the spine
at some rows — west-side rooms at z=138 must stop at x=88.

## 2026-07-19 — Fort #5, session 3 (Winter y100 day 304 → Spring y101 day 33, PAUSED mid-tavern)

WAVE-5 LIVE-VERIFICATION + the overseer's tavern challenge, ending with
the fort's SECOND AQUIFER PIERCED. Session closed deliberately (late
night); Fort #5 paused with the tavern stratum half-built. NEXT FORT:
#6, fresh embark, played by a new session with the proactive doctrine.

**Wave-5 verify scoreboard:** ~13 VERIFIED live — world-identity
dashboard field (caught DF's own seasonal autosave as a switch),
position_vacancies (squad_size renders), appoint_position (methkat =
first tool-made Bookkeeper; overseer then downshifted my nearest_100
precision — canon: precision follows the holder's skill),
set_bookkeeper_precision, create_squad (squad #18 client-visible),
edit_order (frequency AND amount in place), job_types subtype
discovery (30 weapon + 191 tool tokens), SUBTYPE-PINNED FORGING (a
FinelyCrafted iron pick by name — the gap that capped miners for two
forts is closed), caravan_status, map-state scope=fort (old-save digs
visible, honest cave caveat), orders freq rendering, smooth
stairs-warning, dig-over-stairs warning (SAVED THE SHAFT, see below).
THREE FINDINGS: assign_squad fails SYSTEMICALLY (founding dwarf +
migrants alike; squad healthy; overseer sees the squad in client —
client-staffing discriminator still untested), list_squads renders all
19 WORLD squads (needs fort-entity filter; overseer caught it), stocks
stack-units absent in every view despite shipping. Also: mugs are
GOBLETS (no ITEM_TOOL_MUG — MakeGoblet is the job; 10 rock mugs
ordered via material-pinned order path).

**The tavern challenge (overseer): a tavern below the queen's floor,
all-stockpile-sized (~12x14), street-grid planned, still+kitchen
adjacent, food/drink stockpile inside.** Geology: z=123-121 is the
deep stone aquifer, PATCHY — bore fan found 1-3 wet levels per column
(a single representative bore would have lied; auto-bore is now spec'd
in 011). Overseer: don't build IN the aquifer — pierce it. SECOND
PIERCE at the spine (61-62,43-44), stone-aquifer variant of the
protocol: stairs z=123→119 through 3 wet levels (loud damp cancel
once, then silent-cancel rounds surfaced by the repeat-aggregation —
the protocol's blindest phase is now countable), SMOOTH-seal strips
(no boulders needed, stone branch), catch basin at z=119 drained by
the overseer's spread-the-water trick (dig the landing room wide → 1/7
sheet → evaporation). STRUCK IN THE WET BAND: MAGNETITE (richest iron
ore), bauxite, carnelian, more hematite+tetrahedrite — the descent
pays for itself.

**Two incidents, both instructive:** (1) my drainage rect covered the
z=119 landing stairs — the NEW dig-over-stairs warning fired at
designation time ("4 will remove existing stairs: vertical connection
lost"), I tried to cancel but the jobs were already CLAIMED
(cancel_designation cannot reach in-flight jobs — new gap, logged),
stairs flattened, repaired with constructed block up-stairs at 1/7
water. (2) Shorter-steps-near-water coaching from the overseer
adopted mid-pierce (900-tick cycles with looks between).

**State at PAUSE (spring y101 day 33):** z=119 tavern stratum: 10x8
hall DUG (vein-walled: hematite+tetrahedrite), constructed stairs in,
water down to scattered 1/7 evaporating; DESIGNATED and being dug:
hall extensions to full 12x14-ish (67-70,40-47 + 57-70,48-51), north
service block (58-65,34-38) + doorways (60,39),(64,39) for
still+kitchen, west street (52-56,43-45), east street (71-74,43-45).
Seal state: smooth strips designated at z=123/122/121 and being
worked per overseer's client view — §9 SEAL VERIFICATION (dry shaft
bottom) NOT YET DONE. IF RESUMING FORT #5 FIRST ACTS: (1) verify seal
per aquifer-piercing §9 (cross_section the shaft columns, basin must
be dry), (2) finish tavern: furniture from the standing seasonal
orders (tables/chairs/thrones x10 cycling), food+drink stockpile in
the hall, meeting_hall zone → create_location tavern → NAME IT,
still+kitchen builds in the north block, 10 rock goblets en route,
(3) squad #18 staffing still broken — engineering, (4) spring caravan
never arrived by day 33 — watch caravan_status.

Fort #5 totals at pause: 17 living + döbar entombed, year 2, all
make-mandates ever issued fulfilled, iron industry + governance +
standing furniture orders running, queen's floor smoothed + first
masterpiece engraving, 81-tool surface with wave-5 verified.

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
