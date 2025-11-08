package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/commands"
	appcontext "github.com/df-ai/orchestrator/internal/context"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/llm"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

// AutonomousLoop orchestrates the main decision cycle
// Assembles context -> queries LLM -> executes commands -> tracks feedback
type AutonomousLoop struct {
	logger              *logging.Logger
	timer               *Timer
	dfhackClient        *dfhack.Client
	modOverlay          *modifications.ModificationOverlay
	topoOverlay         *topology.TopologyOverlay
	hazardMgr           *hazards.HazardManager
	contextAssembler    *appcontext.Assembler
	llmProvider         llm.Provider
	commandExecutor     *commands.CommandExecutor
	feedbackGenerator   *commands.FeedbackGenerator
	conversationHistory *ConversationHistory
	systemPrompt        string
	logFile             *os.File
	mu                  sync.RWMutex
	running             bool
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup

	// Pending command tracking
	pendingCommands map[uint32]*commands.PendingCommand
	pendingMu       sync.RWMutex
}

// NewLoop creates a new autonomous loop
func NewLoop(
	logger *logging.Logger,
	interval time.Duration,
	dfhackClient *dfhack.Client,
	modOverlay *modifications.ModificationOverlay,
	topoOverlay *topology.TopologyOverlay,
	hazardMgr *hazards.HazardManager,
	contextAssembler *appcontext.Assembler,
	llmProvider llm.Provider,
	commandExecutor *commands.CommandExecutor,
	systemPrompt string,
) *AutonomousLoop {
	ctx, cancel := context.WithCancel(context.Background())

	loop := &AutonomousLoop{
		logger:              logger,
		timer:               NewTimer(interval),
		dfhackClient:        dfhackClient,
		modOverlay:          modOverlay,
		topoOverlay:         topoOverlay,
		hazardMgr:           hazardMgr,
		contextAssembler:    contextAssembler,
		llmProvider:         llmProvider,
		commandExecutor:     commandExecutor,
		feedbackGenerator:   commands.NewFeedbackGenerator(),
		conversationHistory: NewConversationHistory(5), // Keep last 5 turns
		systemPrompt:        systemPrompt,
		ctx:                 ctx,
		cancel:              cancel,
		pendingCommands:     make(map[uint32]*commands.PendingCommand),
	}

	// Create logs directory and open log file
	if err := os.MkdirAll("logs", 0755); err != nil {
		logger.Warn("failed to create logs directory", logging.Field{Key: "error", Value: err.Error()})
	} else {
		logPath := filepath.Join("logs", "llm-interactions.jsonl")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger.Warn("failed to open LLM log file", logging.Field{Key: "error", Value: err.Error()})
		} else {
			loop.logFile = logFile
		}
	}

	return loop
}

// Start begins the autonomous decision loop in a background goroutine
func (al *AutonomousLoop) Start(ctx context.Context) {
	al.mu.Lock()
	if al.running {
		al.mu.Unlock()
		al.logger.Warn("autonomous loop already running")
		return
	}
	al.running = true
	al.mu.Unlock()

	al.logger.Info("autonomous loop starting",
		logging.Field{Key: "interval", Value: al.timer.GetInterval()},
		logging.Field{Key: "provider", Value: al.llmProvider.GetProviderType()},
		logging.Field{Key: "model", Value: al.llmProvider.GetModelName()})

	al.wg.Add(1)
	go al.run(ctx)
}

// run is the main loop goroutine
func (al *AutonomousLoop) run(ctx context.Context) {
	defer al.wg.Done()
	defer func() {
		al.mu.Lock()
		al.running = false
		al.mu.Unlock()
	}()

	al.logger.Info("autonomous loop started")

	for {
		// Wait for next cycle or shutdown
		select {
		case <-ctx.Done():
			al.logger.Info("autonomous loop stopping (context cancelled)")
			return
		case <-al.ctx.Done():
			al.logger.Info("autonomous loop stopping (internal shutdown)")
			return
		default:
			// Wait for timer
			if !al.timer.Wait() {
				al.logger.Info("autonomous loop stopping (timer stopped)")
				return
			}
		}

		// Run decision cycle
		al.logger.Info("starting autonomous decision cycle")
		if err := al.runCycle(); err != nil {
			al.logger.Error("decision cycle failed", err)
		}
	}
}

