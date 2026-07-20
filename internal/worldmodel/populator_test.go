package worldmodel

import (
	"testing"
	"time"

	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

// TestBuildEntitySnapshot_PartitionsByType locks in the dwarf/animal census
// split that internal/predicate (HasMinDwarves, HasShelter) and the
// dwarves/dwarf_detail MCP tools all trust blindly via len(snap.Entities.
// Dwarves). The 2026-07-12 census bug lived entirely on the plugin side
// (dfhack-plugin/entities.cpp mis-tagging tame animals as ENTITY_TYPE_DWARF
// via Units::isFortControlled instead of Units::isAnimal) — this test
// exists to make sure the Go partitioning itself, which was already
// correct, stays correct: any entity tagged EntityTypeAnimal by the wire
// protocol must never leak into Dwarves, and vice versa.
func TestBuildEntitySnapshot_PartitionsByType(t *testing.T) {
	entities := []protocol.EntityInfo{
		{ID: 1, Type: protocol.EntityTypeDwarf, Subtype: 1},
		{ID: 2, Type: protocol.EntityTypeDwarf, Subtype: 1},
		{ID: 3, Type: protocol.EntityTypeAnimal, Subtype: 99}, // tame pack animal
		{ID: 4, Type: protocol.EntityTypeAnimal, Subtype: 99}, // pet
		{ID: 5, Type: protocol.EntityTypeEnemy, Subtype: 5},
		{ID: 6, Type: protocol.EntityTypeOther, Subtype: 2}, // merchant/visitor
	}

	now := time.Now()
	snap := buildEntitySnapshot(entities, now, 42)

	if got, want := len(snap.All), 6; got != want {
		t.Fatalf("All: got %d entities, want %d", got, want)
	}
	if got, want := len(snap.Dwarves), 2; got != want {
		t.Fatalf("Dwarves: got %d, want %d (animals must not be counted as dwarves)", got, want)
	}
	if got, want := len(snap.Enemies), 1; got != want {
		t.Fatalf("Enemies: got %d, want %d", got, want)
	}
	if got, want := len(snap.Animals), 2; got != want {
		t.Fatalf("Animals: got %d, want %d", got, want)
	}

	for _, e := range snap.Dwarves {
		if e.Type != protocol.EntityTypeDwarf {
			t.Errorf("Dwarves contains non-dwarf entity id=%d type=%d", e.ID, e.Type)
		}
	}
	for _, e := range snap.Animals {
		if e.Type != protocol.EntityTypeAnimal {
			t.Errorf("Animals contains non-animal entity id=%d type=%d", e.ID, e.Type)
		}
	}

	if snap.EventSeq != 42 {
		t.Errorf("EventSeq: got %d, want 42", snap.EventSeq)
	}
	if !snap.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt: got %v, want %v", snap.UpdatedAt, now)
	}
}

// TestBuildEntitySnapshot_Empty guards the zero-entity path (fort not yet
// observed) doesn't panic and yields nil/empty slices rather than a
// populated-looking snapshot.
func TestBuildEntitySnapshot_Empty(t *testing.T) {
	snap := buildEntitySnapshot(nil, time.Now(), 0)
	if len(snap.All) != 0 || len(snap.Dwarves) != 0 || len(snap.Enemies) != 0 || len(snap.Animals) != 0 {
		t.Fatalf("expected all-empty snapshot for nil input, got %+v", snap)
	}
}

