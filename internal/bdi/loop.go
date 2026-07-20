package bdi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/llm"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/plan"
	"github.com/df-ai/orchestrator/internal/predicate"
	"github.com/df-ai/orchestrator/internal/query"
	"github.com/df-ai/orchestrator/internal/reconcile"
	"github.com/df-ai/orchestrator/internal/skill"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// Config bundles parameters for one Loop instance.
type Config struct {
	Cadence             time.Duration // deliberator wakeup interval
	MinDwarvesToStart   int           // skip cycles until at least this many dwarves are visible
	MaxTokens           int           // LLM response budget
	Temperature         float64
	LogPath             string // empty → no file logging
	WaitOnEmptySnapshot bool   // skip cycle when no entity data yet
	MaxPasses           int    // multi-pass deliberator bound (0 → default 5)
}

// DefaultConfig returns reasonable defaults for early experimentation.
func DefaultConfig() Config {
	return Config{
		Cadence:             45 * time.Second,
		MinDwarvesToStart:   1,
		MaxTokens:           2048,
		Temperature:         0.7,
		LogPath:             filepath.Join("logs", "bdi-interactions.jsonl"),
		WaitOnEmptySnapshot: true,
		MaxPasses:           5,
	}
}

// Loop wires the four BDI goroutines together: populator (perception),
// deliberator, executor, reconciler. Construct with NewLoop, Start with
// the parent context, Stop to drain.
type Loop struct {
	cfg        Config
	wm         *worldmodel.WorldModel
	populator  *worldmodel.Populator
	dag        *plan.DAG
	executor   *plan.Executor
	reconciler *reconcile.Reconciler
	llm        llm.Provider
	library    *predicate.Library
	registry   *query.Registry
	skills     *skill.Library         // optional; nil → no skill catalog injected
	plugin     query.PluginDispatcher // optional; nil → Tier 2 queries error out
	logger     *logging.Logger

	mu      sync.Mutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	logFile *os.File

	turnCounter uint64
}

// NewLoop wires the dependencies together. Mandatory: wm, populator, dag,
// executor, reconciler, provider. library, registry, and plugin may be
// nil (each falls back to a no-op variant).
func NewLoop(
	cfg Config,
	wm *worldmodel.WorldModel,
	populator *worldmodel.Populator,
	dag *plan.DAG,
	executor *plan.Executor,
	reconciler *reconcile.Reconciler,
	provider llm.Provider,
	library *predicate.Library,
	registry *query.Registry,
	skills *skill.Library,
	plugin query.PluginDispatcher,
	logger *logging.Logger,
) (*Loop, error) {
	if wm == nil {
		return nil, fmt.Errorf("worldmodel is required")
	}
	if populator == nil {
		return nil, fmt.Errorf("populator is required")
	}
	if dag == nil {
		return nil, fmt.Errorf("plan dag is required")
	}
	if executor == nil {
		return nil, fmt.Errorf("plan executor is required")
	}
	if reconciler == nil {
		return nil, fmt.Errorf("reconciler is required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider is required")
	}
	if cfg.Cadence <= 0 {
		cfg.Cadence = DefaultConfig().Cadence
	}
	if cfg.MaxPasses <= 0 {
		cfg.MaxPasses = DefaultConfig().MaxPasses
	}
	if library == nil {
		library = predicate.NewLibrary()
	}
	if registry == nil {
		registry = query.NewRegistry()
	}

	l := &Loop{
		cfg:        cfg,
		wm:         wm,
		populator:  populator,
		dag:        dag,
		executor:   executor,
		reconciler: reconciler,
		llm:        provider,
		library:    library,
		registry:   registry,
		skills:     skills,
		plugin:     plugin,
		logger:     logger,
	}

	if cfg.LogPath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.LogPath), 0o755); err == nil {
			f, err := os.OpenFile(cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err == nil {
				l.logFile = f
			} else if logger != nil {
				logger.Warn("bdi: failed to open log file",
					logging.Field{Key: "path", Value: cfg.LogPath},
					logging.Field{Key: "error", Value: err.Error()})
			}
		}
	}

	// Make the DAG visible to anyone reading the world model.
	wm.SetPlanReader(dag)

	return l, nil
}

