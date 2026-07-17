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
	case *CommandMessage:
		if err := serializeCommand(&buf, m); err != nil {
			return nil, err
		}
	case *CommandAckMessage:
		if err := serializeCommandAck(&buf, m); err != nil {
			return nil, err
		}
	case *QueryMessage:
		if err := serializeQuery(&buf, m); err != nil {
			return nil, err
		}
	case *QueryResponseMessage:
		if err := serializeQueryResponse(&buf, m); err != nil {
			return nil, err
		}
	case *AnnouncementUpdateMessage:
		if err := serializeAnnouncementUpdate(&buf, m); err != nil {
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
	case MessageTypeCommand:
		msg, err = deserializeCommand(payload)
	case MessageTypeCommandAck:
		msg, err = deserializeCommandAck(payload)
	case MessageTypeQuery:
		msg, err = deserializeQuery(payload)
	case MessageTypeQueryResponse:
		msg, err = deserializeQueryResponse(payload)
	case MessageTypeAnnouncementUpdate:
		msg, err = deserializeAnnouncementUpdate(payload)
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
	// [4: Count] [N: Entity Array] [1: HasFortInfo] [N: FortInfo if present]
	if err := binary.Write(w, binary.BigEndian, msg.Count); err != nil {
		return err
	}

	// Serialize entities
	for i := range msg.Entities {
		if err := serializeEntityInfo(w, &msg.Entities[i]); err != nil {
			return fmt.Errorf("entity %d: %w", i, err)
		}
	}

	// Serialize optional FortInfo
	hasFortInfo := uint8(0)
	if msg.FortInfo != nil {
		hasFortInfo = 1
	}
	if err := binary.Write(w, binary.BigEndian, hasFortInfo); err != nil {
		return err
	}

	if msg.FortInfo != nil {
		// [4: DaysElapsed] [8: CreatedWealth] [1: Season] [4: Year]
		if err := binary.Write(w, binary.BigEndian, msg.FortInfo.DaysElapsed); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.FortInfo.CreatedWealth); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.FortInfo.Season); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.FortInfo.Year); err != nil {
			return err
		}
	}

	// Zones block -- additive, appended after FortInfo. Byte layout must
	// exactly match dfhack-plugin/entities.cpp's serialize_entity_update.
	hasZones := uint8(0)
	if len(msg.Zones) > 0 {
		hasZones = 1
	}
	if err := binary.Write(w, binary.BigEndian, hasZones); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(msg.Zones))); err != nil {
		return err
	}
	for _, z := range msg.Zones {
		if err := binary.Write(w, binary.BigEndian, z.ZoneID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, z.ZoneType); err != nil {
			return err
		}
		for _, v := range []int16{z.X1, z.Y1, z.X2, z.Y2, z.Z1} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, z.OwnerUnitID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, uint16(len(z.AssignedUnits))); err != nil {
			return err
		}
		for _, uid := range z.AssignedUnits {
			if err := binary.Write(w, binary.BigEndian, uid); err != nil {
				return err
			}
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

	// Check for optional FortInfo
	var hasFortInfo uint8
	if err := binary.Read(buf, binary.BigEndian, &hasFortInfo); err == nil && hasFortInfo == 1 {
		// Read FortInfo fields
		fortInfo := &FortInfo{}
		if err := binary.Read(buf, binary.BigEndian, &fortInfo.DaysElapsed); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &fortInfo.CreatedWealth); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &fortInfo.Season); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &fortInfo.Year); err != nil {
			return nil, err
		}
		msg.FortInfo = fortInfo
	}
	// If no hasFortInfo byte or hasFortInfo==0, FortInfo remains nil (backward compatible)

	// Zones block (additive — absent on an old peer, decoded as empty).
	var hasZones uint8
	if err := binary.Read(buf, binary.BigEndian, &hasZones); err == nil && hasZones == 1 {
		var zoneCount uint32
		if err := binary.Read(buf, binary.BigEndian, &zoneCount); err != nil {
			return nil, err
		}
		msg.Zones = make([]ZoneData, zoneCount)
		for i := uint32(0); i < zoneCount; i++ {
			z := &msg.Zones[i]
			if err := binary.Read(buf, binary.BigEndian, &z.ZoneID); err != nil {
				return nil, err
			}
			if err := binary.Read(buf, binary.BigEndian, &z.ZoneType); err != nil {
				return nil, err
			}
			var x1, y1, x2, y2, zLevel int16
			for _, p := range []*int16{&x1, &y1, &x2, &y2, &zLevel} {
				if err := binary.Read(buf, binary.BigEndian, p); err != nil {
					return nil, err
				}
			}
			z.X1, z.Y1, z.X2, z.Y2 = x1, y1, x2, y2
			z.Z1, z.Z2 = zLevel, zLevel
			if err := binary.Read(buf, binary.BigEndian, &z.OwnerUnitID); err != nil {
				return nil, err
			}
			var assignedCount uint16
			if err := binary.Read(buf, binary.BigEndian, &assignedCount); err != nil {
				return nil, err
			}
			if assignedCount > 0 {
				z.AssignedUnits = make([]int32, assignedCount)
				for j := uint16(0); j < assignedCount; j++ {
					if err := binary.Read(buf, binary.BigEndian, &z.AssignedUnits[j]); err != nil {
						return nil, err
					}
				}
			}
		}
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

// serializeCommand encodes CommandMessage payload
func serializeCommand(w io.Writer, msg *CommandMessage) error {
	// [4: CommandID] [1: CommandType] [N: Payload]
	if err := binary.Write(w, binary.BigEndian, msg.CommandID); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, msg.CommandType); err != nil {
		return err
	}

	// Type-specific payload
	switch msg.CommandType {
	case CommandTypeDig, CommandTypeCancel:
		// [1: DigType] [2: X1] [2: Y1] [2: Z1] [2: X2] [2: Y2] [2: Z2]
		if err := binary.Write(w, binary.BigEndian, msg.DigType); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Region.X1); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Region.Y1); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Region.Z1); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Region.X2); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Region.Y2); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Region.Z2); err != nil {
			return err
		}
	case CommandTypeBuild:
		// [2: X] [2: Y] [2: Z] [1: BuildType] [1: MaterialClass]
		// The material byte is ALWAYS appended by this encoder; the plugin
		// treats a payload without it as MaterialClassAny (backward compat).
		if err := binary.Write(w, binary.BigEndian, msg.Build.X); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Build.Y); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Build.Z); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Build.BuildType); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Build.Material); err != nil {
			return err
		}
	case CommandTypeChop, CommandTypeGather:
		// [2: X1] [2: Y1] [2: Z1] [2: X2] [2: Y2] [2: Z2]
		for _, v := range []int16{msg.Region.X1, msg.Region.Y1, msg.Region.Z1, msg.Region.X2, msg.Region.Y2, msg.Region.Z2} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypeZone:
		// [1: ZoneType] [2: X1] [2: Y1] [2: Z] [2: X2] [2: Y2]
		if err := binary.Write(w, binary.BigEndian, msg.Zone.ZoneType); err != nil {
			return err
		}
		for _, v := range []int16{msg.Zone.X1, msg.Zone.Y1, msg.Zone.Z, msg.Zone.X2, msg.Zone.Y2} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypeAssignZone:
		// [2: X] [2: Y] [2: Z] [4: UnitID]
		for _, v := range []int16{msg.AssignZone.X, msg.AssignZone.Y, msg.AssignZone.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, msg.AssignZone.UnitID); err != nil {
			return err
		}
	case CommandTypeUnassignZone:
		// [2: X] [2: Y] [2: Z] [4: UnitID]
		for _, v := range []int16{msg.UnassignZone.X, msg.UnassignZone.Y, msg.UnassignZone.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, msg.UnassignZone.UnitID); err != nil {
			return err
		}
	case CommandTypeCreateLocation:
		// [2: X] [2: Y] [2: Z] [1: LocationType] [2: ProfessionLen] [N: Profession]
		for _, v := range []int16{msg.CreateLocation.X, msg.CreateLocation.Y, msg.CreateLocation.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, msg.CreateLocation.LocationType); err != nil {
			return err
		}
		profBytes := []byte(msg.CreateLocation.Profession)
		if err := binary.Write(w, binary.BigEndian, uint16(len(profBytes))); err != nil {
			return err
		}
		if _, err := w.Write(profBytes); err != nil {
			return err
		}
	case CommandTypeAssignLodging:
		// [2: TavernX] [2: TavernY] [2: TavernZ] [2: BedroomX] [2: BedroomY] [2: BedroomZ]
		for _, v := range []int16{
			msg.AssignLodging.TavernX, msg.AssignLodging.TavernY, msg.AssignLodging.TavernZ,
			msg.AssignLodging.BedroomX, msg.AssignLodging.BedroomY, msg.AssignLodging.BedroomZ,
		} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypeUnassignLodging:
		// [2: BedroomX] [2: BedroomY] [2: BedroomZ]
		for _, v := range []int16{msg.UnassignLodging.BedroomX, msg.UnassignLodging.BedroomY, msg.UnassignLodging.BedroomZ} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypeUnsuspend:
		// [2: X] [2: Y] [2: Z]
		for _, v := range []int16{msg.Unsuspend.X, msg.Unsuspend.Y, msg.Unsuspend.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypeWorkOrder:
		// [1: OrderType] [2: Quantity]
		if err := binary.Write(w, binary.BigEndian, msg.Order.OrderType); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Order.Quantity); err != nil {
			return err
		}
	case CommandTypeStockpile:
		// [2: X1] [2: Y1] [2: Z] [2: X2] [2: Y2] [4: GroupMask]
		for _, v := range []int16{msg.Stockpile.X1, msg.Stockpile.Y1, msg.Stockpile.Z, msg.Stockpile.X2, msg.Stockpile.Y2} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, msg.Stockpile.GroupMask); err != nil {
			return err
		}
	case CommandTypeSmooth:
		// [1: SmoothType] [2: X1] [2: Y1] [2: Z] [2: X2] [2: Y2]
		if err := binary.Write(w, binary.BigEndian, msg.Smooth.SmoothType); err != nil {
			return err
		}
		for _, v := range []int16{msg.Smooth.X1, msg.Smooth.Y1, msg.Smooth.Z, msg.Smooth.X2, msg.Smooth.Y2} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypePause:
		// [1: Mode] [4: Ticks]
		if err := binary.Write(w, binary.BigEndian, msg.Pause.Mode); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Pause.Ticks); err != nil {
			return err
		}
	case CommandTypeRemoveBuilding:
		// [2: X] [2: Y] [2: Z]
		for _, v := range []int16{msg.Remove.X, msg.Remove.Y, msg.Remove.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypeRemoveZone:
		// [2: X] [2: Y] [2: Z]
		for _, v := range []int16{msg.RemoveZone.X, msg.RemoveZone.Y, msg.RemoveZone.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypeBuildFarmPlot:
		// [2: X1] [2: Y1] [2: Z] [2: X2] [2: Y2] — byte-identical to
		// STOCKPILE minus the trailing GroupMask.
		for _, v := range []int16{msg.FarmPlot.X1, msg.FarmPlot.Y1, msg.FarmPlot.Z, msg.FarmPlot.X2, msg.FarmPlot.Y2} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypeSetFarmCrop:
		// [2: X] [2: Y] [2: Z] [1: Season] [2: NameLen] [N: CropName]
		for _, v := range []int16{msg.SetFarmCrop.X, msg.SetFarmCrop.Y, msg.SetFarmCrop.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, msg.SetFarmCrop.Season); err != nil {
			return err
		}
		cropBytes := []byte(msg.SetFarmCrop.CropName)
		if len(cropBytes) > 65535 {
			return errors.New("set_farm_crop crop name too long (max 65535 bytes)")
		}
		if err := binary.Write(w, binary.BigEndian, uint16(len(cropBytes))); err != nil {
			return err
		}
		if _, err := w.Write(cropBytes); err != nil {
			return err
		}
	case CommandTypeDesignateBurrow:
		// [2: NameLen] [N: Name] [2: X1] [2: Y1] [2: Z1] [2: X2] [2: Y2] [2: Z2]
		// Name-first, matching CommandTypeBlueprint's shape (case 0x07).
		nameBytes := []byte(msg.DesignateBurrow.Name)
		if len(nameBytes) > 255 {
			return errors.New("designate_burrow name too long (max 255 bytes)")
		}
		if err := binary.Write(w, binary.BigEndian, uint16(len(nameBytes))); err != nil {
			return err
		}
		if _, err := w.Write(nameBytes); err != nil {
			return err
		}
		for _, v := range []int16{
			msg.DesignateBurrow.X1, msg.DesignateBurrow.Y1, msg.DesignateBurrow.Z1,
			msg.DesignateBurrow.X2, msg.DesignateBurrow.Y2, msg.DesignateBurrow.Z2,
		} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypeRemoveBurrow:
		// [2: NameLen] [N: Name]
		nameBytes := []byte(msg.RemoveBurrow.Name)
		if len(nameBytes) > 255 {
			return errors.New("remove_burrow name too long (max 255 bytes)")
		}
		if err := binary.Write(w, binary.BigEndian, uint16(len(nameBytes))); err != nil {
			return err
		}
		if _, err := w.Write(nameBytes); err != nil {
			return err
		}
	case CommandTypeAssignBurrow:
		// [2: NameLen] [N: Name] [1: Assign] [1: AllCitizens] [4: UnitID]
		nameBytes := []byte(msg.AssignBurrow.Name)
		if len(nameBytes) > 255 {
			return errors.New("assign_burrow name too long (max 255 bytes)")
		}
		if err := binary.Write(w, binary.BigEndian, uint16(len(nameBytes))); err != nil {
			return err
		}
		if _, err := w.Write(nameBytes); err != nil {
			return err
		}
		assignByte, allCitizensByte := uint8(0), uint8(0)
		if msg.AssignBurrow.Assign {
			assignByte = 1
		}
		if msg.AssignBurrow.AllCitizens {
			allCitizensByte = 1
		}
		if err := binary.Write(w, binary.BigEndian, assignByte); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, allCitizensByte); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.AssignBurrow.UnitID); err != nil {
			return err
		}
	case CommandTypeSetAlert:
		// [2: NameLen] [N: Name] [1: Active]
		nameBytes := []byte(msg.SetAlert.Name)
		if len(nameBytes) > 255 {
			return errors.New("set_alert name too long (max 255 bytes)")
		}
		if err := binary.Write(w, binary.BigEndian, uint16(len(nameBytes))); err != nil {
			return err
		}
		if _, err := w.Write(nameBytes); err != nil {
			return err
		}
		activeByte := uint8(0)
		if msg.SetAlert.Active {
			activeByte = 1
		}
		if err := binary.Write(w, binary.BigEndian, activeByte); err != nil {
			return err
		}
	case CommandTypeQueueJob:
		// [2: X] [2: Y] [2: Z] [1: OrderType] [2: NameLen][N: Name]
		// The name tail is present ONLY when OrderType == OrderTypeByName or
		// OrderTypeCustomReaction — mirrors CommandTypeBlueprint's
		// length-prefixed name below. Existing byte-vocabulary callers
		// (OrderType 0x01-0x0C) produce the exact same 12-byte payload as
		// before this change.
		for _, v := range []int16{msg.QueueJob.X, msg.QueueJob.Y, msg.QueueJob.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, msg.QueueJob.OrderType); err != nil {
			return err
		}
		var queueJobTrailingName string
		switch msg.QueueJob.OrderType {
		case OrderTypeByName:
			queueJobTrailingName = msg.QueueJob.JobTypeName
		case OrderTypeCustomReaction:
			queueJobTrailingName = msg.QueueJob.ReactionCode
		}
		if msg.QueueJob.OrderType == OrderTypeByName || msg.QueueJob.OrderType == OrderTypeCustomReaction {
			nameBytes := []byte(queueJobTrailingName)
			if len(nameBytes) > 255 {
				return errors.New("queue_job job type/reaction name too long (max 255 bytes)")
			}
			if err := binary.Write(w, binary.BigEndian, uint16(len(nameBytes))); err != nil {
				return err
			}
			if _, err := w.Write(nameBytes); err != nil {
				return err
			}
		}
	case CommandTypeSetLabor:
		// [4: UnitID] [1: LaborID] [1: Enable]
		if err := binary.Write(w, binary.BigEndian, msg.SetLabor.UnitID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.SetLabor.LaborID); err != nil {
			return err
		}
		enable := uint8(0)
		if msg.SetLabor.Enable {
			enable = 1
		}
		if err := binary.Write(w, binary.BigEndian, enable); err != nil {
			return err
		}
	case CommandTypeBlueprint:
		// [2: NameLen] [N: Name] [2: OriginX] [2: OriginY] [2: OriginZ]
		nameBytes := []byte(msg.BlueprintName)
		if len(nameBytes) > 255 {
			return errors.New("blueprint name too long (max 255 bytes)")
		}
		if err := binary.Write(w, binary.BigEndian, uint16(len(nameBytes))); err != nil {
			return err
		}
		if _, err := w.Write(nameBytes); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.OriginX); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.OriginY); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.OriginZ); err != nil {
			return err
		}
	case CommandTypeLinkBuilding:
		// [2: LeverX] [2: LeverY] [2: LeverZ] [2: TargetX] [2: TargetY] [2: TargetZ]
		for _, v := range []int16{
			msg.LinkBuilding.LeverX, msg.LinkBuilding.LeverY, msg.LinkBuilding.LeverZ,
			msg.LinkBuilding.TargetX, msg.LinkBuilding.TargetY, msg.LinkBuilding.TargetZ,
		} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypePullLever:
		// [2: X] [2: Y] [2: Z]
		for _, v := range []int16{msg.PullLever.X, msg.PullLever.Y, msg.PullLever.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypeBuildBridge:
		// [2: X1] [2: Y1] [2: Z] [2: X2] [2: Y2] [1: Direction] — byte-identical
		// to BUILD_FARM_PLOT plus a trailing direction byte.
		for _, v := range []int16{msg.BuildBridge.X1, msg.BuildBridge.Y1, msg.BuildBridge.Z, msg.BuildBridge.X2, msg.BuildBridge.Y2} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, msg.BuildBridge.Direction); err != nil {
			return err
		}
	}

	return nil
}