// runCycle executes a single decision cycle
func (al *AutonomousLoop) runCycle() error {
	// Skip cycle if DFHack not connected (prevents wasting API calls on stale data)
	if al.dfhackClient != nil && !al.dfhackClient.IsConnected() {
		al.logger.Debug("skipping cycle - DFHack not connected")
		return nil
	}

	cycleStart := time.Now()
	turnBuilder := NewTurnBuilder()

	// Step 1: Generate feedback from pending commands
	feedback := al.generateFeedback()
	turnBuilder.SetFeedback(feedback)

	al.logger.Debug("generated feedback",
		logging.Field{Key: "feedback_count", Value: len(feedback)})

	// Step 2: Assemble fort context (Level 0 + Level 1)
	context, err := al.assembleContext()
	if err != nil {
		return fmt.Errorf("failed to assemble context: %w", err)
	}
	turnBuilder.SetContext(context)

	al.logger.Debug("assembled context",
		logging.Field{Key: "context_size", Value: len(context)})

	// Step 3: Format prompt with system instructions, history, and context
	prompt := al.formatPrompt(context, feedback)
	turnBuilder.SetLLMPrompt(prompt)

	al.logger.Debug("formatted prompt",
		logging.Field{Key: "prompt_size", Value: len(prompt)})

	// Step 4: Send to LLM provider
	llmPrompt := &llm.Prompt{
		SystemPrompt: al.systemPrompt,
		UserMessage:  prompt,
		History:      []llm.Message{}, // History is embedded in prompt
		MaxTokens:    4096,
		Temperature:  0.7,
	}

	al.logger.Info("sending prompt to LLM",
		logging.Field{Key: "provider", Value: al.llmProvider.GetProviderType()},
		logging.Field{Key: "model", Value: al.llmProvider.GetModelName()})

	response, err := al.llmProvider.SendPrompt(al.ctx, llmPrompt)
	if err != nil {
		return fmt.Errorf("LLM request failed: %w", err)
	}

	turnBuilder.SetLLMResponse(response.Text)

	al.logger.Info("received LLM response",
		logging.Field{Key: "tokens_prompt", Value: response.TokensPrompt},
		logging.Field{Key: "tokens_completion", Value: response.TokensCompletion},
		logging.Field{Key: "latency_ms", Value: response.Latency.Milliseconds()},
		logging.Field{Key: "finish_reason", Value: response.FinishReason})

	// Step 5: Parse response for commands
	parsed, err := llm.ParseResponse(response.Text)
	if err != nil {
		al.logger.Warn("failed to parse LLM response", logging.Field{Key: "error", Value: err.Error()})
		turnBuilder.SetParsedAction("No action (parse failed)")
	} else {
		al.logger.Debug("parsed LLM response",
			logging.Field{Key: "reasoning", Value: parsed.Reasoning},
			logging.Field{Key: "command_count", Value: len(parsed.Commands)})

		// Step 6: Execute commands
		if len(parsed.Commands) > 0 {
			commandID, err := al.executeCommands(parsed.Commands)
			if err != nil {
				al.logger.Error("command execution failed", err)
				turnBuilder.SetParsedAction(fmt.Sprintf("Command failed: %v", err))
			} else {
				action := fmt.Sprintf("%s (%d commands)", parsed.Reasoning, len(parsed.Commands))
				turnBuilder.SetParsedAction(action)
				if commandID > 0 {
					turnBuilder.SetCommandSent(commandID)
				}
			}
		} else {
			turnBuilder.SetParsedAction(parsed.Reasoning)
		}
	}

	// Step 7: Build turn and add to history
	turn := turnBuilder.Build()
	al.conversationHistory.AddTurn(turn)

	al.logger.Info("completed decision cycle",
		logging.Field{Key: "turn_number", Value: turn.TurnNumber},
		logging.Field{Key: "duration_ms", Value: turn.Duration.Milliseconds()},
		logging.Field{Key: "action", Value: turn.ParsedAction})

	// Step 8: Log full interaction to file
	al.logInteraction(turn, response, cycleStart)

	return nil
}

// assembleContext assembles fort state context
func (al *AutonomousLoop) assembleContext() (string, error) {
	// For now, use Level 0 (text overview) + Level 1 (active area)
	// Future: adaptive context level based on task complexity

	entities := appcontext.EntityInfoSlice{} // TODO: Get entities from entity tracking

	// Generate Level 0 context (text overview)
	ctx0, err := al.contextAssembler.AssembleContext(
		appcontext.Level0,
		al.modOverlay,
		al.hazardMgr,
		entities,
		al.topoOverlay,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate Level 0 context: %w", err)
	}

	// Generate Level 1 context (active area with chambers)
	ctx1, err := al.contextAssembler.AssembleContext(
		appcontext.Level1,
		al.modOverlay,
		al.hazardMgr,
		entities,
		al.topoOverlay,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate Level 1 context: %w", err)
	}

	// Combine contexts
	var sb strings.Builder
	sb.WriteString("## Fort State Overview\n\n")
	sb.WriteString(ctx0.TextOverview)
	sb.WriteString("\n\n")

	// Add Level 1 JSON data
	if ctx1 != nil {
		jsonData, err := appcontext.FormatAsJSON(ctx1)
		if err == nil {
			sb.WriteString("## Detailed Context (JSON)\n\n")
			sb.WriteString("```json\n")
			sb.Write(jsonData)
			sb.WriteString("\n```\n")
		}
	}

	return sb.String(), nil
}

