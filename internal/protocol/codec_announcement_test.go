package protocol

import (
	"testing"
)

// TestAnnouncementUpdateRoundTrip locks in the new-format wire shape: the
// trailing RepeatCount block (one uint32 per entry, appended after all N
// entries) round-trips through Serialize/Deserialize intact, in the same
// per-entry order it was written in.
func TestAnnouncementUpdateRoundTrip(t *testing.T) {
	msg := &AnnouncementUpdateMessage{
		Count: 2,
		Announcements: []AnnouncementInfo{
			{ID: 10, TypeID: 5, Severity: 1, X: 3, Y: 4, Z: 5, GameYear: 250, GameTick: 1000, Text: "damp stone", RepeatCount: 3},
			{ID: 11, TypeID: 6, Severity: 0, X: -1, Y: -1, Z: -1, GameYear: 250, GameTick: 1001, Text: "migrants arrived", RepeatCount: 0},
		},
	}
	encoded, err := SerializeMessage(msg)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	decodedMsg, err := DeserializeMessage(encoded)
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	decoded, ok := decodedMsg.(*AnnouncementUpdateMessage)
	if !ok {
		t.Fatalf("expected *AnnouncementUpdateMessage, got %T", decodedMsg)
	}
	if len(decoded.Announcements) != 2 {
		t.Fatalf("expected 2 announcements, got %d", len(decoded.Announcements))
	}
	if got, want := decoded.Announcements[0].RepeatCount, uint32(3); got != want {
		t.Errorf("entry 0 RepeatCount: got %d, want %d", got, want)
	}
	if got, want := decoded.Announcements[1].RepeatCount, uint32(0); got != want {
		t.Errorf("entry 1 RepeatCount: got %d, want %d", got, want)
	}
	if decoded.Announcements[0].Text != "damp stone" || decoded.Announcements[1].Text != "migrants arrived" {
		t.Fatalf("text mismatch after round-trip: %+v", decoded.Announcements)
	}
}

// TestAnnouncementUpdateOldFormatStillDecodes reproduces a message from a
// plugin that predates the RepeatCount field: exactly the [4:Count][N ×
// entry] bytes, no trailing block at all. The live DF session can still be
// running this exact old plugin while the Go server reconnects with the new
// decode code (deploying the .plug.dll requires closing DF first) — this
// must decode cleanly with RepeatCount defaulting to 0, never error.
func TestAnnouncementUpdateOldFormatStillDecodes(t *testing.T) {
	msg := &AnnouncementUpdateMessage{
		Count: 2,
		Announcements: []AnnouncementInfo{
			{ID: 20, TypeID: 7, Severity: 1, X: 1, Y: 2, Z: 3, GameYear: 251, GameTick: 500, Text: "cancel: dangerous terrain"},
			{ID: 21, TypeID: 8, Severity: 2, X: -1, Y: -1, Z: -1, GameYear: 251, GameTick: 501, Text: "ambush!"},
		},
	}
	fullEncoded, err := SerializeMessage(msg)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	// Old plugin behavior: strip the trailing RepeatCount block (8 bytes =
	// 2 entries × 4) and fix up the length header to match.
	oldFormat := fullEncoded[:len(fullEncoded)-8]
	oldFormat[0] = byte(len(oldFormat) >> 24)
	oldFormat[1] = byte(len(oldFormat) >> 16)
	oldFormat[2] = byte(len(oldFormat) >> 8)
	oldFormat[3] = byte(len(oldFormat))

	decodedMsg, err := DeserializeMessage(oldFormat)
	if err != nil {
		t.Fatalf("old-format message must decode without error, got: %v", err)
	}
	decoded, ok := decodedMsg.(*AnnouncementUpdateMessage)
	if !ok {
		t.Fatalf("expected *AnnouncementUpdateMessage, got %T", decodedMsg)
	}
	if len(decoded.Announcements) != 2 {
		t.Fatalf("expected 2 announcements, got %d", len(decoded.Announcements))
	}
	for i, a := range decoded.Announcements {
		if a.RepeatCount != 0 {
			t.Errorf("entry %d: RepeatCount = %d, want 0 (old format has no repeat_count)", i, a.RepeatCount)
		}
	}
	if decoded.Announcements[0].Text != "cancel: dangerous terrain" || decoded.Announcements[1].Text != "ambush!" {
		t.Fatalf("text mismatch decoding old format: %+v", decoded.Announcements)
	}
}
