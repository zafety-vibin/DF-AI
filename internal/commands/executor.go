package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// DFHackClient interface for sending commands
type DFHackClient interface {
	SendCommand(cmd *protocol.CommandMessage) error
	SubscribeCommandAcks() <-chan *protocol.CommandAckMessage
	IsConnected() bool
}

// CommandExecutor handles sending commands to DFHack and tracking responses
type CommandExecutor struct {
	logger  *logging.Logger
	client  DFHackClient
	tracker *CommandTracker
	timeout time.Duration
	stopCh  chan struct{}
}

// NewCommandExecutor creates a new command executor
func NewCommandExecutor(logger *logging.Logger, client DFHackClient, timeout time.Duration) *CommandExecutor {
	if timeout == 0 {
		timeout = 5 * time.Second // Default 5 second timeout
	}

	e := &CommandExecutor{
		logger:  logger,
		client:  client,
		tracker: NewCommandTracker(timeout),
		timeout: timeout,
		stopCh:  make(chan struct{}),
	}

	// Start ACK handler
	go e.handleAcks()

	// Start timeout checker
	go e.checkTimeouts()

	return e
}

// SendDigCommand sends a dig designation command on a single Z-level.
// digType: DigTypeDefault (standard), DigTypeUpDownStair, DigTypeChannel, etc.
func (e *CommandExecutor) SendDigCommand(digType uint8, x1, y1, z int16, x2, y2 int16) (*CommandResult, error) {
	return e.SendDigRegion(digType, x1, y1, z, x2, y2, z)
}

// SendDigRegion sends a dig designation command across a 3D rectangle
// (Z1 may differ from Z2). The plugin handles per-tile designation
// natively. Stair-type digTypes across Z1 != Z2 produce a stair shaft:
// UpStair on the bottom Z, DownStair on the top, UpDownStair on middles.
//
// Callers naturally describe shafts top-down (z_start=surface,
// z_end=deep); the wire format and Validate() want ascending Z. Normalize
// here — stair orientation is derived from Z magnitude, not argument
// order, so the swap is semantically free.
func (e *CommandExecutor) SendDigRegion(digType uint8, x1, y1, z1, x2, y2, z2 int16) (*CommandResult, error) {
	if z1 > z2 {
		z1, z2 = z2, z1
	}
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeDig,
		DigType:     digType,
		Region: protocol.Region{
			X1: x1, Y1: y1, Z1: z1,
			X2: x2, Y2: y2, Z2: z2,
		},
	}
	return e.SendCommand(cmd)
}

// SendBuildCommand sends a build designation command with no material
// class preference (DF's job system picks any suitable item).
func (e *CommandExecutor) SendBuildCommand(x, y, z int16, buildType uint8) (*CommandResult, error) {
	return e.SendBuildCommandWithMaterial(x, y, z, buildType, protocol.MaterialClassAny)
}

// SendBuildCommandWithMaterial sends a build designation command with an
// explicit material class constraint (protocol.MaterialClass*). The class
// narrows what DF's job system may claim (wood/boulders/blocks); DF still
// picks the specific item within the class. The wire payload always
// carries the material byte — MaterialClassAny means "no constraint".
func (e *CommandExecutor) SendBuildCommandWithMaterial(x, y, z int16, buildType, material uint8) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeBuild,
		Build: protocol.BuildDesignation{
			X:         x,
			Y:         y,
			Z:         z,
			BuildType: buildType,
			Material:  material,
		},
	}

	return e.SendCommand(cmd)
}

// SendCancelCommand sends a cancel designation command
func (e *CommandExecutor) SendCancelCommand(x1, y1, z int16, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeCancel,
		Region: protocol.Region{
			X1: x1, Y1: y1, Z1: z,
			X2: x2, Y2: y2, Z2: z,
		},
	}

	return e.SendCommand(cmd)
}

// SendChopCommand sends a tree chopping designation command
func (e *CommandExecutor) SendChopCommand(x1, y1, z int16, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeChop,
		Region: protocol.Region{
			X1: x1, Y1: y1, Z1: z,
			X2: x2, Y2: y2, Z2: z,
		},
	}

	return e.SendCommand(cmd)
}

