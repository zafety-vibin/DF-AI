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
	Type            string                 // "dig", "build", "chop", "gather", "zone", "stockpile", "unsuspend", "order", "wait", "dismiss"
	DigType         string                 // For dig: "default", "stairs", "channel", "ramp", "upstair", "downstair"
	BuildType       uint8                  // For build: protocol BuildType byte (workshop / furniture / construction / door)
	ZoneType        uint8                  // For zone: protocol ZoneType byte
	OrderType       uint8                  // For order: protocol OrderType byte
	Quantity        uint16                 // For order: number to produce
	StockpileGroups uint32                 // For stockpile: GroupMask bitfield
	SmoothType      uint8                  // For smooth: 1=smooth, 2=engrave
	AlertID         uint32                 // For dismiss: the alert ID to dismiss; 0 = all
	Region          *RegionSpec            // Coordinates; build/unsuspend use single tile (X1=X2,Y1=Y2,Z1=Z2)
	Params          map[string]interface{} // Additional parameters
}

// RegionSpec contains coordinates for a region-based command
type RegionSpec struct {
	X1 uint16 `json:"x1"`
	Y1 uint16 `json:"y1"`
	Z  uint16 `json:"z"`  // Start Z
	X2 uint16 `json:"x2"`
	Y2 uint16 `json:"y2"`
	Z2 uint16 `json:"z2"` // End Z (for vertical shafts)
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

// parseNaturalLanguage uses regex to extract commands from natural language.
// It matches three syntaxes:
//
//   dig [type] from (x1, y1, z1) to (x2, y2, z2)         -- ranged digs
//   chop / gather from (x1, y1, z1) to (x2, y2, z2)      -- ranged designations
//   build <type-name> at (x, y, z)                       -- single-tile builds
//   wait                                                  -- no-op
func parseNaturalLanguage(text string) *ParsedResponse {
	result := &ParsedResponse{
		Reasoning: extractReasoning(text),
		Commands:  make([]CommandSpec, 0),
		RawText:   text,
	}

	// Pattern: dig|chop|gather [type] from (x1, y1, z1) to (x2, y2, z2)
	// Captures: cmd + optional dig type + coordinates (including optional ending Z for vertical shafts)
	rangedPattern := regexp.MustCompile(`(?i)(dig|chop|gather)\s+(?:(stairs|updownstairs|updown|channel|ramp|upstair|downstair)\s+)?(?:from\s+)?\(?(\d+),\s*(\d+),\s*(\d+)\)?\s+(?:to\s+)?\(?(\d+),\s*(\d+)(?:,\s*(\d+))?\)?`)
	for _, match := range rangedPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 8 {
			continue
		}
		cmdType := strings.ToLower(match[1])
		digType := strings.ToLower(match[2])
		if digType == "updown" {
			digType = "updownstairs"
		}
		if digType == "" && cmdType == "dig" {
			digType = "default"
		}

		x1, _ := strconv.ParseUint(match[3], 10, 16)
		y1, _ := strconv.ParseUint(match[4], 10, 16)
		z1, _ := strconv.ParseUint(match[5], 10, 16)
		x2, _ := strconv.ParseUint(match[6], 10, 16)
		y2, _ := strconv.ParseUint(match[7], 10, 16)
		z2 := z1
		if len(match) >= 9 && match[8] != "" {
			z2Val, _ := strconv.ParseUint(match[8], 10, 16)
			z2 = z2Val
		}

		result.Commands = append(result.Commands, CommandSpec{
			Type:    cmdType,
			DigType: digType,
			Region: &RegionSpec{
				X1: uint16(x1), Y1: uint16(y1), Z: uint16(z1),
				X2: uint16(x2), Y2: uint16(y2), Z2: uint16(z2),
			},
			Params: make(map[string]interface{}),
		})
	}

	// Pattern: build <type> at (x, y, z)
	buildPattern := regexp.MustCompile(`(?i)build\s+([\w_-]+)\s+at\s+\(?(\d+),\s*(\d+),\s*(\d+)\)?`)
	for _, match := range buildPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 5 {
			continue
		}
		typeName := strings.ToLower(match[1])
		buildType, ok := lookupBuildType(typeName)
		if !ok {
			// Unknown build target — skip; LLM will see no command emitted
			// and may correct on next turn.
			continue
		}
		x, _ := strconv.ParseUint(match[2], 10, 16)
		y, _ := strconv.ParseUint(match[3], 10, 16)
		z, _ := strconv.ParseUint(match[4], 10, 16)
		result.Commands = append(result.Commands, CommandSpec{
			Type:      "build",
			BuildType: buildType,
			Region: &RegionSpec{
				X1: uint16(x), Y1: uint16(y), Z: uint16(z),
				X2: uint16(x), Y2: uint16(y), Z2: uint16(z),
			},
			Params: map[string]interface{}{"build_type_name": typeName},
		})
	}

	// Pattern: stockpile [<category>] from (x1, y1, z) to (x2, y2, z)
	// "stockpile from ... to ..." defaults to "all" (every category accepted).
	// "stockpile food from ... to ..." restricts to that category group.
	stockpilePattern := regexp.MustCompile(`(?i)stockpile(?:\s+([\w_-]+))?\s+(?:from\s+)?\(?(\d+),\s*(\d+),\s*(\d+)\)?\s+(?:to\s+)?\(?(\d+),\s*(\d+)\)?`)
	for _, match := range stockpilePattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 7 {
			continue
		}
		categoryName := strings.ToLower(strings.TrimSpace(match[1]))
		mask, ok := lookupStockpileGroup(categoryName)
		if !ok {
			continue
		}
		x1, _ := strconv.ParseUint(match[2], 10, 16)
		y1, _ := strconv.ParseUint(match[3], 10, 16)
		z, _ := strconv.ParseUint(match[4], 10, 16)
		x2, _ := strconv.ParseUint(match[5], 10, 16)
		y2, _ := strconv.ParseUint(match[6], 10, 16)
		result.Commands = append(result.Commands, CommandSpec{
			Type:            "stockpile",
			StockpileGroups: mask,
			Region: &RegionSpec{
				X1: uint16(x1), Y1: uint16(y1), Z: uint16(z),
				X2: uint16(x2), Y2: uint16(y2), Z2: uint16(z),
			},
			Params: map[string]interface{}{"stockpile_category": categoryName},
		})
	}

	// Pattern: zone <type> from (x1, y1, z) to (x2, y2, z)
	zonePattern := regexp.MustCompile(`(?i)zone\s+([\w_-]+)\s+(?:from\s+)?\(?(\d+),\s*(\d+),\s*(\d+)\)?\s+(?:to\s+)?\(?(\d+),\s*(\d+)\)?`)
	for _, match := range zonePattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 7 {
			continue
		}
		typeName := strings.ToLower(match[1])
		zoneType, ok := lookupZoneType(typeName)
		if !ok {
			continue
		}
		x1, _ := strconv.ParseUint(match[2], 10, 16)
		y1, _ := strconv.ParseUint(match[3], 10, 16)
		z, _ := strconv.ParseUint(match[4], 10, 16)
		x2, _ := strconv.ParseUint(match[5], 10, 16)
		y2, _ := strconv.ParseUint(match[6], 10, 16)
		result.Commands = append(result.Commands, CommandSpec{
			Type:     "zone",
			ZoneType: zoneType,
			Region: &RegionSpec{
				X1: uint16(x1), Y1: uint16(y1), Z: uint16(z),
				X2: uint16(x2), Y2: uint16(y2), Z2: uint16(z),
			},
			Params: map[string]interface{}{"zone_type_name": typeName},
		})
	}

	// Pattern: smooth|engrave from (x1, y1, z) to (x2, y2, z)
	smoothPattern := regexp.MustCompile(`(?i)(smooth|engrave)\s+(?:from\s+)?\(?(\d+),\s*(\d+),\s*(\d+)\)?\s+(?:to\s+)?\(?(\d+),\s*(\d+)\)?`)
	for _, match := range smoothPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 7 {
			continue
		}
		var smoothType uint8 = 1 // smooth
		if strings.EqualFold(match[1], "engrave") {
			smoothType = 2
		}
		x1, _ := strconv.ParseUint(match[2], 10, 16)
		y1, _ := strconv.ParseUint(match[3], 10, 16)
		z, _ := strconv.ParseUint(match[4], 10, 16)
		x2, _ := strconv.ParseUint(match[5], 10, 16)
		y2, _ := strconv.ParseUint(match[6], 10, 16)
		result.Commands = append(result.Commands, CommandSpec{
			Type:       "smooth",
			SmoothType: smoothType,
			Region: &RegionSpec{
				X1: uint16(x1), Y1: uint16(y1), Z: uint16(z),
				X2: uint16(x2), Y2: uint16(y2), Z2: uint16(z),
			},
		})
	}

	// Pattern: unsuspend at (x, y, z)
	unsuspendPattern := regexp.MustCompile(`(?i)unsuspend\s+(?:at\s+)?\(?(\d+),\s*(\d+),\s*(\d+)\)?`)
	for _, match := range unsuspendPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 4 {
			continue
		}
		x, _ := strconv.ParseUint(match[1], 10, 16)
		y, _ := strconv.ParseUint(match[2], 10, 16)
		z, _ := strconv.ParseUint(match[3], 10, 16)
		result.Commands = append(result.Commands, CommandSpec{
			Type: "unsuspend",
			Region: &RegionSpec{
				X1: uint16(x), Y1: uint16(y), Z: uint16(z),
				X2: uint16(x), Y2: uint16(y), Z2: uint16(z),
			},
		})
	}

	// Pattern: order N <item>
	orderPattern := regexp.MustCompile(`(?i)order\s+(\d+)\s+([\w_-]+)`)
	for _, match := range orderPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 3 {
			continue
		}
		qty, _ := strconv.ParseUint(match[1], 10, 16)
		if qty == 0 || qty > 100 {
			continue
		}
		typeName := strings.ToLower(match[2])
		// Allow plural forms ("beds", "tables") by trimming a trailing 's'.
		orderType, ok := lookupOrderType(typeName)
		if !ok && strings.HasSuffix(typeName, "s") {
			orderType, ok = lookupOrderType(strings.TrimSuffix(typeName, "s"))
		}
		if !ok {
			continue
		}
		result.Commands = append(result.Commands, CommandSpec{
			Type:      "order",
			OrderType: orderType,
			Quantity:  uint16(qty),
			Params:    map[string]interface{}{"order_type_name": typeName},
		})
	}

	// Pattern: dismiss alert <id> | dismiss all alerts
	dismissAllPattern := regexp.MustCompile(`(?i)dismiss\s+all\s+alerts?`)
	if dismissAllPattern.MatchString(text) {
		result.Commands = append(result.Commands, CommandSpec{
			Type:    "dismiss",
			AlertID: 0, // 0 means all
		})
	} else {
		// Single-alert dismissal: "dismiss alert 42" or "dismiss 42"
		dismissPattern := regexp.MustCompile(`(?i)dismiss\s+(?:alert\s+)?(\d+)`)
		for _, match := range dismissPattern.FindAllStringSubmatch(text, -1) {
			if len(match) < 2 {
				continue
			}
			id, _ := strconv.ParseUint(match[1], 10, 32)
			if id == 0 {
				continue
			}
			result.Commands = append(result.Commands, CommandSpec{
				Type:    "dismiss",
				AlertID: uint32(id),
			})
		}
	}

	// Pattern: wait
	waitPattern := regexp.MustCompile(`(?i)\bwait\b(?:\s+(?:for\s+)?(\d+)\s+(turn|second|minute)s?)?`)
	for _, match := range waitPattern.FindAllStringSubmatch(text, -1) {
		params := map[string]interface{}{}
		if len(match) >= 3 && match[1] != "" {
			if duration, err := strconv.Atoi(match[1]); err == nil {
				params["duration"] = duration
				params["unit"] = strings.ToLower(match[2])
			}
		}
		result.Commands = append(result.Commands, CommandSpec{Type: "wait", Params: params})
	}

	return result
}

