# Current Status: Autonomous DF AI (2025-11-08)

## What Works ✅

**Data Collection & Overlays**:
- Topology overlay: 172 KB bit array, 67.5% open tiles
- Hazard overlays: Aquifer (4737), Water (14573), Lava (335), Caverns (47591)
- Modification tracking: Detects player changes, extracts chambers via flood-fill
- Entity tracking: Receives and stores 63 entities from ENTITY_UPDATE
- Auto-update: Sends tile/entity data every 100s

**AI Integration**:
- Claude Haiku configured ($0.02/hour cost)
- Context assembly: 4-level viewport system
- Entity cache: AI now sees "18 dwarves" (was showing 0 - FIXED!)
- Embark point detection: Calculates dwarf centroid, caches permanently
- LLM calls working: Receives responses with reasoning and commands
- Command parsing: Extracts dig/chop/gather from AI text
- Conversation history: 5-turn sliding window

**Protocol**:
- Bidirectional commands: COMMAND (0x09), COMMAND_ACK (0x0A)
- DFHack plugin rebuilt for 53.02-r2 (version match)
- TCP communication stable
- All message types working

## What Doesn't Work ❌

**Critical Issue: Dig Designations Not Appearing in DF**:
- AI issues dig command: `dig (40,30,120) to (52,46,120)`
- Server sends COMMAND message to plugin
- Plugin receives, parses coordinates
- Plugin calls `applyDigDesignation()`
- Plugin sends ACK status=0 (SUCCESS)
- **BUT: No blue 'd' markers appear in DF UI**
- **Dwarves don't mine**

**Debugging Done**:
1. ✅ Added MapCache (vs direct block access)
2. ✅ Added `cache.setDesignationAt(pos, des)` (vs modifying memory directly)
3. ✅ Added `cache.WriteAll()` to flush changes
4. ✅ Rebuilt for DFHack 53.02-r2 (version match)
5. ⏳ Added debug logging (not yet tested)
6. ⏳ Adding topology slice to context (in progress)

**Current Hypothesis**:
- AI is blind-digging (no terrain visibility)
- Coordinates might be wrong Z-level or in fog of war
- Or designation write isn't actually reaching DF despite WriteAll()

## Next Steps (Fresh Session)

### Immediate Debugging

1. **Test with debug logging** (already added to plugin):
   - Rebuild plugin
   - Connect and let AI dig
   - Check DFHack console for DEBUG messages
   - Look for: "skipped X hidden, designated Y tiles, WriteAll() result"

2. **Add topology slice to Level 1 context** (partially done):
   - Add TopologySliceData type
   - Implement extractTopologySlice()
   - Include 60×60 terrain map on first turn
   - AI sees walls/floors/open space
   - AI can make informed decisions about WHERE to dig

3. **Manual test command**:
   - Add `ai-test-dig X Y Z` plugin command
   - Directly calls applyDigDesignation with known-good coords
   - Bypasses AI/server to isolate plugin issue

### Alternative Approaches if Designation Still Fails

**Option A: Use DFHack dig command**:
```cpp
Core::getInstance().runCommand(out, "dig rect");
// But requires setting cursor position first
```

**Option B: Use Lua API**:
```cpp
Core::getInstance().runCommand(out, "devel/luacmd dfhack.maps.getTileBlock(...).designation[x][y].dig = 1");
```

**Option C: Direct DF memory write**:
- Research DF memory offsets
- Write designation directly to DF process memory
- Nuclear option, very fragile

### Code Locations

**Plugin dig code**: `dfhack-plugin/df_ai_protocol.cpp` line 199-277
**Context assembly**: `internal/context/assembly.go` line 109-201
**Entity cache**: `internal/autonomous/loop.go` line 632-645
**Command execution**: `internal/commands/executor.go` line 50-121

### Files Modified (Not Pushed to GitHub)

**Branch**: 005-llm-integration
**Commits since last push** (local only):
- c68ebe2 - Entity cache fix
- 1a43700 - CHOP/GATHER commands
- 19acc0e - Feedback crash fix

**Changes ready to commit**:
- Debug logging in plugin
- Topology slice support (partial)
- cache.WriteAll() flush

## Testing Protocol

**Current best test**:
1. Fresh embark (clean baseline)
2. Unpause briefly (dwarves appear)
3. Connect: `ai-connect` → `ai-send-entities`
4. Wait ~5s for AI decision
5. **Check DFHack console** for DEBUG messages
6. Check DF UI for 'd' markers
7. Check `/ai/history` endpoint for AI reasoning

## Configuration

**Current config** (`config/orchestrator.yaml`):
- Model: claude-haiku-4-5-20251001
- API key: (in file, not env var - env var expansion not implemented)
- Update frequency: 100s
- Max tokens: 2048
- Temperature: 0.7

## Known Issues

1. **Entity classification**: Using tame/resident flags (seems to work, gets ~18 dwarves)
2. **Modification tracking**: Session-scoped only (loaded saves lose history)
3. **Z-coordinate system**: Using 0-153 absolute, seems correct for df::coord()
4. **System prompt**: Sent every turn (not cached - costs tokens)
5. **Blueprint support**: Not implemented yet
6. **CHOP command**: Uses `chop-designate` (command may not exist)
7. **GATHER command**: Uses `getplants all` (works globally, not by region)

## Cost So Far

Approximately $0.10-0.15 in Claude API calls during testing (mostly debugging entity cache issues).

## Next Session Goals

1. Get dig designations working and visible in DF
2. Verify dwarves actually mine designated tiles
3. Add topology slice so AI can see terrain
4. Cache system prompt to reduce token costs
5. Test chop and gather commands
6. Add blueprint support
7. Push all commits to GitHub when "it works"
