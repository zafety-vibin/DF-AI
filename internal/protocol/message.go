package protocol

import (
	"errors"
	"fmt"
)

// Message is the base interface for all protocol messages
type Message interface {
	// Type returns the message type ID (0x01-0x07)
	Type() uint8

	// Serialize converts the message to binary format
	// Returns byte slice with full message including length header
	Serialize() ([]byte, error)

	// Deserialize populates the message from binary data
	// Input should be the full message including headers
	// Returns error if data is malformed or wrong type
	Deserialize(data []byte) error

	// Validate checks that message fields are valid
	// Returns error describing validation failure
	Validate() error
}

// Message type constants
const (
	MessageTypeHandshake      uint8 = 0x01
	MessageTypeFullState      uint8 = 0x02
	MessageTypeTileUpdate     uint8 = 0x03
	MessageTypeResyncRequest  uint8 = 0x04
	MessageTypeHeartbeat      uint8 = 0x05
	MessageTypeError          uint8 = 0x06
	MessageTypeDisconnect     uint8 = 0x07
	MessageTypeEntityUpdate   uint8 = 0x08
	MessageTypeCommand        uint8 = 0x09
	MessageTypeCommandAck     uint8 = 0x0A
)

// Common errors
var (
	ErrInvalidMessageLength = errors.New("invalid message length")
	ErrInvalidMessageType   = errors.New("invalid message type")
	ErrVersionMismatch      = errors.New("protocol version mismatch")
)

// HandshakeMessage is sent by both sides on connection establishment
type HandshakeMessage struct {
	ProtocolVersion uint8
	ConnectionID    uint32
	Capabilities    string
}

func (m *HandshakeMessage) Type() uint8 { return MessageTypeHandshake }

func (m *HandshakeMessage) Validate() error {
	if m.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("%w: got %d, expected %d", ErrVersionMismatch, m.ProtocolVersion, ProtocolVersion)
	}
	if m.ConnectionID == 0 {
		return errors.New("connection ID must be non-zero")
	}
	return nil
}

// TileState represents the state of a single tile in Dwarf Fortress
type TileState struct {
	X        int16  // X coordinate
	Y        int16  // Y coordinate
	Z        int16  // Z coordinate (can be negative for underground levels)
	TileType uint16 // DF tiletype enum value
	Flags    uint8  // Packed bit flags
}

// Flag bit masks for TileState.Flags
const (
	FlagHidden     uint8 = 0x01 // Tile in fog of war
	FlagDiscovered uint8 = 0x02 // Tile has been seen before
	FlagDesignated uint8 = 0x04 // Designated for mining
	FlagConstruct  uint8 = 0x08 // Designated for construction
	// Bits 4-7: Reserved for future use
)

// HasFlag checks if a specific flag bit is set
func (t *TileState) HasFlag(flag uint8) bool {
	return (t.Flags & flag) != 0
}

// SetFlag sets a specific flag bit
func (t *TileState) SetFlag(flag uint8) {
	t.Flags |= flag
}

// ClearFlag clears a specific flag bit
func (t *TileState) ClearFlag(flag uint8) {
	t.Flags &^= flag
}

// FullStateMessage contains complete map state
type FullStateMessage struct {
	Width  uint16
	Height uint16
	Depth  uint16
	Tiles  []TileState
}

func (m *FullStateMessage) Type() uint8 { return MessageTypeFullState }

func (m *FullStateMessage) Validate() error {
	expectedCount := int(m.Width) * int(m.Height) * int(m.Depth)
	if len(m.Tiles) != expectedCount {
		return fmt.Errorf("tile count mismatch: got %d, expected %d (W=%d H=%d D=%d)",
			len(m.Tiles), expectedCount, m.Width, m.Height, m.Depth)
	}
	if m.Width == 0 || m.Height == 0 || m.Depth == 0 {
		return errors.New("dimensions must be greater than zero")
	}
	return nil
}

// ResyncRequestMessage requests a full state resynchronization
type ResyncRequestMessage struct {
	Reason uint8
}

func (m *ResyncRequestMessage) Type() uint8 { return MessageTypeResyncRequest }

func (m *ResyncRequestMessage) Validate() error {
	// Reason codes 0x00-0x03 are valid
	if m.Reason > 0x03 {
		return fmt.Errorf("invalid resync reason: 0x%02X", m.Reason)
	}
	return nil
}

// Resync reason constants
const (
	ReasonManual        uint8 = 0x00
	ReasonInconsistency uint8 = 0x01
	ReasonReconnect     uint8 = 0x02
	ReasonPeriodic      uint8 = 0x03
)

// TileUpdateMessage contains incremental tile changes
type TileUpdateMessage struct {
	Count uint32
	Tiles []TileState
}

func (m *TileUpdateMessage) Type() uint8 { return MessageTypeTileUpdate }

func (m *TileUpdateMessage) Validate() error {
	if uint32(len(m.Tiles)) != m.Count {
		return fmt.Errorf("tile count mismatch: Count=%d, len(Tiles)=%d", m.Count, len(m.Tiles))
	}
	if m.Count == 0 {
		return errors.New("tile update must have at least one tile")
	}
	// No maximum - we can handle full map updates if needed
	return nil
}

// HeartbeatMessage is used to detect connection liveness
type HeartbeatMessage struct {
	Timestamp uint64 // Unix timestamp in milliseconds
	Sequence  uint8  // Incrementing sequence number for echo tracking
}

func (m *HeartbeatMessage) Type() uint8 { return MessageTypeHeartbeat }

func (m *HeartbeatMessage) Validate() error {
	// Heartbeat messages are always valid - no constraints
	return nil
}

