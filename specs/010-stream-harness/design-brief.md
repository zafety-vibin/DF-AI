# Feature 010: Stream Harness — design brief

**Status**: brief for a future brainstorming cycle — NOT an approved design.
**Gate**: begins after Feature 009's learning loop is stable across 2+ forts. Nothing here blocks 009.
**Goal**: the north star — unattended long-run play, broadcast as a watchable stream where the AI's reasoning is the content: it reacts to sieges and dinosaurs, narrates its plans, and visibly improves across forts.

## Platform reality (verified 2026-07-11, re-verify at kickoff)
Claude subscription auth covers interactive sessions only; unattended runs require the Agent SDK + API billing. The MCP server is client-agnostic — nothing rebuilds. Early streaming can be supervised subscription sessions ("streamer plays with their AI overseer" format, several hours/day within Max limits); true 24/7 is an API-cost decision informed by measured tokens/hour from 009-era sessions.

## Workstreams

### 1. Unattended runtime (Agent SDK)
Session lifecycle: launch → play loop → deliberate compaction cadence with goals/learnings re-injection (the 009 memory discipline, automated) → resume across restarts. Supervisor process: DF-alive watcher, plugin reconnect automation, MCP server restart, "the fort pauses when the mind is away" as the universal failure posture. Measure cost/hour and cache hit rates before committing to a duty cycle.

### 2. Broadcast layer (external MCP allowed here — presentation boundary per house rule)
- **Camera control** (user request, 2026-07-12): a `focus_camera(x,y,z)` plugin command (DFHack `Gui::revealInDwarfmodeMap`) + auto-focus policy — the viewport follows designations/builds/alerts so the stream image is never static. Cheap; could land late in 009 as a dev-QoL tool.
- `game_speed` control (deferred from 008) for viewer pacing during long steps.
- OBS integration: scene switching (map view / journal view / goals view), the model's turn narration as an overlay, `fortress/memory/journal.md` as a ticker. The narration IS the show — design turn output for an audience (the charter gains a "stream voice" section).
- Failure-mode screens: rate-limit "the overseer is napping," reconnect cards, crash-recovery montage.

### 3. Interaction
Twitch chat as an input MCP tool (read-only suggestions the model may consider; explicit moderation posture; never direct command execution). Milestone events (first dino siege, year rollover) as clips/markers. Viewer-visible `check_goals` scoreboard.

### 4. Realtime interrupts
MCP channel push (`claude/channel`, deferred from 008) matters again in unattended mode: siege announcements interrupt the session rather than waiting for the next turn. Combine with 009's step tripwires — the game brakes, the channel wakes the mind.

### 5. Research telemetry
Decision logs, per-fort outcome stats, and journals as publishable artifacts — the "does it actually learn?" evidence, which is also stream content (fort graveyard retrospectives).

## Success criteria
1. An 8-hour unattended session: zero human interventions, DF alive throughout, coherent journal.
2. The broadcast is legible to a DF-naive viewer for 10 minutes (camera follows action, narration explains it).
3. A siege or megabeast arrival is detected, reacted to, and narrated without prompting.
4. Cost/hour measured and sustainable for the chosen duty cycle.

## Open questions (for the 010 brainstorm)
- Streaming platform policies for AI-driven broadcasts; disclosure framing.
- Narration cadence: every turn, or milestone-triggered summaries?
- Chat influence: advisory-only vs voted goals (advisory-only until moderation is proven).
- One persistent fort vs seasons-of-forts format (deaths are content; the learning arc is the series).
