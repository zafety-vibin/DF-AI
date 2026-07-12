# First Fort — supervised session runbook

Setup: DF running with a FRESH embark (default 7 dwarves), plugin loaded,
`ai-connect` done. The embark must be in a FRESHLY GENERATED world — DF
53.15's extinct-species content only appears in new worlds, not worlds
generated before that update. Open Claude Code in fortress/ (the .mcp.json
here wires df-fortress).

Config path note: `config/orchestrator.yaml` (used by the server) is
resolved relative to the repo root, not to fortress/, so the df-mcp
process must be launched with the repo root as its working directory.
Claude Code spawns the MCP server with fortress/ as its working directory
(that's where .mcp.json lives), so `.mcp.json`'s command uses
`go run -C .. ./cmd/df-mcp`: the `-C ..` tells `go` to change to the repo
root before running, which makes the resulting process's cwd the repo
root even though Claude Code itself is open in fortress/. This was
verified empirically — plain `go run ../cmd/df-mcp` (no `-C`), run from
fortress/, fails with `open config/orchestrator.yaml: The system cannot
find the path specified`, because it only resolves the *module* from the
parent dir, it does not change the *process's* working directory. `-C`
fixes that.

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

Kickoff prompt: "You're taking over this fresh embark. Survey the site,
establish an underground fort, and work toward the goals in
memory/goals.md. Play turn by turn per CLAUDE.md."

## Exit criteria (Phase 1 complete when ALL hold)
- [ ] Model called survey_site / cross_section before its first dig
- [ ] A 2x2 stair shaft descends from the surface into stone (one command)
- [ ] Rooms are dug UNDERGROUND into previously-hidden tiles. The
      hidden-dig fix (F1) was already live-verified at the 2026-07-12
      checkpoint; the First Fort criterion here is that the MODEL
      exercises it end-to-end via MCP tools, unaided.
- [ ] Carpenter workshop built; beds ordered, produced, and placed in a
      zoned bedroom (wood chain: chop -> carpenter -> order -> build)
- [ ] Food/drink: farm plots zoned or gathering + still running; no
      starvation/dehydration deaths through the first migrant wave and
      into year 2 spring
- [ ] check_goals reports NOW + SOON satisfied; journal/goals/learnings
      files show real maintenance
- [ ] Human observations filed as issues: every moment the model was
      confused-by-presentation (not by DF) is a perception-tool bug

Budget note: a supervised session fits comfortably in a Max 20x 5-hour
window at ~600-1200 tick steps; pause the session (game pauses too) when
the window runs out.
