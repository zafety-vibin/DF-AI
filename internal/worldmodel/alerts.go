package worldmodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Alert is one DF announcement / cancellation surfaced to the orchestrator.
//
// These come from df::global::world->status.announcements and represent the
// game's own player-facing diagnostic stream — "Urist cancels Mine: improper
// tile location", "construction site occupied", "ambush!", etc. They are the
// primary signal the LLM should read when work isn't progressing.
type Alert struct {
	ID          uint32    // DF report id; stable identifier for dismissal
	TypeID      uint16    // DF announcement_type enum value
	Severity    uint8     // 0=info, 1=warn, 2=critical (derived from type)
	Text        string    // human-readable announcement text
	X, Y, Z     int16     // position; (-1,-1,-1) if non-positional
	GameYear    uint32    // DF in-game year
	GameTick    uint32    // DF in-game tick within year
	ReceivedAt  time.Time // orchestrator wall-clock
	Dismissed   bool      // true after the LLM (or operator) marks it as seen
	RepeatCount uint32    // DF report.repeat_count as of the most recent send; grows when the SAME announcement (e.g. a damp-stone dig cancel) fires again without a new report id — see AlertStore.Add
}

// HasPosition reports whether the alert has a meaningful (x, y, z).
func (a Alert) HasPosition() bool {
	return a.X >= 0 && a.Y >= 0 && a.Z >= 0
}

// AlertStore keeps the most recent N alerts in a ring buffer. Older alerts
// are evicted when the buffer fills. Dismissed alerts are kept (with the
// flag set) until evicted, so the LLM can still see "you dismissed this on
// turn N" if it asks.
//
// Concurrency: safe for one writer (Populator) and many readers (Snapshot).
type AlertStore struct {
	mu       sync.RWMutex
	capacity int
	entries  []Alert        // newest at the end
	byID     map[uint32]int // ID → index in entries (for dedup + dismiss)

	// Dismissal seed restored from disk — see Load/Save and the
	// AlertsFile doc comment. seed maps a previously-dismissed alert ID to
	// the RepeatCount it carried when it was dismissed; seedWorld is the
	// world identity the file was stamped with; seedApplied records which
	// IDs Add actually auto-dismissed from the seed, so ReconcileWorld can
	// revert exactly those (and nothing else) if the connecting world turns
	// out to be a different fort.
	seed        map[uint32]uint32
	seedWorld   WorldSnapshot
	seedApplied map[uint32]bool
}

// NewAlertStore returns an empty store with the given capacity. Capacity
// <= 0 falls back to 200, which holds ~10-20 minutes of normal-fortress
// announcements at a comfortable margin.
func NewAlertStore(capacity int) *AlertStore {
	if capacity <= 0 {
		capacity = 200
	}
	return &AlertStore{
		capacity: capacity,
		entries:  make([]Alert, 0, capacity),
		byID:     make(map[uint32]int, capacity),
	}
}

// Add inserts a new alert, or — if an alert with the same ID is already
// present — folds a repeat-count bump into the existing entry. DF report
// IDs are unique and monotonic, but the game re-announces a repeated
// identical event (e.g. "Digging designation cancelled: damp stone
// located." firing over and over) by bumping repeat_count IN PLACE on the
// SAME report instead of allocating a new one; the plugin re-sends that
// entry (same ID, higher RepeatCount) rather than a fresh ID. So an
// existing ID is not always a true no-op: Add updates RepeatCount and
// ReceivedAt on the stored entry and returns true ("changed") whenever the
// incoming RepeatCount is higher than what's stored, so callers (stepReport)
// can tell "the same problem happened again" from a genuine no-op resend.
// Returns true if the alert was newly stored OR its repeat count grew;
// false if this is a true duplicate (same ID, same-or-lower RepeatCount).
func (s *AlertStore) Add(a Alert) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byID[a.ID]; !exists && s.seed != nil {
		// First sighting of an ID a previous df-mcp process had already
		// dismissed, at the same repeat count — this is the plugin replaying
		// its backlog (it rewinds its cursor to -1 on every reconnect by
		// design), not a new problem. Restore the dismissal instead of
		// resurfacing dozens of handled alerts. A HIGHER repeat count means
		// the problem genuinely fired again since, and falls through to a
		// normal, visible insert.
		if seededRepeat, ok := s.seed[a.ID]; ok && a.RepeatCount <= seededRepeat {
			a.Dismissed = true
			if s.seedApplied == nil {
				s.seedApplied = make(map[uint32]bool, len(s.seed))
			}
			s.seedApplied[a.ID] = true
			s.insertLocked(a)
			return false
		}
	}

	if idx, exists := s.byID[a.ID]; exists {
		if a.RepeatCount <= s.entries[idx].RepeatCount {
			return false
		}
		s.entries[idx].RepeatCount = a.RepeatCount
		s.entries[idx].ReceivedAt = a.ReceivedAt
		// A repeat-count bump means the SAME problem fired again -- if the
		// LLM had previously dismissed this ID, that dismissal no longer
		// applies to the new occurrence. Without this, a dismissed alert's
		// repeat bump stays invisible forever: Snapshot/Active() filter
		// Dismissed alerts out, so the recurrence never reaches stepReport.
		s.entries[idx].Dismissed = false
		return true
	}

	s.insertLocked(a)
	return true
}