// deserializeCommand decodes CommandMessage payload
func deserializeCommand(data []byte) (*CommandMessage, error) {
	if len(data) < 5 { // Minimum: 4 (CommandID) + 1 (CommandType)
		return nil, errors.New("command payload too short")
	}

	msg := &CommandMessage{}
	buf := bytes.NewReader(data)

	if err := binary.Read(buf, binary.BigEndian, &msg.CommandID); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &msg.CommandType); err != nil {
		return nil, err
	}

	// Type-specific payload
	switch msg.CommandType {
	case CommandTypeDig, CommandTypeCancel:
		// Read dig type + region (1 + 12 bytes)
		if err := binary.Read(buf, binary.BigEndian, &msg.DigType); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Region.X1); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Region.Y1); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Region.Z1); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Region.X2); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Region.Y2); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Region.Z2); err != nil {
			return nil, err
		}
	case CommandTypeBuild:
		// Read build designation (7 bytes + optional trailing material byte)
		if err := binary.Read(buf, binary.BigEndian, &msg.Build.X); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Build.Y); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Build.Z); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Build.BuildType); err != nil {
			return nil, err
		}
		// Optional material class byte: a payload without it (pre-material
		// peer) decodes as MaterialClassAny — mirrors the plugin's rule.
		if err := binary.Read(buf, binary.BigEndian, &msg.Build.Material); err != nil {
			if err != io.EOF {
				return nil, err
			}
			msg.Build.Material = MaterialClassAny
		}
	case CommandTypeChop, CommandTypeGather:
		for _, p := range []*int16{&msg.Region.X1, &msg.Region.Y1, &msg.Region.Z1, &msg.Region.X2, &msg.Region.Y2, &msg.Region.Z2} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypeZone:
		if err := binary.Read(buf, binary.BigEndian, &msg.Zone.ZoneType); err != nil {
			return nil, err
		}
		for _, p := range []*int16{&msg.Zone.X1, &msg.Zone.Y1, &msg.Zone.Z, &msg.Zone.X2, &msg.Zone.Y2} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypeAssignZone:
		for _, p := range []*int16{&msg.AssignZone.X, &msg.AssignZone.Y, &msg.AssignZone.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.AssignZone.UnitID); err != nil {
			return nil, err
		}
	case CommandTypeUnassignZone:
		for _, p := range []*int16{&msg.UnassignZone.X, &msg.UnassignZone.Y, &msg.UnassignZone.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.UnassignZone.UnitID); err != nil {
			return nil, err
		}
	case CommandTypeCreateLocation:
		for _, p := range []*int16{&msg.CreateLocation.X, &msg.CreateLocation.Y, &msg.CreateLocation.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.CreateLocation.LocationType); err != nil {
			return nil, err
		}
		var profLen uint16
		if err := binary.Read(buf, binary.BigEndian, &profLen); err != nil {
			return nil, err
		}
		if profLen > 0 {
			profBytes := make([]byte, profLen)
			if _, err := io.ReadFull(buf, profBytes); err != nil {
				return nil, err
			}
			msg.CreateLocation.Profession = string(profBytes)
		}
	case CommandTypeAssignLodging:
		for _, p := range []*int16{
			&msg.AssignLodging.TavernX, &msg.AssignLodging.TavernY, &msg.AssignLodging.TavernZ,
			&msg.AssignLodging.BedroomX, &msg.AssignLodging.BedroomY, &msg.AssignLodging.BedroomZ,
		} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypeUnassignLodging:
		for _, p := range []*int16{&msg.UnassignLodging.BedroomX, &msg.UnassignLodging.BedroomY, &msg.UnassignLodging.BedroomZ} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypeUnsuspend:
		for _, p := range []*int16{&msg.Unsuspend.X, &msg.Unsuspend.Y, &msg.Unsuspend.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypeWorkOrder:
		if err := binary.Read(buf, binary.BigEndian, &msg.Order.OrderType); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Order.Quantity); err != nil {
			return nil, err
		}
	case CommandTypeStockpile:
		for _, p := range []*int16{&msg.Stockpile.X1, &msg.Stockpile.Y1, &msg.Stockpile.Z, &msg.Stockpile.X2, &msg.Stockpile.Y2} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Stockpile.GroupMask); err != nil {
			return nil, err
		}
	case CommandTypeSmooth:
		if err := binary.Read(buf, binary.BigEndian, &msg.Smooth.SmoothType); err != nil {
			return nil, err
		}
		for _, p := range []*int16{&msg.Smooth.X1, &msg.Smooth.Y1, &msg.Smooth.Z, &msg.Smooth.X2, &msg.Smooth.Y2} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypePause:
		if err := binary.Read(buf, binary.BigEndian, &msg.Pause.Mode); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Pause.Ticks); err != nil {
			return nil, err
		}
	case CommandTypeRemoveBuilding:
		for _, p := range []*int16{&msg.Remove.X, &msg.Remove.Y, &msg.Remove.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypeRemoveZone:
		for _, p := range []*int16{&msg.RemoveZone.X, &msg.RemoveZone.Y, &msg.RemoveZone.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypeBuildFarmPlot:
		for _, p := range []*int16{&msg.FarmPlot.X1, &msg.FarmPlot.Y1, &msg.FarmPlot.Z, &msg.FarmPlot.X2, &msg.FarmPlot.Y2} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypeSetFarmCrop:
		for _, p := range []*int16{&msg.SetFarmCrop.X, &msg.SetFarmCrop.Y, &msg.SetFarmCrop.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.SetFarmCrop.Season); err != nil {
			return nil, err
		}
		var cropLen uint16
		if err := binary.Read(buf, binary.BigEndian, &cropLen); err != nil {
			return nil, err
		}
		cropBytes := make([]byte, cropLen)
		if _, err := io.ReadFull(buf, cropBytes); err != nil {
			return nil, err
		}
		msg.SetFarmCrop.CropName = string(cropBytes)
	case CommandTypeDesignateBurrow:
		var nameLen uint16
		if err := binary.Read(buf, binary.BigEndian, &nameLen); err != nil {
			return nil, err
		}
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(buf, nameBytes); err != nil {
			return nil, err
		}
		msg.DesignateBurrow.Name = string(nameBytes)
		for _, p := range []*int16{
			&msg.DesignateBurrow.X1, &msg.DesignateBurrow.Y1, &msg.DesignateBurrow.Z1,
			&msg.DesignateBurrow.X2, &msg.DesignateBurrow.Y2, &msg.DesignateBurrow.Z2,
		} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypeRemoveBurrow:
		var nameLen uint16
		if err := binary.Read(buf, binary.BigEndian, &nameLen); err != nil {
			return nil, err
		}
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(buf, nameBytes); err != nil {
			return nil, err
		}
		msg.RemoveBurrow.Name = string(nameBytes)
	case CommandTypeAssignBurrow:
		var nameLen uint16
		if err := binary.Read(buf, binary.BigEndian, &nameLen); err != nil {
			return nil, err
		}
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(buf, nameBytes); err != nil {
			return nil, err
		}
		msg.AssignBurrow.Name = string(nameBytes)
		var assignByte, allCitizensByte uint8
		if err := binary.Read(buf, binary.BigEndian, &assignByte); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &allCitizensByte); err != nil {
			return nil, err
		}
		msg.AssignBurrow.Assign = assignByte != 0
		msg.AssignBurrow.AllCitizens = allCitizensByte != 0
		if err := binary.Read(buf, binary.BigEndian, &msg.AssignBurrow.UnitID); err != nil {
			return nil, err
		}
	case CommandTypeSetAlert:
		var nameLen uint16
		if err := binary.Read(buf, binary.BigEndian, &nameLen); err != nil {
			return nil, err
		}
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(buf, nameBytes); err != nil {
			return nil, err
		}
		msg.SetAlert.Name = string(nameBytes)
		var activeByte uint8
		if err := binary.Read(buf, binary.BigEndian, &activeByte); err != nil {
			return nil, err
		}
		msg.SetAlert.Active = activeByte != 0
	case CommandTypeQueueJob:
		for _, p := range []*int16{&msg.QueueJob.X, &msg.QueueJob.Y, &msg.QueueJob.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.QueueJob.OrderType); err != nil {
			return nil, err
		}
		// Name tail present ONLY when OrderType == OrderTypeByName or
		// OrderTypeCustomReaction — a pre-existing byte-vocabulary payload
		// (0x01-0x0C) ends here, same as before this change.
		if msg.QueueJob.OrderType == OrderTypeByName || msg.QueueJob.OrderType == OrderTypeCustomReaction {
			var nameLen uint16
			if err := binary.Read(buf, binary.BigEndian, &nameLen); err != nil {
				return nil, err
			}
			nameBytes := make([]byte, nameLen)
			if _, err := io.ReadFull(buf, nameBytes); err != nil {
				return nil, err
			}
			switch msg.QueueJob.OrderType {
			case OrderTypeByName:
				msg.QueueJob.JobTypeName = string(nameBytes)
			case OrderTypeCustomReaction:
				msg.QueueJob.ReactionCode = string(nameBytes)
			}
		}
	case CommandTypeSetLabor:
		if err := binary.Read(buf, binary.BigEndian, &msg.SetLabor.UnitID); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.SetLabor.LaborID); err != nil {
			return nil, err
		}
		var enable uint8
		if err := binary.Read(buf, binary.BigEndian, &enable); err != nil {
			return nil, err
		}
		msg.SetLabor.Enable = enable != 0
	case CommandTypeBlueprint:
		// Read blueprint name (2 bytes length + N bytes name)
		var nameLen uint16
		if err := binary.Read(buf, binary.BigEndian, &nameLen); err != nil {
			return nil, err
		}
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(buf, nameBytes); err != nil {
			return nil, err
		}
		msg.BlueprintName = string(nameBytes)

		// Read origin coordinates (3 x 2 bytes)
		if err := binary.Read(buf, binary.BigEndian, &msg.OriginX); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.OriginY); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.OriginZ); err != nil {
			return nil, err
		}
	case CommandTypeLinkBuilding:
		for _, p := range []*int16{
			&msg.LinkBuilding.LeverX, &msg.LinkBuilding.LeverY, &msg.LinkBuilding.LeverZ,
			&msg.LinkBuilding.TargetX, &msg.LinkBuilding.TargetY, &msg.LinkBuilding.TargetZ,
		} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypePullLever:
		for _, p := range []*int16{&msg.PullLever.X, &msg.PullLever.Y, &msg.PullLever.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypeBuildBridge:
		for _, p := range []*int16{&msg.BuildBridge.X1, &msg.BuildBridge.Y1, &msg.BuildBridge.Z, &msg.BuildBridge.X2, &msg.BuildBridge.Y2} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.BuildBridge.Direction); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown command type: 0x%02X", msg.CommandType)
	}

	return msg, nil
}

