package dfhack

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// SessionMetrics tracks connection statistics
type SessionMetrics struct {
	ConnectionTime    time.Time
	MessagesSent      uint64
	MessagesReceived  uint64
	BytesSent         uint64
	BytesReceived     uint64
	ErrorCount        uint64
	LastError         error
	LastErrorTime     time.Time
	HeartbeatsSent    uint64
	HeartbeatsReceived uint64
}

// Client is the high-level interface for communicating with DFHack
type Client struct {
	logger           *logging.Logger
	listener         net.Listener
	conn             *protocol.Connection
	port             uint16
	mu               sync.RWMutex
	fullStateCh      chan *protocol.FullStateMessage
	tileUpdateCh     chan *protocol.TileUpdateMessage
	entityUpdateCh   chan *protocol.EntityUpdateMessage
	commandAckCh     chan *protocol.CommandAckMessage
	stopCh           chan struct{}
	wg               sync.WaitGroup
	heartbeatSeq     uint8
	lastHeartbeatAck time.Time
	heartbeatMu      sync.Mutex
	metrics          SessionMetrics
	metricsMu        sync.Mutex
	onFullState      func(*protocol.FullStateMessage) // Callback for FULL_STATE messages
	onSaveRequest    func()                            // Callback for save requests
}

// NewClient creates a new DFHack client
func NewClient(logger *logging.Logger) *Client {
	return &Client{
		logger:         logger,
		fullStateCh:    make(chan *protocol.FullStateMessage, 1),
		tileUpdateCh:   make(chan *protocol.TileUpdateMessage, 100),
		entityUpdateCh: make(chan *protocol.EntityUpdateMessage, 100),
		commandAckCh:   make(chan *protocol.CommandAckMessage, 100),
		stopCh:         make(chan struct{}),
	}
}

// Start begins listening for DFHack plugin connections
// Blocks until connection established or context cancelled
func (c *Client) Start(ctx context.Context, port uint16) error {
	c.port = port
	addr := fmt.Sprintf(":%d", port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start listener on port %d: %w", port, err)
	}

	c.mu.Lock()
	c.listener = listener
	c.mu.Unlock()

	c.logger.Info("server listening for DFHack connections",
		logging.Field{Key: "address", Value: addr})

	// Accept connections in a goroutine
	c.wg.Add(1)
	go c.acceptLoop(ctx)

	return nil
}

// acceptLoop accepts incoming connections from DFHack plugin
func (c *Client) acceptLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		// Set accept deadline to allow checking context
		c.mu.RLock()
		listener := c.listener
		c.mu.RUnlock()

		if listener == nil {
			return
		}

		listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
		conn, err := listener.Accept()

		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Timeout is expected, check context and retry
			}
			c.logger.Error("accept error", err)
			continue
		}

		c.logger.Info("connection accepted",
			logging.Field{Key: "remote", Value: conn.RemoteAddr().String()})

		// Handle connection
		c.handleConnection(conn)
	}
}

// handleConnection manages a single DFHack plugin connection
func (c *Client) handleConnection(tcpConn net.Conn) {
	// Wrap the accepted connection
	protocolConn := protocol.NewConnection()
	protocolConn.AcceptConnection(tcpConn)

	c.mu.Lock()
	c.conn = protocolConn
	c.mu.Unlock()

	// Initialize session metrics
	c.metricsMu.Lock()
	c.metrics = SessionMetrics{
		ConnectionTime: time.Now(),
	}
	c.metricsMu.Unlock()

	// Perform handshake
	if err := c.performHandshake(protocolConn); err != nil {
		c.logger.Error("handshake failed", err)
		tcpConn.Close()
		return
	}

	c.logger.Info("handshake complete", logging.Field{Key: "version", Value: protocol.ProtocolVersion})

	// Request initial full state after successful handshake
	resyncReq := &protocol.ResyncRequestMessage{
		Reason: protocol.ReasonReconnect,
	}
	if err := protocolConn.Send(resyncReq); err != nil {
		c.logger.Error("failed to send initial resync request", err)
	} else {
		c.logger.Info("sent resync request", logging.Field{Key: "reason", Value: "initial_connection"})
	}

	// Start heartbeat sender
	c.startHeartbeat(protocolConn)

	// Start message handling loop
	c.wg.Add(1)
	go c.messageLoop(protocolConn)
}