// formatPrompt creates the full prompt with history and context
func (al *AutonomousLoop) formatPrompt(context string, feedback []*commands.Feedback) string {
	var sb strings.Builder

	// Add conversation history
	sb.WriteString(al.conversationHistory.FormatForPrompt())
	sb.WriteString("\n\n")

	// Add current feedback
	if len(feedback) > 0 {
		sb.WriteString("## Current Task Status\n\n")
		for _, fb := range feedback {
			sb.WriteString(fmt.Sprintf("- %s\n", fb.Message))
		}
		sb.WriteString("\n")
	}

	// Add fort context
	sb.WriteString(context)
	sb.WriteString("\n\n")

	// Add instructions
	sb.WriteString("## Instructions\n\n")
	sb.WriteString("Based on the fort state and task status above, decide on the next action.\n")
	sb.WriteString("Respond with your reasoning and any commands to execute.\n\n")
	sb.WriteString("Command format:\n")
	sb.WriteString("- Dig: dig from (x1, y1, z) to (x2, y2, z)\n")
	sb.WriteString("- Build: build at (x, y, z)\n")
	sb.WriteString("- Wait: wait for N turns\n")

	return sb.String()
}

// executeCommands executes parsed commands
func (al *AutonomousLoop) executeCommands(specs []llm.CommandSpec) (uint32, error) {
	if len(specs) == 0 {
		return 0, nil
	}

	// Execute first command (multi-command support in future)
	spec := specs[0]

	switch spec.Type {
	case "dig":
		if spec.Region == nil {
			return 0, fmt.Errorf("dig command missing region")
		}
		return al.executeDig(spec)
	case "build":
		if spec.Region == nil {
			return 0, fmt.Errorf("build command missing region")
		}
		return al.executeBuild(spec)
	case "wait":
		al.logger.Info("executing wait command",
			logging.Field{Key: "params", Value: spec.Params})
		return 0, nil // No command ID for wait
	default:
		return 0, fmt.Errorf("unknown command type: %s", spec.Type)
	}
}

// executeDig executes a dig command
func (al *AutonomousLoop) executeDig(spec llm.CommandSpec) (uint32, error) {
	region := spec.Region
	al.logger.Info("executing dig command",
		logging.Field{Key: "region", Value: fmt.Sprintf("(%d,%d,%d) to (%d,%d,%d)",
			region.X1, region.Y1, region.Z, region.X2, region.Y2, region.Z)})

	result, err := al.commandExecutor.SendDigCommand(
		int16(region.X1),
		int16(region.Y1),
		int16(region.Z),
		int16(region.X2),
		int16(region.Y2),
	)
	if err != nil {
		return 0, fmt.Errorf("dig command failed: %w", err)
	}

	if !result.Success {
		return 0, fmt.Errorf("dig command rejected: %s", result.ErrorMsg)
	}

	// Track as pending command
	cmdID := result.Response.CommandID
	expectedTiles := commands.CalculateExpectedTiles(protocol.Region{
		X1: int16(region.X1), Y1: int16(region.Y1), Z1: int16(region.Z),
		X2: int16(region.X2), Y2: int16(region.Y2), Z2: int16(region.Z),
	})

	al.pendingMu.Lock()
	al.pendingCommands[cmdID] = &commands.PendingCommand{
		Command:          nil, // Will be set by executor
		TaskStatus:       commands.TaskStatusNotStarted,
		ExpectedTiles:    expectedTiles,
		CompletedTiles:   0,
		LastProgressTime: time.Now(),
		StartTime:        time.Now(),
		Description: fmt.Sprintf("Dig region (%d,%d,%d) to (%d,%d,%d)",
			region.X1, region.Y1, region.Z, region.X2, region.Y2, region.Z),
	}
	al.pendingMu.Unlock()

	return cmdID, nil
}

