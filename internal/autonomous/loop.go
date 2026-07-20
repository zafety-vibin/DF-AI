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
	"github.com/df-ai/orchestrator/internal/blueprints"
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
	"github.com/df-ai/orchestrator/internal/zones"
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
	phaseManager        *phases.PhaseManager  // Fort development phases
	agentRegistry       *agents.AgentRegistry // Goal-oriented agents (Feature 006)
	enableGoalAgents    bool                  // Enable graph-based agents vs direct-LLM
	graphExecutor       *agents.GraphExecutor // Converts nodes to commands

	// Feature 007: Spatial Validator Planner
	svp              *spatial.SpatialValidatorPlanner // Z-level designations
	svpState         SVPState                         // Tracks SVP initialization
	topologyReceived bool                             // Whether RESYNC received
	entitiesReceived bool                             // Whether ENTITY_UPDATE received

	// Feature 007: Zone Extraction
	zoneExtractor *zones.ZoneExtractor // Zone data extractor
	cachedZones   []protocol.ZoneData  // Latest zone data from ENTITY_UPDATE
	zonesMu       sync.RWMutex         // Protects zone cache

	// Feature 007: Blueprint Integration
	blueprintMetadata []*blueprints.BlueprintMetadata // Available blueprint designs
	blueprintLib      *blueprints.BlueprintLibrary    // Blueprint library for expansion

	// Feature 007: Intent-based planning (HRM architecture)
	useIntentPlanning bool // Enable intent-based planning vs coordinate-based

	logFile *os.File
	mu      sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

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
		// CRITICAL: Block until SVP analyzes (no point running agents without it)
		if al.svp != nil && !al.svp.IsReady() {
			// Retry SVP analysis
			al.tryAnalyzeSVP()

			// If still not ready, skip this cycle
			if !al.svp.IsReady() {
				al.logger.Info("skipping cycle - waiting for SVP terrain analysis",
					logging.Field{Key: "topology_received", Value: al.topologyReceived},
					logging.Field{Key: "entities_received", Value: al.entitiesReceived})
				return nil
			}
		}

		// Compute fort metrics from current state
		metrics := al.computeFortMetrics()

		al.logger.Debug("computed fort metrics",
			logging.Field{Key: "dwarves", Value: metrics.DwarfCount},
			logging.Field{Key: "enemies", Value: metrics.EnemyCount},
			logging.Field{Key: "fort_age", Value: metrics.FortAge})

		// Branch: Intent-based flow vs coordinate-based flow
		if al.useIntentPlanning {
			// HRM Architecture: Intent-based planning (R011)
			intentProposals := al.collectIntentProposals(metrics)

			if len(intentProposals) == 0 {
				// NO FALLBACK - if agents don't propose, we FAIL LOUDLY
				al.logger.Error("NO AGENT PROPOSALS - SYSTEM HALTED",
					nil,
					logging.Field{Key: "dwarf_count", Value: metrics.DwarfCount},
					logging.Field{Key: "reason", Value: "Agents require dwarves to propose. Check entity detection in plugin."})
				return fmt.Errorf("agent system failure: dwarf_count=%d, cannot generate proposals", metrics.DwarfCount)
			}

			if len(intentProposals) > 0 {
				layout := al.svp.GetStrategicLayout()
				if layout == nil {
					al.logger.Warn("SVP layout not available for intent planning")
				} else {
					// Build arbiter input
					arbiterInput := al.buildArbiterIntentInput(layout, intentProposals)

					// Query arbiter
					arbiterPrompt := &llm.Prompt{
						SystemPrompt: al.getIntentArbiterSystemPrompt(),
						UserMessage:  arbiterInput,
						MaxTokens:    1000,
						Temperature:  0.7,
					}

					al.logger.Debug("sending intent proposals to arbiter",
						logging.Field{Key: "proposal_count", Value: len(intentProposals)},
						logging.Field{Key: "input_size", Value: len(arbiterInput)})

					arbiterResp, err := al.llmProvider.SendPrompt(al.ctx, arbiterPrompt)
					if err != nil {
						al.logger.Error("arbiter intent request failed", err)
					} else {
						al.logger.Info("arbiter intent response received",
							logging.Field{Key: "tokens_prompt", Value: arbiterResp.TokensPrompt},
							logging.Field{Key: "tokens_completion", Value: arbiterResp.TokensCompletion})

						// Log full arbiter response for debugging
						al.logger.Debug("arbiter intent response", logging.Field{Key: "response", Value: arbiterResp.Text})

						// Parse response
						response, err := al.parseArbiterIntentResponse(arbiterResp.Text)
						if err != nil {
							al.logger.Warn("failed to parse arbiter intent response",
								logging.Field{Key: "error", Value: err.Error()})
						} else {
							// Log strategic reasoning
							if response.Reasoning != "" {
								al.logger.Info("arbiter strategic reasoning",
									logging.Field{Key: "reasoning", Value: response.Reasoning})
							}

							// Convert to protocol commands
							commands := al.convertArbiterCommandsToProtocol(response.Commands)

							al.logger.Info("arbiter intent decision parsed",
								logging.Field{Key: "command_count", Value: len(commands)},
								logging.Field{Key: "deferred_count", Value: len(response.Deferred)})

							// Send commands to DFHack
							for i, cmd := range commands {
								cmd.CommandID = uint32(time.Now().UnixNano() + int64(i))
								if err := al.dfhackClient.SendCommand(&cmd); err != nil {
									al.logger.Error("failed to send command", err)
								} else {
									al.logger.Info("blueprint command sent",
										logging.Field{Key: "command_id", Value: cmd.CommandID},
										logging.Field{Key: "blueprint", Value: cmd.BlueprintName})
								}
							}

							// Intent-based commands sent - skip legacy LLM
							al.logger.Info("intent-based cycle complete",
								logging.Field{Key: "commands_sent", Value: len(commands)})
							return nil
						}
					}
				}
			} else {
				al.logger.Info("no intent proposals this cycle (all targets met)")
				return nil // No work needed - skip legacy LLM
			}
		} else {
			// Existing coordinate-based flow
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
						SystemPrompt: al.getArbiterSystemPrompt(),
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

							// Feature 007: Log blueprint selections (T074)
							blueprintCount := 0
							for _, node := range decision.ExecutionSequence {
								if node.Metadata != nil {
									if blueprintName, ok := node.Metadata["blueprint_used"].(string); ok && blueprintName != "" {
										blueprintCount++
										al.logger.Info("arbiter selected blueprint",
											logging.Field{Key: "node_id", Value: node.ID},
											logging.Field{Key: "blueprint", Value: blueprintName},
											logging.Field{Key: "rationale", Value: node.Rationale})
									}
								}
							}

							al.logger.Info("arbiter decision parsed",
								logging.Field{Key: "execution_count", Value: len(decision.ExecutionSequence)},
								logging.Field{Key: "synergies_recognized", Value: len(arbiterOutput.Synergies)},
								logging.Field{Key: "blueprints_used", Value: blueprintCount})

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

// abs returns absolute value of int
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// assembleContext assembles fort state context
// Uses smart assembly: first turn shows terrain, later turns show modifications + rooms
func (al *AutonomousLoop) assembleContext() (string, error) {
	// Get entities from cached entities (updated via ENTITY_UPDATE)
	entities := al.getEntities()

	// DEBUG: Log entity count and types
	dwarfCount := 0
	typeMap := make(map[uint8]int)
	for _, e := range entities {
		typeMap[e.Type]++
		if e.Type == protocol.EntityTypeDwarf {
			dwarfCount++
		}
	}
	al.logger.Info("assembling context",
		logging.Field{Key: "total_entities", Value: len(entities)},
		logging.Field{Key: "dwarf_count", Value: dwarfCount},
		logging.Field{Key: "entity_types", Value: typeMap})

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

	// Deduplicate commands - remove exact duplicates by region
	deduped := make([]llm.CommandSpec, 0, len(specs))
	seen := make(map[string]bool)
	duplicates := 0

	for _, spec := range specs {
		// Create region key for dig commands
		if spec.Type == "dig" || spec.Type == "cancel" {
			key := fmt.Sprintf("%s_%d_%d_%d_%d_%d_%d",
				spec.Type, spec.Region.X1, spec.Region.Y1, spec.Region.Z,
				spec.Region.X2, spec.Region.Y2, spec.Region.Z2)
			if !seen[key] {
				seen[key] = true
				deduped = append(deduped, spec)
			} else {
				duplicates++
			}
		} else {
			deduped = append(deduped, spec)
		}
	}

	if duplicates > 0 {
		al.logger.Warn("removed duplicate commands",
			logging.Field{Key: "duplicates_removed", Value: duplicates},
			logging.Field{Key: "original_count", Value: len(specs)},
			logging.Field{Key: "deduped_count", Value: len(deduped)})
		specs = deduped
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

// executeDig executes a dig command (handles Z-ranges for vertical shafts)
func (al *AutonomousLoop) executeDig(spec llm.CommandSpec) (uint32, error) {
	region := spec.Region

	// Map dig type string to protocol constant
	digType := mapDigType(spec.DigType)

	// Determine Z-range (Z2 if set, otherwise use Z)
	z1 := int16(region.Z)
	z2 := z1
	if region.Z2 > 0 && region.Z2 != region.Z {
		z2 = int16(region.Z2)
	}

	// For stairs across Z-levels, dig at each level
	isStairCommand := spec.DigType == "stairs" || spec.DigType == "updownstairs"
	if isStairCommand && z1 != z2 {
		al.logger.Info("executing vertical shaft",
			logging.Field{Key: "region", Value: fmt.Sprintf("(%d,%d,%d) to (%d,%d,%d)",
				region.X1, region.Y1, z1, region.X2, region.Y2, z2)},
			logging.Field{Key: "dig_type", Value: spec.DigType},
			logging.Field{Key: "z_levels", Value: abs(int(z2-z1)) + 1})

		// Dig stairs at each Z-level from z1 to z2
		var lastCmdID uint32
		zStart, zEnd := z1, z2
		if z1 > z2 {
			zStart, zEnd = z2, z1 // Swap if going down
		}

		for z := zStart; z <= zEnd; z++ {
			result, err := al.commandExecutor.SendDigCommand(
				digType,
				int16(region.X1),
				int16(region.Y1),
				z,
				int16(region.X2),
				int16(region.Y2),
			)
			if err != nil {
				al.logger.Warn("stair level failed",
					logging.Field{Key: "z", Value: z},
					logging.Field{Key: "error", Value: err.Error()})
				continue
			}
			if result.Success {
				lastCmdID = result.Response.CommandID
			}
		}

		return lastCmdID, nil
	}

	// Single Z-level dig
	al.logger.Info("executing dig command",
		logging.Field{Key: "region", Value: fmt.Sprintf("(%d,%d,%d) to (%d,%d,%d)",
			region.X1, region.Y1, z1, region.X2, region.Y2, z1)},
		logging.Field{Key: "dig_type", Value: spec.DigType})

	// VALIDATION: For non-stair dig commands, check if region has any walls to dig
	// Stairs can start on open tiles (surface level), so only validate default/channel digs
	if al.topoOverlay != nil && digType == protocol.DigTypeDefault {
		totalTiles := 0
		wallTiles := 0 // Closed tiles that CAN be dug
		for x := region.X1; x <= region.X2; x++ {
			for y := region.Y1; y <= region.Y2; y++ {
				isOpen, err := al.topoOverlay.GetTile(int16(x), int16(y), z1)
				if err == nil {
					totalTiles++
					if !isOpen {
						wallTiles++ // Closed tile = diggable
					}
				}
			}
		}
		// Reject if <20% of tiles are walls (i.e., >80% already open/floor)
		// This prevents digging areas that are already excavated
		if totalTiles > 0 && float64(wallTiles)/float64(totalTiles) < 0.2 {
			al.logger.Warn("dig command rejected - region has no walls to dig",
				logging.Field{Key: "wall_pct", Value: fmt.Sprintf("%.1f%%", 100.0*float64(wallTiles)/float64(totalTiles))},
				logging.Field{Key: "wall_tiles", Value: wallTiles},
				logging.Field{Key: "total_tiles", Value: totalTiles})
			return 0, fmt.Errorf("cannot dig - region has only %.1f%% walls (already excavated)", 100.0*float64(wallTiles)/float64(totalTiles))
		}
	}

	result, err := al.commandExecutor.SendDigCommand(
		digType,
		int16(region.X1),
		int16(region.Y1),
		z1,
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
		X1: int16(region.X1), Y1: int16(region.Y1), Z1: z1,
		X2: int16(region.X2), Y2: int16(region.Y2), Z2: z1,
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

// SetZoneExtractor sets the zone extractor (Feature 007)
func (al *AutonomousLoop) SetZoneExtractor(extractor *zones.ZoneExtractor) {
	al.zoneExtractor = extractor
	al.logger.Info("zone extractor configured",
		logging.Field{Key: "enabled", Value: extractor.IsEnabled()})
}

// SetBlueprintMetadata configures blueprint metadata for arbiter prompt
func (al *AutonomousLoop) SetBlueprintMetadata(metadata []*blueprints.BlueprintMetadata) {
	al.blueprintMetadata = metadata
	al.logger.Info("blueprint metadata loaded",
		logging.Field{Key: "blueprint_count", Value: len(metadata)})
}

// SetBlueprintLibrary configures the blueprint library for expansion
func (al *AutonomousLoop) SetBlueprintLibrary(lib *blueprints.BlueprintLibrary) {
	al.blueprintLib = lib
	al.logger.Info("blueprint library configured for expansion",
		logging.Field{Key: "blueprint_count", Value: len(lib.ListBlueprints())})
}

// SetUseIntentPlanning enables or disables intent-based planning (HRM architecture)
func (al *AutonomousLoop) SetUseIntentPlanning(enabled bool) {
	al.useIntentPlanning = enabled
	if enabled {
		al.logger.Info("intent-based planning enabled (HRM architecture)")
	} else {
		al.logger.Info("coordinate-based planning enabled (legacy mode)")
	}
}

// UpdateZones updates the cached zone data from ENTITY_UPDATE (Feature 007)
func (al *AutonomousLoop) UpdateZones(zoneData []protocol.ZoneData) {
	al.zonesMu.Lock()
	defer al.zonesMu.Unlock()
	al.cachedZones = zoneData
}

// getZones returns a copy of cached zones (thread-safe)
func (al *AutonomousLoop) getZones() []protocol.ZoneData {
	al.zonesMu.RLock()
	defer al.zonesMu.RUnlock()
	zonesCopy := make([]protocol.ZoneData, len(al.cachedZones))
	copy(zonesCopy, al.cachedZones)
	return zonesCopy
}

// OnTopologyReceived notifies the loop that topology (RESYNC) has been received (Feature 007)
func (al *AutonomousLoop) OnTopologyReceived() {
	if al.svp == nil {
		return
	}

	al.topologyReceived = true

	// If SVP was loaded from disk, rebuild StrategicLayout now that topology is available
	if al.svp.IsReady() && al.topoOverlay != nil && al.hazardMgr != nil {
		al.svp.RebuildStrategicLayout(al.topoOverlay, al.hazardMgr)
	}
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
	metrics.FoodPerDwarf = 15.0  // Placeholder
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

	// Feature 007: Extract zone counts from DF
	if al.zoneExtractor != nil && al.zoneExtractor.IsEnabled() {
		zoneData := al.getZones()
		extractedZones, err := al.zoneExtractor.ExtractZones(zoneData)

		if err == nil && len(extractedZones) > 0 {
			// Count zones by type
			zoneCounts := al.zoneExtractor.CountByType(extractedZones)
			metrics.BedroomZoneCount = zoneCounts[zones.ZoneTypeBedroom]
			metrics.DiningZoneCount = zoneCounts[zones.ZoneTypeDining]
			metrics.DormitoryZoneCount = zoneCounts[zones.ZoneTypeDormitory]
			metrics.OfficeZoneCount = zoneCounts[zones.ZoneTypeOffice]

			// Count unassigned bedrooms
			metrics.UnassignedBedroomCount = al.zoneExtractor.GetUnassignedCount(extractedZones, zones.ZoneTypeBedroom)

			// Calculate housing deficit
			metrics.HousingDeficit = metrics.DwarfCount - metrics.BedroomZoneCount

			al.logger.Debug("zone metrics computed",
				logging.Field{Key: "bedroom_zones", Value: metrics.BedroomZoneCount},
				logging.Field{Key: "dining_zones", Value: metrics.DiningZoneCount},
				logging.Field{Key: "housing_deficit", Value: metrics.HousingDeficit})

			// HRM Architecture: Update StrategicLayout with zone counts (R006)
			if al.svp != nil && al.svp.IsReady() {
				zonesByZ := al.zoneExtractor.GetZonesByZLevel(extractedZones)
				al.svp.UpdateStrategicLayoutWithZones(zonesByZ)
			}
		} else {
			if err != nil {
				al.logger.Warn("zone extraction failed, using fallback",
					logging.Field{Key: "error", Value: err.Error()})
			}

			// Fallback to chamber count estimation (Feature 005 behavior)
			// BedroomCount remains 0 (placeholder)
			metrics.BedroomZoneCount = 0
			metrics.HousingDeficit = metrics.DwarfCount
		}
	} else {
		// Zone extraction disabled - use placeholder
		metrics.BedroomZoneCount = 0
		metrics.HousingDeficit = metrics.DwarfCount
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

// getArbiterSystemPrompt returns the system prompt for graph arbiter (Feature 007: includes blueprint metadata)
func (al *AutonomousLoop) getArbiterSystemPrompt() string {
	basePrompt := `You are a Dwarf Fortress fort manager coordinating 5 specialized agents.

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

`

	// Feature 007: Add blueprint library if available (T066-T067)
	if len(al.blueprintMetadata) > 0 {
		basePrompt += blueprints.GetMetadataPrompt(al.blueprintMetadata)
		basePrompt += "\nBlueprint Selection:\n"
		basePrompt += "- When proposing bedrooms/dining halls, SELECT a blueprint that fits the available space\n"
		basePrompt += "- Include \"blueprint_used\": \"<name>\" in the node metadata when using a blueprint\n"
		basePrompt += "- Use geometric patterns (arbitrary rectangles) only when blueprints don't fit or for non-bedroom structures\n"
		basePrompt += "- Prefer proven designs (blueprints) over ad-hoc patterns for housing\n\n"
	}

	basePrompt += `Output format:
{
  "sequence": [
    {"id": "corridor_1", "type": "corridor_connector", "region": {...}, "rationale": "Shared access"},
    {"id": "food_1", "rationale": "Higher priority", "metadata": {"blueprint_used": "bedroom_cluster_10"}},
    {"id": "housing_1", "rationale": "Parallel with food"}
  ],
  "synergies": ["corridor_1"],
  "deferred": []
}

Prioritize: Food (10) > Housing (9) > Mining (7) > Wealth (6), Defense (variable 0-10)
Resolve conflicts by priority. Recognize spatial synergies when beneficial.`

	return basePrompt
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

// collectIntentProposals runs agents in intent mode (R008)
func (al *AutonomousLoop) collectIntentProposals(metrics *agents.FortMetrics) []agents.IntentProposal {
	proposals := make([]agents.IntentProposal, 0)

	// Currently only HousingAgent supports intent mode
	if housing, ok := al.agentRegistry.Get("Housing"); ok {
		if housingAgent, ok := housing.(*agents.HousingAgent); ok {
			intents := housingAgent.AnalyzeIntent(metrics)
			proposals = append(proposals, intents...)
		}
	}

	return proposals
}

// buildArbiterIntentInput constructs JSON input for intent-based arbiter (R009)
func (al *AutonomousLoop) buildArbiterIntentInput(
	layout *spatial.StrategicLayout,
	proposals []agents.IntentProposal,
) string {
	input := map[string]interface{}{
		"fort_layout": layout,
		"proposals":   proposals,
		"blueprints":  al.blueprintMetadata,
	}

	jsonBytes, _ := json.MarshalIndent(input, "", "  ")
	return string(jsonBytes)
}

// getIntentArbiterSystemPrompt returns prompt for intent-based planning (R008)
func (al *AutonomousLoop) getIntentArbiterSystemPrompt() string {
	return `You are a Dwarf Fortress spatial planning arbiter (HRM H-module).

You receive:
1. StrategicLayout JSON: Available construction regions per Z-level with existing infrastructure counts
2. Agent Intent Proposals: High-level needs WITHOUT coordinates (quantity, purpose, constraints)
3. Blueprint Library: Proven spatial designs

Your task: Map intents → blueprint placements with anchor points

Input Format:
{
  "fort_layout": {
    "layers": {
      "housing": {
        "z_level": 95,
        "regions": [{"id": "housing_1", "bbox": [40,30,95,60,45,95], "area": 300, "status": "available"}],
        "existing_zones": {"bedroom": 5, "dining": 1}
      },
      "workshop": {"z_level": 94, "regions": [...]}
    }
  },
  "proposals": [
    {
      "agent": "Housing",
      "intent": "provide_housing",
      "purpose": "housing",
      "quantity": 7,
      "blueprint_hint": "bedroom_80r4t_37x36_9scenter_simple",
      "constraints": ["safe_layer"]
    }
  ],
  "blueprints": [
    {"name": "bedroom_80r4t_37x36_9scenter_simple", "width": 35, "height": 35, "capacity": 80},
    {"name": "bedroom_100r4t_48x49_9scenter_vherid", "width": 45, "height": 49, "capacity": 100}
  ]
}

Output Format:
{
  "reasoning": "Prioritizing food security first (farm plots on soil layers 111-110), then addressing housing deficit on safe layer 106. Blueprint placement optimized for available space.",
  "commands": [
    {
      "type": "apply_blueprint",
      "blueprint": "bedroom_80r4t_37x36_9scenter_simple",
      "anchor": [42, 31, 106],
      "rotation": 0,
      "reasoning": "Housing layer has 29584 tiles available. Simple design (35x35) fits within bounds. Addresses 18-bedroom deficit."
    }
  ],
  "deferred": []
}

Decision Rules:
1. Check region.area >= (blueprint.width * blueprint.height) before placement
2. Respect layer purposes (housing blueprints only on housing layers)
3. Avoid overlapping existing zones
4. Use blueprint_hint when provided
5. Prefer blueprints over geometric patterns
6. If blueprint doesn't fit, use dig command fallback (see below)

FALLBACK: Dig Commands (when blueprints unavailable or don't fit)
When blueprints cannot be used, generate dig commands instead:

Dig Command Format:
{
  "type": "dig",
  "region": {"x1": 40, "y1": 30, "z": 106, "x2": 60, "y2": 50, "z2": 106},
  "dig_type": "default",
  "reasoning": "Creating entrance tunnel on housing layer"
}

Dig Types:
- "default": Standard mining (removes walls, creates floors)
- "channel": Removes floor, creates ramps down
- "ramp": Carves ramps in walls
- "updown_stair": Vertical shaft (z != z2)

CRITICAL: Use Topology Data for Dig Commands
- The fort_layout includes "topology_slice" showing WALL vs OPEN tiles
- ONLY dig regions with significant WALL tiles (>20% walls = diggable)
- DO NOT dig regions that are already OPEN/FLOOR (<20% walls = already excavated)
- Check topology_slice.description for guidance like "60% WALL/ROCK (CAN DIG)"

Dig Command Strategy:
1. For housing needs: Dig rectangular rooms 5x5 to 10x10
2. For access: Dig 3-wide corridors to connect areas
3. For vertical access: Dig updown_stair shafts (single column, z != z2)
4. Check topology FIRST - don't dig already-open areas

Example dig command for housing:
{"type":"dig","region":{"x1":40,"y1":30,"z":106,"x2":50,"y2":40,"z2":106},"dig_type":"default","reasoning":"Creating 11x11 bedroom area, topology shows 85% walls"}

CRITICAL: Staircase Connectivity Rules
Staircases connect Z-levels vertically. Without proper anchoring, levels become ISOLATED!

Staircase Anchoring Algorithm:
1. Check fort_layout.layers[purpose].existing_stairs for current staircase positions
2. If existing stairs found on target layer:
   a) Calculate anchor that ALIGNS blueprint stair with existing stair
   b) anchor_x = existing_stair.x - blueprint.stair_locations[0].x
   c) anchor_y = existing_stair.y - blueprint.stair_locations[0].y
   d) This ensures stairs stack vertically (perfect (x,y) alignment)
3. If no existing stairs on layer:
   a) Place blueprint freely (its stair becomes the vertical shaft anchor)
   b) NOTE: Future blueprints on adjacent Z-levels MUST align to this stair
4. Verify connectivity:
   - Blueprint with up_stair requires matching down_stair on Z+1
   - Blueprint with down_stair requires matching up_stair on Z-1
   - If mismatch detected, DEFER the proposal

Anchoring Example:
  Existing stair: (50, 50, 95) type="updown"
  Blueprint stair offset: (9, 6) type="updown"
  Calculated anchor: (50-9, 50-6, 95) = (41, 44, 95)
  Verification: Blueprint stair will be at (41+9, 44+6) = (50, 50) ✓ Aligned!

Priority: Connectivity > Optimal placement
If aligning to stair forces blueprint outside region bounds, DEFER proposal

Priority: Food (10) > Housing (9) > Mining (7) > Wealth (6) > Defense (variable)

CRITICAL OUTPUT FORMAT REQUIREMENTS:
- You MUST output ONLY valid JSON
- Do NOT include any text before the JSON
- Do NOT include any text after the JSON
- Do NOT include markdown code blocks
- Start your response directly with the opening brace {
- End your response directly with the closing brace }
- The entire response must be parseable by json.Unmarshal()

Example CORRECT outputs:

Blueprint command:
{"reasoning":"Addressing housing shortage with blueprint","commands":[{"type":"apply_blueprint","blueprint":"bedroom_30r25t_67x26_9scenter_ribbon","anchor":[42,31,95],"rotation":0,"reasoning":"Placed on housing layer"}],"deferred":[]}

Dig command (fallback when blueprint unavailable):
{"reasoning":"Creating housing area with manual dig","commands":[{"type":"dig","region":{"x1":40,"y1":30,"z":106,"x2":55,"y2":45,"z2":106},"dig_type":"default","reasoning":"16x16 bedroom area, topology shows 78% walls"}],"deferred":[]}

Example INCORRECT outputs that will FAIL:
- Any text before or after the JSON object
- Markdown code fences around the JSON
- Multiple JSON objects`
}

// parseArbiterIntentResponse parses arbiter's blueprint+anchor commands (R010)
func (al *AutonomousLoop) parseArbiterIntentResponse(responseText string) (*agents.ArbiterIntentResponse, error) {
	var response agents.ArbiterIntentResponse

	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		return nil, fmt.Errorf("failed to parse arbiter intent response: %w", err)
	}

	return &response, nil
}

// convertArbiterCommandsToProtocol converts arbiter commands to protocol messages (R011)
func (al *AutonomousLoop) convertArbiterCommandsToProtocol(commands []agents.ArbiterCommand) []protocol.CommandMessage {
	protocolCommands := make([]protocol.CommandMessage, 0, len(commands))

	for _, cmd := range commands {
		if cmd.Type == "apply_blueprint" {
			// WORKAROUND: Blueprint command has thread safety issues in DFHack plugin
			// Instead, expand blueprint into individual dig commands here and send those
			if al.blueprintLib != nil {
				bp := al.blueprintLib.GetBlueprint(cmd.Blueprint)
				if bp != nil {
					al.logger.Info("expanding blueprint into dig commands",
						logging.Field{Key: "blueprint", Value: cmd.Blueprint},
						logging.Field{Key: "anchor", Value: cmd.Anchor},
						logging.Field{Key: "dig_count", Value: len(bp.Digs)})

					// Convert blueprint to dig commands
					origin := modifications.Coordinate{
						X: int16(cmd.Anchor[0]),
						Y: int16(cmd.Anchor[1]),
						Z: int16(cmd.Anchor[2]),
					}
					digCommands := bp.ApplyAt(origin)

					// Group consecutive digs into regions for efficiency
					// For now, send each as individual tile (can optimize later)
					for _, dig := range digCommands {
						var digType uint8
						switch dig.DigType {
						case "default", "d":
							digType = protocol.DigTypeDefault
						case "channel", "h":
							digType = protocol.DigTypeChannel
						case "ramp", "r":
							digType = protocol.DigTypeRamp
						case "updown_stair", "i", "updownstair":
							digType = protocol.DigTypeUpDownStair
						case "down_stair", "j", "downstair":
							digType = protocol.DigTypeDownStair
						case "up_stair", "u", "upstair":
							digType = protocol.DigTypeUpStair
						default:
							digType = protocol.DigTypeDefault
						}

						protocolCommands = append(protocolCommands, protocol.CommandMessage{
							CommandType: protocol.CommandTypeDig,
							DigType:     digType,
							Region: protocol.Region{
								X1: dig.X,
								Y1: dig.Y,
								Z1: dig.Z,
								X2: dig.X, // Single tile
								Y2: dig.Y,
								Z2: dig.Z,
							},
						})
					}

					al.logger.Info("blueprint expanded to dig commands",
						logging.Field{Key: "blueprint", Value: cmd.Blueprint},
						logging.Field{Key: "commands_generated", Value: len(digCommands)},
						logging.Field{Key: "reasoning", Value: cmd.Reasoning})
					continue
				} else {
					al.logger.Warn("blueprint not found in library",
						logging.Field{Key: "blueprint", Value: cmd.Blueprint})
				}
			}

			// Fallback: send as blueprint command (will fail but logged)
			protocolCommands = append(protocolCommands, protocol.CommandMessage{
				CommandType:   protocol.CommandTypeBlueprint,
				BlueprintName: cmd.Blueprint,
				OriginX:       int16(cmd.Anchor[0]),
				OriginY:       int16(cmd.Anchor[1]),
				OriginZ:       int16(cmd.Anchor[2]),
			})

			al.logger.Warn("sending blueprint command (no library available)",
				logging.Field{Key: "blueprint", Value: cmd.Blueprint},
				logging.Field{Key: "anchor", Value: cmd.Anchor},
				logging.Field{Key: "reasoning", Value: cmd.Reasoning})
		} else if cmd.Type == "dig" && cmd.Region != nil {
			// Map dig_type string to protocol constant
			var digType uint8
			switch cmd.DigType {
			case "default":
				digType = protocol.DigTypeDefault
			case "channel":
				digType = protocol.DigTypeChannel
			case "ramp":
				digType = protocol.DigTypeRamp
			case "updown_stair":
				digType = protocol.DigTypeUpDownStair
			case "down_stair":
				digType = protocol.DigTypeDownStair
			case "up_stair":
				digType = protocol.DigTypeUpStair
			default:
				digType = protocol.DigTypeDefault
			}

			protocolCommands = append(protocolCommands, protocol.CommandMessage{
				CommandType: protocol.CommandTypeDig,
				DigType:     digType,
				Region: protocol.Region{
					X1: int16(cmd.Region.X1),
					Y1: int16(cmd.Region.Y1),
					Z1: int16(cmd.Region.Z),
					X2: int16(cmd.Region.X2),
					Y2: int16(cmd.Region.Y2),
					Z2: int16(cmd.Region.Z2),
				},
			})

			al.logger.Info("arbiter dig command",
				logging.Field{Key: "region", Value: cmd.Region},
				logging.Field{Key: "dig_type", Value: cmd.DigType},
				logging.Field{Key: "reasoning", Value: cmd.Reasoning})
		}
	}

	return protocolCommands
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