// performHandshake performs the protocol handshake with the plugin
func (c *Client) performHandshake(conn *protocol.Connection) error {
	// Generate random connection ID
	connID := rand.Uint32()

	// Send our handshake
	outgoingHandshake := &protocol.HandshakeMessage{
		ProtocolVersion: protocol.ProtocolVersion,
		ConnectionID:    connID,
		Capabilities:    "",
	}

	if err := conn.Send(outgoingHandshake); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	c.logger.Debug("sent handshake",
		logging.Field{Key: "version", Value: protocol.ProtocolVersion},
		logging.Field{Key: "connection_id", Value: connID})

	// Receive plugin's handshake
	msg, err := conn.Receive()
	if err != nil {
		return fmt.Errorf("failed to receive handshake: %w", err)
	}

	handshake, ok := msg.(*protocol.HandshakeMessage)
	if !ok {
		return fmt.Errorf("expected handshake, got %T", msg)
	}

	c.logger.Debug("received handshake",
		logging.Field{Key: "version", Value: handshake.ProtocolVersion},
		logging.Field{Key: "connection_id", Value: handshake.ConnectionID})

	// Validate protocol version matches
	if handshake.ProtocolVersion != protocol.ProtocolVersion {
		return fmt.Errorf("protocol version mismatch: plugin=%d, server=%d",
			handshake.ProtocolVersion, protocol.ProtocolVersion)
	}

	return nil
}

// messageLoop handles incoming messages from the plugin
func (c *Client) messageLoop(conn *protocol.Connection) {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		msg, err := conn.Receive()
		if err != nil {
			c.logger.Error("receive error", err)
			c.metricsMu.Lock()
			c.metrics.ErrorCount++
			c.metrics.LastError = err
			c.metrics.LastErrorTime = time.Now()
			c.metricsMu.Unlock()
			return
		}

		// Track received message
		c.metricsMu.Lock()
		c.metrics.MessagesReceived++
		c.metricsMu.Unlock()

		// Debug: log every message received
		c.logger.Info("messageLoop received message",
			logging.Field{Key: "type", Value: fmt.Sprintf("0x%02X", msg.Type())})

		// Route message to appropriate handler
		switch m := msg.(type) {
		case *protocol.FullStateMessage:
			c.logger.Info("received full state",
				logging.Field{Key: "width", Value: m.Width},
				logging.Field{Key: "height", Value: m.Height},
				logging.Field{Key: "depth", Value: m.Depth},
				logging.Field{Key: "tiles", Value: len(m.Tiles)})

			// Send to full state channel (non-blocking)
			select {
			case c.fullStateCh <- m:
			default:
				c.logger.Warn("full state channel full, dropping message")
			}

			// Build topology overlay (callback to be set by main.go)
			if c.onFullState != nil {
				c.onFullState(m)
			}

		case *protocol.TileUpdateMessage:
			c.logger.Info("received tile update",
				logging.Field{Key: "changed_tiles", Value: m.Count})

			// Send to tile update channel (non-blocking)
			select {
			case c.tileUpdateCh <- m:
			default:
				c.logger.Warn("tile update channel full, dropping message")
			}

		case *protocol.EntityUpdateMessage:
			c.logger.Info("received entity update",
				logging.Field{Key: "entity_count", Value: m.Count})

			// Send to entity update channel (non-blocking)
			select {
			case c.entityUpdateCh <- m:
			default:
				c.logger.Warn("entity update channel full, dropping message")
			}

		case *protocol.HeartbeatMessage:
			// Echo heartbeat back with incremented sequence
			c.handleHeartbeat(conn, m)

		case *protocol.CommandAckMessage:
			c.logger.Info("received command ack",
				logging.Field{Key: "command_id", Value: m.CommandID},
				logging.Field{Key: "status", Value: m.Status})

			// Send to command ACK channel (non-blocking)
			select {
			case c.commandAckCh <- m:
			default:
				c.logger.Warn("command ack channel full, dropping message")
			}

		case *protocol.ResyncRequestMessage:
			// Plugin sending RESYNC back to us (unusual)
			// Reason 0x04 = save request from ai-save command
			if m.Reason == 0x04 {
				c.logger.Info("received save request from plugin")
				// Trigger save callback if set
				if c.onSaveRequest != nil {
					c.onSaveRequest()
				}
			} else {
				c.logger.Warn("received unexpected resync from plugin",
					logging.Field{Key: "reason", Value: m.Reason})
			}

		case *protocol.DisconnectMessage:
			c.logger.Info("received disconnect",
				logging.Field{Key: "reason", Value: m.Reason})
			return

		default:
			c.logger.Warn("unhandled message type",
				logging.Field{Key: "type", Value: fmt.Sprintf("0x%02X", m.Type())})
		}
	}
}

// RequestFullState requests a full state resync from the plugin
// Returns channel that will receive FullStateMessage or error
func (c *Client) RequestFullState(reason uint8) (<-chan *protocol.FullStateMessage, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, errors.New("not connected")
	}

	// Send resync request
	req := &protocol.ResyncRequestMessage{
		Reason: reason,
	}

	if err := conn.Send(req); err != nil {
		return nil, fmt.Errorf("failed to send resync request: %w", err)
	}

	c.logger.Info("requested full state resync",
		logging.Field{Key: "reason", Value: reason})

	return c.fullStateCh, nil
}

// SubscribeTileUpdates returns channel for incremental tile updates
func (c *Client) SubscribeTileUpdates() <-chan *protocol.TileUpdateMessage {
	return c.tileUpdateCh
}