// insertLocked appends a to the ring buffer, evicting the oldest entry when
// full. Caller holds s.mu.
func (s *AlertStore) insertLocked(a Alert) {
	if len(s.entries) >= s.capacity {
		// evict oldest
		evicted := s.entries[0]
		s.entries = s.entries[1:]
		delete(s.byID, evicted.ID)
		// shift indices in byID
		for id, idx := range s.byID {
			s.byID[id] = idx - 1
		}
	}

	s.entries = append(s.entries, a)
	s.byID[a.ID] = len(s.entries) - 1
}

// Dismiss marks the alert with the given ID as dismissed. Returns true if
// the alert exists and was newly dismissed (false if already dismissed or
// never seen).
func (s *AlertStore) Dismiss(id uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, ok := s.byID[id]
	if !ok {
		return false
	}
	if s.entries[idx].Dismissed {
		return false
	}
	s.entries[idx].Dismissed = true
	return true
}

// Reset discards every stored alert. Used when the underlying world
// identity changes mid-connection (see WorldSnapshot/Populator.
// OnEntityUpdate): unlike Entities/Zones (wholesale-replaced by every
// ENTITY_UPDATE regardless), Alerts is a ROLLING accumulator that would
// otherwise keep serving the PREVIOUS world's cancellations/ambushes
// forever after a save-swap -- the same stale-residue bug the entity-cache
// flush closes, applied to the one piece of Observed state that isn't
// naturally overwritten every message.
func (s *AlertStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = s.entries[:0]
	s.byID = make(map[uint32]int, s.capacity)
	s.seed = nil
	s.seedWorld = WorldSnapshot{}
	s.seedApplied = nil
}

// DismissedSeedEntry is one persisted dismissal: the alert's DF report id and
// the repeat count it carried when it was dismissed. The repeat count is what
// distinguishes "the plugin is replaying an alert I already handled" from "the
// same problem fired AGAIN since I handled it" — only the former should stay
// dismissed (see AlertStore.Add).
type DismissedSeedEntry struct {
	ID          uint32 `json:"id"`
	RepeatCount uint32 `json:"repeat_count"`
}

// AlertsFile is the on-disk shape written by AlertStore.Save.
//
// Only DISMISSALS are persisted, not the alerts themselves: the plugin
// rewinds its announcement cursor to -1 on every reconnect and replays DF's
// whole announcement history (df::report objects are never deleted), so the
// alert TEXT always comes back on its own. The one thing that does not come
// back across a df-mcp process restart is the model's own record of what it
// had already read and handled — which is exactly what made a restart
// resurface dozens of handled alerts.
//
// World stamps the fort the dismissals belong to. DF report ids restart at
// low numbers in a different save, so an unstamped seed could silently
// auto-dismiss a NEW fort's early alerts; see AlertStore.ReconcileWorld.
type AlertsFile struct {
	World     WorldSnapshot        `json:"world"`
	Dismissed []DismissedSeedEntry `json:"dismissed"`
}

