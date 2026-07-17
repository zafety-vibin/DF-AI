# Feature Specification: LLM Integration with Modification Tracking and Command Execution

**Feature Branch**: `005-llm-integration`
**Created**: 2025-11-07
**Status**: Draft

## User Scenarios & Testing

### User Story 1 - Modification Tracking Overlay (Priority: P1)

The AI needs to understand what has been built/modified in the fort rather than viewing the entire natural map state. Track only player and AI-initiated tile changes (mining, construction, designations) as a sparse overlay that becomes the "fort signature" showing actual work performed.

**Why this priority**: Without modification tracking, AI cannot distinguish between natural terrain and player work. This is foundational for all other AI interactions - the modification overlay IS the fort from the AI's perspective. Must be implemented first.

**Independent Test**: Start with untouched embark, mine 3 chambers and build 2 walls. Modification overlay should contain only those changed tiles (~150 modifications) not the 6.9M natural tiles. Query modification bounds returns min/max XYZ of work area. Flood-fill extraction identifies 3 connected chambers. Memory usage under 100 KB for typical early fort.

**Acceptance Scenarios**:

1. **Given** fresh embark with no modifications, **When** dwarf mines first tile, **Then** modification overlay adds entry with action type DUG, current timestamp, and null command ID (player action)
2. **Given** modification overlay tracking 500 mined tiles, **When** AI issues dig command and dwarf executes, **Then** new modifications added with command ID linking to AI decision
3. **Given** scattered modifications across multiple Z-levels, **When** flood-fill chamber extraction runs, **Then** returns list of connected spaces with bounds and tile counts
4. **Given** modifications spanning (10,20,95) to (50,60,98), **When** query modification bounds, **Then** returns correct min/max XYZ coordinates
5. **Given** designation placed but not yet executed, **When** designation cancelled, **Then** modification overlay adds CANCELLED entry with timestamp

---

### User Story 2 - Context Assembly with Viewports (Priority: P1)

Format fort state into staged detail levels optimized for LLM token efficiency. Level 0 provides text overview (500 bytes), Level 1 shows active work area with features (10 KB), Level 2 expands to caverns (50 KB), Level 3 provides full context (200 KB). Each level builds on previous with progressive detail.

**Why this priority**: Context budget is hard constraint (200 KB). Without viewport system, we'd waste tokens sending irrelevant data (underground caverns when AI working on surface). Staged detail matches natural gameplay progression.

**Independent Test**: With active fort at Z=95-98 containing 3 mined chambers and aquifer nearby, generate Level 0 context (under 1 KB text), Level 1 context (under 15 KB with chamber features, hazards, dwarves), verify token counts. AI query for water sources in Z=80-100 returns coordinate list. AI request for topology slice Z=95 returns data for that level only.

**Acceptance Scenarios**:

1. **Given** early game fort with surface modifications only, **When** generate Level 0 overview, **Then** returns text summary under 500 bytes describing embark year, dwarf count, food/drink status, and work focus
2. **Given** modifications at work focus defined z area, **When** generate Level 1 active area context, **Then** includes modification bounds, chamber rectangles extracted from flood-fill, hazards within bounds plus 10 tile margin, dwarf positions, total under 15 KB
3. **Given** AI requests cavern exploration, **When** generate Level 2 deep planning, **Then** expands to Z=10-90 cavern layers with water/lava sources, under 50 KB
4. **Given** AI explicitly requests full map, **When** generate Level 3 emergency context, **Then** sends complete compressed topology and all overlays under 200 KB
5. **Given** AI queries "show topology slice Z=95, X=10-30, Y=20-40", **When** query API processes request, **Then** returns only specified region as JSON coordinate array or ASCII representation
6. **Given** AI queries "list water sources in Z=80-100", **When** query processes, **Then** filters water hazard overlay to specified Z range and returns coordinate list or flood fill bounds

---

### User Story 3 - First Interaction (Priority: P1)

Send initial prompt to LLM with fort state context and receive first AI response. Parse response to extract reasoning and proposed commands. Demonstrates end-to-end LLM communication pipeline.

**Why this priority**: First tangible AI interaction - proves the system works. Required to validate context format is understandable and LLM can generate useful responses. Deliverable: AI describes fort and suggests action. Should describe what to do in a scenario with no player made modificaitons made yet to the gamestate. 

