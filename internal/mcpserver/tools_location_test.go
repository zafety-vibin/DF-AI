package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// listRegisteredTools returns every tool the server publishes, over the same
// in-memory session a real client uses (mirrors TestEveryToolNilBridge and
// TestToolSchemaBudget).
func listRegisteredTools(t *testing.T) []*mcp.Tool {
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

	list, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	return list.Tools
}

// TestCreateLocationDeitySchema pins the temple-dedication input's shape: it
// must be OPTIONAL (a temple with no dedication is the normal case, and the
// Go side uses a pointer so an omitted id stays distinguishable from an
// explicit id 0 — historical-figure id 0 is a real deity id in a generated
// world), and its description must name the DISCOVERY PATH rather than
// enumerate per-deity facts, per the tool-schema house rule.
func TestCreateLocationDeitySchema(t *testing.T) {
	var schema string
	for _, tool := range listRegisteredTools(t) {
		if tool.Name != "create_location" {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal schema: %v", err)
		}
		schema = string(raw)
	}
	if schema == "" {
		t.Fatal("create_location is not registered")
	}
	if !strings.Contains(schema, "deity_hf_id") {
		t.Fatalf("create_location schema is missing deity_hf_id:\n%s", schema)
	}
	if !strings.Contains(schema, "fort_story") {
		t.Fatalf("deity_hf_id should name fort_story as the discovery path:\n%s", schema)
	}

	var parsed struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	for _, r := range parsed.Required {
		if r == "deity_hf_id" {
			t.Fatalf("deity_hf_id must stay optional, got required=%v", parsed.Required)
		}
	}
}

// TestCageTrapVocabulary guards the new curated build name end to end: the
// wire byte it maps to, and the fact that it carries its per-value facts in
// the building_types discovery catalog rather than in build's schema.
func TestCageTrapVocabulary(t *testing.T) {
	got, ok := buildTypes["cage_trap"]
	if !ok {
		t.Fatal("buildTypes has no cage_trap entry")
	}
	if got != protocol.BuildTypeCageTrap {
		t.Fatalf("cage_trap maps to 0x%02X, want 0x%02X", got, protocol.BuildTypeCageTrap)
	}
	if protocol.BuildTypeCageTrap != 0xB4 {
		t.Fatalf("BuildTypeCageTrap = 0x%02X, want 0xB4 (must match dfhack-plugin/protocol.h)", protocol.BuildTypeCageTrap)
	}
	if !protocol.IsBuildTypeTrap(protocol.BuildTypeCageTrap) {
		t.Fatal("cage_trap must fall inside the trap range so the plugin dispatches it to placeTrap")
	}

	var entry *buildingTypeEntry
	for i := range buildingTypeCatalog {
		if buildingTypeCatalog[i].Name == "cage_trap" {
			entry = &buildingTypeCatalog[i]
			break
		}
	}
	if entry == nil {
		t.Fatal("building_types catalog has no cage_trap entry")
	}
	// The trap builds unarmed and DF arms it with a separate Load Cage Trap
	// job this project does not expose — the catalog must say so rather than
	// implying a working capture trap.
	if !strings.Contains(strings.ToUpper(entry.Requires), "UNARMED") {
		t.Fatalf("cage_trap catalog entry must state that it builds unarmed, got %q", entry.Requires)
	}
	if !strings.Contains(entry.Requires, "mechanism") {
		t.Fatalf("cage_trap catalog entry should name its mechanism reagent, got %q", entry.Requires)
	}
}

func TestRenderLocations_Basic(t *testing.T) {
	raw := []byte(`{"locations":[
		{"id":5,"type":"INN_TAVERN","x1":10,"y1":10,"x2":11,"y2":11,"z":90,"lodging":[{"civzone_id":42,"x":15,"y":15,"z":90}]},
		{"id":6,"type":"TEMPLE","x1":20,"y1":20,"x2":21,"y2":21,"z":90,"lodging":[]}
	]}`)
	out := renderLocations(raw)
	if !strings.Contains(out, "Tavern") || !strings.Contains(out, "(10,10)") {
		t.Fatalf("expected the tavern's type and extents, got:\n%s", out)
	}
	if !strings.Contains(out, "1 lodging room") {
		t.Fatalf("expected a lodging count for the tavern, got:\n%s", out)
	}
	if !strings.Contains(out, "Temple") {
		t.Fatalf("expected the temple, got:\n%s", out)
	}
}

