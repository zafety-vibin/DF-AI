package mcpserver

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

func TestAckText(t *testing.T) {
	ok := ackText(&commands.CommandResult{Success: true, Status: 0}, nil, "dig 5x5")
	if !strings.Contains(ok, "SUCCESS") || !strings.Contains(ok, "dig 5x5") {
		t.Fatalf("success text wrong: %q", ok)
	}
	partial := ackText(&commands.CommandResult{Status: protocol.AckStatusPartial, ErrorMsg: "3 of 25 tiles blocked at (10,11,90)"}, nil, "dig 5x5")
	if !strings.Contains(partial, "PARTIAL") || !strings.Contains(partial, "3 of 25") {
		t.Fatalf("partial must carry plugin text: %q", partial)
	}
	// A SUCCESS-status ack carrying an informational note must render as
	// SUCCESS with the note verbatim — not get downgraded to PARTIAL from
	// mere message presence (regression: remove_zone's clean success note).
	note := ackText(&commands.CommandResult{Success: true, Status: protocol.AckStatusSuccess, ErrorMsg: "zone removed immediately"}, nil, "remove_zone id=3")
	if !strings.HasPrefix(note, "SUCCESS") || !strings.Contains(note, "zone removed immediately") {
		t.Fatalf("success-with-note must stay SUCCESS and carry the note: %q", note)
	}
	failed := ackText(nil, errors.New("timeout waiting for ACK"), "dig 5x5")
	if !strings.Contains(failed, "FAILED") || !strings.Contains(failed, "timeout") {
		t.Fatalf("failure text wrong: %q", failed)
	}
}

func TestDigTypeMapper(t *testing.T) {
	if v, err := digTypeFromName("stairs"); err != nil || v != protocol.DigTypeUpDownStair {
		t.Fatalf("stairs mapping wrong: %v %v", v, err)
	}
	if _, err := digTypeFromName("lasergun"); err == nil {
		t.Fatal("unknown dig type must error")
	}
}

// TestClampFlatDigZ2 is the regression test for the live incident where a
// flat-type designate_dig call with z1 != z2 silently painted every
// intervening z-level (a typo painted a 4x5 block through 30 z-levels and
// two aquifers). Flat types (default/channel/ramp) must clamp z2 to z1 and
// carry a truthful note; the stair types legitimately span a z-range and
// must pass through untouched.
func TestClampFlatDigZ2(t *testing.T) {
	for _, name := range []string{"default", "channel", "ramp"} {
		dt, err := digTypeFromName(name)
		if err != nil {
			t.Fatalf("digTypeFromName(%q): %v", name, err)
		}
		z2, note := clampFlatDigZ2(name, dt, 100, 70)
		if z2 != 100 {
			t.Fatalf("%s: expected z2 clamped to z1=100, got %d", name, z2)
		}
		if note == "" || !strings.Contains(note, "one z-level per call") {
			t.Fatalf("%s: expected a truthful clamp note, got %q", name, note)
		}
	}
	for _, name := range []string{"stairs", "upstair", "downstair"} {
		dt, err := digTypeFromName(name)
		if err != nil {
			t.Fatalf("digTypeFromName(%q): %v", name, err)
		}
		z2, note := clampFlatDigZ2(name, dt, 100, 70)
		if z2 != 70 {
			t.Fatalf("%s: stair types must span the requested z-range unclamped, got z2=%d", name, z2)
		}
		if note != "" {
			t.Fatalf("%s: stair types must not carry a clamp note, got %q", name, note)
		}
	}
	// z1 == z2 for a flat type: nothing to clamp, no note.
	dt, _ := digTypeFromName("default")
	if z2, note := clampFlatDigZ2("default", dt, 50, 50); z2 != 50 || note != "" {
		t.Fatalf("equal z1/z2 must not produce a clamp note, got z2=%d note=%q", z2, note)
	}
}

