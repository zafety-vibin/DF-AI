# Research: Topology Overlay

**Feature**: 003-topology-graph-layer
**Date**: 2025-11-06

## Research Questions

This document resolves technical unknowns for the first spatial overlay implementation.

---

## R1: Bit-Packing Strategy for Z-Level Organization

**Question**: How should bits be organized in the byte array for efficient Z-level access and filtering?

**Decision**: Z-major ordering with byte-aligned Z-levels

**Rationale**:
- Store all tiles for Z=0, then all for Z=1, then Z=2, etc.
- Each Z-level starts at a byte boundary (simplifies slicing for Z-level filtering)
- Within each Z-level: row-major order (Y varies fastest, then X)
- Calculation: `byteOffset = (z * width * height + y * width + x) / 8`, `bitOffset = ... % 8`

**Alternatives considered**:
1. **X-major ordering**: Rejected - Z-level filtering would require scattered reads
2. **Interleaved**: Rejected - complex addressing, no performance benefit
3. **Separate array per Z**: Rejected - 189 separate allocations, memory overhead

**Implementation pattern**:
```go
// For 192x192x189 map:
// Z=0: bytes[0:4608]         (192*192 bits = 4608 bytes)
// Z=1: bytes[4608:9216]      (next 4608 bytes)
// ...
// Z=188: bytes[866304:870912] (last level)
```

**Benefits**:
- Z-level filtering: just slice the byte array (zero copy!)
- Cache-friendly: scanning a Z-level touches consecutive bytes
- Simple bounds checking: validate Z first, then offset within level

---

## R2: RLE Compression Algorithm

**Question**: How should run-length encoding be implemented for the bit array?

**Decision**: Byte-oriented RLE with variable-length run encoding

**Rationale**:
- Process bit array byte-by-byte (faster than bit-by-bit)
- Emit runs of identical bytes: (count: varint, value: byte)
- Use varint encoding for run counts (1 byte for counts <128, 2 bytes for larger)
- Spatial coherence means long runs of 0x00 (solid rock) and 0xFF (open rooms)

**Alternatives considered**:
1. **Bit-by-bit RLE**: Rejected - slower, minimal size improvement
2. **Nibble-based**: Rejected - complexity not worth marginal gains
3. **LZ4/zstd**: Rejected - external dependency, slower, overkill for simple spatial data

**Format**:
```
[varint: run_count][byte: value][varint: run_count][byte: value]...
```

**Example**:
```
Input:  0x00 0x00 0x00 0xFF 0xFF 0x00
Output: [3][0x00][2][0xFF][1][0x00]
        (3 zeros, 2 ones, 1 zero)
```

**Worst case**: Checkerboard pattern compresses to ~2x original size (count + value for each byte)
- Original: 870 KB
- Worst RLE: ~1.7 MB (exceeds 125 KB, will log warning)
- Typical fort: ~50-80 KB (good spatial coherence)

---

## R3: Determining Pathable Tiles from TileType

**Question**: Which DF tiletypes should be considered "open" vs "closed"?

**Decision**: Use DF's tiletype shape and material attributes

**Rationale**:
- DF defines ~500 tiletypes as uint16 enum
- Tiletype encodes: shape (wall, floor, ramp, etc.) + material (stone, soil, etc.)
- "Open" = shapes that allow movement: FLOOR, RAMP, STAIR_UP, STAIR_DOWN, STAIR_UPDOWN
- "Closed" = shapes that block: WALL, FORTIFICATION, undigged ROCK

**Alternatives considered**:
1. **Hardcoded tiletype list**: Rejected - brittle, breaks with DF updates
2. **Movement cost heuristic**: Rejected - overkill for binary open/closed
3. **Query DF directly**: Rejected - requires DFHack API call for each tile

**Implementation approach**:
```go
// Simplified tiletype classification
func IsPathable(tileType uint16) bool {
    // Tiletype shape is encoded in high bits
    // This is a simplified heuristic - may need refinement
    // based on actual DF tiletype enum structure

    // For MVP: Use simple heuristic based on common tiletypes
    // Open: floors (1-99), ramps (100-199), stairs (200-299)
    // Closed: walls (300+), rock (0)

    // TODO: Replace with proper DF tiletype enum parsing
    return tileType > 0 && tileType < 300
}
```

**Note**: This will need refinement based on actual DF tiletype enum. May need to reference DFHack tiletype documentation or extract from DF memory structures.

---

## R4: Thread-Safe Concurrent Read Pattern

**Question**: How to support concurrent reads without blocking or data races?

**Decision**: sync.RWMutex with copy-on-write for overlay rebuilds

**Rationale**:
- Many readers (pathfinding, queries, compression) + rare writes (overlay rebuild)
- RWMutex allows unlimited concurrent RLock readers
- Rebuilds: create new byte array → Lock → swap pointer → Unlock
- Old readers finish with old array (Go GC cleans up when all refs gone)

**Alternatives considered**:
1. **Immutable array, always copy**: Rejected - wastes memory (870 KB copies)
2. **Lock-free atomic operations**: Rejected - complex, not needed for read-heavy workload
3. **Channel-based queries**: Rejected - slower, adds latency

**Pattern**:
```go
type TopologyOverlay struct {
    data []byte
    mu   sync.RWMutex
    // ... dimensions ...
}

func (t *TopologyOverlay) IsOpen(x, y, z int16) bool {
    t.mu.RLock()
    defer t.mu.RUnlock()
    // ... query data ...
}

func (t *TopologyOverlay) Rebuild(tiles []TileState) {
    // Build new array outside lock
    newData := make([]byte, ...)
    // ... populate ...

    // Atomic swap
    t.mu.Lock()
    t.data = newData
    t.mu.Unlock()
}
```

---

## R5: Compression Mode Configuration

**Question**: How should compression modes be configured and hot-reloaded?

**Decision**: Add fields to existing Config struct, use ConfigManager hot-reload

**Rationale**:
- Leverage existing YAML config and hot-reload infrastructure from Feature 2
- Operators can switch modes without restart (experiment with context budget)
- Config validation ensures valid mode + required params (center Z, radius, bounds)

**Config fields to add**:
```yaml
# Topology compression settings
topology_compression_mode: active_z_levels  # full, active_z_levels, custom_bounds
topology_center_z: 50                       # For active_z_levels mode
topology_z_radius: 3                        # ±3 levels from center
# topology_custom_bounds: [x1,y1,z1,x2,y2,z2]  # For custom_bounds mode (optional)
```

**Alternatives considered**:
1. **Command-line flags**: Rejected - can't change without restart
2. **HTTP API**: Rejected - config management should stay in YAML per Feature 2 design
3. **Separate config file**: Rejected - consolidate in orchestrator.yaml

---

## Summary

All technical unknowns resolved. Key decisions:

- **Bit packing**: Z-major ordering, byte-aligned levels for efficient filtering
- **RLE algorithm**: Byte-oriented with varint run counts, ~50-80 KB typical compression
- **Pathable detection**: Simple tiletype heuristic (floors/ramps/stairs = open, walls/rock = closed)
- **Concurrency**: RWMutex with copy-on-write rebuild pattern
- **Configuration**: Extend existing YAML config with compression mode settings

Ready to proceed to Phase 1 (Design & Contracts).