func TestRenderLocations_NoLocations(t *testing.T) {
	out := renderLocations([]byte(`{"locations":[]}`))
	if out != "No locations." {
		t.Fatalf("expected 'No locations.', got %q", out)
	}
}

func TestRenderLocations_Truncated(t *testing.T) {
	out := renderLocations([]byte(`{"locations":[{"id":1,"type":"LIBRARY","x1":1,"y1":1,"x2":1,"y2":1,"z":1,"lodging":[]}],"truncated":true}`))
	if !strings.Contains(out, "truncated") {
		t.Fatalf("expected a truncation note, got:\n%s", out)
	}
}

// TestRenderLocationsTempleDedication covers the read-back of a temple's deity
// dedication. create_location can write it, and its own ACK truthfully calls
// that write UNVERIFIED and says to confirm in-game — an instruction the
// playing model cannot follow, so list_locations has to be able to show it.
func TestRenderLocationsTempleDedication(t *testing.T) {
	raw := []byte(`{"locations":[
		{"id":6,"type":"TEMPLE","x1":20,"y1":20,"x2":21,"y2":21,"z":90,
		 "deity_type":"WORSHIP_HFID","deity_hf_id":482,"deity_name":"Lorbam Silverfeather","lodging":[]}
	]}`)
	out := renderLocations(raw)
	if !strings.Contains(out, "Lorbam Silverfeather") || !strings.Contains(out, "482") {
		t.Fatalf("expected the dedication's name and hf id, got:\n%s", out)
	}
}

// A temple with the field present but set to NONE is genuinely undedicated;
// a temple from a plugin build predating the field is UNKNOWN. Rendering the
// second as the first would invent a measurement — the same old-plugin-skew
// rule the stockpile container ceilings follow.
func TestRenderLocationsTempleDedicationAbsentVsNone(t *testing.T) {
	none := renderLocations([]byte(`{"locations":[{"id":6,"type":"TEMPLE","x1":1,"y1":1,"x2":2,"y2":2,"z":9,"deity_type":"NONE","lodging":[]}]}`))
	if !strings.Contains(none, "no particular deity") {
		t.Fatalf("deity_type=NONE should read as undedicated, got:\n%s", none)
	}
	old := renderLocations([]byte(`{"locations":[{"id":6,"type":"TEMPLE","x1":1,"y1":1,"x2":2,"y2":2,"z":9,"lodging":[]}]}`))
	if !strings.Contains(old, "not reported by this plugin build") {
		t.Fatalf("an omitted deity_type must say so, got:\n%s", old)
	}
	if strings.Contains(old, "no particular deity") {
		t.Fatalf("an omitted deity_type must not render as a measured NONE, got:\n%s", old)
	}
}

// An id that resolves to no name still reports the id, and the religion branch
// reads the entity id rather than mislabeling it as a historical figure.
func TestRenderLocationsTempleDedicationDegradations(t *testing.T) {
	unnamed := renderLocations([]byte(`{"locations":[{"id":6,"type":"TEMPLE","x1":1,"y1":1,"x2":2,"y2":2,"z":9,"deity_type":"WORSHIP_HFID","deity_hf_id":0,"lodging":[]}]}`))
	if !strings.Contains(unnamed, "historical figure 0") {
		t.Fatalf("hf id 0 is a valid id and must be reported, got:\n%s", unnamed)
	}
	religion := renderLocations([]byte(`{"locations":[{"id":6,"type":"TEMPLE","x1":1,"y1":1,"x2":2,"y2":2,"z":9,"deity_type":"RELIGION_ENID","deity_entity_id":77,"deity_name":"The Cult of Anvils","lodging":[]}]}`))
	if !strings.Contains(religion, "The Cult of Anvils") || !strings.Contains(religion, "77") {
		t.Fatalf("expected the religion's name and entity id, got:\n%s", religion)
	}
	// Non-temples carry no dedication clause at all.
	tavern := renderLocations([]byte(`{"locations":[{"id":5,"type":"INN_TAVERN","x1":1,"y1":1,"x2":2,"y2":2,"z":9,"lodging":[]}]}`))
	if strings.Contains(tavern, "dedicat") {
		t.Fatalf("a tavern must carry no dedication clause, got:\n%s", tavern)
	}
}