// --- find_dig_site "stale solidity" regression (2026-07-12) ---
//
// Root cause: dfhack-plugin/tile_updates.cpp's detect_tile_changes() used to
// hardcode `uint8_t flags = 0x02` (FLAG_DISCOVERED only) for every changed
// tile in a TILE_UPDATE delta, regardless of the tile's actual wall/floor/
// liquid state. OnTileUpdate below feeds those flags straight into
// topology.ClassifyState + TopologyOverlay.SetTileState with no live
// requery — Go only ever sees what the wire told it. ClassifyState(0x02)
// has none of FlagFloor/FlagWall/FlagVoid/FlagLiquid7 set, so it always
// fell through to StateUnknown, and mapview.FindDigSites' solidity scan
// treats Unknown identically to Closed ("default: solid" — see finder.go,
// unknown/hidden ground is diggable mass by design). Net effect: a
// freshly-dug room's TILE_UPDATE landed as "unknown", which read right back
// out as "fully solid" — find_dig_site handed back the exact footprint of a
// room that had already been carved.
//
// The fix (commit 062455f, dfhack-plugin/tile_updates.cpp) makes the delta
// path call the same compute_tile_flags() the full-state extractor uses, so
// TILE_UPDATE deltas carry real classification bits. That fix lives in
// plugin code that requires a live DFHack/MapCache and has no Go-testable
// pure function — this test instead locks in the Go-side half of the
// contract: given the flags a *fixed* plugin sends, OnTileUpdate +
// FindDigSites must not re-offer a just-dug floor as a dig candidate. The
// second test documents the failure mode itself (degraded flags in ==
// false-solid candidate out) so a future change can't silently reintroduce
// it from the Go side (e.g. a default/placeholder flags value on some new
// ingestion path).

// buildSolidLevel returns a topology overlay where every tile at every Z is
// a discovered solid wall — a stand-in FULL_STATE baseline before any
// digging has happened.
func buildSolidLevel(t *testing.T, width, height, depth uint16) *topology.TopologyOverlay {
	t.Helper()
	topo := topology.NewTopologyOverlay(width, height, depth)
	if topo == nil {
		t.Fatal("overlay nil")
	}
	var tiles []protocol.TileState
	for z := int16(0); z < int16(depth); z++ {
		for y := int16(0); y < int16(height); y++ {
			for x := int16(0); x < int16(width); x++ {
				tiles = append(tiles, protocol.TileState{
					X: x, Y: y, Z: z, TileType: 600,
					Flags: protocol.FlagDiscovered | protocol.FlagWall,
				})
			}
		}
	}
	if err := topo.BuildFromTiles(tiles); err != nil {
		t.Fatalf("build: %v", err)
	}
	return topo
}

// digRoomTiles returns TileState fixtures for an already-carved WxH floor
// room at (x0,y0,z), with the given flags — callers plug in either the
// post-fix (real) flags or the pre-fix (degraded, hardcoded-0x02) flags to
// exercise both sides of the bug.
func digRoomTiles(x0, y0, z, w, h int16, flags uint8) []protocol.TileState {
	var tiles []protocol.TileState
	for dy := int16(0); dy < h; dy++ {
		for dx := int16(0); dx < w; dx++ {
			tiles = append(tiles, protocol.TileState{
				X: x0 + dx, Y: y0 + dy, Z: z, TileType: 100, Flags: flags,
			})
		}
	}
	return tiles
}

// candidateCoversRoom reports whether any FindDigSites candidate exactly
// covers the dug room's footprint — the shape of the reported bug ("find_dig_site
// returned an already-dug room exact floor footprint as a fully solid
// candidate").
func candidateCoversRoom(sites []mapview.DigSite, x0, y0, x1, y1 int16) bool {
	for _, s := range sites {
		if s.X1 == x0 && s.Y1 == y0 && s.X2 == x1 && s.Y2 == y1 {
			return true
		}
	}
	return false
}

// TestOnTileUpdate_PostFixFlags_DoesNotOfferDugRoomAsCandidate is the
// regression guard: with a TILE_UPDATE carrying correctly-computed flags
// (what the fixed plugin sends — FlagDiscovered|FlagFloor for a dug-out
// tile), the room must read back as StateOpen and FindDigSites must never
// suggest digging it again.
func TestOnTileUpdate_PostFixFlags_DoesNotOfferDugRoomAsCandidate(t *testing.T) {
	const w, h, depth = 20, 20, 2
	const z, x0, y0, roomW, roomH = 1, 5, 5, 3, 3

	topo := buildSolidLevel(t, w, h, depth)
	wm := New(topo, nil, nil, nil)
	pop := NewPopulator(wm, nil, nil)

	tiles := digRoomTiles(x0, y0, z, roomW, roomH, protocol.FlagDiscovered|protocol.FlagFloor)
	pop.OnTileUpdate(&protocol.TileUpdateMessage{Count: uint32(len(tiles)), Tiles: tiles})

	for dy := int16(0); dy < roomH; dy++ {
		for dx := int16(0); dx < roomW; dx++ {
			if got := topo.GetTileState(x0+dx, y0+dy, z); got != topology.StateOpen {
				t.Fatalf("tile (%d,%d,%d): got %v, want StateOpen after post-fix TILE_UPDATE", x0+dx, y0+dy, z, got)
			}
		}
	}

	sites := mapview.FindDigSites(topo, mapview.DigSiteRequest{
		W: roomW, H: roomH, Z: z, NearX: x0, NearY: y0, MaxCandidates: 10,
	})
	if candidateCoversRoom(sites, x0, y0, x0+roomW-1, y0+roomH-1) {
		t.Fatalf("find_dig_site re-offered the already-dug room as a candidate: %+v", sites)
	}
}

