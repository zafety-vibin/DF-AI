# DF-AI Development Guidelines

Auto-generated from all feature plans. Last updated: 2025-11-05

## Active Technologies
- Filesystem (config files), in-memory (metrics, health state) (002-foundation-infrastructure)
- In-memory bit array (~870 KB), no disk persistence (003-topology-graph-layer)
- Go 1.21+ + Go stdlib only (may need protocol enhancement) (004-hazard-overlays)
- In-memory sparse maps (~10-50 KB total) (004-hazard-overlays)

- Go 1.21+ (server), C++17 (DFHack plugin) (001-binary-protocol)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Go 1.21+ (server), C++17 (DFHack plugin)

## Code Style

Go 1.21+ (server), C++17 (DFHack plugin): Follow standard conventions

## Recent Changes
- 004-hazard-overlays: Added Go 1.21+ + Go stdlib only (may need protocol enhancement)
- 003-topology-graph-layer: Added Go 1.21+
- 002-foundation-infrastructure: Added Go 1.21+


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
