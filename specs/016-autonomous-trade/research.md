# Feature 016 — Autonomous trade & diplomacy commit (research brief)

**Status:** research only, 2026-08-14. No production code written. This is the
"close it eventually" investigation into the last human-required action in the
fort loop: executing the trade exchange with a caravan, and conducting the
liaison meeting.

**Provenance.** Source dive against the DFHack checkout at
`C:\Users\zmanl\Projects\dfhack-build` (tag `53.16-r1.1`, commit `b638b59d`)
plus a web/community sweep. Evidence tiers used throughout:
**(a)** = read directly from that checkout (file:line cited) or official DFHack
docs; **(b)** = community/wiki claim (URL cited); **(c)** = inference, labeled.
Anything unproven is marked **[UNVERIFIED]**.

**Relationship to prior verdicts.** `docs/decisions.md` 2026-07-19 and
`specs/012-trade-diplomacy-and-papercuts/brief.md` item 6 concluded "no safe
non-viewscreen API exists; stays human." That conclusion was **correct and is
reconfirmed** for the direct-state-mutation path (§4). But both passes flagged
the viewscreen-driving path as "untested territory this project has no evidence
either way on" — **that is now resolved with evidence**: DFHack itself executes
the trade commit via synthetic mouse input in production (§3.1), and the DF 50+
trade screen turns out not to be a viewscreen at all, which removes the
push-a-screen-from-headless-context problem entirely (§2.1). One correction to
the 012 brief's terminology: `viewscreen_tradegoodsst` **does not exist** in
53.16's structures — that was 0.47-era vocabulary.

---

## 1. Executive verdict

1. **The trade commit is automatable**, via synthetic input against the vanilla
   trade window — the same mechanism DFHack's shipped `confirm` plugin uses
   every time a user confirms a trade. Selection state and the negotiation
   outcome are plain readable/writable memory. Confidence: high on mechanism
   (in-tree production precedent), medium on our threading context (strong
   precedent, one residual unknown), low-medium on programmatic screen *entry*
   (the one genuinely unproven step, §3.6).
2. **The direct-mutation path (move items + fake the counters, no screen) must
   be refused permanently.** It is the silent-accounting-desync scenario: a
   trade that looks complete locally but never registers exported wealth, which
   would shrink migrant waves and civ relations while reporting success — the
   exact opposite of this project's truthful-ACK doctrine (§4).
3. **Diplomacy is more automatable than trade**: the liaison-meeting import
   priorities are directly writable ints with an in-tree precedent; only the
   meeting's Done/advance buttons need the same synthetic-click machinery, and
   skipping the meeting entirely is already survivable (Fort #6 lived through
   an invisible one) (§5).
4. Recommended: staged throwaway-save experiments (§8), starting with a
   zero-plugin-code console reproduction that proves or kills the commit
   mechanism in under an hour. Not for the current deploy.

---

## 2. Mechanism: how DF 50/53 actually models the trade commit

### 2.1 There is no trade viewscreen anymore

The 53.16 df-structures define only ~24 viewscreen classes (title, legends,
loadgame, dwarfmode, …) — `viewscreen_tradegoodsst`, `viewscreen_tradelistst`,
and `viewscreen_meetingst` are **absent** (verified by grep across
`library/xml/*.xml`; the surviving list is in `df.d_interface.xml:6042-7305`
and `df.adventure_log.xml:152`). The Steam-era trade UI is a **window inside
`main_interface`**, rendered and fed by `viewscreen_dwarfmodest` — which is
*always the current viewscreen* during fort play:

- `trade_interfacest` struct — `library/xml/df.d_interface.xml:875-932`,
  instantiated as `game.main_interface.trade` (`df.d_interface.xml:5486`).
- DFHack derives the focus string `dwarfmode/Trade/Default` purely from
  `main_interface.trade.open && !choosing_merchant` —
  `library/modules/Gui.cpp:660-668`. Diplomacy focus strings
  (`dwarfmode/Diplomacy/{Requests,ElevateLandHolder,Default}`) same way at
  `Gui.cpp:679-689`.

Consequence: **no viewscreen ever needs to be pushed or popped** to trade. The
"can a plugin push a viewscreen from a headless background context" question
from the 2026-07-22 verdict is moot for this feature.

### 2.2 The trade window's state (all readable, selection writable)

`trade_interfacest` (`df.d_interface.xml:875-932`) carries the entire session:

