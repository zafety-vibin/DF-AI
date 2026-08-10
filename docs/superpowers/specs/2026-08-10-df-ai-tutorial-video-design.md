# DF-AI setup tutorial video — design

**Date:** 2026-08-10
**Trigger:** a YouTube commenter asked how to run DF-AI themselves.
**Deliverable:** one chaptered YouTube video (~18:45, 16:9) that takes a viewer
who has *nothing* installed to a fort their own Claude session is digging.

## Audience and posture

A non-developer who watched DF-AI play and wants to run it. Assume they have
never used a terminal, never cloned a repo, and do not know what Go or MSVC
are. Assume they own a Windows PC and nothing else.

Two postures follow from that:

- **Real footage beats mockups.** A non-developer needs to see the actual
  Windows installer button, the actual DFHack console. An animated
  approximation of an installer is worse than useless when their screen looks
  different. Only the architecture explainer is fully animated.
- **Honest about roughness.** This is early research software. The video says
  so at 0:35 rather than letting a viewer discover it at minute 25.

## Scope

**In:** installing everything from zero, deploying the prebuilt plugin,
wiring the MCP server, embarking, and watching the AI take one real turn.
Plus two skippable appendices: building the plugin from source, and driving
the server from a non-Claude harness.

**Out:** teaching Dwarf Fortress. Chapter 6 spends ~60s on embark and links a
beginner video: *"come back when you have a fort loaded."* Also out: a full
play session (that is a different video for a different audience).

## Prerequisite repo work

The video cannot be recorded until these land. They are what make the
non-developer path real rather than aspirational.

### 1. Cut a GitHub release

Pinned to **DFHack 53.16-r1.1** (the tag currently in `../dfhack-build`).
Contents:

| Artifact | Kills prerequisite | Notes |
|---|---|---|
| `df_ai_protocol.plug.dll` | MSVC 2022 + CMake | drops into DFHack's `hack/plugins/` |
| `df-mcp.exe` | Go 1.25 | drops into the repo's `bin/` |
| `fortress/.mcp.json` (exe variant) | manual config editing | see below |

Combined with GitHub's "Download ZIP" button (which kills Git), the viewer's
prerequisites collapse to **Steam DF + Steam DFHack + Claude Code** — three
normal Windows installers.

The release is version-locked to one DFHack tag and must be re-cut on every
tag bump; the plugin loader enforces an exact version-string match. The video
states this plainly and points at Appendix A as the escape hatch.

### 2. Ship an exe-variant `.mcp.json`

`df-mcp` resolves `config/orchestrator.yaml` relative to its working
directory, and Claude Code launches MCP servers with `fortress/` as cwd. The
repo keeps the developer form (`go run -C .. ./cmd/df-mcp`); the release ZIP
overwrites it with the `cd ..` wrapper form already verified in
`fortress/SESSION-SETUP.md`:

```json
{"mcpServers": {"df-fortress": {"type": "stdio",
  "command": "cmd", "args": ["/c", "cd .. && bin\\df-mcp.exe"], "env": {}}}}
```

The viewer edits no config. Zero-config is the point.

### 3. Move the live fortress memory out of version control

**Problem.** `fortress/memory/{goals,journal,learnings}.md` and
`fortress/state/places.json` are tracked. A new viewer clones and their AI's
first act is reading the Fort #6 "Lanehold" journal — goals referencing a fort
they never built, learnings from digs they never made — and `fortress/CLAUDE.md`
explicitly instructs it to trust those files. Worse, every later `git pull`
conflicts on the player's own fort diary, which is exactly the update path the
video recommends.

**Fix.**

- Move the existing journals to `docs/example-fort-journals/` — still tracked,
  still public. They are the best storytelling artifact in the repo and the
  README already points at that reporting style.
- Gitignore `fortress/memory/*.md` and `fortress/state/`.
- Ship empty seed templates so a fresh clone starts clean.

Then `git pull` is always conflict-free and "pull often for tooling updates"
becomes safe advice.

*Future, out of scope:* accepting other players' fort logs and learnings back
via PRs.

## Video structure

~18:45, chaptered so both appendices are skippable.

| # | Chapter | Time | Footage | Key beat |
|---|---|---|---|---|
| — | Cold open | 0:00–0:35 | gameplay + session b-roll | "Not a script. It looks at the map, decides, and reads what happened." |
| 1 | Before you start | 0:35–1:25 | Remotion card | Windows only. Paid DF. Claude Code + a plan. Early research software. |
| 2 | What you're building | 1:25–2:25 | **fully animated** | DF ⇄ plugin ⇄ TCP ⇄ MCP server ⇄ Claude. *senses & hands / nervous system / the mind.* |
| 3 | The game side | 2:25–4:45 | capture | Steam DF; **DFHack as a separate Steam app** (2346660); launch once via `launchdf.exe`; locate `hack/plugins/`. |
| 4 | Claude Code | 4:45–5:45 | capture | Install, sign in, `claude --version`. |
| 5 | DF-AI itself | 5:45–7:30 | capture | Download repo, grab release, `.plug.dll` → `hack/plugins/`, `df-mcp.exe` → `bin/`. |
| 6 | Connect it | 7:30–9:15 | capture | Embark (60s, linked out). Then **order matters**: DF loaded → Claude Code open in `fortress/` → `ai-connect` → `status`. |
| 7 | Your first turn | 9:15–12:15 | existing b-roll | Payoff. Kickoff prompt → `survey_site` → `find_dig_site` → `designate_dig` → `step(1200)` → read the delta. |
| 8 | When it breaks | 12:15–13:45 | capture + card | Top four failures (below). |
| 9 | Appendix A: build it yourself | 13:45–16:45 | capture | MSVC, DFHack checkout at tag, junction, cmake. "You need this when DFHack moves past the release." |
| 10 | Appendix B: other models | 16:45–18:15 | Remotion + capture | Standard MCP. What you lose. Untested on cheap models — said plainly. |
| — | Outro | 18:15–18:45 | b-roll | Repo link; a `journal.md`-style write-up is the useful bug report. |

