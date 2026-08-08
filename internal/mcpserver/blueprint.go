package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/df-ai/orchestrator/internal/blueprints"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/topology"
)

// reloadBlueprints re-scans blueprints/*.csv into lib before every lookup.
// LoadAll runs once at NewBridge time, which means a CSV authored (or
// edited) mid-session was invisible until a server restart — this call
// fixes that. It only ADDS or OVERWRITES map entries keyed by filename
// stem; it never deletes, so blueprints registered via AddBlueprint (used
// by tests, and by any future in-memory-only blueprint) survive unless a
// file of the same name appears on disk, in which case the file wins.
// File counts here are tiny (single digits to low tens), so the
// glob+parse pass costs microseconds — trivial next to a wire round trip.
func reloadBlueprints(lib *blueprints.BlueprintLibrary) {
	_ = lib.LoadAll() // best-effort: a stat/glob error here just means "nothing new"
}

func expandBlueprint(lib *blueprints.BlueprintLibrary, name string, origin modifications.Coordinate) ([]blueprints.DigCommand, error) {
	reloadBlueprints(lib)
	bp := lib.GetBlueprint(name)
	if bp == nil {
		return nil, fmt.Errorf("blueprint %q not found (available: %s)", name, strings.Join(lib.ListBlueprints(), ", "))
	}
	return bp.ApplyAt(origin), nil
}

// listBlueprints re-scans (see reloadBlueprints) and returns a sorted
// name list — sorted so tool output is deterministic across calls.
func listBlueprints(lib *blueprints.BlueprintLibrary) []string {
	reloadBlueprints(lib)
	names := lib.ListBlueprints()
	sort.Strings(names)
	return names
}

// normalizeRegion builds a modifications.Region from two arbitrary corner
// coordinates, swapping each axis so Min<=Max regardless of call order.
func normalizeRegion(x1, y1, z1, x2, y2, z2 int16) modifications.Region {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	if z1 > z2 {
		z1, z2 = z2, z1
	}
	return modifications.Region{XMin: x1, XMax: x2, YMin: y1, YMax: y2, ZMin: z1, ZMax: z2}
}

// saveBlueprint captures the currently carved/modified tiles inside region
// (via a live region_scan query — see blueprints.CreateBlueprintFromRegionScan)
// and writes them to blueprints/<name>.csv via SaveToCSV. This replaced an
// earlier version that read the session-delta Modifications overlay
// (CreateBlueprintFromModifications, kept for reference/tests but no
// longer this function's caller): that overlay is wiped+rebaselined on
// every reconnect and its backing TILE_UPDATE stream empirically delivers
// nothing, so a save_blueprint call after any reconnect used to silently
// see zero tiles. region_scan reads the live map instead, so it works
// regardless of connection history. Returns the tile count and the path
// written.
func saveBlueprint(ctx context.Context, b *Bridge, name string, region modifications.Region) (int, string, error) {
	safeName := filepath.Base(strings.TrimSpace(name))
	if safeName == "" || safeName == "." || safeName == string(filepath.Separator) {
		return 0, "", fmt.Errorf("invalid blueprint name %q", name)
	}
	tiles, err := regionScan(ctx, b, region)
	if err != nil {
		return 0, "", fmt.Errorf("region_scan failed: %w", err)
	}
	bp, err := blueprints.CreateBlueprintFromRegionScan(tiles, region, safeName)
	if err != nil {
		return 0, "", err
	}
	path := filepath.Join("blueprints", safeName+".csv")
	if err := blueprints.SaveToCSV(bp, path); err != nil {
		return 0, "", fmt.Errorf("wrote blueprint but failed to save to %s: %w", path, err)
	}
	return len(bp.Digs), path, nil
}

// summarizeDryRun previews what a blueprint would designate: counts per
// dig type and a solid-vs-open breakdown (rooms should carve solid rock;
// open targets usually mean a mis-placed origin).
func summarizeDryRun(topo *topology.TopologyOverlay, cmds []blueprints.DigCommand) string {
	byType := map[string]int{}
	open, solid := 0, 0
	for _, c := range cmds {
		byType[c.DigType]++
		if topo != nil && topo.GetTileState(c.X, c.Y, c.Z) == topology.StateOpen {
			open++
		} else {
			solid++
		}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "DRY RUN: %d tiles would be designated (", len(cmds))
	first := true
	for k, v := range byType {
		if !first {
			sb.WriteString(", ")
		}
		first = false
		fmt.Fprintf(&sb, "%s=%d", k, v)
	}
	fmt.Fprintf(&sb, "); %d into solid ground (good), %d onto already-open tiles (check origin!)", solid, open)
	return sb.String()
}

