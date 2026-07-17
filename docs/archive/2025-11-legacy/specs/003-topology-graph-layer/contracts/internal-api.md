# Internal API: Topology Overlay

**Feature**: 003-topology-graph-layer
**Type**: Internal Go API (not HTTP REST)

This document specifies the Go package API for the topology overlay.

---

## Package: internal/topology

### Construction & Lifecycle

```go
// NewTopologyOverlay creates an empty topology overlay with given dimensions
func NewTopologyOverlay(width, height, depth uint16) *TopologyOverlay

// BuildFromTiles populates the overlay from tile data
// Extracts open/closed bit for each tile based on tiletype
// Time complexity: O(n) where n = number of tiles
// Thread-safe: Acquires write lock during build
func (t *TopologyOverlay) BuildFromTiles(tiles []TileState) error
```

**Error Conditions**:
- Returns error if `len(tiles)` doesn't match `width × height × depth`
- Returns error if dimensions are invalid (zero or > 256)

---

### Query Operations

```go
// IsOpen returns whether the tile at (x, y, z) is pathable
// Returns: (isOpen bool, error)
// Error if coordinates out of bounds
// Performance: <1 microsecond
// Thread-safe: Acquires read lock
func (t *TopologyOverlay) IsOpen(x, y, z int16) (bool, error)

// GetOpenTileCount counts open tiles in the specified region
// Returns: (count uint32, error)
// Error if bounds are invalid or out of range
// Performance: <100 microseconds for 1000 tiles
// Thread-safe: Acquires read lock
func (t *TopologyOverlay) GetOpenTileCount(
    xMin, xMax, yMin, yMax, zMin, zMax int16,
) (uint32, error)

// GetDimensions returns the map dimensions
// Thread-safe: No lock needed (dimensions immutable after creation)
func (t *TopologyOverlay) GetDimensions() (width, height, depth uint16)
```

**Query Error Handling**:
- Out-of-bounds coordinates: `error: "coordinate out of bounds: (x,y,z)"`
- Invalid region (min > max): `error: "invalid region bounds"`
- Nil overlay: `error: "topology overlay not initialized"`

---

### Metrics & Introspection

```go
// GetMemoryUsage returns bytes used by the bit array
// Does not include struct overhead, just the []byte slice
func (t *TopologyOverlay) GetMemoryUsage() uint64

// GetOpenPercentage returns percentage of tiles that are open (0.0 to 100.0)
// Useful for logging and understanding fort layout density
func (t *TopologyOverlay) GetOpenPercentage() float64

// GetBuildTime returns when the overlay was last built
func (t *TopologyOverlay) GetBuildTime() time.Time
```

---

### Compression Operations

```go
// Compress creates an RLE-compressed representation of the overlay
// Supports three modes: full, active_z_levels, custom_bounds
// Returns CompressedTopology with metadata
// Performance: <10 milliseconds for full map
// Thread-safe: Acquires read lock during compression
func (t *TopologyOverlay) Compress(config CompressionConfig) (*CompressedTopology, error)
```

**Compression Modes**:

**Mode: "full"**
- Compresses all Z-levels (0 to depth-1)
- Target size: ≤125 KB
- Use case: AI needs to see entire fort layout

**Mode: "active_z_levels"**
- Compresses only `[centerZ - radius, centerZ + radius]` (clamped to valid range)
- Target size: ~3-5% of full (e.g., 7/189 levels = ~5 KB)
- Use case: AI focused on specific construction/mining area

**Mode: "custom_bounds"**
- Compresses only tiles within explicit XYZ bounding box
- Target size: Varies based on region size
- Use case: AI analyzing specific room or zone

