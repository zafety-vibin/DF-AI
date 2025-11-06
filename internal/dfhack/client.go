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

// Client is the high-level interface for communicating with DFHack
type Client struct {
	logger     *logging.Logger
	listener   net.Listener
	conn         *protocol.Connection
	port         uint16
	mu           sync.RWMutex
	fullStateCh  chan *protocol.FullStateMessage
	tileUpdateCh chan *protocol.TileUpdateMessage
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewClient creates a new DFHack client
func NewClient(logger *logging.Logger) *Client {
	return &Client{
		logger:       logger,
		fullStateCh:  make(chan *protocol.FullStateMessage, 1),
		tileUpdateCh: make(chan *protocol.TileUpdateMessage, 100),
		stopCh:       make(chan struct{}),
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
			return
		}

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

		case *protocol.TileUpdateMessage:
			c.logger.Info("received tile update",
				logging.Field{Key: "changed_tiles", Value: m.Count})

			// Send to tile update channel (non-blocking)
			select {
			case c.tileUpdateCh <- m:
			default:
				c.logger.Warn("tile update channel full, dropping message")
			}

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

// IsConnected returns true if connection is active
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && c.conn.State() == protocol.StateConnected
}

// Stop gracefully shuts down the client
func (c *Client) Stop() error {
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
