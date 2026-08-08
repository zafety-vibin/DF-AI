package protocol

import (
	"bytes"
	"encoding/binary"
	"reflect"
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

func TestDesignateBurrowRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   11,
		CommandType: CommandTypeDesignateBurrow,
		DesignateBurrow: DesignateBurrowDesignation{
			Name: "shelter",
			X1:   10, Y1: 10, Z1: 90,
			X2: 20, Y2: 20, Z2: 92,
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
	if got.CommandID != 11 || got.CommandType != CommandTypeDesignateBurrow || got.DesignateBurrow != orig.DesignateBurrow {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestRemoveBurrowRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:    12,
		CommandType:  CommandTypeRemoveBurrow,
		RemoveBurrow: RemoveBurrowDesignation{Name: "shelter"},
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
	if got.CommandID != 12 || got.CommandType != CommandTypeRemoveBurrow || got.RemoveBurrow != orig.RemoveBurrow {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestAssignBurrowRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   13,
		CommandType: CommandTypeAssignBurrow,
		AssignBurrow: AssignBurrowDesignation{
			Name: "shelter", Assign: true, AllCitizens: false, UnitID: 42,
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
	if got.CommandID != 13 || got.CommandType != CommandTypeAssignBurrow || got.AssignBurrow != orig.AssignBurrow {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestAssignBurrowAllCitizensRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   14,
		CommandType: CommandTypeAssignBurrow,
		AssignBurrow: AssignBurrowDesignation{
			Name: "shelter", Assign: false, AllCitizens: true, UnitID: 0,
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
	if got.CommandID != 14 || got.CommandType != CommandTypeAssignBurrow || got.AssignBurrow != orig.AssignBurrow {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestSetAlertRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   15,
		CommandType: CommandTypeSetAlert,
		SetAlert:    SetAlertDesignation{Name: "shelter", Active: true},
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
	if got.CommandID != 15 || got.CommandType != CommandTypeSetAlert || got.SetAlert != orig.SetAlert {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestDesignateBurrowEmptyNameRejected(t *testing.T) {
	msg := &CommandMessage{
		CommandID:       1,
		CommandType:     CommandTypeDesignateBurrow,
		DesignateBurrow: DesignateBurrowDesignation{Name: "", X2: 1, Y2: 1, Z2: 1},
	}
	if err := msg.Validate(); err == nil {
		t.Fatal("expected error for empty burrow name")
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

// TestWorkOrderByNameRoundTrip covers WorkOrderDesignation's generalized
// name-based path (OrderTypeByName + JobTypeName) — mirrors
// TestQueueJobByNameRoundTrip above, which covers the same sentinel on
// QueueJobDesignation. The trailing name here follows Quantity instead of
// coordinates, since WorkOrderDesignation carries no X/Y/Z.
func TestWorkOrderByNameRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   20,
		CommandType: CommandTypeWorkOrder,
		Order: WorkOrderDesignation{
			OrderType:   OrderTypeByName,
			Quantity:    5,
			JobTypeName: "ProcessPlants",
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
	if got.CommandID != 20 || got.CommandType != CommandTypeWorkOrder || got.Order != orig.Order {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Order.JobTypeName != "ProcessPlants" {
		t.Fatalf("job type name lost in round-trip: %+v", got.Order)
	}
}

// TestWorkOrderByNameEmptyNameRejected ensures Validate() catches the
// footgun of sending OrderTypeByName with no name — mirrors
// TestQueueJobByNameEmptyNameRejected above.
func TestWorkOrderByNameEmptyNameRejected(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   21,
		CommandType: CommandTypeWorkOrder,
		Order:       WorkOrderDesignation{OrderType: OrderTypeByName, Quantity: 1},
	}
	if _, err := SerializeMessage(msg); err == nil {
		t.Fatal("expected validation error for empty JobTypeName with OrderTypeByName")
	}
}

// TestWorkOrderOldPayloadStillDecodes constructs a raw WORK_ORDER payload in
// the EXACT pre-this-change wire shape (3 bytes: OrderType+Quantity, no
// trailing name) and confirms it still decodes correctly — the
// backward-compat guarantee this feature depends on (old byte-vocabulary
// callers must keep working unchanged).
func TestWorkOrderOldPayloadStillDecodes(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(0)) // length placeholder
	binary.Write(&buf, binary.BigEndian, ProtocolVersion)
	binary.Write(&buf, binary.BigEndian, MessageTypeCommand)
	binary.Write(&buf, binary.BigEndian, uint32(22)) // CommandID
	binary.Write(&buf, binary.BigEndian, CommandTypeWorkOrder)
	binary.Write(&buf, binary.BigEndian, OrderTypeMakeBed) // OrderType — no trailing name
	binary.Write(&buf, binary.BigEndian, uint16(3))        // Quantity
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
	want := WorkOrderDesignation{OrderType: OrderTypeMakeBed, Quantity: 3}
	if got.Order != want {
		t.Fatalf("old-shape payload round-trip mismatch: got %+v, want %+v", got.Order, want)
	}
}

// TestWorkOrderMaterialFrequencyRoundTrip covers the manager-work-order
// fix wave's two new fields: Material (a job_material_category keyword or
// DFHack material token) and Frequency (a recurring cadence) — both travel
// unconditionally alongside the byte-vocabulary path (OrderTypeMakeBed, not
// OrderTypeByName), confirming they don't require the by-name sentinel.
func TestWorkOrderMaterialFrequencyRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   23,
		CommandType: CommandTypeWorkOrder,
		Order: WorkOrderDesignation{
			OrderType: OrderTypeMakeBed,
			Quantity:  4,
			Material:  "wood",
			Frequency: WorkOrderFrequencyDaily,
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
	if got.CommandID != 23 || got.CommandType != CommandTypeWorkOrder || got.Order != orig.Order {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Order.Material != "wood" || got.Order.Frequency != WorkOrderFrequencyDaily {
		t.Fatalf("material/frequency lost in round-trip: %+v", got.Order)
	}
}

// TestWorkOrderInvalidFrequencyRejected ensures Validate() catches a
// Frequency byte outside df::workquota_frequency_type's real range
// (OneTime..Yearly, 0-4).
func TestWorkOrderInvalidFrequencyRejected(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   24,
		CommandType: CommandTypeWorkOrder,
		Order:       WorkOrderDesignation{OrderType: OrderTypeMakeBed, Quantity: 1, Frequency: 0x05},
	}
	if _, err := SerializeMessage(msg); err == nil {
		t.Fatal("expected validation error for out-of-range work order frequency")
	}
}

// TestQueueJobMaterialRoundTrip covers the SmeltOre direct-queue unlock's
// Material field (an exact ore token, e.g. "INORGANIC:LIMONITE") — travels
// unconditionally alongside the by-name path the same way WorkOrder's
// Material does.
func TestQueueJobMaterialRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   25,
		CommandType: CommandTypeQueueJob,
		QueueJob: QueueJobDesignation{
			X: 50, Y: 50, Z: 139,
			OrderType:   OrderTypeByName,
			JobTypeName: "SmeltOre",
			Material:    "INORGANIC:LIMONITE",
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
	if got.CommandID != 25 || got.CommandType != CommandTypeQueueJob || got.QueueJob != orig.QueueJob {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.QueueJob.Material != "INORGANIC:LIMONITE" {
		t.Fatalf("material lost in round-trip: %+v", got.QueueJob)
	}
}

// TestQueueJobSubtypeRoundTrip covers the item-SUBTYPE pinning wave's
// Subtype field (a bare raws itemdef token, e.g. "ITEM_WEAPON_PICK") —
// travels unconditionally after Material, alongside the by-name path, the
// same way TestQueueJobMaterialRoundTrip above covers Material.
func TestQueueJobSubtypeRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   26,
		CommandType: CommandTypeQueueJob,
		QueueJob: QueueJobDesignation{
			X: 50, Y: 50, Z: 139,
			OrderType:   OrderTypeByName,
			JobTypeName: "MakeWeapon",
			Subtype:     "ITEM_WEAPON_PICK",
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
	if got.CommandID != 26 || got.CommandType != CommandTypeQueueJob || got.QueueJob != orig.QueueJob {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.QueueJob.Subtype != "ITEM_WEAPON_PICK" {
		t.Fatalf("subtype lost in round-trip: %+v", got.QueueJob)
	}
}

// TestWorkOrderSubtypeRoundTrip covers the item-SUBTYPE pinning wave's
// Subtype field on WorkOrderDesignation — travels unconditionally after
// Frequency, alongside the byte-vocabulary path (mirrors
// TestWorkOrderMaterialFrequencyRoundTrip above).
func TestWorkOrderSubtypeRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   27,
		CommandType: CommandTypeWorkOrder,
		Order: WorkOrderDesignation{
			OrderType:   OrderTypeByName,
			Quantity:    2,
			JobTypeName: "MakeArmor",
			Subtype:     "ITEM_ARMOR_MAIL_SHIRT",
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
	if got.CommandID != 27 || got.CommandType != CommandTypeWorkOrder || got.Order != orig.Order {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Order.Subtype != "ITEM_ARMOR_MAIL_SHIRT" {
		t.Fatalf("subtype lost in round-trip: %+v", got.Order)
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
	want := BuildDesignation{X: 3, Y: 4, Z: 5, BuildType: BuildTypeBed, Material: MaterialClassAny, Quality: QualityTierAny, Orientation: BuildOrientAny}
	if got.CommandID != 11 || got.Build != want {
		t.Fatalf("legacy payload mismatch: %+v", got)
	}
}

// TestBuildLegacyPayloadWithMaterialButNoQuality hand-crafts a BUILD
// payload with the trailing material byte but WITHOUT the trailing quality
// byte (as an already-deployed material-only peer emits) and checks it
// decodes as QualityTierAny — the same backward-compatibility rule as
// TestBuildLegacyPayloadWithoutMaterialByte, one wire generation later.
func TestBuildLegacyPayloadWithMaterialButNoQuality(t *testing.T) {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(0)) // length placeholder
	buf.WriteByte(ProtocolVersion)
	buf.WriteByte(MessageTypeCommand)
	_ = binary.Write(&buf, binary.BigEndian, uint32(15)) // CommandID
	buf.WriteByte(CommandTypeBuild)
	_ = binary.Write(&buf, binary.BigEndian, int16(3)) // X
	_ = binary.Write(&buf, binary.BigEndian, int16(4)) // Y
	_ = binary.Write(&buf, binary.BigEndian, int16(5)) // Z
	buf.WriteByte(BuildTypeWall)
	buf.WriteByte(MaterialClassStone)
	data := buf.Bytes()
	binary.BigEndian.PutUint32(data[0:4], uint32(len(data)))

	decoded, err := DeserializeMessage(data)
	if err != nil {
		t.Fatalf("decode material-only build payload: %v", err)
	}
	got, ok := decoded.(*CommandMessage)
	if !ok {
		t.Fatalf("decoded wrong type %T", decoded)
	}
	want := BuildDesignation{X: 3, Y: 4, Z: 5, BuildType: BuildTypeWall, Material: MaterialClassStone, Quality: QualityTierAny, Orientation: BuildOrientAny}
	if got.CommandID != 15 || got.Build != want {
		t.Fatalf("material-only payload mismatch: %+v", got)
	}
}

func TestBuildQualityRoundTrip(t *testing.T) {
	for _, qual := range []uint8{QualityTierAny, QualityTierOrdinary, QualityTierWellCrafted, QualityTierFinelyCrafted, QualityTierSuperior, QualityTierExceptional, QualityTierMasterful, QualityTierArtifact} {
		orig := &CommandMessage{
			CommandID:   16,
			CommandType: CommandTypeBuild,
			Build:       BuildDesignation{X: 1, Y: 2, Z: 3, BuildType: BuildTypeBed, Material: MaterialClassAny, Quality: qual},
		}
		data, err := SerializeMessage(orig)
		if err != nil {
			t.Fatalf("quality %d encode: %v", qual, err)
		}
		decoded, err := DeserializeMessage(data)
		if err != nil {
			t.Fatalf("quality %d decode: %v", qual, err)
		}
		got, ok := decoded.(*CommandMessage)
		if !ok {
			t.Fatalf("decoded wrong type %T", decoded)
		}
		if got.Build != orig.Build {
			t.Fatalf("quality %d round-trip mismatch: %+v", qual, got.Build)
		}
	}
}

func TestBuildQualityInvalidRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   16,
		CommandType: CommandTypeBuild,
		Build:       BuildDesignation{X: 1, Y: 1, Z: 1, BuildType: BuildTypeBed, Quality: QualityTierArtifact + 1},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("quality tier out of range must fail validation")
	}
}

// TestBuildOrientationRoundTrip covers the water/power-transmission
// building family's Orientation trailing byte — mirrors
// TestBuildMaterialRoundTrip/TestBuildQualityRoundTrip, one wire generation
// later.
func TestBuildOrientationRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		buildType uint8
		orient    uint8
	}{
		{BuildTypeScrewPump, BuildOrientNorth},
		{BuildTypeScrewPump, BuildOrientEast},
		{BuildTypeScrewPump, BuildOrientSouth},
		{BuildTypeScrewPump, BuildOrientWest},
		{BuildTypeAxleHorizontal, BuildOrientHorizontal},
		{BuildTypeAxleHorizontal, BuildOrientVertical},
		{BuildTypeWaterWheel, BuildOrientHorizontal},
		{BuildTypeWaterWheel, BuildOrientVertical},
		{BuildTypeRollers, BuildOrientNorth},
		{BuildTypeGearAssembly, BuildOrientAny},
		{BuildTypeAxleVertical, BuildOrientAny},
		{BuildTypeWindmill, BuildOrientAny},
	} {
		orig := &CommandMessage{
			CommandID:   20,
			CommandType: CommandTypeBuild,
			Build:       BuildDesignation{X: 1, Y: 2, Z: 3, BuildType: tc.buildType, Material: MaterialClassAny, Quality: QualityTierAny, Orientation: tc.orient},
		}
		data, err := SerializeMessage(orig)
		if err != nil {
			t.Fatalf("buildType 0x%02X orientation 0x%02X encode: %v", tc.buildType, tc.orient, err)
		}
		decoded, err := DeserializeMessage(data)
		if err != nil {
			t.Fatalf("buildType 0x%02X orientation 0x%02X decode: %v", tc.buildType, tc.orient, err)
		}
		got, ok := decoded.(*CommandMessage)
		if !ok {
			t.Fatalf("decoded wrong type %T", decoded)
		}
		if got.Build != orig.Build {
			t.Fatalf("buildType 0x%02X orientation 0x%02X round-trip mismatch: %+v", tc.buildType, tc.orient, got.Build)
		}
	}
}

func TestBuildOrientationInvalidRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   20,
		CommandType: CommandTypeBuild,
		Build:       BuildDesignation{X: 1, Y: 1, Z: 1, BuildType: BuildTypeScrewPump, Orientation: BuildOrientWest + 1},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("orientation byte out of range must fail validation")
	}
}

// TestIsBuildTypeWaterPower checks the water/power-transmission range
// helper's boundaries against every curated BuildType constant in that
// family plus its immediate neighbors (the room-value-furniture family
// below it, and the first unassigned byte above it).
func TestIsBuildTypeWaterPower(t *testing.T) {
	for _, bt := range []uint8{BuildTypeScrewPump, BuildTypeGearAssembly, BuildTypeAxleHorizontal, BuildTypeAxleVertical, BuildTypeWaterWheel, BuildTypeWindmill, BuildTypeRollers} {
		if !IsBuildTypeWaterPower(bt) {
			t.Fatalf("IsBuildTypeWaterPower(0x%02X) = false, want true", bt)
		}
	}
	for _, bt := range []uint8{BuildTypeInstrument, 0xB0} {
		if IsBuildTypeWaterPower(bt) {
			t.Fatalf("IsBuildTypeWaterPower(0x%02X) = true, want false", bt)
		}
	}
}

// TestIsBuildTypeTrap checks the trap-subtype range helper's boundaries
// against every curated BuildType constant in that family plus its
// immediate lower neighbor (the water/power family just below it) and
// BuildTypeLever (a trap_type subtype that deliberately lives OUTSIDE this
// range, in the doors/hatches family instead).
func TestIsBuildTypeTrap(t *testing.T) {
	for _, bt := range []uint8{BuildTypePressurePlate, BuildTypeStoneFallTrap, BuildTypeWeaponTrap, BuildTypeTrackStop} {
		if !IsBuildTypeTrap(bt) {
			t.Fatalf("IsBuildTypeTrap(0x%02X) = false, want true", bt)
		}
	}
	for _, bt := range []uint8{BuildTypeRollers, BuildTypeLever} {
		if IsBuildTypeTrap(bt) {
			t.Fatalf("IsBuildTypeTrap(0x%02X) = true, want false", bt)
		}
	}
}

// TestBuildTrapTypesRoundTrip covers the four new df::trap_type BuildType
// values end to end (encode -> decode), mirroring TestBuildMaterialRoundTrip.
// TrackStop additionally accepts a Material constraint (see
// dfhack-plugin/buildings.cpp placeTrap); the other three always send
// MaterialClassAny since the plugin rejects any other value for them.
func TestBuildTrapTypesRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		buildType uint8
		material  uint8
	}{
		{BuildTypePressurePlate, MaterialClassAny},
		{BuildTypeStoneFallTrap, MaterialClassAny},
		{BuildTypeWeaponTrap, MaterialClassAny},
		{BuildTypeTrackStop, MaterialClassAny},
		{BuildTypeTrackStop, MaterialClassStone},
	} {
		orig := &CommandMessage{
			CommandID:   21,
			CommandType: CommandTypeBuild,
			Build:       BuildDesignation{X: 5, Y: 6, Z: 7, BuildType: tc.buildType, Material: tc.material, Quality: QualityTierAny, Orientation: BuildOrientAny},
		}
		data, err := SerializeMessage(orig)
		if err != nil {
			t.Fatalf("buildType 0x%02X encode: %v", tc.buildType, err)
		}
		decoded, err := DeserializeMessage(data)
		if err != nil {
			t.Fatalf("buildType 0x%02X decode: %v", tc.buildType, err)
		}
		got, ok := decoded.(*CommandMessage)
		if !ok {
			t.Fatalf("decoded wrong type %T", decoded)
		}
		if got.Build != orig.Build {
			t.Fatalf("buildType 0x%02X round-trip mismatch: %+v", tc.buildType, got.Build)
		}
	}
}

// TestBuildByNameRoundTrip covers the generalized name-based path
// (BuildTypeByName + BuildTypeName) — mirrors TestQueueJobByNameRoundTrip,
// which covers the identical mechanism for QUEUE_JOB.
func TestBuildByNameRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   17,
		CommandType: CommandTypeBuild,
		Build: BuildDesignation{
			X: 10, Y: 20, Z: 100,
			BuildType:     BuildTypeByName,
			BuildTypeName: "Well",
			Material:      MaterialClassAny,
			Quality:       QualityTierAny,
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
	if got.CommandID != 17 || got.CommandType != CommandTypeBuild || got.Build != orig.Build {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Build.BuildTypeName != "Well" {
		t.Fatalf("build type name lost in round-trip: %+v", got.Build)
	}
}

// TestBuildByNameEmptyNameRejected ensures Validate() catches the footgun
// of sending BuildTypeByName with no name — mirrors
// TestQueueJobByNameEmptyNameRejected.
func TestBuildByNameEmptyNameRejected(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   18,
		CommandType: CommandTypeBuild,
		Build:       BuildDesignation{X: 1, Y: 1, Z: 1, BuildType: BuildTypeByName},
	}
	if _, err := SerializeMessage(msg); err == nil {
		t.Fatal("expected validation error for empty BuildTypeName with BuildTypeByName")
	}
}

// TestBuildOldPayloadStillDecodes constructs a raw BUILD payload in the
// EXACT pre-this-change wire shape (no trailing name, since BuildTypeByName
// is brand new — no old peer ever emits it) and confirms a normal curated
// BuildType payload still decodes correctly with no name tail read. Mirrors
// TestQueueJobOldPayloadStillDecodes.
func TestBuildOldPayloadStillDecodes(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(0)) // length placeholder
	binary.Write(&buf, binary.BigEndian, ProtocolVersion)
	binary.Write(&buf, binary.BigEndian, MessageTypeCommand)
	binary.Write(&buf, binary.BigEndian, uint32(19)) // CommandID
	binary.Write(&buf, binary.BigEndian, CommandTypeBuild)
	binary.Write(&buf, binary.BigEndian, int16(10)) // X
	binary.Write(&buf, binary.BigEndian, int16(20)) // Y
	binary.Write(&buf, binary.BigEndian, int16(30)) // Z
	binary.Write(&buf, binary.BigEndian, BuildTypeWall)
	binary.Write(&buf, binary.BigEndian, MaterialClassStone)
	binary.Write(&buf, binary.BigEndian, QualityTierAny) // no trailing name
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
	want := BuildDesignation{X: 10, Y: 20, Z: 30, BuildType: BuildTypeWall, Material: MaterialClassStone, Quality: QualityTierAny, Orientation: BuildOrientAny}
	if got.Build != want {
		t.Fatalf("old-shape payload round-trip mismatch: got %+v, want %+v", got.Build, want)
	}
}

func TestSetWorkshopProfileRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   17,
		CommandType: CommandTypeSetWorkshopProfile,
		SetWorkshopProfile: SetWorkshopProfileDesignation{
			X: 10, Y: 20, Z: 100,
			MinSkillLevel: 5, MaxSkillLevel: -1, WorkerUnitID: 42,
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
	if got.CommandID != 17 || got.CommandType != CommandTypeSetWorkshopProfile || got.SetWorkshopProfile != orig.SetWorkshopProfile {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestSetWorkshopProfileInvalidRangeRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   17,
		CommandType: CommandTypeSetWorkshopProfile,
		SetWorkshopProfile: SetWorkshopProfileDesignation{
			X: 1, Y: 1, Z: 1,
			MinSkillLevel: 21, MaxSkillLevel: -1, WorkerUnitID: -1,
		},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("min_skill_level out of range must fail validation")
	}
}

func TestSetWorkshopProfileMaxBelowMinRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   17,
		CommandType: CommandTypeSetWorkshopProfile,
		SetWorkshopProfile: SetWorkshopProfileDesignation{
			X: 1, Y: 1, Z: 1,
			MinSkillLevel: 10, MaxSkillLevel: 5, WorkerUnitID: -1,
		},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("max_skill_level < min_skill_level must fail validation")
	}
}

func TestLinkBuildingRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   12,
		CommandType: CommandTypeLinkBuilding,
		LinkBuilding: LinkBuildingDesignation{
			LeverX: 10, LeverY: 20, LeverZ: 100,
			TargetX: 15, TargetY: 25, TargetZ: 100,
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
	if got.CommandID != 12 || got.CommandType != CommandTypeLinkBuilding || got.LinkBuilding != orig.LinkBuilding {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestPullLeverRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   13,
		CommandType: CommandTypePullLever,
		PullLever:   PullLeverDesignation{X: 10, Y: 20, Z: 100},
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
	if got.CommandID != 13 || got.CommandType != CommandTypePullLever || got.PullLever != orig.PullLever {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestBuildBridgeRoundTrip(t *testing.T) {
	for _, dir := range []uint8{BridgeDirectionRetract, BridgeDirectionRaiseN, BridgeDirectionRaiseS, BridgeDirectionRaiseE, BridgeDirectionRaiseW} {
		orig := &CommandMessage{
			CommandID:   14,
			CommandType: CommandTypeBuildBridge,
			BuildBridge: BuildBridgeDesignation{X1: 10, Y1: 20, Z: 100, X2: 14, Y2: 22, Direction: dir},
		}
		data, err := SerializeMessage(orig)
		if err != nil {
			t.Fatalf("direction %d encode: %v", dir, err)
		}
		decoded, err := DeserializeMessage(data)
		if err != nil {
			t.Fatalf("direction %d decode: %v", dir, err)
		}
		got, ok := decoded.(*CommandMessage)
		if !ok {
			t.Fatalf("decoded wrong type %T", decoded)
		}
		if got.BuildBridge != orig.BuildBridge {
			t.Fatalf("direction %d round-trip mismatch: %+v", dir, got.BuildBridge)
		}
	}
}

func TestBuildBridgeInvalidDirectionRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   14,
		CommandType: CommandTypeBuildBridge,
		BuildBridge: BuildBridgeDesignation{X1: 1, Y1: 1, Z: 1, X2: 2, Y2: 2, Direction: BridgeDirectionRaiseW + 1},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("bridge direction out of range must fail validation")
	}
}

func TestBuildBridgeInvalidRegionRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   14,
		CommandType: CommandTypeBuildBridge,
		BuildBridge: BuildBridgeDesignation{X1: 5, Y1: 1, Z: 1, X2: 2, Y2: 2, Direction: BridgeDirectionRetract},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("x2<x1 must fail validation")
	}
}

func TestAssignWorkDetailRoundTrip(t *testing.T) {
	for _, add := range []bool{true, false} {
		orig := &CommandMessage{
			CommandID:        15,
			CommandType:      CommandTypeAssignWorkDetail,
			AssignWorkDetail: AssignWorkDetailDesignation{DetailIndex: 3, UnitID: 42, Add: add},
		}
		data, err := SerializeMessage(orig)
		if err != nil {
			t.Fatalf("add=%v encode: %v", add, err)
		}
		decoded, err := DeserializeMessage(data)
		if err != nil {
			t.Fatalf("add=%v decode: %v", add, err)
		}
		got, ok := decoded.(*CommandMessage)
		if !ok {
			t.Fatalf("decoded wrong type %T", decoded)
		}
		if got.CommandID != 15 || got.CommandType != CommandTypeAssignWorkDetail || got.AssignWorkDetail != orig.AssignWorkDetail {
			t.Fatalf("add=%v round-trip mismatch: %+v", add, got.AssignWorkDetail)
		}
	}
}

func TestSetWorkDetailModeRoundTrip(t *testing.T) {
	for _, mode := range []uint8{WorkDetailModeDefault, WorkDetailModeEverybodyDoesThis, WorkDetailModeNobodyDoesThis, WorkDetailModeOnlySelectedDoesThis} {
		orig := &CommandMessage{
			CommandID:         16,
			CommandType:       CommandTypeSetWorkDetailMode,
			SetWorkDetailMode: SetWorkDetailModeDesignation{DetailIndex: 7, Mode: mode},
		}
		data, err := SerializeMessage(orig)
		if err != nil {
			t.Fatalf("mode %d encode: %v", mode, err)
		}
		decoded, err := DeserializeMessage(data)
		if err != nil {
			t.Fatalf("mode %d decode: %v", mode, err)
		}
		got, ok := decoded.(*CommandMessage)
		if !ok {
			t.Fatalf("decoded wrong type %T", decoded)
		}
		if got.CommandID != 16 || got.CommandType != CommandTypeSetWorkDetailMode || got.SetWorkDetailMode != orig.SetWorkDetailMode {
			t.Fatalf("mode %d round-trip mismatch: %+v", mode, got.SetWorkDetailMode)
		}
	}
}

func TestSetWorkDetailModeInvalidModeRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:         16,
		CommandType:       CommandTypeSetWorkDetailMode,
		SetWorkDetailMode: SetWorkDetailModeDesignation{DetailIndex: 0, Mode: WorkDetailModeOnlySelectedDoesThis + 1},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("mode out of range must fail validation")
	}
}

func TestCreateWorkDetailRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   17,
		CommandType: CommandTypeCreateWorkDetail,
		CreateWorkDetail: CreateWorkDetailDesignation{
			Name:     "Ore Haulers",
			Mode:     WorkDetailModeOnlySelectedDoesThis,
			LaborIDs: []uint8{LaborHaulStone, LaborHaulItem},
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
	if got.CommandID != 17 || got.CommandType != CommandTypeCreateWorkDetail ||
		got.CreateWorkDetail.Name != orig.CreateWorkDetail.Name ||
		got.CreateWorkDetail.Mode != orig.CreateWorkDetail.Mode ||
		!reflect.DeepEqual(got.CreateWorkDetail.LaborIDs, orig.CreateWorkDetail.LaborIDs) {
		t.Fatalf("round-trip mismatch: %+v", got.CreateWorkDetail)
	}
}

func TestCreateWorkDetailEmptyNameRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:        17,
		CommandType:      CommandTypeCreateWorkDetail,
		CreateWorkDetail: CreateWorkDetailDesignation{Name: "", Mode: WorkDetailModeDefault},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("empty name must fail validation")
	}
}

func TestCreateWorkDetailNoLaborsRoundTrip(t *testing.T) {
	// LaborIDs may legitimately be empty (a detail created with no
	// pre-enabled labors, edited further later) -- exercise the
	// zero-length trailing list explicitly.
	orig := &CommandMessage{
		CommandID:   17,
		CommandType: CommandTypeCreateWorkDetail,
		CreateWorkDetail: CreateWorkDetailDesignation{
			Name: "Empty Detail",
			Mode: WorkDetailModeDefault,
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
	if len(got.CreateWorkDetail.LaborIDs) != 0 {
		t.Fatalf("expected no labor ids, got: %v", got.CreateWorkDetail.LaborIDs)
	}
}

func TestBringGoodsToDepotRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   18,
		CommandType: CommandTypeBringGoodsToDepot,
		BringGoodsToDepot: BringGoodsToDepotDesignation{
			X: 50, Y: 60, Z: 139,
			ItemTypeFilter: "CRAFTS",
			MaterialFilter: "silver",
			MaxCount:       10,
			MaxTotalValue:  5000,
			ItemClass:      ItemClassCrafts,
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
	if got.CommandID != 18 || got.CommandType != CommandTypeBringGoodsToDepot ||
		got.BringGoodsToDepot != orig.BringGoodsToDepot {
		t.Fatalf("round-trip mismatch: %+v", got.BringGoodsToDepot)
	}
}

// TestBringGoodsToDepotLegacyPayloadWithoutItemClass hand-crafts a
// BRING_GOODS_TO_DEPOT payload ending right after MaxTotalValue — exactly
// what a pre-item_class peer emits — and checks it decodes as ItemClassAny,
// the same EOF-tolerant trailing-field backward-compat rule
// TestBuildLegacyPayloadWithoutMaterialByte establishes for BUILD.
func TestBringGoodsToDepotLegacyPayloadWithoutItemClass(t *testing.T) {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(0)) // length placeholder
	buf.WriteByte(ProtocolVersion)
	buf.WriteByte(MessageTypeCommand)
	_ = binary.Write(&buf, binary.BigEndian, uint32(20)) // CommandID
	buf.WriteByte(CommandTypeBringGoodsToDepot)
	_ = binary.Write(&buf, binary.BigEndian, int16(50))  // X
	_ = binary.Write(&buf, binary.BigEndian, int16(60))  // Y
	_ = binary.Write(&buf, binary.BigEndian, int16(139)) // Z
	itemTypeBytes := []byte("CRAFTS")
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(itemTypeBytes)))
	buf.Write(itemTypeBytes)
	materialBytes := []byte("silver")
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(materialBytes)))
	buf.Write(materialBytes)
	_ = binary.Write(&buf, binary.BigEndian, int32(10))   // MaxCount
	_ = binary.Write(&buf, binary.BigEndian, int64(5000)) // MaxTotalValue
	// no trailing ItemClass byte -- legacy shape
	data := buf.Bytes()
	binary.BigEndian.PutUint32(data[0:4], uint32(len(data)))

	decoded, err := DeserializeMessage(data)
	if err != nil {
		t.Fatalf("decode legacy bring_goods_to_depot payload: %v", err)
	}
	got, ok := decoded.(*CommandMessage)
	if !ok {
		t.Fatalf("decoded wrong type %T", decoded)
	}
	if got.CommandID != 20 || got.BringGoodsToDepot.ItemClass != ItemClassAny {
		t.Fatalf("legacy payload must decode ItemClass as ItemClassAny: %+v", got.BringGoodsToDepot)
	}
}