// DisconnectMessage signals intentional connection closure
type DisconnectMessage struct {
	Reason uint8
}

func (m *DisconnectMessage) Type() uint8 { return MessageTypeDisconnect }

func (m *DisconnectMessage) Validate() error {
	// Reason codes 0x00-0x03 are valid
	if m.Reason > 0x03 {
		return fmt.Errorf("invalid disconnect reason: 0x%02X", m.Reason)
	}
	return nil
}

// Disconnect reason constants
const (
	ReasonNormalShutdown  uint8 = 0x00
	ReasonPluginUnload    uint8 = 0x01
	ReasonServerShutdown  uint8 = 0x02
	ReasonRestart         uint8 = 0x03
)

// ErrorMessage signals protocol errors
type ErrorMessage struct {
	Code    uint16
	Message string
}

func (m *ErrorMessage) Type() uint8 { return MessageTypeError }

func (m *ErrorMessage) Validate() error {
	// All error codes are valid
	// Message can be any string (including empty)
	return nil
}

// Error code constants
const (
	ErrUnknownType      uint16 = 0x0001
	ErrInvalidPayload   uint16 = 0x0002
	ErrVersionMismatch_ uint16 = 0x0003
	ErrInternal         uint16 = 0x0004
)

// EntityInfo represents a single entity (dwarf, enemy, animal)
type EntityInfo struct {
	ID      uint32 // Unit ID from DF
	X       int16  // X coordinate
	Y       int16  // Y coordinate
	Z       int16  // Z coordinate
	Type    uint8  // 1=dwarf, 2=enemy, 3=animal, 4=other
	Subtype uint16 // Race ID for detailed classification
}

// Entity type constants
const (
	EntityTypeDwarf  uint8 = 0x01
	EntityTypeEnemy  uint8 = 0x02
	EntityTypeAnimal uint8 = 0x03
	EntityTypeOther  uint8 = 0x04
)

// EntityUpdateMessage contains entity position updates
type EntityUpdateMessage struct {
	Count    uint32
	Entities []EntityInfo
}

func (m *EntityUpdateMessage) Type() uint8 { return MessageTypeEntityUpdate }

func (m *EntityUpdateMessage) Validate() error {
	if uint32(len(m.Entities)) != m.Count {
		return fmt.Errorf("entity count mismatch: Count=%d, len(Entities)=%d", m.Count, len(m.Entities))
	}
	// Count can be 0 (no entities) - that's valid
	return nil
}

// CommandType constants for command messages
const (
	CommandTypeDig    uint8 = 0x01
	CommandTypeBuild  uint8 = 0x02
	CommandTypeCancel uint8 = 0x03
	CommandTypeChop   uint8 = 0x04
	CommandTypeGather uint8 = 0x05
)

// AckStatus constants for command acknowledgment messages
const (
	AckStatusSuccess uint8 = 0x00
	AckStatusPartial uint8 = 0x01
	AckStatusFailure uint8 = 0x02
)

// BuildType constants for build designation commands
const (
	BuildTypeWall        uint8 = 0x01
	BuildTypeFloor       uint8 = 0x02
	BuildTypeUpStair     uint8 = 0x03
	BuildTypeDownStair   uint8 = 0x04
	BuildTypeUpDownStair uint8 = 0x05
)

// Region represents a 3D bounding box for designations
type Region struct {
	X1, Y1, Z1 int16 // Start coordinates
	X2, Y2, Z2 int16 // End coordinates (inclusive)
}

// BuildDesignation represents a single build command
type BuildDesignation struct {
	X, Y, Z   int16 // Build location
	BuildType uint8 // Type of construction
}

// CommandMessage represents a command from server to DFHack
type CommandMessage struct {
	CommandID   uint32 // Unique command identifier
	CommandType uint8  // Type of command (dig/build/cancel)
	Region      Region // For DIG and CANCEL commands
	Build       BuildDesignation // For BUILD commands
}

func (m *CommandMessage) Type() uint8 { return MessageTypeCommand }

func (m *CommandMessage) Validate() error {
	// Validate CommandType
	if m.CommandType < CommandTypeDig || m.CommandType > CommandTypeCancel {
		return fmt.Errorf("invalid command type: 0x%02X", m.CommandType)
	}

	// Type-specific validation
	switch m.CommandType {
	case CommandTypeDig, CommandTypeCancel:
		// Validate region bounds
		if m.Region.X2 < m.Region.X1 || m.Region.Y2 < m.Region.Y1 || m.Region.Z2 < m.Region.Z1 {
			return errors.New("invalid region: end coordinates must be >= start coordinates")
		}
		// Single Z-level requirement
		if m.Region.Z1 != m.Region.Z2 {
			return errors.New("region must be on single Z-level (Z1 must equal Z2)")
		}
	case CommandTypeBuild:
		// Validate BuildType
		if m.Build.BuildType < BuildTypeWall || m.Build.BuildType > BuildTypeUpDownStair {
			return fmt.Errorf("invalid build type: 0x%02X", m.Build.BuildType)
		}
	}

	return nil
}

// CommandAckMessage represents acknowledgment from DFHack to server
type CommandAckMessage struct {
	CommandID uint32 // Original command ID
	Status    uint8  // Execution status (success/partial/failure)
	ErrorMsg  string // Error description (empty if success)
}

func (m *CommandAckMessage) Type() uint8 { return MessageTypeCommandAck }

func (m *CommandAckMessage) Validate() error {
	// Validate Status
	if m.Status > AckStatusFailure {
		return fmt.Errorf("invalid ack status: 0x%02X", m.Status)
	}
	// ErrorMsg length is validated during serialization
	return nil
}
