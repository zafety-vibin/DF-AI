package worldmodel

import (
	"context"
	"time"

	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

// Populator owns the dfhack event stream. It is the SINGLE consumer of the
// client's tile and entity update channels, fanning each event out into the
// observation overlays (topology, hazards, modifications) and snapshot
// fields (entities, zones, fort).
//
// Wiring:
//
//   pop := worldmodel.NewPopulator(wm, dfClient, logger)
//   pop.SetModDetector(detector)   // once available, after FULL_STATE
//   go pop.Run(ctx)                // single goroutine drives all updates
//
// FullState is still wired through the dfhack client's SetOnFullState
// callback (single-callback API, not a channel) — call OnFullState from
// inside that callback after constructing overlays.
type Populator struct {
	wm          *WorldModel
	client      *dfhack.Client
	modDetector *modifications.Detector
	logger      *logging.Logger
}

// NewPopulator returns a populator bound to wm and the given dfhack client.
// logger may be nil.
func NewPopulator(wm *WorldModel, client *dfhack.Client, logger *logging.Logger) *Populator {
	return &Populator{
		wm:     wm,
		client: client,
		logger: logger,
	}
}

// SetModDetector installs the modifications detector. Call this after the
// FULL_STATE handshake builds the modification baseline.
func (p *Populator) SetModDetector(d *modifications.Detector) {
	p.modDetector = d
}

// Run consumes tile and entity update channels until ctx cancels. Intended
// to be invoked in its own goroutine. Returns when ctx is cancelled or both
// channels close.
func (p *Populator) Run(ctx context.Context) {
	if p == nil || p.client == nil {
		return
	}
	tileCh := p.client.SubscribeTileUpdates()
	entityCh := p.client.SubscribeEntityUpdates()
	announceCh := p.client.SubscribeAnnouncementUpdates()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-tileCh:
			if !ok {
				return
			}
			p.OnTileUpdate(msg)
		case msg, ok := <-entityCh:
			if !ok {
				return
			}
			p.OnEntityUpdate(msg)
		case msg, ok := <-announceCh:
			if !ok {
				return
			}
			p.OnAnnouncementUpdate(msg)
		}
	}
}

// OnAnnouncementUpdate folds a batch of new (or repeat-count-bumped) DF
// announcements into the world model's AlertStore. True duplicates (same
// ID, same-or-lower RepeatCount) are silently dropped; an existing ID whose
// RepeatCount grew (the same problem firing again — see AlertStore.Add) is
// folded into the stored entry and counted as "changed" below.
func (p *Populator) OnAnnouncementUpdate(msg *protocol.AnnouncementUpdateMessage) {
	if p == nil || p.wm == nil || msg == nil {
		return
	}
	if p.wm.Observed.Alerts == nil {
		return
	}
	changed := 0
	for _, a := range msg.Announcements {
		alert := Alert{
			ID:          a.ID,
			TypeID:      a.TypeID,
			Severity:    a.Severity,
			Text:        a.Text,
			X:           a.X,
			Y:           a.Y,
			Z:           a.Z,
			GameYear:    a.GameYear,
			GameTick:    a.GameTick,
			ReceivedAt:  time.Now(),
			RepeatCount: a.RepeatCount,
		}
		if p.wm.Observed.Alerts.Add(alert) {
			changed++
		}
	}
	p.wm.markUpdated()
	if p.logger != nil && changed > 0 {
		p.logger.Info("announcements: added or repeat-bumped",
			logging.Field{Key: "count", Value: changed})
	}
}

// OnFullState handles the FULL_STATE handshake message. The caller is still
// responsible for constructing overlays and calling WorldModel.SetOverlays
// before calling this method. OnFullState bumps the tick and logs.
func (p *Populator) OnFullState(msg *protocol.FullStateMessage) {
	if p == nil || p.wm == nil || msg == nil {
		return
	}
	tick := p.wm.AdvanceTick()
	p.wm.markUpdated()

	if p.logger != nil {
		p.logger.Debug("worldmodel: full state received",
			logging.Field{Key: "tick", Value: tick},
			logging.Field{Key: "tile_count", Value: len(msg.Tiles)},
			logging.Field{Key: "dimensions", Value: [3]uint16{msg.Width, msg.Height, msg.Depth}})
	}
}