**Error Conditions**:
- Returns error if config.Mode is invalid
- Returns error if required params missing (e.g., active_z_levels without CenterZ)
- Logs warning if compressed size exceeds 125 KB (doesn't error, still returns data)

---

### Decompression & Validation

```go
// Decompress reconstructs the original bit array from RLE data
// Used for validation and testing (not for normal operation)
// Performance: <20 milliseconds
// Returns []byte matching original overlay.data
func (c *CompressedTopology) Decompress() ([]byte, error)

// Validate performs round-trip test: compress → decompress → compare
// Returns error if decompressed data doesn't match original
// Used in tests and optional runtime validation
func (c *CompressedTopology) Validate(original *TopologyOverlay) error
```

**Validation Process**:
1. Decompress the RLE data
2. Compare byte-by-byte with original overlay.data
3. Return error if any byte differs (indicates lossy compression or bug)

---

## Package: internal/topology (Helper Functions)

### Tiletype Classification

```go
// IsPathable determines if a DF tiletype is walkable
// Based on tiletype shape (floor, ramp, stairs = open; wall, rock = closed)
// Input: TileType uint16 from TileState struct
// Returns: true if tile allows movement, false if blocked
func IsPathable(tileType uint16) bool
```

**Classification Rules** (MVP heuristic):
- `0`: Undigged rock → `false`
- `1-299`: Floors, ramps, stairs → `true`
- `300+`: Walls, fortifications → `false`

**Note**: This is a simplified heuristic. May need refinement based on actual DF tiletype enum. Future work: parse DFHack tiletype definitions for accurate classification.

---

## Configuration Integration

### Config Struct Extensions (internal/config/config.go)

```go
type Config struct {
    // ... existing fields ...

    // Topology compression settings
    TopologyCompressionMode string        `yaml:"topology_compression_mode"` // full, active_z_levels, custom_bounds
    TopologyCenterZ         uint16        `yaml:"topology_center_z"`
    TopologyZRadius         uint16        `yaml:"topology_z_radius"`
    TopologyCustomBounds    *BoundingBox  `yaml:"topology_custom_bounds,omitempty"`
}
```

**Default Values** (in setDefaults):
```go
if c.TopologyCompressionMode == "" {
    c.TopologyCompressionMode = "full"
}
if c.TopologyZRadius == 0 {
    c.TopologyZRadius = 3
}
```

**Validation** (in ConfigManager.validate):
```go
validModes := map[string]bool{"full": true, "active_z_levels": true, "custom_bounds": true}
if !validModes[cfg.TopologyCompressionMode] {
    return error
}
```

---

## Usage Examples

### Building Overlay from Tile Data

```go
// Receive full state from DFHack
state := <-client.SubscribeFullState()

// Create and build topology overlay
overlay := topology.NewTopologyOverlay(state.Width, state.Height, state.Depth)
if err := overlay.BuildFromTiles(state.Tiles); err != nil {
    logger.Error("failed to build topology", err)
    return
}

logger.Info("topology overlay built",
    logging.Field{Key: "memory_kb", Value: overlay.GetMemoryUsage() / 1024},
    logging.Field{Key: "open_pct", Value: overlay.GetOpenPercentage()})
```

### Querying Topology

```go
// Single tile query
isOpen, err := overlay.IsOpen(50, 50, 5)
if err != nil {
    logger.Error("invalid coordinates", err)
    return
}

if isOpen {
    // Tile is pathable - safe to designate for use
} else {
    // Tile is blocked - wall or rock
}

// Region query (find open tiles in 10×10×5 area)
openCount, err := overlay.GetOpenTileCount(45, 55, 45, 55, 3, 8)
logger.Info("region analysis", logging.Field{Key: "open_tiles", Value: openCount})
```

### Compressing for LLM

```go
// Get compression config from YAML
config := topology.CompressionConfig{
    Mode:    cfg.TopologyCompressionMode,
    CenterZ: cfg.TopologyCenterZ,
    ZRadius: cfg.TopologyZRadius,
}

// Compress overlay
compressed, err := overlay.Compress(config)
if err != nil {
    logger.Error("compression failed", err)
    return
}

logger.Info("topology compressed",
    logging.Field{Key: "mode", Value: config.Mode},
    logging.Field{Key: "size_kb", Value: compressed.GetSize() / 1024},
    logging.Field{Key: "ratio", Value: compressed.GetRatio()})

// Validate round-trip (optional, for testing)
if err := compressed.Validate(overlay); err != nil {
    logger.Error("compression validation failed", err)
}
```

---

## Performance Expectations

Based on Feature 2 benchmarks showing ~10 ns for map lookups:

- **IsOpen query**: Expected ~50 ns (bit extraction overhead)
- **Region query (1000 tiles)**: Expected ~50 μs (1000 × 50ns)
- **Overlay build**: Expected ~100ms (6.9M tiles × tiletype check)
- **RLE compression**: Expected ~5-8ms (iterate 870 KB bytes, emit runs)
- **Memory**: Exactly 870,912 bytes for 192×192×189 map (6,967,296 bits ÷ 8)
