package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Tier 2 handlers forward to the plugin via Context.Plugin. Args are
// JSON-encoded for transport; the plugin parses them with whatever JSON
// library it has. Results come back as raw JSON which we surface to the
// LLM verbatim (the renderer will marshal them again, but JSON-in-JSON
// stays valid).

// pluginQuery is the shared body for Tier 2 handlers.
func pluginQuery(ctx Context, name string, argsObj any) (Result, error) {
	if ctx.Plugin == nil {
		return Result{
			Error: "plugin dispatcher not configured (run is offline or plugin handler not yet implemented)",
		}, fmt.Errorf("no plugin dispatcher")
	}
	argsJSON := "{}"
	if argsObj != nil {
		b, err := json.Marshal(argsObj)
		if err != nil {
			return Result{Error: fmt.Sprintf("marshal args: %v", err)}, err
		}
		argsJSON = string(b)
	}
	c := ctx.Ctx
	if c == nil {
		c = context.Background()
	}
	cctx, cancel := context.WithTimeout(c, 5*time.Second)
	defer cancel()
	data, err := ctx.Plugin.Dispatch(cctx, name, argsJSON)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	// Parse plugin's JSON response into a generic structure so the
	// renderer can re-marshal it cleanly.
	var parsed any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &parsed); err != nil {
			// Fall back to raw string if response isn't valid JSON.
			return Result{
				Note: fmt.Sprintf("plugin returned %d bytes (non-JSON)", len(data)),
				Data: string(data),
			}, nil
		}
	}
	return Result{Data: parsed}, nil
}

// ---------------------------------------------------------------------------
// list_orders
// ---------------------------------------------------------------------------

type ListOrdersHandler struct{}

func (h ListOrdersHandler) Name() string { return "list_orders" }
func (h ListOrdersHandler) Description() string {
	return "list manager-orderable job types from DF, filtered by category (furniture/weapons/food/raw if given)"
}
func (h ListOrdersHandler) ArgsSpec() []ArgSpec {
	return []ArgSpec{{Name: "filter", Type: "string", Optional: true}}
}
func (h ListOrdersHandler) Execute(ctx Context, args []string) (Result, error) {
	filter := ""
	if len(args) > 0 {
		filter = args[0]
	}
	r, _ := pluginQuery(ctx, "list_orders", map[string]any{"filter": filter})
	if r.Error == "" {
		r.Note = "manager-orderable job types"
	}
	return r, nil
}

// ---------------------------------------------------------------------------
// manager_orders
// ---------------------------------------------------------------------------

type ManagerOrdersHandler struct{}

func (h ManagerOrdersHandler) Name() string { return "manager_orders" }
func (h ManagerOrdersHandler) Description() string {
	return "list current manager work orders with status, target item, quantity, remaining"
}
func (h ManagerOrdersHandler) ArgsSpec() []ArgSpec { return nil }
func (h ManagerOrdersHandler) Execute(ctx Context, args []string) (Result, error) {
	r, _ := pluginQuery(ctx, "manager_orders", nil)
	if r.Error == "" {
		r.Note = "current manager queue"
	}
	return r, nil
}

// ---------------------------------------------------------------------------
// dwarf_detail
// ---------------------------------------------------------------------------

type DwarfDetailHandler struct{}

func (h DwarfDetailHandler) Name() string { return "dwarf_detail" }
func (h DwarfDetailHandler) Description() string {
	return "full record for one dwarf: skills, top labors, mood, current job, position, attributes"
}
func (h DwarfDetailHandler) ArgsSpec() []ArgSpec {
	return []ArgSpec{{Name: "id", Type: "int"}}
}
func (h DwarfDetailHandler) Execute(ctx Context, args []string) (Result, error) {
	if len(args) < 1 {
		return Result{Error: "dwarf_detail needs unit ID"}, fmt.Errorf("missing arg")
	}
	id, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		return Result{Error: fmt.Sprintf("dwarf_detail: bad id %q: %v", args[0], err)}, err
	}
	r, _ := pluginQuery(ctx, "dwarf_detail", map[string]any{"id": id})
	if r.Error == "" {
		r.Note = fmt.Sprintf("dwarf %d", id)
	}
	return r, nil
}

