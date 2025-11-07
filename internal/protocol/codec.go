package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Codec handles binary encoding/decoding of protocol messages
// All multi-byte integers use big-endian (network byte order)

// Message header structure (6 bytes):
// [0-3]: uint32 Length (big-endian) - total message size including header
// [4]:   uint8 Protocol Version
// [5]:   uint8 Message Type

// SerializeMessage encodes a message to binary format with length-prefixed header
func SerializeMessage(msg Message) ([]byte, error) {
	// Validate before serializing
	if err := msg.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	var buf bytes.Buffer

	// Reserve space for length header (will fill in later)
	if err := binary.Write(&buf, binary.BigEndian, uint32(0)); err != nil {
		return nil, err
	}

	// Write protocol version
	if err := binary.Write(&buf, binary.BigEndian, ProtocolVersion); err != nil {
		return nil, err
	}

	// Write message type
	if err := binary.Write(&buf, binary.BigEndian, msg.Type()); err != nil {
		return nil, err
	}

	// Write type-specific payload
	switch m := msg.(type) {
	case *HandshakeMessage:
		if err := serializeHandshake(&buf, m); err != nil {
			return nil, err
		}
	case *FullStateMessage:
		if err := serializeFullState(&buf, m); err != nil {
			return nil, err
		}
	case *ResyncRequestMessage:
		if err := serializeResyncRequest(&buf, m); err != nil {
			return nil, err
		}
	case *TileUpdateMessage:
		if err := serializeTileUpdate(&buf, m); err != nil {
			return nil, err
		}
	case *HeartbeatMessage:
		if err := serializeHeartbeat(&buf, m); err != nil {
			return nil, err
		}
	case *DisconnectMessage:
		if err := serializeDisconnect(&buf, m); err != nil {
			return nil, err
		}
	case *ErrorMessage:
		if err := serializeError(&buf, m); err != nil {
			return nil, err
		}
	case *EntityUpdateMessage:
		if err := serializeEntityUpdate(&buf, m); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown message type: %T", msg)
	}

	// Get complete message
	data := buf.Bytes()

	// Fill in length header
	binary.BigEndian.PutUint32(data[0:4], uint32(len(data)))

	return data, nil
}

// DeserializeMessage decodes a binary message
func DeserializeMessage(data []byte) (Message, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("%w: got %d bytes, minimum 6", ErrInvalidMessageLength, len(data))
	}

	// Read header
	length := binary.BigEndian.Uint32(data[0:4])
	version := data[4]
	msgType := data[5]

	// Validate length matches actual data
	if uint32(len(data)) != length {
		return nil, fmt.Errorf("%w: header says %d, got %d", ErrInvalidMessageLength, length, len(data))
	}

	// Validate version
	if version != ProtocolVersion {
		return nil, fmt.Errorf("%w: got %d, expected %d", ErrVersionMismatch, version, ProtocolVersion)
	}

	// Payload starts at offset 6
	payload := data[6:]

	// Deserialize based on type
	var msg Message
	var err error

	switch msgType {
	case MessageTypeHandshake:
		msg, err = deserializeHandshake(payload)
	case MessageTypeFullState:
		msg, err = deserializeFullState(payload)
	case MessageTypeResyncRequest:
		msg, err = deserializeResyncRequest(payload)
	case MessageTypeTileUpdate:
		msg, err = deserializeTileUpdate(payload)
	case MessageTypeHeartbeat:
		msg, err = deserializeHeartbeat(payload)
	case MessageTypeDisconnect:
		msg, err = deserializeDisconnect(payload)
	case MessageTypeError:
		msg, err = deserializeError(payload)
	case MessageTypeEntityUpdate:
		msg, err = deserializeEntityUpdate(payload)
	default:
		return nil, fmt.Errorf("%w: 0x%02X", ErrInvalidMessageType, msgType)
	}

	if err != nil {
		return nil, err
	}

	// Validate deserialized message
	if err := msg.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return msg, nil
}

