package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/df-ai/orchestrator/internal/blueprints"
	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

func TestExpandBlueprint(t *testing.T) {
	lib := blueprints.NewBlueprintLibrary(t.TempDir()) // empty dir → empty lib
	bp := &blueprints.DigBlueprint{
		Name: "tiny", Digs: []blueprints.DigEntry{
			{X: 0, Y: 0, Z: 0, DigType: "default"},
			{X: 1, Y: 0, Z: 0, DigType: "default"},
		},
	}
	lib.AddBlueprint("tiny", bp)
	cmds, err := expandBlueprint(lib, "tiny", modifications.Coordinate{X: 50, Y: 60, Z: 100})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(cmds) != 2 || cmds[1].X != 51 || cmds[0].Y != 60 || cmds[0].Z != 100 {
		t.Fatalf("expansion wrong: %+v", cmds)
	}
	if _, err := expandBlueprint(lib, "nope", modifications.Coordinate{}); err == nil {
		t.Fatal("missing blueprint must error")
	}
	_ = strings.TrimSpace
}

func TestSummarizeDryRunNilTopo(t *testing.T) {
	cmds := []blueprints.DigCommand{
		{X: 1, Y: 1, Z: 5, DigType: "default"},
		{X: 2, Y: 1, Z: 5, DigType: "default"},
		{X: 3, Y: 1, Z: 5, DigType: "stairs"},
	}
	got := summarizeDryRun(nil, cmds)
	if !strings.Contains(got, "3 tiles would be designated") {
		t.Errorf("missing total count: %q", got)
	}
	if !strings.Contains(got, "default=2") || !strings.Contains(got, "stairs=1") {
		t.Errorf("missing per-digtype counts: %q", got)
	}
	// nil topology → every tile counts as solid, none open
	if !strings.Contains(got, "3 into solid ground (good), 0 onto already-open tiles") {
		t.Errorf("nil topo should count all solid: %q", got)
	}
}

func TestSummarizeDryRunOpenVsSolid(t *testing.T) {
	topo := topology.NewTopologyOverlay(10, 10, 10)
	if topo == nil {
		t.Fatal("overlay nil")
	}
	// Tile (1,1,5) open, (2,1,5) closed, (3,1,5) left unknown (counts solid).
	if err := topo.SetTileState(1, 1, 5, topology.StateOpen); err != nil {
		t.Fatal(err)
	}
	if err := topo.SetTileState(2, 1, 5, topology.StateClosed); err != nil {
		t.Fatal(err)
	}
	cmds := []blueprints.DigCommand{
		{X: 1, Y: 1, Z: 5, DigType: "default"},
		{X: 2, Y: 1, Z: 5, DigType: "default"},
		{X: 3, Y: 1, Z: 5, DigType: "default"},
	}
	got := summarizeDryRun(topo, cmds)
	if !strings.Contains(got, "2 into solid ground (good), 1 onto already-open tiles") {
		t.Errorf("open/solid split wrong: %q", got)
	}
}

// TestDigTypeFromNameAcceptsQuickfortAliases routes REAL parseQuickfortGrid
// output (via the public LoadFromCSV entry point, on a small fixture using
// every quickfort dig cell this project's shipped blueprints/*.csv files
// use, plus "x" for ramp removal) through digTypeFromName. Regression test
// for the bug where every stair/channel/ramp cell in every shipped CSV
// silently failed because parseQuickfortGrid's snake_case dig_type strings
// (updown_stair, down_stair, up_stair, remove_ramp) didn't match any case
// digTypeFromName accepted.
func TestDigTypeFromNameAcceptsQuickfortAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.csv")
	// Row of quickfort cells: d, i, j, u, h, r, x — one of every dig
	// designation the format defines (see docs/guides or quickfort's own
	// appendix: d=dig, i=up/down stair, j=down stair, u=up stair,
	// h=channel, r=ramp, x=remove ramp in this project's own parser).
	csv := "#dig start(0;0),,,,,,\nd,i,j,u,h,r,x\n"
	if err := os.WriteFile(path, []byte(csv), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	bp, err := blueprints.LoadFromCSV(path)
	if err != nil {
		t.Fatalf("LoadFromCSV: %v", err)
	}
	if len(bp.Digs) != 7 {
		t.Fatalf("expected 7 dig entries (one per cell), got %d: %+v", len(bp.Digs), bp.Digs)
	}

	for _, dig := range bp.Digs {
		dt, err := digTypeFromName(dig.DigType)
		if dig.DigType == "remove_ramp" {
			if err == nil {
				t.Errorf("remove_ramp: expected the documented not-on-the-wire error, got dig type %d and no error", dt)
			} else if !strings.Contains(err.Error(), "remove_ramp") || !strings.Contains(err.Error(), "wire") {
				t.Errorf("remove_ramp: error not specific/educational: %v", err)
			}
			continue
		}
		if err != nil {
			t.Errorf("dig_type %q (from real parseQuickfortGrid output) failed to resolve: %v", dig.DigType, err)
		}
	}
}

