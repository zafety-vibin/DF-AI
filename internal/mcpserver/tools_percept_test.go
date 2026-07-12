package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/mapview"
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
