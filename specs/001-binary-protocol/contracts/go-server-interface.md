# Go Server Interface Contract

**Version**: 1.0
**Date**: 2025-11-04

## Overview

This contract defines the interfaces that the Go orchestrator server MUST implement to handle the DFHack binary protocol.

---

## Package: `internal/protocol`

### Interface: `Connection`

Manages a single TCP connection with a DFHack plugin.

```go
type Connection interface {
    // Connect initiates connection to DFHack plugin
    // Returns error if connection fails or times out
    Connect(host string, port uint16) error

    // Close gracefully closes the connection
    // Sends DISCONNECT message before closing socket
    Close() error

    // Send transmits a message to the plugin
    // Returns error if serialization or transmission fails
    Send(msg Message) error

    // Receive blocks until a message is received
    // Returns error on timeout, parse failure, or connection closed
    Receive() (Message, error)

    // State returns current connection state
    State() ConnectionState

    // SessionID returns the unique ID for this connection
    SessionID() uint64
}
```

**Implementation Requirements**:
- Must handle TCP socket management
- Must implement heartbeat mechanism (send every 10s, timeout after 5s no response)
- Must log all connection state changes
- Must handle graceful shutdown
- Must support reconnection logic with exponential backoff

---

### Interface: `Message`

Base interface for all protocol messages.

```go
type Message interface {
    // Type returns the message type ID (0x01-0x07)
    Type() uint8

    // Serialize converts message to binary format
    // Returns byte slice with length-prefixed header
    Serialize() ([]byte, error)

    // Deserialize populates message from binary data
    // Returns error if data is malformed or wrong type
    Deserialize(data []byte) error

    // Validate checks message fields are valid
    // Returns error describing validation failure
    Validate() error
}
```

**Concrete Types** (must implement `Message`):
- `HandshakeMessage`
- `FullStateMessage`
- `TileUpdateMessage`
- `ResyncRequestMessage`
- `HeartbeatMessage`
- `ErrorMessage`
- `DisconnectMessage`

---

### Type: `HandshakeMessage`

```go
type HandshakeMessage struct {
    ProtocolVersion uint8
    ConnectionID    uint32
    Capabilities    string
}

func (m *HandshakeMessage) Type() uint8 { return 0x01 }
```

---

### Type: `FullStateMessage`

```go
type FullStateMessage struct {
    Width  uint16
    Height uint16
    Depth  uint16
    Tiles  []TileState
}

func (m *FullStateMessage) Type() uint8 { return 0x02 }
```

**Validation Rules**:
- `len(Tiles) == Width * Height * Depth`
- All coordinates in Tiles must be within bounds
- Tiles must be in row-major order (X fastest)

---

### Type: `TileState`

```go
type TileState struct {
    X        int16
    Y        int16
    Z        int16
    TileType uint16
    Flags    uint8
}

// Flag bit masks
const (
    FlagHidden      = 0x01
    FlagDiscovered  = 0x02
    FlagDesignated  = 0x04
    FlagConstruct   = 0x08
)
```

---

### Type: `TileUpdateMessage`

```go
type TileUpdateMessage struct {
    Count uint32
    Tiles []TileState
}

func (m *TileUpdateMessage) Type() uint8 { return 0x03 }
```

**Validation Rules**:
- `len(Tiles) == Count`
- `Count > 0`

---

### Type: `ResyncRequestMessage`

```go
type ResyncRequestMessage struct {
    Reason uint8
}

func (m *ResyncRequestMessage) Type() uint8 { return 0x04 }

// Reason codes
const (
    ReasonManual       = 0x00
    ReasonInconsistency = 0x01
    ReasonReconnect    = 0x02
    ReasonPeriodic     = 0x03
)
```

---

### Type: `HeartbeatMessage`

```go
type HeartbeatMessage struct {
    Timestamp uint64  // Unix milliseconds
    Sequence  uint8
}

func (m *HeartbeatMessage) Type() uint8 { return 0x05 }
```

---

### Type: `ErrorMessage`