// TestCoalesceDigRuns exercises the pure run-splitting logic:
// same-(digType,Z,Y) tiles sorted by X should merge into maximal
// contiguous runs, with gaps, different Y, and different DigType each
// forcing a new run.
func TestCoalesceDigRuns(t *testing.T) {
	cmds := []blueprints.DigCommand{
		// Contiguous run X=1..3 at (Z=0,Y=5,default).
		{X: 1, Y: 5, Z: 0, DigType: "default"},
		{X: 2, Y: 5, Z: 0, DigType: "default"},
		{X: 3, Y: 5, Z: 0, DigType: "default"},
		// Gap at X=4 -> separate run at X=5, same key.
		{X: 5, Y: 5, Z: 0, DigType: "default"},
		// Different Y -> separate run even though X range overlaps.
		{X: 1, Y: 6, Z: 0, DigType: "default"},
		// Different DigType, same (Z,Y,X) neighborhood -> separate run.
		{X: 1, Y: 5, Z: 0, DigType: "stairs"},
		// Duplicate tile (already covered by the first run) must not
		// inflate the count or fracture the run.
		{X: 2, Y: 5, Z: 0, DigType: "default"},
	}

	runs := coalesceDigRuns(cmds)

	find := func(digType string, z, y, x1, x2 int16) *digRun {
		for i := range runs {
			r := &runs[i]
			if r.DigType == digType && r.Z == z && r.Y == y && r.X1 == x1 && r.X2 == x2 {
				return r
			}
		}
		return nil
	}

	if r := find("default", 0, 5, 1, 3); r == nil {
		t.Errorf("missing contiguous run default (0,5) X1..3; runs=%+v", runs)
	} else if r.Count != 3 {
		t.Errorf("run X1..3 count = %d, want 3 (duplicate at X=2 must not inflate it)", r.Count)
	}
	if r := find("default", 0, 5, 5, 5); r == nil || r.Count != 1 {
		t.Errorf("missing gap-separated run default (0,5) X=5; runs=%+v", runs)
	}
	if r := find("default", 0, 6, 1, 1); r == nil || r.Count != 1 {
		t.Errorf("missing different-Y run default (0,6) X=1; runs=%+v", runs)
	}
	if r := find("stairs", 0, 5, 1, 1); r == nil || r.Count != 1 {
		t.Errorf("missing different-DigType run stairs (0,5) X=1; runs=%+v", runs)
	}

	total := 0
	for _, r := range runs {
		total += r.Count
	}
	if total != 6 { // 7 input tiles minus 1 duplicate
		t.Errorf("total tile count across runs = %d, want 6", total)
	}
}

// cannedAckClient is a commands.DFHackClient that answers every command it is
// handed with one canned ACK status/message. It is the only seam available
// for driving applyBlueprintCmds without a live plugin: Bridge.Exec is a
// concrete *commands.CommandExecutor, but that executor is constructed from
// this interface, so a fake client reaches the real send/ack/track path.
type cannedAckClient struct {
	ackCh  chan *protocol.CommandAckMessage
	status uint8
	errMsg string
	sent   int
}

func (c *cannedAckClient) IsConnected() bool { return true }

