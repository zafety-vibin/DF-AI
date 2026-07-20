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
// COST MODEL (revised 2026-07-19 per scaling report #2): harnesses with
// client-side tool deferral (Claude Code's tool search, on by default)
// load only TOOL NAMES at session start; a tool's description+schema is
// paid per-load, only when a session actually fetches that tool. Three
// budgets follow:
//   1. per-tool bytes — the real recurring cost, paid on every load of
//      that tool in every session. This is the ceiling that bites.
//   2. total bytes — paid in full only by NON-deferring harnesses (raw
//      SDK integrations without defer_loading, proxied deployments,
//      ENABLE_TOOL_SEARCH=false). Kept as a generous backstop so such a
//      harness degrades to "large but sane," not unbounded.
//   3. names bytes — the true always-loaded tax under deferral (~30
//      bytes/tool in the listing). Small today; this metric is the one
//      that grows monotonically with tool COUNT.
// Schema bloat (verbose per-value prose, long enums) remains wrong at any
// budget: under deferral, descriptions are ToolSearch's discovery
// metadata, and the "TOOL-SCHEMA HOUSE RULE" still applies — enum
// parameters list a short curated vocabulary plus a name-passthrough;
// per-value facts belong in an on-demand discovery tool, not baked into
// the schema description.
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
// registered tool, in bytes — the non-deferring-harness backstop (see the
// cost model above). Measured 57,443 bytes across 81 tools on 2026-07-19;
// raised from 60KB to 80KB per scaling report #2 rather than fought,
// because deferring harnesses never pay this sum.
const totalCeilingBytes = 81920 // 80KB

// namesCeilingBytes caps the summed tool-NAME bytes — the always-loaded
// tax under client-side deferral. Measured 932 bytes across 81 tools on
// 2026-07-19 (~2.4KB with the mcp__server__ prefixes a client adds).
// Hitting this ceiling (~140 tools at current naming) is the signal to
// consider CRUD-mirror consolidation for tool-count reasons.
const namesCeilingBytes = 1600

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
	namesTotal := 0
	maxTool := toolSize{}
	var overBudget []string
	for _, tool := range list.Tools {
		size := toolSchemaSize(t, tool)
		total += size
		namesTotal += len(tool.Name)
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

	if namesTotal > namesCeilingBytes {
		t.Errorf("summed tool-name bytes = %d, exceeding the %d byte always-loaded ceiling across %d tools — "+
			"tool COUNT is now the scaling concern; consider CRUD-mirror consolidation (see scaling report #2)",
			namesTotal, namesCeilingBytes, len(list.Tools))
	}

	t.Logf("tool schema budget: %d tools, %d bytes total (ceiling %d), names %d bytes (ceiling %d), largest tool %q at %d bytes",
		len(list.Tools), total, totalCeilingBytes, namesTotal, namesCeilingBytes, maxTool.name, maxTool.size)
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
