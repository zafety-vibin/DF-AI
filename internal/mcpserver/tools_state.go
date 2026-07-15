package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// maxDwarfList caps the dwarves listing; beyond it the tool summarizes.
const maxDwarfList = 50

// rawJSONCap bounds raw plugin-query payloads (stocks, orders) so one huge
// response cannot blow the MCP output budget (~10k-token cap per response).
const rawJSONCap = 8 * 1024

// capRawJSON truncates an oversized raw payload at a rune boundary and says
// so, instead of silently flooding the response.
func capRawJSON(raw string) string {
	if len(raw) <= rawJSONCap {
		return raw
	}
	cut := rawJSONCap
	for cut > 0 && !utf8.RuneStart(raw[cut]) {
		cut--
	}
	return raw[:cut] + "\n...truncated, refine your query"
}

// buildingListEntry is one entry from the plugin's list_buildings query.
// Shared between the flat `buildings` tool (renderBuildings) and the
// `buildings` look lens (internal/mcpserver/lenses.go) so both parse the
// exact same wire shape once.
type buildingListEntry struct {
	Type     string `json:"type"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Z        int    `json:"z"`
	X1       int    `json:"x1"`
	Y1       int    `json:"y1"`
	X2       int    `json:"x2"`
	Y2       int    `json:"y2"`
	Stage    int    `json:"stage"`
	MaxStage int    `json:"max_stage"`
	Done     bool   `json:"done"`
}

// renderBuildings renders the list_buildings query response, one line per
// building. Unfinished buildings are the interesting case: a planned
// building is invisible in every map view, so a plan that silently died
// (missing materials, unreachable site) only shows up here — construction
// stage is rendered loudly.
func renderBuildings(raw []byte) string {
	var resp struct {
		Buildings []buildingListEntry `json:"buildings"`
		Truncated bool                `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_buildings response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Buildings) == 0 {
		return "No buildings."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d buildings:\n", len(resp.Buildings))
	for _, bl := range resp.Buildings {
		if bl.Done {
			fmt.Fprintf(&sb, "- %s at (%d,%d,%d) — built\n", bl.Type, bl.X, bl.Y, bl.Z)
		} else {
			fmt.Fprintf(&sb, "- %s at (%d,%d,%d) — UNDER CONSTRUCTION (stage %d/%d)\n",
				bl.Type, bl.X, bl.Y, bl.Z, bl.Stage, bl.MaxStage)
		}
	}
	if resp.Truncated {
		sb.WriteString("... list truncated at the plugin's cap — pass z to narrow it\n")
	}
	return sb.String()
}

// stockItem is one (item_type, material) entry from the plugin's
// stockpile_inventory query.
type stockItem struct {
	ItemType string `json:"item_type"`
	Material string `json:"material"`
	Count    int    `json:"count"`
	Economic bool   `json:"economic,omitempty"`
}

