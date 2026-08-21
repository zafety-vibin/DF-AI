---
name: caravan-trading
description: Use when a caravan or liaison is on the map or expected — staging goods to the depot, setting depot flags, reading trade agreements, choosing what to buy and sell, diagnosing depot access — or when planning production and exports because migrant waves keep coming up small. Covers the overshoot rule (matching the merchant's price is refused), buy-what-the-fort-cannot-make, the bin-staging reach gap, liaison timing, and why exported wealth is a growth lever rather than a score.
---

# Caravan Trading — The Loop That Sizes the Fort

Trade is not a shopping errand; it is the fort's growth thermostat. The
departing caravan reports the fort's created AND exported wealth to its
home civilization, and that report sizes future migrant waves. A fort
that trades thriftily starves its own population curve — two migration
seasons were missed live at low exported wealth before this clicked.
**Trade generously: exports are an investment in migrants, not a cost.**

One standing rule overrides everything below: **the exchange itself has
no safe headless path.** Staging, flags, and reading agreements are tool
work; the actual trade screen is executed by the overseer in the client.
PAUSE AND ASK before any exchange.

## The annual rhythm

Each trading civilization sends a caravan in its own season, and the home
civ's caravan brings a LIAISON who negotiates NEXT year's imports with
the fort's leader. This makes trade a year-scale loop: what you request
this autumn is what arrives (at a markup you agreed to) next autumn, and
what you produce all year decides what you can afford then. Plan
production toward the loop, not toward the visit.

## Before arrival

- **Read `trade_agreements` WHILE a civ is present — it reads only civs
  on the map RIGHT NOW** (live-verified truthful-empty after departure;
  the agreement is unrecoverable once they leave). Record the requested
  goods and their markup percentages in fortress memory the moment you
  see them.
- **Requested imports carry the margins.** Goods the buyer civ has asked
  for sell at their stated markup — often double default value — while
  everything else sells at par. Producing TO the request list beats
  producing more of what the fort already makes. Chase the highest
  percentage the fort can actually supply (idle skilled labor + raw
  material already surveyed).
- **Arrive with something to sell**: standing seasonal craft orders
  exist so a caravan never catches the depot empty. Note the two wealth
  levers are different — smoothing/engraving raises CREATED wealth but
  produces nothing sellable; only goods at the depot become EXPORTED
  wealth. Both feed the migration report; only one buys imports.
- **Check depot access early** (`caravan_status` renders
  walkable_from_edge plus an access diagnostic). Early caravans use pack
  animals, which can take stairs — an underground depot still trades.
  Full wagon caravans cannot use stairs at all: they need an excavated
  ramp road of wagon-passable width (three tiles — a DF engine
  constraint, not a style choice) from the map edge to the depot. Plan
  that road into the fort layout (see `fort-planning`); it does not
  retrofit well. accessible=false with walkable_from_edge=true suggests
  a width pinch along the route (vegetation regrowth is a suspected,
  unproven cause).
- A broker (`appoint_position`) is who conducts the trade when
  requested — see flags below.

## Staging

- `bring_goods_to_depot` marks goods for hauling; `depot_goods` renders
  the ours/theirs split; `unmark_trade_goods` reverses a mistake.
- Staging whole bins works (live-verified). But **reaching INTO
  containers for individual items works only for item_class=crafts** as
  of the last live check (2026-08-14) — every other class returns
  "marked 0 item(s)" for binned goods. Since the stockpile-container
  fix, nearly everything IS binned, so this silently collapses the
  fort's tradeable surface to crafts. **Probe before planning**: make
  one staging call per intended class and read the marked count; a zero
  on goods you know exist means the reach gap still stands (an
  engineering fix is queued — verify, don't assume, in either
  direction).
- Read the marked counts, not your intentions: the staged total is what
  the shopping list below is budgeted against.

## Depot flags

- `trader_requested` alone summons the broker.
- `anyone_can_trade` is a SEPARATE policy meaning any dwarf may conduct
  the trade — setting it alongside trader_requested bypasses the broker
  you meant to summon (live mistake). Set it only when broker
  availability is the actual problem.
- Unsetting flags truthfully cancels any pending trade job — the ack
  says so.

## The exchange (overseer-executed; you set the strategy)

- **OVERSHOOT the merchant's value — matching it is refused.** The
  trader needs a visible profit margin; an offer equal to the asking
  price does not clear (live-verified at the trade screen). Budget the
  shopping list at well UNDER the staged value, and stage more than the
  list seems to need.
- **Rank purchases by what the fort CANNOT produce, not by what looks
  valuable.** Anything an existing workshop chain already makes is
  wasted barter weight. What qualifies as can't-produce is fort-specific
  — live creatures and cages before capture tooling works, seeds and
  plant variety for the brew chain, books and instruments for social
  attractions, foods no local industry makes — the criterion is the
  fort's actual production map, read fresh each time.
- Tell the liaison next year's requests using the same criterion, one
  year ahead: request what the fort's PLAN cannot yet produce.

## After departure

- Note the exported-wealth figure the departing caravan reports — that
  number, plus created wealth, is the migration signal. If waves keep
  missing, this is the lever to pump.
- Unsold staged goods should return to stock — verify with `stocks`
  rather than assuming, and `unmark_trade_goods` anything left marked.
- Journal the agreement percentages and requests (they are next year's
  production brief), because `trade_agreements` cannot re-read them
  after departure.

## Verification status

Overshoot rule, broker-bypass flag interaction, whole-bin staging,
crafts-only container reach, truthful-empty agreements after departure,
and the requested-imports markup readout: all live-verified 2026-08-08
through 2026-08-14. The wagon ramp-road requirement and pack-animals-use-
stairs: overseer doctrine, not yet exercised by this project's own
depot (no wagon caravan has reached an underground depot here). The
migration-sizing mechanism (created + exported wealth → wave size) is DF
mechanics as taught by the overseer, observed only negatively so far
(low exports, missed waves) — treat the direction as solid and the
magnitudes as unknown.
