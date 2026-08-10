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