| field | meaning |
|---|---|
| `open`, `choosing_merchant`, `merlist` | window open; merchant-picker subscreen |
| `st`, `bld`, `mer`, `civ` | site, depot building, `caravan_state*`, civ entity |
| `merchant_trader`, `fortress_trader` | the two negotiating units |
| `stillunloading`, `havetalker` | gates: goods unloaded, trader present — DFHack's own trade UI button enables only when `stillunloading==0 and havetalker==1` (`scripts/internal/caravan/trade.lua:788`) |
| `good[2]`, `goodflag[2]`, `good_amount[2]` | both inventories: index 0 = caravan, 1 = fort; per-item `trade_interface_good_flag` (`selected`/`contained`/`container_collapsed`/`filtered_off`, `df.d_interface.xml:869-874`) |
| `talkline` | **the merchant's reply — the machine-readable commit outcome** (§2.4) |
| `counter_offer`, `counter_offer_item` | merchant counteroffer state |
| `buildlists` | `int8_t`, semantics unknown — plausibly a rebuild-lists trigger; no DFHack code touches it **[UNVERIFIED]** |

Selection is **plain data**: DFHack's shipped trade overlay toggles items for
trade by writing `trade.goodflag[side][idx].selected` directly
(`scripts/internal/caravan/trade.lua:443-454`), and its container helpers
rewrite `selected`/`container_collapsed`/`i_height`/`scroll_position_item`
wholesale (`trade.lua:540-674`). Writing selection state is
production-proven, not an experiment.

### 2.3 The caravan's ledger: `caravan_state`

`df.plotinfo.xml:441-471` (original `plot_merchantst`), reachable via
`df.global.plotinfo.caravans` (`df.plotinfo.xml:821`):