// buildTypeNames maps human-readable buildable names (lowercase) to the
// protocol BuildType byte. The LLM emits the name; the parser looks it up.
// Synonyms (e.g., "carpenter" / "carpenters") map to the same byte.
var buildTypeNames = map[string]uint8{
	// Constructions
	"wall":         0x01, // protocol.BuildTypeWall
	"floor":        0x02, // protocol.BuildTypeFloor
	"upstair":      0x03,
	"downstair":    0x04,
	"updownstair":  0x05,
	"updownstairs": 0x05,
	"buildramp":    0x06, // distinguish from "dig ramp"

	// Workshops
	"carpenter":    0x10,
	"carpenters":   0x10,
	"mason":        0x11,
	"masons":       0x11,
	"still":        0x12,
	"farmer":       0x13,
	"farmers":      0x13,
	"craftsdwarf":  0x14,
	"craftsdwarfs": 0x14,
	"mechanic":     0x15,
	"mechanics":    0x15,
	"butcher":      0x16,
	"butchers":     0x16,
	"kitchen":      0x17,
	"fishery":      0x18,

	// Furniture
	"bed":     0x30,
	"table":   0x31,
	"chair":   0x32,
	"cabinet": 0x33,
	"coffer":  0x34,

	// Doors
	"door":  0x50,
	"hatch": 0x51,
}

