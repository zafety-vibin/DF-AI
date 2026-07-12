package topology

import "github.com/df-ai/orchestrator/internal/protocol"

// TileState is the three-state classification used by the topology overlay.
//
// Why three states: a single open/closed bit can't distinguish "we know
// this tile is solid wall" from "we have no data on this tile yet". DF
// lazily allocates blocks; a fresh embark has tile data only for a small
// fraction of the map's volume. Without an Unknown state, all unallocated
// tiles look like walls and the agent thinks the world is sealed.
type TileState uint8

const (
	StateUnknown TileState = 0 // No reliable data — unallocated block, hidden, or unclassified.
	StateOpen    TileState = 1 // Walkable surface (FLAG_FLOOR).
	StateClosed  TileState = 2 // Definitively impassable (wall, void, dangerous liquid).
)

// String returns the canonical lowercase name for the state.
func (s TileState) String() string {
	switch s {
	case StateOpen:
		return "open"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// ClassifyState maps the protocol's flag byte to a TileState.
//
// Rules (priority order, first match wins):
//   - FLAG_HIDDEN set: Unknown. The plugin sets this whenever DF returns
//     no tiletype (unallocated block) OR DF marks the tile hidden in the
//     player's UI. We treat both as "we don't have reliable data."
//   - FLAG_FLOOR set: Open.
//   - FLAG_WALL / FLAG_VOID / FLAG_LIQUID7 set: Closed.
//   - No classification flags set: Unknown (defensive; shouldn't happen
//     with the post-fix plugin but cheap insurance).
func ClassifyState(flags uint8) TileState {
	if flags&protocol.FlagHidden != 0 {
		return StateUnknown
	}
	if flags&protocol.FlagFloor != 0 {
		return StateOpen
	}
	if flags&(protocol.FlagWall|protocol.FlagVoid|protocol.FlagLiquid7) != 0 {
		return StateClosed
	}
	return StateUnknown
}