- `trade_state` (None/Approaching/AtDepot/Leaving/Stuck), `time_remaining`
- `mood` (original `tolerance`, init 50, "reflects satisfaction with last
  trading session"), `haggle_fail_count`
- `import_value` (goods they brought), `export_value_total` (goods we gave),
  `export_value_personal` ("excluding foreign-produced items"), `offer_value`
  (gifts)
- `activity_stats` — an `entity_activity_statistics` compound, original name
  **`report`** — the snapshot carried home (§2.5)
- `sell_prices` / `buy_prices` — the concluded liaison agreements
  (`entity_sell_prices`/`entity_buy_prices`, `df.civagreement.xml:46-52`)
- `flags` (`plot_merchant_flag`, `df.plotinfo.xml:430-440`): `check_cleanup`
  ("set each time a merchant leaves the map or dies"), `casualty`, `hardship`,
  **`communicate` ("send data to mountainhomes")**, `seized`, `offended`,
  `greatly_offended`, `tribute`.

DFHack's `caravan.lua` command set writes `trade_state`, `time_remaining`, and
`flags.whole=0` directly with no ill effect (`scripts/caravan.lua:52-117`) —
caravan *metadata* mutation is established practice. The *exchange* is not.

### 2.4 What pressing Trade actually does, and where

The commit logic is **closed-source binary code inside the dwarfmode click
handler**. There is no exported function, no structure field meaning "execute
the exchange", and no fort-mode keyboard binding: the interface_key enum's only
trade key is adventure-mode `A_BARTER_TRADE`
(`library/xml/df.g_src.keybindings.xml:484`). The Trade/Offer/Seize controls
are **mouse-only vanilla buttons**, whose screen rects DFHack maintains in its
confirm specs (`scripts/internal/confirm/specs.lua`):

- Trade button: `{l=0, r=23, b=4, w=11, h=3}` (`specs.lua:168-177`,
  id `trade-confirm-trade`)
- Offer button: `{l=40, r=5, b=4, w=19, h=3}` (`specs.lua:190-199`)
- Seize button: `{l=0, r=73, b=4, w=11, h=3}` (`specs.lua:179-188`)
- mark/unmark-all buttons at `b=7` (`specs.lua:115-157`)

Frames are relative to the interface rect
(`gui.get_interface_rect`, `library/lua/gui.lua:124-134`: window size clamped
by `init.display.max_interface_percentage`, min width 114, centered) and
resolved by `gui.compute_frame_rect` (`gui.lua` — `l`/`r`/`b` are edge gaps,
`w`/`h` sizes). All reimplementable in plugin C++ from `gps` +
`init.display` fields.

**The handler runs synchronously inside `feed()`** — evidence (c, from shipped
behavior): DFHack's confirm propagation (§3.1) calls
`screen->feed(&keys)` once, with the synthetic mouse-button state restored
immediately after the call returns (`library/lua/gui.lua:94-104`), and the
trade completes; the reply state below is populated for the very next render.
No sim tick is required for the commit itself (the confirm prompts are
`pausable=true` — they fire and propagate while the game is paused).

**The outcome is machine-readable**: `trade.talkline` is a `talk_line_type`
(`df.plotinfo.xml:239-280`) covering the complete reply vocabulary — `Trade`
(accepted), `Offer`, `Seize`, `Haggle`, `CounterOffer`,
`CouldNotFindCounterOffer1/2`, `ICannotAfford`, `YouCannotAfford`,
`NoMoreTradeHaggleFailure` (mood exhausted), `AnimalReject`, `TreeReject`,
`LiveAnimalReject`, `ReceivedGift`, weight warnings, etc. Combined with
`counter_offer`/`counter_offer_item`, `caravan_state.mood`/
`haggle_fail_count`/`export_value_total`, and per-item `flags.trader`
(original `NOTYOURS`, `df.item.xml:404`) / `flags.foreign` (`NONWEALTH`,
`df.item.xml:403`), every commit attempt's true result is observable — a
perfect fit for truthful ACKs.

### 2.5 The accounting that must not desync (the migration crux)

`entity_activity_statistics` (original **`reportst`**,
`library/xml/df.report.xml:8-60`) is DF's fort-census structure and contains
`wealth.total/weapons/armor/…/imported/**offered**/**exported**`
(`df.report.xml:31-43`). It appears in three places:

1. `df.global.plotinfo.tasks` (`df.plotinfo.xml:852`, original `status`) — the
   fort's own live ledger.
2. `caravan_state.activity_stats` (`df.plotinfo.xml:453`, original `report`) —
   the snapshot the caravan carries.
3. `historical_entity.activity_stats` (`df.entity.xml:1682`, original
   **`lastreport`**) — the parent civ's copy, i.e. what the mountainhomes
   "know" about the fort; delivery is gated by the caravan's `communicate`
   flag ("send data to mountainhomes") and presumably requires the caravan to
   leave alive.

Community mechanics (**(b)** — dwarffortresswiki.org refused connections during
this pass; claims are from search snippets of
`https://dwarffortresswiki.org/index.php/DF2014:Immigration` and
`https://dwarffortresswiki.org/index.php/Wealth`, and are v0.47-namespace text
**[UNVERIFIED for v50 specifically]**): migrant wave size from the third wave
onward is driven by fortress wealth *as reported by the last outgoing dwarven
caravan*; items made in the fortress that leave on a caravan count as exports;
offerings (gifts) matter separately for relations and monarch arrival. Nuance
worth keeping: *created* wealth dominates wave sizing — so failing to trade is
survivable; what is **not** survivable is a fake trade that strips the fort of
goods without the binary ever writing the export/offer ledgers or the civ
relationship effects. That asymmetry is why §4 refuses direct mutation.

---

## 3. Option A — drive the vanilla trade window with synthetic input

### 3.1 The existence proof: DFHack already commits trades synthetically

`scripts/confirm.lua:114-147`: when its overlay intercepts a click on the
Trade/Offer/Seize button, it *swallows the real click*, shows a dialog, and on
"Ok" replays the commit:

```lua
if keys._MOUSE_L then
    df.global.gps.mouse_x = mouse_pos.x
    df.global.gps.mouse_y = mouse_pos.y
end
self.simulating = true
gui.simulateInput(scr, keys)      -- scr = the real dwarfmode viewscreen
self.simulating = false
```

The click that ultimately executes every confirmed trade in every DFHack
install **is a synthetic one**. This is the strongest possible evidence tier:
shipped, default-enabled (`trade-confirm-trade` etc.), exercised by thousands
of users on exactly our DF/DFHack versions.

### 3.2 The machinery underneath

- `gui.simulateInput` (`library/lua/gui.lua:40-104`): accepts interface_key
  names **and pseudo-keys** `_MOUSE_L/_MOUSE_R/_MOUSE_M` (+`_DOWN` variants),
  implemented by temporarily setting `df.global.enabler.mouse_lbut` etc. plus
  `enabler.tracking_on=1`, calling the feed, then restoring.
- `dscreen._doSimulateInput` (`library/LuaApi.cpp:3262-3296`): builds the
  `std::set<df::interface_key>` and calls **`screen->feed(&keys)`** — a plain
  virtual method call, synchronous, no thread affinity beyond DF-state access
  rules. Mouse *position* comes from `df.global.gps.mouse_x/mouse_y` (tile
  coords), which callers set beforehand.
- Target screen: `Gui::getDFViewscreen(true)`
  (`library/include/modules/Gui.h:214`) — the top *real* DF screen, which in
  fort mode is always `viewscreen_dwarfmodest`.

### 3.3 More in-tree precedents (breadth of the pattern)

- `scripts/load-save.lua:1-40` — non-interactive startup command: writes
  vanilla screen selection state directly, then `gui.simulateInput(screen,
  'SELECT')` to drive title → loadgame. Proof that state-write + synthetic key
  from a *command context* (not a UI hook) deterministically drives vanilla
  screens with no human present.
- `scripts/hide-tutorials.lua:32-34` — sets `gps.mouse_x/y` then
  `simulateInput(scr, '_MOUSE_L')` to click vanilla main_interface popup
  buttons. The exact shape needed for the Trade button.
- `scripts/internal/caravan/trade.lua:443-454` — direct `goodflag` selection
  writes (§2.2).
- `scripts/internal/caravan/tradeagreement.lua:61-81` — direct diplomacy
  priority writes (§5).
- Historical (**(b)**, verified by GitHub code search): BenLubar's df-ai ran
  **fully autonomous trades** in the 0.42-0.47 era — `trade_manager.cpp`'s
  `PerformTradeExclusive` fed `interface_key::TRADE_TRADE` to the old
  `viewscreen_tradegoodsst`, iterated `counteroffer`, and its CHANGELOG
  documents the strategy ("offering at least 110% of the requested value,
  adding offerings or removing requests each time the trade is declined").
  `https://github.com/BenLubar/df-ai` (last push 2022-10-10, pre-Steam; none
  of it ports — the fort-mode `TRADE_TRADE` key no longer exists — but it
  proves the negotiation loop is tractable for an agent, and its
  exclusive-callback architecture is the right mental model). Its liaison
  handling only ever *dismissed* the meeting screens, never conducted them.

### 3.4 Reachability from our architecture (the threading question)

Our plugin serves commands from a **socket thread** and drains via
`drain_from_socket_thread` under `CoreSuspender`; `plugin_onupdate` does not
run while paused. Assessment:

- `feed()` requires only that DF state is safe to touch — i.e. exactly the
  `CoreSuspender` we already hold. DFHack's own precedents run `simulateInput`
  from the console/init/command threads (load-save.lua at startup;
  any user typing a script into the dfhack console runs off-main-thread under
  suspension). **No main-thread trampoline and no onupdate hook is needed**,
  so the pause constraint does not bite.
- Works while paused: the confirm trade prompts are `pausable=true` and the
  input pump runs while DF is paused; the commit is click-handler code, not
  sim-tick code.
- Residual risk (**the one real unknown**): the vanilla window's *layout*
  state (`i_height`, scroll positions, list contents) is computed during
  render frames on the main thread. A commit clicked in the same suspended
  scope that opened the window might act on un-laid-out state. Mitigation:
  multi-phase operation — open, release suspension, let a frame or two render
  (the render loop runs even while paused), re-suspend, verify, click. This
  maps naturally onto our existing pause → act → observe turn protocol.
  Stage-1 of §8 tests precisely this.
- No headless path exists and none is needed: we drive the real Steam client;
  the requirements are valid `gps` dimensions and the window actually open
  (focus-string gate, §3.7).

### 3.5 The negotiation loop (what the tool would actually do)

1. Preconditions: `caravan_state.trade_state==AtDepot`, depot built, goods
   staged (existing `bring_goods_to_depot`), `trade.stillunloading==0 &&
   havetalker==1` (§2.2), and a broker/trader at the depot (the vanilla screen
   cannot be opened without one — `trader_requested`/`anyone_can_trade`
   already writable via our `set_depot_trade_flags`).
2. Enter the trade screen (§3.6 — the missing link).
3. Read both inventories from `trade.good[0/1]`; pre-validate ethics and
   banned/export-mandate items using the same logic DFHack ships
   (`scripts/internal/caravan/common.lua:297-345` animal/tree ethics,
   `:335-427` banned/risky items from noble mandates — export violations are a
   justice-system landmine, not just a mood hit).
4. Select with direct `goodflag[..].selected` writes; price the basket with the
   caravan-aware `Items::getValue(item, caravan)`
   (`library/include/modules/Items.h:171`) and
   `Items::isRequestedTradeGood` (`Items.h:190`) — this is also where the 012
   brief's read-only `propose_trade_offer` recommender slots in.
5. Set `gps.mouse_x/y` to the center of the Trade-button rect (computed from
   the confirm-spec constants + interface-rect math, §2.4) and feed
   `_MOUSE_L`.
6. Read `trade.talkline` and branch: accepted → done; `Haggle`/`CounterOffer`
   → read `counter_offer_item`, decide, accept or adjust selection and retry;
   `ICannotAfford` → shrink basket; `NoMoreTradeHaggleFailure` → stop
   truthfully (mood exhausted; caravan_state.mood/haggle_fail_count confirm).
   The counteroffer-accept control's rect is not in the confirm specs —
   **[UNVERIFIED, empirical]** (Stage-0 measures it).
7. Exit via `_MOUSE_R`/`LEAVESCREEN` feed (proven keys for this screen — the
   `trade-cancel` confirm spec intercepts exactly those,
   `specs.lua:106-113`).
8. ACK with the talkline verbatim + value deltas; never soften.

**Policy: never automate Seize.** Its button sits on the same row; a rect
error away. Guard by verifying the computed rect against the never-overlapping
Trade frame and refusing to click when `trade_goods_any_selected(0)`-style
checks suggest the merchant side is selected in a seize-shaped way.

### 3.6 The missing link: programmatic screen ENTRY

No interface_key opens the trade screen, and **no in-tree code opens any
vanilla main_interface window by writing `.open=true`** (grep across
`scripts/` found none). Candidates, in order of preference:

- **E-a: synthetic click chain through the depot sheet.** The depot's building
  sheet is a known context (`dwarfmode/ViewSheets/BUILDING/TradeDepot` — the
  `depot-remove` confirm spec, `specs.lua:230-239`, which also shows
  building-sheet buttons hit-test via `main_interface.current_hover` against
  `main_hover_instruction` ids). The sheet's **Trade button has no hover id**
  (the `BUILDING_SHEET_*` enum, `df.d_interface.xml:3918-4001`, lacks a trade
  entry), so its rect must be measured empirically once. Opening the sheet
  itself can likely be done by clicking the depot tile (or possibly by seeding
  `view_sheets` state — also unproven). Multi-step and layout-dependent, but
  every step is the already-proven click mechanism.
- **E-b: seed `trade_interfacest` directly** (`open=true`, `mer`, `bld`,
  `civ`, possibly `buildlists`) and let the binary populate the rest on the
  next render. Zero precedent anywhere; the field inventory (§2.2) makes it
  *plausible* the binary rebuilds `good`/`goodflag` itself (it must do so on
  legitimate opens). **[UNVERIFIED — throwaway-save experiment only; treat as
  crash-capable until proven.]**
- **E-c: fallback — human opens the screen once, AI does everything else.**
  Even this is a large autonomy win over today (human currently does the whole
  negotiation), and it de-risks shipping in stages.

### 3.7 Safety rails specific to Option A

- **Focus-string gate before any feed**: `Gui::getFocusStrings` must contain
  `dwarfmode/Trade/Default` (exact mechanism DFHack overlays use). Feeding
  clicks at trade-button coordinates while a different window is open executes
  *some other real action* — this is Option A's actual worst case (wrong-verb
  execution, e.g. Seize = diplomatic incident), not save corruption.
- **Rect provenance**: derive button rects from DFHack's own maintained
  constants (vendored per DFHack tag — we already hard-match the tag for the
  plugin ABI, so a tag bump forces re-verification by construction; or read
  `specs.lua`'s REGISTRY via DFHack's Lua interop at runtime).
- **Mandatory post-click talkline read in the ACK.** A click that produces
  `talkline==NONE` and no state delta is reported as FAILED, never retried
  blind.
- **Single-writer**: only under CoreSuspender, never concurrent with a running
  `step`.

---

## 4. Option B — direct state mutation (REFUSED) and Option C (REFUSED HARDER)

**Option B: perform the exchange by hand** — flip `flags.trader` both ways,
move items between depot and caravan animals/wagons, add/remove ownership
refs, bump `export_value_total`, write `plotinfo.tasks.wealth.exported`, set
mood…

What the binary's handler actually touches (partial, and that's the point):
item flags and ownership/containment on both sides including bin contents;
merchant unit/pack-animal inventories (so the goods physically leave when the
caravan walks off); `caravan_state` value counters + `mood` +
`haggle_fail_count`; the reply/announcement stream; broker skill XP; the
fort ledger `plotinfo.tasks.wealth.exported/offered` (update timing
**[UNVERIFIED]** — possibly at commit, possibly at departure/bookkeeping);
the caravan's carried `report` snapshot and the `communicate` handoff to
`historical_entity.activity_stats` at departure; civ-relationship effects of
trades vs offerings; export-mandate justice checks. None of this is exported,
none is documented, and **no prior art exists anywhere** — the community sweep
(§6) found not one tool, script, or even forum attempt that mutates the
exchange directly, in 15+ years of DFHack.