// TestOnTileUpdate_DegradedFlags_ReproducesStaleSolidityBug pins the exact
// pre-fix failure mode: a TILE_UPDATE that carries only FLAG_DISCOVERED
// (0x02) — the plugin's old hardcoded value, regardless of the tile's real
// state — classifies as StateUnknown, and FindDigSites' "unknown is
// diggable mass" rule (by design, for real fog-of-war) then reports the
// dug room as a fully solid candidate. This is intentionally NOT a "must
// never happen" assertion: it documents why the wire contract (real flags
// on every delta) matters, so a future change can't quietly reintroduce a
// degraded/placeholder flags value on some new ingestion path without this
// test calling it out.
func TestOnTileUpdate_DegradedFlags_ReproducesStaleSolidityBug(t *testing.T) {
	const w, h, depth = 20, 20, 2
	const z, x0, y0, roomW, roomH = 1, 5, 5, 3, 3

	topo := buildSolidLevel(t, w, h, depth)
	wm := New(topo, nil, nil, nil)
	pop := NewPopulator(wm, nil, nil)

	tiles := digRoomTiles(x0, y0, z, roomW, roomH, protocol.FlagDiscovered) // pre-fix: FLAG_DISCOVERED only
	pop.OnTileUpdate(&protocol.TileUpdateMessage{Count: uint32(len(tiles)), Tiles: tiles})

	for dy := int16(0); dy < roomH; dy++ {
		for dx := int16(0); dx < roomW; dx++ {
			if got := topo.GetTileState(x0+dx, y0+dy, z); got != topology.StateUnknown {
				t.Fatalf("tile (%d,%d,%d): got %v, want StateUnknown from degraded flags (0x02 only)", x0+dx, y0+dy, z, got)
			}
		}
	}

	sites := mapview.FindDigSites(topo, mapview.DigSiteRequest{
		W: roomW, H: roomH, Z: z, NearX: x0, NearY: y0, MaxCandidates: 10,
	})
	if !candidateCoversRoom(sites, x0, y0, x0+roomW-1, y0+roomH-1) {
		t.Fatalf("expected degraded flags to reproduce the stale-solidity bug (dug room offered as solid candidate), got %+v", sites)
	}
}

