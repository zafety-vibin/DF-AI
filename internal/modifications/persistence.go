package modifications

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PersistentModifications adds save/load capability to modification overlay
type PersistentModifications struct {
	*ModificationOverlay

	savePath         string
	autoSaveInterval time.Duration
	lastSave         time.Time
	enabled          bool
}

// SaveData represents persisted modification state
type SaveData struct {
	Version       string                      `json:"version"`
	SavedAt       time.Time                   `json:"saved_at"`
	Modifications map[string]ModificationInfo `json:"modifications"` // Serialized coordinates as "X,Y,Z"
	Bounds        Region                      `json:"bounds"`
	TotalCount    int                         `json:"total_count"`
}

// NewPersistentModifications creates a persistent modification overlay
func NewPersistentModifications(mapBounds Bounds, savePath string, autoSaveInterval time.Duration, enabled bool) *PersistentModifications {
	return &PersistentModifications{
		ModificationOverlay: NewModificationOverlay(mapBounds),
		savePath:            savePath,
		autoSaveInterval:    autoSaveInterval,
		lastSave:            time.Now(),
		enabled:             enabled,
	}
}

// Save persists modifications to disk
func (pm *PersistentModifications) Save(fortName string) error {
	if !pm.enabled {
		return nil // Persistence disabled
	}

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Create save directory if needed
	saveDir := filepath.Join(pm.savePath, fortName)
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return fmt.Errorf("failed to create save directory: %w", err)
	}

	// Get all modifications (need to add GetAll() method to ModificationOverlay)
	allMods := pm.GetAll()

	// Serialize modifications
	serialized := make(map[string]ModificationInfo)
	for coord, info := range allMods {
		key := fmt.Sprintf("%d,%d,%d", coord.X, coord.Y, coord.Z)
		serialized[key] = info
	}

	data := &SaveData{
		Version:       "1.0",
		SavedAt:       time.Now(),
		Modifications: serialized,
		Bounds:        pm.GetBounds(),
		TotalCount:    int(pm.GetCount()),
	}

	// Write to file
	filePath := filepath.Join(saveDir, "modifications.json")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create save file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode save data: %w", err)
	}

	pm.lastSave = time.Now()
	return nil
}

// Load restores modifications from disk
func (pm *PersistentModifications) Load(fortName string) error {
	if !pm.enabled {
		return nil // Persistence disabled
	}

	filePath := filepath.Join(pm.savePath, fortName, "modifications.json")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // No saved state, start fresh
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open save file: %w", err)
	}
	defer file.Close()

	var data SaveData
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode save data: %w", err)
	}

	// Restore modifications using public Add method
	for coordStr, info := range data.Modifications {
		var x, y, z int16
		fmt.Sscanf(coordStr, "%d,%d,%d", &x, &y, &z)
		coord := Coordinate{X: x, Y: y, Z: z}
		pm.Add(coord, info)
	}

	return nil
}

// AutoSave saves if interval has elapsed
func (pm *PersistentModifications) AutoSave(fortName string) error {
	if !pm.enabled {
		return nil
	}

	if time.Since(pm.lastSave) < pm.autoSaveInterval {
		return nil // Not yet time
	}

	return pm.Save(fortName)
}

// ShouldAutoSave checks if autosave interval has elapsed
func (pm *PersistentModifications) ShouldAutoSave() bool {
	if !pm.enabled {
		return false
	}
	return time.Since(pm.lastSave) >= pm.autoSaveInterval
}