// OnTileUpdate fans an incremental tile update out into the topology
// overlay, hazard overlays, and modification detector. Bumps the tick.
func (p *Populator) OnTileUpdate(msg *protocol.TileUpdateMessage) {
	if p == nil || p.wm == nil || msg == nil {
		return
	}

	if topo := p.wm.Observed.Topology; topo != nil && msg.Count > 0 {
		for _, tile := range msg.Tiles {
			_ = topo.SetTileState(tile.X, tile.Y, tile.Z, topology.ClassifyState(tile.Flags))
		}
	}
	if hzd := p.wm.Observed.Hazards; hzd != nil && msg.Count > 0 {
		hzd.UpdateFromTiles(msg.Tiles)
	}
	if p.modDetector != nil && msg.Count > 0 {
		p.modDetector.DetectModifications(msg.Tiles)
	}

	tick := p.wm.AdvanceTick()
	p.wm.markUpdated()

	if p.logger != nil && msg.Count > 0 {
		p.logger.Debug("worldmodel: tile update received",
			logging.Field{Key: "tick", Value: tick},
			logging.Field{Key: "changed_tiles", Value: msg.Count})
	}
}

// OnEntityUpdate handles entity position updates. Builds an EntitySnapshot
// partitioned by type, copies optional FortInfo / ZoneData payloads, and
// bumps the tick.
func (p *Populator) OnEntityUpdate(msg *protocol.EntityUpdateMessage) {
	if p == nil || p.wm == nil || msg == nil {
		return
	}

	tick := p.wm.AdvanceTick()
	now := time.Now()
	eventSeq := p.wm.NextEventID()

	entSnap := buildEntitySnapshot(msg.Entities, now, eventSeq)
	zoneSnap := ZoneSnapshot{
		All:       append([]protocol.ZoneData(nil), msg.Zones...),
		UpdatedAt: now,
		EventSeq:  eventSeq,
	}

	var fortSnap FortSnapshot
	if msg.FortInfo != nil {
		fortSnap = FortSnapshot{
			DaysElapsed:   msg.FortInfo.DaysElapsed,
			CreatedWealth: msg.FortInfo.CreatedWealth,
			Season:        msg.FortInfo.Season,
			Year:          msg.FortInfo.Year,
			UpdatedAt:     now,
			Valid:         true,
		}
	}

	p.wm.mu.Lock()
	p.wm.Observed.Entities = entSnap
	p.wm.Observed.Zones = zoneSnap
	if msg.FortInfo != nil {
		p.wm.Observed.Fort = fortSnap
	}
	p.wm.mu.Unlock()

	p.wm.markUpdated()

	if p.logger != nil {
		p.logger.Debug("worldmodel: entity update received",
			logging.Field{Key: "tick", Value: tick},
			logging.Field{Key: "entity_count", Value: len(msg.Entities)},
			logging.Field{Key: "dwarf_count", Value: len(entSnap.Dwarves)},
			logging.Field{Key: "enemy_count", Value: len(entSnap.Enemies)},
			logging.Field{Key: "zone_count", Value: len(zoneSnap.All)},
			logging.Field{Key: "fort_valid", Value: fortSnap.Valid})
	}
}

func buildEntitySnapshot(entities []protocol.EntityInfo, now time.Time, seq uint64) EntitySnapshot {
	all := append([]protocol.EntityInfo(nil), entities...)

	var dwarves, enemies, animals []protocol.EntityInfo
	for _, e := range entities {
		switch e.Type {
		case protocol.EntityTypeDwarf:
			dwarves = append(dwarves, e)
		case protocol.EntityTypeEnemy:
			enemies = append(enemies, e)
		case protocol.EntityTypeAnimal:
			animals = append(animals, e)
		}
	}

	return EntitySnapshot{
		All:       all,
		Dwarves:   dwarves,
		Enemies:   enemies,
		Animals:   animals,
		UpdatedAt: now,
		EventSeq:  seq,
	}
}