// ---------------------------------------------------------------------------
// building_status
// ---------------------------------------------------------------------------

type BuildingStatusHandler struct{}

func (h BuildingStatusHandler) Name() string { return "building_status" }
func (h BuildingStatusHandler) Description() string {
	return "what's at this tile: building type, subtype, in-progress/suspended/complete state, owner"
}
func (h BuildingStatusHandler) ArgsSpec() []ArgSpec {
	return []ArgSpec{
		{Name: "x", Type: "int"},
		{Name: "y", Type: "int"},
		{Name: "z", Type: "int"},
	}
}
func (h BuildingStatusHandler) Execute(ctx Context, args []string) (Result, error) {
	if len(args) < 3 {
		return Result{Error: "building_status needs x, y, z"}, fmt.Errorf("missing args")
	}
	coords := make([]int64, 3)
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseInt(args[i], 10, 16)
		if err != nil {
			return Result{Error: fmt.Sprintf("arg %d not an int16: %v", i, err)}, err
		}
		coords[i] = v
	}
	r, _ := pluginQuery(ctx, "building_status", map[string]any{
		"x": coords[0], "y": coords[1], "z": coords[2],
	})
	if r.Error == "" {
		r.Note = fmt.Sprintf("building at (%d,%d,%d)", coords[0], coords[1], coords[2])
	}
	return r, nil
}

// ---------------------------------------------------------------------------
// workshop_jobs
// ---------------------------------------------------------------------------

type WorkshopJobsHandler struct{}

func (h WorkshopJobsHandler) Name() string { return "workshop_jobs" }
func (h WorkshopJobsHandler) Description() string {
	return "list current jobs at a workshop (by tile coordinate). includes suspended jobs."
}
func (h WorkshopJobsHandler) ArgsSpec() []ArgSpec {
	return []ArgSpec{
		{Name: "x", Type: "int"},
		{Name: "y", Type: "int"},
		{Name: "z", Type: "int"},
	}
}
func (h WorkshopJobsHandler) Execute(ctx Context, args []string) (Result, error) {
	if len(args) < 3 {
		return Result{Error: "workshop_jobs needs x, y, z"}, fmt.Errorf("missing args")
	}
	coords := make([]int64, 3)
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseInt(args[i], 10, 16)
		if err != nil {
			return Result{Error: fmt.Sprintf("arg %d not an int16: %v", i, err)}, err
		}
		coords[i] = v
	}
	r, _ := pluginQuery(ctx, "workshop_jobs", map[string]any{
		"x": coords[0], "y": coords[1], "z": coords[2],
	})
	if r.Error == "" {
		r.Note = fmt.Sprintf("jobs at (%d,%d,%d)", coords[0], coords[1], coords[2])
	}
	return r, nil
}

// ---------------------------------------------------------------------------
// stockpile_inventory
// ---------------------------------------------------------------------------

type StockpileInventoryHandler struct{}

func (h StockpileInventoryHandler) Name() string { return "stockpile_inventory" }
func (h StockpileInventoryHandler) Description() string {
	return "aggregate item counts by type (food, drink, wood, stone, weapons, armor, ...). pass a category to filter."
}
func (h StockpileInventoryHandler) ArgsSpec() []ArgSpec {
	return []ArgSpec{{Name: "category", Type: "string", Optional: true}}
}
func (h StockpileInventoryHandler) Execute(ctx Context, args []string) (Result, error) {
	cat := ""
	if len(args) > 0 {
		cat = args[0]
	}
	r, _ := pluginQuery(ctx, "stockpile_inventory", map[string]any{"category": cat})
	if r.Error == "" {
		r.Note = "stockpile aggregates"
	}
	return r, nil
}

// Compile-time interface checks.
var (
	_ Handler = ListOrdersHandler{}
	_ Handler = ManagerOrdersHandler{}
	_ Handler = DwarfDetailHandler{}
	_ Handler = BuildingStatusHandler{}
	_ Handler = WorkshopJobsHandler{}
	_ Handler = StockpileInventoryHandler{}
)
