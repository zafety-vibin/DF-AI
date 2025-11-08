package autonomous

import (
	"context"
	"time"
)

// Timer manages the timing of autonomous decision cycles
// Supports both periodic and immediate triggers
type Timer struct {
	interval    time.Duration
	ticker      *time.Ticker
	immediateCh chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewTimer creates a new timer with the specified interval
func NewTimer(interval time.Duration) *Timer {
	if interval <= 0 {
		interval = 100 * time.Second // Default to 100 seconds
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Timer{
		interval:    interval,
		ticker:      time.NewTicker(interval),
		immediateCh: make(chan struct{}, 1), // Buffered to prevent blocking
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Wait blocks until the next cycle should run
// Returns true if a cycle should run, false if timer was stopped
func (t *Timer) Wait() bool {
	select {
	case <-t.ctx.Done():
		return false // Timer stopped
	case <-t.ticker.C:
		return true // Regular interval elapsed
	case <-t.immediateCh:
		return true // Immediate trigger requested
	}
}

// TriggerImmediate requests an immediate cycle (non-blocking)
// Safe to call from multiple goroutines
func (t *Timer) TriggerImmediate() {
	select {
	case t.immediateCh <- struct{}{}:
		// Trigger sent
	default:
		// Channel full (trigger already pending), ignore
	}
}

// SetInterval updates the timer interval
// Takes effect on the next cycle
func (t *Timer) SetInterval(interval time.Duration) {
	if interval <= 0 {
		return // Ignore invalid intervals
	}

	t.ticker.Stop()
	t.interval = interval
	t.ticker = time.NewTicker(interval)
}

// GetInterval returns the current interval
func (t *Timer) GetInterval() time.Duration {
	return t.interval
}

// Stop stops the timer
func (t *Timer) Stop() {
	t.cancel()
	t.ticker.Stop()
}
