package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/df-ai/orchestrator/internal/blueprints"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// regionScanResponse mirrors dfhack-plugin/queries.cpp's handleRegionScan
// JSON response.
type regionScanResponse struct {
	Count int `json:"count"`
	Tiles []struct {
		X    int16  `json:"x"`
		Y    int16  `json:"y"`
		Z    int16  `json:"z"`
		Kind string `json:"kind"`
	} `json:"tiles"`
}

// parseRegionScanResponse decodes the plugin's region_scan query response.
// Split out from regionScan so it's unit-testable directly against canned
// bytes — the same shape parseFortFootprintResponse's test already uses
// for plugin-response parsing.
func parseRegionScanResponse(raw []byte) ([]blueprints.RegionScanTile, error) {
	var resp regionScanResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("region_scan: unparseable response: %w", err)
	}
	tiles := make([]blueprints.RegionScanTile, 0, len(resp.Tiles))
	for _, t := range resp.Tiles {
		tiles = append(tiles, blueprints.RegionScanTile{X: t.X, Y: t.Y, Z: t.Z, Kind: t.Kind})
	}
	return tiles, nil
}

// regionScan calls the plugin's region_scan query (a live MapExtras::MapCache
// scan — see queries.cpp's handleRegionScan) over region and decodes the
// per-tile result. This is the live map-state replacement for reading
// wm.Observed.Modifications: that overlay is a session-delta journal fed by
// a TILE_UPDATE stream that empirically delivers nothing, and is
// wiped+rebaselined on every reconnect (bridge.go's SetOverlays call from a
// fresh FULL_STATE), so it cannot answer "what's actually dug right now"
// once a session has restarted mid-fort.
func regionScan(ctx context.Context, b *Bridge, region modifications.Region) ([]blueprints.RegionScanTile, error) {
	args := fmt.Sprintf(`{"x1":%d,"y1":%d,"z1":%d,"x2":%d,"y2":%d,"z2":%d}`,
		region.XMin, region.YMin, region.ZMin, region.XMax, region.YMax, region.ZMax)
	raw, err := b.Query(ctx, "region_scan", args)
	if err != nil {
		return nil, err
	}
	return parseRegionScanResponse(raw)
}

// countDugTiles counts every tile NOT of kind "construction" — built
// walls/floors are player modifications too, but not DUG ones. Mirrors
// blueprints.CreateBlueprintFromRegionScan's identical construction-skip
// filter, so a predicate reading this count and a save_blueprint call
// reading the same region_scan response never disagree about what "dug"
// means for a hand-picked box.
//
// excludeAmbiguousFloor additionally skips kind "floor" — the one
// region_scan kind natural, never-dwarf-touched ground can also produce:
// DF's tiletype carries no per-tile "a dwarf did this" flag once a floor
// tile isn't a stair/smooth/track/fortification/ramp, so a bare cavern
// floor at depth (material STONE/SOIL/MINERAL, same as regionScanKind's
// queries.cpp comment discusses) is indistinguishable from genuinely-dug
// floor by tiletype alone. liveDugTileCount passes true here exactly when
// fortFootprintMapState's mayIncludeNaturalCave flag says its auto-derived
// bbox can span such untouched terrain between two separate dig sites —
// every other kind (stairs, fortification, ramp, track, smooth,
// construction) stays strong evidence of player action regardless of the
// flag, so only the ambiguous "floor" kind needs to yield when the box
// itself can't be trusted. save_blueprint's own hand-picked-box capture
// (blueprints.CreateBlueprintFromRegionScan) never sets this — that caller
// already knows its box IS the dig site, so floor legitimately counts
// there.
func countDugTiles(tiles []blueprints.RegionScanTile, excludeAmbiguousFloor bool) uint32 {
	var n uint32
	for _, t := range tiles {
		if t.Kind == "construction" {
			continue
		}
		if excludeAmbiguousFloor && t.Kind == "floor" {
			continue
		}
		n++
	}
	return n
}