// digRun is a maximal contiguous run of tiles sharing (DigType, Z, Y),
// sorted along X, that coalesceDigRuns groups so applyBlueprintCmds can
// send it as ONE SendDigRegion(X1,Y,Z, X2,Y,Z) wire call instead of one
// call per tile.
type digRun struct {
	DigType string
	Z, Y    int16
	X1, X2  int16 // inclusive
	Count   int   // == X2-X1+1; tracked explicitly so callers never re-derive it
}

// coalesceDigRuns groups cmds by (DigType, Z, Y), sorts each group by X,
// and splits it into maximal contiguous (no-gap) runs. Pure function —
// no wire calls — so it's unit-testable on its own. Duplicate (X,Y,Z)
// entries within a group are deduplicated before run-splitting so a
// blueprint that (accidentally) lists the same tile twice doesn't inflate
// a run's tile count or fracture it at the duplicate.
func coalesceDigRuns(cmds []blueprints.DigCommand) []digRun {
	type key struct {
		DigType string
		Z, Y    int16
	}
	groups := make(map[key]map[int16]struct{})
	order := make([]key, 0, len(cmds))
	for _, c := range cmds {
		k := key{c.DigType, c.Z, c.Y}
		xs, ok := groups[k]
		if !ok {
			xs = make(map[int16]struct{})
			groups[k] = xs
			order = append(order, k)
		}
		xs[c.X] = struct{}{}
	}

	runs := make([]digRun, 0, len(cmds))
	for _, k := range order {
		xset := groups[k]
		xs := make([]int16, 0, len(xset))
		for x := range xset {
			xs = append(xs, x)
		}
		sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })

		i := 0
		for i < len(xs) {
			j := i
			for j+1 < len(xs) && xs[j+1] == xs[j]+1 {
				j++
			}
			runs = append(runs, digRun{
				DigType: k.DigType, Z: k.Z, Y: k.Y,
				X1: xs[i], X2: xs[j], Count: j - i + 1,
			})
			i = j + 1
		}
	}
	return runs
}

// applyBlueprintCmds sends the expanded digs, coalesced into contiguous
// same-(digType,Z,Y) runs (coalesceDigRuns) so a 700-tile pod costs one
// SendDigRegion call per run instead of one per tile. A run's ok/fail
// count is attributed wholesale from its single ack — DFHack's dig
// designation either accepts a rectangle or it doesn't; there's no
// partial-tile failure mode to split out.
//
// "Designated" here means ackDesignated, not res.Success: an ACK_STATUS_
// PARTIAL run is one the plugin fully designated while warning that it
// destroys something (the dig-over-a-carved-stair case). Counting those as
// failures would report every tile of a completed run as FAILED and skip the
// b.Digs record, so the returned partialWarn carries the plugin's warning
// text to the tool output instead of dropping it — the run is ok AND the
// caveat is spoken.
//
// Returns (okCount, failCount, firstErr, partialWarn).
func applyBlueprintCmds(ctx context.Context, b *Bridge, cmds []blueprints.DigCommand) (int, int, string, string) {
	okCount, failCount := 0, 0
	var firstErr string
	var firstPartial string
	partialRuns := 0
	for _, run := range coalesceDigRuns(cmds) {
		dt, err := digTypeFromName(run.DigType)
		if err != nil {
			failCount += run.Count
			if firstErr == "" {
				firstErr = err.Error()
			}
			continue
		}
		res, err := b.Exec.SendDigRegion(dt, run.X1, run.Y, run.Z, run.X2, run.Y, run.Z)
		what := fmt.Sprintf("run %s (%d,%d,%d)-(%d,%d,%d)", run.DigType, run.X1, run.Y, run.Z, run.X2, run.Y, run.Z)
		if !ackDesignated(res, err) {
			failCount += run.Count
			if firstErr == "" {
				firstErr = ackText(res, err, what)
			}
			continue
		}
		if !res.Success {
			partialRuns++
			if firstPartial == "" {
				firstPartial = ackText(res, err, what)
			}
		}
		okCount += run.Count
		// Feed the session dig record so a later designate_dig beside this
		// blueprint's still-solid tiles doesn't hint "not yet connected".
		b.Digs.add(run.X1, run.Y, run.Z, run.X2, run.Y, run.Z)
	}
	partialWarn := ""
	if partialRuns > 0 {
		partialWarn = fmt.Sprintf("%d run%s designated WITH A CAVEAT — first: %s", partialRuns, plural(partialRuns), firstPartial)
	}
	return okCount, failCount, firstErr, partialWarn
}
