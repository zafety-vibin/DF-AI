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

	"github.com/df-ai/orchestrator/internal/agents"
	"github.com/df-ai/orchestrator/internal/commands"
	appcontext "github.com/df-ai/orchestrator/internal/context"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/llm"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/phases"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/spatial"
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
	agentRegistry       *agents.AgentRegistry // Goal-oriented agents (Feature 006)
	enableGoalAgents    bool                  // Enable graph-based agents vs direct-LLM
	graphExecutor       *agents.GraphExecutor  // Converts nodes to commands

	// Feature 007: Spatial Validator Planner
	svp              *spatial.SpatialValidatorPlanner // Z-level designations
	svpState         SVPState                         // Tracks SVP initialization
	topologyReceived bool                             // Whether RESYNC received
	entitiesReceived bool                             // Whether ENTITY_UPDATE received

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

// SVPState tracks SVP initialization progress
type SVPState int

const (
	SVPStateWaitingForTopology SVPState = iota
	SVPStateWaitingForEntities
	SVPStateReady
)

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
		graphExecutor:       agents.NewGraphExecutor(logger),
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

	// Step 2.5: Agent analysis (if goal agents enabled)
	var proposalGraph *agents.ProposalGraph
	if al.enableGoalAgents && al.agentRegistry != nil {
		// Compute fort metrics from current state
		metrics := al.computeFortMetrics()

		al.logger.Debug("computed fort metrics",
			logging.Field{Key: "dwarves", Value: metrics.DwarfCount},
			logging.Field{Key: "enemies", Value: metrics.EnemyCount},
			logging.Field{Key: "fort_age", Value: metrics.FortAge})

		// Run all enabled agents in parallel
		proposals, agentLatency := al.analyzeAgents(metrics)

		if len(proposals) > 0 {
			// Assemble proposal graph
			proposalGraph = al.assembleProposalGraph(proposals)

			// Log agent proposals (JSON format for debugging)
			for _, proposal := range proposals {
				proposalJSON, _ := json.Marshal(proposal)
				al.logger.Debug("agent proposal",
					logging.Field{Key: "agent", Value: proposal.AgentName},
					logging.Field{Key: "type", Value: string(proposal.Type)},
					logging.Field{Key: "priority", Value: proposal.Priority},
					logging.Field{Key: "urgency", Value: proposal.Urgency},
					logging.Field{Key: "rationale", Value: proposal.Rationale},
					logging.Field{Key: "proposal_json", Value: string(proposalJSON)})
			}

			// Log performance metrics
			al.logger.Info("agent analysis metrics",
				logging.Field{Key: "total_latency_ms", Value: agentLatency.Milliseconds()},
				logging.Field{Key: "proposal_count", Value: len(proposals)},
				logging.Field{Key: "graph_nodes", Value: len(proposalGraph.Nodes)},
				logging.Field{Key: "conflicts_detected", Value: len(proposalGraph.ConflictEdges)})

			// Send graph to arbiter for coordination
			arbiterStart := time.Now()

			// Build arbiter user message with graph JSON and fort metrics
			graphJSON, err := proposalGraph.ToJSON()
			if err != nil {
				al.logger.Error("failed to serialize graph", err)
			} else {
				arbiterPrompt := &llm.Prompt{
					SystemPrompt: getArbiterSystemPrompt(),
					UserMessage:  graphJSON,
					MaxTokens:    500,
					Temperature:  0.7,
				}

				al.logger.Debug("sending graph to arbiter",
					logging.Field{Key: "graph_json_size", Value: len(graphJSON)})

				arbiterResp, err := al.llmProvider.SendPrompt(al.ctx, arbiterPrompt)
				if err != nil {
					al.logger.Error("arbiter request failed", err)
					// TODO: Fallback to heuristic (highest priority non-conflicting proposal)
				} else {
					arbiterLatency := time.Since(arbiterStart)

					al.logger.Info("arbiter decision received",
						logging.Field{Key: "latency_ms", Value: arbiterLatency.Milliseconds()},
						logging.Field{Key: "tokens_prompt", Value: arbiterResp.TokensPrompt},
						logging.Field{Key: "tokens_completion", Value: arbiterResp.TokensCompletion})

					// Log full arbiter response for analysis
					al.logger.Debug("arbiter response", logging.Field{Key: "response", Value: arbiterResp.Text})

					// Parse arbiter JSON response
					var arbiterOutput struct {
						Sequence  []agents.ModificationNode `json:"sequence"`
						Synergies []string                  `json:"synergies"`
						Deferred  []string                  `json:"deferred"`
					}

					if err := json.Unmarshal([]byte(arbiterResp.Text), &arbiterOutput); err != nil {
						al.logger.Warn("failed to parse arbiter JSON, using fallback",
							logging.Field{Key: "error", Value: err.Error()})
						// TODO: Fallback to heuristic execution
					} else {
						// Create ArbitrationDecision from parsed output
						decision := &agents.ArbitrationDecision{
							ExecutionSequence: arbiterOutput.Sequence,
							InjectedNodes:     []agents.ModificationNode{}, // Filter synergies from sequence
							Timestamp:         time.Now(),
							Latency:           arbiterLatency,
							TokensUsed:        arbiterResp.TokensPrompt + arbiterResp.TokensCompletion,
						}

						al.logger.Info("arbiter decision parsed",
							logging.Field{Key: "execution_count", Value: len(decision.ExecutionSequence)},
							logging.Field{Key: "synergies_recognized", Value: len(arbiterOutput.Synergies)})

						// Execute via GraphExecutor
						commands, err := al.graphExecutor.Execute(decision)
						if err != nil {
							al.logger.Error("graph execution failed", err)
						} else if len(commands) > 0 {
							al.logger.Info("executing graph commands",
								logging.Field{Key: "command_count", Value: len(commands)})

							// Send commands to DFHack
							for i, cmd := range commands {
								cmd.CommandID = uint32(time.Now().UnixNano() + int64(i))
								if err := al.dfhackClient.SendCommand(&cmd); err != nil {
									al.logger.Error("failed to send command", err,
										logging.Field{Key: "index", Value: i})
								} else {
									al.logger.Info("command sent",
										logging.Field{Key: "command_id", Value: cmd.CommandID},
										logging.Field{Key: "type", Value: cmd.CommandType})
								}
							}
						}
					}
				}
			}
		} else {
			al.logger.Info("no agent proposals this cycle (all targets met)")
		}
	}

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