func (c *cannedAckClient) SubscribeCommandAcks() <-chan *protocol.CommandAckMessage {
	return c.ackCh
}

func (c *cannedAckClient) SendCommand(cmd *protocol.CommandMessage) error {
	c.sent++
	ack := &protocol.CommandAckMessage{CommandID: cmd.CommandID, Status: c.status, ErrorMsg: c.errMsg}
	go func() { c.ackCh <- ack }()
	return nil
}

// TestApplyBlueprintCmdsPartialRun: a blueprint dig run that crosses an
// existing carved stair now ACKs ACK_STATUS_PARTIAL — the run WAS fully
// designated, with a warning. commands/tracker.go sets Success only for
// ACK_STATUS_SUCCESS, so the old `!res.Success` gate counted every tile of
// that run as FAILED, surfaced the stair warning as the run's "first
// failure", and skipped the b.Digs record whose whole purpose is to stop a
// later designate_dig beside those tiles from hinting "not yet connected".
// A PARTIAL run must count as ok, be recorded, and still speak its warning.
func TestApplyBlueprintCmdsPartialRun(t *testing.T) {
	const warning = "3 designated (2 will remove existing stairs: vertical connection lost)"
	client := &cannedAckClient{
		ackCh:  make(chan *protocol.CommandAckMessage, 8),
		status: protocol.AckStatusPartial,
		errMsg: warning,
	}
	exec := commands.NewCommandExecutor(logging.NewStderrTextLogger("error"), client, 2*time.Second)
	defer exec.Stop()

	b := &Bridge{Exec: exec, Digs: &pendingDigs{}}
	cmds := []blueprints.DigCommand{
		{X: 10, Y: 20, Z: 100, DigType: "default"},
		{X: 11, Y: 20, Z: 100, DigType: "default"},
		{X: 12, Y: 20, Z: 100, DigType: "default"},
	}

	ok, fail, firstErr, partialWarn := applyBlueprintCmds(context.Background(), b, cmds)
	if ok != 3 || fail != 0 {
		t.Fatalf("a fully-designated PARTIAL run must count as ok: ok=%d fail=%d", ok, fail)
	}
	if firstErr != "" {
		t.Fatalf("a PARTIAL run is not a failure and must not populate firstErr: %q", firstErr)
	}
	if !strings.Contains(partialWarn, warning) {
		t.Fatalf("the plugin's stair warning must not be swallowed: %q", partialWarn)
	}
	if !b.Digs.contains(11, 20, 100) {
		t.Fatal("a PARTIAL run's rectangle must still be recorded in pendingDigs")
	}
}

// TestApplyBlueprintCmdsFailedRun is the other half of the same gate: a real
// ACK_STATUS_FAILURE must still count every tile of the run as failed, report
// the plugin's text as the first failure, and leave no pendingDigs record.
func TestApplyBlueprintCmdsFailedRun(t *testing.T) {
	client := &cannedAckClient{
		ackCh:  make(chan *protocol.CommandAckMessage, 8),
		status: protocol.AckStatusFailure,
		errMsg: "designation rejected",
	}
	exec := commands.NewCommandExecutor(logging.NewStderrTextLogger("error"), client, 2*time.Second)
	defer exec.Stop()

	b := &Bridge{Exec: exec, Digs: &pendingDigs{}}
	cmds := []blueprints.DigCommand{
		{X: 10, Y: 20, Z: 100, DigType: "default"},
		{X: 11, Y: 20, Z: 100, DigType: "default"},
	}

	ok, fail, firstErr, partialWarn := applyBlueprintCmds(context.Background(), b, cmds)
	if ok != 0 || fail != 2 {
		t.Fatalf("a FAILED run must count every tile as failed: ok=%d fail=%d", ok, fail)
	}
	if !strings.Contains(firstErr, "designation rejected") {
		t.Fatalf("the plugin's failure text must reach firstErr: %q", firstErr)
	}
	if partialWarn != "" {
		t.Fatalf("a FAILED run must not produce a partial caveat: %q", partialWarn)
	}
	if b.Digs.contains(10, 20, 100) {
		t.Fatal("a FAILED run must not be recorded in pendingDigs")
	}
}
