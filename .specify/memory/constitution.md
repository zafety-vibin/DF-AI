<!--
SYNC IMPACT REPORT
==================
Version Change: Initial → 1.0.0
Ratification Date: 2025-11-04
Last Amended: 2025-11-04

New Constitution Created
- Research-focused AI development principles established
- 7 core principles defined for DF AI orchestration research
- Emphasis on logging, modularity, and iterative learning
- Local LLM compatibility and fine-tuning readiness

Templates Status:
- ✅ plan-template.md: Constitution Check section ready for principle validation
- ✅ spec-template.md: Requirements align with research-focused approach
- ✅ tasks-template.md: Task structure supports modular experimentation
- ✅ Command files: No agent-specific references, generic guidance maintained

Follow-up TODOs: None - all placeholders filled
-->

# Dwarf Fortress AI Orchestration Constitution

## Core Principles

### I. Research-First Development

This is an EXPERIMENTAL AI RESEARCH PROJECT, not a production system. Every decision
prioritizes learning and iteration over stability or polish. All features exist to
test hypotheses about AI fort management.

**Key Rules**:
- Document hypotheses before implementing features
- Track what works and what doesn't in logs and metrics
- Accept that experiments will fail - plan for easy rollback
- Rapid iteration beats perfect architecture
- Research artifacts (experiments, results, insights) are first-class deliverables

**Rationale**: We're exploring unknown territory in AI game-playing. Traditional
software engineering practices that optimize for stability would slow discovery.

---

### II. Comprehensive Logging & Observability

Log EVERYTHING. Every graph update, every LLM decision, every command executed,
every failure. Logs are the training data for future fine-tuning and the primary
tool for understanding AI behavior.

**Key Rules**:
- Structured logging with consistent format (JSON preferred)
- Log levels: DEBUG for graph operations, INFO for LLM decisions, WARN for
  safety validations, ERROR for failures
- Every LLM interaction logged: full context sent, full response received,
  timing, token usage
- Performance metrics: graph update latency, compression ratios, query times
- Game state snapshots at decision points
- NO logging suppression except for explicit performance testing

**Rationale**: Logs serve multiple purposes: debugging, fine-tuning dataset
generation, behavioral analysis, and RL reward signal calculation. Without
comprehensive logging, we lose the ability to learn from the AI's actions.

---

### III. Modularity & Pluggability

The architecture MUST support rapidly swapping components to test different
approaches. Graph layers, LLM providers, compression strategies, planning
algorithms - all should be pluggable modules.

**Key Rules**:
- Clear interface boundaries between components (topology, hazards, traffic,
  semantic, LLM client, command executor)
- Configuration-driven component selection (YAML/JSON config files)
- Each graph layer independently testable and benchmarkable
- Mock implementations required for all external dependencies (DFHack, LLM APIs)
- Document extension points for new experimental modules

**Rationale**: We'll want to try different approaches to spatial reasoning,
context compression, planning strategies, etc. Tight coupling would make
experimentation prohibitively slow.

---

### IV. Lean & Efficient (Local LLM Compatible)

Design for resource-constrained environments. The system MUST work with local LLMs
(7B-13B parameter models) running on consumer hardware, not just cloud APIs.

**Key Rules**:
- Context budget: ≤200KB per turn (target: 150KB)
- Memory efficient data structures: bit-packing, sparse grids, RLE compression
- Minimize LLM calls: one decision per turn cycle, batch queries
- No unnecessary dependencies: prefer stdlib over frameworks
- Profile and optimize hot paths (graph updates, compression)
- Document hardware requirements and performance characteristics

**Rationale**: Cloud API costs would limit experimentation volume. Local LLMs
enable unlimited iterations and eventual fine-tuning. Resource efficiency
ensures this scales to larger forts without hardware upgrades.

---

### V. Iterative Experimentation

Build the simplest version first, measure it, then improve. Premature
optimization and speculative features are forbidden.

**Key Rules**:
- Implement features in order: Foundation → Topology → LLM → Safety → Traffic
  → Semantics → Learning
- Each phase MUST produce measurable results before moving to next
- A/B testing mindset: keep old implementations available for comparison
- Version experimental changes: track which version produced which results
- Document "what we tried" as thoroughly as "what worked"