// SetAgentRegistry sets the goal agent registry and enables agent mode
func (al *AutonomousLoop) SetAgentRegistry(registry *agents.AgentRegistry, enabled bool) {
	al.agentRegistry = registry
	al.enableGoalAgents = enabled
	al.logger.Info("agent registry configured",
		logging.Field{Key: "enabled", Value: enabled},
		logging.Field{Key: "agent_count", Value: len(registry.List())})
}

// SetSVP sets the Spatial Validator Planner (Feature 007)
func (al *AutonomousLoop) SetSVP(svp *spatial.SpatialValidatorPlanner) {
	al.svp = svp
	al.svpState = SVPStateWaitingForTopology
	al.logger.Info("SVP configured - waiting for topology and entity data")
}

// OnTopologyReceived notifies the loop that topology (RESYNC) has been received (Feature 007)
func (al *AutonomousLoop) OnTopologyReceived() {
	if al.svp == nil {
		return
	}

	al.topologyReceived = true

	if al.svpState == SVPStateWaitingForTopology {
		al.svpState = SVPStateWaitingForEntities
		al.logger.Info("SVP: Topology received, waiting for entity data")
	}

	// Check if we can analyze now (if entities already received)
	al.tryAnalyzeSVP()
}

// OnEntitiesReceived notifies the loop that entities (ENTITY_UPDATE) have been received (Feature 007)
func (al *AutonomousLoop) OnEntitiesReceived() {
	if al.svp == nil {
		return
	}

	al.entitiesReceived = true

	// Check if we can analyze now (if topology already received)
	al.tryAnalyzeSVP()
}