// Start kicks off all four background goroutines. Idempotent.
func (l *Loop) Start(parent context.Context) {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return
	}
	l.ctx, l.cancel = context.WithCancel(parent)
	l.running = true
	l.mu.Unlock()

	if l.logger != nil {
		l.logger.Info("bdi: starting",
			logging.Field{Key: "cadence", Value: l.cfg.Cadence.String()},
			logging.Field{Key: "model", Value: l.llm.GetModelName()},
			logging.Field{Key: "provider", Value: l.llm.GetProviderType()})
	}

	l.wg.Add(4)
	go func() { defer l.wg.Done(); l.populator.Run(l.ctx) }()
	go func() { defer l.wg.Done(); l.executor.Run(l.ctx) }()
	go func() { defer l.wg.Done(); l.reconciler.Run(l.ctx) }()
	go func() { defer l.wg.Done(); l.runDeliberator() }()
}

// Stop cancels the loop and waits for all goroutines to drain.
func (l *Loop) Stop() {
	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()
		return
	}
	l.cancel()
	l.running = false
	l.mu.Unlock()

	l.wg.Wait()

	if l.logFile != nil {
		_ = l.logFile.Close()
	}
	if l.logger != nil {
		l.logger.Info("bdi: stopped")
	}
}

// runDeliberator wakes on cadence and runs one decision cycle. Skips cycles
// where the snapshot has no observed dwarves yet (waiting for the first
// ENTITY_UPDATE) when WaitOnEmptySnapshot is set.
func (l *Loop) runDeliberator() {
	// Initial settle delay so the FULL_STATE handshake has time to
	// complete before we ask the LLM to look at an empty world.
	select {
	case <-l.ctx.Done():
		return
	case <-time.After(5 * time.Second):
	}

	t := time.NewTicker(l.cfg.Cadence)
	defer t.Stop()

	l.runCycle()

	for {
		select {
		case <-l.ctx.Done():
			return
		case <-t.C:
			l.runCycle()
		}
	}
}

