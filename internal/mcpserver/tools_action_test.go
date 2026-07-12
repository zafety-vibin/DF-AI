package mcpserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/protocol"
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
		{"zone", map[string]any{"type": "bedroom", "x1": 10, "y1": 10, "z": 90, "x2": 12, "y2": 12}},
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