// tryAnalyzeSVP triggers SVP analysis if both topology and entities are available
func (al *AutonomousLoop) tryAnalyzeSVP() {
	if al.svp == nil || al.svp.IsReady() {
		return // SVP not configured or already analyzed
	}

	if !al.topologyReceived || !al.entitiesReceived {
		return // Still waiting for data
	}

	// Both topology and entities available - trigger analysis
	al.logger.Info("SVP: Both topology and entities received, triggering terrain analysis")

	// Get entities for analysis
	entities := al.getEntities()
	if len(entities) == 0 {
		al.logger.Warn("SVP: No entities available for embark detection, deferring analysis")
		return
	}

	// Convert to EntityInfo pointers for SVP
	entityPtrs := make([]*protocol.EntityInfo, len(entities))
	for i := range entities {
		entityPtrs[i] = &entities[i]
	}

	// Perform SVP terrain analysis
	if err := al.svp.AnalyzeTerrain(al.topoOverlay, entityPtrs, al.hazardMgr); err != nil {
		al.logger.Error("SVP: Terrain analysis failed", err)
		return
	}

	// Save SVP layout to disk
	if err := al.svp.Save(); err != nil {
		al.logger.Warn("SVP: Failed to save layout",
			logging.Field{Key: "error", Value: err.Error()})
	}

	al.svpState = SVPStateReady
	al.logger.Info("SVP: Terrain analysis complete and saved")
}

// computeFortMetrics extracts fort metrics from current state
func (al *AutonomousLoop) computeFortMetrics() *agents.FortMetrics {
	metrics := &agents.FortMetrics{}

	// Get entities
	entities := al.getEntities()

	// Count dwarves and enemies
	for _, e := range entities {
		switch e.Type {
		case protocol.EntityTypeDwarf:
			metrics.DwarfCount++
		case protocol.EntityTypeEnemy:
			metrics.EnemyCount++
		}
	}

	// Calculate food per dwarf (placeholder - needs actual food tracking)
	// TODO: Add food stock tracking from game state
	metrics.FoodPerDwarf = 15.0 // Placeholder
	metrics.DrinkPerDwarf = 15.0 // Placeholder

	// Count bedrooms from modifications (chambers)
	// TODO: Improve this by tracking actual bedroom designations
	metrics.BedroomCount = 0 // Placeholder

	// Mining metrics (placeholder - needs cycle tracking)
	metrics.MiningTilesPerCycle = 0 // TODO: Track tiles dug per cycle
	metrics.NoStrikeCycles = 0      // TODO: Track cycles since ore discovery

	// Wealth metrics (placeholder)
	metrics.WealthGrowthRate = 0.05 // TODO: Track wealth growth
	metrics.TotalWealth = 0         // TODO: From fort info

	// Fort age from phase manager
	if al.phaseManager != nil {
		metrics.FortAge = al.phaseManager.GetDaysElapsed()
		metrics.Phase = al.phaseManager.GetPhase()
	}

	// Feature 007: Add SVP designations to metrics
	if al.svp != nil && al.svp.IsReady() {
		metrics.SVPHousingZ = al.svp.GetHousingZ()
		metrics.SVPWorkshopZ = al.svp.GetWorkshopZ()
		metrics.SVPFarmZ = al.svp.GetFarmZ()
	}

	return metrics
}

