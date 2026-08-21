package worldmodel

import (
	"path/filepath"
	"testing"
)

// TestAlertStore_Add_RepeatBumpUndismisses locks in the fix for: a
// repeat-count bump on an already-DISMISSED alert must clear Dismissed, or
// the recurrence stays invisible forever (Active/Snapshot filter dismissed
// alerts out, so the bump would appear in neither the before- nor
// after-step ActiveAlerts view, and stepReport would never surface it).
// Reachable via the ordinary flow: LLM dismisses a damp-cancel warning,
// re-designates, DF pools the next identical cancel onto the same report
// id -- silent failure again.
func TestAlertStore_Add_RepeatBumpUndismisses(t *testing.T) {
	s := NewAlertStore(10)

	if changed := s.Add(Alert{ID: 7, Severity: 1, Text: "damp stone", RepeatCount: 0}); !changed {
		t.Fatal("expected initial Add to report changed=true")
	}

	if dismissed := s.Dismiss(7); !dismissed {
		t.Fatal("expected Dismiss to succeed on a freshly-added alert")
	}
	if active := s.Active(0); len(active) != 0 {
		t.Fatalf("expected 0 active alerts after dismiss, got %d", len(active))
	}

	// DF pools the next identical cancel onto the same report id: same ID,
	// higher RepeatCount.
	if changed := s.Add(Alert{ID: 7, Severity: 1, Text: "damp stone", RepeatCount: 1}); !changed {
		t.Fatal("expected repeat-count bump to report changed=true")
	}

	active := s.Active(0)
	if len(active) != 1 {
		t.Fatalf("expected the repeat bump to reappear in Active(), got %d entries", len(active))
	}
	if active[0].Dismissed {
		t.Fatal("expected Dismissed to be cleared by the repeat-count bump")
	}
	if active[0].RepeatCount != 1 {
		t.Fatalf("expected RepeatCount 1, got %d", active[0].RepeatCount)
	}
}

// TestAlertStore_Add_TrueDuplicateStaysDismissed ensures the fix above does
// NOT undismiss on a true no-op resend (same ID, same-or-lower RepeatCount)
// -- only an actual repeat-count increase should clear Dismissed.
func TestAlertStore_Add_TrueDuplicateStaysDismissed(t *testing.T) {
	s := NewAlertStore(10)

	s.Add(Alert{ID: 9, Severity: 1, Text: "damp stone", RepeatCount: 2})
	s.Dismiss(9)

	if changed := s.Add(Alert{ID: 9, Severity: 1, Text: "damp stone", RepeatCount: 2}); changed {
		t.Fatal("expected a true duplicate (same RepeatCount) to report changed=false")
	}
	if active := s.Active(0); len(active) != 0 {
		t.Fatalf("expected true duplicate to remain dismissed, got %d active", len(active))
	}
}

var testWorld = WorldSnapshot{SaveDir: "region13", ID1: 111, ID2: 222, Known: true}

// TestAlertStore_LoadMissingFile mirrors PlaceStore's contract: a fresh
// checkout or a fort that has never dismissed anything has no file, and that
// is normal, not an error.
func TestAlertStore_LoadMissingFile(t *testing.T) {
	s := NewAlertStore(10)
	if err := s.Load(filepath.Join(t.TempDir(), "nope", "alerts.json")); err != nil {
		t.Fatalf("expected a missing alerts file to load cleanly, got %v", err)
	}
	if s.seed != nil {
		t.Fatal("expected no dismissal seed from a missing file")
	}
}

// TestAlertStore_DismissalsSurviveProcessRestart is the actual bug: df-mcp
// restarts, the plugin rewinds its announcement cursor and replays DF's whole
// backlog, and everything the model already handled reappears. Save/Load must
// keep the handled ones handled.
func TestAlertStore_DismissalsSurviveProcessRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "alerts.json")

	before := NewAlertStore(10)
	before.Add(Alert{ID: 1, Text: "damp stone", RepeatCount: 3})
	before.Add(Alert{ID: 2, Text: "ambush!", RepeatCount: 0})
	before.Dismiss(1)
	if err := before.Save(path, testWorld); err != nil {
		t.Fatalf("save: %v", err)
	}

	// New process: empty store, load the seed, then the plugin replays both
	// reports verbatim (same ids, same repeat counts).
	after := NewAlertStore(10)
	if err := after.Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	if changed := after.Add(Alert{ID: 1, Text: "damp stone", RepeatCount: 3}); changed {
		t.Fatal("expected a replayed, already-dismissed alert to report changed=false")
	}
	after.Add(Alert{ID: 2, Text: "ambush!", RepeatCount: 0})

	active := after.Active(0)
	if len(active) != 1 || active[0].ID != 2 {
		t.Fatalf("expected only the undismissed alert 2 to be active, got %+v", active)
	}
	if _, _, total := after.Counts(); total != 2 {
		t.Fatalf("expected both alerts stored (dismissed ones are kept, not dropped), got total=%d", total)
	}
}