func TestStockpileGroupsAll(t *testing.T) {
	if stockpileGroups["all"] != protocol.StockpileGroupAll {
		t.Fatalf("stockpileGroups[all] = %#x, want %#x", stockpileGroups["all"], protocol.StockpileGroupAll)
	}
	if len(stockpileGroups) != 18 { // 17 categories + "all"
		t.Fatalf("stockpileGroups has %d entries, want 18", len(stockpileGroups))
	}
}

// TestActionToolsNilBridge proves the action tools are registered on the
// server and that a nil bridge (scaffold contract in New) reports the
// missing connection instead of panicking.
func TestActionToolsNilBridge(t *testing.T) {
	ctx := context.Background()
	srv := New(nil)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer clientSession.Close()

	calls := []struct {
		name string
		args map[string]any
	}{
		{"designate_dig", map[string]any{
			"type": "stairs", "x1": 10, "y1": 10, "z1": 100, "x2": 11, "y2": 11, "z2": 94,
		}},
		{"chop", map[string]any{"x1": 10, "y1": 10, "z": 110, "x2": 30, "y2": 30}},
		{"gather", map[string]any{"x1": 10, "y1": 10, "z": 110, "x2": 30, "y2": 30}},
		{"cancel_designation", map[string]any{"x1": 10, "y1": 10, "z": 110, "x2": 30, "y2": 30}},
		{"bring_goods_to_depot", map[string]any{"x": 28, "y": 52, "z": 110, "max_count": 10}},
		{"unmark_trade_goods", map[string]any{"x": 28, "y": 52, "z": 110, "max_count": 10}},
		{"set_depot_trade_flags", map[string]any{"x": 28, "y": 52, "z": 110, "trader_requested": true, "anyone_can_trade": false}},
		{"appoint_position", map[string]any{"unit_id": 12, "position_code": "MANAGER"}},
		{"set_bookkeeper_precision", map[string]any{"precision": "nearest_100"}},
		{"cancel_order", map[string]any{"id": 5}},
		{"edit_order", map[string]any{"id": 5, "amount": 10}},
	}
	for _, call := range calls {
		res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
			Name: call.name, Arguments: call.args,
		})
		if err != nil {
			t.Fatalf("call %s: %v", call.name, err)
		}
		tc, ok := res.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatalf("%s: expected *mcp.TextContent, got %T", call.name, res.Content[0])
		}
		if !strings.Contains(tc.Text, "NOT CONNECTED") {
			t.Errorf("%s: expected NOT CONNECTED message, got %q", call.name, tc.Text)
		}
	}
}

func TestConnectorSuggestion_TouchingRegionReturnsEmpty(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 5)
	// existing open room at (0,0,0)-(2,2,0)
	for x := int16(0); x <= 2; x++ {
		for y := int16(0); y <= 2; y++ {
			_ = topo.SetTileState(x, y, 0, topology.StateOpen)
		}
	}
	// new designation directly adjacent (touches x=3, which borders x=2)
	got := connectorSuggestion(context.Background(), nil, topo, nil, 3, 0, 0, 5, 2, 0)
	if got != "" {
		t.Fatalf("expected no suggestion for a touching designation, got %q", got)
	}
}

func TestConnectorSuggestion_DisconnectedReturnsSuggestion(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 5)
	// 6x6 room (36 open tiles) — comfortably above the coverage gate's
	// absolute floor so this test still exercises the suggestion path,
	// not the gate (see TestConnectorSuggestion_SparseOverlaySuppresses).
	for x := int16(0); x <= 5; x++ {
		for y := int16(0); y <= 5; y++ {
			_ = topo.SetTileState(x, y, 0, topology.StateOpen)
		}
	}
	// new designation far away, no shared border
	got := connectorSuggestion(context.Background(), nil, topo, nil, 10, 10, 0, 12, 12, 0)
	if !strings.Contains(got, "not yet connected to existing space") {
		t.Fatalf("expected a connector suggestion, got %q", got)
	}
	if !strings.Contains(got, "designate_dig default") {
		t.Fatalf("expected the suggestion to name a designate_dig call, got %q", got)
	}
}