// TestOnAnnouncementUpdate_RepeatCountBump_SurfacesAsChanged locks in the
// silent-damp-cancel-blindness fix: DF re-announces a repeated identical
// cancellation (e.g. a dig blocked by damp stone) by bumping repeat_count
// on the SAME report id rather than allocating a new one. A pure id-dedup
// AlertStore would treat the re-send as a no-op duplicate and the LLM would
// never learn the job is still failing. The plugin re-sends the entry with
// a higher RepeatCount; OnAnnouncementUpdate must fold that into the stored
// Alert (bump RepeatCount, refresh ReceivedAt) and report it as "changed" —
// not silently swallow it as a dupe.
func TestOnAnnouncementUpdate_RepeatCountBump_SurfacesAsChanged(t *testing.T) {
	wm := New(nil, nil, nil, nil)
	pop := NewPopulator(wm, nil, nil)

	first := &protocol.AnnouncementUpdateMessage{
		Count: 1,
		Announcements: []protocol.AnnouncementInfo{
			{ID: 42, TypeID: 1, Severity: 1, X: -1, Y: -1, Z: -1, Text: "Digging designation cancelled: damp stone located.", RepeatCount: 0},
		},
	}
	pop.OnAnnouncementUpdate(first)

	active := wm.Observed.Alerts.Active(0)
	if len(active) != 1 {
		t.Fatalf("expected 1 active alert after first send, got %d: %+v", len(active), active)
	}
	if active[0].RepeatCount != 0 {
		t.Fatalf("expected initial RepeatCount 0, got %d", active[0].RepeatCount)
	}
	firstSeenAt := active[0].ReceivedAt

	// A true duplicate resend (same ID, same RepeatCount) must NOT bump
	// anything -- this is the "cheap insurance" no-op path.
	pop.OnAnnouncementUpdate(first)
	active = wm.Observed.Alerts.Active(0)
	if len(active) != 1 || active[0].RepeatCount != 0 {
		t.Fatalf("true duplicate resend must be a no-op, got %+v", active)
	}

	// Same report id, repeat_count bumped: DF re-announced the SAME
	// cancellation again. This must update the stored alert in place (not
	// add a second entry) and bump RepeatCount + ReceivedAt.
	repeated := &protocol.AnnouncementUpdateMessage{
		Count: 1,
		Announcements: []protocol.AnnouncementInfo{
			{ID: 42, TypeID: 1, Severity: 1, X: -1, Y: -1, Z: -1, Text: "Digging designation cancelled: damp stone located.", RepeatCount: 3},
		},
	}
	pop.OnAnnouncementUpdate(repeated)

	active = wm.Observed.Alerts.Active(0)
	if len(active) != 1 {
		t.Fatalf("repeat-count bump must update the existing alert, not add a new one; got %d alerts: %+v", len(active), active)
	}
	if active[0].RepeatCount != 3 {
		t.Fatalf("expected RepeatCount bumped to 3, got %d", active[0].RepeatCount)
	}
	if active[0].ReceivedAt.Before(firstSeenAt) {
		t.Fatalf("expected ReceivedAt to refresh (or at least not regress) on repeat-count bump")
	}
}

// TestOnEntityUpdate_WorldIdentity_NoSwitchOnFirstObservation covers the
// startup case: the very first ENTITY_UPDATE a process ever sees reporting
// a world identity is NOT a "switch" (there was nothing to switch away
// from) — Switches must start at 0, not 1.
func TestOnEntityUpdate_WorldIdentity_NoSwitchOnFirstObservation(t *testing.T) {
	wm := New(nil, nil, nil, nil)
	pop := NewPopulator(wm, nil, nil)

	pop.OnEntityUpdate(&protocol.EntityUpdateMessage{
		World: &protocol.WorldIdentity{SaveDir: "region1", ID1: 10, ID2: 20},
	})

	snap := wm.Snapshot()
	if !snap.World.Known {
		t.Fatalf("expected World.Known=true after the first identity report")
	}
	if snap.World.Switches != 0 {
		t.Fatalf("first-ever identity observation must not count as a switch, got Switches=%d", snap.World.Switches)
	}
	if snap.World.SaveDir != "region1" {
		t.Fatalf("expected SaveDir=region1, got %q", snap.World.SaveDir)
	}
}

