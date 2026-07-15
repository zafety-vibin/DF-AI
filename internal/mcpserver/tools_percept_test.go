package mcpserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/modifications"
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

// TestClampLookRadius covers the default/cap arithmetic look's radius
// parameter uses, including the raised 23 cap (up from 15) — 47x47 is the
// largest crop that still fits under the plugin's 48x48 map_slice region
// cap without a plugin-side change.
func TestClampLookRadius(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, 12},   // unset -> default
		{-5, 12},  // negative -> default
		{1, 1},    // pass-through below the cap
		{23, 23},  // exactly at the cap
		{24, 23},  // one over -> clamped
		{40, 23},  // scout note's example
		{1000, 23},
	}
	for _, c := range cases {
		if got := clampLookRadius(c.in); got != c.want {
			t.Errorf("clampLookRadius(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestLookScopeOverview_NilBridgeReadable exercises look's scope=overview
// branch against a nil bridge: b.Topo() must nil-safely report "not built
// yet" instead of panicking, matching the harness-wide nil-bridge contract
// (TestEveryToolNilBridge in server_test.go covers every tool's default
// path; this covers the scope=overview branch specifically since it isn't
// in minToolArgs).
func TestLookScopeOverview_NilBridgeReadable(t *testing.T) {
	out := callTool(t, nil, "look", map[string]any{"x": 10, "y": 10, "z": 5, "scope": "overview"})
	if !strings.Contains(out, "topology not built yet") {
		t.Fatalf("expected a readable 'not built yet' message, got: %q", out)
	}
}

// TestLookScopeOverview_RendersFromResidentTopology asserts scope=overview
// renders a whole-map view straight from the Bridge's resident
// TopologyOverlay, with no plugin round-trip required (solidTestBridge has
// no Client wired at all — MapSlice would fail "plugin not connected" if
// this path touched it).
func TestLookScopeOverview_RendersFromResidentTopology(t *testing.T) {
	b := solidTestBridge(t) // 20x20x2, all solid/closed
	out := callTool(t, b, "look", map[string]any{"x": 10, "y": 10, "z": 1, "scope": "overview"})
	if !strings.Contains(out, "orientation overview") {
		t.Fatalf("expected overview header, got: %q", out)
	}
	if !strings.Contains(out, "20x20 map downsampled") {
		t.Fatalf("expected map dimensions in header, got: %q", out)
	}
	if strings.Contains(out, "plugin not connected") {
		t.Fatalf("overview scope made a plugin round-trip, it shouldn't: %q", out)
	}
}

// TestLookScopeOverview_RejectsOutOfRangeZ asserts the same z bounds
// checking find_dig_site does, so overview never fabricates a grid for a
// nonexistent level.
func TestLookScopeOverview_RejectsOutOfRangeZ(t *testing.T) {
	b := solidTestBridge(t) // depth 2: valid z is 0..1
	out := callTool(t, b, "look", map[string]any{"x": 10, "y": 10, "z": 99, "scope": "overview"})
	if !strings.Contains(out, "out of range") {
		t.Fatalf("expected out-of-range rejection, got: %q", out)
	}
}

// TestLookUnknownScope asserts an unrecognized scope value gets a specific,
// educational rejection rather than silently falling back to local.
func TestLookUnknownScope(t *testing.T) {
	out := callTool(t, nil, "look", map[string]any{"x": 10, "y": 10, "z": 5, "scope": "bogus"})
	if !strings.Contains(out, "unknown scope") {
		t.Fatalf("expected unknown-scope rejection, got: %q", out)
	}
}

// TestLookScopeElevation_NilBridgeReadable mirrors
// TestLookScopeOverview_NilBridgeReadable for the elevation scope: nil
// bridge must nil-safely report "not built yet" instead of panicking.
func TestLookScopeElevation_NilBridgeReadable(t *testing.T) {
	out := callTool(t, nil, "look", map[string]any{"x": 10, "y": 10, "z": 5, "scope": "elevation"})
	if !strings.Contains(out, "topology not built yet") {
		t.Fatalf("expected a readable 'not built yet' message, got: %q", out)
	}
}

// TestLookScopeFort_NilBridgeReadable mirrors the same nil-bridge contract
// for the fort scope's topology check.
func TestLookScopeFort_NilBridgeReadable(t *testing.T) {
	out := callTool(t, nil, "look", map[string]any{"x": 10, "y": 10, "z": 5, "scope": "fort"})
	if !strings.Contains(out, "topology not built yet") {
		t.Fatalf("expected a readable 'not built yet' message, got: %q", out)
	}
}

// TestLookScopeFort_ModsNilButTopoPresent: solidTestBridge wires a real
// topology but no modifications overlay (worldmodel.New's mods param is
// nil) — fort scope must report the modifications-specific unavailable
// message, not the topology one, and never a nil-pointer panic.
func TestLookScopeFort_ModsNilButTopoPresent(t *testing.T) {
	b := solidTestBridge(t)
	out := callTool(t, b, "look", map[string]any{"x": 10, "y": 10, "z": 1, "scope": "fort"})
	if !strings.Contains(out, "modifications overlay not built yet") {
		t.Fatalf("expected modifications-not-built message, got: %q", out)
	}
}

// TestLookScopeFort_NoModificationsAtZ: an empty (but present) modification
// overlay at the requested z must produce the truthful "nothing recorded"
// message, never a silent fallback to some other crop.
func TestLookScopeFort_NoModificationsAtZ(t *testing.T) {
	b := solidTestBridge(t)
	b.WM.Observed.Modifications = modifications.NewModificationOverlay(modifications.Bounds{Width: 20, Height: 20, Depth: 2})
	out := callTool(t, b, "look", map[string]any{"x": 10, "y": 10, "z": 1, "scope": "fort"})
	if !strings.Contains(out, "no modifications recorded on z=1") {
		t.Fatalf("expected truthful no-modifications message, got: %q", out)
	}
}

// TestFortFootprintBBox_EmptyIsNotOK asserts an overlay with no
// modifications at the requested z reports ok=false — the signal callers
// use to return a truthful "nothing here yet" message instead of computing
// a bogus bbox.
func TestFortFootprintBBox_EmptyIsNotOK(t *testing.T) {
	mods := modifications.NewModificationOverlay(modifications.Bounds{Width: 50, Height: 50, Depth: 5})
	if _, _, _, _, ok := fortFootprintBBox(mods, 50, 50, 3, 8); ok {
		t.Fatal("expected ok=false with no modifications recorded")
	}
}

// TestFortFootprintBBox_MarginAndClamp exercises the core bbox arithmetic:
// two modified tiles at z=3 define a (2,2)-(10,10) bbox; +8 margin clamped
// at 0 on the low side. A modification at a DIFFERENT z must not widen the
// z=3 bbox — GetModificationsInRegion's z filter is load-bearing here.
func TestFortFootprintBBox_MarginAndClamp(t *testing.T) {
	mods := modifications.NewModificationOverlay(modifications.Bounds{Width: 50, Height: 50, Depth: 5})
	for _, c := range [][2]int16{{2, 2}, {10, 10}} {
		if err := mods.Add(modifications.Coordinate{X: c[0], Y: c[1], Z: 3},
			modifications.ModificationInfo{Type: modifications.ModificationDug, DetectedAt: time.Now()}); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := mods.Add(modifications.Coordinate{X: 40, Y: 40, Z: 4},
		modifications.ModificationInfo{Type: modifications.ModificationDug, DetectedAt: time.Now()}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	x0, y0, x1, y1, ok := fortFootprintBBox(mods, 50, 50, 3, 8)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if x0 != 0 || y0 != 0 || x1 != 18 || y1 != 18 {
		t.Fatalf("bbox = (%d,%d)-(%d,%d), want (0,0)-(18,18)", x0, y0, x1, y1)
	}
}

// TestFortFootprintBBox_ClampsHighSideToMapBounds checks the high-side
// clamp independently of the low-side clamp covered above.
func TestFortFootprintBBox_ClampsHighSideToMapBounds(t *testing.T) {
	mods := modifications.NewModificationOverlay(modifications.Bounds{Width: 20, Height: 20, Depth: 5})
	if err := mods.Add(modifications.Coordinate{X: 18, Y: 18, Z: 1},
		modifications.ModificationInfo{Type: modifications.ModificationDug, DetectedAt: time.Now()}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	x0, y0, x1, y1, ok := fortFootprintBBox(mods, 20, 20, 1, 8)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if x1 != 19 || y1 != 19 {
		t.Fatalf("high side not clamped: x1=%d y1=%d, want 19,19", x1, y1)
	}
	if x0 != 10 || y0 != 10 {
		t.Fatalf("low side wrong: x0=%d y0=%d, want 10,10", x0, y0)
	}
}

// TestRenderFullOrDownsampled_FullFidelityAtBudget asserts a stitched
// region exactly at the 100x100 budget takes the full-fidelity path (no
// downsampling).
func TestRenderFullOrDownsampled_FullFidelityAtBudget(t *testing.T) {
	rows := make([]string, 100)
	for i := range rows {
		rows[i] = strings.Repeat(".", 100)
	}
	s := &mapview.Slice{Z: 1, X1: 0, Y1: 0, Rows: rows}
	out := renderFullOrDownsampled(s, "test header")
	if !strings.Contains(out, "full fidelity") {
		t.Fatalf("expected full-fidelity path at 100x100, got: %.200q", out)
	}
	if strings.Contains(out, "downsampled") {
		t.Fatalf("should not downsample at exactly the budget: %.200q", out)
	}
}

// TestElevationDownsamplePolicy_OverBudgetSelectsBlockPath asserts a
// stitched region larger than the 100x100 budget on either axis switches to
// the block-downsample path, with the block size disclosed in the header
// (mirrors RenderOverview's own disclosure contract).
func TestElevationDownsamplePolicy_OverBudgetSelectsBlockPath(t *testing.T) {
	rows := make([]string, 150)
	for i := range rows {
		rows[i] = strings.Repeat(".", 150)
	}
	s := &mapview.Slice{Z: 1, X1: 0, Y1: 0, Rows: rows}
	out := renderFullOrDownsampled(s, "test header")
	if !strings.Contains(out, "downsampled") {
		t.Fatalf("expected downsample path over 100x100, got: %.200q", out)
	}
	if !strings.Contains(out, "1 cell =") {
		t.Fatalf("expected block-size disclosure in header, got: %.300q", out)
	}
}