// analyzeAgents runs all enabled agents in parallel and collects proposals
func (al *AutonomousLoop) analyzeAgents(metrics *agents.FortMetrics) ([]agents.ModificationNode, time.Duration) {
	start := time.Now()

	if al.agentRegistry == nil {
		return nil, 0
	}

	enabledAgents := al.agentRegistry.GetEnabled()
	if len(enabledAgents) == 0 {
		return nil, 0
	}

	// Run agents in parallel
	type agentResult struct {
		agentName string
		proposals []agents.ModificationNode
		latency   time.Duration
	}

	results := make(chan agentResult, len(enabledAgents))
	var wg sync.WaitGroup

	for _, agent := range enabledAgents {
		wg.Add(1)
		go func(a agents.GoalAgent) {
			defer wg.Done()
			agentStart := time.Now()
			proposals := a.Analyze(metrics)
			latency := time.Since(agentStart)

			results <- agentResult{
				agentName: a.Name(),
				proposals: proposals,
				latency:   latency,
			}
		}(agent)
	}

	// Wait for all agents to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	allProposals := make([]agents.ModificationNode, 0)
	for result := range results {
		al.logger.Debug("agent analysis complete",
			logging.Field{Key: "agent", Value: result.agentName},
			logging.Field{Key: "proposals", Value: len(result.proposals)},
			logging.Field{Key: "latency_ms", Value: result.latency.Milliseconds()})

		allProposals = append(allProposals, result.proposals...)
	}

	totalLatency := time.Since(start)
	al.logger.Info("all agents analyzed",
		logging.Field{Key: "agent_count", Value: len(enabledAgents)},
		logging.Field{Key: "total_proposals", Value: len(allProposals)},
		logging.Field{Key: "latency_ms", Value: totalLatency.Milliseconds()})

	return allProposals, totalLatency
}

// getArbiterSystemPrompt returns the system prompt for graph arbiter
func getArbiterSystemPrompt() string {
	return `You are a Dwarf Fortress fort manager coordinating 5 specialized agents.

Agents propose modification nodes with spatial dependencies and conflicts.
Your task: Perform topological sort, detect synergies, resolve conflicts.

Node types: bedroom_cluster, mining_shaft, farm_plot, workshop_zone,
            corridor_connector, seal_entrance, exploratory_tunnel, defensive_wall,
            stair_cluster, stockpile_zone, gather_zone, production_area

Dependencies: requires_access_from (needs corridor), requires_water (needs well), requires_stairs (vertical access)
Conflicts: overlaps_spatially (same coordinates)

Output: JSON execution sequence (topologically sorted node list)

Recognize synergies: bedroom_cluster + dining_hall → inject corridor_connector
                     mining_shaft + bedrooms → inject stair_cluster

Output format:
{
  "sequence": [
    {"id": "corridor_1", "type": "corridor_connector", "region": {...}, "rationale": "Shared access"},
    {"id": "food_1", "rationale": "Higher priority"},
    {"id": "housing_1", "rationale": "Parallel with food"}
  ],
  "synergies": ["corridor_1"],
  "deferred": []
}

Prioritize: Food (10) > Housing (9) > Mining (7) > Wealth (6), Defense (variable 0-10)
Resolve conflicts by priority. Recognize spatial synergies when beneficial.`
}

// assembleProposalGraph creates a graph from agent proposals
func (al *AutonomousLoop) assembleProposalGraph(proposals []agents.ModificationNode) *agents.ProposalGraph {
	start := time.Now()

	graph := agents.NewProposalGraph()

	// Add all nodes to graph
	for _, proposal := range proposals {
		if err := graph.AddNode(proposal); err != nil {
			al.logger.Warn("failed to add node to graph",
				logging.Field{Key: "node_id", Value: proposal.ID},
				logging.Field{Key: "error", Value: err.Error()})
			continue
		}
	}

	// Detect spatial conflicts using AABB intersection
	graph.DetectSpatialConflicts()

	assemblyTime := time.Since(start)
	al.logger.Debug("proposal graph assembled",
		logging.Field{Key: "node_count", Value: len(graph.Nodes)},
		logging.Field{Key: "conflicts", Value: len(graph.ConflictEdges)},
		logging.Field{Key: "assembly_ms", Value: assemblyTime.Milliseconds()})

	return graph
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
