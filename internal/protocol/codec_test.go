package protocol

import (
	"bytes"
	"testing"
)

func TestZoneTypeConstants_MatchThePlanTable(t *testing.T) {
	cases := map[uint8]uint8{
		ZoneTypeBedroom: 0x01, ZoneTypeOffice: 0x02, ZoneTypeTomb: 0x03,
		ZoneTypeDiningHall: 0x04, ZoneTypeMeetingHall: 0x05, ZoneTypeDormitory: 0x06,
		ZoneTypeBarracks: 0x07, ZoneTypePen: 0x08, ZoneTypePond: 0x09,
		ZoneTypeArcheryRange: 0x0A, ZoneTypePlantGathering: 0x0B, ZoneTypeWaterSource: 0x0C,
		ZoneTypeDump: 0x0D, ZoneTypeSandCollection: 0x0E, ZoneTypeFishingArea: 0x0F,
		ZoneTypeClayCollection: 0x10, ZoneTypeDungeon: 0x11, ZoneTypeAnimalTraining: 0x12,
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("constant value mismatch: got 0x%02X, want 0x%02X", got, want)
		}
	}
}

func TestEncodeDecodeAssignZoneCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   7,
		CommandType: CommandTypeAssignZone,
		AssignZone:  AssignZoneDesignation{X: 10, Y: 20, Z: 90, UnitID: 42},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.AssignZone != msg.AssignZone {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.AssignZone, msg.AssignZone)
	}
}

func TestEncodeDecodeUnassignZoneCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:    8,
		CommandType:  CommandTypeUnassignZone,
		UnassignZone: UnassignZoneDesignation{X: 10, Y: 20, Z: 90, UnitID: 42},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.UnassignZone != msg.UnassignZone {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.UnassignZone, msg.UnassignZone)
	}
}

func TestEntityUpdateMessage_ZonesRoundTrip(t *testing.T) {
	msg := &EntityUpdateMessage{
		Count:    0,
		Entities: nil,
		Zones: []ZoneData{
			{ZoneID: 99, ZoneType: ZoneTypeBedroom, X1: 1, Y1: 1, Z1: 90, X2: 2, Y2: 2, Z2: 90, OwnerUnitID: 42, AssignedUnits: nil},
			{ZoneID: 100, ZoneType: ZoneTypePen, X1: 5, Y1: 5, Z1: 90, X2: 8, Y2: 8, Z2: 90, OwnerUnitID: -1, AssignedUnits: []int32{1, 2, 3}},
		},
	}
	var buf bytes.Buffer
	if err := serializeEntityUpdate(&buf, msg); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	decoded, err := deserializeEntityUpdate(buf.Bytes())
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if len(decoded.Zones) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(decoded.Zones))
	}
	z0 := decoded.Zones[0]
	if z0.ZoneID != 99 || z0.ZoneType != ZoneTypeBedroom || z0.X1 != 1 || z0.Y1 != 1 ||
		z0.X2 != 2 || z0.Y2 != 2 || z0.Z1 != 90 || z0.Z2 != 90 || z0.OwnerUnitID != 42 {
		t.Fatalf("zone 0 mismatch: got %+v", z0)
	}
	if len(z0.AssignedUnits) != 0 {
		t.Fatalf("expected zone 0 to have no assigned units, got %v", z0.AssignedUnits)
	}
	z1 := decoded.Zones[1]
	if len(z1.AssignedUnits) != 3 || z1.AssignedUnits[0] != 1 || z1.AssignedUnits[1] != 2 || z1.AssignedUnits[2] != 3 {
		t.Fatalf("expected zone 1 assigned units [1,2,3], got %v", z1.AssignedUnits)
	}
}

func TestEntityUpdateMessage_NoZonesDecodesEmpty(t *testing.T) {
	msg := &EntityUpdateMessage{Count: 0, Entities: nil, Zones: nil}
	var buf bytes.Buffer
	if err := serializeEntityUpdate(&buf, msg); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	decoded, err := deserializeEntityUpdate(buf.Bytes())
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if len(decoded.Zones) != 0 {
		t.Fatalf("expected 0 zones, got %d", len(decoded.Zones))
	}
}