// runCycle is one full deliberator turn. May make up to MaxPasses LLM
// calls: each pass either dispatches REQUEST queries (and recalls with
// the results) or commits action commands and ends the turn. If
// neither is emitted in a pass, the turn ends with a warning.
//
// Layout per turn:
//
//	pass 1: send initial prompt (snapshot + tool catalog) → parse
//	pass 2..N: append previous LLM response + tool results, send → parse
//	end: commit emitted commands; log turn-level summary.
func (l *Loop) runCycle() {
	snap := l.wm.Snapshot()

	if l.cfg.WaitOnEmptySnapshot && len(snap.Entities.Dwarves) < l.cfg.MinDwarvesToStart {
		l.debug("bdi: cycle skipped — waiting for dwarves",
			logging.Field{Key: "dwarves", Value: len(snap.Entities.Dwarves)})
		return
	}

	turn := l.nextTurn()
	cycleStart := time.Now()

	predResults := l.library.CheckAll(l.wm)

	skillCatalog := ""
	if l.skills != nil {
		skillCatalog = l.skills.RenderForPrompt()
	}
	in := RenderInput{
		WM:                l.wm,
		Predicates:        predResults,
		PlanStats:         l.dag.Stats(),
		RecentNodes:       l.dag.Recent(8),
		RecentDivergences: l.reconciler.Events().Recent(5),
		ToolCatalog:       l.registry.CatalogMarkdown(),
		SkillCatalog:      skillCatalog,
	}
	initialMsg := Render(in, DefaultRenderOptions())

	l.info("bdi: deliberating",
		logging.Field{Key: "turn", Value: turn},
		logging.Field{Key: "predicates_satisfied", Value: predicate.SatisfiedCount(predResults)},
		logging.Field{Key: "predicates_total", Value: len(predResults)},
		logging.Field{Key: "plan_active", Value: in.PlanStats.Active},
		logging.Field{Key: "plan_done", Value: in.PlanStats.Done},
		logging.Field{Key: "recent_divergences", Value: len(in.RecentDivergences)})

	qctx := query.Context{
		Ctx:    l.ctx,
		WM:     l.wm,
		DAG:    l.dag,
		Lib:    l.library,
		Events: l.reconciler.Events(),
		Plugin: l.plugin,
	}

	history := []llm.Message{}
	currentMsg := initialMsg

	var (
		totalTokensPrompt     int
		totalTokensCompletion int
		totalLatencyMs        int64
		totalRequests         int
		committed             int
		finalRespText         string
		stopReason            string
	)

	maxPasses := l.cfg.MaxPasses
	if maxPasses <= 0 {
		maxPasses = 5
	}

PASSES:
	for pass := 1; pass <= maxPasses; pass++ {
		passStart := time.Now()
		prompt := &llm.Prompt{
			SystemPrompt: SystemPrompt,
			UserMessage:  currentMsg,
			History:      history,
			MaxTokens:    l.cfg.MaxTokens,
			Temperature:  l.cfg.Temperature,
		}

		resp, err := l.llm.SendPrompt(l.ctx, prompt)
		if err != nil {
			l.warn("bdi: llm call failed",
				logging.Field{Key: "turn", Value: turn},
				logging.Field{Key: "pass", Value: pass},
				logging.Field{Key: "error", Value: err.Error()})
			stopReason = fmt.Sprintf("llm error pass %d: %v", pass, err)
			break PASSES
		}

		totalTokensPrompt += resp.TokensPrompt
		totalTokensCompletion += resp.TokensCompletion
		totalLatencyMs += resp.Latency.Milliseconds()
		finalRespText = resp.Text

		parsed, _ := llm.ParseResponse(resp.Text)
		requests := query.ParseRequests(resp.Text)

		// If the model emitted action commands, commit them and end the
		// turn. Commands win over REQUESTs in the same response (any
		// REQUESTs are dropped — the LLM should not mix modes).
		if parsed != nil && len(parsed.Commands) > 0 {
			committed = l.commitPlan(turn, parsed)
			stopReason = fmt.Sprintf("committed %d commands on pass %d", committed, pass)
			if len(requests) > 0 {
				l.warn("bdi: response mixed commands and requests; dropping requests",
					logging.Field{Key: "turn", Value: turn},
					logging.Field{Key: "pass", Value: pass},
					logging.Field{Key: "dropped_request_count", Value: len(requests)})
			}
			l.logPass(turn, pass, passStart, currentMsg, resp.Text, len(requests), committed, "commit")
			break PASSES
		}

		// REQUESTs only — dispatch and recall.
		if len(requests) > 0 {
			results := l.registry.DispatchAll(qctx, requests)
			totalRequests += len(requests)
			l.info("bdi: requests dispatched",
				logging.Field{Key: "turn", Value: turn},
				logging.Field{Key: "pass", Value: pass},
				logging.Field{Key: "request_count", Value: len(requests)})

			history = append(history,
				llm.Message{Role: "user", Content: currentMsg},
				llm.Message{Role: "assistant", Content: resp.Text},
			)
			currentMsg = query.RenderResults(results) +
				"\nContinue with more `request` calls if you need more information, or emit action commands to commit your plan.\n"

			l.logPass(turn, pass, passStart, currentMsg, resp.Text, len(requests), 0, "request")
			continue PASSES
		}

		// Neither commands nor requests — log and end turn.
		l.warn("bdi: response had no commands and no requests",
			logging.Field{Key: "turn", Value: turn},
			logging.Field{Key: "pass", Value: pass})
		stopReason = fmt.Sprintf("empty response pass %d", pass)
		l.logPass(turn, pass, passStart, currentMsg, resp.Text, 0, 0, "empty")
		break PASSES
	}

	if stopReason == "" {
		stopReason = fmt.Sprintf("max passes (%d) exhausted without commit", maxPasses)
		l.warn("bdi: max passes exhausted",
			logging.Field{Key: "turn", Value: turn},
			logging.Field{Key: "max_passes", Value: maxPasses})
	}

	l.info("bdi: turn complete",
		logging.Field{Key: "turn", Value: turn},
		logging.Field{Key: "passes_used", Value: countPasses(history)},
		logging.Field{Key: "tokens_prompt_total", Value: totalTokensPrompt},
		logging.Field{Key: "tokens_completion_total", Value: totalTokensCompletion},
		logging.Field{Key: "latency_ms_total", Value: totalLatencyMs},
		logging.Field{Key: "requests_total", Value: totalRequests},
		logging.Field{Key: "nodes_committed", Value: committed},
		logging.Field{Key: "stop_reason", Value: stopReason})

	l.logTurn(turn, cycleStart, snap, predResults, in, initialMsg, finalRespText, committed,
		fmt.Sprintf("%s; passes=%d requests=%d", stopReason, countPasses(history)+1, totalRequests))
}

// countPasses returns the number of completed passes implied by a history
// slice (each pass adds a user+assistant pair).
func countPasses(history []llm.Message) int {
	return len(history) / 2
}

// commitPlan turns parsed CommandSpecs into PlanNodes and adds them to the
// DAG. Returns the count committed. The executor will pick them up on its
// next pass.
func (l *Loop) commitPlan(turn uint64, parsed *llm.ParsedResponse) int {
	if parsed == nil || len(parsed.Commands) == 0 {
		return 0
	}

	count := 0
	for idx, spec := range parsed.Commands {
		action := actionFromCommandSpec(spec)
		if action.Type == "" {
			continue
		}
		node := &plan.Node{
			ID:             fmt.Sprintf("t%d-n%d", turn, idx),
			Action:         action,
			Status:         plan.StatusPending, // promoted to Ready by Add when no deps
			CreatedAt:      time.Now(),
			CommittingTurn: turn,
		}
		l.dag.Add(node)
		count++
	}
	return count
}

