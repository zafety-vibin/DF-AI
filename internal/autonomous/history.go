package autonomous

import (
	"fmt"
	"strings"
	"time"

	"github.com/df-ai/orchestrator/internal/commands"
)

// Turn represents a single decision cycle in the autonomous loop
type Turn struct {
	TurnNumber   int                  // Sequential turn counter
	Timestamp    time.Time            // When this turn started
	Context      string               // Context assembled for this turn
	LLMPrompt    string               // Full prompt sent to LLM
	LLMResponse  string               // Raw LLM response
	ParsedAction string               // Extracted action/command
	CommandSent  bool                 // Whether a command was sent
	CommandID    uint32               // ID of command sent (if any)
	Feedback     []*commands.Feedback // Feedback from previous commands
	Duration     time.Duration        // Time taken for this turn
}

// ConversationHistory maintains a sliding window of recent turns
type ConversationHistory struct {
	turns     []*Turn
	maxTurns  int // Maximum number of turns to keep
	turnCount int // Total turns processed (including evicted)
}

// NewConversationHistory creates a new conversation history with a sliding window
func NewConversationHistory(maxTurns int) *ConversationHistory {
	if maxTurns <= 0 {
		maxTurns = 5 // Default to 5 turns
	}
	return &ConversationHistory{
		turns:     make([]*Turn, 0, maxTurns),
		maxTurns:  maxTurns,
		turnCount: 0,
	}
}

// AddTurn adds a new turn to the history
func (ch *ConversationHistory) AddTurn(turn *Turn) {
	ch.turnCount++
	turn.TurnNumber = ch.turnCount

	// Add turn to history
	ch.turns = append(ch.turns, turn)

	// Trim to max size (keep most recent)
	if len(ch.turns) > ch.maxTurns {
		ch.turns = ch.turns[1:] // Remove oldest
	}
}

// GetRecentTurns returns the N most recent turns
func (ch *ConversationHistory) GetRecentTurns(n int) []*Turn {
	if n <= 0 || len(ch.turns) == 0 {
		return nil
	}

	if n >= len(ch.turns) {
		// Return all turns
		result := make([]*Turn, len(ch.turns))
		copy(result, ch.turns)
		return result
	}

	// Return last N turns
	start := len(ch.turns) - n
	result := make([]*Turn, n)
	copy(result, ch.turns[start:])
	return result
}

// GetLastTurn returns the most recent turn
func (ch *ConversationHistory) GetLastTurn() *Turn {
	if len(ch.turns) == 0 {
		return nil
	}
	return ch.turns[len(ch.turns)-1]
}

// GetTurnCount returns the total number of turns processed
func (ch *ConversationHistory) GetTurnCount() int {
	return ch.turnCount
}

// GetHistorySize returns the current number of turns in memory
func (ch *ConversationHistory) GetHistorySize() int {
	return len(ch.turns)
}

// FormatForPrompt formats the conversation history for inclusion in an LLM prompt
func (ch *ConversationHistory) FormatForPrompt() string {
	if len(ch.turns) == 0 {
		return "No previous turns."
	}

	var sb strings.Builder
	sb.WriteString("## Conversation History\n\n")
	sb.WriteString(fmt.Sprintf("Recent decision cycles (showing last %d of %d total turns):\n\n",
		len(ch.turns), ch.turnCount))

	for i, turn := range ch.turns {
		sb.WriteString(fmt.Sprintf("### Turn %d (%.1fs ago)\n",
			turn.TurnNumber,
			time.Since(turn.Timestamp).Seconds()))

		// Include action taken
		if turn.CommandSent {
			sb.WriteString(fmt.Sprintf("**Action**: %s (Command ID: %d)\n",
				turn.ParsedAction,
				turn.CommandID))
		} else {
			sb.WriteString(fmt.Sprintf("**Action**: %s\n", turn.ParsedAction))
		}

		// Include feedback summary
		if len(turn.Feedback) > 0 {
			sb.WriteString(fmt.Sprintf("**Feedback**: %d task(s) tracked\n", len(turn.Feedback)))
			for _, fb := range turn.Feedback {
				sb.WriteString(fmt.Sprintf("  - %s: %.1f%% complete (%s)\n",
					fb.TaskDescription,
					fb.CompletionPct,
					fb.Status.String()))
			}
		}

		// Include duration
		sb.WriteString(fmt.Sprintf("**Duration**: %s\n", formatDuration(turn.Duration)))

		if i < len(ch.turns)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// FormatShortSummary creates a compact summary of recent history
func (ch *ConversationHistory) FormatShortSummary() string {
	if len(ch.turns) == 0 {
		return "No history"
	}

	lastTurn := ch.turns[len(ch.turns)-1]
	summary := fmt.Sprintf("Turn %d: %s", lastTurn.TurnNumber, lastTurn.ParsedAction)

	if len(lastTurn.Feedback) > 0 {
		completedCount := 0
		inProgressCount := 0
		for _, fb := range lastTurn.Feedback {
			if fb.Status == commands.TaskStatusCompleted {
				completedCount++
			} else if fb.Status == commands.TaskStatusInProgress {
				inProgressCount++
			}
		}
		summary += fmt.Sprintf(" | %d completed, %d in progress", completedCount, inProgressCount)
	}

	return summary
}

// Clear removes all turns from history
func (ch *ConversationHistory) Clear() {
	ch.turns = make([]*Turn, 0, ch.maxTurns)
	ch.turnCount = 0
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
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

// TurnBuilder helps construct a Turn incrementally
type TurnBuilder struct {
	turn *Turn
}

// NewTurnBuilder creates a new turn builder
func NewTurnBuilder() *TurnBuilder {
	return &TurnBuilder{
		turn: &Turn{
			Timestamp: time.Now(),
			Feedback:  make([]*commands.Feedback, 0),
		},
	}
}

// SetContext sets the context for this turn
func (tb *TurnBuilder) SetContext(context string) *TurnBuilder {
	tb.turn.Context = context
	return tb
}

// SetLLMPrompt sets the LLM prompt
func (tb *TurnBuilder) SetLLMPrompt(prompt string) *TurnBuilder {
	tb.turn.LLMPrompt = prompt
	return tb
}

// SetLLMResponse sets the LLM response
func (tb *TurnBuilder) SetLLMResponse(response string) *TurnBuilder {
	tb.turn.LLMResponse = response
	return tb
}

// SetParsedAction sets the parsed action
func (tb *TurnBuilder) SetParsedAction(action string) *TurnBuilder {
	tb.turn.ParsedAction = action
	return tb
}

// SetCommandSent marks that a command was sent
func (tb *TurnBuilder) SetCommandSent(commandID uint32) *TurnBuilder {
	tb.turn.CommandSent = true
	tb.turn.CommandID = commandID
	return tb
}

// AddFeedback adds feedback to this turn
func (tb *TurnBuilder) AddFeedback(feedback *commands.Feedback) *TurnBuilder {
	tb.turn.Feedback = append(tb.turn.Feedback, feedback)
	return tb
}

// SetFeedback sets all feedback at once
func (tb *TurnBuilder) SetFeedback(feedbacks []*commands.Feedback) *TurnBuilder {
	tb.turn.Feedback = feedbacks
	return tb
}

// Build finalizes the turn and calculates duration
func (tb *TurnBuilder) Build() *Turn {
	tb.turn.Duration = time.Since(tb.turn.Timestamp)
	return tb.turn
}
