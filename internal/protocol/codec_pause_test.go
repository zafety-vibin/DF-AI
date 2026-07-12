package protocol

import "testing"

func TestPauseCommandRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   42,
		CommandType: CommandTypePause,
		Pause:       PauseControl{Mode: PauseModeStep, Ticks: 1200},
	}
	data, err := SerializeMessage(orig)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DeserializeMessage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, ok := decoded.(*CommandMessage)
	if !ok {
		t.Fatalf("decoded wrong type %T", decoded)
	}
	if got.Pause.Mode != PauseModeStep || got.Pause.Ticks != 1200 || got.CommandID != 42 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
