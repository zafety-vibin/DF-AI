package agents

import (
	"fmt"
	"math"

	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// GraphExecutor converts modification nodes to DFHack commands
type GraphExecutor struct {
	logger *logging.Logger
}

// NewGraphExecutor creates a new graph executor
func NewGraphExecutor(logger *logging.Logger) *GraphExecutor {
	return &GraphExecutor{
		logger: logger,
	}
}

// Execute converts arbiter decision (nodes) to DFHack commands
func (ge *GraphExecutor) Execute(decision *ArbitrationDecision) ([]protocol.CommandMessage, error) {
	commands := make([]protocol.CommandMessage, 0)

	// Execute nodes in sequence order (topologically sorted)
	for _, node := range decision.ExecutionSequence {
		nodeCommands, err := ge.convertNodeToCommands(node)
		if err != nil {
			ge.logger.Warn("failed to convert node to commands",
				logging.Field{Key: "node_id", Value: node.ID},
				logging.Field{Key: "node_type", Value: string(node.Type)},
				logging.Field{Key: "error", Value: err.Error()})
			continue
		}

		commands = append(commands, nodeCommands...)
		ge.logger.Debug("converted node to commands",
			logging.Field{Key: "node_id", Value: node.ID},
			logging.Field{Key: "node_type", Value: string(node.Type)},
			logging.Field{Key: "command_count", Value: len(nodeCommands)})
	}

	// Also execute injected synergy nodes
	for _, node := range decision.InjectedNodes {
		nodeCommands, err := ge.convertNodeToCommands(node)
		if err != nil {
			ge.logger.Warn("failed to convert injected node",
				logging.Field{Key: "node_id", Value: node.ID},
				logging.Field{Key: "error", Value: err.Error()})
			continue
		}

		commands = append(commands, nodeCommands...)
		ge.logger.Debug("converted injected node",
			logging.Field{Key: "node_id", Value: node.ID},
			logging.Field{Key: "node_type", Value: string(node.Type)})
	}

	return commands, nil
}

// convertNodeToCommands dispatches to appropriate handler based on node type
func (ge *GraphExecutor) convertNodeToCommands(node ModificationNode) ([]protocol.CommandMessage, error) {
	switch node.Type {
	case NodeTypeBedroom:
		return ge.handleBedroomCluster(node)
	case NodeTypeMiningShaft:
		return ge.handleMiningShaft(node)
	case NodeTypeFarmPlot, NodeTypeGatherZone:
		return ge.handleFarmPlot(node)
	case NodeTypeCorridor:
		return ge.handleCorridor(node)
	case NodeTypeWorkshop, NodeTypeProductionArea:
		return ge.handleWorkshop(node)
	case NodeTypeSealEntrance, NodeTypeDefensive:
		return ge.handleDefensive(node)
	case NodeTypeExploratory:
		return ge.handleExploratory(node)
	case NodeTypeStairCluster:
		return ge.handleStairCluster(node)
	default:
		return nil, fmt.Errorf("unsupported node type: %s", node.Type)
	}
}

// convertRegion converts modifications.Region to protocol.Region
func convertRegion(r modifications.Region) protocol.Region {
	return protocol.Region{
		X1: r.XMin,
		Y1: r.YMin,
		Z1: r.ZMin,
		X2: r.XMax,
		Y2: r.YMax,
		Z2: r.ZMax,
	}
}

// handleBedroomCluster generates DIG commands for 3×3 bedroom grid
func (ge *GraphExecutor) handleBedroomCluster(node ModificationNode) ([]protocol.CommandMessage, error) {
	// For now, dig the entire region as bedrooms
	// TODO: Generate actual 3×3 grid pattern
	cmd := protocol.CommandMessage{
		CommandType: protocol.CommandTypeDig,
		DigType:     protocol.DigTypeDefault,
		Region:      convertRegion(node.Region),
	}

	return []protocol.CommandMessage{cmd}, nil
}

// handleMiningShaft generates DIG commands with DownStair for vertical shaft
func (ge *GraphExecutor) handleMiningShaft(node ModificationNode) ([]protocol.CommandMessage, error) {
	// Dig vertical shaft using DownStair dig type
	cmd := protocol.CommandMessage{
		CommandType: protocol.CommandTypeDig,
		DigType:     protocol.DigTypeDownStair,
		Region:      convertRegion(node.Region),
	}

	return []protocol.CommandMessage{cmd}, nil
}

// handleFarmPlot generates DIG commands for farm area (flat excavation)
func (ge *GraphExecutor) handleFarmPlot(node ModificationNode) ([]protocol.CommandMessage, error) {
	// Dig flat area for farming
	cmd := protocol.CommandMessage{
		CommandType: protocol.CommandTypeDig,
		DigType:     protocol.DigTypeDefault,
		Region:      convertRegion(node.Region),
	}

	return []protocol.CommandMessage{cmd}, nil
}

// handleCorridor generates DIG commands for Manhattan path corridor
func (ge *GraphExecutor) handleCorridor(node ModificationNode) ([]protocol.CommandMessage, error) {
	// For now, dig entire region as corridor
	// TODO: Calculate actual Manhattan path between connected nodes
	cmd := protocol.CommandMessage{
		CommandType: protocol.CommandTypeDig,
		DigType:     protocol.DigTypeDefault,
		Region:      convertRegion(node.Region),
	}

	return []protocol.CommandMessage{cmd}, nil
}

// handleWorkshop generates DIG commands for workshop area
func (ge *GraphExecutor) handleWorkshop(node ModificationNode) ([]protocol.CommandMessage, error) {
	// Dig workshop area
	cmd := protocol.CommandMessage{
		CommandType: protocol.CommandTypeDig,
		DigType:     protocol.DigTypeDefault,
		Region:      convertRegion(node.Region),
	}

	return []protocol.CommandMessage{cmd}, nil
}

// handleDefensive generates BUILD commands for walls (placeholder)
func (ge *GraphExecutor) handleDefensive(node ModificationNode) ([]protocol.CommandMessage, error) {
	// For now, just dig the area (actual wall building needs BUILD command support)
	// TODO: Implement BUILD command in protocol
	cmd := protocol.CommandMessage{
		CommandType: protocol.CommandTypeDig,
		DigType:     protocol.DigTypeDefault,
		Region:      convertRegion(node.Region),
	}

	return []protocol.CommandMessage{cmd}, nil
}

// handleExploratory generates DIG commands for exploratory tunnel
func (ge *GraphExecutor) handleExploratory(node ModificationNode) ([]protocol.CommandMessage, error) {
	// Dig exploratory tunnel
	cmd := protocol.CommandMessage{
		CommandType: protocol.CommandTypeDig,
		DigType:     protocol.DigTypeDefault,
		Region:      convertRegion(node.Region),
	}

	return []protocol.CommandMessage{cmd}, nil
}

// handleStairCluster generates UPDOWNSTAIR commands for vertical connection
func (ge *GraphExecutor) handleStairCluster(node ModificationNode) ([]protocol.CommandMessage, error) {
	// Dig staircase using UpDownStair type
	cmd := protocol.CommandMessage{
		CommandType: protocol.CommandTypeDig,
		DigType:     protocol.DigTypeUpDownStair,
		Region:      convertRegion(node.Region),
	}

	return []protocol.CommandMessage{cmd}, nil
}

// calculateManhattanPath computes Manhattan path between two points
// Returns list of intermediate coordinates
func calculateManhattanPath(start, end modifications.Coordinate) []modifications.Coordinate {
	path := make([]modifications.Coordinate, 0)

	// Move horizontally first (X direction)
	x := start.X
	for x != end.X {
		if x < end.X {
			x++
		} else {
			x--
		}
		path = append(path, modifications.Coordinate{X: x, Y: start.Y, Z: start.Z})
	}

	// Then move vertically (Y direction)
	y := start.Y
	for y != end.Y {
		if y < end.Y {
			y++
		} else {
			y--
		}
		path = append(path, modifications.Coordinate{X: end.X, Y: y, Z: start.Z})
	}

	return path
}

// calculateDistance computes Euclidean distance between two regions
func calculateDistance(a, b modifications.Region) float64 {
	// Use region centers
	ax := float64(a.XMin+a.XMax) / 2.0
	ay := float64(a.YMin+a.YMax) / 2.0
	az := float64(a.ZMin+a.ZMax) / 2.0

	bx := float64(b.XMin+b.XMax) / 2.0
	by := float64(b.YMin+b.YMax) / 2.0
	bz := float64(b.ZMin+b.ZMax) / 2.0

	dx := ax - bx
	dy := ay - by
	dz := az - bz

	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}