// SendGatherCommand sends a plant gathering designation command
func (e *CommandExecutor) SendGatherCommand(x1, y1, z int16, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeGather,
		Region: protocol.Region{
			X1: x1, Y1: y1, Z1: z,
			X2: x2, Y2: y2, Z2: z,
		},
	}

	return e.SendCommand(cmd)
}

// SendZoneCommand designates a region as a civzone (bedroom, dining, etc.).
func (e *CommandExecutor) SendZoneCommand(zoneType uint8, x1, y1, z, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeZone,
		Zone: protocol.ZoneDesignation{
			X1: x1, Y1: y1, Z: z,
			X2: x2, Y2: y2,
			ZoneType: zoneType,
		},
	}
	return e.SendCommand(cmd)
}

// SendUnsuspendCommand clears the suspend flag on jobs at the given tile.
// Used to resume an auto-suspended construction once the underlying
// blocker has been cleared.
func (e *CommandExecutor) SendUnsuspendCommand(x, y, z int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeUnsuspend,
		Unsuspend: protocol.UnsuspendDesignation{
			X: x, Y: y, Z: z,
		},
	}
	return e.SendCommand(cmd)
}

// SendRemoveBuilding marks the building occupying (x,y,z) for
// deconstruction. Any tile of a multi-tile building's footprint works;
// dwarves do the actual teardown over game time.
func (e *CommandExecutor) SendRemoveBuilding(x, y, z int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeRemoveBuilding,
		Remove: protocol.RemoveBuildingDesignation{
			X: x, Y: y, Z: z,
		},
	}
	return e.SendCommand(cmd)
}

// SendWorkOrderCommand adds a manager work order to produce N items of
// the given type. The manager dispatches to whichever workshop can fulfill
// the order, drawing reagents from stockpiles automatically.
func (e *CommandExecutor) SendWorkOrderCommand(orderType uint8, quantity uint16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeWorkOrder,
		Order: protocol.WorkOrderDesignation{
			OrderType: orderType,
			Quantity:  quantity,
		},
	}
	return e.SendCommand(cmd)
}

// SendStockpileCommand designates a rectangular region as a stockpile
// accepting items in the named groups. groupMask is a bitfield of
// protocol.StockpileGroup* constants (use StockpileGroupAll for an
// "everything" stockpile).
func (e *CommandExecutor) SendStockpileCommand(x1, y1, z, x2, y2 int16, groupMask uint32) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeStockpile,
		Stockpile: protocol.StockpileDesignation{
			X1: x1, Y1: y1, Z: z,
			X2: x2, Y2: y2,
			GroupMask: groupMask,
		},
	}
	return e.SendCommand(cmd)
}

// SendSmoothCommand designates a rectangular region for smoothing or
// engraving. smoothType is protocol.SmoothTypeSmooth (1) or
// SmoothTypeEngrave (2). Only natural stone walls/floors will be acted on
// by DF; soil/sand and constructed walls are silently skipped.
func (e *CommandExecutor) SendSmoothCommand(smoothType uint8, x1, y1, z, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeSmooth,
		Smooth: protocol.SmoothDesignation{
			X1: x1, Y1: y1, Z: z,
			X2: x2, Y2: y2,
			SmoothType: smoothType,
		},
	}
	return e.SendCommand(cmd)
}

// SendPauseCommand pauses (true) or unpauses (false) the DF simulation.
func (e *CommandExecutor) SendPauseCommand(pause bool) (*CommandResult, error) {
	mode := protocol.PauseModeUnpause
	if pause {
		mode = protocol.PauseModePause
	}
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypePause,
		Pause:       protocol.PauseControl{Mode: mode},
	}
	return e.SendCommand(cmd)
}

// SendStepCommand unpauses DF and asks the plugin to re-pause after N
// ticks. The ACK arrives immediately ("step started"); poll the
// sim_status query for completion.
func (e *CommandExecutor) SendStepCommand(ticks uint32) (*CommandResult, error) {
	if ticks == 0 {
		return nil, fmt.Errorf("step ticks must be > 0")
	}
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypePause,
		Pause:       protocol.PauseControl{Mode: protocol.PauseModeStep, Ticks: ticks},
	}
	return e.SendCommand(cmd)
}