func TestConnectorSuggestion_EmptyGraphReturnsEmpty(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 5) // nothing dug yet
	got := connectorSuggestion(context.Background(), nil, topo, nil, 0, 0, 0, 2, 2, 0)
	if got != "" {
		t.Fatalf("expected no suggestion on a fresh embark with nothing dug, got %q", got)
	}
}

// TestConnectorSuggestion_SparseOverlaySuppressesSuggestion is the
// regression test for the coverage gate (2026-07-19 live incident: a
// reconnect/resync left the overlay's known-open coverage tiny, and
// NearestRegion pointed at a real-but-misleading tile near a map corner).
// A handful of known-open tiles anywhere on the map is not enough to trust
// a NearestRegion answer — the gate must return "" instead.
func TestConnectorSuggestion_SparseOverlaySuppressesSuggestion(t *testing.T) {
	topo := topology.NewTopologyOverlay(50, 50, 10)
	_ = topo.SetTileState(0, 0, 9, topology.StateOpen)
	_ = topo.SetTileState(1, 0, 9, topology.StateOpen)

	got := connectorSuggestion(context.Background(), nil, topo, nil, 30, 30, 5, 34, 34, 5)
	if got != "" {
		t.Fatalf("expected the coverage gate to suppress a suggestion on a sparse overlay, got %q", got)
	}
}

// TestConnectorSuggestion_WellCoveredOverlayStillSuggests proves the gate
// added above only suppresses the sparse case — a topology with plenty of
// known-open tiles relative to both the absolute floor and the designated
// rectangle must still produce a real suggestion.
func TestConnectorSuggestion_WellCoveredOverlayStillSuggests(t *testing.T) {
	topo := topology.NewTopologyOverlay(50, 50, 10)
	for x := int16(0); x <= 9; x++ {
		for y := int16(0); y <= 9; y++ {
			_ = topo.SetTileState(x, y, 0, topology.StateOpen)
		}
	}
	got := connectorSuggestion(context.Background(), nil, topo, nil, 30, 30, 0, 32, 32, 0)
	if !strings.Contains(got, "not yet connected to existing space") {
		t.Fatalf("expected a real suggestion on a well-covered overlay, got %q", got)
	}
}

// TestConnectorSuggestion_TargetIsARealRegionTile is the regression test
// for the bug where connectorSuggestion used a region's bounding-box min
// corner as the suggested connector target. For a non-rectangular region
// (this fixture: three walls of a square — left bar x=0,y=3..10; bottom
// bar y=10,x=0..10; right bar x=10,y=0..10, all one connected region
// under 4-connectivity via the shared corners (0,10) and (10,10)) the
// BBox min corner is (0,0,0) — a tile the flood-fill never visited and
// that is not a member of the region at all. The suggested target must
// instead be an actual tile of the nearest region.
func TestConnectorSuggestion_TargetIsARealRegionTile(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 5)
	// left bar: x=0, y=3..10
	for y := int16(3); y <= 10; y++ {
		_ = topo.SetTileState(0, y, 0, topology.StateOpen)
	}
	// bottom bar: y=10, x=0..10
	for x := int16(0); x <= 10; x++ {
		_ = topo.SetTileState(x, 10, 0, topology.StateOpen)
	}
	// right bar: x=10, y=0..10
	for y := int16(0); y <= 10; y++ {
		_ = topo.SetTileState(10, y, 0, topology.StateOpen)
	}

	// Sanity-check the fixture: BBox[0] must be (0,0,0), and that tile
	// must NOT be a member of the region — otherwise this test doesn't
	// exercise the bug at all.
	rg := topology.BuildRegionGraph(topo)
	region, ok := rg.RegionAt(topology.Coord{X: 0, Y: 10, Z: 0})
	if !ok {
		t.Fatal("fixture setup: expected the C-shaped region to exist")
	}
	if region.BBox[0] != (topology.Coord{X: 0, Y: 0, Z: 0}) {
		t.Fatalf("fixture setup: expected BBox min corner (0,0,0), got %+v", region.BBox[0])
	}
	if _, ok := rg.RegionAt(topology.Coord{X: 0, Y: 0, Z: 0}); ok {
		t.Fatal("fixture setup: (0,0,0) must NOT be a member of the region — it's the trap corner")
	}

	// A disconnected designation nearby (far enough that it doesn't touch
	// the C shape's open tiles).
	got := connectorSuggestion(context.Background(), nil, topo, nil, 15, 15, 0, 17, 17, 0)
	if got == "" {
		t.Fatal("expected a connector suggestion for a disconnected designation")
	}

	// Whether this renders as one straight leg or an L-shaped pair of
	// legs (see TestConnectorSuggestion_NonCollinearSuggestsLShape), the
	// FINAL destination named in the suggestion must be a real tile the
	// flood-fill actually visited — never a synthesized bounding-box
	// corner. Parse the last "(...)" tuple in the string, whichever shape
	// the message took.
	lastOpen := strings.LastIndex(got, "(")
	lastClose := strings.LastIndex(got, ")")
	if lastOpen == -1 || lastClose == -1 || lastClose < lastOpen {
		t.Fatalf("could not find a target coordinate tuple in %q", got)
	}
	parts := strings.Split(got[lastOpen+1:lastClose], ",")
	if len(parts) != 3 {
		t.Fatalf("expected 3 coordinate parts, got %d in %q", len(parts), got)
	}
	tx, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	ty, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	tz, err3 := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("failed to parse target coordinates from %q: %v %v %v", got, err1, err2, err3)
	}

	target := topology.Coord{X: int16(tx), Y: int16(ty), Z: int16(tz)}
	if _, ok := topology.BuildRegionGraph(topo).RegionAt(target); !ok {
		t.Fatalf("suggested connector target %+v is not a member of any real region — flood-fill never visited it (parsed from %q)", target, got)
	}
}

