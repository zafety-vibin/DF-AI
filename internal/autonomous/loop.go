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
	"github.com/df-ai/orchestrator/internal/phases"
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
	phaseManager        *phases.PhaseManager // Fort development phases
	logFile             *os.File
	mu                  sync.RWMutex
	running             bool
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup

	// Pending command tracking
	pendingCommands map[uint32]*commands.PendingCommand
	pendingMu       sync.RWMutex

	// Entity cache (updated from ENTITY_UPDATE messages)
	cachedEntities []protocol.EntityInfo
	entityMu       sync.RWMutex
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
	phaseManager *phases.PhaseManager, // Optional phase manager
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
		phaseManager:        phaseManager,
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

		// Check connection before running cycle (prevents wasting API calls)
		if al.dfhackClient != nil && !al.dfhackClient.IsConnected() {
			al.logger.Debug("skipping cycle - DFHack not connected")
			continue // Skip this cycle, wait for next timer
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

// mapDigType converts string dig type to protocol constant
func mapDigType(digType string) uint8 {
	switch digType {
	case "stairs", "updownstairs":
		return protocol.DigTypeUpDownStair
	case "channel":
		return protocol.DigTypeChannel
	case "ramp":
		return protocol.DigTypeRamp
	case "downstair":
		return protocol.DigTypeDownStair
	case "upstair":
		return protocol.DigTypeUpStair
	default:
		return protocol.DigTypeDefault // Standard mining
	}
}

// assembleContext assembles fort state context
// Uses smart assembly: first turn shows terrain, later turns show modifications + rooms
func (al *AutonomousLoop) assembleContext() (string, error) {
	// Get entities from cached entities (updated via ENTITY_UPDATE)
	entities := al.getEntities()

	// DEBUG: Log entity count
	dwarfCount := 0
	for _, e := range entities {
		if e.Type == protocol.EntityTypeDwarf {
			dwarfCount++
		}
	}
	al.logger.Info("assembling context",
		logging.Field{Key: "total_entities", Value: len(entities)},
		logging.Field{Key: "dwarf_count", Value: dwarfCount})

	// Smart context assembly based on fort state
	// First turn (no mods): Show embark point + terrain + dwarves
	// Later turns: Show modifications + chambers + dwarves + hazards

	isFirstTurn := al.modOverlay.GetCount() == 0

	if isFirstTurn {
		// FIRST TURN: Use L1 with topology slice
		ctx, err := al.contextAssembler.AssembleContext(
			appcontext.Level1,
			al.modOverlay,
			al.hazardMgr,
			entities,
			al.topoOverlay,
		)
		if err != nil {
			return "", fmt.Errorf("failed to generate first turn context: %w", err)
		}

		return al.formatContextAsJSON(ctx)
	}

	// SUBSEQUENT TURNS: Use L1 (modifications + chambers with room types)
	ctx, err := al.contextAssembler.AssembleContext(
		appcontext.Level1,
		al.modOverlay,
		al.hazardMgr,
		entities,
		al.topoOverlay,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate context: %w", err)
	}

	return al.formatContextAsJSON(ctx)
}

// formatContextAsJSON converts ViewportContext to JSON string
func (al *AutonomousLoop) formatContextAsJSON(ctx *appcontext.ViewportContext) (string, error) {
	jsonData, err := appcontext.FormatAsJSON(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to format context as JSON: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("## Fort State\n\n")
	sb.WriteString("```json\n")
	sb.Write(jsonData)
	sb.WriteString("\n```\n")

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

	// Add phase information (if phase manager exists)
	if al.phaseManager != nil {
		phaseText := al.phaseManager.GetPromptAddition()
		sb.WriteString(phaseText)
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

// executeCommands executes ALL parsed commands
func (al *AutonomousLoop) executeCommands(specs []llm.CommandSpec) (uint32, error) {
	if len(specs) == 0 {
		return 0, nil
	}

	al.logger.Info("executing commands", logging.Field{Key: "count", Value: len(specs)})

	// Execute ALL commands
	var lastCommandID uint32
	for i, spec := range specs {
		var cmdID uint32
		var err error

		switch spec.Type {
		case "dig":
			if spec.Region == nil {
				al.logger.Warn("dig command missing region, skipping",
					logging.Field{Key: "index", Value: i})
				continue
			}
			cmdID, err = al.executeDig(spec)
		case "chop":
			if spec.Region == nil {
				al.logger.Warn("chop command missing region, skipping",
					logging.Field{Key: "index", Value: i})
				continue
			}
			cmdID, err = al.executeChop(spec)
		case "gather":
			if spec.Region == nil {
				al.logger.Warn("gather command missing region, skipping",
					logging.Field{Key: "index", Value: i})
				continue
			}
			cmdID, err = al.executeGather(spec)
		case "build":
			if spec.Region == nil {
				al.logger.Warn("build command missing region, skipping",
					logging.Field{Key: "index", Value: i})
				continue
			}
			cmdID, err = al.executeBuild(spec)
		case "wait":
			al.logger.Info("executing wait command")
			continue
		default:
			al.logger.Warn("unknown command type, skipping",
				logging.Field{Key: "type", Value: spec.Type},
				logging.Field{Key: "index", Value: i})
			continue
		}

		if err != nil {
			al.logger.Error("command failed", err,
				logging.Field{Key: "index", Value: i},
				logging.Field{Key: "type", Value: spec.Type})
			// Continue with other commands despite error
		} else if cmdID > 0 {
			lastCommandID = cmdID
		}
	}

	return lastCommandID, nil
}

// executeDig executes a dig command
func (al *AutonomousLoop) executeDig(spec llm.CommandSpec) (uint32, error) {
	region := spec.Region

	// Map dig type string to protocol constant
	digType := mapDigType(spec.DigType)

	al.logger.Info("executing dig command",
		logging.Field{Key: "region", Value: fmt.Sprintf("(%d,%d,%d) to (%d,%d,%d)",
			region.X1, region.Y1, region.Z, region.X2, region.Y2, region.Z)},
		logging.Field{Key: "dig_type", Value: spec.DigType})

	result, err := al.commandExecutor.SendDigCommand(
		digType,
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

// executeChop executes a tree chopping command
func (al *AutonomousLoop) executeChop(spec llm.CommandSpec) (uint32, error) {
	region := spec.Region
	al.logger.Info("executing chop command",
		logging.Field{Key: "region", Value: fmt.Sprintf("(%d,%d,%d) to (%d,%d,%d)",
			region.X1, region.Y1, region.Z, region.X2, region.Y2, region.Z)})

	result, err := al.commandExecutor.SendChopCommand(
		int16(region.X1),
		int16(region.Y1),
		int16(region.Z),
		int16(region.X2),
		int16(region.Y2),
	)
	if err != nil {
		return 0, fmt.Errorf("chop command failed: %w", err)
	}

	if !result.Success {
		return 0, fmt.Errorf("chop command rejected: %s", result.ErrorMsg)
	}

	cmdID := result.Response.CommandID
	expectedTiles := commands.CalculateExpectedTiles(protocol.Region{
		X1: int16(region.X1), Y1: int16(region.Y1), Z1: int16(region.Z),
		X2: int16(region.X2), Y2: int16(region.Y2), Z2: int16(region.Z),
	})

	al.pendingMu.Lock()
	al.pendingCommands[cmdID] = &commands.PendingCommand{
		TaskStatus:    commands.TaskStatusNotStarted,
		ExpectedTiles: expectedTiles,
		StartTime:     time.Now(),
		Description:   fmt.Sprintf("Chop trees in region (%d,%d,%d) to (%d,%d,%d)", region.X1, region.Y1, region.Z, region.X2, region.Y2, region.Z),
	}
	al.pendingMu.Unlock()

	return cmdID, nil
}

// executeGather executes a plant gathering command
func (al *AutonomousLoop) executeGather(spec llm.CommandSpec) (uint32, error) {
	region := spec.Region
	al.logger.Info("executing gather command",
		logging.Field{Key: "region", Value: fmt.Sprintf("(%d,%d,%d) to (%d,%d,%d)",
			region.X1, region.Y1, region.Z, region.X2, region.Y2, region.Z)})

	result, err := al.commandExecutor.SendGatherCommand(
		int16(region.X1),
		int16(region.Y1),
		int16(region.Z),
		int16(region.X2),
		int16(region.Y2),
	)
	if err != nil {
		return 0, fmt.Errorf("gather command failed: %w", err)
	}

	if !result.Success {
		return 0, fmt.Errorf("gather command rejected: %s", result.ErrorMsg)
	}

	cmdID := result.Response.CommandID
	expectedTiles := commands.CalculateExpectedTiles(protocol.Region{
		X1: int16(region.X1), Y1: int16(region.Y1), Z1: int16(region.Z),
		X2: int16(region.X2), Y2: int16(region.Y2), Z2: int16(region.Z),
	})

	al.pendingMu.Lock()
	al.pendingCommands[cmdID] = &commands.PendingCommand{
		TaskStatus:    commands.TaskStatusNotStarted,
		ExpectedTiles: expectedTiles,
		StartTime:     time.Now(),
		Description:   fmt.Sprintf("Gather plants in region (%d,%d,%d) to (%d,%d,%d)", region.X1, region.Y1, region.Z, region.X2, region.Y2, region.Z),
	}
	al.pendingMu.Unlock()

	return cmdID, nil
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

// UpdateEntities updates the cached entity list (called from main.go when ENTITY_UPDATE received)
func (al *AutonomousLoop) UpdateEntities(entities []protocol.EntityInfo) {
	al.entityMu.Lock()
	defer al.entityMu.Unlock()
	al.cachedEntities = entities
	al.logger.Debug("entity cache updated", logging.Field{Key: "count", Value: len(entities)})
}

// getEntities returns the cached entity list for context assembly
func (al *AutonomousLoop) getEntities() appcontext.EntityInfoSlice {
	al.entityMu.RLock()
	defer al.entityMu.RUnlock()
	return appcontext.EntityInfoSlice(al.cachedEntities)
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