// SendCommand sends a command and waits for acknowledgment
func (e *CommandExecutor) SendCommand(cmdMsg *protocol.CommandMessage) (*CommandResult, error) {
	// Check if connected
	if !e.client.IsConnected() {
		return nil, fmt.Errorf("not connected to DFHack")
	}

	// Create command tracking entry
	cmd := &Command{
		ID:         cmdMsg.CommandID,
		Type:       cmdMsg.CommandType,
		Message:    cmdMsg,
		Status:     CommandStatusPending,
		SentAt:     time.Now(),
		TimeoutAt:  time.Now().Add(e.timeout),
		ResultChan: make(chan *CommandResult, 1),
	}

	// Add to tracker
	e.tracker.AddCommand(cmd)

	e.logger.Debug("sending command",
		logging.Field{Key: "command_id", Value: cmd.ID},
		logging.Field{Key: "command_type", Value: cmd.Type})

	// Send command to DFHack
	if err := e.client.SendCommand(cmdMsg); err != nil {
		cmd.Status = CommandStatusFailed
		e.tracker.RemoveCommand(cmd.ID)
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Mark as sent
	cmd.Status = CommandStatusSent
	cmd.SentAt = time.Now()
	cmd.TimeoutAt = time.Now().Add(e.timeout)

	// Wait for result with timeout
	select {
	case result := <-cmd.ResultChan:
		e.logger.Info("command completed",
			logging.Field{Key: "command_id", Value: cmd.ID},
			logging.Field{Key: "success", Value: result.Success},
			logging.Field{Key: "duration_ms", Value: result.Duration.Milliseconds()})
		return result, nil
	case <-time.After(e.timeout + time.Second):
		// Extra second grace period
		e.logger.Warn("command timeout",
			logging.Field{Key: "command_id", Value: cmd.ID})
		return &CommandResult{
			Success:  false,
			Status:   protocol.AckStatusFailure,
			ErrorMsg: "timeout waiting for acknowledgment",
			Duration: time.Since(cmd.SentAt),
		}, nil
	}
}

// WaitForAck waits for acknowledgment of a specific command
func (e *CommandExecutor) WaitForAck(commandID uint32, timeout time.Duration) (*CommandResult, error) {
	cmd, ok := e.tracker.GetCommand(commandID)
	if !ok {
		return nil, fmt.Errorf("command %d not found", commandID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case result := <-cmd.ResultChan:
		return result, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout waiting for acknowledgment")
	}
}

// HandleAck processes an acknowledgment from DFHack
func (e *CommandExecutor) HandleAck(ack *protocol.CommandAckMessage) error {
	e.logger.Debug("received command ack",
		logging.Field{Key: "command_id", Value: ack.CommandID},
		logging.Field{Key: "status", Value: ack.Status})

	if err := e.tracker.MarkAcked(ack.CommandID, ack); err != nil {
		e.logger.Warn("failed to mark command as acked",
			logging.Field{Key: "command_id", Value: ack.CommandID},
			logging.Field{Key: "error", Value: err.Error()})
		return err
	}

	return nil
}

// handleAcks listens for ACK messages from DFHack
func (e *CommandExecutor) handleAcks() {
	ackCh := e.client.SubscribeCommandAcks()

	for {
		select {
		case <-e.stopCh:
			return
		case ack, ok := <-ackCh:
			if !ok {
				e.logger.Warn("ACK channel closed")
				return
			}
			e.HandleAck(ack)
		}
	}
}

// checkTimeouts periodically checks for timed-out commands
func (e *CommandExecutor) checkTimeouts() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			timeoutCount := e.tracker.CheckTimeouts()
			if timeoutCount > 0 {
				e.logger.Warn("commands timed out",
					logging.Field{Key: "count", Value: timeoutCount})
			}
		}
	}
}

// GetStats returns statistics about command execution
func (e *CommandExecutor) GetStats() (total, pending, acked, timeout, failed int) {
	return e.tracker.GetStats()
}

// Stop stops the command executor
func (e *CommandExecutor) Stop() {
	close(e.stopCh)
	e.tracker.Stop()
}
