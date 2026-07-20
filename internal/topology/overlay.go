package topology

import (
	"fmt"
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// TopologyOverlay stores three-state pathability data for all map tiles.
//
// Two parallel bit arrays:
//   - data:  open bit (1 = walkable, 0 = not walkable)
//   - known: known bit (1 = we have classification data, 0 = unknown)
//
// State derivation: open = data && known, closed = !data && known,
// unknown = !known. This decouples "we know this tile is closed" from
// "we have no data on this tile" — critical for partially-observable
// maps where most tiles are unallocated by DF until explored.
//
// Memory: 2 bits per tile. For a 192×192×129 map that's ~1.18 MB total
// (vs 580 KB for the legacy single-bit overlay). The legacy `data` array
// remains semantically "is open" so existing readers (legacy compression
// layer) continue to work.
type TopologyOverlay struct {
	data            []byte       // Bit-packed: 1 = open (walkable), 0 = not open.
	known           []byte       // Bit-packed: 1 = state is known, 0 = unknown.
	mu              sync.RWMutex // Protects data + known during reads/writes.
	width           uint16       // Map width (X dimension)
	height          uint16       // Map height (Y dimension)
	depth           uint16       // Map depth (Z dimension, number of levels)
	totalTiles      uint32       // Total tile count (width × height × depth)
	openTileCount   uint32       // Count of tiles in StateOpen.
	closedTileCount uint32       // Count of tiles in StateClosed.
	// Unknown count is implied: totalTiles - openTileCount - closedTileCount.
	buildTime   time.Time // When overlay was last built
	memoryBytes uint64    // Actual memory usage (data + known)
}

// NewTopologyOverlay creates a new topology overlay with given dimensions
// Allocates bit array with Z-major byte-aligned layout
// Each Z-level starts at a byte boundary for efficient filtering
func NewTopologyOverlay(width, height, depth uint16) *TopologyOverlay {
	if width == 0 || height == 0 || depth == 0 {
		return nil
	}

	totalTiles := uint32(width) * uint32(height) * uint32(depth)

	// Calculate byte array size: round up to nearest byte
	bytesNeeded := (totalTiles + 7) / 8

	return &TopologyOverlay{
		data:        make([]byte, bytesNeeded),
		known:       make([]byte, bytesNeeded),
		width:       width,
		height:      height,
		depth:       depth,
		totalTiles:  totalTiles,
		memoryBytes: uint64(bytesNeeded) * 2,
	}
}

// BuildFromTiles populates the overlay from tile data.
//
// Each tile is classified into one of three states (Open / Closed /
// Unknown) using the DFHack classification flags. Both bit arrays
// (data, known) are populated in a single pass.
//
// Thread-safe: acquires write lock for the swap.
func (t *TopologyOverlay) BuildFromTiles(tiles []protocol.TileState) error {
	if uint32(len(tiles)) != t.totalTiles {
		return fmt.Errorf("tile count mismatch: expected %d, got %d", t.totalTiles, len(tiles))
	}

	// Build new bit arrays outside lock.
	bytesNeeded := (t.totalTiles + 7) / 8
	newData := make([]byte, bytesNeeded)
	newKnown := make([]byte, bytesNeeded)
	openCount := uint32(0)
	closedCount := uint32(0)

	for _, tile := range tiles {
		state := ClassifyState(tile.Flags)
		if state == StateUnknown {
			continue // both bits stay 0
		}

		bitIndex := uint32(tile.Z)*uint32(t.width)*uint32(t.height) +
			uint32(tile.Y)*uint32(t.width) +
			uint32(tile.X)
		byteIndex := bitIndex / 8
		bitOffset := bitIndex % 8

		// Mark known.
		newKnown[byteIndex] |= (1 << bitOffset)

		if state == StateOpen {
			newData[byteIndex] |= (1 << bitOffset)
			openCount++
		} else {
			closedCount++
		}
	}

	t.mu.Lock()
	t.data = newData
	t.known = newKnown
	t.openTileCount = openCount
	t.closedTileCount = closedCount
	t.buildTime = time.Now()
	t.mu.Unlock()

	return nil
}

// GetDimensions returns the map dimensions
func (t *TopologyOverlay) GetDimensions() (width, height, depth uint16) {
	return t.width, t.height, t.depth
}

// GetMemoryUsage returns bytes used by the bit array
func (t *TopologyOverlay) GetMemoryUsage() uint64 {
	return t.memoryBytes
}

// GetOpenPercentage returns the fraction of KNOWN tiles that are open,
// expressed as a percentage. Unknown tiles are excluded from the
// denominator — including them produced a meaningless number dominated
// by the (large) volume of unallocated map blocks.
//
// Returns 0 if there are no known tiles yet.
func (t *TopologyOverlay) GetOpenPercentage() float64 {
	t.mu.RLock()
	known := t.openTileCount + t.closedTileCount
	open := t.openTileCount
	t.mu.RUnlock()

	if known == 0 {
		return 0.0
	}
	return float64(open) / float64(known) * 100.0
}

// Counts returns the current open / closed / unknown tile counts.
// Sum equals totalTiles.
func (t *TopologyOverlay) Counts() (open, closed, unknown uint32) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	open = t.openTileCount
	closed = t.closedTileCount
	unknown = t.totalTiles - open - closed
	return
}

