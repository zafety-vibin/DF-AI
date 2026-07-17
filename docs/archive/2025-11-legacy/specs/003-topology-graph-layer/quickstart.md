# Quickstart: Topology Overlay

**Feature**: 003-topology-graph-layer

This guide covers building the topology overlay, querying spatial data, and compressing for LLM context.

---

## Building the Topology Overlay

### Automatic Build on Connection

When DFHack connects and sends FULL_STATE, the topology overlay is automatically built:

```go
// In main.go or dfhack client handler:
case state := <-client.SubscribeFullState():
    // Build topology overlay from tiles
    overlay := topology.NewTopologyOverlay(state.Width, state.Height, state.Depth)
    if err := overlay.BuildFromTiles(state.Tiles); err != nil {
        logger.Error("failed to build topology", err)
        return
    }

    logger.Info("topology overlay ready",
        logging.Field{Key: "dimensions", Value: fmt.Sprintf("%dx%dx%d",
            state.Width, state.Height, state.Depth)},
        logging.Field{Key: "memory_kb", Value: overlay.GetMemoryUsage() / 1024},
        logging.Field{Key: "open_percentage", Value: overlay.GetOpenPercentage()})
```

**Expected logs**:
```json
{
  "level": "INFO",
  "msg": "topology overlay ready",
  "dimensions": "192x192x189",
  "memory_kb": 870,
  "open_percentage": 15.3
}
```

---

## Querying the Overlay

### Single Tile Query

Check if a specific tile is open (pathable):

```go
isOpen, err := overlay.IsOpen(50, 50, 5)
if err != nil {
    // Coordinate out of bounds
    logger.Error("invalid coordinate", err)
    return
}

if isOpen {
    logger.Info("tile is pathable",
        logging.Field{Key: "x", Value: 50},
        logging.Field{Key: "y", Value: 50},
        logging.Field{Key: "z", Value: 5})
} else {
    logger.Info("tile is blocked (wall/rock)")
}
```

### Region Query

Count open tiles in a bounding box:

```go
// Check 20x20x10 region around (50, 50, 5)
openCount, err := overlay.GetOpenTileCount(
    40, 60,  // X range: 40-60
    40, 60,  // Y range: 40-60
    0, 10,   // Z range: 0-10
)

if err != nil {
    logger.Error("invalid region", err)
    return
}

logger.Info("region analysis",
    logging.Field{Key: "open_tiles", Value: openCount},
    logging.Field{Key: "total_tiles", Value: 20 * 20 * 10},
    logging.Field{Key: "open_percentage", Value: float64(openCount) / 4000.0 * 100})
```

**Use cases for region queries**:
- "How much open space in this 10x10 room?"
- "Is this corridor clear for 50 tiles?"
- "Count pathable tiles on this Z-level"

---

## Compression for LLM Context

### Configuration

Edit `config/orchestrator.yaml` to set compression mode:

**Full Map Mode** (default):
```yaml
# Topology compression settings
topology_compression_mode: full
```

**Active Z-Levels Mode** (recommended for focused AI):
```yaml
topology_compression_mode: active_z_levels
topology_center_z: 50       # Current construction/mining level
topology_z_radius: 3        # Include ±3 levels (Z=47 to Z=53)
```

**Custom Bounds Mode** (for specific regions):
```yaml
topology_compression_mode: custom_bounds
topology_custom_bounds:
  x_min: 40
  x_max: 100
  y_min: 40
  y_max: 100
  z_min: 45
  z_max: 55
```

### Compressing the Overlay

```go
// Get compression config from YAML
config := topology.CompressionConfig{
    Mode:    cfg.TopologyCompressionMode,
    CenterZ: cfg.TopologyCenterZ,
    ZRadius: cfg.TopologyZRadius,
}

// Compress overlay using configured mode
compressed, err := overlay.Compress(config)
if err != nil {
    logger.Error("compression failed", err)
    return
}

logger.Info("topology compressed for LLM",
    logging.Field{Key: "mode", Value: config.Mode},
    logging.Field{Key: "compressed_kb", Value: compressed.GetSize() / 1024},
    logging.Field{Key: "ratio", Value: compressed.GetRatio()},
    logging.Field{Key: "z_levels", Value: len(compressed.ZLevelsIncluded)})
```

**Expected logs (full mode)**:
```json
{
  "level": "INFO",
  "msg": "topology compressed for LLM",
  "mode": "full",
  "compressed_kb": 78,
  "ratio": 0.089,
  "z_levels": 189
}
```

**Expected logs (filtered mode, ±3 Z)**:
```json
{
  "level": "INFO",
  "msg": "topology compressed for LLM",
  "mode": "active_z_levels",
  "compressed_kb": 4,
  "ratio": 0.004,
  "z_levels": 7
}
```

### Validation (Testing)

```go
// Validate lossless compression
if err := compressed.Validate(overlay); err != nil {
    logger.Error("compression validation failed - data corruption!", err)
    // This should never happen - indicates bug in RLE implementation
}
```