// renderStocks renders the stockpile_inventory response. The plugin keys
// every item by exact (item_type, material) with no cap, which already
// exceeds the MCP payload budget at a 7-dwarf fort (see
// docs/archive/2026-07-12-token-scaling-research.md) — the raw pass-through
// this replaced silently truncated mid-JSON. Default (detailed=false):
// aggregate to one line per item_type with top-3 materials by count.
// detailed=true (set when the caller passed a category filter, which the
// plugin already narrows before this ever sees it): one line per entry.
func renderStocks(raw []byte, detailed bool, minCount int) string {
	var resp struct {
		Items []stockItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable stockpile_inventory response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	items := resp.Items
	if minCount > 0 {
		filtered := items[:0]
		for _, it := range items {
			if it.Count >= minCount {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	if len(items) == 0 {
		return "No stock items (or all below min_count)."
	}

	if detailed {
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d item/material entries:\n", len(items))
		for _, it := range items {
			econ := ""
			if it.Economic {
				econ = " [economic]"
			}
			fmt.Fprintf(&sb, "- %s: %s x%d%s\n", it.ItemType, it.Material, it.Count, econ)
		}
		return sb.String()
	}

	type typeAgg struct {
		total     int
		economic  int
		materials []stockItem
	}
	order := make([]string, 0)
	byType := make(map[string]*typeAgg)
	for _, it := range items {
		agg, ok := byType[it.ItemType]
		if !ok {
			agg = &typeAgg{}
			byType[it.ItemType] = agg
			order = append(order, it.ItemType)
		}
		agg.total += it.Count
		if it.Economic {
			agg.economic += it.Count
		}
		agg.materials = append(agg.materials, it)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d item types, %d item/material entries total (pass category to see full per-material detail for one type):\n",
		len(order), len(items))
	for _, t := range order {
		agg := byType[t]
		sort.Slice(agg.materials, func(i, j int) bool { return agg.materials[i].Count > agg.materials[j].Count })
		topN := agg.materials
		if len(topN) > 3 {
			topN = topN[:3]
		}
		tops := make([]string, 0, len(topN))
		for _, m := range topN {
			tops = append(tops, fmt.Sprintf("%s %d", m.Material, m.Count))
		}
		econNote := ""
		if agg.economic > 0 {
			econNote = fmt.Sprintf(" (%d economic)", agg.economic)
		}
		fmt.Fprintf(&sb, "- %s: %d total across %d material%s%s; top: %s\n",
			t, agg.total, len(agg.materials), plural(len(agg.materials)), econNote, strings.Join(tops, ", "))
	}
	return sb.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// managerOrder is one entry from the plugin's manager_orders query.
type managerOrder struct {
	ID          int    `json:"id"`
	JobType     string `json:"job_type"`
	AmountTotal int    `json:"amount_total"`
	AmountLeft  int    `json:"amount_left"`
	Validated   bool   `json:"validated"`
	Active      bool   `json:"active"`
}

// renderManagerOrders renders the manager_orders response compactly. Does
// NOT fetch list_orders (the DF job-type enum catalog, ~240 static entries,
// ~2000 tokens of unchanging noise every call) — the common orderable
// vocabulary is already in the order/queue_job tool schemas, and anything
// outside it is discoverable on demand via the separate job_types tool
// (see registerStateTools below) rather than paid on every orders call.
func renderManagerOrders(raw []byte) string {
	var resp struct {
		Orders []managerOrder `json:"orders"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable manager_orders response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Orders) == 0 {
		return "No manager orders queued."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d manager orders:\n", len(resp.Orders))
	for _, o := range resp.Orders {
		status := "queued, not yet dispatched"
		if o.Active {
			status = "ACTIVE"
		} else if !o.Validated {
			status = "invalid"
		}
		fmt.Fprintf(&sb, "- id=%d %s x%d (%d left) — %s\n", o.ID, o.JobType, o.AmountTotal, o.AmountLeft, status)
	}
	return sb.String()
}

// jobTypeEntry is one entry from the plugin's list_orders query — a
// DFHack df::job_type enum key name plus a heuristic category tag.
type jobTypeEntry struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

// maxJobTypesList caps the job_types tool's output. Unfiltered, list_orders
// returns DF's full job_type enum (~240 entries) — this is exactly the
// static noise the orders/order/queue_job tools deliberately stopped
// paying on every call (see renderManagerOrders above); job_types exists
// so the model pays for that dump only when it actually needs to look up
// an unfamiliar name, and the cap keeps a forgotten filter from blowing
// the response budget anyway.
const maxJobTypesList = 80

// renderJobTypes renders the list_orders response for the job_types
// discovery tool. Every name shown here round-trips into queue_job's
// name-based path (protocol.OrderTypeByName) verbatim — this is the
// catalog half of generalized item construction, work_orders.cpp
// resolveJobTypeByName is the lookup half.
func renderJobTypes(raw []byte, filtered bool) string {
	var resp struct {
		Orders []jobTypeEntry `json:"orders"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_orders response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Orders) == 0 {
		return "No job types matched that filter."
	}
	entries := resp.Orders
	truncated := false
	if len(entries) > maxJobTypesList {
		entries = entries[:maxJobTypesList]
		truncated = true
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d job types", len(resp.Orders))
	if truncated {
		fmt.Fprintf(&sb, " (showing first %d — pass filter to narrow)", maxJobTypesList)
	}
	sb.WriteString(":\n")
	for _, o := range entries {
		fmt.Fprintf(&sb, "- %s [%s]\n", o.Name, o.Category)
	}
	if !filtered && !truncated {
		sb.WriteString("(pass filter next time to narrow this list)\n")
	}
	return sb.String()
}

// reactionReagentEntry is one reagent summary from the plugin's
// list_reactions query. ItemType is omitted when the reagent isn't an
// item-typed reagent (queue_job's reaction path currently rejects those —
// see work_orders.cpp applyQueueReactionJob).
type reactionReagentEntry struct {
	Code     string `json:"code"`
	Quantity int    `json:"quantity"`
	ItemType string `json:"item_type,omitempty"`
}

// reactionEntry is one entry from the plugin's list_reactions query — a
// df::reaction's code, display name, the workshop(s) it can run at, and a
// summary of its reagents.
type reactionEntry struct {
	Code      string                 `json:"code"`
	Name      string                 `json:"name"`
	Buildings []string               `json:"buildings"`
	Reagents  []reactionReagentEntry `json:"reagents"`
}

// maxReactionsList caps the list_reactions tool's output, same rationale
// as maxJobTypesList below — this is the pay-on-demand discovery tool for
// queue_job's reaction path, not something called on every turn.
const maxReactionsList = 60

// renderReactions renders the list_reactions response for the discovery
// tool. Every code shown here round-trips into queue_job's reaction param
// (protocol.OrderTypeCustomReaction) verbatim — this is the catalog half
// of reaction-based job queueing, work_orders.cpp
// applyQueueReactionJob is the lookup/build half.
func renderReactions(raw []byte, filtered bool) string {
	var resp struct {
		Reactions []reactionEntry `json:"reactions"`
		Truncated bool            `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_reactions response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Reactions) == 0 {
		return "No reactions matched that filter."
	}
	entries := resp.Reactions
	truncated := resp.Truncated
	if len(entries) > maxReactionsList {
		entries = entries[:maxReactionsList]
		truncated = true
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d reactions", len(resp.Reactions))
	if truncated {
		fmt.Fprintf(&sb, " (showing first %d — pass filter to narrow)", len(entries))
	}
	sb.WriteString(":\n")
	for _, r := range entries {
		fmt.Fprintf(&sb, "- %s (%s) @ %s", r.Code, r.Name, strings.Join(r.Buildings, "/"))
		if len(r.Reagents) > 0 {
			var parts []string
			for _, rg := range r.Reagents {
				part := rg.Code
				if rg.ItemType != "" {
					part += "=" + rg.ItemType
				}
				if rg.Quantity != 1 {
					part += fmt.Sprintf("x%d", rg.Quantity)
				}
				parts = append(parts, part)
			}
			fmt.Fprintf(&sb, " [needs: %s]", strings.Join(parts, ", "))
		}
		sb.WriteString("\n")
	}
	if !filtered && !truncated {
		sb.WriteString("(pass filter next time to narrow this list)\n")
	}
	return sb.String()
}

// renderDwarfList renders the id/position roster, capped at maxDwarfList.
func renderDwarfList(dwarves []protocol.EntityInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d dwarves:\n", len(dwarves))
	for i, d := range dwarves {
		if i >= maxDwarfList {
			fmt.Fprintf(&sb, "... and %d more (use dwarf_detail by id)\n", len(dwarves)-maxDwarfList)
			break
		}
		fmt.Fprintf(&sb, "- id=%d @(%d,%d,%d)\n", d.ID, d.X, d.Y, d.Z)
	}
	return sb.String()
}

func registerStateTools(srv *mcp.Server, b *Bridge) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "status",
		Description: "Connection and simulation status: is the DFHack plugin connected, is DF paused, current tick. Call this first if anything seems wrong.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		if b == nil || !b.Connected() {
			return TextResult("NOT CONNECTED: start DF, then run `ai-connect` in the DFHack console. The MCP server listens on the port in config/orchestrator.yaml."), nil, nil
		}
		return withDash(b, ctx, "Connected. The ground-truth header above is live game state."), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "alerts",
		Description: "DF's own announcement stream (job cancellations with reasons, sieges, moods, migrants). READ THIS when work isn't progressing — DF usually says exactly why. Once you've read and acted on one, call dismiss_alerts so it stops repeating in every future call — nothing here auto-clears.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		snap := b.Snapshot()
		if len(snap.ActiveAlerts) == 0 {
			return withDash(b, ctx, "No active alerts."), nil, nil
		}
		var sb strings.Builder
		for _, a := range snap.ActiveAlerts {
			fmt.Fprintf(&sb, "- [%d] sev=%d %s", a.ID, a.Severity, a.Text)
			if a.HasPosition() {
				fmt.Fprintf(&sb, " @(%d,%d,%d)", a.X, a.Y, a.Z)
			}
			sb.WriteString("\n")
		}
		if snap.ActiveAlertCount > len(snap.ActiveAlerts) {
			fmt.Fprintf(&sb, "... and %d more (dismiss some to see them)\n", snap.ActiveAlertCount-len(snap.ActiveAlerts))
		}
		return withDash(b, ctx, sb.String()), nil, nil
	})

	type dismissAlertsIn struct {
		IDs []int `json:"ids,omitempty" jsonschema:"specific alert ids to dismiss (the [N] prefix from the alerts tool)"`
		All bool  `json:"all,omitempty" jsonschema:"dismiss every currently active alert instead of listing ids"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "dismiss_alerts",
		Description: "Mark alerts as seen/handled so they stop repeating in alerts and the dashboard's alert count. Dismissed alerts are NOT deleted from DF's own history — this only shrinks the model's 'still need to look at this' set. Call after reading and acting on an alert.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in dismissAlertsIn) (*mcp.CallToolResult, any, error) {
		if b == nil || b.WM == nil || b.WM.Observed.Alerts == nil {
			return withDash(b, ctx, "no alert store available"), nil, nil
		}
		store := b.WM.Observed.Alerts
		if in.All {
			n := store.DismissAll()
			return withDash(b, ctx, fmt.Sprintf("dismissed all %d active alerts", n)), nil, nil
		}
		if len(in.IDs) == 0 {
			return withDash(b, ctx, "no ids given and all=false — nothing to dismiss"), nil, nil
		}
		dismissed := 0
		var notFound []int
		for _, id := range in.IDs {
			if store.Dismiss(uint32(id)) {
				dismissed++
			} else {
				notFound = append(notFound, id)
			}
		}
		msg := fmt.Sprintf("dismissed %d/%d alerts", dismissed, len(in.IDs))
		if len(notFound) > 0 {
			msg += fmt.Sprintf(" (not found or already dismissed: %v)", notFound)
		}
		return withDash(b, ctx, msg), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "dwarves",
		Description: "List all dwarves with id and position. Use dwarf_detail for skills/mood/job of one dwarf.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		return withDash(b, ctx, renderDwarfList(b.Snapshot().Entities.Dwarves)), nil, nil
	})

	type dwarfDetailIn struct {
		ID int `json:"id" jsonschema:"the dwarf's id from the dwarves tool"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "dwarf_detail",
		Description: "One dwarf's full record: skills, labors, mood, current job.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in dwarfDetailIn) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "dwarf_detail", fmt.Sprintf(`{"id":%d}`, in.ID))
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, string(raw)), nil, nil
	})

	type stocksIn struct {
		Category string `json:"category,omitempty" jsonschema:"optional substring filter on item type (e.g. 'boulder', 'wood') — narrows the query AND switches the response to full per-material detail for that type"`
		MinCount int    `json:"min_count,omitempty" jsonschema:"optional: hide item/material entries below this count"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "stocks",
		Description: "Stockpile inventory by item type and material — what the fort actually has. Default view is aggregated (one line per item type, top materials); pass category to narrow the query and see full per-material detail for one type.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in stocksIn) (*mcp.CallToolResult, any, error) {
		args := "{}"
		if in.Category != "" {
			argBytes, _ := json.Marshal(map[string]string{"category": in.Category})
			args = string(argBytes)
		}
		raw, err := b.Query(ctx, "stockpile_inventory", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderStocks(raw, in.Category != "", in.MinCount)), nil, nil
	})

	type buildingsIn struct {
		Z *int `json:"z,omitempty" jsonschema:"optional: only list buildings on this z-level"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "buildings",
		Description: "List every placed building with position and construction progress. Planned buildings are INVISIBLE in map views until built — this is the only way to see whether a build order is actually being worked or died silently. Optional z filters to one level.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in buildingsIn) (*mcp.CallToolResult, any, error) {
		args := "{}"
		if in.Z != nil {
			args = fmt.Sprintf(`{"z":%d}`, *in.Z)
		}
		raw, err := b.Query(ctx, "list_buildings", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderBuildings(raw)), nil, nil
	})

	// queries.cpp handleWorkshopJobs rejects x<0 || y<0, so the workshop
	// tile is required (no plugin-side default).
	type jobsIn struct {
		X int `json:"x" jsonschema:"workshop tile x"`
		Y int `json:"y" jsonschema:"workshop tile y"`
		Z int `json:"z" jsonschema:"workshop tile z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "jobs",
		Description: "Jobs queued at one workshop, targeted by its tile (find workshops via building queries).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in jobsIn) (*mcp.CallToolResult, any, error) {
		args := fmt.Sprintf(`{"x":%d,"y":%d,"z":%d}`, in.X, in.Y, in.Z)
		raw, err := b.Query(ctx, "workshop_jobs", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, string(raw)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "orders",
		Description: "Manager work orders — what's queued and its status. The orderable item vocabulary lives in the order/queue_job tool schemas, not here.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "manager_orders", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderManagerOrders(raw)), nil, nil
	})

	type jobTypesIn struct {
		Filter string `json:"filter,omitempty" jsonschema:"optional case-insensitive substring filter on the job type name (e.g. 'hatch', 'construct') — narrows the ~240-entry DF job_type enum instead of dumping all of it"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "job_types",
		Description: "Look up DFHack job_type enum names for queue_job's name-based path — names not in queue_job's short vocabulary (bed/table/.../blocks) can still be queued by passing the exact name found here as queue_job's item, though queue_job still rejects a few workshop-compatible job types outright (meal, crafts) pending a verified material filter — see queue_job's item description. Call this ONLY when you need to discover a name you don't already know; it is a separate tool from order/queue_job/orders precisely so it isn't paid on every call. Pass filter to narrow.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in jobTypesIn) (*mcp.CallToolResult, any, error) {
		args := "{}"
		if in.Filter != "" {
			argBytes, _ := json.Marshal(map[string]string{"filter": in.Filter})
			args = string(argBytes)
		}
		raw, err := b.Query(ctx, "list_orders", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderJobTypes(raw, in.Filter != "")), nil, nil
	})

	type listReactionsIn struct {
		Filter string `json:"filter,omitempty" jsonschema:"optional case-insensitive substring filter against the reaction code or display name (e.g. 'brew', 'plant') — narrows the fortress-mode reaction catalog instead of dumping all of it"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_reactions",
		Description: "Look up raw-defined df::reaction codes for queue_job's reaction param — this is the ONLY way to queue reactions like brewing that have no job_type mapping at all (queue_job's item vocabulary can't reach them). Pass the exact code found here (e.g. BREW_DRINK_FROM_PLANT) as queue_job's reaction argument. Call this ONLY when you need to discover a code you don't already know; it is a separate tool from queue_job precisely so it isn't paid on every call. Pass filter to narrow.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listReactionsIn) (*mcp.CallToolResult, any, error) {
		args := "{}"
		if in.Filter != "" {
			argBytes, _ := json.Marshal(map[string]string{"filter": in.Filter})
			args = string(argBytes)
		}
		raw, err := b.Query(ctx, "list_reactions", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderReactions(raw, in.Filter != "")), nil, nil
	})
}