// modeDwarfZ returns the Z level with the most ALIVE dwarves (ties broken
// toward the lowest Z, for determinism) — check_goals' cheap proxy for
// "where the fort's shelter activity currently concentrates". Scanning
// every z-level the fort has ever touched would cost one fort_footprint
// round trip per level (not cheap on a tall fort); sampling the single
// z-level with the most current occupants keeps the live cross-check to
// exactly two plugin round trips regardless of fort depth. ok=false when
// no alive dwarf is observed (embark instant, or a total wipe).
func modeDwarfZ(dwarves []protocol.EntityInfo) (z int16, ok bool) {
	counts := map[int16]int{}
	for _, d := range dwarves {
		if d.Dead {
			continue
		}
		counts[d.Z]++
	}
	if len(counts) == 0 {
		return 0, false
	}
	zs := make([]int16, 0, len(counts))
	for zz := range counts {
		zs = append(zs, zz)
	}
	sort.Slice(zs, func(i, j int) bool { return zs[i] < zs[j] })
	best, bestCount := zs[0], -1
	for _, zz := range zs {
		if counts[zz] > bestCount {
			bestCount = counts[zz]
			best = zz
		}
	}
	return best, true
}

// shelterScanMaxZLevels bounds liveDugTileCount's per-z fort_footprint +
// region_scan round trips. Each candidate z costs one fortFootprintBBox call
// (itself one fort_footprint query) plus one region_scan call — up to
// shelterScanMaxZLevels*2 sequential plugin round trips (each under
// Bridge.Query's 10s timeout) for a single check_goals call. 30 keeps that
// bounded to roughly the cost of a handful of `look scope=fort` calls rather
// than scaling unbounded with a very tall fort's full z-range.
const shelterScanMaxZLevels = 30

// candidateShelterZLevels returns the bounded, deduplicated, sorted set of
// z-levels liveDugTileCount should scan: every Z1..Z2 span recorded in the
// fort's declared zones (protocol.ZoneData, already resident in
// b.Snapshot().Zones.All) plus every distinct Z among currently-alive
// dwarves — data already resident Go-side, needing zero new plugin round
// trips to assemble. This directly fixes the too-narrow-sampling half of the
// undercount: modeDwarfZ's single busiest z alone collapsed a 9-level fort
// to one level's worth of tiles by construction.
//
// Falls back to [fallbackZ] (modeDwarfZ's single-z sample) when the union is
// empty — a fresh embark with nothing zoned or built yet, the one case where
// there is truly nothing else to scan; fallbackOK=false there too means no
// candidate exists at all. When the union exceeds shelterScanMaxZLevels, it
// is truncated (clamped=true) rather than silently scanning a random subset
// forever — callers must surface the clamp truthfully rather than imply a
// complete scan.
func candidateShelterZLevels(zones []protocol.ZoneData, dwarves []protocol.EntityInfo, fallbackZ int16, fallbackOK bool) (levels []int16, clamped bool) {
	seen := map[int16]bool{}
	for _, zn := range zones {
		lo, hi := zn.Z1, zn.Z2
		if lo > hi {
			lo, hi = hi, lo
		}
		for z := lo; z <= hi; z++ {
			seen[z] = true
		}
	}
	for _, d := range dwarves {
		if d.Dead {
			continue
		}
		seen[d.Z] = true
	}
	if len(seen) == 0 {
		if !fallbackOK {
			return nil, false
		}
		return []int16{fallbackZ}, false
	}
	levels = make([]int16, 0, len(seen))
	for z := range seen {
		levels = append(levels, z)
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })
	if len(levels) > shelterScanMaxZLevels {
		levels = levels[:shelterScanMaxZLevels]
		clamped = true
	}
	return levels, clamped
}