// serializeHandshake encodes HandshakeMessage payload
func serializeHandshake(w io.Writer, msg *HandshakeMessage) error {
	// [1: Version] [4: ConnectionID] [2: Capabilities Length] [N: Capabilities]
	if err := binary.Write(w, binary.BigEndian, msg.ProtocolVersion); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, msg.ConnectionID); err != nil {
		return err
	}
	// Capabilities as length-prefixed string
	capBytes := []byte(msg.Capabilities)
	if err := binary.Write(w, binary.BigEndian, uint16(len(capBytes))); err != nil {
		return err
	}
	if len(capBytes) > 0 {
		if _, err := w.Write(capBytes); err != nil {
			return err
		}
	}
	return nil
}

// deserializeHandshake decodes HandshakeMessage payload
func deserializeHandshake(data []byte) (*HandshakeMessage, error) {
	if len(data) < 7 { // min: 1 + 4 + 2 = 7 bytes
		return nil, errors.New("handshake payload too short")
	}

	msg := &HandshakeMessage{}
	buf := bytes.NewReader(data)

	if err := binary.Read(buf, binary.BigEndian, &msg.ProtocolVersion); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &msg.ConnectionID); err != nil {
		return nil, err
	}

	// Read capabilities string
	var capLen uint16
	if err := binary.Read(buf, binary.BigEndian, &capLen); err != nil {
		return nil, err
	}
	if capLen > 0 {
		capBytes := make([]byte, capLen)
		if _, err := io.ReadFull(buf, capBytes); err != nil {
			return nil, err
		}
		msg.Capabilities = string(capBytes)
	}

	return msg, nil
}

// serializeFullState encodes FullStateMessage payload
func serializeFullState(w io.Writer, msg *FullStateMessage) error {
	// [2: Width] [2: Height] [2: Depth] [N: Tile Array]
	if err := binary.Write(w, binary.BigEndian, msg.Width); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, msg.Height); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, msg.Depth); err != nil {
		return err
	}

	// Serialize tiles in row-major order
	for i := range msg.Tiles {
		if err := serializeTileState(w, &msg.Tiles[i]); err != nil {
			return fmt.Errorf("tile %d: %w", i, err)
		}
	}

	return nil
}

// deserializeFullState decodes FullStateMessage payload
func deserializeFullState(data []byte) (*FullStateMessage, error) {
	if len(data) < 6 {
		return nil, errors.New("full state payload too short")
	}

	msg := &FullStateMessage{}
	buf := bytes.NewReader(data)

	if err := binary.Read(buf, binary.BigEndian, &msg.Width); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &msg.Height); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &msg.Depth); err != nil {
		return nil, err
	}

	// Calculate expected tile count
	tileCount := int(msg.Width) * int(msg.Height) * int(msg.Depth)
	msg.Tiles = make([]TileState, tileCount)

	// Deserialize all tiles
	for i := 0; i < tileCount; i++ {
		tile, err := deserializeTileState(buf)
		if err != nil {
			return nil, fmt.Errorf("tile %d: %w", i, err)
		}
		msg.Tiles[i] = *tile
	}

	return msg, nil
}

// serializeResyncRequest encodes ResyncRequestMessage payload
func serializeResyncRequest(w io.Writer, msg *ResyncRequestMessage) error {
	// [1: Reason]
	return binary.Write(w, binary.BigEndian, msg.Reason)
}

// deserializeResyncRequest decodes ResyncRequestMessage payload
func deserializeResyncRequest(data []byte) (*ResyncRequestMessage, error) {
	if len(data) < 1 {
		return nil, errors.New("resync request payload too short")
	}

	return &ResyncRequestMessage{
		Reason: data[0],
	}, nil
}

// serializeTileUpdate encodes TileUpdateMessage payload
func serializeTileUpdate(w io.Writer, msg *TileUpdateMessage) error {
	// [4: Count] [N: Tile Array]
	if err := binary.Write(w, binary.BigEndian, msg.Count); err != nil {
		return err
	}

	// Serialize tiles
	for i := range msg.Tiles {
		if err := serializeTileState(w, &msg.Tiles[i]); err != nil {
			return fmt.Errorf("tile %d: %w", i, err)
		}
	}

	return nil
}

// deserializeTileUpdate decodes TileUpdateMessage payload
func deserializeTileUpdate(data []byte) (*TileUpdateMessage, error) {
	if len(data) < 4 {
		return nil, errors.New("tile update payload too short")
	}

	msg := &TileUpdateMessage{}
	buf := bytes.NewReader(data)

	if err := binary.Read(buf, binary.BigEndian, &msg.Count); err != nil {
		return nil, err
	}

	// Deserialize tiles
	msg.Tiles = make([]TileState, msg.Count)
	for i := uint32(0); i < msg.Count; i++ {
		tile, err := deserializeTileState(buf)
		if err != nil {
			return nil, fmt.Errorf("tile %d: %w", i, err)
		}
		msg.Tiles[i] = *tile
	}

	return msg, nil
}

