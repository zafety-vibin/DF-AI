# DF-AI — Claude plays Dwarf Fortress

An MCP server + DFHack plugin that lets a Claude session play Steam Dwarf Fortress: perceive the 3D map through semantic tools, dig and build turn-by-turn, and react to the simulation's chaos. The long-term goal is a persistent AI overseer that learns across fortresses — and eventually streams it.

```
DF (Steam) ⇄ DFHack C++ plugin ⇄ TCP binary protocol ⇄ Go MCP server ⇄ Claude session
   senses & hands                                        nervous system     the mind
```

The AI plays the way a human does: paused most of the time, thinking; then `step`ping the simulation forward half a day and reading what changed. It designates digs into hidden rock (the fog of war IS the frontier), places workshops, queues orders, and gets DF's own announcements as its feedback stream.

## Orientation

- **`CLAUDE.md`** — session bootstrap: doc policy, commands, hard-won gotchas. Start there.
- **`docs/index.md`** — doc map (stateless-docs policy: guides say WHAT/WHY; run the code for current state).
- **`specs/008-mcp-server/design.md`** — the architecture of record and why it's shaped this way.
- **`fortress/`** — the AI player's workspace: its charter, its memory files, and the First Fort runbook for supervised play sessions.

## Quick start (development)

```bash
go build ./... && go test ./...        # everything Go
go run ./cmd/df-mcp                    # the MCP server (repo root as cwd)
```

Plugin build + deploy against a DFHack source checkout: see `docs/guides/plugin-build.md`. Playing a supervised session: see `fortress/FIRST-FORT-RUNBOOK.md`.

## Status

Active development on the Feature 008 branch — check `git log --oneline` and `specs/008-mcp-server/` rather than trusting any README to be current. Earlier architecture generations (direct-API orchestrator, BDI loop) are superseded; their docs live in `docs/archive/`.