---

## Performance Benchmarks

### Running Topology Benchmarks

Execute all topology benchmarks:

```bash
cd /c/Users/zmanl/Projects/DF-AI
go test -bench=BenchmarkTopology ./tests/benchmark/ -benchmem
```

Execute specific benchmarks:

```bash
# Memory usage
go test -bench=BenchmarkTopologyMemory ./tests/benchmark/

# Query latency
go test -bench=BenchmarkTopologyQuery ./tests/benchmark/

# Compression speed
go test -bench=BenchmarkTopologyCompression ./tests/benchmark/
```

### Expected Results

**Memory Benchmark**:
```
BenchmarkTopologyMemory/Map_192x192x189-20    100    1500000 ns/op    870912 B/op    1 allocs/op
```
- Target: ~870 KB (6,967,296 bits ÷ 8)
- Interpretation: Exactly 1 bit per tile ✅

**Query Benchmark**:
```
BenchmarkTopologyQuery/IsOpen-20         50000000    25.3 ns/op    0 B/op    0 allocs/op
BenchmarkTopologyQuery/Region1000-20      2000000    750 ns/op     0 B/op    0 allocs/op
```
- IsOpen target: <1 μs (1000 ns) - achieved at 25 ns ✅
- Region (1000 tiles) target: <100 μs - achieved at 750 ns ✅

**Compression Benchmark**:
```
BenchmarkTopologyCompression/Full-20          200    5500000 ns/op    78000 B/op    50 allocs/op
BenchmarkTopologyCompression/Filtered-20     2000     800000 ns/op     4500 B/op    20 allocs/op
```
- Full mode target: <10ms - achieved at 5.5ms ✅
- Filtered mode target: <5ms - achieved at 0.8ms ✅
- Full compressed size: ~78 KB (under 125 KB target) ✅
- Filtered compressed size: ~4.5 KB ✅

### Race Detection

Test concurrent access safety:

```bash
go test -race ./internal/topology/...
```

**Expected**: `PASS` with no data race warnings

---

## Troubleshooting

### Compression Exceeds 125 KB

**Issue**: Logs show compressed size > 125 KB

**Likely Causes**:
- Highly fragmented fort (many small rooms, scattered mining)
- Checkerboard patterns (worst case for RLE)

**Solutions**:
1. Switch to "active_z_levels" mode (only compress active area)
2. Increase context budget for topology (adjust other overlays)
3. Log compression ratio to identify problematic fort layouts for research

### Query Returns Unexpected Results

**Issue**: IsOpen returns wrong value for known tiles

**Debugging**:
1. Check tiletype classification: Add logging to IsPathable() to see decisions
2. Verify tile data matches overlay: Compare TileState.TileType with overlay bit
3. Check bit addressing math: Verify byteIndex and bitOffset calculations

### Overlay Build Takes Too Long

**Issue**: BuildFromTiles() takes >1 second

**Expected**: ~50-100ms for 6.9M tiles

**Causes**:
- Slow tiletype classification logic
- Memory allocation issues

**Solutions**:
- Profile with `go test -cpuprofile=cpu.prof`
- Optimize IsPathable() if it's the bottleneck
- Pre-allocate byte array to avoid resizing

---

## Integration with Other Systems

### Future: Pathfinding (Feature 12)

Topology overlay will be queried by A* pathfinding:

```go
// Check if tile is traversable
if isOpen, _ := overlay.IsOpen(nextX, nextY, nextZ); isOpen {
    // Add to pathfinding frontier
}
```

### Future: Context Assembly (Feature 18)

Compressed topology will be included in LLM context:

```go
context := ContextPackage{
    Topology: compressed,  // CompressedTopology (78 KB)
    Hazards: ...,          // Future overlay (10 KB)
    Traffic: ...,          // Future overlay (20 KB)
    Semantics: ...,        // Future overlay (15 KB)
}
// Total: ~123 KB of 200 KB budget
```

### Metrics Endpoint

Topology metrics will be exposed via HTTP:

```bash
curl http://localhost:8081/metrics
```

**Response** (enhanced):
```json
{
  "topology": {
    "memory_bytes": 870912,
    "open_percentage": 15.3,
    "last_build": "2025-11-06T16:00:00Z",
    "last_compression": {
      "mode": "active_z_levels",
      "size_bytes": 4500,
      "ratio": 0.005,
      "z_levels": [47, 48, 49, 50, 51, 52, 53]
    }
  }
}
```

---

## Next Steps

After completing this feature:
- ✅ Topology overlay built from tile data
- ✅ Spatial queries working (<1 μs)
- ✅ RLE compression functional (<10ms, <125 KB)
- ✅ Configurable modes (full, filtered, custom)

**Ready for**:
- Feature 5: Topology Updates (keep overlay in sync with tile changes)
- Feature 12: A* Pathfinding (use overlay for traversability checks)
- Feature 18: LLM Context Assembly (include compressed topology)