// TestOnEntityUpdate_WorldIdentity_SwitchFlushesEntitiesAndAlerts is the
// live-incident regression test (Q5): a save-swap mid-connection must (a)
// serve the NEW world's entity roster, never the previous one, (b) bump the
// dashboard-visible Switches counter so the model can notice even if it
// wasn't watching for it, and (c) flush the rolling Alerts buffer, which
// unlike Entities/Zones is not wholesale-replaced by every message and
// would otherwise keep serving the previous fort's cancellations forever.
func TestOnEntityUpdate_WorldIdentity_SwitchFlushesEntitiesAndAlerts(t *testing.T) {
	wm := New(nil, nil, nil, nil)
	pop := NewPopulator(wm, nil, nil)

	// Fort A: one dwarf, one alert.
	pop.OnEntityUpdate(&protocol.EntityUpdateMessage{
		Count:    1,
		Entities: []protocol.EntityInfo{{ID: 1, Type: protocol.EntityTypeDwarf, X: 5, Y: 5, Z: 90}},
		World:    &protocol.WorldIdentity{SaveDir: "region1", ID1: 10, ID2: 20},
	})
	pop.OnAnnouncementUpdate(&protocol.AnnouncementUpdateMessage{
		Count:         1,
		Announcements: []protocol.AnnouncementInfo{{ID: 1, TypeID: 1, Severity: 1, Text: "fort A cancellation", X: -1, Y: -1, Z: -1}},
	})
	if len(wm.Snapshot().Entities.Dwarves) != 1 {
		t.Fatalf("setup: expected 1 dwarf from fort A")
	}
	if len(wm.Observed.Alerts.Active(0)) != 1 {
		t.Fatalf("setup: expected 1 active alert from fort A")
	}

	// Fort B loads into the same running DF process (different save
	// identity), reporting a completely different roster and no alerts of
	// its own yet.
	pop.OnEntityUpdate(&protocol.EntityUpdateMessage{
		Count:    2,
		Entities: []protocol.EntityInfo{{ID: 101, Type: protocol.EntityTypeDwarf, X: 1, Y: 1, Z: 50}, {ID: 102, Type: protocol.EntityTypeDwarf, X: 2, Y: 2, Z: 50}},
		World:    &protocol.WorldIdentity{SaveDir: "region2", ID1: 30, ID2: 40},
	})

	snap := wm.Snapshot()
	if len(snap.Entities.Dwarves) != 2 {
		t.Fatalf("expected fort B's 2 dwarves, got %d — stale roster served across the switch", len(snap.Entities.Dwarves))
	}
	for _, d := range snap.Entities.Dwarves {
		if d.ID == 1 {
			t.Fatalf("fort A's dwarf (id=1) must not survive the world switch: %+v", snap.Entities.Dwarves)
		}
	}
	if snap.World.SaveDir != "region2" || snap.World.ID1 != 30 || snap.World.ID2 != 40 {
		t.Fatalf("expected World identity updated to fort B's, got %+v", snap.World)
	}
	if snap.World.Switches != 1 {
		t.Fatalf("expected exactly 1 detected switch, got %d", snap.World.Switches)
	}
	if active := wm.Observed.Alerts.Active(0); len(active) != 0 {
		t.Fatalf("expected fort A's alerts flushed on the world switch, got %+v", active)
	}

	// A repeat ENTITY_UPDATE for the SAME world (fort B again) must not
	// double-count as another switch.
	pop.OnEntityUpdate(&protocol.EntityUpdateMessage{
		Count:    2,
		Entities: []protocol.EntityInfo{{ID: 101, Type: protocol.EntityTypeDwarf}, {ID: 102, Type: protocol.EntityTypeDwarf}},
		World:    &protocol.WorldIdentity{SaveDir: "region2", ID1: 30, ID2: 40},
	})
	if got := wm.Snapshot().World.Switches; got != 1 {
		t.Fatalf("a repeat report of the same world must not bump Switches again, got %d", got)
	}
}

// TestOnEntityUpdate_WorldIdentity_NilPreservesPriorKnowledge covers an
// older-plugin message (msg.World == nil, the backward-compatible decode):
// it must leave the previously observed World snapshot untouched rather
// than resetting it to unknown/zero.
func TestOnEntityUpdate_WorldIdentity_NilPreservesPriorKnowledge(t *testing.T) {
	wm := New(nil, nil, nil, nil)
	pop := NewPopulator(wm, nil, nil)

	pop.OnEntityUpdate(&protocol.EntityUpdateMessage{
		World: &protocol.WorldIdentity{SaveDir: "region1", ID1: 10, ID2: 20},
	})
	pop.OnEntityUpdate(&protocol.EntityUpdateMessage{Count: 0}) // old-plugin-shaped: no World

	snap := wm.Snapshot()
	if !snap.World.Known || snap.World.SaveDir != "region1" {
		t.Fatalf("a message with no World data must not clobber previously known identity, got %+v", snap.World)
	}
}