// executeBuild executes a build command
func (al *AutonomousLoop) executeBuild(spec llm.CommandSpec) (uint32, error) {
	region := spec.Region
	buildType := uint8(0) // Default build type (floor/wall)

	al.logger.Info("executing build command",
		logging.Field{Key: "position", Value: fmt.Sprintf("(%d,%d,%d)",
			region.X1, region.Y1, region.Z)})

	result, err := al.commandExecutor.SendBuildCommand(
		int16(region.X1),
		int16(region.Y1),
		int16(region.Z),
		buildType,
	)
	if err != nil {
		return 0, fmt.Errorf("build command failed: %w", err)
	}

	if !result.Success {
		return 0, fmt.Errorf("build command rejected: %s", result.ErrorMsg)
	}

	return result.Response.CommandID, nil
}

// generateFeedback generates feedback for all pending commands
func (al *AutonomousLoop) generateFeedback() []*commands.Feedback {
	al.pendingMu.Lock()
	defer al.pendingMu.Unlock()

	feedback := make([]*commands.Feedback, 0, len(al.pendingCommands))

	// Check progress on each pending command
	for cmdID, pending := range al.pendingCommands {
		// Count completed tiles by checking modification overlay
		// This is a simplified implementation - in production would query modifications by command ID
		completedTiles := 0
		if al.modOverlay != nil {
			// Get modifications in region (rough approximation)
			bounds := al.modOverlay.GetBounds()
			mods := al.modOverlay.GetModificationsInRegion(modifications.Region{
				XMin: bounds.XMin, XMax: bounds.XMax,
				YMin: bounds.YMin, YMax: bounds.YMax,
				ZMin: bounds.ZMin, ZMax: bounds.ZMax,
			}, time.Time{})
			completedTiles = len(mods)
		}

		// Update task status (would use tracker.CheckProgress in full implementation)
		if completedTiles >= pending.ExpectedTiles {
			pending.TaskStatus = commands.TaskStatusCompleted
			pending.LastProgressTime = time.Now()
		} else if completedTiles > pending.CompletedTiles {
			pending.TaskStatus = commands.TaskStatusInProgress
			pending.LastProgressTime = time.Now()
		} else if pending.TaskStatus == commands.TaskStatusInProgress &&
			time.Since(pending.LastProgressTime) > 60*time.Second {
			pending.TaskStatus = commands.TaskStatusStalled
		}

		pending.CompletedTiles = completedTiles

		// Generate feedback
		fb := al.feedbackGenerator.GenerateFeedback(pending)
		feedback = append(feedback, fb)

		// Remove completed commands
		if pending.TaskStatus == commands.TaskStatusCompleted {
			delete(al.pendingCommands, cmdID)
		}
	}

	return feedback
}

// logInteraction logs the full interaction to JSONL file
func (al *AutonomousLoop) logInteraction(turn *Turn, response *llm.Response, cycleStart time.Time) {
	if al.logFile == nil {
		return
	}

	logEntry := map[string]interface{}{
		"timestamp":         cycleStart.Format(time.RFC3339),
		"turn_number":       turn.TurnNumber,
		"duration_ms":       turn.Duration.Milliseconds(),
		"context_size":      len(turn.Context),
		"prompt_size":       len(turn.LLMPrompt),
		"response_size":     len(turn.LLMResponse),
		"tokens_prompt":     response.TokensPrompt,
		"tokens_completion": response.TokensCompletion,
		"latency_ms":        response.Latency.Milliseconds(),
		"finish_reason":     response.FinishReason,
		"action":            turn.ParsedAction,
		"command_sent":      turn.CommandSent,
		"command_id":        turn.CommandID,
		"feedback_count":    len(turn.Feedback),
		"provider":          al.llmProvider.GetProviderType(),
		"model":             al.llmProvider.GetModelName(),
	}

	data, err := json.Marshal(logEntry)
	if err != nil {
		al.logger.Warn("failed to marshal log entry", logging.Field{Key: "error", Value: err.Error()})
		return
	}

	if _, err := al.logFile.Write(append(data, '\n')); err != nil {
		al.logger.Warn("failed to write log entry", logging.Field{Key: "error", Value: err.Error()})
	}
}

// TriggerImmediate triggers an immediate decision cycle
func (al *AutonomousLoop) TriggerImmediate() {
	al.timer.TriggerImmediate()
	al.logger.Info("immediate cycle triggered")
}

// GetHistory returns the conversation history
func (al *AutonomousLoop) GetHistory() *ConversationHistory {
	return al.conversationHistory
}

// Stop stops the autonomous loop
func (al *AutonomousLoop) Stop() {
	al.logger.Info("stopping autonomous loop")
	al.cancel()
	al.timer.Stop()
	al.wg.Wait()

	if al.logFile != nil {
		al.logFile.Close()
	}

	al.logger.Info("autonomous loop stopped")
}
