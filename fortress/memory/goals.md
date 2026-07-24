# Goals — Fort #6 "Lanehold" (world=region13, PAUSED summer y100 day 100)
# Session 1 complete: founded day 14, paused day 100. Zero dwarf deaths.
# Fort #5 PAUSED mid-tavern in its own world — resume script in its
# journal session-3 entry. WAVE 6 engineering = planning layer
# (specs/011-planning-layer/brief.md) + carry-over gaps + NEW findings
# at bottom.

## NOW (survival) — all green at pause
- [x] Aquifer pierced+sealed at spine (1 wet level z=135; BONE DRY;
      region13 soil aquifer = LIGHT, see learnings.md)
- [x] All 7 founders own 2x3 bedrooms at z=131 (+3 furnished spares)
- [x] Drink chain live (still + brew verified; 16 drinks; 20 barrels;
      rock-pot order queued = permanent container fix)
- [x] Food: 2 farm plots producing (plump/pig tail/dimple), fishery+
      kitchen built, 24 plants banked, gather sweeps done
- [x] Vault: 12x13 all-category stockpile at z=132 (full-hall extent)
- [x] no_active_hostiles — but CARNOTAURUS wildlife roams (dog lost);
      civilian-alert burrow "Lanehold Interior" (6 levels) VERIFIED

## SOON (headroom)
- [ ] WATCH orders: 3 rock pots + 5 goblets validated-not-dispatched
      (adil busy hauling) — verify vs stocks when dispatched (trust
      boundary: completion alerts can lie)
- [ ] Migrant wave 1 due: 3 spare bedrooms + dorm overflow ready
- [ ] Autumn caravan: depot built, adil=Broker; caravan_status/
      depot_goods/bring_goods_to_depot never live-tested
- [ ] Iron chain: hematite x9 + lignite x13 + iron anvil banked; build
      wood_furnace+smelter+forge at 133, SmeltOre INORGANIC:HEMATITE
- [ ] Starter teardown: move still/kitchen/carpenter underground
      (137/133), then wall or repurpose surface shops
- [ ] Defense: lane mouth = door+walls only; stage 3/4 needs a
      mechanic workshop (none yet) + mechanisms

## EVENTUAL (trajectory)
- [ ] East satellite stair 131→128 (5-level dry band) for wave-2 housing
- [ ] Deep fort z=122-118 (deep aquifer 127-123 is patchy; SE damp halo
      reaches z=133 — NEVER dig 133 east of x≈98 south of y≈105)
- [ ] River works: cistern/well at 136, mist rooms; surface compound +
      archer deck on the promontory
- [ ] Crypt level (2 coffins banked) + temple/library as pop grows

## WAVE 7 SHIPPED (2026-07-22 late, compile-verified, NOT DEPLOYED —
## next session FIRST ACTS: close DF if open → rebuild NOT needed (gate
## built it) → copy df_ai_protocol.plug.dll from the checkout's
## build/plugins/df_ai_protocol/Release/ to Steam hack/plugins/ →
## restart df-mcp → live-verify: (1) bring_goods item_class=crafts on
## BINNED goods (elven caravan spring y101 = the window), (2)
## unmark_trade_goods, (3) depot_goods ours/theirs sections, (4)
## set_depot_trade_flags trader_requested, (5) caravan_status
## walkable_from_edge, (6) trade_agreements readout (liaison agreement
## from THIS autumn should be readable!), (7) create_location fix —
## A/B the z=137 phantom vs z=130 Drakehall in client + make a fresh
## location, (8) flat-rect-over-pending-stairs now WARNS, (9) wildlife
## tripwire (a dangerous wild spawn auto-pauses the step), (10) build
## Instrument now refuses truthfully (handheld in stock), (11)
## check_goals shelter count spans all fort z-levels, (12) connector
## hint self-heals via live recheck. Everything below this line is
## the ORIGINAL findings list, kept for context:
## TRADE/DIPLOMACY GAPS (first live caravan, 2026-07-22 — overseer
## wants this wired next; findings from attempting the trade solo):
- BINS BLOCK TRADE (the big one): bring_goods_to_depot can't reach
  into containers, and finished goods auto-bin in the Vault — melbil's
  shell crafts were UNSTAGEABLE; I marked 25 worthless raw shells by
  material-substring instead. Need: whole-bin staging (vanilla's
  "bring bin") or a crafts/finished-goods class filter that reaches
  binned items.
- No UNMARK/undo for trade-marked items (8 rough gems now
  trade-reserved that lorbam (EncrustGem Lvl10, wave-2 arrival!)
  could have cut — recoverable only if unsold).
- depot_goods aggregates OUR staged goods with the MERCHANTS' import
  inventory in one list — can't audit staging vs their stock. Split.
- No trader_requested toggle (depot flag) and no diplomacy visibility:
  the liaison met adil and the whole agreement (import requests/
  export prices) happened INVISIBLY (viewscreen_meetingst unwired).
- depot accessible=false despite merchants arriving (pack animals) —
  suspect saplings regrowing on the old chop field break the
  wagon-path check; investigate/report tile cause.
- WORKED WELL: caravan_status (state/days/value/mood/liaison flag),
  bring_goods filters+value tally for loose items, staged-vs-pending
  split in depot_goods.

## Fort #6 tool findings — WAVES 6a+6b SHIPPED + DLL DEPLOYED same
## night (2026-07-22; see journal wave section + decisions.md).
## ALL compile/test-verified; live-verify checklist (10 items) in the
## journal is next session's FIRST ACT (restart df-mcp first!).
- [FIXED-deployed] name_place: live map_slice fallback seeds topology
- [FIXED-deployed] save_blueprint + check_goals predicates: new
  region_scan live query (re-capture the apartment comb after verify:
  (92,96,131)-(105,108,131) — overseer wants it, "copy pasted in rows")
- [REFUTED→OPEN] zone dance: the extents-at-creation theory was
  DISPROVEN by review (constructAbstract already init_extents's every
  civzone; patch was a no-op, reverted) — real mechanism unknown; the
  no-step designate→assign test next session decides whether the
  dance was ever real vs folklore + farm-plot stage-0 confusion
- [FIXED-deployed] lens=wildlife (V/v + danger footnote); tripwire on
  predator sighting DEFERRED (thread-context spread — future wave)
- [FIXED-deployed] flat-dig z-range clamp; orders status ladder;
  connector-hint honesty gate; stocks empty-container counts; dwarves
  verbose+idle rollup; queue_job "(queue now N/10)" ACKs
- [OPEN] WHY the TILE_UPDATE stream delivers nothing — step's new
  "tile deltas this step: N" line answers it in one live session
- [OPEN] crafts-class at Craftsdwarfs (manager-order path canonical)
- [OPEN] queue_job ack timeout once at still (verify-then-retry OK)