// EncodeCommand serializes a CommandMessage to wire bytes (header +
// payload). Thin typed wrapper around SerializeMessage for callers (and
// tests) that only ever deal in CommandMessage and don't want to juggle
// the Message interface.
func EncodeCommand(msg *CommandMessage) ([]byte, error) {
	return SerializeMessage(msg)
}

// DecodeCommand deserializes wire bytes into a CommandMessage. Thin typed
// wrapper around DeserializeMessage — see EncodeCommand.
func DecodeCommand(data []byte) (*CommandMessage, error) {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return nil, err
	}
	cmd, ok := msg.(*CommandMessage)
	if !ok {
		return nil, fmt.Errorf("expected CommandMessage, got %T", msg)
	}
	return cmd, nil
}

func (m *CommandMessage) Serialize() ([]byte, error) {
	return SerializeMessage(m)
}

func (m *CommandMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	c, ok := msg.(*CommandMessage)
	if !ok {
		return fmt.Errorf("expected CommandMessage, got %T", msg)
	}
	*m = *c
	return nil
}

// serializeCommandAck encodes CommandAckMessage payload
func serializeCommandAck(w io.Writer, msg *CommandAckMessage) error {
	// [4: CommandID] [1: Status] [2: ErrorMsgLen] [N: ErrorMsg]
	if err := binary.Write(w, binary.BigEndian, msg.CommandID); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, msg.Status); err != nil {
		return err
	}

	// Error message (length-prefixed)
	errorBytes := []byte(msg.ErrorMsg)
	if len(errorBytes) > 65535 {
		return errors.New("error message too long (max 65535 bytes)")
	}
	if err := binary.Write(w, binary.BigEndian, uint16(len(errorBytes))); err != nil {
		return err
	}
	if len(errorBytes) > 0 {
		if _, err := w.Write(errorBytes); err != nil {
			return err
		}
	}

	return nil
}