// TestConnectorSuggestion_NonCollinearSuggestsLShape is the regression test
// for bug (a): designate_dig's two endpoints define a RECTANGLE, so a
// single suggestion from a disconnected designation's center straight to a
// diagonally-placed nearest tile would designate a giant bounding box
// instead of a corridor. When center and target share neither x nor y (and
// are on the same z), the suggestion must break into two axis-aligned legs
// instead of one naive box.
func TestConnectorSuggestion_NonCollinearSuggestsLShape(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 5)
	// 6x6 room (36 open tiles) — comfortably above the coverage gate's
	// absolute floor (see TestConnectorSuggestion_DisconnectedReturnsSuggestion).
	for x := int16(0); x <= 5; x++ {
		for y := int16(0); y <= 5; y++ {
			_ = topo.SetTileState(x, y, 0, topology.StateOpen)
		}
	}
	// Center of this designation is (11,7,0); nearest open tile differs in
	// both x and y, so this must NOT collapse to one line.
	got := connectorSuggestion(context.Background(), nil, topo, nil, 10, 6, 0, 12, 8, 0)
	if got == "" {
		t.Fatal("expected a connector suggestion for a disconnected designation")
	}
	if n := strings.Count(got, "designate_dig default"); n != 2 {
		t.Fatalf("expected a two-leg L-shaped suggestion (2 designate_dig calls), got %d in %q", n, got)
	}
	if !strings.Contains(got, "L-shaped") {
		t.Fatalf("expected the suggestion to name itself L-shaped, got %q", got)
	}
}

// TestConnectorSuggestion_VerticallyAdjacentReturnsEmpty is the regression
// test for bug (b): the connectivity scan only checked the lateral ring at
// each swept Z, never the footprint tiles directly above/below — so a
// designation whose only connection is vertical (a surface shaft's open
// top, a room dug directly over open space) false-positived as
// disconnected. An open tile directly above the footprint must suppress
// the suggestion entirely.
func TestConnectorSuggestion_VerticallyAdjacentReturnsEmpty(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 10)
	// Open tile directly above the designation's 1x1 footprint — nowhere
	// near its lateral ring, only reachable by looking straight up.
	_ = topo.SetTileState(5, 5, 4, topology.StateOpen)
	got := connectorSuggestion(context.Background(), nil, topo, nil, 5, 5, 5, 5, 5, 5)
	if got != "" {
		t.Fatalf("expected no suggestion for a vertically-adjacent footprint, got %q", got)
	}
}

