# DF-AI — Claude plays Dwarf Fortress

An MCP server + DFHack plugin that lets a [Claude Code](https://claude.com/claude-code) session play Steam Dwarf Fortress for real: perceive the map through semantic tools, dig and build turn-by-turn, and react to the simulation's chaos. The long-term goal is a persistent AI overseer that learns across fortresses — and eventually streams it.

```
DF (Steam) ⇄ DFHack C++ plugin ⇄ TCP binary protocol ⇄ Go MCP server ⇄ Claude Code session
   senses & hands                                        nervous system     the mind
```

Claude plays the way a human does: paused most of the time, thinking; then stepping the simulation forward and reading what changed. It designates digs into hidden rock (the fog of war *is* the frontier), places workshops, queues orders, and reads DF's own announcements as its feedback stream. There's no scripted playbook — Claude decides what to build, when, and why, and the project's `fortress/memory/` files are its own persistent journal across sessions and forts.

## Status

Personal research project, moving fast. Multiple forts have been played end-to-end (aquifer piercing, migrant waves, mechanism/lever chains, the works) but there's no guarantee any given tool or doc is current — this is early, exploratory software, not a polished release. Built and tested on **Windows only** (the plugin build depends on MSVC); Linux/Mac DFHack builds exist upstream but this project hasn't been adapted or tested against them.

## Requirements

- **Steam Dwarf Fortress** (paid, [store page](https://store.steampowered.com/app/975370/Dwarf_Fortress/)) — the game DF-AI plays.
- **[DFHack](https://github.com/DFHack/dfhack)**, matching the exact release your Steam DF version needs. Two things from it: the packaged release (a separate Steam app, supplying the `hack/plugins/` we deploy into) and a **source checkout** at the same tag (for building the plugin — DFHack ships no SDK, plugins build in-tree; see `docs/guides/plugin-build.md`).
- **Go 1.25+**
- **MSVC 2022 (v143)** — the plugin's only supported toolchain right now.
- **[Claude Code](https://claude.com/claude-code)** (or another MCP client that can run a long-lived stdio server) and Claude API access — this is what actually plays.

## Setup

1. **Build the plugin.** Clone a DFHack source checkout as a sibling of this repo (conventionally `../dfhack-build`) at the tag matching your installed DFHack, junction `plugins/df_ai_protocol` to this repo's `dfhack-plugin/`, and build:
   ```bash
   cmake --build build --target df_ai_protocol --config Release
   ```
   Full detail, including the exact-version-match gotcha and the upgrade procedure: `docs/guides/plugin-build.md`.
2. **Deploy it.** With DF closed, copy the built `.plug.dll` into DFHack's `hack/plugins/` — the folder holding its stock plugins. Steam installs DFHack as its own app, so this may or may not live under the DF folder; `docs/guides/plugin-build.md` shows how to derive the current path.
3. **Build the MCP server.**
   ```bash
   go build ./... && go test ./...
   ```
4. **Point an MCP client at it.** This repo's `.mcp.json` (and `fortress/.mcp.json`, for playing from that directory) already wires up a `df-fortress` stdio server via `go run`. If you're using Claude Code, opening either directory picks it up automatically.
5. **Launch DF, load a fort, and connect.** In the DFHack console: `load df_ai_protocol` then `ai-connect`. In your Claude session, call the `status` tool and confirm it reports a live connection.

Playing a session end-to-end (recommended kickoff prompt, working-directory gotchas, etc.): `fortress/SESSION-SETUP.md`.

## Orientation

- **`CLAUDE.md`** — the session bootstrap every Claude Code session reads first: doc policy, commands, hard-won gotchas.
- **`docs/index.md`** — the doc map (stateless-docs policy: guides say WHAT/WHY, never a current count or version — run the code for that).
- **`specs/008-mcp-server/design.md`** — the architecture of record.
- **`fortress/`** — the AI player's own workspace: its charter (`CLAUDE.md`), its persistent memory (`memory/goals.md`, `journal.md`, `learnings.md`), and the session-setup runbook. This is gameplay state the playing session owns, not documentation to "clean up."
- **`.claude/skills/`** — procedural knowledge (aquifer piercing, farming, fort planning) written as *how to think about it*, deliberately never with baked-in dimensions or materials — the same skill should serve a peasant hut and a royal suite.

## Contributing

Issues and PRs are welcome, but treat this as a fast-moving personal project rather than a maintained library — expect churn, and no promised response time. If you build a fort with it, the `fortress/memory/journal.md` style of "what happened and why" is the kind of report that's actually useful to include.

## License

[MIT](LICENSE). Dwarf Fortress itself is not open source and is not included — you need your own copy. DFHack is its own project with its own license; see [DFHack/dfhack](https://github.com/DFHack/dfhack).

## Acknowledgments

[Bay 12 Games](https://www.bay12games.com/dwarves/) for Dwarf Fortress, the [DFHack](https://github.com/DFHack/dfhack) team for making a project like this possible at all, and Anthropic's [Model Context Protocol](https://modelcontextprotocol.io/) for the tool-calling substrate.