// deserializeCommandAck decodes CommandAckMessage payload
func deserializeCommandAck(data []byte) (*CommandAckMessage, error) {
	if len(data) < 7 { // Minimum: 4 (CommandID) + 1 (Status) + 2 (ErrorMsgLen)
		return nil, errors.New("command ack payload too short")
	}

	msg := &CommandAckMessage{}
	buf := bytes.NewReader(data)

	if err := binary.Read(buf, binary.BigEndian, &msg.CommandID); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &msg.Status); err != nil {
		return nil, err
	}

	// Read error message
	var errorLen uint16
	if err := binary.Read(buf, binary.BigEndian, &errorLen); err != nil {
		return nil, err
	}
	if errorLen > 0 {
		errorBytes := make([]byte, errorLen)
		if _, err := io.ReadFull(buf, errorBytes); err != nil {
			return nil, err
		}
		msg.ErrorMsg = string(errorBytes)
	}

	return msg, nil
}

func (m *CommandAckMessage) Serialize() ([]byte, error) {
	return SerializeMessage(m)
}

func (m *CommandAckMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	a, ok := msg.(*CommandAckMessage)
	if !ok {
		return fmt.Errorf("expected CommandAckMessage, got %T", msg)
	}
	*m = *a
	return nil
}

// serializeQuery encodes QueryMessage payload:
// [4: QueryID] [2: NameLen] [N: Name] [2: ArgsLen] [M: Args]
func serializeQuery(w io.Writer, msg *QueryMessage) error {
	if err := binary.Write(w, binary.BigEndian, msg.QueryID); err != nil {
		return err
	}
	nameBytes := []byte(msg.Name)
	if err := binary.Write(w, binary.BigEndian, uint16(len(nameBytes))); err != nil {
		return err
	}
	if _, err := w.Write(nameBytes); err != nil {
		return err
	}
	argsBytes := []byte(msg.Args)
	if err := binary.Write(w, binary.BigEndian, uint16(len(argsBytes))); err != nil {
		return err
	}
	if len(argsBytes) > 0 {
		if _, err := w.Write(argsBytes); err != nil {
			return err
		}
	}
	return nil
}

