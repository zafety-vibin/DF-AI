package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CommandSpec represents a parsed command from LLM response
type CommandSpec struct {
	Type    string                 // "dig", "build", "wait", etc.
	DigType string                 // For dig commands: "default", "stairs", "updownstairs", "channel", "ramp", "upstair", "downstair"
	Region  *RegionSpec            // Coordinates for dig/build commands
	Params  map[string]interface{} // Additional parameters
}

// RegionSpec contains coordinates for a region-based command
type RegionSpec struct {
	X1 uint16 `json:"x1"`
	Y1 uint16 `json:"y1"`
	Z  uint16 `json:"z"`
	X2 uint16 `json:"x2"`
	Y2 uint16 `json:"y2"`
}

// ParsedResponse contains structured data extracted from LLM response
type ParsedResponse struct {
	Reasoning string        // LLM's reasoning/explanation
	Commands  []CommandSpec // Parsed commands
	RawText   string        // Original response text
}

// structuredResponse matches expected JSON format from LLM
type structuredResponse struct {
	Reasoning string        `json:"reasoning"`
	Commands  []CommandSpec `json:"commands"`
}

// ParseResponse attempts to parse LLM response into structured commands
// Tries JSON first, falls back to natural language parsing
func ParseResponse(responseText string) (*ParsedResponse, error) {
	result := &ParsedResponse{
		RawText:  responseText,
		Commands: make([]CommandSpec, 0),
	}

	// Try structured JSON parsing first
	if parsed, err := parseJSON(responseText); err == nil {
		result.Reasoning = parsed.Reasoning
		result.Commands = parsed.Commands
		return result, nil
	}

	// Fall back to natural language parsing
	parsed := parseNaturalLanguage(responseText)
	result.Reasoning = parsed.Reasoning
	result.Commands = parsed.Commands

	return result, nil
}

// parseJSON attempts to extract structured JSON from response
// Expected format: {"reasoning": "...", "commands": [...]}
func parseJSON(text string) (*ParsedResponse, error) {
	// Try to find JSON block in text (may be wrapped in markdown code blocks)
	jsonStart := strings.Index(text, "{")
	jsonEnd := strings.LastIndex(text, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonStart >= jsonEnd {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonText := text[jsonStart : jsonEnd+1]

	var structured structuredResponse
	if err := json.Unmarshal([]byte(jsonText), &structured); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &ParsedResponse{
		Reasoning: structured.Reasoning,
		Commands:  structured.Commands,
		RawText:   text,
	}, nil
}

// parseNaturalLanguage uses regex to extract commands from natural language
// Looks for patterns like "dig from (10, 20, 95) to (15, 25, 95)"
func parseNaturalLanguage(text string) *ParsedResponse {
	result := &ParsedResponse{
		Reasoning: extractReasoning(text),
		Commands:  make([]CommandSpec, 0),
		RawText:   text,
	}

	// Pattern: dig [type] from (x1, y1, z) to (x2, y2, z)
	// Captures: dig type (optional: stairs, updownstairs, channel, ramp, upstair, downstair, default)
	coordPattern := regexp.MustCompile(`(?i)(dig|build)\s+(?:(stairs|updownstairs|updown|channel|ramp|upstair|downstair)\s+)?(?:from\s+)?\(?(\d+),\s*(\d+),\s*(\d+)\)?\s+(?:to\s+)?\(?(\d+),\s*(\d+)(?:,\s*\d+)?\)?`)

	matches := coordPattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) >= 8 {
			cmdType := strings.ToLower(match[1])
			digType := strings.ToLower(match[2]) // Empty string if not specified

			// Normalize dig types
			if digType == "updown" {
				digType = "updownstairs"
			}
			if digType == "" && cmdType == "dig" {
				digType = "default" // Standard mining if not specified
			}

			x1, _ := strconv.ParseUint(match[3], 10, 16)
			y1, _ := strconv.ParseUint(match[4], 10, 16)
			z, _ := strconv.ParseUint(match[5], 10, 16)
			x2, _ := strconv.ParseUint(match[6], 10, 16)
			y2, _ := strconv.ParseUint(match[7], 10, 16)

			cmd := CommandSpec{
				Type:    cmdType,
				DigType: digType,
				Region: &RegionSpec{
					X1: uint16(x1),
					Y1: uint16(y1),
					Z:  uint16(z),
					X2: uint16(x2),
					Y2: uint16(y2),
				},
				Params: make(map[string]interface{}),
			}

			result.Commands = append(result.Commands, cmd)
		}
	}

	// Pattern: wait for X turns/seconds
	waitPattern := regexp.MustCompile(`(?i)wait\s+(?:for\s+)?(\d+)\s+(turn|second|minute)s?`)
	waitMatches := waitPattern.FindAllStringSubmatch(text, -1)
	for _, match := range waitMatches {
		if len(match) >= 3 {
			duration, _ := strconv.Atoi(match[1])
			unit := strings.ToLower(match[2])

			cmd := CommandSpec{
				Type: "wait",
				Params: map[string]interface{}{
					"duration": duration,
					"unit":     unit,
				},
			}

			result.Commands = append(result.Commands, cmd)
		}
	}

	return result
}

// extractReasoning attempts to extract reasoning/explanation from text
// Looks for common patterns like "Because...", "The fort needs...", etc.
func extractReasoning(text string) string {
	// Split by common sentence delimiters
	sentences := strings.Split(text, ".")
	if len(sentences) == 0 {
		return text
	}

	// Look for reasoning indicators
	reasoningPatterns := []string{
		"because",
		"since",
		"the fort needs",
		"we should",
		"i recommend",
		"reasoning:",
		"explanation:",
	}

	var reasoning strings.Builder
	for _, sentence := range sentences {
		lower := strings.ToLower(strings.TrimSpace(sentence))
		for _, pattern := range reasoningPatterns {
			if strings.Contains(lower, pattern) {
				if reasoning.Len() > 0 {
					reasoning.WriteString(". ")
				}
				reasoning.WriteString(strings.TrimSpace(sentence))
				break
			}
		}
	}

	if reasoning.Len() > 0 {
		return reasoning.String()
	}

	// If no reasoning found, return first few sentences
	if len(sentences) > 3 {
		return strings.Join(sentences[:3], ". ") + "."
	}

	return text
}
