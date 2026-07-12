package query

import (
	"regexp"
	"strings"
)

var requestPattern = regexp.MustCompile(`(?i)request\s+([a-z][\w_]*)\s*\(([^)]*)\)`)

// ParseRequests extracts all `request <name>(<args>)` calls from text in
// order of appearance. Args are split by comma, trimmed of whitespace and
// surrounding quotes, and returned as a flat string slice. Each handler
// is responsible for parsing its own arg types.
func ParseRequests(text string) []Call {
	matches := requestPattern.FindAllStringSubmatchIndex(text, -1)
	if matches == nil {
		return nil
	}
	out := make([]Call, 0, len(matches))
	for _, m := range matches {
		// m = [matchStart, matchEnd, group1Start, group1End, group2Start, group2End]
		raw := text[m[0]:m[1]]
		name := strings.ToLower(strings.TrimSpace(text[m[2]:m[3]]))
		argStr := strings.TrimSpace(text[m[4]:m[5]])
		args := splitArgs(argStr)
		out = append(out, Call{Name: name, Args: args, Raw: raw})
	}
	return out
}

// splitArgs splits "40, 30, 120" → ["40","30","120"], handling quoted
// strings and stripping surrounding quotes. Empty input returns nil.
func splitArgs(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		// Strip surrounding " or '
		if len(t) >= 2 {
			first, last := t[0], t[len(t)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				t = t[1 : len(t)-1]
			}
		}
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