func deserializeQuery(data []byte) (*QueryMessage, error) {
	if len(data) < 8 { // 4 + 2 + 0 + 2 + 0
		return nil, errors.New("query payload too short")
	}
	msg := &QueryMessage{}
	buf := bytes.NewReader(data)
	if err := binary.Read(buf, binary.BigEndian, &msg.QueryID); err != nil {
		return nil, err
	}
	var nameLen uint16
	if err := binary.Read(buf, binary.BigEndian, &nameLen); err != nil {
		return nil, err
	}
	if nameLen > 0 {
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(buf, nameBytes); err != nil {
			return nil, err
		}
		msg.Name = string(nameBytes)
	}
	var argsLen uint16
	if err := binary.Read(buf, binary.BigEndian, &argsLen); err != nil {
		return nil, err
	}
	if argsLen > 0 {
		argsBytes := make([]byte, argsLen)
		if _, err := io.ReadFull(buf, argsBytes); err != nil {
			return nil, err
		}
		msg.Args = string(argsBytes)
	}
	return msg, nil
}

func (m *QueryMessage) Serialize() ([]byte, error) { return SerializeMessage(m) }

func (m *QueryMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	q, ok := msg.(*QueryMessage)
	if !ok {
		return fmt.Errorf("expected QueryMessage, got %T", msg)
	}
	*m = *q
	return nil
}

