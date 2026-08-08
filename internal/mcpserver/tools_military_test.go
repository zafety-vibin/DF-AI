package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// militaryToolByName lists tools over a real in-memory MCP session (the
// same path a real client uses) and returns the named one, mirroring
// TestLookDescriptionDocumentsPendingBuildingMarker's setup.
func militaryToolByName(t *testing.T, name string) *mcp.Tool {
	t.Helper()
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
	for _, tool := range res.Tools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("%s tool not found", name)
	return nil
}

// TestCreateSquadDescriptionDropsMilitiaCaptainDefault pins the model-facing
// half of the 2026-08-07 squad fix. The plugin no longer rewrites an empty
// position_code to "MILITIA_CAPTAIN" -- it auto-selects a squad-leader
// position that actually has an unlocked assignment slot, and refuses when
// none does. A schema still promising a MILITIA_CAPTAIN default would be
// promising a default the plugin may now refuse to honour, which is exactly
// the kind of untruthful surface this project forbids.
func TestCreateSquadDescriptionDropsMilitiaCaptainDefault(t *testing.T) {
	tool := militaryToolByName(t, "create_squad")
	if strings.Contains(tool.Description, "MILITIA_CAPTAIN") {
		t.Fatalf("create_squad description must not promise a MILITIA_CAPTAIN default: %q", tool.Description)
	}
	if !strings.Contains(tool.Description, "auto-select") {
		t.Fatalf("create_squad description must document the auto-select behavior for an omitted position_code: %q", tool.Description)
	}
	if !strings.Contains(tool.Description, "refuse") {
		t.Fatalf("create_squad description must say auto-selection can refuse rather than silently minting: %q", tool.Description)
	}
	schema, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal create_squad input schema: %v", err)
	}
	if strings.Contains(string(schema), "MILITIA_CAPTAIN") {
		t.Fatalf("create_squad position_code schema must not promise a MILITIA_CAPTAIN default: %s", schema)
	}
}

// TestAssignSquadDescriptionKeepsCommanderLimit guards the one claim the
// squad fix did NOT make true: the commander slot (position 0) is still
// unreachable, and it is still UNVERIFIED whether DF binds it on its own.
// assign_squad's description must keep saying so.
func TestAssignSquadDescriptionKeepsCommanderLimit(t *testing.T) {
	tool := militaryToolByName(t, "assign_squad")
	if !strings.Contains(tool.Description, "commander slot (position 0) cannot be filled") {
		t.Fatalf("assign_squad description must keep documenting the unfillable commander slot: %q", tool.Description)
	}
}

func TestRenderSquads_NoSquads(t *testing.T) {
	out := renderSquads([]byte(`{"squads":[]}`))
	if out != "No squads." {
		t.Fatalf("expected 'No squads.', got %q", out)
	}
}

func TestRenderSquads_BadJSON(t *testing.T) {
	out := renderSquads([]byte(`not json`))
	if !strings.Contains(out, "unparseable list_squads response") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

func TestRenderSquads_MembersAndVacancy(t *testing.T) {
	raw := []byte(`{"squads":[
		{"id":3,"name":"The Ax Wardens","entity_id":1,
		 "members":[
		   {"position_index":0,"is_commander":true,"vacant":true,"unit_id":-1},
		   {"position_index":1,"is_commander":false,"vacant":false,"unit_id":42}
		 ],
		 "orders":[]}
	]}`)
	out := renderSquads(raw)
	if !strings.Contains(out, `squad #3 "The Ax Wardens"`) {
		t.Fatalf("expected squad header, got:\n%s", out)
	}
	if !strings.Contains(out, "1/2 slots filled") {
		t.Fatalf("expected filled/total count, got:\n%s", out)
	}
	if !strings.Contains(out, "commander: vacant") {
		t.Fatalf("expected vacant commander line, got:\n%s", out)
	}
	if !strings.Contains(out, "soldier: unit#42") {
		t.Fatalf("expected filled soldier line, got:\n%s", out)
	}
	if !strings.Contains(out, "orders: none") {
		t.Fatalf("expected 'orders: none' for an empty orders list, got:\n%s", out)
	}
}

func TestRenderSquads_UnnamedSquad(t *testing.T) {
	raw := []byte(`{"squads":[{"id":1,"name":"","entity_id":1,"members":[],"orders":[]}]}`)
	out := renderSquads(raw)
	if !strings.Contains(out, "(unnamed)") {
		t.Fatalf("expected a placeholder name for an empty squad name, got:\n%s", out)
	}
}

func TestRenderSquads_StationOrder(t *testing.T) {
	raw := []byte(`{"squads":[{"id":2,"name":"Guard","entity_id":1,"members":[],
		"orders":[{"type":"station","x":10,"y":20,"z":90,"burrows":[]}]}]}`)
	out := renderSquads(raw)
	if !strings.Contains(out, "station at (10,20,90)") {
		t.Fatalf("expected station order line, got:\n%s", out)
	}
}

func TestRenderSquads_DefendBurrowOrder(t *testing.T) {
	raw := []byte(`{"squads":[{"id":2,"name":"Guard","entity_id":1,"members":[],
		"orders":[{"type":"defend_burrow","x":0,"y":0,"z":0,"burrows":[{"id":5,"name":"chokepoint"}]}]}]}`)
	out := renderSquads(raw)
	if !strings.Contains(out, `defend burrow(s) "chokepoint"`) {
		t.Fatalf("expected defend_burrow order line, got:\n%s", out)
	}
}

func TestRenderSquads_OtherOrderType(t *testing.T) {
	// An order type this tool doesn't decode (e.g. a kill-list order set
	// by some other means) must be reported generically, never dropped
	// silently or misrendered as one of the two known types.
	raw := []byte(`{"squads":[{"id":2,"name":"Guard","entity_id":1,"members":[],
		"orders":[{"type":"other","x":0,"y":0,"z":0,"burrows":[]}]}]}`)
	out := renderSquads(raw)
	if !strings.Contains(out, "other order type") {
		t.Fatalf("expected a generic fallback line for an undecoded order type, got:\n%s", out)
	}
}
