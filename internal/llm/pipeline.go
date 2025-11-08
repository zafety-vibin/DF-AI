package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

// MultiModelProvider chains multiple LLM providers in sequence:
// Raw data → Extractor (feature extraction) → Features → Planner (decision making) → Commands
// Useful for separating concerns: one model extracts structured features, another makes decisions
type MultiModelProvider struct {
	extractorProvider Provider
	plannerProvider   Provider
	extractorPrompt   string // System prompt for feature extraction
}

// FeatureExtractionResponse represents the structured output from the extractor model
type FeatureExtractionResponse struct {
	Timestamp   string       `json:"timestamp"`
	Summary     string       `json:"summary"`
	Features    FeatureSet   `json:"features"`
	RawAnalysis string       `json:"raw_analysis"`
	TokensUsed  TokenMetrics `json:"tokens_used"`
	Latency     int          `json:"latency_ms"`
}

// FeatureSet contains extracted features for planning
type FeatureSet struct {
	ChamberCount     int      `json:"chamber_count"`
	ActiveMods       int      `json:"active_modifications"`
	HazardTypes      []string `json:"hazard_types"`
	ResourceStatus   string   `json:"resource_status"`   // adequate, low, critical
	PopulationStatus string   `json:"population_status"` // healthy, stressed, endangered
	UrgentNeeds      []string `json:"urgent_needs"`      // list of immediate priorities
	OpportunityCost  string   `json:"opportunity_cost"`  // what will happen if we do nothing
}

// TokenMetrics tracks token usage
type TokenMetrics struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewMultiModelProvider creates a multi-model pipeline with feature extraction and planning stages
// extractorProvider: model for parsing raw data into structured features
// plannerProvider: model for making decisions based on extracted features
func NewMultiModelProvider(extractorProvider, plannerProvider Provider) (*MultiModelProvider, error) {
	if extractorProvider == nil {
		return nil, fmt.Errorf("extractorProvider cannot be nil")
	}
	if plannerProvider == nil {
		return nil, fmt.Errorf("plannerProvider cannot be nil")
	}

	return &MultiModelProvider{
		extractorProvider: extractorProvider,
		plannerProvider:   plannerProvider,
		extractorPrompt:   getFeatureExtractionSystemPrompt(),
	}, nil
}

// SendPrompt executes the full pipeline: raw data → extractor → features → planner → commands
// The pipeline works as follows:
// 1. Send raw context to extractor with feature extraction instructions
// 2. Parse extractor response to get structured features
// 3. Send features + original prompt to planner
// 4. Return final response with command recommendations
func (m *MultiModelProvider) SendPrompt(ctx context.Context, prompt *Prompt) (*Response, error) {
	// Stage 1: Feature Extraction
	// Create extraction request with raw data
	extractionPrompt := &Prompt{
		SystemPrompt: m.extractorPrompt,
		UserMessage:  prompt.UserMessage,
		History:      prompt.History,
		MaxTokens:    2048, // Features should be compact
		Temperature:  0.3,  // Low temperature for consistent parsing
	}

	extractionResp, err := m.extractorProvider.SendPrompt(ctx, extractionPrompt)
	if err != nil {
		return nil, fmt.Errorf("feature extraction failed: %w", err)
	}

	// Parse structured features from extraction response
	var features FeatureExtractionResponse
	if err := json.Unmarshal([]byte(extractionResp.Text), &features); err != nil {
		// If JSON parsing fails, treat entire response as raw analysis
		features = FeatureExtractionResponse{
			Summary:     extractionResp.Text,
			RawAnalysis: extractionResp.Text,
			TokensUsed: TokenMetrics{
				PromptTokens:     extractionResp.TokensPrompt,
				CompletionTokens: extractionResp.TokensCompletion,
			},
		}
	}

	// Stage 2: Planning with extracted features
	// Build planning context from features
	planningUserMessage := buildPlanningContext(prompt.UserMessage, &features)

	planningPrompt := &Prompt{
		SystemPrompt: prompt.SystemPrompt, // Original system prompt for planning
		UserMessage:  planningUserMessage,
		History:      prompt.History,
		MaxTokens:    prompt.MaxTokens,
		Temperature:  prompt.Temperature,
	}

	planningResp, err := m.plannerProvider.SendPrompt(ctx, planningPrompt)
	if err != nil {
		return nil, fmt.Errorf("planning stage failed: %w", err)
	}

	// Combine token usage from both stages
	combinedTokenPrompt := extractionResp.TokensPrompt + planningResp.TokensPrompt
	combinedTokenCompletion := extractionResp.TokensCompletion + planningResp.TokensCompletion

	// Return final response with metadata about pipeline execution
	return &Response{
		Text:             planningResp.Text,
		TokensPrompt:     combinedTokenPrompt,
		TokensCompletion: combinedTokenCompletion,
		FinishReason:     planningResp.FinishReason,
		Latency:          extractionResp.Latency + planningResp.Latency,
		ProviderMetadata: map[string]interface{}{
			"pipeline_type":            "multi_model",
			"extractor_provider":       m.extractorProvider.GetProviderType(),
			"planner_provider":         m.plannerProvider.GetProviderType(),
			"feature_extraction":       features,
			"extractor_latency_ms":     extractionResp.Latency.Milliseconds(),
			"planner_latency_ms":       planningResp.Latency.Milliseconds(),
			"extraction_finish_reason": extractionResp.FinishReason,
			"planning_finish_reason":   planningResp.FinishReason,
		},
	}, nil
}