// serializeQueryResponse encodes QueryResponseMessage payload:
// [4: QueryID] [1: Status] [4: DataLen] [N: Data]
func serializeQueryResponse(w io.Writer, msg *QueryResponseMessage) error {
	if err := binary.Write(w, binary.BigEndian, msg.QueryID); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, msg.Status); err != nil {
		return err
	}
	dataBytes := []byte(msg.Data)
	if err := binary.Write(w, binary.BigEndian, uint32(len(dataBytes))); err != nil {
		return err
	}
	if len(dataBytes) > 0 {
		if _, err := w.Write(dataBytes); err != nil {
			return err
		}
	}
	return nil
}

func deserializeQueryResponse(data []byte) (*QueryResponseMessage, error) {
	if len(data) < 9 { // 4 + 1 + 4
		return nil, errors.New("query response payload too short")
	}
	msg := &QueryResponseMessage{}
	buf := bytes.NewReader(data)
	if err := binary.Read(buf, binary.BigEndian, &msg.QueryID); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &msg.Status); err != nil {
		return nil, err
	}
	var dataLen uint32
	if err := binary.Read(buf, binary.BigEndian, &dataLen); err != nil {
		return nil, err
	}
	if dataLen > 0 {
		dataBytes := make([]byte, dataLen)
		if _, err := io.ReadFull(buf, dataBytes); err != nil {
			return nil, err
		}
		msg.Data = string(dataBytes)
	}
	return msg, nil
}

