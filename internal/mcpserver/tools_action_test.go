package mcpserver

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

func TestAckText(t *testing.T) {
	ok := ackText(&commands.CommandResult{Success: true, Status: 0}, nil, "dig 5x5")
	if !strings.Contains(ok, "SUCCESS") || !strings.Contains(ok, "dig 5x5") {
		t.Fatalf("success text wrong: %q", ok)
	}
	partial := ackText(&commands.CommandResult{Success: true, Status: protocol.AckStatusPartial, ErrorMsg: "3 of 25 tiles blocked at (10,11,90)"}, nil, "dig 5x5")
	if !strings.Contains(partial, "PARTIAL") || !strings.Contains(partial, "3 of 25") {
		t.Fatalf("partial must carry plugin text: %q", partial)
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
	got := connectorSuggestion(topo, 3, 0, 0, 5, 2, 0)
	if got != "" {
		t.Fatalf("expected no suggestion for a touching designation, got %q", got)
	}
}

func TestConnectorSuggestion_DisconnectedReturnsSuggestion(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 5)
	for x := int16(0); x <= 2; x++ {
		for y := int16(0); y <= 2; y++ {
			_ = topo.SetTileState(x, y, 0, topology.StateOpen)
		}
	}
	// new designation far away, no shared border
	got := connectorSuggestion(topo, 10, 10, 0, 12, 12, 0)
	if !strings.Contains(got, "not yet connected to existing space") {
		t.Fatalf("expected a connector suggestion, got %q", got)
	}
	if !strings.Contains(got, "designate_dig default") {
		t.Fatalf("expected the suggestion to name a designate_dig call, got %q", got)
	}
}

func TestConnectorSuggestion_EmptyGraphReturnsEmpty(t *testing.T) {
	topo := topology.NewTopologyOverlay(20, 20, 5) // nothing dug yet
	got := connectorSuggestion(topo, 0, 0, 0, 2, 2, 0)
	if got != "" {
		t.Fatalf("expected no suggestion on a fresh embark with nothing dug, got %q", got)
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
	got := connectorSuggestion(topo, 15, 15, 0, 17, 17, 0)
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
	for x := int16(0); x <= 2; x++ {
		for y := int16(0); y <= 2; y++ {
			_ = topo.SetTileState(x, y, 0, topology.StateOpen)
		}
	}
	// Center of this designation is (11,7,0); nearest open tile is (2,2,0)
	// — differs in both x and y, so this must NOT collapse to one line.
	got := connectorSuggestion(topo, 10, 6, 0, 12, 8, 0)
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
	got := connectorSuggestion(topo, 5, 5, 5, 5, 5, 5)
	if got != "" {
		t.Fatalf("expected no suggestion for a vertically-adjacent footprint, got %q", got)
	}
}

func TestBuildWireCoords(t *testing.T) {
	// Workshops (0x10-0x2F): model-facing center -> wire NW corner.
	if x, y := buildWireCoords(0x10, 28, 52); x != 27 || y != 51 {
		t.Fatalf("carpenter center (28,52) should wire as corner (27,51), got (%d,%d)", x, y)
	}
	// 1x1 buildings pass through unchanged.
	for _, bt := range []uint8{0x01, 0x30, 0x50} {
		if x, y := buildWireCoords(bt, 28, 52); x != 28 || y != 52 {
			t.Fatalf("1x1 type 0x%02X must pass through, got (%d,%d)", bt, x, y)
		}
	}
}