```go
type ErrorMessage struct {
    Code    uint16
    Message string
}

func (m *ErrorMessage) Type() uint8 { return 0x06 }

// Error codes
const (
    ErrUnknownType    = 0x0001
    ErrInvalidPayload = 0x0002
    ErrVersionMismatch = 0x0003
    ErrInternal       = 0x0004
)
```

---

### Type: `DisconnectMessage`

```go
type DisconnectMessage struct {
    Reason uint8
}

func (m *DisconnectMessage) Type() uint8 { return 0x07 }

// Disconnect reasons
const (
    ReasonNormalShutdown = 0x00
    ReasonPluginUnload   = 0x01
    ReasonServerShutdown = 0x02
    ReasonRestart        = 0x03
)
```

---

### Type: `ConnectionState`

```go
type ConnectionState int

const (
    StateDisconnected ConnectionState = iota
    StateConnecting
    StateConnected
    StateError
)

func (s ConnectionState) String() string { /* ... */ }
```

---

## Package: `internal/dfhack`

### Interface: `Client`

High-level client for interacting with DFHack.

```go
type Client interface {
    // Start begins listening for DFHack connections
    // Blocks until connection established or ctx cancelled
    Start(ctx context.Context, port uint16) error

    // Stop gracefully shuts down the client
    Stop() error

    // RequestFullState requests complete map dump from plugin
    // Returns channel that will receive FullStateMessage or error
    RequestFullState(reason uint8) (<-chan *FullStateMessage, error)

    // SubscribeTileUpdates returns channel for incremental tile updates
    // Channel receives TileUpdateMessage as they arrive
    SubscribeTileUpdates() <-chan *TileUpdateMessage

    // IsConnected returns true if connection is active
    IsConnected() bool

    // GetSessionMetrics returns metrics for current session
    GetSessionMetrics() SessionMetrics
}
```

**Implementation Requirements**:
- Must implement reconnection logic
- Must handle message routing to appropriate channels
- Must implement heartbeat monitoring
- Must log all protocol events
- Must expose Prometheus metrics

---

### Type: `SessionMetrics`

```go
type SessionMetrics struct {
    SessionID       uint64
    StartTime       time.Time
    EndTime         time.Time
    MessagesSent    uint64
    MessagesReceived uint64
    BytesSent       uint64
    BytesReceived   uint64
    Errors          uint32
    AvgMessageSize  uint32
    LastHeartbeat   time.Time
}
```

---

## Package: `internal/logging`

### Interface: `Logger`

Structured logging interface.

```go
type Logger interface {
    // Debug logs debug-level message with structured fields
    Debug(msg string, fields ...Field)

    // Info logs info-level message with structured fields
    Info(msg string, fields ...Field)

    // Warn logs warning-level message with structured fields
    Warn(msg string, fields ...Field)

    // Error logs error-level message with structured fields
    Error(msg string, err error, fields ...Field)

    // With returns a new logger with additional fields
    With(fields ...Field) Logger
}

type Field struct {
    Key   string
    Value interface{}
}
```

**Implementation**: Use `log/slog` from Go stdlib

**Required Log Fields**:
- `timestamp`: ISO8601
- `level`: DEBUG/INFO/WARN/ERROR
- `component`: "protocol", "dfhack", etc.
- `event`: specific event name
- Additional context fields as needed

---

## Server Configuration

### Type: `Config`

```go
type Config struct {
    // Network
    ListenPort    uint16        `yaml:"listen_port"`
    ReadTimeout   time.Duration `yaml:"read_timeout"`
    WriteTimeout  time.Duration `yaml:"write_timeout"`

    // Protocol
    HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
    HeartbeatTimeout  time.Duration `yaml:"heartbeat_timeout"`
    MaxMessageSize    uint32        `yaml:"max_message_size"`

    // Reconnection
    ReconnectDelay     time.Duration `yaml:"reconnect_delay"`
    ReconnectMaxDelay  time.Duration `yaml:"reconnect_max_delay"`
    ReconnectBackoffFactor float64   `yaml:"reconnect_backoff_factor"`

    // Logging
    LogLevel  string `yaml:"log_level"`   // debug, info, warn, error
    LogFormat string `yaml:"log_format"`  // json, text
}
```