// TestConnectorSuggestion_PendingDesignationSuppresses is the regression
// test for the dig-ahead false positive: a stair spine designated but not
// yet carved is hidden, so its tiles classify Unknown in the topology
// overlay — a room designated beside it read "not yet connected" and the
// nearest-open-region fallback pointed at distant wilderness (map edge).
// With the spine's ACKed dig rect recorded in pendingDigs, the adjoining
// room must be treated as connected (no suggestion at all).
func TestConnectorSuggestion_PendingDesignationSuppresses(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 10)
	// Distant open "wilderness" so the region graph is non-empty — this is
	// what the false positive used to point at. Sized as a 5x5 block (25
	// open tiles), comfortably above the coverage gate's absolute floor
	// and above the 4x4 rectangle designated below, so the no-digs
	// sub-test below still exercises the real suggestion path rather than
	// tripping the gate (see TestConnectorSuggestion_SparseOverlaySuppresses).
	for x := int16(0); x <= 4; x++ {
		for y := int16(0); y <= 4; y++ {
			_ = topo.SetTileState(x, y, 9, topology.StateOpen)
		}
	}

	digs := &pendingDigs{}
	// The spine: a 2x2 stairs designation spanning many z, ACKed earlier
	// this session, still solid/hidden in the topology overlay.
	digs.add(10, 10, 8, 11, 11, 2)

	// A room designated beside the spine at z=5 — its ring touches (11,11,5).
	got := connectorSuggestion(context.Background(), nil, topo, digs, 12, 10, 5, 15, 13, 5)
	if got != "" {
		t.Fatalf("room adjoining a pending dig designation must count as connected, got %q", got)
	}

	// Same room WITHOUT the recorded rect still gets the suggestion —
	// proves the suppression comes from pendingDigs, not the fixture.
	if got := connectorSuggestion(context.Background(), nil, topo, nil, 12, 10, 5, 15, 13, 5); !strings.Contains(got, "not yet connected") {
		t.Fatalf("expected a suggestion without the dig record, got %q", got)
	}

	// Vertical variant: a designation directly UNDER a pending rect's
	// bottom is connected through its footprint scan.
	if got := connectorSuggestion(context.Background(), nil, topo, digs, 10, 10, 1, 11, 11, 1); got != "" {
		t.Fatalf("designation under a pending shaft must count as connected, got %q", got)
	}
}

// liveSliceFetcher adapts a plain func to the anchorLiveFetcher interface
// connectorSuggestion's live recheck depends on — the same fake-seam
// pattern as namePlaceTestBridge's canned Bridge.mapSliceFn (places_test.go),
// but here as a standalone fake since connectorSuggestion takes the
// interface directly, not a *Bridge.
type liveSliceFetcher struct {
	fn func(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error)
}

func (f *liveSliceFetcher) MapSlice(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error) {
	return f.fn(ctx, x1, y1, z, x2, y2)
}

// solidSliceWithOpenTile builds a same-shaped-as-requested Slice of solid
// ('#') glyphs, with (openX,openY) marked as open floor ('.') if it falls
// inside the requested window — the minimal fixture for exercising the live
// recheck's glyph classification without hand-writing row strings per test.
func solidSliceWithOpenTile(openX, openY int16) func(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error) {
	return func(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error) {
		w, h := int(x2-x1)+1, int(y2-y1)+1
		rows := make([]string, h)
		for i := range rows {
			rows[i] = strings.Repeat("#", w)
		}
		if openX >= x1 && openX <= x2 && openY >= y1 && openY <= y2 {
			row := []byte(rows[openY-y1])
			row[openX-x1] = '.'
			rows[openY-y1] = string(row)
		}
		return &mapview.Slice{Z: z, X1: x1, Y1: y1, Rows: rows}, nil
	}
}

