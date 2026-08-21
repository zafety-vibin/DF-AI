package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTextResult(t *testing.T) {
	res := TextResult("hello")
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", res.Content[0])
	}
	if tc.Text != "hello" {
		t.Errorf("expected text %q, got %q", "hello", tc.Text)
	}
}

func TestNewNilBridge(t *testing.T) {
	if srv := New(nil); srv == nil {
		t.Fatal("New(nil) returned nil server")
	}
}

func TestNilBridgeIsSafe(t *testing.T) {
	var b *Bridge
	if b.Connected() {
		t.Error("nil bridge reported Connected() == true")
	}
	if got := b.StatusLine(context.Background()); got == "" {
		t.Error("nil bridge StatusLine returned empty string")
	}
}

// minToolArgs supplies minimal schema-valid arguments per tool for the
// every-tool registration sweep. Tools absent here are called with {}.
// ADDING A TOOL? If it has required inputs, add an entry — the sweep
// fails loudly otherwise, which is the point.
var minToolArgs = map[string]map[string]any{
	"dwarf_detail":   {"id": 1},
	"jobs":           {"x": 1, "y": 1, "z": 1},
	"look":           {"x": 10, "y": 10, "z": 100},
	"cross_section":  {"x": 10, "y": 10},
	"elevation_view": {"axis": "x", "x1": 0, "x2": 2, "y": 1, "z_top": 5, "z_bottom": 0},
	"find_dig_site":  {"width": 3, "height": 3, "z": 90, "near_x": 10, "near_y": 10},
	"designate_dig":  {"type": "default", "x1": 1, "y1": 1, "z1": 1, "x2": 2, "y2": 2, "z2": 1},
	"build":          {"type": "bed", "x": 1, "y": 1, "z": 1},
	"designate_zone": {"type": "bedroom", "x1": 1, "y1": 1, "z": 1, "x2": 2, "y2": 2},
	"assign_zone":    {"x": 1, "y": 1, "z": 1, "unit_id": 1},
	"unassign_zone":  {"x": 1, "y": 1, "z": 1, "unit_id": 1},
	"remove_zone":    {"x": 1, "y": 1, "z": 1},
	// list_zones has no required fields — {} default is fine, no entry needed.
	"create_location":  {"x": 1, "y": 1, "z": 1, "type": "tavern"},
	"assign_lodging":   {"tavern_x": 1, "tavern_y": 1, "tavern_z": 1, "bedroom_x": 2, "bedroom_y": 2, "bedroom_z": 2},
	"unassign_lodging": {"bedroom_x": 2, "bedroom_y": 2, "bedroom_z": 2},
	// list_locations has no required fields — {} default is fine, no entry needed.
	"stockpile":          {"category": "all", "x1": 1, "y1": 1, "z": 1, "x2": 2, "y2": 2},
	// bins/barrels/wheelbarrows are optional pointers — exercise the
	// all-omitted (auto-recompute) path the retrofit call actually uses.
	"set_stockpile_containers": {"x": 1, "y": 1, "z": 1},
	"order":              {"item": "bed", "count": 1},
	"queue_job":          {"x": 1, "y": 1, "z": 1, "item": "bed", "count": 1},
	"set_labor":          {"id": 1, "labor": "mine", "enable": true},
	"unsuspend":          {"x": 1, "y": 1, "z": 1},
	"remove_building":    {"x": 1, "y": 1, "z": 1},
	"pull_lever":         {"x": 1, "y": 1, "z": 1},
	"link_building":      {"lever_x": 1, "lever_y": 1, "lever_z": 1, "target_x": 2, "target_y": 2, "target_z": 2},
	"buildings":          {}, // z is optional — exercise the no-arg path
	"cancel_designation": {"x1": 1, "y1": 1, "z": 1, "x2": 2, "y2": 2},
	"chop":               {"x1": 1, "y1": 1, "z": 1, "x2": 2, "y2": 2},
	"gather":             {"x1": 1, "y1": 1, "z": 1, "x2": 2, "y2": 2},
	"smooth":             {"mode": "smooth", "x1": 1, "y1": 1, "z": 1, "x2": 2, "y2": 2},
	"apply_blueprint":    {"name": "", "origin_x": 0, "origin_y": 0, "origin_z": 0},
	"save_blueprint":     {"name": "nil_bridge_smoke_test", "x1": 1, "y1": 1, "z1": 1, "x2": 2, "y2": 2, "z2": 1},
	// list_blueprints has no required fields — {} default is fine, no entry needed.
	"step":       {"ticks": 1},
	"name_place": {"x": 1, "y": 1, "z": 1, "name": "test"},
	// list_places has no required fields — {} default is fine, no entry needed.
	// list_reactions has no required fields — {} default is fine, no entry needed.
	"build_farm_plot": {"x1": 1, "y1": 1, "z": 1, "x2": 2, "y2": 2},
	"assign_crop":     {"x": 1, "y": 1, "z": 1, "season": "spring", "crop": "fallow"},
	// list_crops has no required fields — {} default is fine, no entry needed.
	"designate_burrow": {"name": "nil_bridge_smoke_test", "x1": 1, "y1": 1, "z1": 1, "x2": 2, "y2": 2},
	"remove_burrow":    {"name": "nil_bridge_smoke_test"},
	"assign_burrow":    {"name": "nil_bridge_smoke_test", "unit_id": 1},
	"unassign_burrow":  {"name": "nil_bridge_smoke_test", "unit_id": 1},
	"set_alert":        {"name": "nil_bridge_smoke_test", "active": true},
	// list_burrows has no required fields — {} default is fine, no entry needed.
	// mandates has no required fields — {} default is fine, no entry needed.
	// create_squad has no required fields — {} default is fine, no entry needed.
	"assign_squad": {"squad_id": 1, "unit_id": 1, "add": true},
	"squad_order":  {"squad_id": 1, "type": "cancel"},
	// list_squads has no required fields — {} default is fine, no entry needed.
}

// TestEveryToolNilBridge lists every registered tool and calls each one
// with minimal valid arguments against a nil bridge: every tool must
// return a readable text response — no panic, no protocol error. This is
// the "registration tests drive every tool" contract in
// docs/guides/mcp-server.md.
func TestEveryToolNilBridge(t *testing.T) {
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

	list, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(list.Tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}
	for _, tool := range list.Tools {
		args := minToolArgs[tool.Name]
		if args == nil {
			args = map[string]any{}
		}
		res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: tool.Name, Arguments: args})
		if err != nil {
			t.Fatalf("call %s: %v (new tool with required args? add it to minToolArgs)", tool.Name, err)
		}
		if len(res.Content) == 0 {
			t.Fatalf("%s: empty content", tool.Name)
		}
		tc, ok := res.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatalf("%s: expected *mcp.TextContent, got %T", tool.Name, res.Content[0])
		}
		if strings.TrimSpace(tc.Text) == "" {
			t.Fatalf("%s: blank text response", tool.Name)
		}
	}
}

// TestStatusToolNotConnected drives the status tool over an in-memory
// MCP session and checks the scaffold's NOT CONNECTED response.
func TestStatusToolNotConnected(t *testing.T) {
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

	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "status"})
	if err != nil {
		t.Fatalf("call status tool: %v", err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", res.Content[0])
	}
	if !strings.Contains(tc.Text, "NOT CONNECTED") {
		t.Errorf("expected NOT CONNECTED message, got %q", tc.Text)
	}
}
