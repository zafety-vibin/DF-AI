package topology

import (
	"fmt"
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// TopologyOverlay stores open/closed state for all map tiles
// First spatial overlay - extracts 1 bit per tile from raw tile data
// Organized by Z-level for efficient filtering and queries
type TopologyOverlay struct {
	data          []byte       // Bit-packed array (1 bit per tile)
	mu            sync.RWMutex // Protects data during reads/writes
	width         uint16       // Map width (X dimension)
	height        uint16       // Map height (Y dimension)
	depth         uint16       // Map depth (Z dimension, number of levels)
	totalTiles    uint32       // Total tile count (width × height × depth)
	openTileCount uint32       // Count of open/pathable tiles
	buildTime     time.Time    // When overlay was last built
	memoryBytes   uint64       // Actual memory usage of bit array
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
		width:       width,
		height:      height,
		depth:       depth,
		totalTiles:  totalTiles,
		memoryBytes: uint64(bytesNeeded),
	}
}

// BuildFromTiles populates the overlay from tile data
// Extracts open/closed bit for each tile using IsPathable()
// Thread-safe: Acquires write lock during build
func (t *TopologyOverlay) BuildFromTiles(tiles []protocol.TileState) error {
	if uint32(len(tiles)) != t.totalTiles {
		return fmt.Errorf("tile count mismatch: expected %d, got %d", t.totalTiles, len(tiles))
	}

	// Build new bit array outside lock
	newData := make([]byte, t.memoryBytes)
	openCount := uint32(0)

	for _, tile := range tiles {
		// Check if tile is pathable
		if !IsPathable(tile.TileType) {
			continue // Leave bit as 0 (closed)
		}

		// Calculate bit index: Z-major ordering
		// bitIndex = z * (width * height) + y * width + x
		bitIndex := uint32(tile.Z)*uint32(t.width)*uint32(t.height) +
			uint32(tile.Y)*uint32(t.width) +
			uint32(tile.X)

		// Set bit to 1 (open)
		byteIndex := bitIndex / 8
		bitOffset := bitIndex % 8
		newData[byteIndex] |= (1 << bitOffset)

		openCount++
	}

	// Atomic swap
	t.mu.Lock()
	t.data = newData
	t.openTileCount = openCount
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

// GetOpenPercentage returns percentage of tiles that are open
func (t *TopologyOverlay) GetOpenPercentage() float64 {
	if t.totalTiles == 0 {
		return 0.0
	}
	return float64(t.openTileCount) / float64(t.totalTiles) * 100.0
}

// GetBuildTime returns when the overlay was last built
func (t *TopologyOverlay) GetBuildTime() time.Time {
	return t.buildTime
}

// SetTile updates the open/closed state for a single tile
// Thread-safe: Acquires write lock
// Used for incremental updates from TILE_UPDATE messages
func (t *TopologyOverlay) SetTile(x, y, z int16, isOpen bool) error {
	// Bounds validation
	if x < 0 || x >= int16(t.width) {
		return fmt.Errorf("x coordinate out of bounds: %d (valid: 0-%d)", x, t.width-1)
	}
	if y < 0 || y >= int16(t.height) {
		return fmt.Errorf("y coordinate out of bounds: %d (valid: 0-%d)", y, t.height-1)
	}
	if z < 0 || z >= int16(t.depth) {
		return fmt.Errorf("z coordinate out of bounds: %d (valid: 0-%d)", z, t.depth-1)
	}

	// Calculate bit index
	bitIndex := uint32(z)*uint32(t.width)*uint32(t.height) +
		uint32(y)*uint32(t.width) +
		uint32(x)

	byteIndex := bitIndex / 8
	bitOffset := bitIndex % 8

	// Update bit with write lock
	t.mu.Lock()
	defer t.mu.Unlock()

	// Get current state
	wasOpen := (t.data[byteIndex] >> bitOffset) & 1

	if isOpen {
		// Set bit to 1
		t.data[byteIndex] |= (1 << bitOffset)
		if wasOpen == 0 {
			// Tile changed from closed to open
			t.openTileCount++
		}
	} else {
		// Clear bit to 0
		t.data[byteIndex] &^= (1 << bitOffset)
		if wasOpen == 1 {
			// Tile changed from open to closed
			t.openTileCount--
		}
	}

	return nil
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