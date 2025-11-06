# Data Model: Topology Overlay

**Feature**: 003-topology-graph-layer
**Date**: 2025-11-06

## Overview

This document defines the data structures for the first spatial overlay: topology (open/closed state extraction from tile data).

---

## Topology Storage

### TopologyOverlay

**Purpose**: First XYZ overlay storing just open/closed state extracted from raw tile data. Enables fast spatial queries without parsing full 62 MB dataset.

**Fields**:
- `data` ([]byte): Bit-packed array storing open (1) or closed (0) for each tile
- `mu` (sync.RWMutex): Protects data during reads/writes
- `width` (uint16): Map width (X dimension)
- `height` (uint16): Map height (Y dimension)
- `depth` (uint16): Map depth (Z dimension, number of Z-levels)
- `totalTiles` (uint32): Total tile count (width × height × depth)
- `openTileCount` (uint32): Count of open tiles (for metrics)
- `buildTime` (time.Time): When overlay was last built
- `memoryBytes` (uint64): Actual memory usage of bit array

**Methods** (Public API):
- `NewTopologyOverlay(width, height, depth uint16) *TopologyOverlay`: Creates empty overlay
- `BuildFromTiles(tiles []TileState)`: Populates overlay from tile data
- `IsOpen(x, y, z int16) (bool, error)`: Query if single tile is open (validates bounds)
- `GetOpenTileCount(xMin, xMax, yMin, yMax, zMin, zMax int16) (uint32, error)`: Count open tiles in region
- `GetDimensions() (width, height, depth uint16)`: Returns map dimensions
- `GetMemoryUsage() uint64`: Returns bytes used by bit array
- `GetOpenPercentage() float64`: Returns percentage of tiles that are open (for metrics)

**Validation Rules**:
- Dimensions must be > 0 and ≤ 256 each
- Total tiles must not exceed 16.7M (256³)
- Coordinate queries validated: 0 ≤ x < width, 0 ≤ y < height, 0 ≤ z < depth
- Out-of-bounds queries return error, not panic

**Memory Layout**:
```
Z-major ordering (byte-aligned levels):
Byte[0..4607]:     Z=0 (192×192 = 36,864 tiles = 4,608 bytes)
Byte[4608..9215]:  Z=1 (next 4,608 bytes)
...
Byte[866304..870911]: Z=188 (last level)
```

**Bit Addressing**:
```
For tile (x, y, z):
  bitIndex = z * width * height + y * width + x
  byteIndex = bitIndex / 8
  bitOffset = bitIndex % 8
  isOpen = (data[byteIndex] >> bitOffset) & 1
```

**Thread Safety**:
- Reads: RLock (many concurrent readers allowed)
- Writes: Lock (exclusive, blocks all readers during rebuild)
- Copy-on-write: Build new array outside lock, swap pointer atomically

---

## Compression

### CompressionConfig

**Purpose**: Configures how topology is compressed for LLM context.

**Fields**:
- `Mode` (string): Compression mode - "full", "active_z_levels", "custom_bounds"
- `CenterZ` (uint16): Center Z-level for active_z_levels mode
- `ZRadius` (uint16): Radius in Z-levels (±3 means 7 levels total)
- `CustomBounds` (BoundingBox): For custom_bounds mode
  - `XMin, XMax, YMin, YMax, ZMin, ZMax` (int16): Explicit coordinate range

**Validation Rules**:
- Mode must be one of: full, active_z_levels, custom_bounds
- For active_z_levels: CenterZ and ZRadius required, must be valid
- For custom_bounds: BoundingBox required, coordinates must be valid
- Z-level filtering clamps to map bounds: `[max(0, center-radius), min(maxZ, center+radius)]`

**Default Values** (from config/orchestrator.yaml):
```yaml
topology_compression_mode: full
topology_center_z: 0       # Ignored in full mode
topology_z_radius: 3       # Ignored in full mode
```

---

### CompressedTopology

**Purpose**: RLE-encoded form of topology overlay for LLM context transmission.

**Fields**:
- `Data` ([]byte): RLE-compressed byte stream
- `OriginalWidth` (uint16): Map width (for decompression)
- `OriginalHeight` (uint16): Map height
- `OriginalDepth` (uint16): Map depth
- `CompressedSize` (uint32): Size of Data in bytes
- `UncompressedSize` (uint32): Original bit array size in bytes
- `CompressionRatio` (float64): Compressed / Uncompressed (lower = better)
- `Mode` (string): Which mode was used (full, active_z_levels, custom_bounds)
- `ZLevelsIncluded` ([]uint16): List of Z-levels in compressed data (for filtered modes)
- `CompressedAt` (time.Time): When compression was performed

**Methods**:
- `Compress(overlay *TopologyOverlay, config CompressionConfig) (*CompressedTopology, error)`: RLE compress with mode
- `Decompress() ([]byte, error)`: Reconstruct original bit array (for validation)
- `Validate(original *TopologyOverlay) error`: Round-trip test (compress → decompress → compare)
- `GetSize() uint32`: Returns compressed size in bytes
- `GetRatio() float64`: Returns compression ratio

**RLE Format**:
```
Header (12 bytes):
  [2: width][2: height][2: depth][1: mode][1: zLevelCount][4: dataLength]

ZLevel List (if filtered):
  [2 * zLevelCount: included Z-levels]

RLE Data:
  [varint: runCount][1: byte value][varint: runCount][1: byte value]...
```