// liveDugTileSourceMultiZ formats liveDugTileCount's evidence-string
// provenance across every z-level actually scanned — the wave-7 replacement
// for the single-z liveDugTileSource this superseded (a live incident
// reported "8 dug tiles" from a single z=131 bbox sample of a 9-level fort
// whose street alone held ~70). Names the actual z-levels when there are
// few enough to read at a glance, falling back to a count+range once the
// list would be unwieldy. clamped reports the shelterScanMaxZLevels cap
// trimmed real candidate levels (see candidateShelterZLevels); anyNaturalCave
// carries forward the SAME upper-bound caveat the old single-z string used
// (see fortFootprintBBox's doc comment), now true if ANY scanned z's
// fort_footprint bbox flagged it, since every per-z box gets summed into one
// total the caveat must cover as a whole.
func liveDugTileSourceMultiZ(zs []int16, clamped, anyNaturalCave bool) string {
	var s string
	const maxNamedZ = 6
	if len(zs) <= maxNamedZ {
		parts := make([]string, len(zs))
		for i, z := range zs {
			parts[i] = strconv.Itoa(int(z))
		}
		s = fmt.Sprintf("region_scan summed over %d z-level(s): %s", len(zs), strings.Join(parts, ","))
	} else {
		s = fmt.Sprintf("region_scan summed over %d z-levels (%d..%d)", len(zs), zs[0], zs[len(zs)-1])
	}
	if clamped {
		s += fmt.Sprintf(" — capped at %d z-levels, real footprint may be taller", shelterScanMaxZLevels)
	}
	if anyNaturalCave {
		s += ", may include natural cave/terrain between separate dig sites on at least one level (upper bound, not exact)"
	}
	return s
}

// liveDugTileCount computes a live dug/modified-tile count for check_goals'
// HasModifiedAnything/HasShelter predicates. Wave-7 redesign: rather than
// sampling the single z-level with the most current dwarves (which
// collapsed a 9-level fort to one level's worth of tiles), it scans every
// z-level in candidateShelterZLevels' bounded union of zone spans + dwarf
// positions, and for EACH one calls fortFootprintBBox (tools_percept.go) —
// the strictly better helper `look scope=fort` already uses, which unions
// the map-state marker bbox with this session's own observed-dig bbox and
// adds a margin — rather than the raw, narrower fortFootprintMapState this
// replaced. This also fixes the within-bbox undercount: countDugTiles is
// now called with excludeAmbiguousFloor hardcoded false (a per-call
// all-or-nothing exclusion of kind=="floor" was discarding the majority of
// any real corridor/street on every level whose bbox happened to touch bare
// stone/soil, which is nearly all of them) — the natural-cave caveat is
// still honestly surfaced in the evidence string via
// liveDugTileSourceMultiZ, just no longer used to gate what gets counted.
// ok=false whenever NO candidate z-level yields anything (no dwarves and no
// zones, or every scanned z's query failed) — callers fall back to the
// session-delta overlay rather than treat that as "zero modifications".
func liveDugTileCount(ctx context.Context, b *Bridge) (count uint32, source string, ok bool) {
	dwarves := b.Snapshot().Entities.Dwarves
	fallbackZ, fallbackOK := modeDwarfZ(dwarves)
	levels, clamped := candidateShelterZLevels(b.Snapshot().Zones.All, dwarves, fallbackZ, fallbackOK)
	if len(levels) == 0 {
		return 0, "", false
	}
	mods := b.Mods()
	topo := b.Topo()
	if mods == nil || topo == nil {
		return 0, "", false
	}
	mapW, mapH, _ := topo.GetDimensions()

	var total uint32
	var anyNaturalCave bool
	scanned := make([]int16, 0, len(levels))
	for _, z := range levels {
		x1, y1, x2, y2, mayIncludeNaturalCave, found := fortFootprintBBox(ctx, b, mods, mapW, mapH, z, fortFootprintMargin)
		if !found {
			continue
		}
		region := modifications.Region{XMin: x1, XMax: x2, YMin: y1, YMax: y2, ZMin: z, ZMax: z}
		tiles, err := regionScan(ctx, b, region)
		if err != nil {
			continue
		}
		total += countDugTiles(tiles, false)
		if mayIncludeNaturalCave {
			anyNaturalCave = true
		}
		scanned = append(scanned, z)
	}
	if len(scanned) == 0 {
		return 0, "", false
	}
	return total, liveDugTileSourceMultiZ(scanned, clamped, anyNaturalCave), true
}
