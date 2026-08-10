# DF-AI Setup Tutorial Video Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the repo changes, narration script, and Remotion overlay assets needed to record a ~18:45 non-developer tutorial that takes a viewer from nothing installed to a fort their own Claude session is digging.

**Architecture:** Three independent phases. **Phase A** changes this repo so a non-developer path exists at all — untracking the live fortress memory, seeding templates, and adding a release-packaging script that produces a ZIP containing the prebuilt plugin, `df-mcp.exe`, and a zero-edit `.mcp.json`. **Phase B** writes the narration script and, from it, the overlay cue manifest. **Phase C** builds a new Remotion project that renders that manifest into ProRes 4444 alpha overlays plus opaque full-frame segments for Premiere.

**Tech Stack:** Go 1.25 (existing), PowerShell 5.1 (packaging scripts, Windows-only), Remotion 4.x + React 18 + TypeScript 5 (new video project), `gh` CLI (release).

**Design doc:** `docs/superpowers/specs/2026-08-10-df-ai-tutorial-video-design.md`

## Global Constraints

- **Windows only.** All scripts are PowerShell 5.1. No `&&`, no ternary, no null-coalescing — see the repo's shell notes. Use `;` and `if ($?) { }`.
- **Repo has `core.autocrlf=true` and no `.gitattributes`.** Never bulk-rewrite files; keep diffs free of line-ending churn.
- **Never delete or overwrite `fortress/memory/*.md` or `fortress/state/*`.** These belong to the playing AI (`CLAUDE.md` house rule). Every script that touches them must be copy-if-absent, never copy-over.
- **Release is pinned to DFHack `53.16-r1.1`** — the tag currently checked out in `../dfhack-build`. Verify with `git -C ../dfhack-build describe --tags` before packaging; never hardcode from this doc.
- **The plugin artifact is `df_ai_protocol.plug.dll`** (`.plug.dll`, not `.dll`).
- **Never pipe build/verification output through `tail`/`head`** — pipeline exit codes mask failures. Redirect to a log and test the real exit code.
- **Remotion project style** (matching `grimoire-demo-overlays/`): no semicolons, single quotes, `type XProps = {}` with JSDoc'd props, strict tsconfig with `noUnusedLocals` and `noUnusedParameters`.
- **Video geometry:** 1920×1080, 30fps.
- **`ffprobe` must be on PATH** for the Phase C verification steps. Remotion bundles its own ffmpeg but does not expose `ffprobe`. Check with `ffprobe -version`; if missing, install ffmpeg (`winget install Gyan.FFmpeg`) or substitute the manual check noted in each step.

---

## File Structure

**Phase A — this repo (`C:\Users\zmanl\Projects\DF-AI`)**

| File | Responsibility |
|---|---|
| `.gitignore` (modify) | stop tracking live fortress memory + state |
| `docs/example-fort-journals/{goals,journal,learnings}.md` (create) | the published Fort #6 journals, kept public as the storytelling artifact |
| `fortress/memory-templates/{goals,journal,learnings}.md` (create) | tracked seeds for a fresh clone |
| `scripts/seed-fortress-memory.ps1` (create) | copy-if-absent seeding; safe to re-run |
| `scripts/package-release.ps1` (create) | build exe, gather DLL, write exe-variant `.mcp.json`, seed, zip |
| `README.md` (modify) | add the prebuilt quick-start path above the developer path |

**Phase B — this repo**

| File | Responsibility |
|---|---|
| `docs/video/2026-08-10-tutorial-script.md` (create) | word-for-word narration + shot cues + overlay cue IDs |
| `docs/video/capture-shot-list.md` (create) | what to record, in recording order |

