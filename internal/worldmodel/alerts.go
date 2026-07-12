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
	ID         uint32    // DF report id; stable identifier for dismissal
	TypeID     uint16    // DF announcement_type enum value
	Severity   uint8     // 0=info, 1=warn, 2=critical (derived from type)
	Text       string    // human-readable announcement text
	X, Y, Z    int16     // position; (-1,-1,-1) if non-positional
	GameYear   uint32    // DF in-game year
	GameTick   uint32    // DF in-game tick within year
	ReceivedAt time.Time // orchestrator wall-clock
	Dismissed  bool      // true after the LLM (or operator) marks it as seen
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
	mu        sync.RWMutex
	capacity  int
	entries   []Alert            // newest at the end
	byID      map[uint32]int     // ID → index in entries (for dedup + dismiss)
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

// Add inserts a new alert. If an alert with the same ID is already present,
// it's a no-op (DF report IDs are unique and monotonic). Returns true if
// the alert was newly stored.
func (s *AlertStore) Add(a Alert) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byID[a.ID]; exists {
		return false
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