**Default Values**:
- `ListenPort`: 5001
- `ReadTimeout`: 30s
- `WriteTimeout`: 10s
- `HeartbeatInterval`: 10s
- `HeartbeatTimeout`: 5s
- `MaxMessageSize`: 10MB
- `ReconnectDelay`: 1s
- `ReconnectMaxDelay`: 30s
- `ReconnectBackoffFactor`: 2.0
- `LogLevel`: "info"
- `LogFormat`: "json"

---

## Testing Interfaces

### Interface: `MockDFHackClient`

Mock implementation for testing.

```go
type MockDFHackClient struct {
    // Configurable responses
    FullStateResponse *FullStateMessage
    TileUpdates       []*TileUpdateMessage
    ShouldError       bool
    ErrorToReturn     error

    // Call tracking
    RequestFullStateCalled bool
    SubscribeCalled        bool
}

func (m *MockDFHackClient) Start(ctx context.Context, port uint16) error { /* ... */ }
func (m *MockDFHackClient) RequestFullState(reason uint8) (<-chan *FullStateMessage, error) { /* ... */ }
// ... implement all Client methods
```

---

## Example Usage

```go
package main

import (
    "context"
    "log"

    "df-ai/internal/dfhack"
    "df-ai/internal/logging"
)

func main() {
    // Create logger
    logger := logging.NewJSONLogger("info")

    // Create DFHack client
    client := dfhack.NewClient(logger)

    // Start server
    ctx := context.Background()
    if err := client.Start(ctx, 5001); err != nil {
        log.Fatal(err)
    }
    defer client.Stop()

    // Request initial state
    stateChan, err := client.RequestFullState(dfhack.ReasonManual)
    if err != nil {
        log.Fatal(err)
    }

    // Wait for state
    state := <-stateChan
    logger.Info("Received full state",
        logging.Field{Key: "tile_count", Value: len(state.Tiles)})

    // Subscribe to updates
    updates := client.SubscribeTileUpdates()
    for update := range updates {
        logger.Debug("Tile update",
            logging.Field{Key: "changed_tiles", Value: update.Count})
        // Process update...
    }
}
```

---

## Error Handling Contract

All interface methods MUST return meaningful errors:

```go
var (
    ErrNotConnected       = errors.New("not connected to DFHack")
    ErrConnectionTimeout  = errors.New("connection timeout")
    ErrInvalidMessage     = errors.New("invalid message format")
    ErrVersionMismatch    = errors.New("protocol version mismatch")
    ErrMessageTooLarge    = errors.New("message exceeds size limit")
    ErrHandshakeFailed    = errors.New("handshake failed")
)
```

---

## Performance Contracts

Implementations MUST meet these performance targets:

| Operation | Target | Measurement |
|-----------|--------|-------------|
| Message serialization | <1ms | Benchmark with 100K tiles |
| Message deserialization | <5ms | Benchmark with 100K tiles |
| Full state processing | <100ms | Time to store 1M tiles in memory |
| Tile update processing | <10ms | Time to update 100 tiles |
| Heartbeat round-trip | <100ms | Time from send to echo received |

---

## Thread Safety

All public interfaces MUST be safe for concurrent use:
- `Connection` methods can be called from multiple goroutines
- `Client` methods can be called from multiple goroutines
- Message types are immutable after creation
- Channels returned by `Client` are safe for multiple readers

---

## Metrics Exposure

Implementation MUST expose these Prometheus metrics:

```
# Connection metrics
dfhack_connection_state{state="connected|disconnected|error"} gauge
dfhack_connections_total counter
dfhack_connection_errors_total{type="timeout|handshake|protocol"} counter

# Message metrics
dfhack_messages_sent_total{type="handshake|full_state|..."} counter
dfhack_messages_received_total{type="..."} counter
dfhack_message_bytes_sent counter
dfhack_message_bytes_received counter
dfhack_message_send_duration_seconds{type="..."} histogram
dfhack_message_receive_duration_seconds{type="..."} histogram

# Protocol metrics
dfhack_heartbeat_rtt_seconds histogram
dfhack_full_state_tile_count histogram
dfhack_tile_updates_per_second gauge
```