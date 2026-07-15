package worldmodel

import "testing"

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
