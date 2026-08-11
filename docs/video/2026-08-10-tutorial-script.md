# DF-AI setup tutorial — narration script

Target ~18:45. Read the narration lines aloud; bracketed lines are production
direction and are never spoken.

Legend:
- `[SHOT: ...]` — what's on V1 (screen capture or b-roll)
- `[OVERLAY: <id>]` — a text overlay defined in `src/overlays.json`, rendered
  as an alpha `.mov` and laid on V2. Every id here must exist in that file.
- `[SEGMENT: <name>]` — its own Remotion composition. Most are opaque and cut
  into V1; `turn-loop-ring` is the one alpha segment and rides on V2.

Pace target is ~145 words/minute — tutorial pace, slower than conversational.
Word counts are noted per chapter so timing can be checked against a stopwatch
before recording. Update the timing table below after the read-through.

**Instruction style: brisk.** The capture carries the mechanics — narration
explains what a step is *for* and what goes wrong. Do not narrate clicks the
screen already shows. Exact strings live in overlays so a viewer can pause
and read rather than rewind. Silence over screen capture is fine and wanted.

| Chapter | Budget | Words | Actual (fill in after read) |
|---|---|---|---|
| Cold open | 0:35 | 92 | |
| 1 Before you start | 0:50 | 128 | |
| 2 What you're building | 1:00 | 156 | |
| 3 The game side | 2:20 | 232 | |
| 4 Claude Code | 1:00 | 96 | |
| 5 DF-AI itself | 1:45 | 205 | |
| 6 Connect it | 1:45 | 198 | |
| 7 Your first turn | 3:00 | 340 | |
| 8 When it breaks | 1:30 | 205 | |
| 9 Appendix: build it yourself | 3:00 | 352 | |
| 10 Appendix: other models | 1:30 | 233 | |
| Outro | 0:30 | 76 | |
| **Total** | **18:45** | **2313** | |

At 145 wpm, 2313 words is ~15:57 of pure speech against an 18:45 budget —
the ~2:45 difference is deliberate breathing room over screen capture,
concentrated in chapters 3, 5, and 7 where the viewer is following along.
If the read comes in much faster than 15:57, slow down rather than cutting.

---

## COLD OPEN — 0:00–0:35

[SHOT: gameplay b-roll — dwarves digging, a stairwell going down]

That fort is being dug by Claude.

[SHOT: Claude Code session capture — tool calls scrolling]

Not a script. Not a macro. It's looking at the map, deciding where to dig,
and then reading what actually happened — the same loop you'd run yourself.

[SHOT: back to gameplay, wider — the fort taking shape]

Somebody asked in the comments how to run this themselves. So here's all of
it: from a computer with nothing installed, to your own fort getting dug.

[SHOT: hold on the fort]

It takes about twenty minutes, and you can skip the last two chapters.

[SEGMENT: cold-open-lockup]

---

## CH 1 — Before you start — 0:35–1:25

[SEGMENT: chapter-card-1]
[SHOT: b-roll under narration — fort life, dwarves moving]

Four honest things before you spend twenty minutes.

One: this is Windows only. The game-side piece is built with Microsoft's
compiler, and nobody has adapted it to Mac or Linux.

Two: Dwarf Fortress costs money. It's a paid game on Steam. DFHack — the mod
framework this needs — is free.

Three: you need Claude Code, and a Claude plan. A play session runs long and
calls a lot of tools, so it wants a subscription rather than pay-as-you-go.

[OVERLAY: warn-research-software]

And four, the real one: this is research software. I change it most weeks.
Things will break in ways this video didn't predict — there's a
troubleshooting chapter near the end for exactly that.

[SHOT: hold]

Still here? Good.

---

## CH 2 — What you're building — 1:25–2:25

[SEGMENT: chapter-card-2]
[SEGMENT: architecture-explainer]

Before you install anything, here's the shape of it — otherwise half these
steps look random.

[Animation beat: Dwarf Fortress node appears ~0:04 into the segment]

Dwarf Fortress, on the left. It has no idea any of this exists.

[Animation beat: DFHack plugin node ~0:14]

DFHack is a mod framework that loads inside the game and can read and change
what's in its memory. We add one plugin to it. That plugin is the senses and
the hands — it reads the map, and it carries out orders.