**Independent Test**: With fort in early embark state, send prompt with Level 0 + Level 1 context to Claude API or local LLM. AI returns description of fort state and suggests one action (e.g., "dig bedrooms"). Parse response extracts reasoning text and command proposal. Log shows full context sent, response received, token counts, and latency under 10 seconds. 

**Acceptance Scenarios**:

1. **Given** LLM configured with Claude API credentials, **When** send first prompt with Level 0+1 context, **Then** receives response within 10 seconds with AI description and action suggestion
2. **Given** AI response contains reasoning and dig command, **When** parse response, **Then** extracts reasoning as text and command as structured data (coordinates, action type)
3. **Given** LLM interaction completes, **When** check logs, **Then** contains full context sent (size in bytes), full response received, token usage (prompt + completion), and millisecond timing
4. **Given** configured for local LLM endpoint, **When** send prompt to OpenAI-compatible API, **Then** successfully communicates and receives response
5. **Given** LLM returns unstructured response without clear command, **When** parse, **Then** logs reasoning but indicates no actionable command found

---

### User Story 4 - Command Protocol Bidirectional (Priority: P1)

Define protocol messages for Server to send commands to DFHack (DESIGNATE_DIG, DESIGNATE_BUILD, DESIGNATE_CANCEL). DFHack receives commands, executes via df.designation API, and acknowledges success or failure. Completes the bidirectional communication enabling AI to take actions.

**Why this priority**: Current protocol is one-way (DFHack → Server). AI cannot take actions without this. Critical for autonomous fort management. Once implemented, AI transitions from observer to participant.

**Independent Test**: Server generates COMMAND message with dig designation for coordinates (10,20,95) to (15,25,95) and unique command ID. DFHack plugin receives message, applies designation using df.designation API, sends COMMAND_ACK with success status and command ID. Server receives acknowledgment within 1 second. Verify designation appears in DF UI and dwarves begin mining.

**Acceptance Scenarios**:

1. **Given** server wants to issue dig command, **When** create DESIGNATE_DIG message with target region and command ID, **Then** serializes to binary format with message type, coordinates, and ID
2. **Given** DFHack receives DESIGNATE_DIG message, **When** parses and executes, **Then** applies designation to specified tiles using df.designation API and tiles marked for mining in DF
3. **Given** designation successfully applied, **When** DFHack sends acknowledgment, **Then** COMMAND_ACK message includes original command ID and success status
4. **Given** designation fails (invalid coordinates or occupied tiles), **When** DFHack attempts execution, **Then** sends COMMAND_ACK with failure status and error message
5. **Given** server sends DESIGNATE_BUILD message for wall construction, **When** DFHack executes, **Then** applies construction designation and acknowledges
6. **Given** server sends DESIGNATE_CANCEL for previous designation, **When** DFHack executes, **Then** removes designation and acknowledges

---

### User Story 5 - Command Execution Loop (Priority: P2)

Implement full autonomous cycle: AI observes fort state, proposes action, server sends command, receives result, updates modification tracking, sends feedback to AI. AI learns from outcomes including failures. Repeat cycle automatically or on-demand.

**Why this priority**: Integrates all previous stories into autonomous operation. Enables AI to learn from experience with hazards and failures rather than being blocked by safety rules. This is the core research value - observing AI decision-making and learning patterns.

**Independent Test**: With AI configured and modification overlay active, start autonomous loop. Every 100 seconds: send fort context to AI, AI suggests action, server sends command to DFHack, receive acknowledgment, wait for TILE_UPDATE showing changes, send feedback to AI with results. Run for 10 minutes (6 cycles), verify AI receives feedback about successful digs and any hazard encounters. Logs contain full decision chain for each cycle.

**Acceptance Scenarios**:

