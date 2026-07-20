package worldmodel

import (
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
	return true
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
