# df_ai_protocol — DFHack plugin

The game-side half of DF-AI: extracts state, executes designations /
buildings / orders, and answers queries over the binary TCP protocol
(`protocol.h`, which MUST stay in sync with the Go side's
`internal/protocol`).

**Build and deploy instructions live in `docs/guides/plugin-build.md`.**
The short version: there is no DFHack SDK — this plugin builds IN-TREE
inside a sibling DFHack source checkout, reached via a directory
junction at `plugins/df_ai_protocol`, and the artifact is
`df_ai_protocol.plug.dll` (deploy with DF closed).

An obsolete standalone `DFHACK_ROOT` build guide that used to live here
(53.02-era, plain `.dll` naming, df-orchestrator) is archived at
`docs/archive/2025-11-legacy/dfhack-plugin/README.md` — do not follow
it; that build style silently compiles stale code.