1. **Given** autonomous loop enabled, **When** 100 seconds elapse, **Then** assembles current context (modification overlay + viewport), sends to LLM, waits for response
2. **Given** AI suggests dig command with reasoning, **When** server processes, **Then** validates command has coordinates and action type, sends to DFHack with unique ID, awaits acknowledgment
3. **Given** command acknowledged as success, **When** subsequent TILE_UPDATE shows new DUG modifications, **Then** updates modification overlay with command ID linking AI decision to outcome
4. **Given** modifications detected after AI command, **When** send feedback to AI, **Then** includes result description like "Successfully dug 15 tiles at target location, chamber expanded as planned"
5. **Given** AI command encounters hazard (dig reveals aquifer), **When** TILE_UPDATE shows water tiles appear, **Then** feedback to AI describes "Aquifer breached at (12,22,94), water flowing into chamber" allowing AI to learn consequence
6. **Given** AI requests immediate update, **When** receives request, **Then** triggers cycle immediately instead of waiting for 100 second timer

---

### User Story 6 - Configuration and Provider Support (Priority: P2)

Configure LLM provider (Claude API or local model endpoint), model parameters (temperature, token limits), context budget settings, viewport detail levels, and support multi-model pipelines. Modular design allows swapping providers for experimentation.

**Why this priority**: Configuration enables research flexibility. Need to test different models (Claude vs local), different prompts, different context strategies. Provider modularity critical for cost control and experimentation with fine-tuned models.

**Independent Test**: Configure for Claude API with sonnet-4 model, send test prompt, verify response. Reconfigure for local LLM endpoint (e.g., localhost:8000 OpenAI-compatible), send same prompt, verify response. Change temperature from 0.7 to 0.2, observe more deterministic responses. Adjust context budget from 200 KB to 50 KB, verify viewport limits enforced. Configure multi-model pipeline with feature extractor then planner, verify both called in sequence.

**Acceptance Scenarios**:

1. **Given** config file with Claude API key and model name, **When** initialize LLM client, **Then** successfully authenticates and sets model to claude-sonnet-4
2. **Given** config specifies local endpoint URL localhost:8000, **When** initialize client, **Then** uses OpenAI-compatible HTTP client pointing to local server
3. **Given** temperature set to 0.2 in config, **When** send prompts, **Then** LLM responses are more deterministic and consistent
4. **Given** context budget set to 100 KB, **When** assemble context, **Then** viewport system enforces limit and truncates to highest priority data
5. **Given** viewport config specifies active Z range as plus/minus 5 levels, **When** assemble Level 1 context, **Then** includes only modifications within 5 Z-levels of highest modified tile
6. **Given** multi-model pipeline configured with extractor model and planner model, **When** process context, **Then** extractor receives raw overlays and outputs features, planner receives features and outputs commands

---

### Edge Cases

