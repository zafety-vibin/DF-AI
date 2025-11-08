package commands

import (
	"time"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// Command represents a pending command sent to DFHack
type Command struct {
	ID         uint32                      // Unique command ID
	Type       uint8                       // Command type (dig/build/cancel)
	Message    *protocol.CommandMessage    // The command message
	Status     CommandStatus               // Current status
	Response   *protocol.CommandAckMessage // Response from DFHack (when acked)
	SentAt     time.Time                   // When command was sent
	AckedAt    time.Time                   // When ACK was received
	TimeoutAt  time.Time                   // When command times out
	ResultChan chan *CommandResult         // Channel for command result (buffered size 1)
}

// CommandStatus represents the lifecycle state of a command
type CommandStatus int

const (
	CommandStatusPending CommandStatus = iota // Created but not sent
	CommandStatusSent                         // Sent to DFHack, waiting for ACK
	CommandStatusAcked                        // ACK received
	CommandStatusTimeout                      // Timeout waiting for ACK
	CommandStatusFailed                       // Send failed
)

// CommandResult contains the result of a command execution
type CommandResult struct {
	Success  bool                        // Whether command succeeded
	Status   uint8                       // ACK status from DFHack
	ErrorMsg string                      // Error message if failed
	Response *protocol.CommandAckMessage // Full response message
	Duration time.Duration               // Time from send to ACK
}

// TaskStatus represents the execution progress of a task-level command
type TaskStatus int

const (
	TaskStatusNotStarted TaskStatus = iota // Not yet started
	TaskStatusInProgress                   // Execution in progress
	TaskStatusCompleted                    // Fully completed
	TaskStatusStalled                      // Stalled (no progress detected)
	TaskStatusFailed                       // Failed to complete
)

// String returns the string representation of TaskStatus
func (ts TaskStatus) String() string {
	switch ts {
	case TaskStatusNotStarted:
		return "not_started"
	case TaskStatusInProgress:
		return "in_progress"
	case TaskStatusCompleted:
		return "completed"
	case TaskStatusStalled:
		return "stalled"
	case TaskStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// PendingCommand represents a command with progress tracking
type PendingCommand struct {
	Command          *Command   // Underlying command
	TaskStatus       TaskStatus // Task execution status
	ExpectedTiles    int        // Expected number of tiles to modify
	CompletedTiles   int        // Number of tiles completed
	LastProgressTime time.Time  // Last time progress was detected
	StartTime        time.Time  // When task started
	Description      string     // Human-readable task description
}

// IsComplete returns true if the command has reached a terminal state
func (c *Command) IsComplete() bool {
	return c.Status == CommandStatusAcked ||
		c.Status == CommandStatusTimeout ||
		c.Status == CommandStatusFailed
}

// IsPending returns true if the command is waiting for acknowledgment
func (c *Command) IsPending() bool {
	return c.Status == CommandStatusSent
}