// lookupBuildType returns the protocol BuildType byte for a human-readable
// name. Unknown names return (0, false).
func lookupBuildType(name string) (uint8, bool) {
	v, ok := buildTypeNames[strings.ToLower(name)]
	return v, ok
}

// zoneTypeNames maps zone names to protocol ZoneType bytes.
var zoneTypeNames = map[string]uint8{
	"bedroom":     0x01,
	"dining":      0x02,
	"dining_hall": 0x02,
	"meeting":     0x03,
	"meeting_hall":0x03,
	"barracks":    0x04,
	"dormitory":   0x05,
	"farm":        0x10,
	"pen":         0x11,
	"pasture":     0x11,
	"garbage":     0x12,
	"garbage_dump":0x12,
	"pit":         0x13,
	"pond":        0x13,
	"water":       0x14,
	"water_source":0x14,
	"fishing":     0x15,
	"hospital":    0x16,
	"animal":      0x17,
	"animal_train":0x17,
	"tomb":        0x18,
}

func lookupZoneType(name string) (uint8, bool) {
	v, ok := zoneTypeNames[strings.ToLower(name)]
	return v, ok
}

// orderTypeNames maps order item names to protocol OrderType bytes.
var orderTypeNames = map[string]uint8{
	"bed":     0x01,
	"table":   0x02,
	"chair":   0x03,
	"door":    0x04,
	"barrel":  0x05,
	"bucket":  0x06,
	"cabinet": 0x07,
	"coffer":  0x08,
	"drink":   0x09,
	"alcohol": 0x09,
	"meal":    0x0A,
	"food":    0x0A,
	"block":   0x0B,
	"blocks":  0x0B,
	"craft":   0x0C,
	"crafts":  0x0C,
}