// TestConnectorSuggestion_LiveRecheckSuppressesStaleSameZAdjacency is the
// regression test for the Iron Quarter incident (fortress/memory/goals.md,
// 2026-07-22): a same-z corridor tile dug and ACKed moments ago can still
// read StateUnknown in the topology overlay (fed solely by the TILE_UPDATE
// stream, which empirically delivers nothing) even though live game state
// shows it open. The live recheck must classify this as connected and
// suppress the suggestion — the overlay itself is left deliberately
// unseeded near the designation to reproduce the staleness.
func TestConnectorSuggestion_LiveRecheckSuppressesStaleSameZAdjacency(t *testing.T) {
	topo := topology.NewTopologyOverlay(50, 50, 10)
	// Distant, well-covered region so the region graph is non-empty and
	// clears the coverage gate — this is what the false positive used to
	// point at before the live recheck existed.
	for x := int16(0); x <= 9; x++ {
		for y := int16(0); y <= 9; y++ {
			_ = topo.SetTileState(x, y, 0, topology.StateOpen)
		}
	}
	// Live data shows an open tile at (20,20,0), directly on the ring
	// bordering the designation below (x1-1==20, y=20 within [19,21]) —
	// but the overlay was never seeded there, so the stale-overlay ring
	// scan alone would miss it.
	live := &liveSliceFetcher{fn: solidSliceWithOpenTile(20, 20)}

	got := connectorSuggestion(context.Background(), live, topo, nil, 21, 19, 0, 23, 21, 0)
	if got != "" {
		t.Fatalf("expected the live recheck to suppress the suggestion for a same-z adjacent tile the overlay hasn't caught up on, got %q", got)
	}
}

// TestConnectorSuggestion_LiveRecheckStillSuggestsWhenGenuinelyDetached
// proves the live recheck only ever short-circuits to "connected" — when
// live data also shows the surrounding area solid, the existing coverage-
// gate/NearestRegion suggestion must still fire unchanged.
func TestConnectorSuggestion_LiveRecheckStillSuggestsWhenGenuinelyDetached(t *testing.T) {
	topo := topology.NewTopologyOverlay(50, 50, 10)
	for x := int16(0); x <= 9; x++ {
		for y := int16(0); y <= 9; y++ {
			_ = topo.SetTileState(x, y, 0, topology.StateOpen)
		}
	}
	// (999,999) is never inside any requested window, so this always
	// returns an all-solid slice — live data agrees with the overlay that
	// the designation is genuinely detached.
	live := &liveSliceFetcher{fn: solidSliceWithOpenTile(999, 999)}

	got := connectorSuggestion(context.Background(), live, topo, nil, 30, 30, 0, 32, 32, 0)
	if !strings.Contains(got, "not yet connected to existing space") {
		t.Fatalf("expected a real suggestion when live data also shows the area solid, got %q", got)
	}
}

// TestConnectorSuggestion_LiveRecheckSkippedBeyondZSpanCap proves the live
// recheck is bounded to connectorLiveRecheckMaxZSpan — a tall multi-z stair
// shaft must keep today's unchanged coverage-gate/NearestRegion behavior
// rather than costing one plugin round trip per z-level. The live fetcher
// here would (wrongly) suppress the suggestion if it were ever consulted,
// so a non-empty suggestion proves it was skipped.
func TestConnectorSuggestion_LiveRecheckSkippedBeyondZSpanCap(t *testing.T) {
	topo := topology.NewTopologyOverlay(50, 50, 20)
	for x := int16(0); x <= 9; x++ {
		for y := int16(0); y <= 9; y++ {
			_ = topo.SetTileState(x, y, 0, topology.StateOpen)
		}
	}
	live := &liveSliceFetcher{fn: func(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error) {
		t.Fatal("live fetcher must not be consulted when the z-span exceeds the cap")
		return nil, nil
	}}

	z1, z2 := int16(0), int16(connectorLiveRecheckMaxZSpan+2)
	got := connectorSuggestion(context.Background(), live, topo, nil, 30, 30, z1, 32, 32, z2)
	if !strings.Contains(got, "not yet connected to existing space") {
		t.Fatalf("expected the unchanged suggestion path beyond the z-span cap, got %q", got)
	}
}

