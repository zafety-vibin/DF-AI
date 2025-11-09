package entities

import (
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// DwarfDetails contains extended dwarf information
type DwarfDetails struct {
	ID       uint32
	Position protocol.Coordinate

	// TODO: Extract from DFHack in future
	Name       string
	Profession string
	Skills     map[string]int
	Mood       int8 // -10 (miserable) to +10 (ecstatic)

	// Activity tracking
	CurrentTask string
	IdleTime    time.Duration
	LastSeen    time.Time
}

// Coordinate helper (compatibility with protocol)
type Coordinate struct {
	X, Y, Z int16
}

// EnhancedEntityCache tracks detailed entity information
type EnhancedEntityCache struct {
	mu sync.RWMutex

	// Basic tracking (from protocol.EntityInfo)
	entities []protocol.EntityInfo

	// Enhanced tracking (for dwarves)
	dwarves map[uint32]*DwarfDetails
	animals map[uint32]*protocol.EntityInfo
	enemies map[uint32]*protocol.EntityInfo

	// Statistics
	stats *FortStatistics
}

// FortStatistics tracks fort-level metrics
type FortStatistics struct {
	// Population
	DwarfCount  int
	AnimalCount int
	EnemyCount  int

	// Economic (TODO: Extract from DF)
	FoodStocks  int
	DrinkStocks int
	Wealth      int64

	// Alerts
	CriticalAlerts []string
}

// NewEnhancedEntityCache creates a new enhanced cache
func NewEnhancedEntityCache() *EnhancedEntityCache {
	return &EnhancedEntityCache{
		entities: []protocol.EntityInfo{},
		dwarves:  make(map[uint32]*DwarfDetails),
		animals:  make(map[uint32]*protocol.EntityInfo),
		enemies:  make(map[uint32]*protocol.EntityInfo),
		stats:    &FortStatistics{},
	}
}

// Update updates the cache from entity protocol message
func (eec *EnhancedEntityCache) Update(entities []protocol.EntityInfo) {
	eec.mu.Lock()
	defer eec.mu.Unlock()

	eec.entities = entities

	// Clear existing maps
	eec.dwarves = make(map[uint32]*DwarfDetails)
	eec.animals = make(map[uint32]*protocol.EntityInfo)
	eec.enemies = make(map[uint32]*protocol.EntityInfo)

	// Categorize
	for i := range entities {
		entity := &entities[i]

		switch entity.Type {
		case protocol.EntityTypeDwarf:
			dwarf := &DwarfDetails{
				ID:       entity.ID,
				Position: protocol.Coordinate{X: entity.X, Y: entity.Y, Z: entity.Z},
				LastSeen: time.Now(),
			}
			eec.dwarves[entity.ID] = dwarf

		case protocol.EntityTypeEnemy:
			eec.enemies[entity.ID] = entity

		case protocol.EntityTypeAnimal:
			eec.animals[entity.ID] = entity
		}
	}

	// Update statistics
	eec.updateStatistics()
}

// updateStatistics recalculates fort-level stats
func (eec *EnhancedEntityCache) updateStatistics() {
	eec.stats.DwarfCount = len(eec.dwarves)
	eec.stats.AnimalCount = len(eec.animals)
	eec.stats.EnemyCount = len(eec.enemies)

	// Generate alerts
	eec.stats.CriticalAlerts = []string{}
	if eec.stats.EnemyCount > 0 {
		eec.stats.CriticalAlerts = append(eec.stats.CriticalAlerts,
			fmt.Sprintf("ALERT: %d enemies detected!", eec.stats.EnemyCount))
	}
}

// GetDwarves returns all dwarf details
func (eec *EnhancedEntityCache) GetDwarves() map[uint32]*DwarfDetails {
	eec.mu.RLock()
	defer eec.mu.RUnlock()

	// Return copy to prevent concurrent modification
	dwarves := make(map[uint32]*DwarfDetails, len(eec.dwarves))
	for id, dwarf := range eec.dwarves {
		dwarfCopy := *dwarf
		dwarves[id] = &dwarfCopy
	}
	return dwarves
}

// GetStatistics returns current fort statistics
func (eec *EnhancedEntityCache) GetStatistics() *FortStatistics {
	eec.mu.RLock()
	defer eec.mu.RUnlock()

	// Return copy
	statsCopy := *eec.stats
	statsCopy.CriticalAlerts = append([]string{}, eec.stats.CriticalAlerts...)
	return &statsCopy
}

// GetBasicEntities returns original protocol entities
func (eec *EnhancedEntityCache) GetBasicEntities() []protocol.EntityInfo {
	eec.mu.RLock()
	defer eec.mu.RUnlock()

	// Return copy
	entities := make([]protocol.EntityInfo, len(eec.entities))
	copy(entities, eec.entities)
	return entities
}
