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

	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "designate_dig",
		Arguments: map[string]any{
			"type": "stairs", "x1": 10, "y1": 10, "z1": 100, "x2": 11, "y2": 11, "z2": 94,
		},
	})
	if err != nil {
		t.Fatalf("call designate_dig: %v", err)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", res.Content[0])
	}
	if !strings.Contains(tc.Text, "NOT CONNECTED") {
		t.Errorf("expected NOT CONNECTED message, got %q", tc.Text)
	}
}