Failure mode analysis is what kills it: most of these desyncs are **silent and
deferred**. The fort looks traded; the save doesn't crash; then next year's
migrant waves are small, the civ's `lastreport` never updated, relations
didn't move, and nothing in any tool output says why. An approach whose
failure signature is "everything reports success and the fort quietly starves
for migrants" is categorically worse than no tool — it violates the project's
core truthful-ACK value at the mechanics layer where we can't even detect the
lie. **Refuse, permanently, and record the refusal.**

**Option C: call the binary's internal commit function directly** (pattern-scan
for the handler, cast, call). Keeps the bookkeeping consistent *if* the right
function is found — but: no symbols, byte-signature fragility across every DF
patch, calling-convention guesswork, and a crash inside it takes DF down with
no DFHack guard (`CHECK_*` macros protect DFHack API calls, not raw binary
calls). Strictly dominated by Option A, which reaches the *same handler
through its supported entry point* (the input path). Refuse.

---

## 5. Diplomacy: the liaison meeting

### 5.1 Model (all (a))

The v50 meeting is `main_interface.diplomacy` (`diplomacy_interfacest`,
`df.d_interface.xml:811-851`): a **dipscript interpreter** — `dipev` points to
`meeting_diplomat_info` (`df.diplomacy.xml:14-50`, original
`diplomacy_eventst`) which carries the civ/diplomat ids, `topic_list`
(`meeting_topic` enum, `df.army_controller.xml`: `ImportAgreement`
(TAKE_REQUESTS), `ExportAgreement` (MAKE_REQUESTS), `TreeQuota`,
land-holder topics, …), the script state (`dipscript`/`cur_step` — e.g. token
`DWARF_LIAISON`, `df.dipscript.xml`), `sell_requests`/`buy_requests`
(`entity_sell_requests`/`entity_buy_requests`, `df.civagreement.xml:25-31`),
and the concluded-agreement vectors. The concluded agreements land on the
*next* caravan as `caravan_state.sell_prices`/`buy_prices` — which our
`trade_agreements` tool already reads (`dfhack-plugin/queries.cpp:4244+`).

