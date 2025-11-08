package modifications

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Errors
var (
	ErrOutOfBounds = errors.New("coordinate out of bounds")
)

// ModificationOverlay is a sparse storage overlay for tile modifications
// Tracks only player/AI tile changes (not natural tiles) - "fort signature"
type ModificationOverlay struct {
	mu             sync.RWMutex
	modifications  map[Coordinate]ModificationInfo
	totalCount     uint32    // Cached count
	lastUpdate     time.Time // Last update timestamp
	mapBounds      Bounds    // Map dimensions for validation
	boundsComputed bool      // Whether bounds have been computed
	cachedBounds   Region    // Cached bounding box of all modifications
}

// NewModificationOverlay creates a new sparse modification overlay
func NewModificationOverlay(bounds Bounds) *ModificationOverlay {
	return &ModificationOverlay{
		modifications: make(map[Coordinate]ModificationInfo),
		totalCount:    0,
		lastUpdate:    time.Now(),
		mapBounds:     bounds,
	}
}

// Add adds or updates a modification at the given coordinate (thread-safe write)
func (m *ModificationOverlay) Add(coord Coordinate, info ModificationInfo) error {
	// Validate bounds
	if coord.X < 0 || coord.X >= int16(m.mapBounds.Width) {
		return fmt.Errorf("%w: x=%d (max=%d)", ErrOutOfBounds, coord.X, m.mapBounds.Width-1)
	}
	if coord.Y < 0 || coord.Y >= int16(m.mapBounds.Height) {
		return fmt.Errorf("%w: y=%d (max=%d)", ErrOutOfBounds, coord.Y, m.mapBounds.Height-1)
	}
	if coord.Z < 0 || coord.Z >= int16(m.mapBounds.Depth) {
		return fmt.Errorf("%w: z=%d (max=%d)", ErrOutOfBounds, coord.Z, m.mapBounds.Depth-1)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this is a new coordinate
	_, existed := m.modifications[coord]
	m.modifications[coord] = info

	if !existed {
		m.totalCount++
	}

	m.lastUpdate = time.Now()
	m.boundsComputed = false // Invalidate cached bounds
	return nil
}

// Get retrieves modification info if it exists (thread-safe read)
func (m *ModificationOverlay) Get(coord Coordinate) (ModificationInfo, bool) {
	// Validate bounds
	if coord.X < 0 || coord.X >= int16(m.mapBounds.Width) {
		return ModificationInfo{}, false
	}
	if coord.Y < 0 || coord.Y >= int16(m.mapBounds.Height) {
		return ModificationInfo{}, false
	}
	if coord.Z < 0 || coord.Z >= int16(m.mapBounds.Depth) {
		return ModificationInfo{}, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.modifications[coord]
	return info, exists
}

// GetCount returns the total modification count (thread-safe read)
func (m *ModificationOverlay) GetCount() uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalCount
}

// GetBounds returns the bounding box containing all modifications (thread-safe read)
func (m *ModificationOverlay) GetBounds() Region {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return cached bounds if valid
	if m.boundsComputed {
		return m.cachedBounds
	}

	// If no modifications, return empty region
	if m.totalCount == 0 {
		return Region{XMin: 0, XMax: 0, YMin: 0, YMax: 0, ZMin: 0, ZMax: 0}
	}

	// Compute bounds from all modifications
	first := true
	var bounds Region

	for coord := range m.modifications {
		if first {
			bounds = Region{
				XMin: coord.X, XMax: coord.X,
				YMin: coord.Y, YMax: coord.Y,
				ZMin: coord.Z, ZMax: coord.Z,
			}
			first = false
		} else {
			if coord.X < bounds.XMin {
				bounds.XMin = coord.X
			}
			if coord.X > bounds.XMax {
				bounds.XMax = coord.X
			}
			if coord.Y < bounds.YMin {
				bounds.YMin = coord.Y
			}
			if coord.Y > bounds.YMax {
				bounds.YMax = coord.Y
			}
			if coord.Z < bounds.ZMin {
				bounds.ZMin = coord.Z
			}
			if coord.Z > bounds.ZMax {
				bounds.ZMax = coord.Z
			}
		}
	}

	m.cachedBounds = bounds
	m.boundsComputed = true
	return bounds
}

// GetModificationsInRegion returns all modifications within the bounding box (thread-safe read)
// Optionally filters by modification time (if since is non-zero)
func (m *ModificationOverlay) GetModificationsInRegion(region Region, since time.Time) map[Coordinate]ModificationInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[Coordinate]ModificationInfo)
	for coord, info := range m.modifications {
		// Check spatial bounds
		if coord.X >= region.XMin && coord.X <= region.XMax &&
			coord.Y >= region.YMin && coord.Y <= region.YMax &&
			coord.Z >= region.ZMin && coord.Z <= region.ZMax {
			// Check temporal filter if provided
			if since.IsZero() || info.DetectedAt.After(since) {
				result[coord] = info
			}
		}
	}
	return result
}

// LinkCommandToModifications associates a command ID with modifications in a region
// Used to track which command caused which modifications
func (m *ModificationOverlay) LinkCommandToModifications(commandID uint32, region Region, timestamp time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	linkedCount := 0
	for coord, info := range m.modifications {
		// Check if modification is in region and was detected around the command time
		// (within 5 seconds before or after)
		if coord.X >= region.XMin && coord.X <= region.XMax &&
			coord.Y >= region.YMin && coord.Y <= region.YMax &&
			coord.Z >= region.ZMin && coord.Z <= region.ZMax {

			timeDiff := info.DetectedAt.Sub(timestamp)
			if timeDiff >= -5*time.Second && timeDiff <= 5*time.Second {
				// Update the modification to link it to the command
				if info.CommandID == 0 { // Only link if not already linked
					info.CommandID = commandID
					m.modifications[coord] = info
					linkedCount++
				}
			}
		}
	}
	return linkedCount
}

// Clear removes all modifications (thread-safe write)
func (m *ModificationOverlay) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.modifications = make(map[Coordinate]ModificationInfo)
	m.totalCount = 0
	m.lastUpdate = time.Now()
	m.boundsComputed = false
}

// GetLastUpdateTime returns when the overlay was last updated
func (m *ModificationOverlay) GetLastUpdateTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastUpdate
}

// GetMemoryUsage returns approximate memory usage in bytes
func (m *ModificationOverlay) GetMemoryUsage() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Each map entry: Coordinate (6 bytes) + ModificationInfo (~32 bytes) + map overhead (~8 bytes)
	// = ~46 bytes per entry
	return uint64(m.totalCount) * 46
}