func TestBringGoodsToDepotEmptyFiltersRoundTrip(t *testing.T) {
	// Empty ItemTypeFilter/MaterialFilter ("" = no filter) must round-trip
	// as zero-length strings, not panic on a nil-length read.
	orig := &CommandMessage{
		CommandID:   18,
		CommandType: CommandTypeBringGoodsToDepot,
		BringGoodsToDepot: BringGoodsToDepotDesignation{
			X: 1, Y: 2, Z: 3,
			MaxCount:      9999,
			MaxTotalValue: 0, // no value cap
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
	if got.BringGoodsToDepot.ItemTypeFilter != "" || got.BringGoodsToDepot.MaterialFilter != "" {
		t.Fatalf("expected empty filters, got: %+v", got.BringGoodsToDepot)
	}
	if got.BringGoodsToDepot.MaxCount != 9999 || got.BringGoodsToDepot.MaxTotalValue != 0 {
		t.Fatalf("round-trip mismatch: %+v", got.BringGoodsToDepot)
	}
}

func TestBringGoodsToDepotMaxCountZeroRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:         18,
		CommandType:       CommandTypeBringGoodsToDepot,
		BringGoodsToDepot: BringGoodsToDepotDesignation{X: 1, Y: 2, Z: 3, MaxCount: 0},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("MaxCount == 0 must fail validation")
	}
}

func TestAppointPositionRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   19,
		CommandType: CommandTypeAppointPosition,
		AppointPosition: AppointPositionDesignation{
			UnitID:       42,
			PositionCode: "BOOKKEEPER",
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
	if got.CommandID != 19 || got.CommandType != CommandTypeAppointPosition ||
		got.AppointPosition != orig.AppointPosition {
		t.Fatalf("round-trip mismatch: %+v", got.AppointPosition)
	}
}

func TestAppointPositionEmptyCodeRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:       19,
		CommandType:     CommandTypeAppointPosition,
		AppointPosition: AppointPositionDesignation{UnitID: 42, PositionCode: ""},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("empty PositionCode must fail validation")
	}
}

func TestSetBookkeeperPrecisionRoundTrip(t *testing.T) {
	for _, precision := range []uint8{
		BookkeeperPrecisionNearest10, BookkeeperPrecisionNearest100,
		BookkeeperPrecisionNearest1000, BookkeeperPrecisionNearest10000,
		BookkeeperPrecisionAllAccurate,
	} {
		orig := &CommandMessage{
			CommandID:              20,
			CommandType:            CommandTypeSetBookkeeperPrecision,
			SetBookkeeperPrecision: SetBookkeeperPrecisionDesignation{Precision: precision},
		}
		data, err := SerializeMessage(orig)
		if err != nil {
			t.Fatalf("precision %d encode: %v", precision, err)
		}
		decoded, err := DeserializeMessage(data)
		if err != nil {
			t.Fatalf("precision %d decode: %v", precision, err)
		}
		got, ok := decoded.(*CommandMessage)
		if !ok {
			t.Fatalf("decoded wrong type %T", decoded)
		}
		if got.CommandID != 20 || got.CommandType != CommandTypeSetBookkeeperPrecision ||
			got.SetBookkeeperPrecision != orig.SetBookkeeperPrecision {
			t.Fatalf("precision %d round-trip mismatch: %+v", precision, got.SetBookkeeperPrecision)
		}
	}
}

func TestSetBookkeeperPrecisionInvalidRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:              20,
		CommandType:            CommandTypeSetBookkeeperPrecision,
		SetBookkeeperPrecision: SetBookkeeperPrecisionDesignation{Precision: BookkeeperPrecisionAllAccurate + 1},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("precision out of range must fail validation")
	}
}

func TestCreateSquadRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   21,
		CommandType: CommandTypeCreateSquad,
		CreateSquad: CreateSquadDesignation{PositionCode: "MILITIA_CAPTAIN"},
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
	if got.CommandID != 21 || got.CommandType != CommandTypeCreateSquad || got.CreateSquad != orig.CreateSquad {
		t.Fatalf("round-trip mismatch: %+v", got.CreateSquad)
	}
}

func TestCreateSquadEmptyCodeRoundTrip(t *testing.T) {
	// PositionCode == "" is legal and meaningful on the wire: it is how the
	// caller asks the plugin to auto-select a squad-leader position (it is
	// NOT rewritten to "MILITIA_CAPTAIN" any more, on either side). So it
	// must round-trip as a zero-length string, not panic on a nil-length
	// read and not acquire a default here.
	orig := &CommandMessage{
		CommandID:   22,
		CommandType: CommandTypeCreateSquad,
		CreateSquad: CreateSquadDesignation{PositionCode: ""},
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
	if got.CreateSquad.PositionCode != "" {
		t.Fatalf("expected empty position code, got: %+v", got.CreateSquad)
	}
}

func TestAssignSquadRoundTrip(t *testing.T) {
	for _, add := range []bool{true, false} {
		orig := &CommandMessage{
			CommandID:   23,
			CommandType: CommandTypeAssignSquad,
			AssignSquad: AssignSquadDesignation{SquadID: 3, UnitID: 42, Add: add},
		}
		data, err := SerializeMessage(orig)
		if err != nil {
			t.Fatalf("add=%v encode: %v", add, err)
		}
		decoded, err := DeserializeMessage(data)
		if err != nil {
			t.Fatalf("add=%v decode: %v", add, err)
		}
		got, ok := decoded.(*CommandMessage)
		if !ok {
			t.Fatalf("decoded wrong type %T", decoded)
		}
		if got.CommandID != 23 || got.CommandType != CommandTypeAssignSquad || got.AssignSquad != orig.AssignSquad {
			t.Fatalf("add=%v round-trip mismatch: %+v", add, got.AssignSquad)
		}
	}
}

func TestSquadOrderStationRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   24,
		CommandType: CommandTypeSquadOrder,
		SquadOrder:  SquadOrderDesignation{SquadID: 3, Type: SquadOrderStation, X: 10, Y: 20, Z: 90},
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
	if got.CommandID != 24 || got.CommandType != CommandTypeSquadOrder || got.SquadOrder != orig.SquadOrder {
		t.Fatalf("round-trip mismatch: %+v", got.SquadOrder)
	}
}

func TestSquadOrderDefendBurrowRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   25,
		CommandType: CommandTypeSquadOrder,
		SquadOrder:  SquadOrderDesignation{SquadID: 3, Type: SquadOrderDefendBurrow, BurrowName: "chokepoint"},
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
	if got.CommandID != 25 || got.CommandType != CommandTypeSquadOrder || got.SquadOrder != orig.SquadOrder {
		t.Fatalf("round-trip mismatch: %+v", got.SquadOrder)
	}
}

func TestSquadOrderCancelRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   26,
		CommandType: CommandTypeSquadOrder,
		SquadOrder:  SquadOrderDesignation{SquadID: 3, Type: SquadOrderCancel},
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
	if got.CommandID != 26 || got.CommandType != CommandTypeSquadOrder || got.SquadOrder != orig.SquadOrder {
		t.Fatalf("round-trip mismatch: %+v", got.SquadOrder)
	}
}

func TestSquadOrderInvalidTypeRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   27,
		CommandType: CommandTypeSquadOrder,
		SquadOrder:  SquadOrderDesignation{SquadID: 3, Type: SquadOrderCancel + 1},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("squad order type out of range must fail validation")
	}
}

func TestSquadOrderDefendBurrowEmptyNameRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   28,
		CommandType: CommandTypeSquadOrder,
		SquadOrder:  SquadOrderDesignation{SquadID: 3, Type: SquadOrderDefendBurrow, BurrowName: ""},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("defend_burrow with an empty burrow name must fail validation")
	}
}

func TestCancelOrderRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   29,
		CommandType: CommandTypeCancelOrder,
		CancelOrder: CancelOrderDesignation{OrderID: 7},
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
	if got.CommandID != 29 || got.CommandType != CommandTypeCancelOrder || got.CancelOrder != orig.CancelOrder {
		t.Fatalf("round-trip mismatch: %+v", got.CancelOrder)
	}
}

func TestCancelOrderNegativeIDRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   30,
		CommandType: CommandTypeCancelOrder,
		CancelOrder: CancelOrderDesignation{OrderID: -1},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("negative order id must fail validation")
	}
}

func TestEditOrderAmountOnlyRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   31,
		CommandType: CommandTypeEditOrder,
		EditOrder:   EditOrderDesignation{OrderID: 7, HasAmount: true, Amount: 10},
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
	if got.CommandID != 31 || got.CommandType != CommandTypeEditOrder || got.EditOrder != orig.EditOrder {
		t.Fatalf("round-trip mismatch: %+v", got.EditOrder)
	}
}

func TestEditOrderFrequencyOnlyRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   32,
		CommandType: CommandTypeEditOrder,
		EditOrder:   EditOrderDesignation{OrderID: 7, HasFrequency: true, Frequency: WorkOrderFrequencyMonthly},
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
	if got.CommandID != 32 || got.CommandType != CommandTypeEditOrder || got.EditOrder != orig.EditOrder {
		t.Fatalf("round-trip mismatch: %+v", got.EditOrder)
	}
}

func TestEditOrderBothFieldsRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   33,
		CommandType: CommandTypeEditOrder,
		EditOrder: EditOrderDesignation{
			OrderID: 7, HasAmount: true, Amount: 25,
			HasFrequency: true, Frequency: WorkOrderFrequencyYearly,
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
	if got.CommandID != 33 || got.CommandType != CommandTypeEditOrder || got.EditOrder != orig.EditOrder {
		t.Fatalf("round-trip mismatch: %+v", got.EditOrder)
	}
}

func TestEditOrderNeitherFieldRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   34,
		CommandType: CommandTypeEditOrder,
		EditOrder:   EditOrderDesignation{OrderID: 7},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("edit_order with neither amount nor frequency set must fail validation")
	}
}

func TestEditOrderAmountOutOfRangeRejected(t *testing.T) {
	for _, amount := range []uint16{0, 101} {
		orig := &CommandMessage{
			CommandID:   35,
			CommandType: CommandTypeEditOrder,
			EditOrder:   EditOrderDesignation{OrderID: 7, HasAmount: true, Amount: amount},
		}
		if _, err := SerializeMessage(orig); err == nil {
			t.Fatalf("amount=%d out of range must fail validation", amount)
		}
	}
}

func TestEditOrderInvalidFrequencyRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   36,
		CommandType: CommandTypeEditOrder,
		EditOrder:   EditOrderDesignation{OrderID: 7, HasFrequency: true, Frequency: WorkOrderFrequencyYearly + 1},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("frequency out of range must fail validation")
	}
}

func TestUnmarkTradeGoodsRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   37,
		CommandType: CommandTypeUnmarkTradeGoods,
		UnmarkTradeGoods: UnmarkTradeGoodsDesignation{
			X: 50, Y: 60, Z: 139,
			ItemTypeFilter: "FIGURINE",
			MaterialFilter: "shell",
			ItemClass:      ItemClassCrafts,
			MaxCount:       5,
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
	if got.CommandID != 37 || got.CommandType != CommandTypeUnmarkTradeGoods ||
		got.UnmarkTradeGoods != orig.UnmarkTradeGoods {
		t.Fatalf("round-trip mismatch: %+v", got.UnmarkTradeGoods)
	}
}

func TestUnmarkTradeGoodsEmptyFiltersRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:        38,
		CommandType:      CommandTypeUnmarkTradeGoods,
		UnmarkTradeGoods: UnmarkTradeGoodsDesignation{X: 1, Y: 2, Z: 3, MaxCount: 9999},
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
	if got.UnmarkTradeGoods.ItemTypeFilter != "" || got.UnmarkTradeGoods.MaterialFilter != "" {
		t.Fatalf("expected empty filters, got: %+v", got.UnmarkTradeGoods)
	}
	if got.UnmarkTradeGoods.ItemClass != ItemClassAny || got.UnmarkTradeGoods.MaxCount != 9999 {
		t.Fatalf("round-trip mismatch: %+v", got.UnmarkTradeGoods)
	}
}

func TestUnmarkTradeGoodsMaxCountZeroRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:        39,
		CommandType:      CommandTypeUnmarkTradeGoods,
		UnmarkTradeGoods: UnmarkTradeGoodsDesignation{X: 1, Y: 2, Z: 3, MaxCount: 0},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("MaxCount == 0 must fail validation")
	}
}

func TestUnmarkTradeGoodsInvalidItemClassRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   40,
		CommandType: CommandTypeUnmarkTradeGoods,
		UnmarkTradeGoods: UnmarkTradeGoodsDesignation{
			X: 1, Y: 2, Z: 3, MaxCount: 1, ItemClass: ItemClassCrafts + 1,
		},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("invalid item class must fail validation")
	}
}

func TestBringGoodsToDepotInvalidItemClassRejected(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   41,
		CommandType: CommandTypeBringGoodsToDepot,
		BringGoodsToDepot: BringGoodsToDepotDesignation{
			X: 1, Y: 2, Z: 3, MaxCount: 1, ItemClass: ItemClassCrafts + 1,
		},
	}
	if _, err := SerializeMessage(orig); err == nil {
		t.Fatal("invalid item class must fail validation")
	}
}

func TestSetDepotTradeFlagsRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   42,
		CommandType: CommandTypeSetDepotTradeFlags,
		SetDepotTradeFlags: SetDepotTradeFlagsDesignation{
			X: 28, Y: 52, Z: 110,
			TraderRequested: true,
			AnyoneCanTrade:  false,
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
	if got.CommandID != 42 || got.CommandType != CommandTypeSetDepotTradeFlags ||
		got.SetDepotTradeFlags != orig.SetDepotTradeFlags {
		t.Fatalf("round-trip mismatch: %+v", got.SetDepotTradeFlags)
	}
}

func TestSetDepotTradeFlagsBothFalseRoundTrip(t *testing.T) {
	// Both bools false is a legitimate, distinct state from an unset/omitted
	// payload — confirm it round-trips rather than being confused with a
	// zero-value/absent-field sentinel (there is none here; both fields are
	// always encoded).
	orig := &CommandMessage{
		CommandID:   43,
		CommandType: CommandTypeSetDepotTradeFlags,
		SetDepotTradeFlags: SetDepotTradeFlagsDesignation{
			X: 1, Y: 2, Z: 3,
			TraderRequested: false,
			AnyoneCanTrade:  false,
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
	if got.SetDepotTradeFlags.TraderRequested || got.SetDepotTradeFlags.AnyoneCanTrade {
		t.Fatalf("expected both flags false, got: %+v", got.SetDepotTradeFlags)
	}
}