### 5.2 What is writable today vs viewscreen-bound

- **Import request priorities are plain writable data.** DFHack's shipped
  overlay writes `dipev.sell_requests.priority[category][idx] = 0..4` directly
  (`scripts/internal/caravan/tradeagreement.lua:61-81`); confirm's
  `trade_agreement_items_any_selected` reads the same
  (`specs.lua:76-85`).
- **The commit is the vanilla "Done" button** plus the dipscript
  advance/continue buttons — same synthetic-click machinery as trade, contexts
  `dwarfmode/Diplomacy/Requests` etc. Button rects: **[UNVERIFIED, empirical]**
  (not in confirm's specs beyond the leave-interceptor).
- Export-price agreement and land-holder steps live in the same window
  (`taking_requests`, `selecting_land_holder_position` sub-states).

### 5.3 Entry is not a problem — noticing is

The meeting opens *itself* when the liaison reaches the expedition leader; the
automation problem is inverted: detect `main_interface.diplomacy.open` (or the
`Diplomacy/` focus string) promptly and act before it concludes. Fort #6's
meeting "happened invisibly" — meaning the fort survives an unconducted
meeting (whatever defaulting DF applied). So diplomacy automation is **pure
upside with a safe failure mode**, and lower stakes than trade: a botched
meeting wastes an agreement year; it cannot strip the fort of goods. Reasonable
first ship: a step-tripwire on `diplomacy.open` (pause + surface it), then
priority-writing, then the synthetic Done.

---

## 6. Prior art survey (community + DFHack history)

| tool | what it automates | mechanism | status at 53.x | tier |
|---|---|---|---|---|
| `caravan` script + overlays (modern) | staging, selection UX, ethics warnings, agreement shortcuts, caravan lifecycle (`extend`/`happy`/`leave`) | direct data writes; **never presses Trade** | shipped, current | (a) `scripts/caravan.lua`, `scripts/internal/caravan/*`; docs `https://docs.dfhack.org/en/stable/docs/tools/caravan.html` |
| `logistics` plugin | auto-**mark** stockpiled items for trade when a caravan is due | `Items::markForTrade` (`plugins/logistics.cpp:292`) | shipped, current | (a) |
| `autotrade` (0.4x) | same marking idea, pre-v50 | data layer | **removed**, "merged into logistics" | (a) `https://docs.dfhack.org/en/latest/docs/about/Removed.html` |
| `confirm` | intercepts + **synthetically replays** Trade/Offer/Seize clicks | `gps.mouse` + `simulateInput` | shipped, current, default-on | (a) §3.1 |
| BenLubar df-ai | **full autonomous trade commit + counteroffer loop**; liaison screens dismissed | synthetic keys (`TRADE_TRADE`) on 0.4x viewscreens | dead (last push 2022-10-10); does not port | (b) `https://github.com/BenLubar/df-ai` (code-search verified) |
| any v50 "force trade" tool | — | — | **none found** | (b) searches of DFHack issues/PRs, GitHub, forums returned zero; absence of evidence only **[UNVERIFIED beyond searches run]** |

Notable nulls (all (b)): no DFHack issue or feature request for unattended
trade execution exists (closest: read-only "is caravan ready to trade"
`https://github.com/DFHack/dfhack/issues/3745`, agreement-GUI
`https://github.com/DFHack/dfhack/issues/3550`); no forum/reddit claim of a
working direct-mutation trade was found.

Test utility worth knowing: `scripts/force.lua:42-58` spawns a `Caravan` or
`Diplomat` `timed_event` for the player civ on demand — cheap caravan summons
for throwaway-save testing.

---

## 7. Risk & safety verdict

| approach | save corruption | silent accounting desync | crash (given CHECK_* THROW) | truthfully detectable after the fact |
|---|---|---|---|---|
| **A: synthetic input** | **LOW** — every mutation is made by the binary's own handler, the same code a human click runs | **LOW** — same reason | **LOW-MODERATE** — `feed()` is a plain vmethod; the risks are wrong-state feeds and wrong-rect clicks, i.e. *wrong real actions*, not memory corruption. All new plugin dispatch paths keep the existing try/catch guard | **EXCELLENT** — `talkline` verbatim, counteroffer state, value counters, item flags; the ACK can quote the merchant |
| A's entry variant E-b (seed `trade.open`) | UNKNOWN | LOW (commit still via handler) | **UNKNOWN — treat as crash-capable** | good (window either opens correctly or doesn't) |
| **B: direct mutation** | MODERATE (containment/ownership graphs are easy to half-update) | **HIGH and SILENT — the disqualifier** (§4) | LOW-MODERATE | **POOR — the failure is invisible until next year's migrant wave** |
| **C: call REd internal fn** | HIGH | LOW if correct fn | **HIGH** — no guard catches a fault inside the binary | poor |

