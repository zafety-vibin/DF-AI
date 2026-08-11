# Capture shot list — DF-AI setup tutorial

Ordered for a **single recording session**, not by chapter — all Steam work
together, all terminal work together, and the deliberate failure
reproductions last because they leave the machine dirty. Chapter tags in
brackets show where each shot lands in
`docs/video/2026-08-10-tutorial-script.md`.

## Before you hit record

- **Display scaling and font size up.** Text that reads fine on your monitor
  is unreadable on a phone, and phones are most of this audience. Bump
  Windows scaling and your terminal font well past comfortable.
- 1920×1080 minimum; 2560×1440 downscaled gives crisper text if you have it.
  30fps to match the Remotion assets.
- Clean desktop, no personal notifications, browser with no extra tabs.
- A **second machine or a VM** is ideal for the from-scratch shots — the
  install steps must look like a first install, not like yours.

## Block 1 — Steam [Ch3, Ch5]

1. Dwarf Fortress store page → purchase → install
2. Steam search for **DFHack** → install (free)
3. Library view showing **both** entries side by side — this is the shot that
   sells "separate app"
4. Launching **DFHack** (not DF) from the library
5. DFHack launcher → DF starting → the **DFHack console window** appearing
6. Right-click DFHack → Manage → Browse local files → into `hack` → into
   `plugins`, ending held on the folder **full of `.plug.dll` files**. This is
   the most important frame in chapter 3 — hold it longer than feels natural.

## Block 2 — Browser [Ch4, Ch5]

7. claude.com/claude-code download page
8. GitHub repo landing page
9. Green **Code** button → **Download ZIP**
10. **Releases** page → the v0.1.0 asset → download
11. `INSTALL.txt` open in Notepad

## Block 3 — Installers and terminal [Ch4, Ch5]

12. Claude Code installer running → sign-in flow
13. Terminal: `claude --version` returning a version number
14. Unzipping both archives side by side; merging the release folders into
    the source folder
15. Copying `df_ai_protocol.plug.dll` into `hack/plugins/` **with DF closed**

## Block 4 — In game [Ch6]

16. Launching through DFHack → embark sequence → a loaded fort (keep short;
    the script links out for a real embark tutorial)
17. Opening a terminal in the project's `fortress/` folder and running
    `claude`
18. DFHack console: typing `ai-connect`
19. Claude session: calling `status`, ground-truth dashboard line returning
    CONNECTED

## Block 5 — The first turn [Ch7] — fresh capture, brand-new embark

The instructional beats must match what the viewer just built, so shoot this
on a fresh fort, not a mature one.

20. Typing the kickoff prompt
21. `survey_site` returning; Claude reading it
22. `find_dig_site` being called and returning a site
23. `designate_dig` — a 2×2 stairwell spanning several z-levels in one call
24. The map showing `?` hidden tiles being designated into
25. `step` advancing time, then the game auto-repausing
26. The delta/alerts coming back with a **partially** dug staircase
27. Claude choosing to step again rather than re-issue the order
28. `journal.md` / `goals.md` being written

## Block 6 — Appendix A build [Ch9]

29. Visual Studio installer with the C++ workload selected
30. Cloning DFHack; `git submodule update --init --recursive` running
31. `mklink /J` creating the junction, then the linked folder appearing
    inside `plugins/`
32. `cmake --build` running → the fresh `.plug.dll` in the build output
33. `go build` producing the server
34. `docs/guides/plugin-build.md` open in an editor

## Block 7 — Deliberate failures [Ch8] — SHOOT LAST

Each of these leaves something broken; do them at the end and in this order,
restoring between takes.

35. `ai-connect` typed before Claude Code is open → NOT CONNECTED
36. Copying the `.plug.dll` while DF is running → Windows file-in-use dialog
37. Plugin deployed to the wrong/orphaned `hack/` folder → `ai-connect`
    unrecognised in the console
38. Server started from the wrong directory → config-not-found error

## Already in hand — no capture needed

- Mature fort b-roll: multiple z-levels, workshops, dwarves at work. Used for
  the cold open and the chapter 7 payoff cut ("— to this").
- Claude Code session footage of tool calls scrolling, for the cold open.

## Render and import notes

*(Fill in during Task C7, after the first Premiere import: which alpha
interpretation Premiere needed for the ProRes 4444 overlays, so a re-render
months from now doesn't rediscover it.)*