**Phase C — new project (`C:\Users\zmanl\Projects\Remotion Videos\df-ai-tutorial\`)**

| File | Responsibility |
|---|---|
| `package.json`, `tsconfig.json`, `remotion.config.ts` | project scaffold; ProRes 4444 alpha defaults |
| `src/index.ts`, `src/Root.tsx` | entry; registers manifest overlays + fixed segments |
| `src/theme.ts` | amber/cyan palette, geometry, `seconds()` |
| `src/hooks/useFonts.ts` | `bootFonts()` — JetBrains Mono + Inter |
| `src/overlays.json` | the cue manifest (authored in Phase B) |
| `src/types.ts` | `OverlaySpec`, `OverlayType` |
| `src/components/{CommandCallout,VerifyStrip,WarningCard,StepLowerThird}.tsx` | one alpha overlay each |
| `src/components/OverlayRouter.tsx` | dispatch by `type` |
| `src/segments/{ColdOpenLockup,ChapterCard,ArchitectureExplainer,TroubleshootingTable,OutroCard}.tsx` | opaque full-frame segments |
| `src/segments/TurnLoopRing.tsx` | alpha overlay, phase-pulsing (ch7) |
| `scripts/render-overlays.mjs` | loop the manifest, render each to `out/dfai/<id>.mov` |

---

# PHASE A — Repo changes (blocks recording)

### Task A1: Untrack the live fortress memory

**Files:**
- Create: `docs/example-fort-journals/goals.md`, `docs/example-fort-journals/journal.md`, `docs/example-fort-journals/learnings.md`, `docs/example-fort-journals/README.md`
- Modify: `.gitignore`
- Untrack (keep on disk): `fortress/memory/goals.md`, `fortress/memory/journal.md`, `fortress/memory/learnings.md`, `fortress/state/places.json`

**Interfaces:**
- Consumes: nothing.
- Produces: a repo where `git ls-files fortress/` returns only `.mcp.json`, `CLAUDE.md`, `SESSION-SETUP.md`. Task A2's seeding script depends on `fortress/memory/*.md` being gitignored.

**Why this is non-destructive:** the live files stay on disk untouched. We **copy** them to `docs/example-fort-journals/` and `git rm --cached` the originals. `git rm --cached` removes from the index only. Do **not** use `git mv` or `git rm` without `--cached` — that deletes the playing AI's working memory, which the CLAUDE.md house rule forbids.

- [ ] **Step 1: Verify the current tracked state (this is the "failing test")**

Run:
```bash
git ls-files fortress/
```
Expected output — four files that should NOT be tracked:
```
fortress/.mcp.json
fortress/CLAUDE.md
fortress/SESSION-SETUP.md
fortress/memory/goals.md
fortress/memory/journal.md
fortress/memory/learnings.md
fortress/state/places.json
```

- [ ] **Step 2: Copy the live journals to the public example folder**

```powershell
New-Item -ItemType Directory -Force docs/example-fort-journals
Copy-Item fortress/memory/goals.md      docs/example-fort-journals/goals.md
Copy-Item fortress/memory/journal.md    docs/example-fort-journals/journal.md
Copy-Item fortress/memory/learnings.md  docs/example-fort-journals/learnings.md
```

- [ ] **Step 3: Write the example folder's README**

Create `docs/example-fort-journals/README.md`:

```markdown
# Example fort journals

These are the real memory files from the forts played in this repo — the
AI overseer's own goals, narrative journal, and distilled learnings. They
are published here as an example of what the memory discipline in
`fortress/CLAUDE.md` produces over many sessions, and as the reporting
style that makes a useful bug report.

They are a **snapshot, not live state.** The live files under
`fortress/memory/` are gitignored so that your fort's memory is yours and
`git pull` never conflicts with your own journal.

If you play a fort with DF-AI, a write-up in this style is genuinely
useful to include in an issue or PR.
```

- [ ] **Step 4: Remove the live files from the index (NOT from disk)**

```bash
git rm --cached fortress/memory/goals.md fortress/memory/journal.md fortress/memory/learnings.md fortress/state/places.json
```

- [ ] **Step 5: Confirm the live files still exist on disk with their content**

```powershell
Get-ChildItem fortress/memory/*.md | Select-Object Name, Length
```
Expected: three files, each with a non-zero `Length`. **If any file is missing or zero-length, stop and restore it from git before continuing** (`git checkout HEAD -- fortress/memory/`).

- [ ] **Step 6: Add the gitignore entries**

Append to `.gitignore`, after the existing `fortress/art/` line:

```gitignore
# The playing AI's live memory — yours, not ours. Seeded from
# fortress/memory-templates/ by scripts/seed-fortress-memory.ps1.
# Published snapshots live in docs/example-fort-journals/.
fortress/memory/*.md
fortress/state/
```

- [ ] **Step 7: Verify the tracked set is now correct**

Run:
```bash
git ls-files fortress/
```
Expected — exactly three files:
```
fortress/.mcp.json
fortress/CLAUDE.md
fortress/SESSION-SETUP.md
```

Run:
```bash
git status --short
```
Expected: staged deletions for the four files, new `docs/example-fort-journals/` files, modified `.gitignore`. **No untracked `fortress/memory/*.md` entries** — they are now ignored.

- [ ] **Step 8: Commit**

```bash
git add .gitignore docs/example-fort-journals/
git commit -m "refactor: untrack live fortress memory, publish journals as examples

A fresh clone was shipping Fort #6's journal, goals, and learnings as if
they were the new player's own — and fortress/CLAUDE.md instructs the AI
to trust those files. Every later git pull also conflicted on the
player's own fort diary.

Live fortress/memory/*.md and fortress/state/ are now gitignored; the
existing journals are published read-only under docs/example-fort-journals/."
```

---

### Task A2: Seed templates for a fresh clone

**Files:**
- Create: `fortress/memory-templates/goals.md`, `fortress/memory-templates/journal.md`, `fortress/memory-templates/learnings.md`
- Create: `scripts/seed-fortress-memory.ps1`

**Interfaces:**
- Consumes: Task A1's gitignore (so a fresh clone has an empty `fortress/memory/`).
- Produces: `scripts/seed-fortress-memory.ps1`, called by Task A3's packaging script and by the README quick-start. Idempotent — never overwrites an existing file.

- [ ] **Step 1: Write the goals template**

Create `fortress/memory-templates/goals.md`:

```markdown
# Goals

Three tiers. Keep this under 30 lines; rewrite freely as the fort changes.

## NOW — survival
- Nothing yet. Survey the embark and write the first real goal here.

## SOON — headroom
- Nothing yet.

## EVENTUAL — trajectory
- Nothing yet.
```

- [ ] **Step 2: Write the journal template**

Create `fortress/memory-templates/journal.md`:

```markdown
# Journal

Append-only narrative. One dated block per session, newest at top.
Two to four lines per turn is plenty — what happened and why it mattered.
```

- [ ] **Step 3: Write the learnings template**

Create `fortress/memory-templates/learnings.md`:

```markdown
# Learnings

Distilled cause-and-effect lessons that survive across forts.
Never delete an entry — append and refine.

Format: one line per lesson, cause then effect.
Example: "wagon blocks its own tile for digs — deconstruct or dig around."
```

- [ ] **Step 4: Write the seeding script**

Create `scripts/seed-fortress-memory.ps1`:

```powershell
# Seeds fortress/memory/ from fortress/memory-templates/ on a fresh clone.
#
# COPY-IF-ABSENT ONLY. An existing memory file is the playing AI's own
# working memory and is never overwritten, even if it is empty. Safe to
# re-run at any time.

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$templateDir = Join-Path $repoRoot 'fortress/memory-templates'
$memoryDir = Join-Path $repoRoot 'fortress/memory'

if (-not (Test-Path $templateDir)) {
    throw "Template directory not found: $templateDir"
}

if (-not (Test-Path $memoryDir)) {
    New-Item -ItemType Directory -Path $memoryDir | Out-Null
    Write-Host "Created $memoryDir"
}

$seeded = 0
$kept = 0

foreach ($template in Get-ChildItem -Path $templateDir -Filter '*.md') {
    $target = Join-Path $memoryDir $template.Name
    if (Test-Path $target) {
        Write-Host "kept    $($template.Name) (already exists - not touched)"
        $kept++
    } else {
        Copy-Item -Path $template.FullName -Destination $target
        Write-Host "seeded  $($template.Name)"
        $seeded++
    }
}

Write-Host ""
Write-Host "Done. $seeded seeded, $kept left alone."
```

- [ ] **Step 5: Test that it does NOT overwrite existing memory**

Run:
```powershell
$before = Get-FileHash fortress/memory/journal.md
powershell -ExecutionPolicy Bypass -File scripts/seed-fortress-memory.ps1
$after = Get-FileHash fortress/memory/journal.md
if ($before.Hash -eq $after.Hash) { Write-Host "PASS: journal untouched" } else { Write-Host "FAIL: journal was modified" }
```
Expected: `kept` for all three files, then `PASS: journal untouched`.

- [ ] **Step 6: Test that it DOES seed an empty directory**

Run:
```powershell
$tmp = Join-Path $env:TEMP "dfai-seed-test"
Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force "$tmp/scripts" | Out-Null
New-Item -ItemType Directory -Force "$tmp/fortress" | Out-Null
Copy-Item -Recurse fortress/memory-templates "$tmp/fortress/memory-templates"
Copy-Item scripts/seed-fortress-memory.ps1 "$tmp/scripts/"
powershell -ExecutionPolicy Bypass -File "$tmp/scripts/seed-fortress-memory.ps1"
Get-ChildItem "$tmp/fortress/memory"
```
Expected: three `seeded` lines, then a listing showing `goals.md`, `journal.md`, `learnings.md`.

- [ ] **Step 7: Commit**

```bash
git add fortress/memory-templates/ scripts/seed-fortress-memory.ps1
git commit -m "feat: seed templates for a fresh clone's fortress memory

Copy-if-absent only — an existing memory file is the playing AI's own
working memory and is never overwritten."
```

---

### Task A3: Release packaging script

**Files:**
- Create: `scripts/package-release.ps1`

**Interfaces:**
- Consumes: `scripts/seed-fortress-memory.ps1` (Task A2).
- Produces: `dist/df-ai-<version>-dfhack<tag>.zip`. Task A5 uploads it. The ZIP's `fortress/.mcp.json` is the exe variant that Task A4's README quick-start tells viewers to expect.

**What goes in the ZIP:** `bin/df-mcp.exe`, `plugin/df_ai_protocol.plug.dll`, `fortress/.mcp.json` (exe variant), `fortress/memory/*.md` (seeded), and `INSTALL.txt`.

- [ ] **Step 1: Write the packaging script**

Create `scripts/package-release.ps1`:

```powershell
# Packages a DF-AI release ZIP: prebuilt MCP server + prebuilt DFHack
# plugin + a zero-edit .mcp.json + seeded memory files.
#
# Run from the repo root. Requires the sibling DFHack checkout to have
# been built already (see docs/guides/plugin-build.md).

param(
    [string]$Version = '0.1.0',
    [string]$DfhackCheckout = '../dfhack-build'
)

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

# --- Derive the DFHack tag rather than trusting a constant -----------------
# Test $LASTEXITCODE, not $? — $? reflects the last operation in the
# statement (the .Trim() call), not the native git invocation, so a
# non-zero git exit with partial stdout would sail straight through.
$tagRaw = & git -C $DfhackCheckout describe --tags
if ($LASTEXITCODE -ne 0) { throw "Could not read DFHack tag from $DfhackCheckout (git exit $LASTEXITCODE)" }
$tag = $tagRaw.Trim()
if ([string]::IsNullOrWhiteSpace($tag)) { throw "DFHack tag came back empty from $DfhackCheckout" }
Write-Host "DFHack tag: $tag"

$stage = Join-Path $repoRoot "dist/stage"
Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force "$stage/bin" | Out-Null
New-Item -ItemType Directory -Force "$stage/plugin" | Out-Null
New-Item -ItemType Directory -Force "$stage/fortress/memory" | Out-Null

# --- Build the MCP server -------------------------------------------------
Write-Host "Building df-mcp.exe..."
# Do NOT redirect this native command's stderr — in PS 5.1 that wraps each
# line in a NativeCommandError and sets $? false even on a clean exit.
# Let it write to the console and test $LASTEXITCODE instead.
& go build -o "$stage/bin/df-mcp.exe" ./cmd/df-mcp
if ($LASTEXITCODE -ne 0) { throw "go build failed (exit $LASTEXITCODE)" }
if (-not (Test-Path "$stage/bin/df-mcp.exe")) { throw "df-mcp.exe was not produced" }

# --- Gather the prebuilt plugin ------------------------------------------
$dll = Join-Path $DfhackCheckout 'build/plugins/df_ai_protocol/Release/df_ai_protocol.plug.dll'
if (-not (Test-Path $dll)) {
    throw "Plugin not found at $dll - build it first: cmake --build build --target df_ai_protocol --config Release"
}
$dllAge = (Get-Date) - (Get-Item $dll).LastWriteTime
Write-Host ("Plugin artifact age: {0:N1} hours" -f $dllAge.TotalHours)
if ($dllAge.TotalDays -gt 7) {
    Write-Warning "Plugin DLL is more than 7 days old - is it stale? Rebuild to be sure."
}
Copy-Item $dll "$stage/plugin/df_ai_protocol.plug.dll"

# --- Zero-edit .mcp.json (exe variant) ------------------------------------
# df-mcp resolves config/orchestrator.yaml relative to its working
# directory, and Claude Code launches MCP servers with fortress/ as cwd.
# The `cd ..` wrapper is the form verified in fortress/SESSION-SETUP.md.
$mcpJson = @'
{
  "mcpServers": {
    "df-fortress": {
      "type": "stdio",
      "command": "cmd",
      "args": ["/c", "cd .. && bin\\df-mcp.exe"],
      "env": {}
    }
  }
}
'@
# UTF-8 *without* BOM. Set-Content -Encoding utf8 emits a BOM on PS 5.1,
# and Node's JSON.parse throws on a leading BOM — a BOM here would break
# the MCP config for every viewer following the quick-start.
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText("$stage/fortress/.mcp.json", $mcpJson, $utf8NoBom)

# --- Seed memory files ----------------------------------------------------
Copy-Item "$repoRoot/fortress/memory-templates/*.md" "$stage/fortress/memory/"

# --- Install instructions -------------------------------------------------
$install = @"
DF-AI $Version  (built against DFHack $tag)

This release only works with DFHack $tag. The DFHack plugin loader
enforces an exact version-string match - a different DFHack version will
silently refuse to load the plugin. If your DFHack has moved on, build
the plugin yourself: docs/guides/plugin-build.md

WHAT TO DO WITH THESE FILES
---------------------------
1. Download the DF-AI source (green "Code" button > Download ZIP, or
   git clone) and unzip it somewhere sensible.

2. Copy this release's folders INTO that source folder, merging:
     bin\df-mcp.exe          ->  <source>\bin\df-mcp.exe
     fortress\.mcp.json      ->  <source>\fortress\.mcp.json   (overwrite)
     fortress\memory\*.md    ->  <source>\fortress\memory\     (do NOT
                                 overwrite if you already have a fort)

3. Copy the plugin into DFHack, with Dwarf Fortress CLOSED:
     plugin\df_ai_protocol.plug.dll  ->  <DFHack>\hack\plugins\

   Find <DFHack> via Steam: it is its own app and does NOT necessarily
   live inside the Dwarf Fortress folder. The right hack\plugins\ folder
   already contains dozens of .plug.dll files.

4. Launch DF through DFHack, load or embark a fort, then open Claude Code
   in the source folder's fortress\ directory.

5. In the DFHack console: ai-connect
   In Claude Code, run the `status` tool. It should report a live
   connection.

Order matters in steps 4-5: Claude Code must be open before ai-connect,
because Claude Code is what starts the server that ai-connect dials.
"@
[System.IO.File]::WriteAllText("$stage/INSTALL.txt", $install, $utf8NoBom)

# --- Zip ------------------------------------------------------------------
$zipName = "df-ai-$Version-dfhack$tag.zip"
$zipPath = Join-Path $repoRoot "dist/$zipName"
Remove-Item -Force $zipPath -ErrorAction SilentlyContinue
Compress-Archive -Path "$stage/*" -DestinationPath $zipPath

Write-Host ""
Write-Host "Packaged: $zipPath"
Get-ChildItem $zipPath | Select-Object Name, Length
```

- [ ] **Step 2: Run it**

Run:
```powershell
powershell -ExecutionPolicy Bypass -File scripts/package-release.ps1 -Version 0.1.0
```
Expected: prints the DFHack tag, builds the exe, copies the DLL, and prints `Packaged: ...\dist\df-ai-0.1.0-dfhack53.16-r1.1.zip`.

If it throws `Plugin not found`, build the plugin first from the DFHack checkout:
```bash
cmake --build build --target df_ai_protocol --config Release
```

- [ ] **Step 3: Verify the ZIP contents**

Run:
```powershell
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [System.IO.Compression.ZipFile]::OpenRead((Resolve-Path dist/df-ai-0.1.0-dfhack53.16-r1.1.zip))
$zip.Entries | Select-Object FullName, Length
$zip.Dispose()
```
Expected entries: `INSTALL.txt`, `bin/df-mcp.exe` (non-zero), `plugin/df_ai_protocol.plug.dll` (non-zero), `fortress/.mcp.json`, `fortress/memory/goals.md`, `fortress/memory/journal.md`, `fortress/memory/learnings.md`.

- [ ] **Step 4: Verify `dist/` is gitignored**

Run:
```bash
git status --short dist/
```
Expected: **no output** (`.gitignore` already contains `/dist/`).

- [ ] **Step 5: Commit**

```bash
git add scripts/package-release.ps1
git commit -m "feat: release packaging script

Produces a ZIP with prebuilt df-mcp.exe, the prebuilt DFHack plugin, a
zero-edit exe-variant .mcp.json, and seeded memory files. Derives the
DFHack tag from the checkout rather than hardcoding it."
```

---

### Task A4: README quick-start and fresh-clone verification

**Files:**
- Modify: `README.md` (the `## Setup` section, lines 24–39)

**Interfaces:**
- Consumes: Tasks A1–A3.
- Produces: the README text the video's chapter 5 follows on screen. Chapter 5's narration must match this wording.

- [ ] **Step 1: Replace the Setup section**

In `README.md`, replace the whole `## Setup` section (currently lines 24–39) with:

```markdown
## Setup

Two paths. Most people want the first one.

### Quick start (prebuilt — no compiler, no Go, no Git required)

1. **Get the source.** Green **Code** button above → **Download ZIP**, and unzip it. (Or `git clone` if you have Git — you'll get updates more easily, and the tooling changes often.)
2. **Get the release.** From this repo's [Releases](https://github.com/zafety-vibin/DF-AI/releases), download the ZIP matching your DFHack version and merge its folders into the source folder. `INSTALL.txt` inside spells out each file.
3. **Deploy the plugin.** With DF closed, copy `df_ai_protocol.plug.dll` into DFHack's `hack/plugins/` — the folder holding its stock plugins. Steam installs DFHack as its own app, so this may not live under the DF folder; `docs/guides/plugin-build.md` shows how to derive the current path.
4. **Launch DF** through DFHack and load or embark a fort.
5. **Open Claude Code** in the `fortress/` directory. Its `.mcp.json` starts the server automatically.
6. **Connect.** In the DFHack console: `ai-connect`. Then call the `status` tool in Claude Code and confirm it reports a live connection.

Order matters in steps 5–6: Claude Code must be running before `ai-connect`, because Claude Code is what starts the server `ai-connect` dials.

A release is pinned to one exact DFHack version — the plugin loader enforces an exact version-string match. If your DFHack has moved past the latest release, build the plugin yourself with the developer path below.

### Developer path (build from source)

1. **Build the plugin.** Clone a DFHack source checkout as a sibling of this repo (conventionally `../dfhack-build`) at the tag matching your installed DFHack, junction `plugins/df_ai_protocol` to this repo's `dfhack-plugin/`, and build:
   ```bash
   cmake --build build --target df_ai_protocol --config Release
   ```
   Full detail, including the exact-version-match gotcha and the upgrade procedure: `docs/guides/plugin-build.md`.
2. **Deploy it.** As step 3 above.
3. **Build the MCP server.**
   ```bash
   go build ./... && go test ./...
   ```
4. **Seed your fort's memory files** (first run only; never overwrites):
   ```powershell
   powershell -ExecutionPolicy Bypass -File scripts/seed-fortress-memory.ps1
   ```
5. **Point an MCP client at it.** `fortress/.mcp.json` wires a `df-fortress` stdio server via `go run`. Opening `fortress/` in Claude Code picks it up automatically.
6. **Launch, connect, verify** as steps 4–6 above.

Playing a session end-to-end (recommended kickoff prompt, working-directory gotchas): `fortress/SESSION-SETUP.md`.
```

- [ ] **Step 2: Update the Requirements section**

In `README.md`, replace the `## Requirements` bullet list (lines 18–22) with:

```markdown
- **Steam Dwarf Fortress** (paid, [store page](https://store.steampowered.com/app/975370/Dwarf_Fortress/)) — the game DF-AI plays.
- **[DFHack](https://github.com/DFHack/dfhack)**, matching the exact release your Steam DF version needs. It's a separate free Steam app.
- **[Claude Code](https://claude.com/claude-code)** (or another MCP client that can run a long-lived stdio server) and a Claude plan — this is what actually plays.

The quick-start path needs nothing else. The developer path additionally needs **Go 1.25+**, **MSVC 2022 (v143)** (the plugin's only supported toolchain), and a **DFHack source checkout** at your DFHack's tag — DFHack ships no SDK, so plugins build in-tree; see `docs/guides/plugin-build.md`.
```

- [ ] **Step 3: Fresh-clone test — confirm no Fort #6 memory leaks to a new user**

Run:
```powershell
$tmp = Join-Path $env:TEMP "dfai-freshclone"
Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
git clone -q --no-hardlinks . $tmp
Get-ChildItem "$tmp/fortress" -Recurse | Select-Object FullName
```
Expected: `fortress/.mcp.json`, `fortress/CLAUDE.md`, `fortress/SESSION-SETUP.md`, and `fortress/memory-templates/` with three files. **There must be no `fortress/memory/*.md` and no `fortress/state/`.**

- [ ] **Step 4: Fresh-clone test — confirm seeding works from clean**

Run:
```powershell
powershell -ExecutionPolicy Bypass -File "$tmp/scripts/seed-fortress-memory.ps1"
Get-Content "$tmp/fortress/memory/goals.md" -TotalCount 3
```
Expected: three `seeded` lines, then the goals template header — **not** anything mentioning Lanehold or Fort #6.

- [ ] **Step 5: Clean up and commit**

```powershell
Remove-Item -Recurse -Force $tmp
```
```bash
git add README.md
git commit -m "docs: add prebuilt quick-start path to README

Quick start needs only Steam DF, DFHack, and Claude Code — no compiler,
no Go, no Git. The build-from-source path stays as the developer path
and the escape hatch for DFHack versions past the latest release."
```

---

### Task A5: Cut the GitHub release

**Files:** none — this is a release action.

**Interfaces:**
- Consumes: Tasks A1–A4. Requires `gh` CLI authenticated and DF closed.
- Produces: a published release whose ZIP the video's chapter 5 downloads on camera.

- [ ] **Step 1: Confirm DF is closed**

Run:
```powershell
Get-Process -Name "Dwarf Fortress" -ErrorAction SilentlyContinue
```
Expected: no output. If DF is running, close it — the plugin DLL is file-locked while loaded.

- [ ] **Step 2: Rebuild the plugin fresh**

From the DFHack checkout:
```bash
cmake --build build --target df_ai_protocol --config Release > build.log 2>&1
echo "exit: $?"
```
Expected: `exit: 0`. Do not pipe through `tail` — the pipeline exit code masks failures.

- [ ] **Step 3: Confirm the full Go suite is green**

From the repo root:
```bash
go build ./... && go test ./...
```
Expected: `ok` for every package, no `FAIL`.

- [ ] **Step 4: Package**

```powershell
powershell -ExecutionPolicy Bypass -File scripts/package-release.ps1 -Version 0.1.0
```
Expected: `Packaged: ...\dist\df-ai-0.1.0-dfhack53.16-r1.1.zip`, with no stale-artifact warning.

- [ ] **Step 5: Create the release**

```bash
gh release create v0.1.0 dist/df-ai-0.1.0-dfhack53.16-r1.1.zip \
  --title "v0.1.0 — first prebuilt release (DFHack 53.16-r1.1)" \
  --notes "First prebuilt release: no compiler, no Go, no Git needed to run DF-AI.

**Pinned to DFHack 53.16-r1.1.** The plugin loader enforces an exact version-string match — a different DFHack version will refuse to load this plugin. If yours has moved on, build it yourself: \`docs/guides/plugin-build.md\`.

Contains a prebuilt \`df-mcp.exe\`, the prebuilt \`df_ai_protocol.plug.dll\`, a zero-edit \`fortress/.mcp.json\`, and starter memory files. See \`INSTALL.txt\` inside.

This is early, exploratory research software. Expect rough edges."
```

- [ ] **Step 6: Verify the release downloads and unzips**

```powershell
$t = Join-Path $env:TEMP "dfai-release-check"
Remove-Item -Recurse -Force $t -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $t | Out-Null
gh release download v0.1.0 --dir $t
Expand-Archive (Get-ChildItem "$t/*.zip").FullName -DestinationPath "$t/x"
Get-ChildItem "$t/x" -Recurse | Select-Object FullName, Length
```
Expected: the same seven entries verified in Task A3 Step 3, all non-zero where they should be.

---

# PHASE B — Narration script and cue manifest

### Task B1: Write the narration script and overlay manifest

**Files:**
- Create: `docs/video/2026-08-10-tutorial-script.md`
- Create: `docs/video/capture-shot-list.md`
- Create: `C:\Users\zmanl\Projects\Remotion Videos\df-ai-tutorial\src\overlays.json`

**Interfaces:**
- Consumes: the chapter map in the design doc; the exact README wording from Task A4; the exact `INSTALL.txt` wording from Task A3.
- Produces: `overlays.json` conforming to the `OverlaySpec` type defined in Task C2 — every entry `{id, type, text, subtext?, durationInSeconds}` with `type` one of `command | verify | warning | lowerThird`. Task C2's Root registers one composition per entry, so **every cue ID in the script must exist in the manifest and vice versa.**

- [ ] **Step 1: Write the script skeleton with all eleven chapters**

Create `docs/video/2026-08-10-tutorial-script.md`. Use this exact format — narration is what gets read aloud, everything bracketed is production direction:

```markdown
# DF-AI setup tutorial — narration script

Target ~18:45. Read the narration lines aloud; bracketed lines are
production direction and are never spoken.

Legend:
- `[SHOT: ...]` — what's on V1 (screen capture or b-roll)
- `[OVERLAY: <id>]` — an alpha overlay from src/overlays.json on V2
- `[SEGMENT: <name>]` — a full-frame Remotion segment cut into V1

---

## COLD OPEN — 0:00–0:35

[SHOT: gameplay b-roll — fort being dug, dwarves working]

That fort is being dug by Claude.

[SHOT: Claude Code session capture — tool calls scrolling]

Not by a script, and not by a macro. It's looking at the map, deciding
where to dig, and then reading what actually happened — the same loop
you'd run yourself.

[SHOT: back to gameplay, wider]

People asked how to run this themselves. So: everything, from a computer
with nothing installed, to your own fort getting dug. About twenty
minutes, and you can skip the last two chapters.

[SEGMENT: cold-open-lockup]
```

- [ ] **Step 2: Write chapters 1 through 4**

Continue the same file. Chapter 1 must state the honest gate **accurately** — a supervised session is tool-heavy and long-context and wants a Claude subscription plan, and fits comfortably inside a Max 5-hour window. Do not claim it consumes a whole window.

```markdown
## CH 1 — Before you start — 0:35–1:25

[SEGMENT: chapter-card-1]
[SHOT: b-roll under narration]

Four honest things before you spend twenty minutes.

One: this is Windows only. The plugin builds with Microsoft's compiler,
and nobody's adapted it to Mac or Linux.

Two: Dwarf Fortress costs money — it's a paid game on Steam. DFHack, the
mod framework we need, is free.

Three: you need Claude Code and a Claude plan. A play session is
tool-heavy and runs long, so it wants a subscription rather than
pay-as-you-go — a session sits comfortably inside a five-hour window.

[OVERLAY: warn-research-software]

Four, and this is the real one: this is research software. I'm changing
it every week. Things will break in ways this video didn't predict, and
there's a troubleshooting chapter near the end for exactly that.

Still here? Good.

## CH 2 — What you're building — 1:25–2:25

[SEGMENT: architecture-explainer]

Before you install anything, here's the shape of it — because otherwise
half these steps look random.

Dwarf Fortress on the left. It has no idea anything else exists.

DFHack is a mod framework that loads inside the game and can read and
change its memory. We add one plugin to it. That plugin is the senses
and the hands — it reads the map and carries out orders.

The plugin talks over a network connection on your own machine to a
small server. That server is the nervous system: it turns raw game state
into things a language model can actually reason about, and turns
decisions back into game commands.

And Claude Code is the mind. It never touches the game directly. It asks
for what it wants to know, decides, and acts.

So: four pieces. The game, the plugin, the server, and Claude. Every
install step from here is one of those four.

## CH 3 — The game side — 2:25–4:45

[SEGMENT: chapter-card-3]
[SHOT: Steam store page for Dwarf Fortress]

Start with the game itself. Dwarf Fortress on Steam — buy it, install it,
and you can close Steam's store page.

[SHOT: Steam search for DFHack]

Now the part people get wrong. DFHack is not a mod you install into the
game folder. It's its own separate app on Steam. Search for it, install
it — it's free.

[OVERLAY: warn-dfhack-separate-app]

[SHOT: Steam library showing both, launching DFHack]

From now on you launch the game through DFHack, not through Dwarf
Fortress directly. If you launch DF on its own, none of this works.

[SHOT: DFHack launching DF, then the DFHack console]

Let it start once, so it creates its folders. That black window is the
DFHack console — you'll type one command into it later.

[SHOT: Explorer, right-click DFHack in Steam > Manage > Browse local files]

Last thing in this chapter: find the folder we'll install into. In Steam,
right-click DFHack, Manage, Browse local files. Then into `hack`, then
`plugins`.

[OVERLAY: verify-hack-plugins]

You should see dozens of files ending in `.plug.dll`. If you only see a
handful, you're in the wrong folder — and that's the single most common
reason this whole thing silently fails. Leave this window open.

## CH 4 — Claude Code — 4:45–5:45

[SEGMENT: chapter-card-4]
[SHOT: claude.com/claude-code download page]

Claude Code is the part that actually plays. Install it from here and
sign in with your Claude account.

[SHOT: terminal]

[OVERLAY: cmd-claude-version]

To check it worked, open a terminal and type this.

[OVERLAY: verify-claude-version]

If you get a version number back, you're done with this chapter. If it
says the command isn't recognised, close the terminal and open a fresh
one — the installer changes your PATH and only new terminals pick it up.
```

- [ ] **Step 3: Write chapters 5 through 8**

Chapter 5's on-screen steps must match the README quick-start wording from Task A4 exactly. Chapter 6 must state the ordering constraint (Claude Code before `ai-connect`) explicitly, and must **not** teach Dwarf Fortress — one line linking a beginner embark video, then move on. Chapter 7 is the payoff and runs on the existing b-roll: kickoff prompt → `survey_site` → `find_dig_site` → `designate_dig` → `step(1200)` → read the delta, with `[OVERLAY: turn-loop-ring]` running under it. Chapter 8 covers exactly the four failures in the design doc, over `[SEGMENT: troubleshooting-table]`.

- [ ] **Step 4: Write the two appendices and the outro**

Appendix A (build from source) is paced fast and opens by telling people who don't need it to skip. Appendix B states the true facts and nothing more: a plain stdio MCP server, **86 tools**, no sampling/elicitation/resources/prompts; what does not port (`fortress/CLAUDE.md` charter, `.claude/skills/`, memory-file discipline needing harness file tools); and that cheap-model 24/7 operation is **untested**, with the known hard part being long-horizon spatial reasoning and the truthful-ACK discipline rather than tool-calling.

- [ ] **Step 5: Read the script aloud and time it**

Read the whole thing at normal pace with a stopwatch. Expected: 17–20 minutes. If a chapter runs more than 30 seconds over its slot in the design doc's chapter map, cut narration rather than speeding up delivery — a non-developer following along needs the slower pace.

Record the actual per-chapter timings as a table at the top of the script file.

- [ ] **Step 6: Extract every overlay cue into the manifest**

Every `[OVERLAY: <id>]` in the script becomes one manifest entry. Create `C:\Users\zmanl\Projects\Remotion Videos\df-ai-tutorial\src\overlays.json`:

```json
{
  "overlays": [
    {
      "id": "warn-research-software",
      "type": "warning",
      "text": "This is early research software",
      "subtext": "It changes weekly. Expect rough edges.",
      "durationInSeconds": 6
    },
    {
      "id": "warn-dfhack-separate-app",
      "type": "warning",
      "text": "DFHack is a SEPARATE Steam app",
      "subtext": "Not a mod you copy into the DF folder.",
      "durationInSeconds": 6
    },
    {
      "id": "verify-hack-plugins",
      "type": "verify",
      "text": "dozens of .plug.dll files",
      "subtext": "If you only see a few, you're in the wrong folder.",
      "durationInSeconds": 7
    },
    {
      "id": "cmd-claude-version",
      "type": "command",
      "text": "claude --version",
      "subtext": "type this in a terminal",
      "durationInSeconds": 7
    },
    {
      "id": "verify-claude-version",
      "type": "verify",
      "text": "2.x.x (Claude Code)",
      "subtext": "a version number means it worked",
      "durationInSeconds": 6
    },
    {
      "id": "cmd-ai-connect",
      "type": "command",
      "text": "ai-connect",
      "subtext": "type this in the DFHack console",
      "durationInSeconds": 7
    },
    {
      "id": "warn-order-matters",
      "type": "warning",
      "text": "Open Claude Code BEFORE ai-connect",
      "subtext": "Claude Code starts the server that ai-connect dials.",
      "durationInSeconds": 7
    },
    {
      "id": "warn-close-df",
      "type": "warning",
      "text": "Close Dwarf Fortress before copying the plugin",
      "subtext": "The file is locked while the game is running.",
      "durationInSeconds": 6
    },
    {
      "id": "warn-version-match",
      "type": "warning",
      "text": "The release is pinned to one DFHack version",
      "subtext": "A mismatch means the plugin silently never loads.",
      "durationInSeconds": 7
    },
    {
      "id": "verify-status-connected",
      "type": "verify",
      "text": "status: CONNECTED",
      "subtext": "if it says NOT CONNECTED, see chapter 8",
      "durationInSeconds": 6
    }
  ]
}
```

Add a `lowerThird` entry per chapter as you finalise chapter titles.

- [ ] **Step 7: Cross-check script cues against the manifest**

Run from the repo root:
```powershell
$script = Get-Content docs/video/2026-08-10-tutorial-script.md -Raw
$cues = [regex]::Matches($script, '\[OVERLAY: ([a-z0-9-]+)\]') | ForEach-Object { $_.Groups[1].Value } | Sort-Object -Unique
$manifest = (Get-Content "C:\Users\zmanl\Projects\Remotion Videos\df-ai-tutorial\src\overlays.json" -Raw | ConvertFrom-Json).overlays.id | Sort-Object -Unique
Write-Host "In script but not manifest:"; Compare-Object $cues $manifest | Where-Object SideIndicator -eq '<=' | ForEach-Object { $_.InputObject }
Write-Host "In manifest but not script:"; Compare-Object $cues $manifest | Where-Object SideIndicator -eq '=>' | ForEach-Object { $_.InputObject }
```
Expected: both lists empty.

- [ ] **Step 8: Write the capture shot list**

Create `docs/video/capture-shot-list.md` from the design doc's shot list, ordered for a single recording session rather than by chapter — all Steam work together, all terminal work together, and the four deliberate failure reproductions last (they leave the machine dirty). Mark each shot with the chapter it feeds.

- [ ] **Step 9: Commit**

```bash
git add docs/video/
git commit -m "docs: tutorial narration script and capture shot list"
```

---

# PHASE C — Remotion overlay project

### Task C1: Scaffold the project and theme

**Files:**
- Create: `C:\Users\zmanl\Projects\Remotion Videos\df-ai-tutorial\{package.json,tsconfig.json,remotion.config.ts,.gitignore}`
- Create: `src/index.ts`, `src/Root.tsx`, `src/theme.ts`, `src/hooks/useFonts.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `colors`, `fonts`, `panelStyle`, `VIDEO`, `seconds(s: number): number` from `src/theme.ts`; `bootFonts(): void` from `src/hooks/useFonts.ts`. Every later Phase C task imports these.

- [ ] **Step 1: Create the project directory and package.json**

Create `C:\Users\zmanl\Projects\Remotion Videos\df-ai-tutorial\package.json`:

```json
{
  "name": "df-ai-tutorial",
  "version": "0.1.0",
  "private": true,
  "description": "Overlay and segment assets for the DF-AI setup tutorial video. Alpha overlays composite atop screen capture in Premiere; segments cut in as full frames.",
  "scripts": {
    "studio": "remotion studio",
    "tsc": "tsc --noEmit",
    "render:overlays": "node scripts/render-overlays.mjs",
    "render:segment": "remotion render",
    "render:coldopen": "remotion render ColdOpenLockup out/dfai/cold-open.mp4 --codec=h264 --pixel-format=yuv420p",
    "render:architecture": "remotion render ArchitectureExplainer out/dfai/architecture.mp4 --codec=h264 --pixel-format=yuv420p",
    "render:troubleshooting": "remotion render TroubleshootingTable out/dfai/troubleshooting.mp4 --codec=h264 --pixel-format=yuv420p",
    "render:outro": "remotion render OutroCard out/dfai/outro.mp4 --codec=h264 --pixel-format=yuv420p",
    "render:turnloop": "remotion render TurnLoopRing out/dfai/turn-loop-ring.mov"
  },
  "dependencies": {
    "@remotion/cli": "^4.0.0",
    "@remotion/google-fonts": "^4.0.0",
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "remotion": "^4.0.0"
  },
  "devDependencies": {
    "@types/react": "^18.3.0",
    "@types/react-dom": "^18.3.0",
    "typescript": "^5.6.0"
  }
}
```

Note: chapter cards and manifest overlays are rendered by the loop script, not by named npm scripts — there are too many to enumerate.

- [ ] **Step 2: Copy the tsconfig and write remotion.config.ts**

Create `tsconfig.json` (identical to `grimoire-demo-overlays/tsconfig.json`):

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "lib": ["DOM", "DOM.Iterable", "ES2020"],
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "skipLibCheck": true,
    "isolatedModules": true,
    "resolveJsonModule": true,
    "forceConsistentCasingInFileNames": true,
    "incremental": true
  },
  "include": ["src"]
}
```

Create `remotion.config.ts`:

```ts
import { Config } from '@remotion/cli/config'

// Default to transparent ProRes 4444 — the overlay case, which is most of
// this project. Full-frame segments override with --codec=h264 in their
// npm scripts.
Config.setVideoImageFormat('png')
Config.setPixelFormat('yuva444p10le')
Config.setCodec('prores')
Config.setProResProfile('4444')

Config.setEntryPoint('src/index.ts')
```

Create `.gitignore`:

```gitignore
node_modules/
out/
tsconfig.tsbuildinfo
```

- [ ] **Step 3: Write the theme**

Create `src/theme.ts`:

```ts
// DF-AI tutorial palette.
//
// Two colors carry the video's thesis: AMBER is everything dwarf-world
// (the game, the fort, the plugin), CYAN is everything machine-mind
// (Claude, the MCP server, tool calls). The architecture diagram then
// reads at a glance — amber left, cyan right, the bridge between them.

export const colors = {
  bg: '#0D0F12',
  bgElevated: '#151A20',

  amber: '#E8A33D',
  amberDim: '#A8752A',
  amberGlow: 'rgba(232, 163, 61, 0.28)',

  cyan: '#4DD0E1',
  cyanDim: '#2A8A99',
  cyanGlow: 'rgba(77, 208, 225, 0.28)',

  bone: '#EDE7DD',
  boneDim: '#8A8579',

  ok: '#7BC96F',
  danger: '#E06C5A',
  rule: 'rgba(237, 231, 221, 0.14)',
} as const

export const fonts = {
  // DF is a terminal game — command text and UI text share a family.
  mono: '"JetBrains Mono", monospace',
  sans: 'Inter, sans-serif',
} as const

// Shared legibility panel. Every alpha overlay sits on this so the
// underlying screen capture stays readable but doesn't fight the text.
export const panelStyle = {
  background: 'rgba(13, 15, 18, 0.88)',
  borderRadius: 12,
  backdropFilter: 'blur(8px)',
  boxShadow: '0 12px 40px rgba(0, 0, 0, 0.55)',
} as const

export const VIDEO = {
  width: 1920,
  height: 1080,
  fps: 30,
} as const

export const seconds = (s: number): number => Math.round(s * VIDEO.fps)
```

- [ ] **Step 4: Write the font hook**

Create `src/hooks/useFonts.ts`:

```ts
import { loadFont as loadJetBrainsMono } from '@remotion/google-fonts/JetBrainsMono'
import { loadFont as loadInter } from '@remotion/google-fonts/Inter'

let booted = false

export function bootFonts(): void {
  if (booted) return
  loadJetBrainsMono('normal', { weights: ['400', '500', '700'] })
  loadInter('normal', { weights: ['400', '500', '600', '700'] })
  booted = true
}
```

- [ ] **Step 5: Write the entry point and a placeholder Root**

Create `src/index.ts`:

```ts
import { registerRoot } from 'remotion'
import { RemotionRoot } from './Root'

registerRoot(RemotionRoot)
```

Create `src/Root.tsx` (Task C2 replaces the body; this proves the scaffold):

```tsx
import { AbsoluteFill, Composition } from 'remotion'
import { bootFonts } from './hooks/useFonts'
import { colors, fonts, VIDEO, seconds } from './theme'

bootFonts()

const Placeholder: React.FC = () => (
  <AbsoluteFill
    style={{
      background: colors.bg,
      color: colors.amber,
      fontFamily: fonts.mono,
      fontSize: 64,
      alignItems: 'center',
      justifyContent: 'center',
    }}
  >
    df-ai-tutorial
  </AbsoluteFill>
)

export const RemotionRoot: React.FC = () => (
  <Composition
    id="Placeholder"
    component={Placeholder}
    durationInFrames={seconds(2)}
    width={VIDEO.width}
    height={VIDEO.height}
    fps={VIDEO.fps}
  />
)
```

- [ ] **Step 6: Install and typecheck**

Run from `C:\Users\zmanl\Projects\Remotion Videos\df-ai-tutorial`:
```bash
npm install
npm run tsc
```
Expected: install completes; `tsc` produces **no output** (success).

- [ ] **Step 7: Prove it renders**

Run:
```bash
npx remotion render Placeholder out/dfai/placeholder.mov
```
Expected: a rendered file. Verify it exists and is non-zero:
```powershell
Get-Item out/dfai/placeholder.mov | Select-Object Name, Length
```

- [ ] **Step 8: Commit**

The Remotion project is outside the DF-AI repo and is not version-controlled by it. If `Remotion Videos/` is itself a git repo, commit there:
```bash
git add df-ai-tutorial/
git commit -m "feat(df-ai-tutorial): scaffold Remotion project with amber/cyan theme"
```
If it is not a repo, skip this step and note it — the assets are build inputs, not source of record.

---

### Task C2: Manifest-driven alpha overlays

**Files:**
- Create: `src/types.ts`
- Create: `src/components/CommandCallout.tsx`, `src/components/VerifyStrip.tsx`, `src/components/WarningCard.tsx`, `src/components/StepLowerThird.tsx`, `src/components/OverlayRouter.tsx`
- Create: `scripts/render-overlays.mjs`
- Modify: `src/Root.tsx` (replace the placeholder body)
- Consumes: `src/overlays.json` from Task B1

**Interfaces:**
- Consumes: `colors`, `fonts`, `panelStyle`, `VIDEO`, `seconds` (Task C1); `overlays.json` (Task B1).
- Produces: `OverlayType`, `OverlaySpec` from `src/types.ts`; `OverlayRouter: React.FC<OverlaySpec>`. Task C3's `TurnLoopRing` is registered alongside these in Root but is **not** manifest-driven.

- [ ] **Step 1: Write the types**

Create `src/types.ts`:

```ts
export type OverlayType = 'command' | 'verify' | 'warning' | 'lowerThird'

export type OverlaySpec = {
  /** Stable cue id. Must match an [OVERLAY: id] cue in the script. */
  id: string
  type: OverlayType
  /** The headline line — the command, the expected output, the warning. */
  text: string
  /** Optional supporting line beneath it. */
  subtext?: string
  /** How long the overlay stays on screen. */
  durationInSeconds: number
}
```

- [ ] **Step 2: Write CommandCallout**

Create `src/components/CommandCallout.tsx`. It shows the exact string to type, and must stay legible long enough for a viewer to pause and copy it.

```tsx
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors, fonts, panelStyle } from '../theme'