Cross-cutting fragilities of A, with mitigations:
- **Button rects shift when Bay12 redesigns the screen.** Mitigation: rects
  vendored per DFHack tag (our plugin already refuses to load on tag mismatch,
  so drift forces re-verification); mismatch → FAILED ACK, never a guessed
  click.
- **Merchant departs mid-negotiation** (`time_remaining`, `Leaving`).
  Mitigation: re-check `trade_state`/gates between every phase; all phases are
  short.
- **The overlay layer**: if the user runs DFHack's own trade overlays, our
  synthetic clicks pass through the same interpose chain the confirm plugin
  uses; confirm's own prompts would intercept *our* synthetic Trade click
  (it intercepts `_MOUSE_L` in that frame). Mitigation: disable the
  `trade-confirm-*` prompts in `dfhack-config/confirm.json` on the playing
  install, or feed through a path that bypasses interposes
  **[UNVERIFIED which is cleaner — Stage-0 will hit this immediately if the
  confirm plugin is active]**.

---

## 8. Recommendation and staged experiment plan

**Pursue: Option A** (synthetic input against the vanilla window), staged as
below, starting with entry variant E-c (human opens the screen) and graduating
to E-a/E-b only if the experiments pass. **Refuse: Option B** (direct
mutation) permanently — put it in decisions.md when this brief is acted on —
and Option C without further study. **Ship regardless of the rest**: the
read-only `propose_trade_offer` recommender (012 brief item 6), which is pure
computation and de-risks nothing but improves play immediately.

