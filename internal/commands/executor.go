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

// SendDigCommand sends a dig designation command
func (e *CommandExecutor) SendDigCommand(x1, y1, z int16, x2, y2 int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeDig,
		Region: protocol.Region{
			X1: x1, Y1: y1, Z1: z,
			X2: x2, Y2: y2, Z2: z,
		},
	}

	return e.SendCommand(cmd)
}

// SendBuildCommand sends a build designation command
func (e *CommandExecutor) SendBuildCommand(x, y, z int16, buildType uint8) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeBuild,
		Build: protocol.BuildDesignation{
			X:         x,
			Y:         y,
			Z:         z,
			BuildType: buildType,
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
