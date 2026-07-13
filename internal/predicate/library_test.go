package predicate

import (
	"testing"
	"time"

	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// dwarfEntities returns n synthetic dwarf entities for test fixtures.
func dwarfEntities(n int) []protocol.EntityInfo {
	out := make([]protocol.EntityInfo, n)
	for i := range out {
		out[i] = protocol.EntityInfo{ID: uint32(i + 1), Type: protocol.EntityTypeDwarf}
	}
	return out
}

// animalEntities returns n synthetic animal entities for test fixtures.
func animalEntities(n int) []protocol.EntityInfo {
	out := make([]protocol.EntityInfo, n)
	for i := range out {
		out[i] = protocol.EntityInfo{ID: uint32(1000 + i), Type: protocol.EntityTypeAnimal}
	}
	return out
}

// TestHasMinDwarves_IgnoresAnimals guards the 2026-07-12 census bug from the
// predicate side: HasMinDwarves must count only worldmodel.EntitySnapshot.
// Dwarves, never the animal population that used to leak in when the plugin
// mis-tagged tame animals as ENTITY_TYPE_DWARF (dfhack-plugin/entities.cpp).
// A pre-fix world with 7 real dwarves + 11 animals would have reported 18
// and satisfied a Min:8 threshold it should not have.
func TestHasMinDwarves_IgnoresAnimals(t *testing.T) {
	wm := worldmodel.New(nil, nil, nil, nil)
	entities := append(dwarfEntities(7), animalEntities(11)...)
	wm.Observed.Entities = worldmodel.EntitySnapshot{
		All:       entities,
		Dwarves:   dwarfEntities(7),
		Animals:   animalEntities(11),
		UpdatedAt: time.Now(),
	}

	p := HasMinDwarves{Min: 8, H: HorizonNow}
	r := p.Check(wm)

	if r.Satisfied {
		t.Fatalf("HasMinDwarves{Min:8} satisfied with only 7 real dwarves (11 animals must not count); evidence=%v", r.Evidence)
	}

	p2 := HasMinDwarves{Min: 7, H: HorizonNow}
	r2 := p2.Check(wm)
	if !r2.Satisfied {
		t.Fatalf("HasMinDwarves{Min:7} unsatisfied with exactly 7 real dwarves; evidence=%v", r2.Evidence)
	}
}

// TestHasShelter_DividesByCitizenCountOnly guards the exact regression
// reported live: has_shelter_N_per_dwarf must divide required tiles by the
// dwarf-only count, not an animal-inflated total, or the requirement scales
// up incorrectly and the predicate reports false shortfalls.
func TestHasShelter_DividesByCitizenCountOnly(t *testing.T) {
	wm := worldmodel.New(nil, nil, nil, nil)
	entities := append(dwarfEntities(7), animalEntities(11)...)
	wm.Observed.Entities = worldmodel.EntitySnapshot{
		All:       entities,
		Dwarves:   dwarfEntities(7),
		Animals:   animalEntities(11),
		UpdatedAt: time.Now(),
	}

	p := HasShelter{MinDugTilesPerDwarf: 6, H: HorizonNow}
	r := p.Check(wm)

	wantRequired := "required (6/dwarf): 42" // 7 dwarves * 6, NOT 18 * 6 = 108
	found := false
	for _, e := range r.Evidence {
		if e == wantRequired {
			found = true
		}
	}
	if !found {
		t.Fatalf("HasShelter evidence missing %q (dwarf-only requirement); got %v", wantRequired, r.Evidence)
	}
}

// TestHasBedroomZones_UsesCorrectedWireValue guards against HasBedroomZones
// drifting from protocol.ZoneTypeBedroom via a coincidental local magic
// number: a Pen zone (also Roster-assigned, unconfirmed-adjacent) must never
// count toward the bedroom threshold.
func TestHasBedroomZones_UsesCorrectedWireValue(t *testing.T) {
	wm := worldmodel.New(nil, nil, nil, nil)
	wm.Observed.Zones = worldmodel.ZoneSnapshot{
		All: []protocol.ZoneData{
			{ZoneType: protocol.ZoneTypeBedroom, OwnerUnitID: 1},
			{ZoneType: protocol.ZoneTypeBedroom, OwnerUnitID: -1},
			{ZoneType: protocol.ZoneTypePen, OwnerUnitID: -1}, // must NOT count
		},
		UpdatedAt: time.Now(),
	}
	p := HasBedroomZones{Min: 2, H: HorizonSoon}
	r := p.Check(wm)
	if !r.Satisfied {
		t.Fatalf("expected satisfied with 2 bedroom zones, got %+v", r)
	}
}

// TestHasDiningHall_UsesCorrectedWireValue guards against HasDiningHall
// drifting from protocol.ZoneTypeDiningHall via a coincidental local magic
// number.
func TestHasDiningHall_UsesCorrectedWireValue(t *testing.T) {
	wm := worldmodel.New(nil, nil, nil, nil)
	wm.Observed.Zones = worldmodel.ZoneSnapshot{
		All: []protocol.ZoneData{
			{ZoneType: protocol.ZoneTypeDiningHall},
		},
		UpdatedAt: time.Now(),
	}
	p := HasDiningHall{H: HorizonSoon}
	r := p.Check(wm)
	if !r.Satisfied {
		t.Fatalf("expected satisfied with 1 dining hall, got %+v", r)
	}
}