// TestAlertStore_SeedDoesNotSwallowARecurrence guards the one case a naive
// "dismissed ids" seed would get wrong: the same report id coming back with a
// HIGHER repeat count means DF fired the problem again since it was handled,
// and that must stay visible.
func TestAlertStore_SeedDoesNotSwallowARecurrence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alerts.json")

	before := NewAlertStore(10)
	before.Add(Alert{ID: 1, Text: "damp stone", RepeatCount: 3})
	before.Dismiss(1)
	if err := before.Save(path, testWorld); err != nil {
		t.Fatalf("save: %v", err)
	}

	after := NewAlertStore(10)
	if err := after.Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	if changed := after.Add(Alert{ID: 1, Text: "damp stone", RepeatCount: 4}); !changed {
		t.Fatal("expected a higher repeat count to report changed=true")
	}
	if active := after.Active(0); len(active) != 1 {
		t.Fatalf("expected the recurrence to be visible, got %d active", len(active))
	}
}

// TestAlertStore_ReconcileWorldRevertsForeignSeed covers the guard PlaceStore
// never needed: DF report ids are only unique within one save, so a seed from
// a DIFFERENT fort would silently auto-dismiss the new fort's early alerts.
// ReconcileWorld reverts exactly what the seed applied, once the first
// ENTITY_UPDATE reveals the identity FULL_STATE could not.
func TestAlertStore_ReconcileWorldRevertsForeignSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alerts.json")

	before := NewAlertStore(10)
	before.Add(Alert{ID: 1, Text: "old fort's handled cancel", RepeatCount: 0})
	before.Dismiss(1)
	if err := before.Save(path, testWorld); err != nil {
		t.Fatalf("save: %v", err)
	}

	after := NewAlertStore(10)
	if err := after.Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	// A DIFFERENT fort happens to reuse report id 1, plus one alert the seed
	// never touched.
	after.Add(Alert{ID: 1, Text: "new fort's first cancel", RepeatCount: 0})
	after.Add(Alert{ID: 2, Text: "new fort's second cancel", RepeatCount: 0})
	after.Dismiss(2)
	if active := after.Active(0); len(active) != 0 {
		t.Fatalf("precondition: expected the foreign seed to have hidden alert 1, got %d active", len(active))
	}

	other := WorldSnapshot{SaveDir: "region99", ID1: 333, ID2: 444, Known: true}
	if reverted := after.ReconcileWorld(other); reverted != 1 {
		t.Fatalf("expected exactly the 1 seed-applied dismissal to be reverted, got %d", reverted)
	}
	active := after.Active(0)
	if len(active) != 1 || active[0].ID != 1 {
		t.Fatalf("expected alert 1 to resurface and the genuinely dismissed alert 2 to stay hidden, got %+v", active)
	}
}

// TestAlertStore_ReconcileWorldKeepsMatchingSeed is the common path: same
// fort, restarted process — the seed stays in force.
func TestAlertStore_ReconcileWorldKeepsMatchingSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alerts.json")

	before := NewAlertStore(10)
	before.Add(Alert{ID: 5, Text: "handled", RepeatCount: 0})
	before.Dismiss(5)
	if err := before.Save(path, testWorld); err != nil {
		t.Fatalf("save: %v", err)
	}

	after := NewAlertStore(10)
	if err := after.Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	after.Add(Alert{ID: 5, Text: "handled", RepeatCount: 0})
	if reverted := after.ReconcileWorld(testWorld); reverted != 0 {
		t.Fatalf("expected nothing reverted for the same world, got %d", reverted)
	}
	if active := after.Active(0); len(active) != 0 {
		t.Fatalf("expected the dismissal to hold across the restart, got %d active", len(active))
	}
}

// TestAlertStore_SaveCarriesForwardUnreplayedSeed guards against a Save early
// in a session (before the plugin has replayed everything) truncating the
// previous session's record down to whatever happens to be in memory.
func TestAlertStore_SaveCarriesForwardUnreplayedSeed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alerts.json")

	before := NewAlertStore(10)
	before.Add(Alert{ID: 1, Text: "a", RepeatCount: 0})
	before.Add(Alert{ID: 2, Text: "b", RepeatCount: 0})
	before.Dismiss(1)
	before.Dismiss(2)
	if err := before.Save(path, testWorld); err != nil {
		t.Fatalf("save: %v", err)
	}

	mid := NewAlertStore(10)
	if err := mid.Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	// Only alert 1 has been replayed so far; 2 is still only in the seed.
	mid.Add(Alert{ID: 1, Text: "a", RepeatCount: 0})
	mid.Add(Alert{ID: 3, Text: "c", RepeatCount: 0})
	mid.Dismiss(3)
	if err := mid.Save(path, testWorld); err != nil {
		t.Fatalf("resave: %v", err)
	}

	last := NewAlertStore(10)
	if err := last.Load(path); err != nil {
		t.Fatalf("reload: %v", err)
	}
	for _, id := range []uint32{1, 2, 3} {
		if _, ok := last.seed[id]; !ok {
			t.Fatalf("expected alert %d to survive the mid-session resave, seed=%v", id, last.seed)
		}
	}
}
