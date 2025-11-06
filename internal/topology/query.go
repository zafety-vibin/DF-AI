package topology

import (
	"fmt"
)

// IsOpen returns whether the tile at (x, y, z) is pathable
// Thread-safe: Acquires read lock
// Performance: <1 microsecond per query
func (t *TopologyOverlay) IsOpen(x, y, z int16) (bool, error) {
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

	// Calculate bit index: Z-major ordering
	bitIndex := uint32(z)*uint32(t.width)*uint32(t.height) +
		uint32(y)*uint32(t.width) +
		uint32(x)

	// Extract bit from array (thread-safe read)
	t.mu.RLock()
	defer t.mu.RUnlock()

	byteIndex := bitIndex / 8
	bitOffset := bitIndex % 8
	isOpen := (t.data[byteIndex] >> bitOffset) & 1

	return isOpen == 1, nil
}

// GetOpenTileCount counts open tiles in the specified region
// Returns count of pathable tiles within bounding box
// Thread-safe: Acquires read lock
// Performance: <100 microseconds for 1000 tiles
func (t *TopologyOverlay) GetOpenTileCount(xMin, xMax, yMin, yMax, zMin, zMax int16) (uint32, error) {
	// Bounds validation
	if xMin < 0 || xMax >= int16(t.width) || xMin > xMax {
		return 0, fmt.Errorf("invalid X range: [%d, %d] (valid: 0-%d)", xMin, xMax, t.width-1)
	}
	if yMin < 0 || yMax >= int16(t.height) || yMin > yMax {
		return 0, fmt.Errorf("invalid Y range: [%d, %d] (valid: 0-%d)", yMin, yMax, t.height-1)
	}
	if zMin < 0 || zMax >= int16(t.depth) || zMin > zMax {
		return 0, fmt.Errorf("invalid Z range: [%d, %d] (valid: 0-%d)", zMin, zMax, t.depth-1)
	}

	// Scan region and count open tiles
	count := uint32(0)

	for z := zMin; z <= zMax; z++ {
		for y := yMin; y <= yMax; y++ {
			for x := xMin; x <= xMax; x++ {
				// Query each tile in region
				isOpen, err := t.IsOpen(x, y, z)
				if err != nil {
					return 0, err
				}
				if isOpen {
					count++
				}
			}
		}
	}

	return count, nil
}