- What happens when modification overlay detects conflicting changes (tile state changes but doesn't match any known modification type)?
- How does system handle LLM timeout or API failure (retry logic, fallback to cached response)?
- What if AI suggests command with invalid coordinates (out of bounds or in inaccessible area)?
- How does modification tracking handle simultaneous player and AI actions on same tile?
- What happens when context assembly exceeds budget even at Level 0 (massive fort with thousands of modifications)?
- How does query API handle malformed queries from AI?
- What if DFHack command acknowledgment never arrives (network issue or plugin crash)?
- How does system detect modification baseline for saves loaded mid-game (fort already has modifications)?

## Requirements

### Functional Requirements

- **FR-001**: System MUST track tile modifications as sparse overlay mapping coordinates to modification type (DUG, BUILT_WALL, BUILT_FLOOR, DESIGNATION_DIG, DESIGNATION_BUILD, CANCELLED)
- **FR-002**: Modification overlay MUST detect changes by comparing TILE_UPDATE tile states against natural baseline state
- **FR-003**: Each modification MUST store action type, timestamp, and optional command ID linking to AI decision
- **FR-004**: System MUST provide modification bounds query returning minimum and maximum XYZ coordinates of all player/AI work
- **FR-005**: System MUST flood-fill connected modified spaces to extract chambers as feature list with bounds
- **FR-006**: Context assembly MUST support four detail levels: Level 0 (text only, 500 bytes), Level 1 (active area, 10 KB), Level 2 (caverns, 50 KB), Level 3 (full, 200 KB)
- **FR-007**: Level 0 overview MUST include embark stage, dwarf count, resource status as natural language text
- **FR-008**: Level 1 active area MUST include modification bounds, chamber rectangles, hazards within modification area plus 10 tile margin, and dwarf positions
- **FR-009**: Level 2 deep planning MUST expand to cavern layers (Z=10-90) when requested, including water and lava source locations
- **FR-010**: Level 3 emergency MUST provide complete compressed topology and all overlays only when AI explicitly requests
- **FR-011**: System MUST provide query API allowing AI to request specific data: topology slices, hazard lists in Z-range, pathfinding between coordinates
- **FR-012**: Context format MUST use hybrid JSON with text descriptions, feature lists (chambers as rectangles), hazard coordinates, and dwarf position vectors
- **FR-013**: System MUST send prompts to LLM via configurable provider (Claude API or OpenAI-compatible local endpoint)
- **FR-014**: Initial prompt MUST include Level 0 overview and Level 1 active area, asking AI to describe fort state and suggest one safe action
- **FR-015**: System MUST parse LLM responses to extract reasoning text and any proposed commands
- **FR-016**: All LLM interactions MUST be logged with full context sent, response received, token usage, and millisecond timing
- **FR-017**: System MUST define command messages from Server to DFHack: DESIGNATE_DIG, DESIGNATE_BUILD, DESIGNATE_CANCEL
- **FR-018**: Each command message MUST include target coordinates or region, action type, and unique command ID for tracking
- **FR-019**: DFHack plugin MUST receive command messages and execute using df.designation API
- **FR-020**: DFHack MUST send acknowledgment message (COMMAND_ACK) with original command ID and success/failure status
- **FR-021**: Server MUST update modification overlay when command acknowledgment received, linking AI decision to fort changes
- **FR-022**: Command execution loop MUST observe fort via modification overlay and viewports, send to LLM, parse response, execute commands, wait for TILE_UPDATE confirmation, send feedback to AI
- **FR-023**: Feedback to AI MUST include outcome description (success with tile counts, or failure with hazard description like "aquifer breached")
- **FR-024**: System MUST NOT block AI commands with hard-coded safety rules - AI learns from hazard outcomes through feedback
- **FR-025**: Execution loop MUST repeat every 100 seconds or when AI explicitly requests immediate update
- **FR-026**: Configuration MUST support LLM provider settings: API key, model name, endpoint URL
- **FR-027**: Configuration MUST support both Claude API and OpenAI-compatible local model endpoints
- **FR-028**: Configuration MUST allow model parameters: temperature, max tokens, other generation settings
- **FR-029**: Configuration MUST specify context budget (default 200 KB), update frequency (default 100s), viewport Z-level ranges and margin distances
- **FR-030**: LLM client MUST use modular interface pattern allowing swapping providers without changing core logic
- **FR-031**: System MUST support multi-model pipeline: feature extractor preprocesses raw data, planner model receives extracted features
- **FR-032**: System MUST support ASCII template loading for blueprint library (room designs as CSV or ASCII files)
- **FR-033**: Chamber extraction MUST describe rooms in natural language (e.g., "3 bedroom chambers, 1 dining hall, connected by 5-tile passage")

### Key Entities

- **ModificationInfo**: Represents a single tile change with action type (DUG, BUILT_WALL, etc.), timestamp when change occurred, and optional command ID linking to AI decision that caused it
- **Chamber**: Connected space of modified tiles extracted via flood-fill, with bounding box (min/max XYZ), tile count, and optional type classification (bedroom, workshop, passage)
- **ViewportContext**: Fort state formatted for LLM consumption at specific detail level (0-3), containing text overview, chamber features, hazard lists, dwarf vectors, and query results
- **LLMPrompt**: Request sent to language model including context data, system instructions, and question or task for AI
- **LLMResponse**: Model output containing reasoning text, proposed commands (if any), and metadata (tokens used, finish reason)
- **Command**: Action instruction from Server to DFHack with type (DIG/BUILD/CANCEL), target coordinates or region, unique command ID, and timestamp
- **CommandAcknowledgment**: Response from DFHack to Server confirming command execution with original ID, success/failure status, error message if failed, and timestamp
- **QueryRequest**: AI request for specific data like topology slice, hazard list, or pathfinding result, with query type and parameters
- **QueryResponse**: Filtered data matching query parameters, formatted as JSON or ASCII depending on request

### Non-Functional Requirements

- **NFR-001**: Context assembly MUST complete within 100ms for Level 0, 500ms for Level 1, 2 seconds for Level 2, 5 seconds for Level 3
- **NFR-002**: Command acknowledgment from DFHack MUST arrive within 1 second of sending command
- **NFR-003**: LLM API calls MUST timeout after 30 seconds to prevent hanging
- **NFR-004**: Modification overlay memory usage MUST stay under 1 MB for forts with 10k modifications
- **NFR-005**: Chamber flood-fill extraction MUST complete within 200ms for modification overlay with 1000 tiles
- **NFR-006**: Query API MUST respond within 100ms for simple queries (coordinate lookups), 1 second for complex queries (pathfinding)
- **NFR-007**: All LLM interactions MUST log to structured format suitable for fine-tuning dataset (JSONL with prompt/completion pairs)
- **NFR-008**: System MUST support concurrent context assembly and command execution without race conditions

## Success Criteria

### Measurable Outcomes

- **SC-001**: AI generates contextually appropriate fort state description and action suggestion within 10 seconds of sending first prompt
- **SC-002**: Context assembly stays within 200 KB budget while including sufficient information for AI to understand fort state and make decisions
- **SC-003**: AI-issued commands execute in Dwarf Fortress with visible designation changes within 2 seconds of command being sent
- **SC-004**: AI receives feedback about command outcomes including hazard interactions and can reference outcomes in subsequent decisions
- **SC-005**: Modification overlay accurately tracks 100% of player and AI tile changes with correct action types and timestamps
- **SC-006**: Chamber extraction identifies distinct work areas from scattered modifications with over 90% accuracy compared to human interpretation
- **SC-007**: Viewport system reduces context size by at least 90% compared to sending full unfiltered overlays (200 KB → 10-20 KB typical)
- **SC-008**: Query API responds to AI data requests within 1 second for 95% of queries
- **SC-009**: Autonomous execution loop runs continuously for 1 hour with AI making at least 30 decisions without system crashes
- **SC-010**: System successfully operates with both Claude API and local LLM endpoint with identical functionality
- **SC-011**: Full decision chain is logged for every AI turn suitable for offline analysis: context sent, AI reasoning, command issued, execution result, feedback provided

## Constraints

- Context budget: Maximum 200 KB per LLM turn
- Command latency: Acknowledgment within 1 second of sending
- Update frequency: Default 100 seconds (configurable)
- Memory: Modification overlay under 1 MB for typical fort
- LLM timeout: 30 seconds maximum for API calls
- No hard-coded safety blocks: AI learns from hazard outcomes
- Modification tracking starts from embark (loaded saves use heuristic baseline detection)

## Assumptions

- DFHack df.designation API is available and stable for applying dig/build/cancel designations
- Tile baseline (natural state) can be heuristically determined for loaded saves by assuming unmodified tiles have "natural" TileType ranges
- LLM providers support standard OpenAI-compatible API format for local models
- Claude API rate limits allow 100-second update frequency (36 calls/hour)
- Token costs acceptable for research phase (will optimize later if needed)
- AI responses are parseable text (structured or natural language, not binary)
- Flood-fill chamber extraction assumes 6-connected adjacency (not diagonal)
- Blueprint templates (ASCII) are read-only reference, not dynamic generation
- Command IDs are unique uint32 values, monotonically increasing per session
- Feedback to AI is append-only to conversation context (maintains history)

## Dependencies

- Feature 001: Binary Protocol (bidirectional commands require protocol extension)
- Feature 003: Topology Overlay (viewport filtering uses topology data)
- Feature 004: Hazard Overlays (context includes hazards within viewport)
- Feature 005: This feature (modification tracking is foundational for context)

## Out of Scope

- Advanced pathfinding algorithms (use simple flood-fill for chamber detection, defer A* pathfinding)
- Semantic room type classification (bedroom vs workshop) - use generic "chamber" classification
- Multi-AI collaboration or consensus mechanisms
- Hard-coded safety validation system (Feature 9 - intentionally skipped per constitution)
- Real-time DF game state monitoring beyond tile/entity updates (no economic tracking, no mood monitoring)
- Advanced blueprint generation or procedural design (use templates only)
- Fine-tuning LLM models (log data for future fine-tuning, don't train during this feature)
- Reward function calculation for RL (defer to learning phase)
- Traffic/pathfinding heatmaps (Feature 12-13)
- Historical action tracking beyond command ID linkage (full history is Feature 10-11)