**Rationale**: We don't know which approaches will work. Building everything
upfront wastes time on dead ends. Incremental progress with measurement lets
us pivot quickly.

---

### VI. Metrics-Driven Evaluation

DF provides rich metrics: dwarf happiness, fort wealth, production efficiency,
survival time, etc. Use these to objectively evaluate AI performance.

**Key Rules**:
- Define success metrics for each feature (e.g., topology: update latency <50ms)
- Track game-level metrics: survival time, fort value, dwarf deaths, efficiency
- Log metrics per turn for time-series analysis
- Compare runs: same map seed with different AI strategies
- Build reward functions suitable for future RL fine-tuning
- Quantify improvements: "Strategy B increased survival time by 30%"

**Rationale**: Without objective metrics, we can't distinguish good strategies
from bad ones. DF's built-in metrics give us ground truth for evaluation and
potential reward signals for training.

---

### VII. Simplicity & Clarity

Favor simple, readable code over clever optimizations. The goal is to enable
rapid understanding and modification, not to showcase programming prowess.

**Key Rules**:
- Explicit over implicit: clear variable names, typed function signatures
- Comments explain "why" not "what"
- Avoid abstractions until needed in 3+ places
- Prefer composition over inheritance
- Keep functions small and single-purpose
- Documentation at module level: purpose, inputs, outputs, assumptions

**Rationale**: Research code changes frequently. Complexity slows iteration and
makes bugs harder to find. Clarity enables researchers (who may not be expert
programmers) to contribute and understand results.

---

## Research-Specific Constraints

### Reproducibility

- **Random Seeds**: All non-determinism must be seeded and logged (map generation,
  LLM sampling, pathfinding tie-breaking)
- **Experiment Configuration**: Each run tracked with config version, git commit hash,
  hyperparameters
- **Data Persistence**: Raw logs saved per experiment for later reanalysis

### Fine-Tuning Readiness

- **Training Data Format**: LLM interactions logged in format suitable for
  fine-tuning (prompt/completion pairs)
- **Reward Signals**: Game metrics mapped to scalar rewards (-1 to +1)
- **Behavioral Cloning**: Successful runs saved as demonstrations

### Safety & Ethics

- **No Real-World Risk**: This is a single-player game with no networked components
- **Failure is Acceptable**: Fort failures are learning opportunities, not catastrophes
- **Respect DF Community**: Acknowledge this is experimental, don't claim production
  readiness

---

## Development Workflow

### Experimentation Cycle

1. **Hypothesis**: Document what you're trying to improve and why
2. **Implementation**: Build minimal version following modularity principles
3. **Measurement**: Run experiments, collect logs and metrics
4. **Analysis**: Compare against baseline, document findings
5. **Decision**: Keep, iterate, or abandon based on results

### Code Quality Gates

- **Required**: Logging in place, metrics tracked, hypothesis documented
- **Recommended**: Unit tests for graph operations, integration tests for pipelines
- **Optional**: Code review, performance optimization (unless bottleneck identified)

### Complexity Justification

If violating simplicity principle (e.g., adding sophisticated algorithms,
external dependencies, optimization tricks):
- Document in `specs/*/plan.md` Complexity Tracking section
- Explain hypothesis: what improvement does this enable?
- Show simpler alternatives were insufficient

---

## Governance

### Constitution Authority

This constitution defines the project's values and technical philosophy. All design
decisions should reference these principles. When principles conflict (e.g.,
logging vs efficiency), favor the research mission: log everything, optimize later
if proven necessary.

### Amendments

- **Who**: Any contributor can propose amendments via issues or PRs
- **Process**: Document motivation, impact on existing code, update version
- **Versioning**:
  - MAJOR: Remove/redefine core principles (e.g., abandon local LLM compatibility)
  - MINOR: Add new principles, expand guidance materially
  - PATCH: Clarifications, examples, typo fixes

### Compliance

- All feature specs must include "Constitution Check" section
- Flag violations with justification in plan.md Complexity Tracking
- Periodic review: Are principles still serving research goals?

### Living Document

This constitution will evolve as we learn. Initial version reflects starting
hypotheses about what will enable productive research. If principles prove
counterproductive, amend them with lessons learned documented.

---

**Version**: 1.0.0 | **Ratified**: 2025-11-04 | **Last Amended**: 2025-11-04
