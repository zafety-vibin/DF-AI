package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Schema-size budget for the whole MCP tool surface.
//
// Every tool's name/description/inputSchema is serialized into the
// tools/list response, which loads into EVERY session's context at start —
// unconditionally, whether or not that tool is ever called. Schema bloat
// (verbose per-value prose, long enums) is a direct, permanent tax on every
// conversation this server is used in. See the "TOOL-SCHEMA HOUSE RULE" in
// this project's planning notes: enum parameters should list only a short
// curated common-case vocabulary plus a name-passthrough for full
// generality; per-value facts belong in an on-demand discovery tool, not
// baked into the schema description.
//
// Baseline measured 2026-07-18 (post tool-schema-scaling wave, which added
// Well/MakeChain and a batch of other tools/enum values): tools/list
// serialized to 43657 bytes total across 65 tools. The two largest tools
// were "build" at 2523 bytes (its buildable-type enum, which now includes
// Well) and "look" at 2164 bytes (a deliberately larger scope/lens cost
// table, not enum bloat). Ceilings below are set comfortably above that
// measurement, not at it — a modest number of future tools/enum values
// should fit before this test needs revisiting; a large jump should make
// it fail loudly and specifically, naming the offending tool(s).
//
// If a future wave legitimately grows "build" past its ceiling too (e.g.
// a new buildable-type family), give it a named entry in
// perToolExceptions alongside "look" — do not raise perToolCeilingBytes to
// accommodate it, that would silently relax the budget for every other
// tool as well.

// perToolCeilingBytes is the maximum serialized size (name + description +
// inputSchema + any other Tool fields) for a single tool, in bytes.
// Measured max at write time was "build" at 2523 bytes; this leaves ~550
// bytes (~20%) of headroom above that before CI trips.
const perToolCeilingBytes = 3072 // 3KB

// totalCeilingBytes is the maximum summed serialized size across every
// registered tool, in bytes. Measured total at write time was 43657 bytes
// across 65 tools; this leaves ~17.7KB (~40%) of headroom.
const totalCeilingBytes = 61440 // 60KB

// perToolExceptions lists tools explicitly allowed to exceed
// perToolCeilingBytes, with the reason why. Adding a name here should be
// rare and deliberate — it is a statement that the tool's size is real
// domain complexity (e.g. a cost table), not enum bloat that belongs in a
// skill or a discovery tool instead.
var perToolExceptions = map[string]string{
	"look": "multi-scope perception tool; its schema documents a scope/lens cost table (tokens-per-call tradeoffs), not enumerated domain values — see docs/guides/mcp-server.md",
}

// toolSchemaSize returns the serialized byte size of a single tool
// definition, i.e. exactly what a client's tools/list response spends on
// it.
func toolSchemaSize(t *testing.T, tool *mcp.Tool) int {
	t.Helper()
	b, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal tool %s: %v", tool.Name, err)
	}
	return len(b)
}

// TestToolSchemaBudget lists every registered tool over a real in-memory
// MCP session (the same path a real client uses) and enforces the
// schema-size budget described above.
func TestToolSchemaBudget(t *testing.T) {
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

	sizes := make([]toolSize, 0, len(list.Tools))

	total := 0
	maxTool := toolSize{}
	var overBudget []string
	for _, tool := range list.Tools {
		size := toolSchemaSize(t, tool)
		total += size
		sizes = append(sizes, toolSize{tool.Name, size})
		if size > maxTool.size {
			maxTool = toolSize{tool.Name, size}
		}

		if _, exempt := perToolExceptions[tool.Name]; exempt {
			continue
		}
		if size > perToolCeilingBytes {
			overBudget = append(overBudget, fmt.Sprintf("%s: %d bytes (ceiling %d, over by %d)",
				tool.Name, size, perToolCeilingBytes, size-perToolCeilingBytes))
		}
	}

	if len(overBudget) > 0 {
		sort.Strings(overBudget)
		t.Errorf("%d tool(s) exceed the per-tool schema ceiling of %d bytes (add a named exception in "+
			"perToolExceptions only if the size is real domain complexity, not enum bloat):\n  %s",
			len(overBudget), perToolCeilingBytes, joinLines(overBudget))
	}

	if total > totalCeilingBytes {
		sort.Slice(sizes, func(i, j int) bool { return sizes[i].size > sizes[j].size })
		top := sizes
		if len(top) > 10 {
			top = top[:10]
		}
		lines := make([]string, 0, len(top))
		for _, s := range top {
			lines = append(lines, fmt.Sprintf("%s: %d bytes", s.name, s.size))
		}
		t.Errorf("total serialized tools/list payload is %d bytes, exceeding the %d byte ceiling across %d tools "+
			"(largest contributors):\n  %s",
			total, totalCeilingBytes, len(list.Tools), joinLines(lines))
	}

	t.Logf("tool schema budget: %d tools, %d bytes total (ceiling %d), largest tool %q at %d bytes",
		len(list.Tools), total, totalCeilingBytes, maxTool.name, maxTool.size)
}

// toolSize pairs a tool name with its serialized schema size, for sorting
// and reporting the largest contributors on a budget failure.
type toolSize struct {
	name string
	size int
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n  "
		}
		out += l
	}
	return out
}
