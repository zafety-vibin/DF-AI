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
