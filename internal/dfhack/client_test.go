package dfhack

import (
	"io"
	"net"
	"testing"

	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// connectedConn builds a protocol.Connection in StateConnected backed by an
// in-memory pipe (no live socket needed). The peer end is drained so writes
// (e.g. Close's best-effort DisconnectMessage) never block: net.Pipe writes
// are synchronous.
func connectedConn(t *testing.T) *protocol.Connection {
	t.Helper()
	a, b := net.Pipe()
	go io.Copy(io.Discard, b) //nolint:errcheck // drain until pipe closes
	t.Cleanup(func() { a.Close(); b.Close() })
	conn := protocol.NewConnection()
	conn.AcceptConnection(a)
	return conn
}

// TestHeartbeatStale covers the reconnect-rebind decision: a heartbeat
// goroutine bound to an old connection must exit once the client has
// accepted a replacement connection, or once its own connection dies,
// instead of erroring every tick forever.
func TestHeartbeatStale(t *testing.T) {
	client := NewClient(logging.NewTextLogger("error"))

	connA := connectedConn(t)
	client.mu.Lock()
	client.conn = connA
	client.mu.Unlock()

	if client.heartbeatStale(connA) {
		t.Error("current connected conn reported stale")
	}

	// Plugin reconnects: client rebinds to a new connection. The goroutine
	// still holding connA must now stop.
	connB := connectedConn(t)
	client.mu.Lock()
	client.conn = connB
	client.mu.Unlock()

	if !client.heartbeatStale(connA) {
		t.Error("replaced conn not reported stale")
	}
	if client.heartbeatStale(connB) {
		t.Error("new current conn reported stale")
	}

	// Current connection dies without a replacement: also stale, so the
	// heartbeat loop exits rather than spamming send errors.
	connB.Close()
	if !client.heartbeatStale(connB) {
		t.Errorf("dead current conn (state %s) not reported stale", connB.State())
	}
}