// actionFromCommandSpec converts the LLM parser's command spec into the
// plan package's Action.
func actionFromCommandSpec(spec llm.CommandSpec) plan.Action {
	a := plan.Action{
		Type:            spec.Type,
		DigType:         spec.DigType,
		BuildType:       spec.BuildType,
		ZoneType:        spec.ZoneType,
		OrderType:       spec.OrderType,
		Quantity:        spec.Quantity,
		StockpileGroups: spec.StockpileGroups,
		SmoothType:      spec.SmoothType,
		AlertID:         spec.AlertID,
	}
	if spec.Region != nil {
		z1 := int16(spec.Region.Z)
		z2 := z1
		if spec.Region.Z2 > 0 && uint16(z2) != spec.Region.Z2 {
			z2 = int16(spec.Region.Z2)
		}
		a.Region = plan.Region{
			X1: int16(spec.Region.X1),
			Y1: int16(spec.Region.Y1),
			Z1: z1,
			X2: int16(spec.Region.X2),
			Y2: int16(spec.Region.Y2),
			Z2: z2,
		}
	}
	return a
}

// logPass writes a JSONL row for one pass within a turn. The trace
// pipeline reads these to reconstruct the deliberator's full reasoning
// trail per turn (request → result → request → ... → commit).
func (l *Loop) logPass(turn uint64, pass int, passStart time.Time, userMsg, respText string, requests, committed int, kind string) {
	if l.logFile == nil {
		return
	}
	entry := map[string]any{
		"timestamp":        passStart.Format(time.RFC3339Nano),
		"turn":             turn,
		"pass":             pass,
		"kind":             kind, // "commit" | "request" | "empty"
		"duration_ms":      time.Since(passStart).Milliseconds(),
		"user_msg_size":    len(userMsg),
		"response_size":    len(respText),
		"response_preview": truncate(respText, 4000),
		"requests":         requests,
		"committed":        committed,
		"row_type":         "pass",
	}
	bytes, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = l.logFile.Write(append(bytes, '\n'))
}

// logTurn writes a JSONL row capturing the entire decision cycle. The schema
// is intentionally rich so the trace persistence pipeline can read it later
// without re-running predicates.
func (l *Loop) logTurn(
	turn uint64,
	cycleStart time.Time,
	snap worldmodel.Snapshot,
	results []predicate.Result,
	in RenderInput,
	userMsg string,
	respText string,
	nodesCommitted int,
	summary string,
) {
	if l.logFile == nil {
		return
	}

	entry := map[string]any{
		"timestamp":               cycleStart.Format(time.RFC3339Nano),
		"turn":                    turn,
		"tick":                    snap.Tick,
		"event_seq":               snap.EventSeq,
		"duration_ms":             time.Since(cycleStart).Milliseconds(),
		"dwarf_count":             len(snap.Entities.Dwarves),
		"enemy_count":             len(snap.Entities.Enemies),
		"fort_age_days":           snap.Fort.DaysElapsed,
		"predicates":              summarizePredicates(results),
		"plan_stats":              in.PlanStats,
		"recent_divergences":      len(in.RecentDivergences),
		"predictions_outstanding": snap.Predictions.OutstandingCount,
		"predictions_diverged":    snap.Predictions.DivergedCount,
		"user_msg_size":           len(userMsg),
		"response_size":           len(respText),
		"response_preview":        truncate(respText, 4000),
		"nodes_committed":         nodesCommitted,
		"summary":                 summary,
		"provider":                l.llm.GetProviderType(),
		"model":                   l.llm.GetModelName(),
	}

	bytes, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = l.logFile.Write(append(bytes, '\n'))
}

func summarizePredicates(results []predicate.Result) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"name":       r.Name,
			"horizon":    r.Horizon.String(),
			"satisfied":  r.Satisfied,
			"confidence": r.Confidence,
		})
	}
	return out
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func (l *Loop) nextTurn() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.turnCounter++
	return l.turnCounter
}

// logging helpers (nil-safe wrappers)

func (l *Loop) info(msg string, fields ...logging.Field) {
	if l.logger != nil {
		l.logger.Info(msg, fields...)
	}
}

func (l *Loop) warn(msg string, fields ...logging.Field) {
	if l.logger != nil {
		l.logger.Warn(msg, fields...)
	}
}

func (l *Loop) debug(msg string, fields ...logging.Field) {
	if l.logger != nil {
		l.logger.Debug(msg, fields...)
	}
}