// GetModelName returns a descriptive name for the pipeline
func (m *MultiModelProvider) GetModelName() string {
	return fmt.Sprintf("pipeline(%s→%s)",
		m.extractorProvider.GetModelName(),
		m.plannerProvider.GetModelName())
}

// GetProviderType returns "multi_model"
func (m *MultiModelProvider) GetProviderType() string {
	return "multi_model"
}

// SupportsStreaming returns true if both providers support streaming
func (m *MultiModelProvider) SupportsStreaming() bool {
	return m.extractorProvider.SupportsStreaming() && m.plannerProvider.SupportsStreaming()
}

// getFeatureExtractionSystemPrompt returns the system prompt for the extraction stage
// This prompt instructs the model to parse raw fort data and output structured features
func getFeatureExtractionSystemPrompt() string {
	return `You are a Dwarf Fortress fort analyzer. Your job is to extract key structural features from fort data.

TASK: Parse the fort status report and output ONLY valid JSON (no other text) with this structure:

{
  "timestamp": "ISO8601 timestamp",
  "summary": "1-2 sentence summary of fort status",
  "features": {
    "chamber_count": <number>,
    "active_modifications": <number>,
    "hazard_types": ["type1", "type2"],
    "resource_status": "adequate|low|critical",
    "population_status": "healthy|stressed|endangered",
    "urgent_needs": ["priority1", "priority2"],
    "opportunity_cost": "consequence if no action taken"
  },
  "raw_analysis": "detailed analysis paragraph",
  "tokens_used": {
    "prompt_tokens": 0,
    "completion_tokens": 0,
    "total_tokens": 0
  },
  "latency_ms": 0
}

Guidelines:
- Resource status: adequate (>60% reserves), low (20-60%), critical (<20%)
- Population status: healthy (no stress), stressed (some have stress), endangered (<50 dwarves)
- Focus on CURRENT state, not history
- List top 3 urgent needs by impact
- Be precise with counts; count actual chambers/modifications from data
- Opportunity cost: explain what happens if the fort is idle for 100 seconds

OUTPUT: Valid JSON only. No markdown, no code blocks, no explanation.`
}

// buildPlanningContext creates a user message that incorporates extracted features
// This gives the planner model context about what was learned from analysis
func buildPlanningContext(originalMessage string, features *FeatureExtractionResponse) string {
	featuresJSON, _ := json.MarshalIndent(features.Features, "", "  ")

	return fmt.Sprintf(`FORT STATUS ANALYSIS SUMMARY:
%s

EXTRACTED FEATURES:
%s

ORIGINAL REQUEST:
%s

Based on the above analysis, provide your command recommendations.`,
		features.Summary,
		string(featuresJSON),
		originalMessage)
}
