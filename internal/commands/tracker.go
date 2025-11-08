package commands

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// CommandTracker manages pending commands and their lifecycle
type CommandTracker struct {
	mu            sync.RWMutex
	commands      map[uint32]*Command // Map of command ID -> command
	nextID        uint32              // Next command ID to allocate
	timeout       time.Duration       // Default timeout for commands
	cleanupTicker *time.Ticker        // Periodic cleanup of completed commands
	stopCleanup   chan struct{}       // Signal to stop cleanup goroutine
	wg            sync.WaitGroup      // Wait group for cleanup goroutine
}

// NewCommandTracker creates a new command tracker
func NewCommandTracker(timeout time.Duration) *CommandTracker {
	t := &CommandTracker{
		commands:      make(map[uint32]*Command),
		nextID:        1,
		timeout:       timeout,
		cleanupTicker: time.NewTicker(30 * time.Second), // Clean up every 30 seconds
		stopCleanup:   make(chan struct{}),
	}

	// Start cleanup goroutine
	t.wg.Add(1)
	go t.cleanupLoop()

	return t
}

// GenerateCommandID generates a unique command ID
func (t *CommandTracker) GenerateCommandID() uint32 {
	return atomic.AddUint32(&t.nextID, 1)
}

// AddCommand registers a new command
func (t *CommandTracker) AddCommand(cmd *Command) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.commands[cmd.ID] = cmd
}

// GetCommand retrieves a command by ID
func (t *CommandTracker) GetCommand(id uint32) (*Command, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	cmd, ok := t.commands[id]
	return cmd, ok
}

// UpdateStatus updates the status of a command
func (t *CommandTracker) UpdateStatus(id uint32, status CommandStatus) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	cmd, ok := t.commands[id]
	if !ok {
		return fmt.Errorf("command %d not found", id)
	}

	cmd.Status = status
	return nil
}

// MarkAcked marks a command as acknowledged with the response
func (t *CommandTracker) MarkAcked(id uint32, ack *protocol.CommandAckMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	cmd, ok := t.commands[id]
	if !ok {
		return fmt.Errorf("command %d not found", id)
	}

	cmd.Status = CommandStatusAcked
	cmd.Response = ack
	cmd.AckedAt = time.Now()

	// Send result to result channel (non-blocking)
	result := &CommandResult{
		Success:  ack.Status == protocol.AckStatusSuccess,
		Status:   ack.Status,
		ErrorMsg: ack.ErrorMsg,
		Response: ack,
		Duration: cmd.AckedAt.Sub(cmd.SentAt),
	}

	select {
	case cmd.ResultChan <- result:
	default:
		// Channel full or closed, ignore
	}

	return nil
}

// GetPendingCommands returns all commands waiting for acknowledgment
func (t *CommandTracker) GetPendingCommands() []*Command {
	t.mu.RLock()
	defer t.mu.RUnlock()

	pending := make([]*Command, 0)
	for _, cmd := range t.commands {
		if cmd.IsPending() {
			pending = append(pending, cmd)
		}
	}
	return pending
}

// CheckTimeouts checks for timed-out commands and marks them
func (t *CommandTracker) CheckTimeouts() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	timeoutCount := 0

	for _, cmd := range t.commands {
		if cmd.IsPending() && now.After(cmd.TimeoutAt) {
			cmd.Status = CommandStatusTimeout

			// Send timeout result to result channel
			result := &CommandResult{
				Success:  false,
				Status:   protocol.AckStatusFailure,
				ErrorMsg: "timeout waiting for acknowledgment",
				Duration: now.Sub(cmd.SentAt),
			}

			select {
			case cmd.ResultChan <- result:
			default:
				// Channel full or closed, ignore
			}

			timeoutCount++
		}
	}

	return timeoutCount
}

// RemoveCommand removes a command from tracking
func (t *CommandTracker) RemoveCommand(id uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.commands, id)
}

// GetStats returns statistics about tracked commands
func (t *CommandTracker) GetStats() (total, pending, acked, timeout, failed int) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	total = len(t.commands)
	for _, cmd := range t.commands {
		switch cmd.Status {
		case CommandStatusSent:
			pending++
		case CommandStatusAcked:
			acked++
		case CommandStatusTimeout:
			timeout++
		case CommandStatusFailed:
			failed++
		}
	}
	return
}

// cleanupLoop periodically removes completed commands older than 5 minutes
func (t *CommandTracker) cleanupLoop() {
	defer t.wg.Done()

	for {
		select {
		case <-t.stopCleanup:
			return
		case <-t.cleanupTicker.C:
			t.cleanupOldCommands(5 * time.Minute)
		}
	}
}

// cleanupOldCommands removes completed commands older than the specified age
func (t *CommandTracker) cleanupOldCommands(maxAge time.Duration) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	removed := 0

	for id, cmd := range t.commands {
		if cmd.IsComplete() {
			// Determine when command completed
			completedAt := cmd.SentAt
			if !cmd.AckedAt.IsZero() {
				completedAt = cmd.AckedAt
			}

			if now.Sub(completedAt) > maxAge {
				delete(t.commands, id)
				removed++
			}
		}
	}

	return removed
}

// Stop stops the command tracker and cleanup goroutine
func (t *CommandTracker) Stop() {
	t.cleanupTicker.Stop()
	close(t.stopCleanup)
	t.wg.Wait()
}

// CheckProgress checks the progress of a pending command by comparing expected vs completed tiles
func (t *CommandTracker) CheckProgress(pc *PendingCommand, currentCompletedTiles int) TaskStatus {
	if pc == nil || pc.Command == nil {
		return TaskStatusFailed
	}

	// Update completed tiles
	pc.CompletedTiles = currentCompletedTiles

	// Check if task is complete
	if pc.CompletedTiles >= pc.ExpectedTiles {
		pc.TaskStatus = TaskStatusCompleted
		pc.LastProgressTime = time.Now()
		return TaskStatusCompleted
	}

	// Check for progress
	now := time.Now()
	if pc.CompletedTiles > 0 {
		// Progress detected
		if pc.TaskStatus == TaskStatusNotStarted {
			pc.TaskStatus = TaskStatusInProgress
			pc.StartTime = now
		}
		pc.LastProgressTime = now
		return TaskStatusInProgress
	}

	// Check for stall (no progress for 60 seconds)
	if pc.TaskStatus == TaskStatusInProgress {
		if now.Sub(pc.LastProgressTime) > 60*time.Second {
			pc.TaskStatus = TaskStatusStalled
			return TaskStatusStalled
		}
	}

	return pc.TaskStatus
}

// CalculateExpectedTiles calculates the expected number of tiles from a region
func CalculateExpectedTiles(region protocol.Region) int {
	width := int(region.X2 - region.X1 + 1)
	height := int(region.Y2 - region.Y1 + 1)
	depth := int(region.Z2 - region.Z1 + 1)
	return width * height * depth
}
