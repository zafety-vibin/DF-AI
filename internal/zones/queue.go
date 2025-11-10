package zones

import (
	"time"

	"github.com/yourusername/df-ai/internal/protocol"
)

// QueueStatus represents the current state of a queued zone
type QueueStatus int

const (
	QueueStatusWaitingForDig QueueStatus = iota
	QueueStatusRetrying
	QueueStatusCompleted
	QueueStatusFailed
)

// QueuedZone represents a single queued zone command with retry metadata
type QueuedZone struct {
	ZoneType       protocol.ZoneType // Type of zone to create
	Region         protocol.Region   // Target coordinates
	CommandID      uint32            // Associated dig command ID
	RetryCount     int               // How many retries attempted (0-3)
	MaxRetries     int               // Retry limit (default: 3)
	TimeoutCycles  int               // Cycles remaining before timeout (1-10)
	Status         QueueStatus       // Current state
	CreatedAt      time.Time         // When zone was queued
	LastRetryAt    time.Time         // Last retry attempt timestamp (may be zero)
}

// ShouldRetry returns true if retry conditions met
func (qz *QueuedZone) ShouldRetry() bool {
	return qz.RetryCount < qz.MaxRetries && qz.TimeoutCycles > 0
}

// IncrementRetry updates retry counter and timestamp
func (qz *QueuedZone) IncrementRetry() {
	qz.RetryCount++
	qz.LastRetryAt = time.Now()
}

// DecrementTimeout reduces timeout counter
func (qz *QueuedZone) DecrementTimeout() {
	if qz.TimeoutCycles > 0 {
		qz.TimeoutCycles--
	}
}

// IsTimedOut returns true if timeout reached
func (qz *QueuedZone) IsTimedOut() bool {
	return qz.TimeoutCycles <= 0
}
