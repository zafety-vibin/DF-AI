package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestRemoveBuildingRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   7,
		CommandType: CommandTypeRemoveBuilding,
		Remove:      RemoveBuildingDesignation{X: 50, Y: 60, Z: 139},
	}
	data, err := SerializeMessage(orig)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DeserializeMessage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, ok := decoded.(*CommandMessage)
	if !ok {
		t.Fatalf("decoded wrong type %T", decoded)
	}
	if got.CommandID != 7 || got.CommandType != CommandTypeRemoveBuilding || got.Remove != orig.Remove {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestBuildMaterialRoundTrip(t *testing.T) {
	for _, mat := range []uint8{MaterialClassAny, MaterialClassWood, MaterialClassStone, MaterialClassBlocks} {
		orig := &CommandMessage{
			CommandID:   9,
			CommandType: CommandTypeBuild,
			Build:       BuildDesignation{X: 10, Y: 20, Z: 100, BuildType: BuildTypeWall, Material: mat},
		}
		data, err := SerializeMessage(orig)
		if err != nil {
			t.Fatalf("material %d encode: %v", mat, err)
		}
		decoded, err := DeserializeMessage(data)
		if err != nil {
			t.Fatalf("material %d decode: %v", mat, err)
		}
		got, ok := decoded.(*CommandMessage)
		if !ok {
			t.Fatalf("decoded wrong type %T", decoded)
		}
		if got.Build != orig.Build {
			t.Fatalf("material %d round-trip mismatch: %+v", mat, got.Build)
		}
	}
}

func TestBuildMaterialInvalidRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   9,
		CommandType: CommandTypeBuild,
		Build:       BuildDesignation{X: 1, Y: 1, Z: 1, BuildType: BuildTypeWall, Material: MaterialClassBlocks + 1},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("material class out of range must fail validation")
	}
}

// TestBuildLegacyPayloadWithoutMaterialByte hand-crafts a BUILD payload
// WITHOUT the trailing material byte (as pre-material peers emit) and
// checks it decodes as MaterialClassAny — the backward-compatibility rule
// both sides of the wire contract must obey.
func TestBuildLegacyPayloadWithoutMaterialByte(t *testing.T) {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(0)) // length placeholder
	buf.WriteByte(ProtocolVersion)
	buf.WriteByte(MessageTypeCommand)
	_ = binary.Write(&buf, binary.BigEndian, uint32(11)) // CommandID
	buf.WriteByte(CommandTypeBuild)
	_ = binary.Write(&buf, binary.BigEndian, int16(3)) // X
	_ = binary.Write(&buf, binary.BigEndian, int16(4)) // Y
	_ = binary.Write(&buf, binary.BigEndian, int16(5)) // Z
	buf.WriteByte(BuildTypeBed)
	data := buf.Bytes()
	binary.BigEndian.PutUint32(data[0:4], uint32(len(data)))

	decoded, err := DeserializeMessage(data)
	if err != nil {
		t.Fatalf("decode legacy build payload: %v", err)
	}
	got, ok := decoded.(*CommandMessage)
	if !ok {
		t.Fatalf("decoded wrong type %T", decoded)
	}
	want := BuildDesignation{X: 3, Y: 4, Z: 5, BuildType: BuildTypeBed, Material: MaterialClassAny}
	if got.CommandID != 11 || got.Build != want {
		t.Fatalf("legacy payload mismatch: %+v", got)
	}
}