// serializeTileState encodes a single TileState (9 bytes)
// [2: X] [2: Y] [2: Z] [2: TileType] [1: Flags]
func serializeTileState(w io.Writer, tile *TileState) error {
	if err := binary.Write(w, binary.BigEndian, tile.X); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, tile.Y); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, tile.Z); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, tile.TileType); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, tile.Flags); err != nil {
		return err
	}
	return nil
}

// deserializeTileState decodes a single TileState (9 bytes)
func deserializeTileState(r io.Reader) (*TileState, error) {
	tile := &TileState{}

	if err := binary.Read(r, binary.BigEndian, &tile.X); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &tile.Y); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &tile.Z); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &tile.TileType); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &tile.Flags); err != nil {
		return nil, err
	}

	return tile, nil
}

// Implement Serialize and Deserialize methods for message types

func (m *HandshakeMessage) Serialize() ([]byte, error) {
	return SerializeMessage(m)
}

func (m *HandshakeMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	h, ok := msg.(*HandshakeMessage)
	if !ok {
		return fmt.Errorf("expected HandshakeMessage, got %T", msg)
	}
	*m = *h
	return nil
}

func (m *FullStateMessage) Serialize() ([]byte, error) {
	return SerializeMessage(m)
}

func (m *FullStateMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	f, ok := msg.(*FullStateMessage)
	if !ok {
		return fmt.Errorf("expected FullStateMessage, got %T", msg)
	}
	*m = *f
	return nil
}

func (m *ResyncRequestMessage) Serialize() ([]byte, error) {
	return SerializeMessage(m)
}

func (m *ResyncRequestMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	r, ok := msg.(*ResyncRequestMessage)
	if !ok {
		return fmt.Errorf("expected ResyncRequestMessage, got %T", msg)
	}
	*m = *r
	return nil
}

func (m *TileUpdateMessage) Serialize() ([]byte, error) {
	return SerializeMessage(m)
}

func (m *TileUpdateMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	t, ok := msg.(*TileUpdateMessage)
	if !ok {
		return fmt.Errorf("expected TileUpdateMessage, got %T", msg)
	}
	*m = *t
	return nil
}

// serializeHeartbeat encodes HeartbeatMessage payload
func serializeHeartbeat(w io.Writer, msg *HeartbeatMessage) error {
	// [8: Timestamp] [1: Sequence]
	if err := binary.Write(w, binary.BigEndian, msg.Timestamp); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, msg.Sequence); err != nil {
		return err
	}
	return nil
}

// deserializeHeartbeat decodes HeartbeatMessage payload
func deserializeHeartbeat(payload []byte) (Message, error) {
	if len(payload) < 9 {
		return nil, fmt.Errorf("heartbeat payload too short: got %d, expected 9", len(payload))
	}

	msg := &HeartbeatMessage{
		Timestamp: binary.BigEndian.Uint64(payload[0:8]),
		Sequence:  payload[8],
	}

	return msg, nil
}

func (m *HeartbeatMessage) Serialize() ([]byte, error) {
	return SerializeMessage(m)
}

func (m *HeartbeatMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	h, ok := msg.(*HeartbeatMessage)
	if !ok {
		return fmt.Errorf("expected HeartbeatMessage, got %T", msg)
	}
	*m = *h
	return nil
}

// serializeDisconnect encodes DisconnectMessage payload
func serializeDisconnect(w io.Writer, msg *DisconnectMessage) error {
	// [1: Reason]
	if err := binary.Write(w, binary.BigEndian, msg.Reason); err != nil {
		return err
	}
	return nil
}

// deserializeDisconnect decodes DisconnectMessage payload
func deserializeDisconnect(payload []byte) (Message, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("disconnect payload too short: got %d, expected 1", len(payload))
	}

	msg := &DisconnectMessage{
		Reason: payload[0],
	}

	return msg, nil
}

func (m *DisconnectMessage) Serialize() ([]byte, error) {
	return SerializeMessage(m)
}

