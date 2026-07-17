# Play-session setup runbook

How a human starts (or resumes) a supervised play session. The original
First Fort version of this runbook, with that milestone's exit criteria,
is archived at `docs/archive/2026-07-12-first-fort-runbook.md`.

## Setup (order matters: the df-mcp listener must exist before ai-connect)

1. Launch DF with the df_ai_protocol plugin deployed and load the fort
   (for a fresh embark: DF 53.15's extinct-species content only appears
   in freshly generated worlds, not worlds from before that update).
2. Open Claude Code in fortress/ (the .mcp.json here wires df-fortress).
   This spawns the df-mcp server, which opens the listener the plugin
   connects to — `ai-connect` has nothing to reach until this is up.
3. In the DFHack console: `ai-connect`.
   Do NOT run `ai-auto-update on` — the turn-based flow refreshes state
   at every pause/step boundary, and the legacy auto-update path is not
   teardown-safe. If the dashboard's data-age stamp reads STALE, pause
   or step to trigger a fresh push.
4. In the Claude session, call the `status` tool and confirm the
   ground-truth header reports a live connection (not NOT CONNECTED).

Kickoff prompt shape: "You're the overseer of this fort. Survey the
situation, then work toward the goals in memory/goals.md. Play turn by
turn per CLAUDE.md." (Point it at the relevant skill —
`fort-opening`/`fort-planning` — for a fresh embark.)

## Working-directory gotcha (why .mcp.json looks the way it does)

`config/orchestrator.yaml` (used by the server) is resolved relative to
the repo root, not to fortress/, so the df-mcp process must be launched
with the repo root as its working directory. Claude Code spawns the MCP
server with fortress/ as its working directory (that's where .mcp.json
lives), so `.mcp.json`'s command uses `go run -C .. ./cmd/df-mcp`: the
`-C ..` tells `go` to change to the repo root before running, which
makes the resulting process's cwd the repo root even though Claude Code
itself is open in fortress/. This was verified empirically — plain
`go run ../cmd/df-mcp` (no `-C`), run from fortress/, fails with `open
config/orchestrator.yaml: The system cannot find the path specified`,
because it only resolves the *module* from the parent dir, it does not
change the *process's* working directory. `-C` fixes that.

If `go run -C ..` misbehaves, build the binary from the repo root first:

    go build -o bin/df-mcp.exe ./cmd/df-mcp

then point `.mcp.json`'s `df-fortress` entry at a wrapper that also
forces repo-root cwd, e.g.:

    "command": "cmd",
    "args": ["/c", "cd .. && bin\\df-mcp.exe"]

(run from fortress/, `cd ..` lands in the repo root, matching where
`bin/df-mcp.exe` and `config/orchestrator.yaml` actually live). Do not
point `command` straight at `../bin/df-mcp.exe` without the `cd` first —
that inherits fortress/ as cwd and fails the same way plain `go run`
does. Both the `-C` form and the `cd &&` wrapper were run from fortress/
and confirmed to load `config/orchestrator.yaml` successfully.

## Budget note

A supervised session fits comfortably in a Max 20x 5-hour window at
~600-1200 tick steps; pause the session (game pauses too) when the
window runs out.