What would have to be true for an autonomous commit to be safe to ship:
1. Stage-0/1 prove the synthetic Trade click works from a command context on
   53.16 (kill criterion: talkline never populates or the click lands
   elsewhere).
2. Stage-4 proves accounting parity with a human-executed control trade.
3. The focus-gate + rect-provenance + talkline-mandatory rails (§3.7) are in
   the implementation from the first commit.
4. Seize remains unautomatable by policy.
5. The human path remains available and documented as the fallback.

### The stages (all on throwaway saves; never Fort #6+)

- **Stage 0 — kill test, zero plugin code (~1 hour).** Throwaway fort with a
  depot; summon a caravan with `force Caravan` if needed. Human opens the
  trade screen once. In the DFHack console: flip one cheap item's
  `goodflag[1][i].selected`, compute the Trade rect from the confirm-spec
  constants + `gui.get_interface_rect()`, set `gps.mouse_x/y`, call
  `gui.simulateInput(dfhack.gui.getDFViewscreen(true), '_MOUSE_L')`, then dump
  `trade.talkline`, `counter_offer`, `mer.export_value_total`. Success =
  merchant reply + value delta. Also measure: the counteroffer-accept rect,
  and whether an active confirm plugin intercepts the synthetic click (§7).
- **Stage 1 — threading proof.** Re-run Stage 0 driven entirely via
  `dfhack-run` from outside the client (command context ≈ our socket thread +
  CoreSuspender). Add the multi-phase timing variant: suspend/act/release
  between selection and click. Success = identical behavior with no human at
  the keyboard after screen entry.
