package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

// ConnectionState represents the state of a protocol connection
type ConnectionState int

const (
	// StateDisconnected means no active connection
	StateDisconnected ConnectionState = iota
	// StateConnecting means TCP handshake in progress
	StateConnecting
	// StateConnected means active connection, ready for messages
	StateConnected
	// StateError means connection failed, retry pending
	StateError
)

// String returns the string representation of the connection state
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// Connection manages a TCP connection using the binary protocol
type Connection struct {
	conn  net.Conn
	state ConnectionState
	mu    sync.RWMutex
}

// NewConnection creates a new unconnected Connection
func NewConnection() *Connection {
	return &Connection{
		state: StateDisconnected,
	}
}

// Connect establishes a TCP connection to the specified host and port
func (c *Connection) Connect(host string, port uint16) error {
	c.mu.Lock()
	c.state = StateConnecting
	c.mu.Unlock()

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		c.mu.Lock()
		c.state = StateError
		c.mu.Unlock()
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.mu.Lock()
	c.conn = conn
	c.state = StateConnected
	c.mu.Unlock()

	return nil
}

// AcceptConnection wraps an already-established TCP connection
// Used for server-side accept() scenarios
func (c *Connection) AcceptConnection(tcpConn net.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn = tcpConn
	c.state = StateConnected
}

// Send transmits a message over the connection
// Message is serialized with length-prefixed framing
func (c *Connection) Send(msg Message) error {
	c.mu.RLock()
	if c.state != StateConnected || c.conn == nil {
		c.mu.RUnlock()
		return fmt.Errorf("cannot send: connection state is %s", c.State())
	}
	conn := c.conn
	c.mu.RUnlock()

	// Serialize message
	data, err := SerializeMessage(msg)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}

	// Send complete message
	_, err = conn.Write(data)
	if err != nil {
		c.mu.Lock()
		c.state = StateError
		c.mu.Unlock()
		return fmt.Errorf("send failed: %w", err)
	}

	return nil
}

// Receive blocks until a message is received from the connection
// Returns the deserialized message or an error
func (c *Connection) Receive() (Message, error) {
	c.mu.RLock()
	if c.state != StateConnected || c.conn == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("cannot receive: connection state is %s", c.State())
	}
	conn := c.conn
	c.mu.RUnlock()

	// Read 6-byte header first
	header := make([]byte, 6)
	if _, err := io.ReadFull(conn, header); err != nil {
		c.mu.Lock()
		c.state = StateError
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Parse length from header
	length := binary.BigEndian.Uint32(header[0:4])

	// Validate length is reasonable
	if length < 6 || length > 100*1024*1024 { // max 100 MB (for large forts)
		return nil, fmt.Errorf("%w: %d bytes", ErrInvalidMessageLength, length)
	}

	// Allocate buffer for full message
	fullMsg := make([]byte, length)
	copy(fullMsg, header)

	// Read remaining payload
	if length > 6 {
		remaining := fullMsg[6:]
		if _, err := io.ReadFull(conn, remaining); err != nil {
			c.mu.Lock()
			c.state = StateError
			c.mu.Unlock()
			return nil, fmt.Errorf("failed to read payload: %w", err)
		}
	}

	// Deserialize message
	msg, err := DeserializeMessage(fullMsg)
	if err != nil {
		return nil, fmt.Errorf("deserialization failed: %w", err)
	}

	return msg, nil
}

// State returns the current connection state
func (c *Connection) State() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Close gracefully closes the connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		// Send graceful disconnect message (best effort, ignore errors)
		disconnectMsg := &DisconnectMessage{
			Reason: ReasonNormalShutdown,
		}
		if data, err := SerializeMessage(disconnectMsg); err == nil {
			_, _ = c.conn.Write(data)
		}

		err := c.conn.Close()
		c.conn = nil
		c.state = StateDisconnected
		return err
	}

	c.state = StateDisconnected
	return nil
}
