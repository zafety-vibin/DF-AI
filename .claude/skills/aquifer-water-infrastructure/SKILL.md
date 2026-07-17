---
name: aquifer-water-infrastructure
description: PLANNED — procedure never executed, but the shutoff tooling now exists. Use when a sealed aquifer pierce (see aquifer-piercing) needs to become a deliberate water source (cistern, tap, or well) instead of a sealed-off hazard. The lever→door/hatch/bridge mechanism chain (build + link_building + pull_lever) was live-verified 2026-07-17; floodgate is pending live verification (its crafting bug is fixed but not yet deployed) and wells remain missing entirely — read this to know what's real vs. missing before promising water infrastructure, and note that no shutoff has ever been tested against actual flowing water.
---

# Aquifer Water Infrastructure (PLANNED)

This skill is aspirational. It documents the shape of the work so a future
session (once the tooling gaps below close) doesn't have to re-derive it,
and so a *current* session doesn't accidentally promise a capability that
doesn't exist. Read `aquifer-piercing` first — this skill assumes a shaft
has already been pierced and sealed.

## The governing law

Water never gets a path to the fort without a shutoff. Every tap you
design must have a way to stop the flow before you cut it — decide the
shutoff before you decide the tap, not after.

## What already exists, for free

A sealed pierce shaft from `aquifer-piercing` (§9) is already sitting on
top of a controllable water column. The wet space at the old shaft bottom,
once verified dry-and-sealed, doesn't need to be re-dug or abandoned — it
can be repurposed as a **cistern**: a below-aquifer chamber whose ceiling
is aquifer rock. Ceiling seep into a sealed chamber like this will, given
enough time, bring the chamber to a full 7/7 — the aquifer recharges it
passively as long as the ceiling area stays open above it.

## Taps (ways to draw from an aquifer or cistern) — orientation only

None of these have been executed by this team; they're recorded as
options to evaluate once the shutoff tooling below exists, not as a
validated sequence:

- **Passive ceiling seep**: leave a chamber's ceiling as unsealed aquifer
  rock and let it slowly fill on its own. Simplest, but gives no control
  over rate or a hard stop — not safe to route anywhere dwarves stand
  without a real shutoff (see Shutoffs) downstream of it, and no shutoff
  has yet been proven against flowing water.
- **Channel one-tile tap with a temporary drain**: `designate_dig
  type=channel` a single tile from the level above the aquifer, with a
  drain path already dug to carry away excess so the tap doesn't flood the
  working level while it fills. `channel` genuinely exists as a dig type
  today (unlike the shutoff mechanisms below) — the gap here is control
  at the destination end, not the digging verb itself.
- **Diagonal-gap pressure break**: use a diagonal (non-orthogonal) gap
  between a full-pressure source and a lower destination to break flow
  pressure before it reaches a room dwarves stand in. Orientation-level
  idea only; not attempted.

## Shutoffs — tooling now exists; none tested against water yet

The mechanism chain shipped with the 2026-07-15 defense-systems wave and
was live-verified 2026-07-17 (Fort #4): build a mechanic's workshop,
craft mechanisms, `build type=lever`, `link_building` (consumes 2
mechanisms; valid targets are exactly bridge/door/hatch/floodgate — the
wire protocol caps it at those 4), `pull_lever`. One lever can drive
several linked targets at once (verified: bridge+door+hatch off a single
lever). **Caveat that gates everything below: every verification so far
was mechanical (the link forms, the pull completes) — no shutoff has yet
been toggled against actual flowing water, and `look`/`cross_section`
cannot render a target's raised/lowered or open/closed state, so
triggered state must be inferred from job completion.**

- **Lever-linked door / hatch / bridge**: EXISTS and live-verified
  (2026-07-17) as a working remote toggle. There is still no standalone
  lock/forbid tool for an unlinked door — a door only becomes
  controllable by linking it to a lever.
- **Floodgate + lever (the proper water mechanism)**: build type and
  link target both EXIST in the protocol, but floodgate is **pending
  live verification**: crafting the floodgate item was blocked by a real
  bug (`ConstructFloodgate` missing from `jobTypeAllowedAtWorkshop`'s
  workshop-compatibility table in `dfhack-plugin/work_orders.cpp`),
  fixed 2026-07-17 but **not yet rebuilt/deployed/live-verified**. Do
  not describe a floodgate plan as proven until a floodgate has actually
  been crafted, built, linked, and pulled in a live fort.
- **Screw pumps**: still **MISSING** from the protocol. These matter
  most for heavy-aquifer work (active drain-while-digging) — see
  `aquifer-piercing` §7. Not relevant to light-aquifer taps, but worth
  tracking (no water-moving building type exists yet).

## Wells — also blocked

A well (block + bucket + rope/chain + mechanism, built over a clear
vertical column reaching water at depth >= 3/7) is the standard way to
give dwarves drinking access to a cistern without exposing them to the
water body directly. **There is still no well build type in the
protocol** (confirmed 2026-07-17: `link_building`'s target set is
exactly bridge/door/hatch/floodgate — no well anywhere in the wire
protocol). Don't propose "build a well over the cistern" as a near-term
plan; log it as blocked.

## Refill budget — how to think about capacity once a tap exists

An aquifer's recharge rate is governed by ceiling area exposed to it, not
by anything you can set as a number — a wider open ceiling recharges
faster, a single tile recharges slowest. There's no direct read on
recharge rate; judge it empirically the same way `aquifer-piercing` judges
seal success — by watching depth deltas over game-days via
`cross_section`/`survey_site`, not by assuming a fixed rate. Budget draw
rate against observed refill, not a guess.

## Tooling prerequisite checklist (check reality, not this list, before acting)

| Capability | Status |
|---|---|
| `channel` dig type | EXISTS (`designate_dig type=channel`) |
| Lever building + `link_building` + `pull_lever` | EXISTS — LIVE-VERIFIED 2026-07-17 against door/hatch/bridge targets |
| Door / hatch / bridge as lever-linked toggles | EXISTS — LIVE-VERIFIED 2026-07-17 (mechanically; never against flowing water) |
| Standalone door lock/forbid toggle (no lever) | MISSING — an unlinked door still cannot be closed on command |
| Floodgate building + link target | EXISTS in protocol; PENDING LIVE VERIFICATION — crafting bug (`ConstructFloodgate` absent from `work_orders.cpp`'s workshop table) fixed 2026-07-17 but not yet deployed; never built/linked/pulled live |
| Read-back of open/closed / raised/lowered state | MISSING — `look`/`cross_section` render the built glyph regardless of triggered state; infer from job completion |
| Well building | MISSING — no protocol build type |
| Screw pumps | MISSING — no protocol build type (heavy-aquifer work only; defer) |

**A verified remote shutoff mechanism now exists (lever-linked
door/hatch/bridge), and floodgate is one deploy-and-verify away** — but
no shutoff of any kind has been exercised against real water flow, and
there is no tool-side read on a mechanism's triggered state. Treat the
first live tap as an experiment with a drain path and an escape plan,
not a routine build.

This table reflects the protocol and `internal/mcpserver` as of
2026-07-17 — re-grep before trusting it stale; a floodgate live-verify
or a well build type landing is exactly the kind of change that promotes
this skill from PLANNED to real.
