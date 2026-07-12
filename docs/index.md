# DF-AI docs map

Read this first when a task touches anything you have not seen this session.

## The stateless-docs policy

Every doc in this tree has exactly one of three jobs:

1. **Guides** (`guides/`) explain WHAT a subsystem is and WHY it is shaped that way. They never state current values ("we have 38 tests", "we target DFHack 53.15-r1"); they state where to look (`go test ./...`, `git -C ../dfhack-build describe --tags`). A guide should still be correct in six months without edits. If you catch a guide stating a countable or version-shaped fact, fix the guide.
2. **Dated records** (`decisions.md`, `archive/`) carry their dates and are never rewritten. `decisions.md` is append-only; archive files are frozen history.
3. **Feature specs** (`../specs/`) record what a feature is and how it was planned. The active feature's spec is current intent; executed plans inside it are history.

For current state, run the code and tools. Never trust a doc for "how many X do we have now."

## Directory map

| Path | Contents | Trust level |
|---|---|---|
| `guides/` | Stateless subsystem references: plugin-build, mcp-server, world-model | Trust; fix if stale |
| `decisions.md` | Append-only decision log, one dated line each | Trust as record |
| `archive/` | Superseded docs (pre-MCP architecture era) | Historical only |
| `../specs/` | Numbered feature specs; `008-mcp-server/` = active architecture + implementation plan | Per-feature |
| `../fortress/` | The AI player's workspace: charter (CLAUDE.md), memory files, First Fort runbook | Gameplay state — the playing session owns it |
| `../.superpowers/sdd/` | Execution ledger + task briefs/reports for agent-driven waves (gitignored) | Session recovery map |

## Active work (verify against git, not this line)

Feature 008 (MCP architecture flip) runs on branch `008-mcp-server`; spec and plan in `../specs/008-mcp-server/`. Check `git log --oneline` and `.superpowers/sdd/progress.md` for where execution actually stands.

## When you finish something real

Append one dated line to `decisions.md` if a real decision was made. New analysis or snapshot: new dated file under `archive/` (or a new spec), never an edit to an old one.
