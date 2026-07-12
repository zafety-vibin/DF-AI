package skill

import (
	"fmt"
	"sort"
	"strings"
)

// Skill is a named procedural recipe. Steps are ordered with optional
// dependency relationships. Prerequisites are predicates / other skills
// that must be satisfied before this one is reasonable to attempt.
// Provides describes which predicates this skill aims to satisfy.
type Skill struct {
	Name          string   // snake_case identifier, e.g. "build_bedroom"
	Summary       string   // one-sentence purpose
	Steps         []Step   // ordered procedure
	Prerequisites []string // predicate names or skill names that should be satisfied first
	Provides      []string // predicate names this skill aims to satisfy
	Tips          []string // free-text guidance about choices, scale, quality
}

// Step is one ordered piece of a skill. The Description tells the LLM
// what to accomplish; Action names the existing action vocabulary the
// step will use ("dig", "build", "zone", "order", "wait", "unsuspend").
// DependsOn names other steps within the same skill that must complete
// first.
type Step struct {
	ID          string   // short identifier within the skill (e.g. "carve_room")
	Description string   // one-line "what this step accomplishes"
	Action      string   // matching action vocabulary verb
	DependsOn   []string // IDs of prior steps in this skill that gate this one
	Optional    bool     // skip-able (nice-to-have, not required for the goal)
	Tips        []string // step-specific notes
}

// Library is a registered set of skills. Build with NewLibrary, register
// skills (typically once during main.go startup), then render into the
// deliberator prompt via RenderForPrompt.
type Library struct {
	skills map[string]*Skill
	order  []string // insertion order so RenderForPrompt is stable
}

// NewLibrary returns an empty library.
func NewLibrary() *Library { return &Library{skills: map[string]*Skill{}} }

// Register adds a skill. Panics on duplicate name (programming error).
func (l *Library) Register(s Skill) {
	if s.Name == "" {
		return
	}
	if _, exists := l.skills[s.Name]; exists {
		panic(fmt.Sprintf("skill.Library: duplicate skill name %q", s.Name))
	}
	skill := s
	l.skills[s.Name] = &skill
	l.order = append(l.order, s.Name)
}

// All returns skills in insertion order (stable for prompt output).
func (l *Library) All() []*Skill {
	out := make([]*Skill, 0, len(l.order))
	for _, name := range l.order {
		out = append(out, l.skills[name])
	}
	return out
}

// Get returns one skill by name.
func (l *Library) Get(name string) (*Skill, bool) {
	s, ok := l.skills[name]
	return s, ok
}

// Names returns all registered skill names sorted alphabetically.
// Useful for prompt summaries that need a stable list.
func (l *Library) Names() []string {
	out := make([]string, 0, len(l.skills))
	for name := range l.skills {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// RenderForPrompt emits the library as a markdown section the
// deliberator can read. Each skill becomes a sub-section with its
// summary, prerequisites, ordered steps with dependency notes, and tips.
//
// The output is intentionally readable rather than machine-parseable:
// the LLM is the consumer, and prose context lets it apply skills
// flexibly rather than rigidly.
func (l *Library) RenderForPrompt() string {
	if len(l.order) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Skills\n\n")
	sb.WriteString("Procedural recipes for common DF operations. Each skill names the ORDER of steps, NOT the dimensions, materials, or locations — those are your call. Use these to keep your operations consistent, but adapt freely to context (a peasant bedroom and a king's bedroom use the same skill with different scale and quality).\n\n")

	for _, name := range l.order {
		s := l.skills[name]
		fmt.Fprintf(&sb, "### %s\n", s.Name)
		if s.Summary != "" {
			fmt.Fprintf(&sb, "%s\n\n", s.Summary)
		}

		if len(s.Prerequisites) > 0 {
			sb.WriteString("**Prerequisites:** ")
			sb.WriteString(strings.Join(s.Prerequisites, ", "))
			sb.WriteString("\n\n")
		}
		if len(s.Provides) > 0 {
			sb.WriteString("**Targets:** ")
			sb.WriteString(strings.Join(s.Provides, ", "))
			sb.WriteString("\n\n")
		}

		if len(s.Steps) > 0 {
			sb.WriteString("**Steps:**\n")
			for i, step := range s.Steps {
				marker := fmt.Sprintf("%d.", i+1)
				if step.Optional {
					marker += " *(optional)*"
				}
				fmt.Fprintf(&sb, "%s **%s** — %s", marker, step.Action, step.Description)
				if len(step.DependsOn) > 0 {
					fmt.Fprintf(&sb, " *(after: %s)*", strings.Join(step.DependsOn, ", "))
				}
				sb.WriteString("\n")
				for _, tip := range step.Tips {
					fmt.Fprintf(&sb, "    - %s\n", tip)
				}
			}
			sb.WriteString("\n")
		}

		if len(s.Tips) > 0 {
			sb.WriteString("**Tips:**\n")
			for _, t := range s.Tips {
				fmt.Fprintf(&sb, "- %s\n", t)
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