- **Stage 2 — entry.** Try E-a (click chain: depot tile → building sheet →
  its Trade button; measure the sheet's Trade rect) and E-b (seed
  `trade.open/mer/bld/civ`, render a frame, inspect `good`/`goodflag`
  population; expect possible crash — throwaway). Pick whichever works; keep
  E-c as fallback.
- **Stage 3 — port into `df_ai_protocol`.** New guarded command(s)
  (`execute_trade`, probably phased: `open_trade` / `select_trade_goods` /
  `commit_trade`), socket-thread drain, try/catch guard, focus gates, talkline
  ACKs, never-Seize. MCP tool schemas stay shape-only per house rules; the
  negotiation strategy lives in a skill, not the schema.
- **Stage 4 — accounting parity (the desync detector).** On two copies of the
  same throwaway save: (i) automated trade, (ii) identical human trade. Let
  the caravan leave (verify `communicate` flag), roll to the next year's
  caravan/migrant waves. Compare `plotinfo.tasks.wealth.exported`,
  `historical_entity.activity_stats`, migrant counts. Success = no observable
  difference. Only after this does the tool leave the feature flag.
- **Stage 5 — diplomacy.** Step-tripwire on `main_interface.diplomacy.open`;
  write `sell_requests.priority` (in-tree-precedented); measure and click the
  Done/advance buttons; verify next caravan's `buy_prices` reflects the
  request. Lower stakes; can trail trade by a full development cycle.

### Suggested decisions.md line (append when this is acted on, not before)

> 2026-08-14: Trade-commit research (016) — the 2026-07-19/22 "stays human"
> verdict is REVISED: direct state mutation of the exchange stays refused
> permanently (silent migration-accounting desync), but the synthetic-input
> path is DFHack-production-proven (confirm.lua replays the Trade click;
> v50 trade is a main_interface window on the always-current dwarfmode screen,
> reachable from the socket thread under CoreSuspender) — staged throwaway
> experiments approved per specs/016-autonomous-trade/research.md.