func TestPendingDigs(t *testing.T) {
	var nilDigs *pendingDigs
	nilDigs.add(0, 0, 0, 1, 1, 1) // must not panic
	if nilDigs.contains(0, 0, 0) {
		t.Fatal("nil pendingDigs must contain nothing")
	}

	digs := &pendingDigs{}
	if digs.contains(5, 5, 5) {
		t.Fatal("empty pendingDigs must contain nothing")
	}
	// Corners in reversed order must normalize.
	digs.add(11, 11, 2, 10, 10, 8)
	for _, tc := range []struct {
		x, y, z int16
		want    bool
	}{
		{10, 10, 2, true},
		{11, 11, 8, true},
		{10, 11, 5, true},
		{12, 10, 5, false}, // outside x
		{10, 10, 9, false}, // outside z
	} {
		if got := digs.contains(tc.x, tc.y, tc.z); got != tc.want {
			t.Errorf("contains(%d,%d,%d) = %v, want %v", tc.x, tc.y, tc.z, got, tc.want)
		}
	}
}

func TestBuildWireCoords(t *testing.T) {
	// Workshops (0x10-0x2F): model-facing center -> wire NW corner.
	if x, y := buildWireCoords(0x10, 28, 52); x != 27 || y != 51 {
		t.Fatalf("carpenter center (28,52) should wire as corner (27,51), got (%d,%d)", x, y)
	}
	// Metalsmith's forge is still a Workshop (0x19) -- same 3x3 offset.
	if x, y := buildWireCoords(protocol.BuildTypeWorkshopMetalsmith, 28, 52); x != 27 || y != 51 {
		t.Fatalf("metalsmith center (28,52) should wire as corner (27,51), got (%d,%d)", x, y)
	}
	// Furnaces (0x70-0x7F) are 3x3 like workshops.
	if x, y := buildWireCoords(protocol.BuildTypeFurnaceSmelter, 28, 52); x != 27 || y != 51 {
		t.Fatalf("smelter center (28,52) should wire as corner (27,51), got (%d,%d)", x, y)
	}
	// Trade depot (0x80-0x8F) is 5x5 -- offset -2,-2.
	if x, y := buildWireCoords(protocol.BuildTypeTradeDepot, 28, 52); x != 26 || y != 50 {
		t.Fatalf("depot center (28,52) should wire as corner (26,50), got (%d,%d)", x, y)
	}
	// 1x1 buildings pass through unchanged.
	for _, bt := range []uint8{0x01, 0x30, 0x50} {
		if x, y := buildWireCoords(bt, 28, 52); x != 28 || y != 52 {
			t.Fatalf("1x1 type 0x%02X must pass through, got (%d,%d)", bt, x, y)
		}
	}
}

func TestBuildFootprintCorner(t *testing.T) {
	// Odd footprints match the fixed-size buildWireCoords offsets exactly
	// (3/2==1, 5/2==2 -- same center=size/2 formula DF itself uses).
	if x, y := buildFootprintCorner(28, 52, 3, 3); x != 27 || y != 51 {
		t.Fatalf("3x3 center (28,52) should wire as corner (27,51), got (%d,%d)", x, y)
	}
	if x, y := buildFootprintCorner(28, 52, 5, 5); x != 26 || y != 50 {
		t.Fatalf("5x5 center (28,52) should wire as corner (26,50), got (%d,%d)", x, y)
	}
	// 1x1 passes through unchanged.
	if x, y := buildFootprintCorner(28, 52, 1, 1); x != 28 || y != 52 {
		t.Fatalf("1x1 center (28,52) should pass through, got (%d,%d)", x, y)
	}
	// Even footprint (bridges are not forced to odd sizes): 4/2==2.
	if x, y := buildFootprintCorner(28, 52, 4, 2); x != 26 || y != 51 {
		t.Fatalf("4x2 center (28,52) should wire as corner (26,51), got (%d,%d)", x, y)
	}
}
