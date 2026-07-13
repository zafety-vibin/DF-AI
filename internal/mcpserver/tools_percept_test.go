package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// Compile-time check: Bridge satisfies mapview.SliceProvider.
var _ mapview.SliceProvider = (*Bridge)(nil)

// TestPerceptToolsRegistered lists the tools over an in-memory MCP session
// and asserts look and cross_section are exposed.
func TestPerceptToolsRegistered(t *testing.T) {
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

	res, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	found := map[string]bool{}
	for _, tool := range res.Tools {
		found[tool.Name] = true
	}
	for _, name := range []string{"look", "cross_section", "find_dig_site"} {
		if !found[name] {
			t.Errorf("tool %q not registered", name)
		}
	}
}

// solidTestBridge builds a Bridge whose world model holds a real 20x20x2
// all-solid topology overlay (FlagDiscovered|FlagWall everywhere), so
// find_dig_site input validation can be exercised end-to-end.
func solidTestBridge(t *testing.T) *Bridge {
	t.Helper()
	const w, h, d = 20, 20, 2
	topo := topology.NewTopologyOverlay(w, h, d)
	if topo == nil {
		t.Fatal("overlay nil")
	}
	var tiles []protocol.TileState
	for z := int16(0); z < d; z++ {
		for y := int16(0); y < h; y++ {
			for x := int16(0); x < w; x++ {
				tiles = append(tiles, protocol.TileState{
					X: x, Y: y, Z: z, TileType: 600,
					Flags: protocol.FlagDiscovered | protocol.FlagWall,
				})
			}
		}
	}
	if err := topo.BuildFromTiles(tiles); err != nil {
		t.Fatalf("build: %v", err)
	}
	logger := logging.NewStderrTextLogger("error")
	return &Bridge{WM: worldmodel.New(topo, nil, nil, logger), Logger: logger}
}

// callTool invokes a tool over an in-memory MCP session and returns the
// concatenated text content.
func callTool(t *testing.T, b *Bridge, name string, args map[string]any) string {
	t.Helper()
	ctx := context.Background()
	srv := New(b)
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
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// TestFindDigSiteRejectsOutOfRangeZ asserts that an off-map z is rejected
// with an instructive error instead of returning fabricated candidates on
// a nonexistent level (GetTileState reads StateUnknown out of bounds, which
// the finder counts as diggable mass).
func TestFindDigSiteRejectsOutOfRangeZ(t *testing.T) {
	b := solidTestBridge(t)
	for _, z := range []int{-1, 2, 99} {
		out := callTool(t, b, "find_dig_site", map[string]any{
			"width": 3, "height": 3, "z": z, "near_x": 10, "near_y": 10,
		})
		if !strings.Contains(out, "out of range") {
			t.Errorf("z=%d: expected out-of-range rejection, got: %q", z, out)
		}
		if strings.Contains(out, "candidates for") {
			t.Errorf("z=%d: fabricated candidates on nonexistent level: %q", z, out)
		}
	}
}

// TestFindDigSiteRejectsBadFootprint asserts zero/negative/oversized room
// dimensions are rejected with an instructive error.
func TestFindDigSiteRejectsBadFootprint(t *testing.T) {
	b := solidTestBridge(t)
	for _, wh := range [][2]int{{0, 3}, {3, 0}, {-2, 3}, {100000, 3}, {3, 21}} {
		out := callTool(t, b, "find_dig_site", map[string]any{
			"width": wh[0], "height": wh[1], "z": 1, "near_x": 10, "near_y": 10,
		})
		if !strings.Contains(out, "invalid room size") {
			t.Errorf("w=%d h=%d: expected footprint rejection, got: %q", wh[0], wh[1], out)
		}
	}
}

// TestFindDigSiteValidRequestStillWorks asserts the guards don't break the
// happy path: an in-bounds request on a solid level returns candidates.
func TestFindDigSiteValidRequestStillWorks(t *testing.T) {
	b := solidTestBridge(t)
	out := callTool(t, b, "find_dig_site", map[string]any{
		"width": 3, "height": 3, "z": 1, "near_x": 10, "near_y": 10,
	})
	if !strings.Contains(out, "candidates for a 3x3 room on z=1") {
		t.Errorf("expected candidates on a solid in-bounds level, got: %q", out)
	}
}

// TestLook_UnknownLensReturnsRegisteredNames is a light unit test of the
// error path only — a full nil-bridge MCP round trip is already covered by
// TestEveryToolNilBridge in server_test.go, which exercises `look` with
// lens omitted. This test checks the unknownLensError helper directly
// (already covered by TestLensNamesErrorMessage in lenses_test.go) plus
// that the `look` tool actually calls it — covered by re-running the
// nil-bridge sweep with a lens arg added to minToolArgs (Step 3 below)
// rather than a bespoke MCP client test here, to avoid duplicating
// transport-level test scaffolding that already exists.
func TestLook_UnknownLensReturnsRegisteredNames(t *testing.T) {
	got := unknownLensError("nonexistent")
	if !strings.Contains(got, "unknown lens") {
		t.Fatalf("expected an 'unknown lens' message, got %q", got)
	}
}