type CommandCalloutProps = {
  /** The literal command to type. Rendered in mono, selectable-looking. */
  text: string
  /** Short instruction beneath, e.g. "type this in a terminal". */
  subtext?: string
}

/**
 * Bottom-left mono chip carrying an exact command. Cyan rule on the left
 * because a command is the viewer talking to the machine side.
 */
export const CommandCallout: React.FC<CommandCalloutProps> = ({ text, subtext }) => {
  const frame = useCurrentFrame()
  const { fps, durationInFrames } = useVideoConfig()

  const slide = spring({ fps, frame, config: { damping: 22, mass: 1, stiffness: 110 } })
  const tx = interpolate(slide, [0, 1], [-80, 0])
  const fadeIn = interpolate(frame, [0, 12], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  const fadeOut = interpolate(frame, [durationInFrames - 14, durationInFrames], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <AbsoluteFill
      style={{
        justifyContent: 'flex-end',
        alignItems: 'flex-start',
        padding: '0 96px 110px',
        pointerEvents: 'none',
      }}
    >
      <div
        style={{
          ...panelStyle,
          display: 'flex',
          alignItems: 'stretch',
          overflow: 'hidden',
          opacity: fadeIn * fadeOut,
          transform: `translateX(${tx}px)`,
          maxWidth: 1100,
        }}
      >
        <div style={{ width: 6, background: colors.cyan }} />
        <div style={{ padding: '22px 40px 24px' }}>
          <div
            style={{
              fontFamily: fonts.mono,
              fontSize: 46,
              fontWeight: 700,
              color: colors.cyan,
              letterSpacing: '0.01em',
              whiteSpace: 'pre',
            }}
          >
            {text}
          </div>
          {subtext && (
            <div
              style={{
                fontFamily: fonts.sans,
                fontSize: 24,
                fontWeight: 500,
                color: colors.boneDim,
                marginTop: 12,
              }}
            >
              {subtext}
            </div>
          )}
        </div>
      </div>
    </AbsoluteFill>
  )
}
```

- [ ] **Step 3: Write VerifyStrip**

Create `src/components/VerifyStrip.tsx`. This is the highest-value overlay in the video — it tells a non-developer what success looks like.

```tsx
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors, fonts, panelStyle } from '../theme'

