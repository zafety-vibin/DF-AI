# Feature Specification: DFHack Binary Protocol

**Feature Branch**: `001-binary-protocol`
**Created**: 2025-11-04
**Status**: Draft
**Input**: User description: "DFHack Plugin Binary Protocol - Communication between DFHack and Go orchestrator"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Initial Fort State Synchronization (Priority: P1)

When the Go orchestrator starts up and connects to a running Dwarf Fortress game, it needs to receive a complete snapshot of the current fort state to begin making decisions.

**Why this priority**: This is the foundation - without the initial state, the AI cannot function at all. This is the absolute MVP.

**Independent Test**: Start DF with an existing fort, start the Go server, verify the server receives and can query all tile states for the entire map.

**Acceptance Scenarios**:

1. **Given** DF is running with a 100x100x10 fortress, **When** the Go server starts and connects, **Then** the server receives complete tile data for all 100,000 tiles within 2 seconds
2. **Given** the Go server has received initial state, **When** querying tile (50,50,5), **Then** the server returns accurate tile type (open/solid/liquid)
3. **Given** DF has a fort with mined areas and buildings, **When** initial sync completes, **Then** the Go server can distinguish between solid rock, open space, and constructed walls

---

### User Story 2 - Real-Time Change Notifications (Priority: P2)

As the game progresses, tiles change constantly (dwarves mining, water flowing, buildings constructed). The Go server needs to be notified of these changes to maintain an accurate view.

**Why this priority**: Required for the AI to react to game state changes. Without this, the AI works only with stale data and makes poor decisions.

**Independent Test**: With the Go server connected, designate mining in DF, wait for dwarves to mine, verify the server is notified when tiles change from solid to open.

**Acceptance Scenarios**:

1. **Given** the Go server is connected and synced, **When** a dwarf mines a tile in DF, **Then** the server receives a notification about that tile changing from solid to open within 1 second
2. **Given** water is flowing in the fort, **When** water spreads to new tiles, **Then** the server receives updates about affected tiles becoming liquid
3. **Given** a dwarf constructs a wall, **When** construction completes, **Then** the server is notified that the tile changed from open to constructed

---

### User Story 3 - Connection Recovery (Priority: P3)

If the connection between DF and the Go server drops (network issue, DF restart, server restart), the system should automatically reconnect and resynchronize without crashing either side.

**Why this priority**: Critical for reliability during long experiments. We can't have the entire system fail just because of a temporary connection issue.

**Independent Test**: With both systems running and connected, forcibly kill the connection (close socket), verify both sides detect the failure and automatically reconnect and resync.

**Acceptance Scenarios**:

1. **Given** both systems are connected, **When** the network connection drops, **Then** both sides detect the failure within 5 seconds and log the disconnection
2. **Given** the connection has been lost, **When** both systems are still running, **Then** they automatically attempt to reconnect every 10 seconds
3. **Given** reconnection succeeds, **When** the connection is re-established, **Then** the Go server requests and receives a full state resync
4. **Given** DFHack is restarted while Go server is running, **When** DFHack comes back online, **Then** the connection is re-established automatically

---

### User Story 4 - Diagnostic Logging (Priority: P4)

When things go wrong with the protocol (malformed messages, timing issues, unexpected disconnections), developers need detailed logs to diagnose and fix problems.

**Why this priority**: Essential for research and debugging, but the system can technically function without perfect logging. Aligns with constitution principle II.

**Independent Test**: Trigger various protocol events (connect, disconnect, send messages, errors), verify all events appear in structured logs with timestamps and relevant details.

**Acceptance Scenarios**:

1. **Given** the protocol is operating, **When** any connection event occurs (connect/disconnect/reconnect), **Then** a structured log entry is written with timestamp and connection details
2. **Given** messages are being transmitted, **When** a message is sent or received, **Then** log entry includes message type, size, and transmission time
3. **Given** an error occurs (malformed message, timeout, etc.), **Then** log entry includes error type, context, and any relevant message data for debugging
4. **Given** we need to analyze performance, **When** reviewing logs, **Then** we can calculate metrics like average message size, transmission latency, and throughput

---

### Edge Cases

- What happens when DF sends a partial message and then crashes? (Server should detect incomplete message and discard it, log the error)
- How does the system handle very large maps (200x200x100 = 4M tiles)? (May need chunked transmission or compression)
- What if tile changes happen faster than they can be transmitted? (Should batch updates or have a maximum update rate)
- What happens if the Go server is slow to process updates? (Should have buffering, but prevent unbounded memory growth)
- How to handle protocol version mismatches? (DFHack plugin v2 talking to Go server v1 - should detect and log version incompatibility)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST establish a bidirectional communication channel between DFHack plugin and Go server
- **FR-002**: System MUST support full fort state transfer (all tile data) from DFHack to Go server
- **FR-003**: System MUST support incremental tile change notifications from DFHack to Go server
- **FR-004**: System MUST transmit tile state including at minimum: coordinates (x, y, z), type (open/solid/liquid), and basic tile properties
- **FR-005**: Go server MUST be able to request a full state resync at any time
- **FR-006**: System MUST detect connection failures within 5 seconds on both sides
- **FR-007**: System MUST automatically attempt reconnection when connection is lost
- **FR-008**: System MUST log all protocol events (connections, disconnections, messages, errors) with timestamps
- **FR-009**: System MUST handle graceful shutdown (close connections cleanly, flush buffers)
- **FR-010**: System MUST validate message integrity (detect corrupted or incomplete messages)
- **FR-011**: Protocol MUST include version information to detect compatibility issues

### Key Entities

- **Connection**: Represents the communication channel between DFHack and Go server, with states (disconnected, connecting, connected, error)
- **Tile State**: The data representing a single tile (coordinates, type, properties) at a point in time
- **Message**: A protocol message containing either full state dump, incremental update, control command, or error information
- **Session**: A complete interaction from initial connection through to disconnection, tracked for logging and debugging

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Go server receives complete state for a 100x100x100 fort (1M tiles) in under 2 seconds
- **SC-002**: Tile change notifications arrive at Go server within 1 second of the change occurring in DF
- **SC-003**: Connection failures are detected within 5 seconds by both sides
- **SC-004**: After connection failure, systems successfully reconnect and resync within 30 seconds without manual intervention
- **SC-005**: Zero crashes on either side due to protocol errors (malformed messages, connection issues) during 24-hour test run
- **SC-006**: 100% of protocol events (connect, disconnect, message sent/received, errors) are logged with structured data
- **SC-007**: Protocol overhead adds less than 5% performance impact to DF gameplay (measured by FPS)
- **SC-008**: Logs contain sufficient detail to diagnose and reproduce any protocol failure within 15 minutes of occurrence

### Assumptions

- DFHack provides a plugin API that can access tile data and register event callbacks for tile changes
- Network communication occurs on localhost (same machine) for initial implementation - latency is negligible
- Go server starts after DF is already running (server connects to plugin, not vice versa)
- Binary format is more efficient than JSON/text for 1M+ tile transmissions
- Tile data structure is relatively stable and won't require constant protocol version updates
- A single DF instance connects to a single Go server instance (no multi-client scenarios)

### Out of Scope

- Sending commands FROM Go server TO DFHack (command execution is a separate feature)
- Authentication or encryption (localhost only, single-player game, no security requirements)
- Multi-client support (multiple Go servers connecting to one DF instance)
- Historical state storage or replay capability (protocol only handles current state)
- Cross-network communication (only localhost for initial implementation)