// TestEntityUpdateMessage_DeadUnitsRoundTrip mirrors the Zones round-trip
// test above: the DeadUnits block follows the exact same additive pattern
// (see dfhack-plugin/entities.cpp's serialize_entity_update and this
// package's deserializeEntityUpdate).
func TestEntityUpdateMessage_DeadUnitsRoundTrip(t *testing.T) {
	msg := &EntityUpdateMessage{
		Count: 3,
		Entities: []EntityInfo{
			{ID: 1, X: 10, Y: 10, Z: 90, Type: EntityTypeDwarf},
			{ID: 2, X: 11, Y: 11, Z: 90, Type: EntityTypeDwarf, Dead: true},
			{ID: 3, X: 12, Y: 12, Z: 90, Type: EntityTypeAnimal},
		},
	}
	var buf bytes.Buffer
	if err := serializeEntityUpdate(&buf, msg); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	decoded, err := deserializeEntityUpdate(buf.Bytes())
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if len(decoded.Entities) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(decoded.Entities))
	}
	if decoded.Entities[0].Dead {
		t.Fatalf("entity 1 must decode alive: %+v", decoded.Entities[0])
	}
	if !decoded.Entities[1].Dead {
		t.Fatalf("entity 2 must decode dead: %+v", decoded.Entities[1])
	}
	if decoded.Entities[2].Dead {
		t.Fatalf("entity 3 must decode alive: %+v", decoded.Entities[2])
	}
}

// TestEntityUpdateMessage_NoDeadUnitsBlockDecodesAllAlive simulates an old
// peer's payload (no trailing DeadUnits block at all -- built by hand,
// trimming what a real old sender would have omitted) to confirm the
// backward-compatible default: every entity decodes Dead=false, never an
// error.
func TestEntityUpdateMessage_NoDeadUnitsBlockDecodesAllAlive(t *testing.T) {
	msg := &EntityUpdateMessage{
		Count:    1,
		Entities: []EntityInfo{{ID: 5, X: 1, Y: 1, Z: 1, Type: EntityTypeDwarf}},
	}
	var full bytes.Buffer
	if err := serializeEntityUpdate(&full, msg); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	// Old-format payload = entities + HasFortInfo(0) + HasZones(1) +
	// ZoneCount(0), with everything after that (the DeadUnits block)
	// truncated off, as an old plugin build would never have written it.
	// [4:Count]=4 bytes, [entity]=13 bytes, [HasFortInfo]=1,
	// [HasZones]=1, [ZoneCount]=4 -> 23 bytes total.
	oldFormat := full.Bytes()[:23]
	decoded, err := deserializeEntityUpdate(oldFormat)
	if err != nil {
		t.Fatalf("deserialize truncated (old-format) payload: %v", err)
	}
	if len(decoded.Entities) != 1 || decoded.Entities[0].Dead {
		t.Fatalf("old-format payload must decode the entity alive: %+v", decoded.Entities)
	}
}

// TestEntityUpdateMessage_WorldIdentityRoundTrip mirrors the Zones/DeadUnits
// round-trip tests above: the WorldIdentity block is the newest additive
// trailing block (see dfhack-plugin/entities.cpp's serialize_entity_update
// and this package's deserializeEntityUpdate).
func TestEntityUpdateMessage_WorldIdentityRoundTrip(t *testing.T) {
	msg := &EntityUpdateMessage{
		Count:    0,
		Entities: nil,
		World:    &WorldIdentity{SaveDir: "region2", ID1: 123456, ID2: 789012},
	}
	var buf bytes.Buffer
	if err := serializeEntityUpdate(&buf, msg); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	decoded, err := deserializeEntityUpdate(buf.Bytes())
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if decoded.World == nil {
		t.Fatalf("expected a decoded World identity, got nil")
	}
	if *decoded.World != *msg.World {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", *decoded.World, *msg.World)
	}
}

// TestEntityUpdateMessage_NoWorldIdentityDecodesNil simulates an old peer's
// payload (no trailing WorldIdentity block at all) to confirm the
// backward-compatible default: World stays nil, never an error.
func TestEntityUpdateMessage_NoWorldIdentityDecodesNil(t *testing.T) {
	msg := &EntityUpdateMessage{Count: 0, Entities: nil}
	var buf bytes.Buffer
	if err := serializeEntityUpdate(&buf, msg); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	decoded, err := deserializeEntityUpdate(buf.Bytes())
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if decoded.World != nil {
		t.Fatalf("expected nil World when the message never set one, got %+v", decoded.World)
	}
}

func TestLocationTypeConstants_MatchThePlanTable(t *testing.T) {
	cases := map[uint8]uint8{
		LocationTypeTavern: 0x01, LocationTypeTemple: 0x02,
		LocationTypeLibrary: 0x03, LocationTypeGuildhall: 0x04,
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("constant value mismatch: got 0x%02X, want 0x%02X", got, want)
		}
	}
}

func TestEncodeDecodeCreateLocationCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:      1,
		CommandType:    CommandTypeCreateLocation,
		CreateLocation: CreateLocationDesignation{X: 10, Y: 20, Z: 90, LocationType: LocationTypeGuildhall, Profession: "CARPENTER"},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.CreateLocation != msg.CreateLocation {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.CreateLocation, msg.CreateLocation)
	}
}

func TestEncodeDecodeCreateLocationCommand_NoProfession(t *testing.T) {
	msg := &CommandMessage{
		CommandID:      2,
		CommandType:    CommandTypeCreateLocation,
		CreateLocation: CreateLocationDesignation{X: 5, Y: 5, Z: 90, LocationType: LocationTypeTavern, Profession: ""},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.CreateLocation.Profession != "" {
		t.Fatalf("expected empty profession, got %q", decoded.CreateLocation.Profession)
	}
}

func TestEncodeDecodeAssignLodgingCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:     3,
		CommandType:   CommandTypeAssignLodging,
		AssignLodging: AssignLodgingDesignation{TavernX: 1, TavernY: 2, TavernZ: 3, BedroomX: 4, BedroomY: 5, BedroomZ: 6},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.AssignLodging != msg.AssignLodging {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.AssignLodging, msg.AssignLodging)
	}
}

func TestEncodeDecodeUnassignLodgingCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:       4,
		CommandType:     CommandTypeUnassignLodging,
		UnassignLodging: UnassignLodgingDesignation{BedroomX: 4, BedroomY: 5, BedroomZ: 6},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.UnassignLodging != msg.UnassignLodging {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.UnassignLodging, msg.UnassignLodging)
	}
}

func TestEncodeDecodeBuildFarmPlotCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   5,
		CommandType: CommandTypeBuildFarmPlot,
		FarmPlot:    FarmPlotDesignation{X1: 10, Y1: 20, Z: 90, X2: 13, Y2: 22},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.FarmPlot != msg.FarmPlot {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.FarmPlot, msg.FarmPlot)
	}
}

func TestEncodeDecodeBuildFarmPlotCommand_InvalidRegionRejected(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   6,
		CommandType: CommandTypeBuildFarmPlot,
		FarmPlot:    FarmPlotDesignation{X1: 13, Y1: 20, Z: 90, X2: 10, Y2: 22},
	}
	if err := msg.Validate(); err == nil {
		t.Fatal("expected Validate to reject X2 < X1, got nil")
	}
}

func TestEncodeDecodeSetFarmCropCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   7,
		CommandType: CommandTypeSetFarmCrop,
		SetFarmCrop: SetFarmCropDesignation{X: 11, Y: 21, Z: 90, Season: FarmSeasonSpring, CropName: "plump_helmet"},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.SetFarmCrop != msg.SetFarmCrop {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.SetFarmCrop, msg.SetFarmCrop)
	}
}

func TestEncodeDecodeSetFarmCropCommand_Fallow(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   8,
		CommandType: CommandTypeSetFarmCrop,
		SetFarmCrop: SetFarmCropDesignation{X: 11, Y: 21, Z: 90, Season: FarmSeasonWinter, CropName: "fallow"},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.SetFarmCrop != msg.SetFarmCrop {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.SetFarmCrop, msg.SetFarmCrop)
	}
}

func TestEncodeDecodeSetFarmCropCommand_AllSeasons(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   9,
		CommandType: CommandTypeSetFarmCrop,
		SetFarmCrop: SetFarmCropDesignation{X: 11, Y: 21, Z: 90, Season: FarmSeasonAll, CropName: "Plump Helmet"},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.SetFarmCrop != msg.SetFarmCrop {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.SetFarmCrop, msg.SetFarmCrop)
	}
}

func TestEncodeDecodeSetFarmCropCommand_InvalidSeasonRejected(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   10,
		CommandType: CommandTypeSetFarmCrop,
		SetFarmCrop: SetFarmCropDesignation{X: 11, Y: 21, Z: 90, Season: 4, CropName: "plump_helmet"},
	}
	if err := msg.Validate(); err == nil {
		t.Fatal("expected Validate to reject season byte 4, got nil")
	}
}

func TestEncodeDecodeSetFarmCropCommand_EmptyCropNameRejected(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   11,
		CommandType: CommandTypeSetFarmCrop,
		SetFarmCrop: SetFarmCropDesignation{X: 11, Y: 21, Z: 90, Season: FarmSeasonSpring, CropName: ""},
	}
	if err := msg.Validate(); err == nil {
		t.Fatal("expected Validate to reject empty CropName, got nil")
	}
}
