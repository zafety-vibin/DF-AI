---
name: aquifer-water-infrastructure
description: PLANNED — blocked on tooling. Use when a sealed aquifer pierce (see aquifer-piercing) needs to become a deliberate water source (cistern, tap, or well) instead of a sealed-off hazard — currently undeliverable end-to-end because no shutoff mechanism exists at all (well/floodgate/lever build types are missing, and even a built door has no lock/toggle tool) — read this to know what's real vs. missing before promising water infrastructure.
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
  over rate or a hard stop — not safe to route anywhere dwarves stand until
  a real shutoff (below) exists downstream of it; today that means no
  shutoff exists at all (see Shutoffs).
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

## Shutoffs — this is the actual blocker

**Nothing in this project can shut off water flow today — not even a
door.** Read this section fully before proposing any tap to a live session;
every option below is missing at least one required piece.

- **Door (building exists, locking does not)**: `build` supports `door`
  (`BuildTypeDoor` exists and is wired end-to-end), so a door can be
  *built*. But there is no lock/unlock/forbid toggle tool anywhere in
  `internal/mcpserver` — grep for `lock`/`forbid` before trusting this
  stale, but as of the last read nothing exposes that control. An unlocked
  door does not stop water; a dwarf-passable door is not a shutoff. Do not
  describe "build a door" as a working shutoff mechanism to a live session
  — it builds the door, not the ability to close it.
- **Floodgate + lever (the proper mechanism)**: **MISSING.** There is no
  `BuildTypeFloodgate` and no lever build type in `internal/protocol` at
  all, and there is no linking/wiring command analogous to
  `assign_lodging` for connecting a lever to a floodgate or hatch. Until
  both the building types and a link command exist, there is no
  remote/reliable shutoff — don't describe a floodgate-and-lever plan to
  a live session as executable.
- **Screw pumps**: also **MISSING** from the protocol. These matter most
  for heavy-aquifer work (active drain-while-digging) — see
  `aquifer-piercing` §7. Not relevant to light-aquifer taps, but worth
  tracking as the same underlying gap (no water-moving building types at
  all yet, beyond the passive ones above).

## Wells — also blocked

A well (block + bucket + rope/chain + mechanism, built over a clear
vertical column reaching water at depth >= 3/7) is the standard way to
give dwarves drinking access to a cistern without exposing them to the
water body directly. **There is no well build type in the protocol.**
Don't propose "build a well over the cistern" as a near-term plan; log it
as blocked the same way the floodgate/lever gap is logged.

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
| Door (building) | EXISTS (`build` type `door`) — building only, see next row |
| Door lock/unlock/forbid toggle | MISSING — no tool in `internal/mcpserver`; a built door cannot be closed on command, so it is NOT a shutoff |
| Hatches | EXISTS (`build` type `hatch`) |
| Well building | MISSING — no protocol build type |
| Floodgate building | MISSING — no protocol build type |
| Lever building + link command | MISSING — no protocol build type or link command |
| Screw pumps | MISSING — no protocol build type (heavy-aquifer work only; defer) |

**No shutoff mechanism exists at all today.** Every row above except dig
type and buildings is either missing outright or (for doors) missing the
control that would make it a shutoff. Do not greenlight a live tap of any
kind — passive seep, channel, or otherwise — until at least one row in this
table moves from MISSING to EXISTS with an actual close/lock/toggle
command behind it.

This table reflects `internal/protocol/message.go`'s `BuildType*`
constants as of the last read — re-grep before trusting it stale; new
build types landing is exactly the kind of change that unblocks this
skill from PLANNED to real.
