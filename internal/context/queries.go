package context

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/topology"
)

// QueryExecutor executes queries against game state overlays
type QueryExecutor struct {
	topoOverlay *topology.TopologyOverlay
	hazardMgr   *hazards.HazardManager
	modOverlay  *modifications.ModificationOverlay
}

// NewQueryExecutor creates a query executor with references to overlays
func NewQueryExecutor(
	topo *topology.TopologyOverlay,
	hazards *hazards.HazardManager,
	mods *modifications.ModificationOverlay,
) *QueryExecutor {
	return &QueryExecutor{
		topoOverlay: topo,
		hazardMgr:   hazards,
		modOverlay:  mods,
	}
}

// ExecuteQuery processes AI data requests and returns filtered overlays or computed results
func (q *QueryExecutor) ExecuteQuery(req QueryRequest) (*QueryResponse, error) {
	response := &QueryResponse{
		Type:    req.Type,
		Success: false,
	}

	switch req.Type {
	case QueryTopologySlice:
		data, err := q.executeTopologySlice(req.Params)
		if err != nil {
			response.ErrorMessage = err.Error()
			return response, err
		}
		response.Data = data
		response.Success = true

	case QueryHazardList:
		data, err := q.executeHazardList(req.Params)
		if err != nil {
			response.ErrorMessage = err.Error()
			return response, err
		}
		response.Data = data
		response.Success = true

	case QueryModificationsInRegion:
		data, err := q.executeModificationsInRegion(req.Params)
		if err != nil {
			response.ErrorMessage = err.Error()
			return response, err
		}
		response.Data = data
		response.Success = true

	case QueryPathfind:
		// Future extension - not implemented yet
		response.ErrorMessage = "pathfinding not yet implemented"
		return response, fmt.Errorf("pathfinding not yet implemented")

	default:
		response.ErrorMessage = fmt.Sprintf("unknown query type: %s", req.Type)
		return response, fmt.Errorf("unknown query type: %s", req.Type)
	}

	// Calculate response size
	jsonData, err := json.Marshal(response.Data)
	if err == nil {
		response.SizeBytes = uint32(len(jsonData))
	}

	return response, nil
}

// TopologySliceResult contains topology data for a Z-level slice
type TopologySliceResult struct {
	Z             int16    `json:"z"`
	Width         uint16   `json:"width"`
	Height        uint16   `json:"height"`
	OpenTileCount uint32   `json:"open_tile_count"`
	ASCIIMap      []string `json:"ascii_map,omitempty"` // Each string is a row
}

// executeTopologySlice returns bool array or ASCII for specified Z slice
func (q *QueryExecutor) executeTopologySlice(params map[string]interface{}) (interface{}, error) {
	if q.topoOverlay == nil {
		return nil, fmt.Errorf("topology overlay not available")
	}

	// Extract Z parameter
	zFloat, ok := params["z"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'z' parameter")
	}
	z := int16(zFloat)

	// Get dimensions
	width, height, depth := q.topoOverlay.GetDimensions()

	// Validate Z
	if z < 0 || z >= int16(depth) {
		return nil, fmt.Errorf("z=%d out of bounds (0-%d)", z, depth-1)
	}

	// Check if ASCII format requested
	asciiFormat := false
	if format, ok := params["format"].(string); ok && format == "ascii" {
		asciiFormat = true
	}

	result := TopologySliceResult{
		Z:      z,
		Width:  width,
		Height: height,
	}

	if asciiFormat {
		// Generate ASCII map
		asciiMap := make([]string, height)
		openCount := uint32(0)

		for y := int16(0); y < int16(height); y++ {
			row := ""
			for x := int16(0); x < int16(width); x++ {
				isOpen, err := q.topoOverlay.GetTile(x, y, z)
				if err != nil {
					row += "?"
				} else if isOpen {
					row += "."
					openCount++
				} else {
					row += "#"
				}
			}
			asciiMap[y] = row
		}

		result.ASCIIMap = asciiMap
		result.OpenTileCount = openCount
	} else {
		// Count open tiles (but don't return full array to save space)
		openCount := uint32(0)
		for y := int16(0); y < int16(height); y++ {
			for x := int16(0); x < int16(width); x++ {
				isOpen, err := q.topoOverlay.GetTile(x, y, z)
				if err == nil && isOpen {
					openCount++
				}
			}
		}
		result.OpenTileCount = openCount
	}

	return result, nil
}

// HazardListResult contains filtered hazard positions
type HazardListResult struct {
	HazardType string           `json:"hazard_type"`
	Count      uint32           `json:"count"`
	Positions  []HazardPosition `json:"positions"`
}

