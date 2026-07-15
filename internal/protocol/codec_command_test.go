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

func TestEncodeDecodeRemoveZoneCommand(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   9,
		CommandType: CommandTypeRemoveZone,
		RemoveZone:  RemoveZoneDesignation{X: 20, Y: 30, Z: 90},
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
	if got.CommandID != 9 || got.CommandType != CommandTypeRemoveZone || got.RemoveZone != orig.RemoveZone {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestQueueJobRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   8,
		CommandType: CommandTypeQueueJob,
		QueueJob:    QueueJobDesignation{X: 50, Y: 50, Z: 139, OrderType: OrderTypeMakeBed},
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
	if got.CommandID != 8 || got.CommandType != CommandTypeQueueJob || got.QueueJob != orig.QueueJob {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

// TestQueueJobByNameRoundTrip covers the generalized name-based path
// (OrderTypeByName + JobTypeName) — mirrors TestQueueJobRoundTrip above,
// which covers the pre-existing byte-vocabulary path.
func TestQueueJobByNameRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   11,
		CommandType: CommandTypeQueueJob,
		QueueJob: QueueJobDesignation{
			X: 50, Y: 50, Z: 139,
			OrderType:   OrderTypeByName,
			JobTypeName: "ConstructHatchCover",
		},
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
	if got.CommandID != 11 || got.CommandType != CommandTypeQueueJob || got.QueueJob != orig.QueueJob {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.QueueJob.JobTypeName != "ConstructHatchCover" {
		t.Fatalf("job type name lost in round-trip: %+v", got.QueueJob)
	}
}

// TestQueueJobByNameEmptyNameRejected ensures Validate() catches the
// footgun of sending OrderTypeByName with no name — mirrors the plugin
// side's requirement that a by-name QUEUE_JOB payload always carry a name.
func TestQueueJobByNameEmptyNameRejected(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   12,
		CommandType: CommandTypeQueueJob,
		QueueJob:    QueueJobDesignation{X: 1, Y: 1, Z: 1, OrderType: OrderTypeByName},
	}
	if _, err := SerializeMessage(msg); err == nil {
		t.Fatal("expected validation error for empty JobTypeName with OrderTypeByName")
	}
}

// TestQueueJobCustomReactionRoundTrip covers the reaction-based path
// (OrderTypeCustomReaction + ReactionCode) — mirrors
// TestQueueJobByNameRoundTrip above, which covers the job-type-by-name
// path. Both sentinels share the same trailing length-prefixed-string wire
// shape; this confirms the reaction code lands in ReactionCode, not
// JobTypeName.
func TestQueueJobCustomReactionRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   14,
		CommandType: CommandTypeQueueJob,
		QueueJob: QueueJobDesignation{
			X: 50, Y: 50, Z: 139,
			OrderType:    OrderTypeCustomReaction,
			ReactionCode: "BREW_DRINK_FROM_PLANT",
		},
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
	if got.CommandID != 14 || got.CommandType != CommandTypeQueueJob || got.QueueJob != orig.QueueJob {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.QueueJob.ReactionCode != "BREW_DRINK_FROM_PLANT" {
		t.Fatalf("reaction code lost in round-trip: %+v", got.QueueJob)
	}
	if got.QueueJob.JobTypeName != "" {
		t.Fatalf("reaction code path must not populate JobTypeName: %+v", got.QueueJob)
	}
}

// TestQueueJobCustomReactionEmptyCodeRejected ensures Validate() catches
// the footgun of sending OrderTypeCustomReaction with no code — mirrors
// TestQueueJobByNameEmptyNameRejected above.
func TestQueueJobCustomReactionEmptyCodeRejected(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   15,
		CommandType: CommandTypeQueueJob,
		QueueJob:    QueueJobDesignation{X: 1, Y: 1, Z: 1, OrderType: OrderTypeCustomReaction},
	}
	if _, err := SerializeMessage(msg); err == nil {
		t.Fatal("expected validation error for empty ReactionCode with OrderTypeCustomReaction")
	}
}

// TestQueueJobOldPayloadStillDecodes constructs a raw QUEUE_JOB payload in
// the EXACT pre-this-change wire shape (12 bytes: no trailing name) and
// confirms it still decodes correctly — the backward-compat guarantee this
// feature depends on (old 11-item-vocabulary callers must keep working
// unchanged).
func TestQueueJobOldPayloadStillDecodes(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(0)) // length placeholder
	binary.Write(&buf, binary.BigEndian, ProtocolVersion)
	binary.Write(&buf, binary.BigEndian, MessageTypeCommand)
	binary.Write(&buf, binary.BigEndian, uint32(13)) // CommandID
	binary.Write(&buf, binary.BigEndian, CommandTypeQueueJob)
	binary.Write(&buf, binary.BigEndian, int16(50))        // X
	binary.Write(&buf, binary.BigEndian, int16(60))        // Y
	binary.Write(&buf, binary.BigEndian, int16(139))       // Z
	binary.Write(&buf, binary.BigEndian, OrderTypeMakeBed) // OrderType — no trailing name
	data := buf.Bytes()
	binary.BigEndian.PutUint32(data[0:4], uint32(len(data)))

	decoded, err := DeserializeMessage(data)
	if err != nil {
		t.Fatalf("decode old-shape payload: %v", err)
	}
	got, ok := decoded.(*CommandMessage)
	if !ok {
		t.Fatalf("decoded wrong type %T", decoded)
	}
	want := QueueJobDesignation{X: 50, Y: 60, Z: 139, OrderType: OrderTypeMakeBed}
	if got.QueueJob != want {
		t.Fatalf("old-shape payload round-trip mismatch: got %+v, want %+v", got.QueueJob, want)
	}
}

func TestSetLaborRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   9,
		CommandType: CommandTypeSetLabor,
		SetLabor:    SetLaborDesignation{UnitID: 42, LaborID: LaborMine, Enable: true},
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
	if got.CommandID != 9 || got.CommandType != CommandTypeSetLabor || got.SetLabor != orig.SetLabor {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestSetLaborDisableRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   10,
		CommandType: CommandTypeSetLabor,
		SetLabor:    SetLaborDesignation{UnitID: 7, LaborID: LaborFish, Enable: false},
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
	if got.CommandID != 10 || got.CommandType != CommandTypeSetLabor || got.SetLabor != orig.SetLabor {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestSetLaborInvalidLaborIDRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   11,
		CommandType: CommandTypeSetLabor,
		SetLabor:    SetLaborDesignation{UnitID: 1, LaborID: LaborMaxIndex + 1, Enable: true},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("labor id out of range must fail validation")
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
