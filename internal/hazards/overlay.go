package hazards

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

// Coordinate represents a 3D spatial position
type Coordinate struct {
	X int16
	Y int16
	Z int16
}

// HazardInfo stores metadata for a hazard at a specific coordinate
type HazardInfo struct {
	Severity   uint8     // Hazard severity/depth (0-255)
	Flags      uint8     // Bitfield: flow direction, standing/flowing, type flags
	DetectedAt time.Time // When hazard was first detected
}

// Bounds represents map dimensions
type Bounds struct {
	Width  uint16
	Height uint16
	Depth  uint16
}

// Region represents a 3D bounding box for spatial queries
type Region struct {
	XMin, XMax int16
	YMin, YMax int16
	ZMin, ZMax int16
}

// HazardOverlay is a sparse storage overlay for a single hazard type
type HazardOverlay struct {
	mu          sync.RWMutex
	hazards     map[Coordinate]HazardInfo
	hazardType  string    // For logging (e.g., "aquifer", "water", "lava")
	totalCount  uint32    // Cached count
	lastUpdate  time.Time // Last update timestamp
	mapBounds   Bounds    // Map dimensions for validation
}

// NewHazardOverlay creates a new sparse hazard overlay
func NewHazardOverlay(hazardType string, bounds Bounds) *HazardOverlay {
	return &HazardOverlay{
		hazards:    make(map[Coordinate]HazardInfo),
		hazardType: hazardType,
		totalCount: 0,
		lastUpdate: time.Now(),
		mapBounds:  bounds,
	}
}

// Contains checks if a hazard exists at the given coordinate (thread-safe read)
func (h *HazardOverlay) Contains(x, y, z int16) bool {
	// Validate bounds
	if x < 0 || x >= int16(h.mapBounds.Width) {
		return false
	}
	if y < 0 || y >= int16(h.mapBounds.Height) {
		return false
	}
	if z < 0 || z >= int16(h.mapBounds.Depth) {
		return false
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	coord := Coordinate{X: x, Y: y, Z: z}
	_, exists := h.hazards[coord]
	return exists
}

// Get retrieves hazard info if it exists (thread-safe read)
func (h *HazardOverlay) Get(x, y, z int16) (HazardInfo, bool) {
	// Validate bounds
	if x < 0 || x >= int16(h.mapBounds.Width) {
		return HazardInfo{}, false
	}
	if y < 0 || y >= int16(h.mapBounds.Height) {
		return HazardInfo{}, false
	}
	if z < 0 || z >= int16(h.mapBounds.Depth) {
		return HazardInfo{}, false
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	coord := Coordinate{X: x, Y: y, Z: z}
	info, exists := h.hazards[coord]
	return info, exists
}

// GetInRegion returns all hazard coordinates within the bounding box (thread-safe read)
func (h *HazardOverlay) GetInRegion(region Region) []Coordinate {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []Coordinate
	for coord := range h.hazards {
		if coord.X >= region.XMin && coord.X <= region.XMax &&
			coord.Y >= region.YMin && coord.Y <= region.YMax &&
			coord.Z >= region.ZMin && coord.Z <= region.ZMax {
			result = append(result, coord)
		}
	}
	return result
}

// GetCount returns the total hazard count (thread-safe read)
func (h *HazardOverlay) GetCount() uint32 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.totalCount
}

// Add adds or updates a hazard at the given coordinate (thread-safe write)
func (h *HazardOverlay) Add(coord Coordinate, info HazardInfo) error {
	// Validate bounds
	if coord.X < 0 || coord.X >= int16(h.mapBounds.Width) {
		return fmt.Errorf("%w: x=%d (max=%d)", ErrOutOfBounds, coord.X, h.mapBounds.Width-1)
	}
	if coord.Y < 0 || coord.Y >= int16(h.mapBounds.Height) {
		return fmt.Errorf("%w: y=%d (max=%d)", ErrOutOfBounds, coord.Y, h.mapBounds.Height-1)
	}
	if coord.Z < 0 || coord.Z >= int16(h.mapBounds.Depth) {
		return fmt.Errorf("%w: z=%d (max=%d)", ErrOutOfBounds, coord.Z, h.mapBounds.Depth-1)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if this is a new coordinate
	_, existed := h.hazards[coord]
	h.hazards[coord] = info

	if !existed {
		h.totalCount++
	}

	h.lastUpdate = time.Now()
	return nil
}

// Remove removes a hazard at the given coordinate (thread-safe write)
func (h *HazardOverlay) Remove(coord Coordinate) error {
	// Validate bounds (but don't error if not found)
	if coord.X < 0 || coord.X >= int16(h.mapBounds.Width) {
		return fmt.Errorf("%w: x=%d (max=%d)", ErrOutOfBounds, coord.X, h.mapBounds.Width-1)
	}
	if coord.Y < 0 || coord.Y >= int16(h.mapBounds.Height) {
		return fmt.Errorf("%w: y=%d (max=%d)", ErrOutOfBounds, coord.Y, h.mapBounds.Height-1)
	}
	if coord.Z < 0 || coord.Z >= int16(h.mapBounds.Depth) {
		return fmt.Errorf("%w: z=%d (max=%d)", ErrOutOfBounds, coord.Z, h.mapBounds.Depth-1)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if coordinate existed
	if _, existed := h.hazards[coord]; existed {
		delete(h.hazards, coord)
		h.totalCount--
	}

	h.lastUpdate = time.Now()
	return nil
}

// Clear removes all hazards (thread-safe write)
func (h *HazardOverlay) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.hazards = make(map[Coordinate]HazardInfo)
	h.totalCount = 0
	h.lastUpdate = time.Now()
}

// GetLastUpdateTime returns when the overlay was last updated
func (h *HazardOverlay) GetLastUpdateTime() time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastUpdate
}