type VerifyStripProps = {
  /** What the viewer should be seeing on their own screen. */
  text: string
  /** Optional clarifier, e.g. "a version number means it worked". */
  subtext?: string
}

/**
 * Top-right "you should see this" confirmation strip with a check mark.
 * Green rule — this is the success state, distinct from command cyan and
 * warning amber.
 */
export const VerifyStrip: React.FC<VerifyStripProps> = ({ text, subtext }) => {
  const frame = useCurrentFrame()
  const { fps, durationInFrames } = useVideoConfig()

  const pop = spring({ fps, frame, config: { damping: 18, mass: 0.7, stiffness: 140 } })
  const scale = interpolate(pop, [0, 1], [0.9, 1])
  const fadeIn = interpolate(frame, [0, 10], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  const fadeOut = interpolate(frame, [durationInFrames - 14, durationInFrames], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <AbsoluteFill
      style={{
        justifyContent: 'flex-start',
        alignItems: 'flex-end',
        padding: '96px 96px 0',
        pointerEvents: 'none',
      }}
    >
      <div
        style={{
          ...panelStyle,
          display: 'flex',
          alignItems: 'center',
          gap: 24,
          padding: '20px 36px',
          opacity: fadeIn * fadeOut,
          transform: `scale(${scale})`,
          maxWidth: 900,
          border: `1px solid ${colors.rule}`,
        }}
      >
        <div
          style={{
            fontFamily: fonts.sans,
            fontSize: 40,
            fontWeight: 700,
            color: colors.ok,
            lineHeight: 1,
          }}
        >
          ✓
        </div>
        <div>
          <div
            style={{
              fontFamily: fonts.sans,
              fontSize: 18,
              fontWeight: 600,
              letterSpacing: '0.22em',
              textTransform: 'uppercase',
              color: colors.boneDim,
              marginBottom: 8,
            }}
          >
            You should see
          </div>
          <div
            style={{
              fontFamily: fonts.mono,
              fontSize: 34,
              fontWeight: 500,
              color: colors.bone,
              whiteSpace: 'pre',
            }}
          >
            {text}
          </div>
          {subtext && (
            <div
              style={{
                fontFamily: fonts.sans,
                fontSize: 22,
                color: colors.boneDim,
                marginTop: 10,
              }}
            >
              {subtext}
            </div>
          )}
        </div>
      </div>
    </AbsoluteFill>
  )
}
```

- [ ] **Step 4: Write WarningCard**

Create `src/components/WarningCard.tsx`:

```tsx
import { AbsoluteFill, interpolate, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors, fonts, panelStyle } from '../theme'

type WarningCardProps = {
  /** The gotcha, stated as an imperative or a fact. */
  text: string
  /** Why it matters / what happens if ignored. */
  subtext?: string
}

/**
 * Top-left amber alert for the gotchas that silently ruin a setup
 * (version match, ordering, file locks). Amber because these are all
 * failures on the dwarf-world side of the bridge.
 */
export const WarningCard: React.FC<WarningCardProps> = ({ text, subtext }) => {
  const frame = useCurrentFrame()
  const { durationInFrames } = useVideoConfig()

  const fadeIn = interpolate(frame, [0, 12], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  const fadeOut = interpolate(frame, [durationInFrames - 14, durationInFrames], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  // Slow single pulse on the rule so the eye catches it without it nagging.
  const pulse = interpolate(frame, [0, 20, 40], [0.5, 1, 0.85], {
    extrapolateRight: 'clamp',
  })

  return (
    <AbsoluteFill
      style={{
        justifyContent: 'flex-start',
        alignItems: 'flex-start',
        padding: '96px 0 0 96px',
        pointerEvents: 'none',
      }}
    >
      <div
        style={{
          ...panelStyle,
          display: 'flex',
          alignItems: 'stretch',
          overflow: 'hidden',
          opacity: fadeIn * fadeOut,
          maxWidth: 940,
        }}
      >
        <div style={{ width: 6, background: colors.amber, opacity: pulse }} />
        <div style={{ padding: '22px 40px 24px' }}>
          <div
            style={{
              fontFamily: fonts.sans,
              fontSize: 18,
              fontWeight: 700,
              letterSpacing: '0.24em',
              textTransform: 'uppercase',
              color: colors.amber,
              marginBottom: 10,
            }}
          >
            Watch out
          </div>
          <div
            style={{
              fontFamily: fonts.sans,
              fontSize: 38,
              fontWeight: 600,
              lineHeight: 1.2,
              color: colors.bone,
            }}
          >
            {text}
          </div>
          {subtext && (
            <div
              style={{
                fontFamily: fonts.sans,
                fontSize: 24,
                color: colors.boneDim,
                marginTop: 12,
                lineHeight: 1.35,
              }}
            >
              {subtext}
            </div>
          )}
        </div>
      </div>
    </AbsoluteFill>
  )
}
```

- [ ] **Step 5: Write StepLowerThird**

Create `src/components/StepLowerThird.tsx`:

```tsx
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors, fonts, panelStyle } from '../theme'

type StepLowerThirdProps = {
  /** Chapter or step title. */
  text: string
  /** Optional position marker, e.g. "Step 3 of 6". */
  subtext?: string
}

/** Bottom-left chapter/step label. Amber rule — orientation, not action. */
export const StepLowerThird: React.FC<StepLowerThirdProps> = ({ text, subtext }) => {
  const frame = useCurrentFrame()
  const { fps, durationInFrames } = useVideoConfig()

  const slide = spring({ fps, frame, config: { damping: 22, mass: 1, stiffness: 110 } })
  const tx = interpolate(slide, [0, 1], [-100, 0])
  const fadeIn = interpolate(frame, [0, 12], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  const fadeOut = interpolate(frame, [durationInFrames - 14, durationInFrames], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <AbsoluteFill
      style={{
        justifyContent: 'flex-end',
        alignItems: 'flex-start',
        padding: '0 96px 96px',
        pointerEvents: 'none',
      }}
    >
      <div
        style={{
          ...panelStyle,
          display: 'flex',
          alignItems: 'stretch',
          overflow: 'hidden',
          opacity: fadeIn * fadeOut,
          transform: `translateX(${tx}px)`,
        }}
      >
        <div style={{ width: 6, background: colors.amber }} />
        <div style={{ padding: '18px 36px 20px', minWidth: 340 }}>
          {subtext && (
            <div
              style={{
                fontFamily: fonts.mono,
                fontSize: 18,
                fontWeight: 500,
                letterSpacing: '0.2em',
                textTransform: 'uppercase',
                color: colors.amber,
                marginBottom: 8,
              }}
            >
              {subtext}
            </div>
          )}
          <div
            style={{
              fontFamily: fonts.sans,
              fontSize: 40,
              fontWeight: 700,
              lineHeight: 1.15,
              color: colors.bone,
            }}
          >
            {text}
          </div>
        </div>
      </div>
    </AbsoluteFill>
  )
}
```

- [ ] **Step 6: Write the router**

Create `src/components/OverlayRouter.tsx`:

```tsx
import type { OverlaySpec } from '../types'
import { CommandCallout } from './CommandCallout'
import { VerifyStrip } from './VerifyStrip'
import { WarningCard } from './WarningCard'
import { StepLowerThird } from './StepLowerThird'

/** Dispatches a manifest entry to its overlay component. */
export const OverlayRouter: React.FC<OverlaySpec> = ({ type, text, subtext }) => {
  switch (type) {
    case 'command':
      return <CommandCallout text={text} subtext={subtext} />
    case 'verify':
      return <VerifyStrip text={text} subtext={subtext} />
    case 'warning':
      return <WarningCard text={text} subtext={subtext} />
    case 'lowerThird':
      return <StepLowerThird text={text} subtext={subtext} />
  }
}
```

- [ ] **Step 7: Register every manifest entry in Root**

Replace the body of `src/Root.tsx`:

```tsx
import { Composition } from 'remotion'
import { bootFonts } from './hooks/useFonts'
import { OverlayRouter } from './components/OverlayRouter'
import type { OverlaySpec } from './types'
import { VIDEO, seconds } from './theme'
import manifest from './overlays.json'

bootFonts()

const overlays = manifest.overlays as OverlaySpec[]

export const RemotionRoot: React.FC = () => (
  <>
    {overlays.map((o) => (
      <Composition
        key={o.id}
        id={o.id}
        component={OverlayRouter}
        durationInFrames={seconds(o.durationInSeconds)}
        width={VIDEO.width}
        height={VIDEO.height}
        fps={VIDEO.fps}
        defaultProps={o}
      />
    ))}
  </>
)
```

- [ ] **Step 8: Write the render loop**

Create `scripts/render-overlays.mjs`:

```js
// Renders every overlay in src/overlays.json to out/dfai/<id>.mov as
// transparent ProRes 4444 (the defaults in remotion.config.ts).
//
// Re-run after editing the manifest; it overwrites in place, so a changed
// command means re-rendering one file, not hunting through TSX.

import { execFileSync } from 'node:child_process'
import { readFileSync, mkdirSync } from 'node:fs'

const manifest = JSON.parse(readFileSync(new URL('../src/overlays.json', import.meta.url)))
mkdirSync(new URL('../out/dfai/', import.meta.url), { recursive: true })

let failed = 0
for (const overlay of manifest.overlays) {
  const out = `out/dfai/${overlay.id}.mov`
  process.stdout.write(`rendering ${overlay.id} -> ${out}\n`)
  try {
    execFileSync('npx', ['remotion', 'render', overlay.id, out], {
      stdio: 'inherit',
      shell: true,
    })
  } catch {
    process.stderr.write(`FAILED: ${overlay.id}\n`)
    failed++
  }
}

if (failed > 0) {
  process.stderr.write(`\n${failed} overlay(s) failed to render.\n`)
  process.exit(1)
}
process.stdout.write(`\nRendered ${manifest.overlays.length} overlays.\n`)
```

- [ ] **Step 9: Typecheck**

Run:
```bash
npm run tsc
```
Expected: no output.

- [ ] **Step 10: Render one overlay and confirm it has an alpha channel**

Run:
```bash
npx remotion render cmd-claude-version out/dfai/cmd-claude-version.mov
```
Then confirm the pixel format really is alpha-bearing:
```bash
ffprobe -v error -select_streams v:0 -show_entries stream=pix_fmt,width,height -of default=noprint_wrappers=1 out/dfai/cmd-claude-version.mov
```
Expected:
```
width=1920
height=1080
pix_fmt=yuva444p10le
```
`yuva444p10le` — the `a` is the alpha channel. If it reads `yuv444p10le` with no `a`, the config didn't apply and the overlay will composite as a black box in Premiere.

- [ ] **Step 11: Render them all**

Run:
```bash
npm run render:overlays
```
Expected: one line per overlay, then `Rendered N overlays.` and exit 0.

- [ ] **Step 12: Commit (if `Remotion Videos/` is a git repo)**

```bash
git add df-ai-tutorial/
git commit -m "feat(df-ai-tutorial): manifest-driven alpha overlays"
```

---

### Task C3: Turn-loop ring overlay

**Files:**
- Create: `src/segments/TurnLoopRing.tsx`
- Modify: `src/Root.tsx` (register it)

**Interfaces:**
- Consumes: `colors`, `fonts`, `VIDEO`, `seconds` (Task C1).
- Produces: `TurnLoopRing: React.FC` and `TURN_LOOP_SECONDS: number`. Not manifest-driven — it runs the full length of chapter 7 under the b-roll.

- [ ] **Step 1: Write the component**

Create `src/segments/TurnLoopRing.tsx`:

```tsx
import { AbsoluteFill, interpolate, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors, fonts } from '../theme'

/** One full pass through the loop, in seconds. */
const CYCLE_SECONDS = 12
export const TURN_LOOP_SECONDS = CYCLE_SECONDS * 5

const PHASES = ['PAUSE', 'OBSERVE', 'ACT', 'STEP'] as const

/**
 * Corner diagram of the turn protocol, pulsing whichever phase is active.
 * Runs under the chapter 7 payoff b-roll so the viewer can map what
 * Claude is doing onto the loop being described.
 */
export const TurnLoopRing: React.FC = () => {
  const frame = useCurrentFrame()
  const { fps, durationInFrames } = useVideoConfig()

  const cycleFrames = CYCLE_SECONDS * fps
  const phaseFrames = cycleFrames / PHASES.length
  const activeIndex = Math.floor((frame % cycleFrames) / phaseFrames)

  const fadeIn = interpolate(frame, [0, 20], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  const fadeOut = interpolate(frame, [durationInFrames - 24, durationInFrames], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <AbsoluteFill
      style={{
        justifyContent: 'flex-start',
        alignItems: 'flex-end',
        padding: '80px 80px 0 0',
        pointerEvents: 'none',
      }}
    >
      <div
        style={{
          opacity: fadeIn * fadeOut,
          background: 'rgba(13, 15, 18, 0.86)',
          borderRadius: 14,
          border: `1px solid ${colors.rule}`,
          backdropFilter: 'blur(8px)',
          padding: '24px 30px',
          display: 'flex',
          flexDirection: 'column',
          gap: 14,
          minWidth: 260,
        }}
      >
        <div
          style={{
            fontFamily: fonts.sans,
            fontSize: 15,
            fontWeight: 700,
            letterSpacing: '0.26em',
            textTransform: 'uppercase',
            color: colors.boneDim,
          }}
        >
          The turn loop
        </div>

        {PHASES.map((phase, i) => {
          const active = i === activeIndex
          // Where we are inside the active phase, for the pulse.
          const withinPhase = (frame % cycleFrames) - i * phaseFrames
          const pulse = active
            ? interpolate(withinPhase, [0, 8, phaseFrames], [0.55, 1, 0.8], {
                extrapolateLeft: 'clamp',
                extrapolateRight: 'clamp',
              })
            : 1

          return (
            <div
              key={phase}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 14,
                opacity: active ? pulse : 0.34,
              }}
            >
              <div
                style={{
                  width: 12,
                  height: 12,
                  borderRadius: 6,
                  background: active ? colors.cyan : colors.boneDim,
                  boxShadow: active ? `0 0 16px ${colors.cyanGlow}` : 'none',
                }}
              />
              <div
                style={{
                  fontFamily: fonts.mono,
                  fontSize: 26,
                  fontWeight: active ? 700 : 400,
                  letterSpacing: '0.08em',
                  color: active ? colors.bone : colors.boneDim,
                }}
              >
                {phase}
              </div>
            </div>
          )
        })}
      </div>
    </AbsoluteFill>
  )
}
```

- [ ] **Step 2: Register it in Root**

In `src/Root.tsx`, add the import and the composition inside the fragment, after the overlays map:

```tsx
import { TurnLoopRing, TURN_LOOP_SECONDS } from './segments/TurnLoopRing'
```

```tsx
    <Composition
      id="TurnLoopRing"
      component={TurnLoopRing}
      durationInFrames={seconds(TURN_LOOP_SECONDS)}
      width={VIDEO.width}
      height={VIDEO.height}
      fps={VIDEO.fps}
    />
```

- [ ] **Step 3: Typecheck and render**

Run:
```bash
npm run tsc
npm run render:turnloop
```
Expected: no tsc output; a rendered `out/dfai/turn-loop-ring.mov`.

- [ ] **Step 4: Verify alpha and duration**

Run:
```bash
ffprobe -v error -select_streams v:0 -show_entries stream=pix_fmt,duration -of default=noprint_wrappers=1 out/dfai/turn-loop-ring.mov
```
Expected: `pix_fmt=yuva444p10le` and `duration=60.000000` (5 cycles × 12s).

- [ ] **Step 5: Commit (if applicable)**

```bash
git add df-ai-tutorial/
git commit -m "feat(df-ai-tutorial): turn-loop ring overlay for chapter 7"
```

---

### Task C4: Cold open, chapter cards, and outro

**Files:**
- Create: `src/segments/ColdOpenLockup.tsx`, `src/segments/ChapterCard.tsx`, `src/segments/OutroCard.tsx`
- Modify: `src/Root.tsx`, `scripts/render-overlays.mjs` (add a chapter-card loop)

**Interfaces:**
- Consumes: `colors`, `fonts`, `VIDEO`, `seconds` (Task C1).
- Produces: `ColdOpenLockup: React.FC`, `ChapterCard: React.FC<{number: number, title: string}>`, `OutroCard: React.FC`, and `CHAPTERS: readonly {number: number, title: string}[]` exported from `ChapterCard.tsx`.

- [ ] **Step 1: Write ChapterCard with the chapter list**

Create `src/segments/ChapterCard.tsx`:

```tsx
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors, fonts } from '../theme'

/** Chapter titles, matching the script's chapter map. */
export const CHAPTERS = [
  { number: 1, title: 'Before you start' },
  { number: 2, title: 'What you\u2019re building' },
  { number: 3, title: 'The game side' },
  { number: 4, title: 'Claude Code' },
  { number: 5, title: 'DF-AI itself' },
  { number: 6, title: 'Connect it' },
  { number: 7, title: 'Your first turn' },
  { number: 8, title: 'When it breaks' },
  { number: 9, title: 'Appendix: build it yourself' },
  { number: 10, title: 'Appendix: other models' },
] as const

type ChapterCardProps = {
  number: number
  title: string
}

/** Full-frame chapter divider. Opaque — cut into V1, not overlaid. */
export const ChapterCard: React.FC<ChapterCardProps> = ({ number, title }) => {
  const frame = useCurrentFrame()
  const { fps, durationInFrames } = useVideoConfig()

  const rise = spring({ fps, frame, config: { damping: 20, mass: 0.9, stiffness: 120 } })
  const ty = interpolate(rise, [0, 1], [28, 0])
  const ruleWidth = interpolate(rise, [0, 1], [0, 220])
  const fadeOut = interpolate(frame, [durationInFrames - 12, durationInFrames], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <AbsoluteFill
      style={{
        background: colors.bg,
        justifyContent: 'center',
        alignItems: 'flex-start',
        padding: '0 200px',
        opacity: fadeOut,
      }}
    >
      <div style={{ transform: `translateY(${ty}px)` }}>
        <div
          style={{
            fontFamily: fonts.mono,
            fontSize: 26,
            fontWeight: 500,
            letterSpacing: '0.32em',
            color: colors.amber,
            marginBottom: 26,
          }}
        >
          {String(number).padStart(2, '0')}
        </div>
        <div
          style={{
            fontFamily: fonts.sans,
            fontSize: 96,
            fontWeight: 700,
            lineHeight: 1.05,
            color: colors.bone,
          }}
        >
          {title}
        </div>
        <div
          style={{
            width: ruleWidth,
            height: 4,
            background: colors.cyan,
            marginTop: 34,
          }}
        />
      </div>
    </AbsoluteFill>
  )
}
```

- [ ] **Step 2: Write ColdOpenLockup**

Create `src/segments/ColdOpenLockup.tsx`:

```tsx
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors, fonts } from '../theme'