func (m *QueryResponseMessage) Serialize() ([]byte, error) { return SerializeMessage(m) }

// serializeAnnouncementUpdate encodes AnnouncementUpdateMessage payload:
// [4: Count] [N × AnnouncementInfo] [N × RepeatCount]
//
// Per-entry layout: [4:ID][2:TypeID][1:Severity][2:X][2:Y][2:Z]
//
//	[4:GameYear][4:GameTick][2:TextLen][N:Text]
//
// RepeatCount travels in a TRAILING block after all N entries (not
// interleaved into each entry) — see the AnnouncementInfo doc comment in
// message.go for why.
func serializeAnnouncementUpdate(w io.Writer, msg *AnnouncementUpdateMessage) error {
	if err := binary.Write(w, binary.BigEndian, msg.Count); err != nil {
		return err
	}
	for i, a := range msg.Announcements {
		if err := binary.Write(w, binary.BigEndian, a.ID); err != nil {
			return fmt.Errorf("announcement %d: %w", i, err)
		}
		if err := binary.Write(w, binary.BigEndian, a.TypeID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, a.Severity); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, a.X); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, a.Y); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, a.Z); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, a.GameYear); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, a.GameTick); err != nil {
			return err
		}
		textBytes := []byte(a.Text)
		if err := binary.Write(w, binary.BigEndian, uint16(len(textBytes))); err != nil {
			return err
		}
		if len(textBytes) > 0 {
			if _, err := w.Write(textBytes); err != nil {
				return err
			}
		}
	}
	for i, a := range msg.Announcements {
		if err := binary.Write(w, binary.BigEndian, a.RepeatCount); err != nil {
			return fmt.Errorf("announcement %d repeat_count: %w", i, err)
		}
	}
	return nil
}

