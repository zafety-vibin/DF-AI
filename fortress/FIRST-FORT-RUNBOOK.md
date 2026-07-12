# First Fort — supervised session runbook

Setup: DF running with a FRESH embark (default 7 dwarves), plugin loaded,
`ai-connect` done. The embark must be in a FRESHLY GENERATED world — DF
53.15's extinct-species content only appears in new worlds, not worlds
generated before that update. Open Claude Code in fortress/ (the .mcp.json
here wires df-fortress; if `go run ../cmd/df-mcp` fails, `go build -o
../bin/df-mcp.exe ../cmd/df-mcp` and point .mcp.json's command at the exe).

Config path note: `config/orchestrator.yaml` (used by the server) is
resolved relative to the repo root, not to fortress/. Whether you launch
via `go run ../cmd/df-mcp` or the built `bin/df-mcp.exe`, the server
process must be started with the repo root as its working directory —
this is true for either launch method, not just the fallback. If `go
run`'s working-directory resolution misbehaves (it resolves the module
from the parent dir, which can be inconsistent depending on how Claude
Code spawns the MCP server), building the binary first and pointing
`command` at `bin/df-mcp.exe` from the repo root is the reliable fallback.

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