func (m *DisconnectMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	d, ok := msg.(*DisconnectMessage)
	if !ok {
		return fmt.Errorf("expected DisconnectMessage, got %T", msg)
	}
	*m = *d
	return nil
}

// serializeError encodes ErrorMessage payload
func serializeError(w io.Writer, msg *ErrorMessage) error {
	// [2: Code] [2: Message Length] [N: Message UTF-8]
	if err := binary.Write(w, binary.BigEndian, msg.Code); err != nil {
		return err
	}
	msgBytes := []byte(msg.Message)
	if err := binary.Write(w, binary.BigEndian, uint16(len(msgBytes))); err != nil {
		return err
	}
	if len(msgBytes) > 0 {
		if _, err := w.Write(msgBytes); err != nil {
			return err
		}
	}
	return nil
}

// deserializeError decodes ErrorMessage payload
func deserializeError(payload []byte) (Message, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("error payload too short: got %d, expected at least 4", len(payload))
	}

	code := binary.BigEndian.Uint16(payload[0:2])
	msgLen := binary.BigEndian.Uint16(payload[2:4])

	if len(payload) < 4+int(msgLen) {
		return nil, fmt.Errorf("error payload truncated: got %d, expected %d", len(payload), 4+msgLen)
	}

	msg := &ErrorMessage{
		Code:    code,
		Message: string(payload[4 : 4+msgLen]),
	}

	return msg, nil
}

func (m *ErrorMessage) Serialize() ([]byte, error) {
	return SerializeMessage(m)
}

func (m *ErrorMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	e, ok := msg.(*ErrorMessage)
	if !ok {
		return fmt.Errorf("expected ErrorMessage, got %T", msg)
	}
	*m = *e
	return nil
}

// serializeEntityUpdate encodes EntityUpdateMessage payload
func serializeEntityUpdate(w io.Writer, msg *EntityUpdateMessage) error {
	// [4: Count] [N: Entity Array]
	if err := binary.Write(w, binary.BigEndian, msg.Count); err != nil {
		return err
	}

	// Serialize entities
	for i := range msg.Entities {
		if err := serializeEntityInfo(w, &msg.Entities[i]); err != nil {
			return fmt.Errorf("entity %d: %w", i, err)
		}
	}

	return nil
}

// deserializeEntityUpdate decodes EntityUpdateMessage payload
func deserializeEntityUpdate(data []byte) (*EntityUpdateMessage, error) {
	if len(data) < 4 {
		return nil, errors.New("entity update payload too short")
	}

	msg := &EntityUpdateMessage{}
	buf := bytes.NewReader(data)

	if err := binary.Read(buf, binary.BigEndian, &msg.Count); err != nil {
		return nil, err
	}

	// Deserialize entities
	msg.Entities = make([]EntityInfo, msg.Count)
	for i := uint32(0); i < msg.Count; i++ {
		entity, err := deserializeEntityInfo(buf)
		if err != nil {
			return nil, fmt.Errorf("entity %d: %w", i, err)
		}
		msg.Entities[i] = *entity
	}

	return msg, nil
}

// serializeEntityInfo encodes a single EntityInfo (13 bytes)
// [4: ID] [2: X] [2: Y] [2: Z] [1: Type] [2: Subtype]
func serializeEntityInfo(w io.Writer, entity *EntityInfo) error {
	if err := binary.Write(w, binary.BigEndian, entity.ID); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, entity.X); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, entity.Y); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, entity.Z); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, entity.Type); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, entity.Subtype); err != nil {
		return err
	}
	return nil
}

// deserializeEntityInfo decodes a single EntityInfo (13 bytes)
func deserializeEntityInfo(r io.Reader) (*EntityInfo, error) {
	entity := &EntityInfo{}

	if err := binary.Read(r, binary.BigEndian, &entity.ID); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &entity.X); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &entity.Y); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &entity.Z); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &entity.Type); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &entity.Subtype); err != nil {
		return nil, err
	}

	return entity, nil
}

func (m *EntityUpdateMessage) Serialize() ([]byte, error) {
	return SerializeMessage(m)
}

func (m *EntityUpdateMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	e, ok := msg.(*EntityUpdateMessage)
	if !ok {
		return fmt.Errorf("expected EntityUpdateMessage, got %T", msg)
	}
	*m = *e
	return nil
}
