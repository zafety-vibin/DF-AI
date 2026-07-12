package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/df-ai/orchestrator/internal/blueprints"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/topology"
)

func expandBlueprint(lib *blueprints.BlueprintLibrary, name string, origin modifications.Coordinate) ([]blueprints.DigCommand, error) {
	bp := lib.GetBlueprint(name)
	if bp == nil {
		return nil, fmt.Errorf("blueprint %q not found (available: %s)", name, strings.Join(lib.ListBlueprints(), ", "))
	}
	return bp.ApplyAt(origin), nil
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

// applyBlueprintCmds sends the expanded digs. Groups per (digType, z) into
// single-tile SendDigRegion calls; fine for library-sized blueprints.
func applyBlueprintCmds(ctx context.Context, b *Bridge, cmds []blueprints.DigCommand) (int, int, string) {
	okCount, failCount := 0, 0
	var firstErr string
	for _, c := range cmds {
		dt, err := digTypeFromName(c.DigType)
		if err != nil {
			failCount++
			if firstErr == "" {
				firstErr = err.Error()
			}
			continue
		}
		res, err := b.Exec.SendDigRegion(dt, c.X, c.Y, c.Z, c.X, c.Y, c.Z)
		if err != nil || res == nil || !res.Success {
			failCount++
			if firstErr == "" {
				firstErr = ackText(res, err, fmt.Sprintf("tile (%d,%d,%d)", c.X, c.Y, c.Z))
			}
			continue
		}
		okCount++
	}
	return okCount, failCount, firstErr
}