// ZStats is a per-Z-level open/closed/unknown breakdown.
type ZStats struct {
	Z       int16
	Open    uint32
	Closed  uint32
	Unknown uint32
}

// OpenPercent returns the open fraction of known tiles on this Z, in 0..100.
// Returns 0 if no tiles are known on this Z.
func (s ZStats) OpenPercent() float64 {
	known := s.Open + s.Closed
	if known == 0 {
		return 0.0
	}
	return float64(s.Open) / float64(known) * 100.0
}

// StatsForZ counts open/closed/unknown tiles at the given Z level.
// Z out of bounds returns a zero ZStats.
func (t *TopologyOverlay) StatsForZ(z int16) ZStats {
	if z < 0 || z >= int16(t.depth) {
		return ZStats{Z: z}
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	tilesPerZ := uint32(t.width) * uint32(t.height)
	startBit := uint32(z) * tilesPerZ
	endBit := startBit + tilesPerZ

	stats := ZStats{Z: z}
	for bitIndex := startBit; bitIndex < endBit; bitIndex++ {
		byteIndex := bitIndex / 8
		bitOffset := bitIndex % 8
		isKnown := (t.known[byteIndex] >> bitOffset) & 1
		if isKnown == 0 {
			stats.Unknown++
			continue
		}
		isOpen := (t.data[byteIndex] >> bitOffset) & 1
		if isOpen == 1 {
			stats.Open++
		} else {
			stats.Closed++
		}
	}
	return stats
}

// StatsForZRange returns ZStats for every Z in [zMin, zMax] inclusive,
// clamped to the map's bounds. Newest-Z-first ordering: caller filters
// to the band they care about.
func (t *TopologyOverlay) StatsForZRange(zMin, zMax int16) []ZStats {
	if zMin > zMax {
		return nil
	}
	if zMin < 0 {
		zMin = 0
	}
	if zMax >= int16(t.depth) {
		zMax = int16(t.depth) - 1
	}
	out := make([]ZStats, 0, zMax-zMin+1)
	for z := zMax; z >= zMin; z-- {
		out = append(out, t.StatsForZ(z))
	}
	return out
}

// GetBuildTime returns when the overlay was last built
func (t *TopologyOverlay) GetBuildTime() time.Time {
	return t.buildTime
}

// SetTile updates the open/closed state for a single tile.
//
// In the three-state model, calling SetTile is implicitly a "this is now
// known" assertion: the tile transitions from whatever it was (often
// Unknown) to either Open or Closed depending on isOpen. Callers that
// have flags available should prefer SetTileState for explicit semantics.
//
// Thread-safe; used for incremental TILE_UPDATE handling.
func (t *TopologyOverlay) SetTile(x, y, z int16, isOpen bool) error {
	if isOpen {
		return t.SetTileState(x, y, z, StateOpen)
	}
	return t.SetTileState(x, y, z, StateClosed)
}

// SetTileState updates the tile to the given three-state value, keeping
// the open/closed/unknown counts in sync.
//
// Transitions:
//   - Unknown → Open / Closed: tile becomes known.
//   - Open / Closed → Unknown: tile becomes unknown again. Used when DF
//     reports a tile that was previously visible has gone back to fog of
//     war (rare but possible after mass cave-in / abandonment).
//   - Open ↔ Closed: standard mining / construction transition.
func (t *TopologyOverlay) SetTileState(x, y, z int16, state TileState) error {
	if x < 0 || x >= int16(t.width) {
		return fmt.Errorf("x coordinate out of bounds: %d (valid: 0-%d)", x, t.width-1)
	}
	if y < 0 || y >= int16(t.height) {
		return fmt.Errorf("y coordinate out of bounds: %d (valid: 0-%d)", y, t.height-1)
	}
	if z < 0 || z >= int16(t.depth) {
		return fmt.Errorf("z coordinate out of bounds: %d (valid: 0-%d)", z, t.depth-1)
	}

	bitIndex := uint32(z)*uint32(t.width)*uint32(t.height) +
		uint32(y)*uint32(t.width) +
		uint32(x)
	byteIndex := bitIndex / 8
	bitOffset := bitIndex % 8

	t.mu.Lock()
	defer t.mu.Unlock()

	wasKnown := (t.known[byteIndex] >> bitOffset) & 1
	wasOpen := (t.data[byteIndex] >> bitOffset) & 1

	// Decrement from previous state.
	if wasKnown == 1 {
		if wasOpen == 1 {
			t.openTileCount--
		} else {
			t.closedTileCount--
		}
	}

	// Set new state's bits.
	switch state {
	case StateOpen:
		t.known[byteIndex] |= (1 << bitOffset)
		t.data[byteIndex] |= (1 << bitOffset)
		t.openTileCount++
	case StateClosed:
		t.known[byteIndex] |= (1 << bitOffset)
		t.data[byteIndex] &^= (1 << bitOffset)
		t.closedTileCount++
	case StateUnknown:
		t.known[byteIndex] &^= (1 << bitOffset)
		t.data[byteIndex] &^= (1 << bitOffset)
	}

	return nil
}

// GetTileState returns the three-state classification of a tile.
// Out-of-bounds returns StateUnknown (no error — this is a "best effort"
// query suitable for region scans that don't need to distinguish bounds
// errors from unknown data).
func (t *TopologyOverlay) GetTileState(x, y, z int16) TileState {
	if x < 0 || x >= int16(t.width) ||
		y < 0 || y >= int16(t.height) ||
		z < 0 || z >= int16(t.depth) {
		return StateUnknown
	}
	bitIndex := uint32(z)*uint32(t.width)*uint32(t.height) +
		uint32(y)*uint32(t.width) +
		uint32(x)
	byteIndex := bitIndex / 8
	bitOffset := bitIndex % 8

	t.mu.RLock()
	defer t.mu.RUnlock()

	if (t.known[byteIndex]>>bitOffset)&1 == 0 {
		return StateUnknown
	}
	if (t.data[byteIndex]>>bitOffset)&1 == 1 {
		return StateOpen
	}
	return StateClosed
}

// GetTile returns the open/closed state for a single tile
// Thread-safe: Acquires read lock
func (t *TopologyOverlay) GetTile(x, y, z int16) (bool, error) {
	// Bounds validation
	if x < 0 || x >= int16(t.width) {
		return false, fmt.Errorf("x coordinate out of bounds: %d (valid: 0-%d)", x, t.width-1)
	}
	if y < 0 || y >= int16(t.height) {
		return false, fmt.Errorf("y coordinate out of bounds: %d (valid: 0-%d)", y, t.height-1)
	}
	if z < 0 || z >= int16(t.depth) {
		return false, fmt.Errorf("z coordinate out of bounds: %d (valid: 0-%d)", z, t.depth-1)
	}

	// Calculate bit index
	bitIndex := uint32(z)*uint32(t.width)*uint32(t.height) +
		uint32(y)*uint32(t.width) +
		uint32(x)

	byteIndex := bitIndex / 8
	bitOffset := bitIndex % 8

	// Read bit with read lock
	t.mu.RLock()
	defer t.mu.RUnlock()

	isOpen := (t.data[byteIndex] >> bitOffset) & 1
	return isOpen == 1, nil
}