[Animation beat: MCP server node ~0:28]

The plugin talks over a network connection, on your own machine, to a small
server. That server is the nervous system. It turns raw game state into
something a language model can actually reason about, and turns decisions
back into game commands.

[Animation beat: Claude Code node ~0:42]

And Claude Code is the mind. It never touches the game directly. It asks
what it wants to know, it decides, and it acts.

[Animation beat: side labels ~0:50]

Four pieces. The game, the plugin, the server, and the mind. Every install
step from here is one of those four.

---

## CH 3 — The game side — 2:25–4:45

[SEGMENT: chapter-card-3]
[SHOT: Steam store page for Dwarf Fortress, buy + install]

Piece one: the game. Dwarf Fortress on Steam. Buy it, install it, and you
can leave it alone — you won't launch it from here again.

[SHOT: Steam search for DFHack, install it]

Piece two, and this is where people go wrong. DFHack is not a mod you drop
into the game's folder. It's its own separate app on Steam, and it's free.

[OVERLAY: warn-dfhack-separate-app]

[SHOT: Steam library showing both entries, launching DFHack]

Which means from now on, this is what you launch. Not Dwarf Fortress —
DFHack. It starts the game with its own hooks already inside.

[SHOT: DFHack launcher, then DF running, then the DFHack console window]

Let it come up once so it builds its folders. That extra window is the
DFHack console. Later, you'll type exactly one command into it.

[SHOT: Steam right-click DFHack > Manage > Browse local files, into hack, into plugins]

Now find the folder we install into, because this is the other place people
lose an afternoon. DFHack is its own app, so its folder is not necessarily
inside the Dwarf Fortress folder — and there's often an abandoned one that
looks right.

[OVERLAY: verify-hack-plugins]

Here's how you know you're in the correct one: it's already full of files
ending in `.plug.dll`. Dozens of them. That's DFHack's own plugins sitting
where ours is going to go. If you're looking at a nearly empty folder,
you're in the wrong place, and nothing later will work.

[SHOT: hold on the populated folder]

Leave that window open.

---

## CH 4 — Claude Code — 4:45–5:45

[SEGMENT: chapter-card-4]
[SHOT: claude.com/claude-code, download, run installer, sign in]

Piece four — we'll come back for piece three. Claude Code is the part that
actually plays. Install it, and sign in with your Claude account.

[SHOT: terminal, typing the command]

[OVERLAY: cmd-claude-version]

Then prove it's really there. Open a terminal and ask it its version.

[OVERLAY: verify-claude-version]

A version number means you're done here.

[SHOT: a fresh terminal window]

If instead it says the command isn't recognised, close that terminal and
open a new one. The installer changes where Windows looks for programs, and
only a fresh terminal picks that up. That one trips up experienced people
too.

---

## CH 5 — DF-AI itself — 5:45–7:30

[SEGMENT: chapter-card-5]
[SHOT: GitHub repo page]

Piece three: DF-AI. You're downloading two different things here, and it's
worth knowing why.

[SHOT: green Code button > Download ZIP, unzip somewhere sensible]

This is the project itself — the configuration, the skills that teach Claude
how to think about digging, the folder you'll point Claude Code at. Unzip it
somewhere you'll find again.

[SHOT: Releases page, download the release ZIP]

And this is the release. Two programs, already compiled. Without it you'd
need a C++ compiler and the Go toolchain just to get started — this is what
makes that unnecessary.

[OVERLAY: warn-version-match]

One catch, and it's strict: a release is built against one exact version of
DFHack. Not close enough — exact. If yours has moved on, the plugin won't
load, and it won't tell you why. The last chapter covers building your own.

[SHOT: unzipping the release, opening INSTALL.txt, then merging folders]

Inside is a text file listing what goes where. Merge these folders into the
project folder you just unzipped.

[OVERLAY: warn-close-df]

[SHOT: closing DF, then copying the .plug.dll into hack/plugins]

Except the plugin. That one goes into the DFHack folder you left open — and
Dwarf Fortress has to be fully closed first, or Windows won't let go of the
file.

---

## CH 6 — Connect it — 7:30–9:15

[SEGMENT: chapter-card-6]
[SHOT: DF starting via DFHack, embark screen]