// executeHazardList filters hazard overlay to Z-range and hazard type
func (q *QueryExecutor) executeHazardList(params map[string]interface{}) (interface{}, error) {
	if q.hazardMgr == nil {
		return nil, fmt.Errorf("hazard manager not available")
	}

	// Extract hazard_type parameter
	hazardType, ok := params["hazard_type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'hazard_type' parameter")
	}

	// Get overlay
	overlay := q.hazardMgr.GetOverlay(hazardType)
	if overlay == nil {
		return nil, fmt.Errorf("unknown hazard type: %s", hazardType)
	}

	// Extract Z range (optional)
	var zMin, zMax int16 = -128, 127 // Default to full range

	if zMinFloat, ok := params["z_min"].(float64); ok {
		zMin = int16(zMinFloat)
	}
	if zMaxFloat, ok := params["z_max"].(float64); ok {
		zMax = int16(zMaxFloat)
	}

	// Get map dimensions for region
	width, height, _ := q.topoOverlay.GetDimensions()

	// Create region covering full XY, filtered by Z
	region := hazards.Region{
		XMin: 0, XMax: int16(width) - 1,
		YMin: 0, YMax: int16(height) - 1,
		ZMin: zMin, ZMax: zMax,
	}

	coords := overlay.GetInRegion(region)

	// Convert to HazardPosition format
	positions := make([]HazardPosition, 0, len(coords))
	for _, coord := range coords {
		info, exists := overlay.Get(coord.X, coord.Y, coord.Z)
		if !exists {
			continue
		}

		position := HazardPosition{
			X:        coord.X,
			Y:        coord.Y,
			Z:        coord.Z,
			Severity: info.Severity,
			Flags:    info.Flags,
		}
		positions = append(positions, position)
	}

	result := HazardListResult{
		HazardType: hazardType,
		Count:      uint32(len(positions)),
		Positions:  positions,
	}

	return result, nil
}

// ModificationsResult contains modifications in a region
type ModificationsResult struct {
	Region        modifications.Region `json:"region"`
	Count         uint32               `json:"count"`
	Modifications []ModificationEntry  `json:"modifications"`
}

// ModificationEntry represents a single modification
type ModificationEntry struct {
	X          int16     `json:"x"`
	Y          int16     `json:"y"`
	Z          int16     `json:"z"`
	Type       string    `json:"type"`
	DetectedAt time.Time `json:"detected_at"`
	CommandID  uint32    `json:"command_id,omitempty"`
}

// executeModificationsInRegion lists modifications in specified region
func (q *QueryExecutor) executeModificationsInRegion(params map[string]interface{}) (interface{}, error) {
	if q.modOverlay == nil {
		return nil, fmt.Errorf("modification overlay not available")
	}

	// Extract region parameters
	region := modifications.Region{}

	if xMinFloat, ok := params["x_min"].(float64); ok {
		region.XMin = int16(xMinFloat)
	} else {
		return nil, fmt.Errorf("missing 'x_min' parameter")
	}

	if yMinFloat, ok := params["y_min"].(float64); ok {
		region.YMin = int16(yMinFloat)
	} else {
		return nil, fmt.Errorf("missing 'y_min' parameter")
	}

	if zMinFloat, ok := params["z_min"].(float64); ok {
		region.ZMin = int16(zMinFloat)
	} else {
		return nil, fmt.Errorf("missing 'z_min' parameter")
	}

	if xMaxFloat, ok := params["x_max"].(float64); ok {
		region.XMax = int16(xMaxFloat)
	} else {
		return nil, fmt.Errorf("missing 'x_max' parameter")
	}

	if yMaxFloat, ok := params["y_max"].(float64); ok {
		region.YMax = int16(yMaxFloat)
	} else {
		return nil, fmt.Errorf("missing 'y_max' parameter")
	}

	if zMaxFloat, ok := params["z_max"].(float64); ok {
		region.ZMax = int16(zMaxFloat)
	} else {
		return nil, fmt.Errorf("missing 'z_max' parameter")
	}

	// Optional: filter by time (since parameter)
	since := time.Time{} // Beginning of time
	if sinceStr, ok := params["since"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	// Get modifications in region
	mods := q.modOverlay.GetModificationsInRegion(region, since)

	// Convert to ModificationEntry format
	entries := make([]ModificationEntry, 0, len(mods))
	for coord, info := range mods {
		entry := ModificationEntry{
			X:          coord.X,
			Y:          coord.Y,
			Z:          coord.Z,
			Type:       info.Type.String(),
			DetectedAt: info.DetectedAt,
			CommandID:  info.CommandID,
		}
		entries = append(entries, entry)
	}

	result := ModificationsResult{
		Region:        region,
		Count:         uint32(len(entries)),
		Modifications: entries,
	}

	return result, nil
}