// SubscribeEntityUpdates returns channel for entity position updates
func (c *Client) SubscribeEntityUpdates() <-chan *protocol.EntityUpdateMessage {
	return c.entityUpdateCh
}

// SubscribeCommandAcks returns channel for command acknowledgments
func (c *Client) SubscribeCommandAcks() <-chan *protocol.CommandAckMessage {
	return c.commandAckCh
}

// SendCommand sends a command to the DFHack plugin
func (c *Client) SendCommand(cmd *protocol.CommandMessage) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return errors.New("not connected")
	}

	if err := conn.Send(cmd); err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	c.logger.Debug("sent command to DFHack",
		logging.Field{Key: "command_id", Value: cmd.CommandID},
		logging.Field{Key: "command_type", Value: cmd.CommandType})

	return nil
}

// IsConnected returns true if connection is active
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && c.conn.State() == protocol.StateConnected
}

// handleHeartbeat processes incoming heartbeat from plugin and echoes it back
func (c *Client) handleHeartbeat(conn *protocol.Connection, hb *protocol.HeartbeatMessage) {
	// Update last ack time
	c.heartbeatMu.Lock()
	c.lastHeartbeatAck = time.Now()
	c.heartbeatMu.Unlock()

	// Track heartbeat received
	c.metricsMu.Lock()
	c.metrics.HeartbeatsReceived++
	c.metricsMu.Unlock()

	// Calculate round-trip time
	now := uint64(time.Now().UnixMilli())
	rtt := now - hb.Timestamp

	c.logger.Debug("received heartbeat echo",
		logging.Field{Key: "sequence", Value: hb.Sequence},
		logging.Field{Key: "timestamp", Value: hb.Timestamp},
		logging.Field{Key: "rtt_ms", Value: rtt})
}

// startHeartbeat begins sending periodic heartbeats to the plugin
func (c *Client) startHeartbeat(conn *protocol.Connection) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		// Initialize heartbeat timestamp
		c.heartbeatMu.Lock()
		c.lastHeartbeatAck = time.Now()
		c.heartbeatMu.Unlock()

		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				// Send heartbeat
				c.heartbeatMu.Lock()
				c.heartbeatSeq++
				seq := c.heartbeatSeq
				c.heartbeatMu.Unlock()

				hb := &protocol.HeartbeatMessage{
					Timestamp: uint64(time.Now().UnixMilli()),
					Sequence:  seq,
				}

				if err := conn.Send(hb); err != nil {
					c.logger.Error("failed to send heartbeat", err)
					c.metricsMu.Lock()
					c.metrics.ErrorCount++
					c.metrics.LastError = err
					c.metrics.LastErrorTime = time.Now()
					c.metricsMu.Unlock()
					continue
				}

				// Track sent heartbeat
				c.metricsMu.Lock()
				c.metrics.MessagesSent++
				c.metrics.HeartbeatsSent++
				c.metricsMu.Unlock()

				c.logger.Debug("sent heartbeat",
					logging.Field{Key: "sequence", Value: seq})

				// Check for timeout (no ack in 15 seconds)
				c.heartbeatMu.Lock()
				timeSinceAck := time.Since(c.lastHeartbeatAck)
				c.heartbeatMu.Unlock()

				if timeSinceAck > 15*time.Second {
					c.logger.Warn("heartbeat timeout detected",
						logging.Field{Key: "time_since_ack", Value: timeSinceAck.String()})
					// Connection is considered dead
					conn.Close()
					return
				}
			}
		}
	}()
}

// GetSessionMetrics returns current session statistics
func (c *Client) GetSessionMetrics() SessionMetrics {
	c.metricsMu.Lock()
	defer c.metricsMu.Unlock()
	return c.metrics
}

// SetOnFullState sets callback for FULL_STATE messages
func (c *Client) SetOnFullState(callback func(*protocol.FullStateMessage)) {
	c.onFullState = callback
}

// SetOnSaveRequest sets callback for save requests (from ai-save command)
func (c *Client) SetOnSaveRequest(callback func()) {
	c.onSaveRequest = callback
}

// Stop gracefully shuts down the client
func (c *Client) Stop() error {
	// Log session metrics before shutdown
	metrics := c.GetSessionMetrics()
	if !metrics.ConnectionTime.IsZero() {
		duration := time.Since(metrics.ConnectionTime)
		c.logger.Info("session summary",
			logging.Field{Key: "duration", Value: duration.String()},
			logging.Field{Key: "messages_sent", Value: metrics.MessagesSent},
			logging.Field{Key: "messages_received", Value: metrics.MessagesReceived},
			logging.Field{Key: "bytes_sent", Value: metrics.BytesSent},
			logging.Field{Key: "bytes_received", Value: metrics.BytesReceived},
			logging.Field{Key: "errors", Value: metrics.ErrorCount})
	}

	close(c.stopCh)

	c.mu.Lock()
	if c.listener != nil {
		c.listener.Close()
		c.listener = nil
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	c.wg.Wait()

	c.logger.Info("client stopped")
	return nil
}
