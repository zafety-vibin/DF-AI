package mcpserver

import "context"

// Bridge owns the dfhack client, world model, and executor. Fleshed out
// in the bridge task; nil-safe accessors keep the scaffold runnable.
type Bridge struct{}

func (b *Bridge) Connected() bool                       { return false }
func (b *Bridge) StatusLine(ctx context.Context) string { return "status unavailable" }