/**
 * Cold-open title lockup. The amber/cyan split states the thesis before a
 * word of narration: dwarf-world on one side, machine-mind on the other.
 */
export const ColdOpenLockup: React.FC = () => {
  const frame = useCurrentFrame()
  const { fps, durationInFrames } = useVideoConfig()

  const rise = spring({ fps, frame, config: { damping: 20, mass: 1, stiffness: 100 } })
  const ty = interpolate(rise, [0, 1], [40, 0])
  const subFade = interpolate(frame, [18, 38], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  const ruleWidth = interpolate(frame, [10, 40], [0, 560], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  const fadeOut = interpolate(frame, [durationInFrames - 16, durationInFrames], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <AbsoluteFill
      style={{
        background: colors.bg,
        justifyContent: 'center',
        alignItems: 'center',
        opacity: fadeOut,
      }}
    >
      <div style={{ textAlign: 'center', transform: `translateY(${ty}px)` }}>
        <div
          style={{
            fontFamily: fonts.mono,
            fontSize: 128,
            fontWeight: 700,
            letterSpacing: '0.04em',
            color: colors.bone,
          }}
        >
          <span style={{ color: colors.amber }}>DF</span>
          <span style={{ color: colors.boneDim }}>-</span>
          <span style={{ color: colors.cyan }}>AI</span>
        </div>
        <div
          style={{
            width: ruleWidth,
            height: 3,
            background: colors.rule,
            margin: '30px auto',
          }}
        />
        <div
          style={{
            fontFamily: fonts.sans,
            fontSize: 38,
            fontWeight: 500,
            color: colors.boneDim,
            opacity: subFade,
          }}
        >
          Set it up yourself — from nothing installed
        </div>
      </div>
    </AbsoluteFill>
  )
}
```

- [ ] **Step 3: Write OutroCard**

Create `src/segments/OutroCard.tsx`:

```tsx
import { AbsoluteFill, interpolate, useCurrentFrame } from 'remotion'
import { colors, fonts } from '../theme'

/** End card: where to get it, and what a useful report looks like. */
export const OutroCard: React.FC = () => {
  const frame = useCurrentFrame()

  const fade = interpolate(frame, [0, 20], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  const subFade = interpolate(frame, [24, 46], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <AbsoluteFill
      style={{
        background: colors.bg,
        justifyContent: 'center',
        alignItems: 'center',
        opacity: fade,
      }}
    >
      <div style={{ textAlign: 'center' }}>
        <div
          style={{
            fontFamily: fonts.mono,
            fontSize: 44,
            fontWeight: 500,
            color: colors.cyan,
            marginBottom: 34,
          }}
        >
          github.com/zafety-vibin/DF-AI
        </div>
        <div
          style={{
            fontFamily: fonts.sans,
            fontSize: 32,
            fontWeight: 500,
            lineHeight: 1.5,
            color: colors.boneDim,
            maxWidth: 1100,
            opacity: subFade,
          }}
        >
          If you build a fort with it, a write-up of what happened and why
          <br />
          is the most useful bug report you can send.
        </div>
      </div>
    </AbsoluteFill>
  )
}
```

- [ ] **Step 4: Register all three in Root**

In `src/Root.tsx`, add imports:

```tsx
import { ColdOpenLockup } from './segments/ColdOpenLockup'
import { ChapterCard, CHAPTERS } from './segments/ChapterCard'
import { OutroCard } from './segments/OutroCard'
```

And add inside the fragment:

```tsx
    <Composition
      id="ColdOpenLockup"
      component={ColdOpenLockup}
      durationInFrames={seconds(4)}
      width={VIDEO.width}
      height={VIDEO.height}
      fps={VIDEO.fps}
    />
    {CHAPTERS.map((c) => (
      <Composition
        key={c.number}
        id={`ChapterCard-${c.number}`}
        component={ChapterCard}
        durationInFrames={seconds(2.5)}
        width={VIDEO.width}
        height={VIDEO.height}
        fps={VIDEO.fps}
        defaultProps={{ number: c.number, title: c.title }}
      />
    ))}
    <Composition
      id="OutroCard"
      component={OutroCard}
      durationInFrames={seconds(6)}
      width={VIDEO.width}
      height={VIDEO.height}
      fps={VIDEO.fps}
    />
```

- [ ] **Step 5: Add a chapter-card render script**

Create `scripts/render-chapters.mjs`:

```js
// Renders the ten chapter cards as opaque H.264 — they are full-frame
// dividers cut into V1, not alpha overlays.

import { execFileSync } from 'node:child_process'
import { mkdirSync } from 'node:fs'

mkdirSync(new URL('../out/dfai/', import.meta.url), { recursive: true })

let failed = 0
for (let n = 1; n <= 10; n++) {
  const id = `ChapterCard-${n}`
  const out = `out/dfai/chapter-${String(n).padStart(2, '0')}.mp4`
  process.stdout.write(`rendering ${id} -> ${out}\n`)
  try {
    execFileSync(
      'npx',
      ['remotion', 'render', id, out, '--codec=h264', '--pixel-format=yuv420p'],
      { stdio: 'inherit', shell: true },
    )
  } catch {
    process.stderr.write(`FAILED: ${id}\n`)
    failed++
  }
}

if (failed > 0) process.exit(1)
process.stdout.write('\nRendered 10 chapter cards.\n')
```

Add to `package.json` scripts:
```json
"render:chapters": "node scripts/render-chapters.mjs",
```

- [ ] **Step 6: Typecheck and render**

Run:
```bash
npm run tsc
npm run render:coldopen
npm run render:chapters
npm run render:outro
```
Expected: no tsc output; `out/dfai/cold-open.mp4`, ten `chapter-NN.mp4`, and `outro.mp4`.

- [ ] **Step 7: Verify the segments are opaque, not alpha**

Run:
```bash
ffprobe -v error -select_streams v:0 -show_entries stream=pix_fmt -of default=noprint_wrappers=1 out/dfai/chapter-01.mp4
```
Expected: `pix_fmt=yuv420p` (no alpha — these are full-frame cuts and an alpha MP4 would be wrong).

- [ ] **Step 8: Commit (if applicable)**

```bash
git add df-ai-tutorial/
git commit -m "feat(df-ai-tutorial): cold open, chapter cards, outro"
```

---

### Task C5: Architecture explainer

**Files:**
- Create: `src/segments/ArchitectureExplainer.tsx`
- Modify: `src/Root.tsx`

**Interfaces:**
- Consumes: `colors`, `fonts`, `VIDEO`, `seconds` (Task C1).
- Produces: `ArchitectureExplainer: React.FC` and `ARCHITECTURE_SECONDS: number`.

This is the video's only fully-animated segment and its largest single build (~60s). It reveals four nodes left to right, amber for dwarf-world and cyan for machine-mind, with the link between plugin and server drawn as the bridge.

- [ ] **Step 1: Write the component**

Create `src/segments/ArchitectureExplainer.tsx`:

```tsx
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors, fonts } from '../theme'

export const ARCHITECTURE_SECONDS = 60

type Node = {
  label: string
  role: string
  side: 'dwarf' | 'machine'
  /** Seconds into the segment at which this node appears. */
  at: number
}

const NODES: Node[] = [
  { label: 'Dwarf Fortress', role: 'the world', side: 'dwarf', at: 4 },
  { label: 'DFHack plugin', role: 'senses & hands', side: 'dwarf', at: 14 },
  { label: 'MCP server', role: 'nervous system', side: 'machine', at: 28 },
  { label: 'Claude Code', role: 'the mind', side: 'machine', at: 42 },
]

const NodeBox: React.FC<{ node: Node; frame: number; fps: number }> = ({ node, frame, fps }) => {
  const local = frame - node.at * fps
  const accent = node.side === 'dwarf' ? colors.amber : colors.cyan
  const glow = node.side === 'dwarf' ? colors.amberGlow : colors.cyanGlow

  const pop = spring({ fps, frame: local, config: { damping: 19, mass: 0.8, stiffness: 130 } })
  const scale = interpolate(pop, [0, 1], [0.86, 1])
  const opacity = interpolate(local, [0, 14], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <div
      style={{
        flex: 1,
        opacity,
        transform: `scale(${scale})`,
        background: colors.bgElevated,
        border: `2px solid ${accent}`,
        borderRadius: 16,
        boxShadow: `0 0 44px ${glow}`,
        padding: '38px 26px',
        textAlign: 'center',
      }}
    >
      <div
        style={{
          fontFamily: fonts.sans,
          fontSize: 34,
          fontWeight: 700,
          color: colors.bone,
          lineHeight: 1.2,
          marginBottom: 16,
        }}
      >
        {node.label}
      </div>
      <div
        style={{
          fontFamily: fonts.mono,
          fontSize: 20,
          fontWeight: 500,
          letterSpacing: '0.16em',
          textTransform: 'uppercase',
          color: accent,
        }}
      >
        {node.role}
      </div>
    </div>
  )
}

const Connector: React.FC<{ at: number; frame: number; fps: number; label?: string }> = ({
  at,
  frame,
  fps,
  label,
}) => {
  const local = frame - at * fps
  const grow = interpolate(local, [0, 18], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <div style={{ width: 130, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12 }}>
      <div
        style={{
          width: `${grow * 100}%`,
          height: 3,
          background: `linear-gradient(90deg, ${colors.amber}, ${colors.cyan})`,
        }}
      />
      {label && (
        <div
          style={{
            fontFamily: fonts.mono,
            fontSize: 17,
            letterSpacing: '0.14em',
            color: colors.boneDim,
            opacity: grow,
          }}
        >
          {label}
        </div>
      )}
    </div>
  )
}

/**
 * The four-box chain, revealed left to right as the narration names each
 * piece. Amber = dwarf-world, cyan = machine-mind; the TCP connector is
 * the gradient between them.
 */
export const ArchitectureExplainer: React.FC = () => {
  const frame = useCurrentFrame()
  const { fps, durationInFrames } = useVideoConfig()

  const titleFade = interpolate(frame, [0, 20], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  const outFade = interpolate(frame, [durationInFrames - 20, durationInFrames], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <AbsoluteFill
      style={{
        background: colors.bg,
        flexDirection: 'column',
        justifyContent: 'center',
        alignItems: 'center',
        padding: '0 110px',
        opacity: outFade,
      }}
    >
      <div
        style={{
          fontFamily: fonts.sans,
          fontSize: 26,
          fontWeight: 600,
          letterSpacing: '0.3em',
          textTransform: 'uppercase',
          color: colors.boneDim,
          marginBottom: 64,
          opacity: titleFade,
        }}
      >
        Four pieces
      </div>

      <div style={{ display: 'flex', alignItems: 'center', width: '100%' }}>
        <NodeBox node={NODES[0]} frame={frame} fps={fps} />
        <Connector at={NODES[1].at - 2} frame={frame} fps={fps} label="loads into" />
        <NodeBox node={NODES[1]} frame={frame} fps={fps} />
        <Connector at={NODES[2].at - 2} frame={frame} fps={fps} label="TCP" />
        <NodeBox node={NODES[2]} frame={frame} fps={fps} />
        <Connector at={NODES[3].at - 2} frame={frame} fps={fps} label="MCP" />
        <NodeBox node={NODES[3]} frame={frame} fps={fps} />
      </div>

      <div
        style={{
          display: 'flex',
          width: '100%',
          justifyContent: 'space-between',
          marginTop: 70,
          opacity: interpolate(frame, [50 * fps, 54 * fps], [0, 1], {
            extrapolateLeft: 'clamp',
            extrapolateRight: 'clamp',
          }),
        }}
      >
        <div style={{ fontFamily: fonts.mono, fontSize: 22, color: colors.amber, letterSpacing: '0.14em' }}>
          ◂ THE GAME
        </div>
        <div style={{ fontFamily: fonts.mono, fontSize: 22, color: colors.cyan, letterSpacing: '0.14em' }}>
          THE MIND ▸
        </div>
      </div>
    </AbsoluteFill>
  )
}
```

- [ ] **Step 2: Register it in Root**

Add the import and composition to `src/Root.tsx`:

```tsx
import { ArchitectureExplainer, ARCHITECTURE_SECONDS } from './segments/ArchitectureExplainer'
```

```tsx
    <Composition
      id="ArchitectureExplainer"
      component={ArchitectureExplainer}
      durationInFrames={seconds(ARCHITECTURE_SECONDS)}
      width={VIDEO.width}
      height={VIDEO.height}
      fps={VIDEO.fps}
    />
```

- [ ] **Step 3: Typecheck**

Run:
```bash
npm run tsc
```
Expected: no output.

- [ ] **Step 4: Preview the reveal timing in studio before rendering 60s**

Run:
```bash
npm run studio
```
Open `ArchitectureExplainer` and scrub. Confirm each node appears at its narration beat: DF at 0:04, plugin at 0:14, server at 0:28, Claude at 0:42, and the closing side labels at 0:50. If the narration timing from Task B1 Step 5 differs, adjust the `at` values in `NODES` to match the recorded read — the animation serves the voice, not the other way round.

- [ ] **Step 5: Render**

Run:
```bash
npm run render:architecture
```
Expected: `out/dfai/architecture.mp4`.

Verify duration:
```bash
ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1 out/dfai/architecture.mp4
```
Expected: `duration=60.0` (±0.1).

- [ ] **Step 6: Commit (if applicable)**

```bash
git add df-ai-tutorial/
git commit -m "feat(df-ai-tutorial): architecture explainer segment"
```

---

### Task C6: Troubleshooting table

**Files:**
- Create: `src/segments/TroubleshootingTable.tsx`
- Modify: `src/Root.tsx`

**Interfaces:**
- Consumes: `colors`, `fonts`, `VIDEO`, `seconds` (Task C1).
- Produces: `TroubleshootingTable: React.FC`, `TROUBLESHOOTING_SECONDS: number`.

- [ ] **Step 1: Write the component**

Create `src/segments/TroubleshootingTable.tsx`:

```tsx
import { AbsoluteFill, interpolate, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors, fonts } from '../theme'

export const TROUBLESHOOTING_SECONDS = 80

type Row = {
  symptom: string
  cause: string
  fix: string
  /** Seconds into the segment at which this row appears. */
  at: number
}

const ROWS: Row[] = [
  {
    symptom: 'ai-connect: unknown command',
    cause: 'The plugin never loaded — wrong folder, or a DFHack version mismatch',
    fix: 'Check you copied into DFHack\u2019s own hack/plugins/, and that the release matches your DFHack version',
    at: 6,
  },
  {
    symptom: 'status says NOT CONNECTED',
    cause: 'ai-connect ran before Claude Code was open',
    fix: 'Open Claude Code in fortress/ first, then run ai-connect again',
    at: 22,
  },
  {
    symptom: 'cannot find config/orchestrator.yaml',
    cause: 'The server started in the wrong working directory',
    fix: 'Use the .mcp.json from the release — it handles this for you',
    at: 40,
  },
  {
    symptom: 'Cannot copy .plug.dll — file in use',
    cause: 'Dwarf Fortress is still running and has the file locked',
    fix: 'Close DF completely, then copy again',
    at: 58,
  },
]

const TableRow: React.FC<{ row: Row; frame: number; fps: number }> = ({ row, frame, fps }) => {
  const local = frame - row.at * fps
  const opacity = interpolate(local, [0, 16], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  const ty = interpolate(local, [0, 16], [18, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '1fr 1fr 1fr',
        gap: 40,
        padding: '30px 0',
        borderTop: `1px solid ${colors.rule}`,
        opacity,
        transform: `translateY(${ty}px)`,
      }}
    >
      <div style={{ fontFamily: fonts.mono, fontSize: 25, color: colors.danger, lineHeight: 1.35 }}>
        {row.symptom}
      </div>
      <div style={{ fontFamily: fonts.sans, fontSize: 25, color: colors.boneDim, lineHeight: 1.35 }}>
        {row.cause}
      </div>
      <div style={{ fontFamily: fonts.sans, fontSize: 25, color: colors.ok, lineHeight: 1.35 }}>
        {row.fix}
      </div>
    </div>
  )
}

/** Full-frame symptom → cause → fix reference for chapter 8. */
export const TroubleshootingTable: React.FC = () => {
  const frame = useCurrentFrame()
  const { fps, durationInFrames } = useVideoConfig()

  const headFade = interpolate(frame, [0, 18], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  const outFade = interpolate(frame, [durationInFrames - 20, durationInFrames], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  const headCell = {
    fontFamily: fonts.sans,
    fontSize: 19,
    fontWeight: 700,
    letterSpacing: '0.24em',
    textTransform: 'uppercase' as const,
    color: colors.boneDim,
  }

  return (
    <AbsoluteFill
      style={{
        background: colors.bg,
        flexDirection: 'column',
        justifyContent: 'center',
        padding: '0 110px',
        opacity: outFade,
      }}
    >
      <div
        style={{
          fontFamily: fonts.sans,
          fontSize: 58,
          fontWeight: 700,
          color: colors.bone,
          marginBottom: 44,
          opacity: headFade,
        }}
      >
        When it breaks
      </div>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr 1fr',
          gap: 40,
          paddingBottom: 18,
          opacity: headFade,
        }}
      >
        <div style={headCell}>What you see</div>
        <div style={headCell}>Why</div>
        <div style={headCell}>Fix</div>
      </div>

      {ROWS.map((row) => (
        <TableRow key={row.symptom} row={row} frame={frame} fps={fps} />
      ))}
    </AbsoluteFill>
  )
}
```

- [ ] **Step 2: Register it in Root**

```tsx
import { TroubleshootingTable, TROUBLESHOOTING_SECONDS } from './segments/TroubleshootingTable'
```

```tsx
    <Composition
      id="TroubleshootingTable"
      component={TroubleshootingTable}
      durationInFrames={seconds(TROUBLESHOOTING_SECONDS)}
      width={VIDEO.width}
      height={VIDEO.height}
      fps={VIDEO.fps}
    />
```

- [ ] **Step 3: Typecheck and render**

Run:
```bash
npm run tsc
npm run render:troubleshooting
```
Expected: no tsc output; `out/dfai/troubleshooting.mp4`.

- [ ] **Step 4: Confirm all four rows are legible at 1080p**

Open `out/dfai/troubleshooting.mp4` and scrub to the final second. All four rows must be visible simultaneously without overflowing the frame. If the last row is clipped, reduce row `padding` from `30px 0` to `22px 0`, re-render, and check again.

- [ ] **Step 5: Commit (if applicable)**

```bash
git add df-ai-tutorial/
git commit -m "feat(df-ai-tutorial): troubleshooting table segment"
```

---

### Task C7: Full render and Premiere import check

**Files:** none — this is a verification task.

**Interfaces:**
- Consumes: everything in Phase C.
- Produces: a complete `out/dfai/` asset set, verified importable into Premiere with a working alpha channel.

- [ ] **Step 1: Render everything from clean**

Run from `df-ai-tutorial/`:
```powershell
Remove-Item -Recurse -Force out/dfai -ErrorAction SilentlyContinue
npm run render:overlays
npm run render:chapters
npm run render:coldopen
npm run render:architecture
npm run render:troubleshooting
npm run render:outro
npm run render:turnloop
```
Expected: every command exits 0.

- [ ] **Step 2: Inventory the output**

Run:
```powershell
Get-ChildItem out/dfai | Select-Object Name, Length | Format-Table -AutoSize
```
Expected: one `.mov` per manifest overlay, `turn-loop-ring.mov`, ten `chapter-NN.mp4`, plus `cold-open.mp4`, `architecture.mp4`, `troubleshooting.mp4`, `outro.mp4`. **No zero-length files.**

- [ ] **Step 3: Confirm every .mov carries alpha and every .mp4 does not**

Run:
```powershell
Get-ChildItem out/dfai -Filter *.mov | ForEach-Object {
  $fmt = & ffprobe -v error -select_streams v:0 -show_entries stream=pix_fmt -of csv=p=0 $_.FullName
  if ($fmt -notmatch 'yuva') { Write-Host "FAIL (no alpha): $($_.Name) -> $fmt" } else { Write-Host "ok: $($_.Name)" }
}
Get-ChildItem out/dfai -Filter *.mp4 | ForEach-Object {
  $fmt = & ffprobe -v error -select_streams v:0 -show_entries stream=pix_fmt -of csv=p=0 $_.FullName
  if ($fmt -match 'yuva') { Write-Host "FAIL (unwanted alpha): $($_.Name) -> $fmt" } else { Write-Host "ok: $($_.Name)" }
}
```
Expected: every line starts with `ok:`.

- [ ] **Step 4: Import into Premiere and verify compositing**

In Premiere: create a 1920×1080 30fps sequence. Put any screen capture on V1. Drop `out/dfai/cmd-claude-version.mov` on V2.

Expected: the command chip appears over the capture with the capture visible around it. **If it shows as a black rectangle, the alpha isn't being interpreted** — right-click the clip → Modify → Interpret Footage → Alpha Channel → *Straight (Unmatted)*. If that fails, fall back to the WebM/VP9 alpha path: re-render with `--codec=vp9 --pixel-format=yuva420p` and an `.webm` extension.

- [ ] **Step 5: Record the outcome**

Append a short "Render and import notes" section to `docs/video/capture-shot-list.md` stating which alpha interpretation Premiere needed, so a re-render months from now doesn't rediscover it.

- [ ] **Step 6: Commit**

```bash
git add docs/video/capture-shot-list.md
git commit -m "docs: record Premiere alpha interpretation for tutorial overlays"
```

---

## Execution order and parallelism

- **Phase A is the critical path** — nothing can be recorded until the release exists. Tasks A1 → A2 → A3 → A4 → A5 are strictly sequential.
- **Phase C depends on Phase A only through Task B1** (the manifest). Tasks C1 and A1 can start in parallel; C2 blocks on B1 Step 6.
- **Task B1 depends on A3 and A4** for exact `INSTALL.txt` and README wording quoted on screen.
- Recording happens after A5 and C7 both land. Recording, editing, and upload are outside this plan.

## Out of scope

- Recording, editing, and uploading the video.
- Accepting community fort logs and learnings via PRs (raised during design; a separate future project).
- Testing DeepSeek or any other non-Claude model end-to-end. Appendix B states this is untested, and that must remain true unless someone actually tests it before recording.
- Adapting the plugin to Mac or Linux.