func lookupOrderType(name string) (uint8, bool) {
	v, ok := orderTypeNames[strings.ToLower(name)]
	return v, ok
}

// stockpileGroupMasks maps human-readable category names to GroupMask
// bitfields. Empty / "all" / "everything" = accept every category.
var stockpileGroupMasks = map[string]uint32{
	"":             0x1FFFF, // unspecified → all
	"all":          0x1FFFF,
	"everything":   0x1FFFF,
	"animals":      1 << 0,
	"food":         1 << 1,
	"furniture":    1 << 2,
	"corpses":      1 << 3,
	"refuse":       1 << 4,
	"stone":        1 << 5,
	"ammo":         1 << 6,
	"coins":        1 << 7,
	"bars":         1 << 8,
	"blocks":       1 << 8,
	"bars_blocks":  1 << 8,
	"gems":         1 << 9,
	"goods":        1 << 10,
	"finished":     1 << 10,
	"finished_goods": 1 << 10,
	"leather":      1 << 11,
	"cloth":        1 << 12,
	"wood":         1 << 13,
	"weapons":      1 << 14,
	"armor":        1 << 15,
	"sheet":        1 << 16,
}

func lookupStockpileGroup(name string) (uint32, bool) {
	v, ok := stockpileGroupMasks[strings.ToLower(name)]
	return v, ok
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