### Chapter 1 — cost framing

Honest but accurate. A supervised session is tool-heavy and long-context and
wants a Claude subscription plan; it fits comfortably inside a Max 5-hour
window. Do **not** overstate it as consuming a whole window.

### Chapter 8 — the four failures

1. Plugin never loads — version mismatch, or deployed to the orphaned `hack/`
   under the DF folder instead of DFHack's own `installdir`.
2. `status` reports NOT CONNECTED — `ai-connect` run before Claude Code opened
   the listener. Order matters.
3. `config/orchestrator.yaml` not found — wrong working directory.
4. DLL copy fails — DF still open, file locked.

### Appendix B — what is actually true

`cmd/df-mcp` is a plain `mcp.StdioTransport` server exposing **86 tools** with
no Claude-specific MCP features: no sampling, elicitation, resources, or
prompts — only tools plus a ~300-byte `Instructions` string. Any MCP client can
drive it.

What does *not* port: `fortress/CLAUDE.md` (the charter — other harnesses need
it in a system prompt), `.claude/skills/` (five procedural skills with no
equivalent in most harnesses), and the memory-file discipline (needs harness
file tools, e.g. a filesystem MCP server).

The chapter states that cheap-model 24/7 operation is **untested**. The known
hard part is not tool-calling but long-horizon spatial reasoning and the
truthful-ACK discipline — reading `PARTIAL`/`FAILED` and adapting instead of
re-issuing.

## Production model

```
V2  [Remotion ProRes 4444 alpha .mov overlays]
V1  [screen capture — real installs — and existing b-roll]
A1  [voiceover, recorded to a word-for-word script]
```

Cut in **Premiere Pro**. Narration is the user's own voice from a full script
marked up with shot cues and overlay timings.

Full-frame Remotion segments (cold open, chapter cards, architecture explainer,
troubleshooting table, outro) render as opaque MP4 and cut into V1. Overlay
elements render as ProRes 4444 alpha `.mov` onto V2.

## Remotion project

**New sibling project: `Projects/Remotion Videos/df-ai-tutorial/`.**

Structure copied from the existing `grimoire-demo-overlays/` — which is already
purpose-built for *"transparent ProRes 4444 overlay layers composited atop raw
screen-capture footage in the editor"* and carries a WebM/VP9 alpha fallback
script as insurance if Premiere fights the ProRes. Own `package.json`, own
`theme.ts`. Nothing outside `Remotion Videos/` is touched, and no DF-AI edit
can break a Grimoire render.

### Palette

Near-black slate ground. **Amber/ochre** for everything dwarf-world (game,
fort, plugin); **cold cyan** for everything machine-mind (Claude, MCP server,
tool calls). The architecture diagram then reads at a glance — amber left,
cyan right, TCP bridging. Mono display face throughout: DF is a terminal game,
and command text and UI text share a family.

### Components

Full-frame segments: cold-open lockup, chapter card (×10), **architecture
explainer** (~60s, the largest single build), troubleshooting table, outro.

Alpha overlays: **command callout** (mono chip with the exact string to type,
sticky while they type it); **verify strip** ("you should see:
`go version go1.25.x`" — the highest-value overlay in the video for a
non-developer); **warning card** (corner-anchored amber alert); **lower-third**
(step counter); **turn-loop ring** (pause → observe → act → step, pulsing the
active phase over session b-roll).

### Manifest-driven rendering

`dfai-overlays.json` holds `{id, type, text, subtext, durationInFrames}`
entries. `DFAI_Overlays.tsx` provides one parameterized component per *type*. A
render script loops the manifest and emits `out/dfai/<id>.mov`.

Rationale: the tooling ships weekly and the release tag will bump. Editing one
JSON line and re-rendering one file beats hunting a changed command through
thirty hardcoded compositions.

## Capture shot list

Fresh capture required (clean VM or reset machine, 1080p or higher):

- Steam store → DF purchase/install; DFHack install as a separate app
- First `launchdf.exe` launch; the `hack/plugins/` folder in Explorer
- Claude Code install and sign-in; `claude --version`
- GitHub repo page; Download ZIP / clone; release page and asset download
- Dropping `.plug.dll` and `df-mcp.exe` into place
- DF embark (short); DFHack console `ai-connect`; Claude Code `status`
- Each of the four failure modes, reproduced deliberately
- Appendix A: MSVC/CMake install, DFHack clone at tag, junction, cmake build

Already in hand: gameplay footage of the AI digging, and Claude Code session
footage of tool calls.

## Open items

- Exact release tag/version string for the pinned build.
- Which DF beginner-embark video to link in chapter 6.
- Whether Appendix A also ships as a standalone short later.
