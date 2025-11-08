package commands

import (
	"fmt"
	"time"
)

// Feedback contains analysis of command execution outcomes
type Feedback struct {
	TaskDescription string        // Human-readable task description
	Status          TaskStatus    // Current task status
	CompletionPct   float64       // Percentage complete (0-100)
	Message         string        // Feedback message for LLM
	Recommendation  string        // Suggested next action
	ElapsedTime     time.Duration // Time since task started
}

// FeedbackGenerator generates feedback messages for command execution
type FeedbackGenerator struct{}

// NewFeedbackGenerator creates a new feedback generator
func NewFeedbackGenerator() *FeedbackGenerator {
	return &FeedbackGenerator{}
}

// GenerateFeedback analyzes a pending command and generates feedback
func (fg *FeedbackGenerator) GenerateFeedback(pc *PendingCommand) *Feedback {
	if pc == nil {
		return &Feedback{
			TaskDescription: "Unknown task",
			Status:          TaskStatusFailed,
			CompletionPct:   0,
			Message:         "No task information available",
			Recommendation:  "Task tracking error - please retry",
		}
	}

	// Calculate completion percentage
	completionPct := 0.0
	if pc.ExpectedTiles > 0 {
		completionPct = float64(pc.CompletedTiles) / float64(pc.ExpectedTiles) * 100.0
	}

	// Calculate elapsed time
	elapsed := time.Duration(0)
	if !pc.StartTime.IsZero() {
		elapsed = time.Since(pc.StartTime)
	}

	feedback := &Feedback{
		TaskDescription: pc.Description,
		Status:          pc.TaskStatus,
		CompletionPct:   completionPct,
		ElapsedTime:     elapsed,
	}

	// Generate status-specific feedback
	switch pc.TaskStatus {
	case TaskStatusCompleted:
		feedback.Message = fg.generateSuccessMessage(pc, completionPct, elapsed)
		feedback.Recommendation = "Task completed successfully. Ready for next command."

	case TaskStatusInProgress:
		feedback.Message = fg.generateInProgressMessage(pc, completionPct, elapsed)
		feedback.Recommendation = "Task is progressing normally. Continue monitoring."

	case TaskStatusStalled:
		feedback.Message = fg.generateStalledMessage(pc, completionPct, elapsed)
		feedback.Recommendation = "Task has stalled. Consider investigating obstacles or reassigning."

	case TaskStatusFailed:
		feedback.Message = fg.generateFailureMessage(pc)
		feedback.Recommendation = "Task failed. Review error and retry or adjust approach."

	case TaskStatusNotStarted:
		feedback.Message = fg.generateNotStartedMessage(pc)
		feedback.Recommendation = "Task acknowledged but not yet started by dwarves."

	default:
		feedback.Message = "Unknown task status"
		feedback.Recommendation = "Unable to determine task state"
	}

	return feedback
}

// generateSuccessMessage creates a success feedback message
func (fg *FeedbackGenerator) generateSuccessMessage(pc *PendingCommand, completionPct float64, elapsed time.Duration) string {
	return fmt.Sprintf("SUCCESS: %s completed successfully. "+
		"Processed %d/%d tiles (%.1f%%) in %s. "+
		"Dwarves have finished the designated work.",
		pc.Description,
		pc.CompletedTiles,
		pc.ExpectedTiles,
		completionPct,
		formatDuration(elapsed))
}

// generateInProgressMessage creates an in-progress feedback message
func (fg *FeedbackGenerator) generateInProgressMessage(pc *PendingCommand, completionPct float64, elapsed time.Duration) string {
	return fmt.Sprintf("IN PROGRESS: %s is underway. "+
		"Completed %d/%d tiles (%.1f%%). "+
		"Elapsed time: %s. "+
		"Dwarves are actively working on this task.",
		pc.Description,
		pc.CompletedTiles,
		pc.ExpectedTiles,
		completionPct,
		formatDuration(elapsed))
}

// generateStalledMessage creates a stalled feedback message
func (fg *FeedbackGenerator) generateStalledMessage(pc *PendingCommand, completionPct float64, elapsed time.Duration) string {
	timeSinceProgress := time.Since(pc.LastProgressTime)
	return fmt.Sprintf("STALLED: %s has stalled at %.1f%% complete (%d/%d tiles). "+
		"No progress for %s (total elapsed: %s). "+
		"Possible causes: blocked path, missing resources, or hazard obstruction. "+
		"Last progress detected: %s ago.",
		pc.Description,
		completionPct,
		pc.CompletedTiles,
		pc.ExpectedTiles,
		formatDuration(timeSinceProgress),
		formatDuration(elapsed),
		formatDuration(timeSinceProgress))
}

// generateFailureMessage creates a failure feedback message
func (fg *FeedbackGenerator) generateFailureMessage(pc *PendingCommand) string {
	errorMsg := "Unknown error"
	if pc.Command != nil && pc.Command.Response != nil {
		errorMsg = pc.Command.Response.ErrorMsg
	}

	return fmt.Sprintf("FAILED: %s could not be executed. "+
		"Error: %s. "+
		"The command was rejected or encountered a critical error.",
		pc.Description,
		errorMsg)
}

// generateNotStartedMessage creates a not-started feedback message
func (fg *FeedbackGenerator) generateNotStartedMessage(pc *PendingCommand) string {
	waitTime := time.Since(pc.Command.SentAt)
	return fmt.Sprintf("NOT STARTED: %s has been designated but no progress detected yet. "+
		"Waiting for %s. "+
		"Dwarves may be busy with other tasks or path-finding to the work area.",
		pc.Description,
		formatDuration(waitTime))
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "< 1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds > 0 {
			return fmt.Sprintf("%dm %ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dh", hours)
}

// GenerateBatchFeedback generates feedback for multiple pending commands
func (fg *FeedbackGenerator) GenerateBatchFeedback(commands []*PendingCommand) []*Feedback {
	feedbacks := make([]*Feedback, 0, len(commands))
	for _, cmd := range commands {
		feedbacks = append(feedbacks, fg.GenerateFeedback(cmd))
	}
	return feedbacks
}

// SummarizeFeedback creates a summary of multiple feedback items
func (fg *FeedbackGenerator) SummarizeFeedback(feedbacks []*Feedback) string {
	if len(feedbacks) == 0 {
		return "No active tasks"
	}

	completed := 0
	inProgress := 0
	stalled := 0
	failed := 0
	notStarted := 0

	for _, fb := range feedbacks {
		switch fb.Status {
		case TaskStatusCompleted:
			completed++
		case TaskStatusInProgress:
			inProgress++
		case TaskStatusStalled:
			stalled++
		case TaskStatusFailed:
			failed++
		case TaskStatusNotStarted:
			notStarted++
		}
	}

	summary := fmt.Sprintf("Task Summary: %d total | ", len(feedbacks))

	if completed > 0 {
		summary += fmt.Sprintf("%d completed | ", completed)
	}
	if inProgress > 0 {
		summary += fmt.Sprintf("%d in progress | ", inProgress)
	}
	if stalled > 0 {
		summary += fmt.Sprintf("%d stalled | ", stalled)
	}
	if failed > 0 {
		summary += fmt.Sprintf("%d failed | ", failed)
	}
	if notStarted > 0 {
		summary += fmt.Sprintf("%d not started", notStarted)
	}

	return summary
}