func deserializeAnnouncementUpdate(data []byte) (*AnnouncementUpdateMessage, error) {
	if len(data) < 4 {
		return nil, errors.New("announcement update payload too short")
	}
	msg := &AnnouncementUpdateMessage{}
	buf := bytes.NewReader(data)
	if err := binary.Read(buf, binary.BigEndian, &msg.Count); err != nil {
		return nil, err
	}
	msg.Announcements = make([]AnnouncementInfo, msg.Count)
	for i := uint32(0); i < msg.Count; i++ {
		a := &msg.Announcements[i]
		if err := binary.Read(buf, binary.BigEndian, &a.ID); err != nil {
			return nil, fmt.Errorf("announcement %d ID: %w", i, err)
		}
		if err := binary.Read(buf, binary.BigEndian, &a.TypeID); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &a.Severity); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &a.X); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &a.Y); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &a.Z); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &a.GameYear); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &a.GameTick); err != nil {
			return nil, err
		}
		var textLen uint16
		if err := binary.Read(buf, binary.BigEndian, &textLen); err != nil {
			return nil, err
		}
		if textLen > 0 {
			textBytes := make([]byte, textLen)
			if _, err := io.ReadFull(buf, textBytes); err != nil {
				return nil, err
			}
			a.Text = string(textBytes)
		}
	}

	// Trailing RepeatCount block (backward-compat decode): an OLD plugin
	// (predates this field) sends exactly the N entries above and nothing
	// more — buf is empty here, and every AnnouncementInfo.RepeatCount stays
	// at its zero value, which is the correct default (the plugin has no
	// way to tell us a repeat happened, so treat it as "not observed").
	// A NEW plugin appends exactly Count × 4 more bytes. Any other leftover
	// byte count is unexpected framing we don't understand; per the "old-
	// format messages must not error" rule we deliberately do NOT error in
	// that case either — we just leave RepeatCount at 0, same as old format.
	if expected := int(msg.Count) * 4; expected > 0 && buf.Len() == expected {
		for i := uint32(0); i < msg.Count; i++ {
			if err := binary.Read(buf, binary.BigEndian, &msg.Announcements[i].RepeatCount); err != nil {
				return nil, fmt.Errorf("announcement %d repeat_count: %w", i, err)
			}
		}
	}
	return msg, nil
}

func (m *AnnouncementUpdateMessage) Serialize() ([]byte, error) { return SerializeMessage(m) }

func (m *AnnouncementUpdateMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	a, ok := msg.(*AnnouncementUpdateMessage)
	if !ok {
		return fmt.Errorf("expected AnnouncementUpdateMessage, got %T", msg)
	}
	*m = *a
	return nil
}

func (m *QueryResponseMessage) Deserialize(data []byte) error {
	msg, err := DeserializeMessage(data)
	if err != nil {
		return err
	}
	q, ok := msg.(*QueryResponseMessage)
	if !ok {
		return fmt.Errorf("expected QueryResponseMessage, got %T", msg)
	}
	*m = *q
	return nil
}