// Load reads a persisted dismissal seed. A missing file is not an error — a
// fresh fort or a fresh checkout simply has nothing dismissed yet (same
// contract as PlaceStore.Load).
//
// The seed is held UNVERIFIED until ReconcileWorld sees the connecting
// world's identity: FULL_STATE, the natural place to Load, carries no world
// identity at all (only ENTITY_UPDATE does). Applying it in the meantime is
// the right default — the overwhelmingly common case is a restart into the
// same fort — and ReconcileWorld reverts precisely what it applied if the
// fort turns out to be a different one.
func (s *AlertStore) Load(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var f AlertsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.seed = make(map[uint32]uint32, len(f.Dismissed))
	for _, d := range f.Dismissed {
		s.seed[d.ID] = d.RepeatCount
	}
	s.seedWorld = f.World
	s.seedApplied = nil
	// Alerts already in memory (a reconnect on a still-running process) keep
	// their live state; the seed only ever affects IDs seen for the first
	// time. Nothing to back-apply here.
	return nil
}

// Save writes the current dismissal set, stamped with world, creating the
// parent directory if needed (mirrors PlaceStore.Save). Call it whenever a
// dismissal changes — that is the state a restart loses.
func (s *AlertStore) Save(path string, world WorldSnapshot) error {
	s.mu.RLock()
	f := AlertsFile{World: world, Dismissed: make([]DismissedSeedEntry, 0, len(s.entries))}
	for _, a := range s.entries {
		if a.Dismissed {
			f.Dismissed = append(f.Dismissed, DismissedSeedEntry{ID: a.ID, RepeatCount: a.RepeatCount})
		}
	}
	// Carry forward seeded dismissals whose alerts have not been re-delivered
	// yet, so a Save early in a session cannot truncate the previous
	// session's record.
	for id, rc := range s.seed {
		if _, live := s.byID[id]; !live {
			f.Dismissed = append(f.Dismissed, DismissedSeedEntry{ID: id, RepeatCount: rc})
		}
	}
	s.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ReconcileWorld resolves a loaded dismissal seed against the world identity
// the plugin actually reported (first available on ENTITY_UPDATE, after the
// FULL_STATE where Load runs). If the seed came from a DIFFERENT fort, every
// dismissal it applied is reverted and the seed is dropped — DF report ids
// are only unique within one save, so keeping it would silently swallow the
// new fort's own announcements. Returns the number of alerts un-dismissed.
//
// A seed with no world stamp (saved before any ENTITY_UPDATE arrived) cannot
// be checked and is kept as-is rather than thrown away.
func (s *AlertStore) ReconcileWorld(world WorldSnapshot) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.seed == nil || !s.seedWorld.Changed(world) {
		return 0
	}
	reverted := 0
	for id := range s.seedApplied {
		if idx, ok := s.byID[id]; ok && s.entries[idx].Dismissed {
			s.entries[idx].Dismissed = false
			reverted++
		}
	}
	s.seed = nil
	s.seedWorld = WorldSnapshot{}
	s.seedApplied = nil
	return reverted
}

// DismissAll marks every currently-stored alert as dismissed. Returns the
// number newly dismissed.
func (s *AlertStore) DismissAll() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := 0
	for i := range s.entries {
		if !s.entries[i].Dismissed {
			s.entries[i].Dismissed = true
			n++
		}
	}
	return n
}

// Active returns the undismissed alerts, newest first, up to limit.
// limit <= 0 returns all.
func (s *AlertStore) Active(limit int) []Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Alert, 0)
	for i := len(s.entries) - 1; i >= 0; i-- {
		if s.entries[i].Dismissed {
			continue
		}
		out = append(out, s.entries[i])
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// Recent returns the most recent N alerts regardless of dismissal state,
// newest first.
func (s *AlertStore) Recent(limit int) []Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.entries) {
		limit = len(s.entries)
	}
	out := make([]Alert, 0, limit)
	for i := len(s.entries) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.entries[i])
	}
	return out
}

// Counts returns (active, dismissed, total).
func (s *AlertStore) Counts() (active, dismissed, total int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, a := range s.entries {
		if a.Dismissed {
			dismissed++
		} else {
			active++
		}
	}
	total = len(s.entries)
	return
}
