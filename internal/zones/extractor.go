package zones

import (
	"time"

	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// ZoneExtractor queries DF API for zone data and computes zone counts for fort metrics
type ZoneExtractor struct {
	logger          *logging.Logger
	lastExtractTime time.Time
	zoneCache       map[uint32]*ZoneInfo
	enabled         bool
}

// NewZoneExtractor creates a new zone extractor
func NewZoneExtractor(logger *logging.Logger, enabled bool) *ZoneExtractor {
	return &ZoneExtractor{
		logger:    logger,
		zoneCache: make(map[uint32]*ZoneInfo),
		enabled:   enabled,
	}
}

// ExtractZones processes zone data from EntityUpdate message
func (ze *ZoneExtractor) ExtractZones(zoneData []protocol.ZoneData) ([]*ZoneInfo, error) {
	if !ze.enabled {
		return nil, nil
	}

	zones := make([]*ZoneInfo, 0, len(zoneData))
	ze.lastExtractTime = time.Now()

	for _, zd := range zoneData {
		// Convert protocol.ZoneData to zones.ZoneInfo
		zoneInfo := &ZoneInfo{
			ZoneID:     zd.ZoneID,
			ZoneType:   ze.convertZoneType(zd.ZoneType),
			Region: protocol.Region{
				X1: zd.X1, Y1: zd.Y1, Z1: zd.Z1,
				X2: zd.X2, Y2: zd.Y2, Z2: zd.Z2,
			},
			AssignedTo: zd.AssignedTo,
			SizeX:      uint16(zd.X2 - zd.X1 + 1),
			SizeY:      uint16(zd.Y2 - zd.Y1 + 1),
			CreatedAt:  time.Now(), // First time we see this zone
		}

		zones = append(zones, zoneInfo)

		// Update cache
		ze.zoneCache[zd.ZoneID] = zoneInfo
	}

	ze.logger.Debug("zone extraction complete",
		logging.Field{Key: "zone_count", Value: len(zones)})

	return zones, nil
}

// convertZoneType converts protocol zone type to internal zone type
func (ze *ZoneExtractor) convertZoneType(protoType uint8) ZoneType {
	switch protoType {
	case protocol.ZoneTypeBedroom:
		return ZoneTypeBedroom
	case protocol.ZoneTypeDining:
		return ZoneTypeDining
	case protocol.ZoneTypeDormitory:
		return ZoneTypeDormitory
	case protocol.ZoneTypeOffice:
		return ZoneTypeOffice
	case protocol.ZoneTypeBarracks:
		return ZoneTypeBarracks
	case protocol.ZoneTypeWorkshop:
		return ZoneTypeWorkshop
	case protocol.ZoneTypeStockpile:
		return ZoneTypeStockpile
	default:
		return ZoneTypeBedroom // Default fallback
	}
}

// CountByType returns zone counts by type
func (ze *ZoneExtractor) CountByType(zones []*ZoneInfo) map[ZoneType]int {
	counts := make(map[ZoneType]int)

	for _, zone := range zones {
		counts[zone.ZoneType]++
	}

	return counts
}

// GetUnassignedCount returns count of zones without owners
func (ze *ZoneExtractor) GetUnassignedCount(zones []*ZoneInfo, zoneType ZoneType) int {
	count := 0

	for _, zone := range zones {
		if zone.ZoneType == zoneType && !zone.IsAssigned() {
			count++
		}
	}

	return count
}

// IsEnabled returns true if zone extraction is configured
func (ze *ZoneExtractor) IsEnabled() bool {
	return ze.enabled
}