You need a fort for Claude to play. This isn't a Dwarf Fortress tutorial —
if you've never embarked before, there's a link below for that. Get yourself
to a loaded fort and come back.

[SHOT: fort loaded, dwarves standing around the wagon]

[SHOT: opening a terminal in the project's fortress folder, running claude]

Now open Claude Code — and specifically, open it in the project's `fortress`
folder. There's a config file sitting in there that starts the server for
you. That's why the folder matters.

[OVERLAY: warn-order-matters]

And the order matters more than it looks. Claude Code starts the server. The
game dials into it. So Claude Code has to be running first — otherwise
there's nothing on the other end to answer.

[SHOT: DFHack console, typing ai-connect]

[OVERLAY: cmd-ai-connect]

That's the one command in the DFHack console.

[SHOT: Claude session, calling the status tool, dashboard line appears]

Then ask Claude for status. Every response from these tools starts with a
live line of ground truth — the tick, the season, how many dwarves, whether
the game is paused.

[OVERLAY: verify-status-connected]

If that line says connected, everything is wired. If it doesn't, chapter
eight.

---

## CH 7 — Your first turn — 9:15–12:15

[SEGMENT: chapter-card-7]
[SHOT: FRESH CAPTURE — Claude session, typing the kickoff prompt]

Here's the whole thing working.

You don't give it instructions. You give it a job.

[SHOT: the prompt on screen, read it as you type]

"You're the overseer of this fort. Survey the situation, then work toward
the goals in memory dot goals. Play turn by turn."

[SEGMENT: turn-loop-ring — alpha; runs from here to the end of the chapter]
[SHOT: FRESH CAPTURE — survey_site returning, Claude reading it]

And then it goes and looks. That's the first thing it does — not dig,
look. What's the terrain, where's the water, what's under the soil.

[SHOT: FRESH CAPTURE — find_dig_site being called, a site returned]

When it wants somewhere to dig, it doesn't guess at coordinates. It asks
for a site and gets one back. The arithmetic — what's solid, what's
reachable, where a shaft actually fits — happens outside the model, on
purpose. Language models are bad at three-dimensional coordinates. So they
don't do that part.

[SHOT: FRESH CAPTURE — designate_dig, a 2x2 stairwell spanning many z-levels]

Then it designates. That's a stairwell — two by two, and it goes down
through a stack of levels in a single command.

[SHOT: FRESH CAPTURE — the map showing question marks / hidden tiles]

Notice it's digging into tiles nobody has seen. That's not a bug and it's
not cheating — that's how every fortress gets dug. The rock is hidden until
somebody digs it.

[SHOT: FRESH CAPTURE — step being called, time advancing, then repausing]

And this is the part that makes it a game instead of a script. It steps the
simulation forward — here, about a day — and the game pauses again by
itself.

[SHOT: FRESH CAPTURE — the delta / alerts coming back, stairs partially dug]

Then it reads what changed. Not what it asked for — what actually happened.
Dwarves take breaks. They get thirsty. They path badly. A day of digging
gets you part of a staircase.

[SHOT: FRESH CAPTURE — Claude choosing to step again rather than re-issue]

And the correct response to that is patience. Not repeating the order —
it's already working. That's the loop: look, decide, act, step, read. Over
and over, for as long as the fort lives.

[SHOT: FRESH CAPTURE — journal.md / goals.md being written]

It writes down what happened, too. Its own journal, its own goals, its own
lessons — in plain text files you can read.

[SHOT: CUT TO EXISTING B-ROLL — a mature fort, multiple levels, workshops, dwarves everywhere]

Which is how you get from a hole in the ground —

[SHOT: hold on the mature fort, slow pan]

— to this.

[SHOT: hold, let it breathe]

Same loop. Just a lot more turns.

---

## CH 8 — When it breaks — 12:15–13:45

[SEGMENT: chapter-card-8]
[SEGMENT: troubleshooting-table — rows appear as each is named]

Four things go wrong. They're all the same four.

[Table row 1 appears]
[SHOT: DFHack console, ai-connect not recognised]

The console doesn't know what `ai-connect` is. That means the plugin never
loaded — so either you're on a different DFHack version than the release was
built for, or the file went into the wrong `hack` folder. Go back and check
it's the one already full of other plugins.

[Table row 2 appears]
[SHOT: status returning NOT CONNECTED]

Status says not connected. Almost always the order — Claude Code has to be
running before you type `ai-connect`, because Claude Code is what starts the
thing you're connecting to. Open it, then run the command again.

[Table row 3 appears]
[SHOT: the config-not-found error]

It can't find its config file. The server started in the wrong folder. Use
the config file that came in the release and this doesn't happen.

[Table row 4 appears]
[SHOT: Windows file-in-use dialog]

And Windows won't let you copy the plugin. Dwarf Fortress is still running.
Not minimised — closed.

[SHOT: hold on the full table]

If it's none of those, open an issue. Tell me what you saw and what you
expected. That's genuinely useful to me.

---

## CH 9 — Appendix: build it yourself — 13:45–16:45

[SEGMENT: chapter-card-9]
[SHOT: the release page, highlighting the DFHack version in the title]

Skip this chapter unless you need it. You need it when your DFHack is a
newer version than any release here — because that plugin will refuse to
load, and there's no way around it.

[SHOT: Visual Studio installer, C++ workload selected]

Three things to install. Microsoft's C++ compiler — Visual Studio 2022,
and you only need the C++ workload, not the whole thing. CMake. And Go.

[SHOT: terminal, cloning DFHack]

Then a copy of DFHack's source code, checked out at the exact version tag
you're running. Exact, again. This is the whole reason the chapter exists.

[SHOT: DFHack repo, submodule update running]

It has submodules, and it needs them.

[SHOT: mklink command creating the junction, then the folder appearing inside plugins/]

Now the odd part. DFHack has no separate kit for building plugins — a plugin
compiles inside DFHack's own source tree. So you make a link, inside
DFHack's plugins folder, that points back at DF-AI's plugin code. Edit it in
either place, it's the same files.

[OVERLAY: warn-junction]

If that ever turns into a real folder instead of a link, you'll be compiling
stale code and everything will look like it worked. That one cost me two
months.

[SHOT: cmake build running, then the .plug.dll appearing in build output]

Then build it. Release configuration — a debug build won't load into the
game at all.

[SHOT: copying the fresh .plug.dll into hack/plugins, then go build]

Copy the result where chapter five put the downloaded one. And if you want
to build the server too, that's one Go command in the project folder.

[SHOT: the guide open in an editor]

Every detail here, including what to check when a new DFHack version breaks
something, is written up in the repo under docs, guides, plugin build. It's
more reliable than me saying it out loud.

---

## CH 10 — Appendix: other models — 16:45–18:15

[SEGMENT: chapter-card-10]
[SHOT: the MCP server code, tool list scrolling]

Last thing, because it's the question I get most: does this have to be
Claude?

[SHOT: terminal showing the server running]

The server side, no. It's a plain MCP server — the open standard — with
eighty-six tools and nothing Claude-specific in it. Anything that speaks MCP
over a standard connection can drive it. Other coding agents, other
editors, a script you write yourself against whatever model you like.

[SHOT: fortress/CLAUDE.md and .claude/skills/ in an editor]

What doesn't come along is everything around it. The charter that teaches
the turn protocol. The skills — how to pierce an aquifer, how to plan a
fort. The habit of keeping a journal between sessions. Those are Claude Code
features. On another harness you'd be rebuilding them yourself.

[SHOT: b-roll, fort]

And the honest part: I haven't run this end to end on a cheap model. I want
to — a fort that plays itself around the clock is the obvious next
experiment. But I'm not going to stand here and tell you it works when I
haven't tried it.

[SHOT: hold]

My guess is that tool-calling isn't the hard part. Reasoning about
three-dimensional space over hundreds of turns, and actually reading a
failure message instead of repeating the order — that's the hard part.

[SHOT: hold]

If you get one working, I'd like to hear about it.

---

## OUTRO — 18:15–18:45

[SHOT: b-roll — the fort, dwarves working]

Everything's in the repo — link below.

[SEGMENT: outro-card]

If you get a fort running, tell me what happened. Not a bug report
necessarily — what it built, where it went wrong, what it did that surprised
you. That's the useful thing, and it's how most of what you just installed
got fixed in the first place.

[SHOT: final hold on the fort]

Go dig a hole.