**Varint Encoding**:
- 1 byte: values 0-127 (MSB=0)
- 2 bytes: values 128+ (MSB=1 in first byte)

**Example Compression**:
```
Original: 0x00 0x00 0x00 0xFF 0xFF 0x00  (6 bytes)
RLE:      [3][0x00][2][0xFF][1][0x00]    (6 bytes - no gain)

Original: 100 consecutive 0x00 bytes
RLE:      [100][0x00]                     (2 bytes - 50x compression!)
```

---

## Tiletype Classification

### TileType Helper

**Purpose**: Determines if DF tiletype is pathable (open) or blocked (closed).

**Data Source**: DF tiletype enum (uint16) from TileState.TileType field

**Classification Logic**:
```
Open (pathable):
- FLOOR shapes (1-99)
- RAMP shapes (100-199)
- STAIR shapes (200-299)
- Special: BROOK_BED, TREE_BRANCH (organic pathable)

Closed (blocked):
- WALL shapes (300+)
- FORTIFICATION (partially passable but treat as closed for safety)
- Undigged ROCK (0)
- OBSTACLE shapes
```

**Implementation Strategy**:
```go
// Simplified heuristic for MVP
func IsPathable(tileType uint16) bool {
    // 0 = undigged rock (closed)
    if tileType == 0 {
        return false
    }
    // 1-299 = floors, ramps, stairs (open)
    if tileType < 300 {
        return true
    }
    // 300+ = walls, fortifications (closed)
    return false
}

// Future enhancement: Parse actual DF tiletype enum
// Reference: DFHack tiletype.h or structures/df.tiletype.xml
```

**Note**: This is a simplified heuristic. May need refinement based on actual DF tiletype enum structure. Plan to log misclassifications during testing and adjust thresholds.

---

## Coordinate System

### Coordinate

**Purpose**: Represents a 3D position in the map for all spatial queries.

**Fields**:
- `X` (int16): X coordinate (0 to width-1)
- `Y` (int16): Y coordinate (0 to height-1)
- `Z` (int16): Z coordinate (0 to depth-1)

**Validation**:
- All coordinates must be non-negative
- Must be within map bounds (x < width, y < height, z < depth)
- Invalid coordinates return error, not panic

**Usage Pattern**:
```go
// Single tile query
isOpen, err := overlay.IsOpen(50, 50, 5)
if err != nil {
    // Handle out-of-bounds
}

// Region query
count, err := overlay.GetOpenTileCount(40, 60, 40, 60, 0, 10)
// Count open tiles in 20×20×10 region
```

---

## Relationships Between Entities

```
FULL_STATE message (6.9M TileState structs)
  │
  └─ BuildFromTiles() ──> TopologyOverlay (bit array, 870 KB)
                            │
                            ├─ IsOpen(x,y,z) ──> bool (query API)
                            │
                            └─ Compress(config) ──> CompressedTopology (<125 KB)
                                                      │
                                                      ├─ Decompress() ──> []byte (validation)
                                                      │
                                                      └─ LLM Context (future: Feature 18)

CompressionConfig (YAML)
  │
  └─ determines ──> Compress() behavior (full/filtered/custom)
```

---

## Data Flow: From Tiles to LLM

1. **Receive**: FULL_STATE with 6.9M TileState structs (62 MB over TCP)
2. **Extract**: For each tile, call IsPathable(TileType) → bit (870 KB bit array)
3. **Store**: TopologyOverlay.BuildFromTiles() → populate bit array with RWMutex
4. **Query**: AI pathfinding calls IsOpen(x,y,z) → <1 μs response
5. **Compress**: CompressedTopology.Compress(overlay, config) → RLE with mode → <125 KB
6. **Transmit** (future): Include compressed topology in LLM context (Feature 18)

---

## Performance Characteristics

| Operation | Target | Expected (based on benchmarks) |
|-----------|--------|--------------------------------|
| Overlay build | N/A | ~50ms for 6.9M tiles |
| Memory usage | <1 MB | ~870 KB (1 bit per tile) |
| IsOpen query | <1 μs | ~10-50 ns (proven from Feature 2 benchmarks) |
| Region query (1000 tiles) | <100 μs | ~10-50 μs (1000× single query) |
| Compression (full) | <10ms | ~5-8ms (simple RLE) |
| Compression (filtered) | <5ms | ~1-2ms (fewer levels) |
| Compressed size (full) | ≤125 KB | ~50-80 KB typical |
| Compressed size (filtered ±3Z) | ~5 KB | ~3-5 KB (7/189 levels) |

---

## Validation & Testing

| Test Type | What's Validated | Method |
|-----------|------------------|--------|
| Memory benchmark | Bit array uses ~870 KB | `go test -bench=BenchmarkTopologyMemory -benchmem` |
| Query benchmark | IsOpen <1 μs | `go test -bench=BenchmarkTopologyQuery` |
| Compression benchmark | RLE <10ms | `go test -bench=BenchmarkTopologyCompression` |
| Round-trip test | Lossless compression | Compress → Decompress → Compare |
| Race detection | No data races | `go test -race ./internal/topology/...` |
| Bounds validation | Out-of-bounds returns error | Unit tests with invalid coordinates |
