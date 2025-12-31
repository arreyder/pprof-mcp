package transport

import (
	"context"
	"errors"
	"sync"
)

// Connection represents a basic client connection
type Connection interface {
	// ID returns a unique identifier for the connection
	ID() string

	// Send delivers a message to the client
	Send(ctx context.Context, message []byte) error

	// Close terminates the connection
	Close() error
}

// ConnectionManager tracks active client connections
type ConnectionManager interface {
	// AddConnection registers a new client connection
	AddConnection(ctx context.Context, conn Connection) error

	// RemoveConnection unregisters a client connection
	RemoveConnection(ctx context.Context, connID string) error

	// GetConnection retrieves a connection by ID
	GetConnection(ctx context.Context, connID string) (Connection, bool)

	// GetConnectionsBySession returns all connections for a given session ID
	GetConnectionsBySession(ctx context.Context, sessionID string) []Connection

	// Broadcast sends a message to all connections
	Broadcast(ctx context.Context, message []byte) error

	// BroadcastToSession sends a message to all connections for a session
	BroadcastToSession(ctx context.Context, sessionID string, message []byte) error
}

// InMemoryConnectionManager is a simple implementation of ConnectionManager
// that stores connections in memory. Suitable for single-instance deployments.
type InMemoryConnectionManager struct {
	mu                sync.RWMutex
	connections       map[string]Connection
	sessionToConnIDs  map[string][]string
	connIDToSessionID map[string]string
}

// NewInMemoryConnectionManager creates a new in-memory connection manager
func NewInMemoryConnectionManager() *InMemoryConnectionManager {
	return &InMemoryConnectionManager{
		connections:       make(map[string]Connection),
		sessionToConnIDs:  make(map[string][]string),
		connIDToSessionID: make(map[string]string),
	}
}

// AddConnection registers a new client connection
func (m *InMemoryConnectionManager) AddConnection(ctx context.Context, conn Connection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Store the connection
	connID := conn.ID()
	m.connections[connID] = conn

	// If connection has session info, associate it
	if sessionConn, ok := conn.(interface{ SessionID() string }); ok {
		sessionID := sessionConn.SessionID()
		if sessionID != "" {
			m.connIDToSessionID[connID] = sessionID
			m.sessionToConnIDs[sessionID] = append(m.sessionToConnIDs[sessionID], connID)
		}
	}

	return nil
}

// RemoveConnection unregisters a client connection
func (m *InMemoryConnectionManager) RemoveConnection(ctx context.Context, connID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if connection exists
	if _, exists := m.connections[connID]; !exists {
		return errors.New("connection not found")
	}

	// Remove session association if it exists
	if sessionID, ok := m.connIDToSessionID[connID]; ok {
		// Filter out this connection ID from the session's list
		connIDs := m.sessionToConnIDs[sessionID]
		newConnIDs := make([]string, 0, len(connIDs)-1)
		for _, id := range connIDs {
			if id != connID {
				newConnIDs = append(newConnIDs, id)
			}
		}

		// Update or clean up session mapping
		if len(newConnIDs) > 0 {
			m.sessionToConnIDs[sessionID] = newConnIDs
		} else {
			delete(m.sessionToConnIDs, sessionID)
		}

		delete(m.connIDToSessionID, connID)
	}

	// Remove the connection
	delete(m.connections, connID)

	return nil
}

// GetConnection retrieves a connection by ID
func (m *InMemoryConnectionManager) GetConnection(ctx context.Context, connID string) (Connection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, ok := m.connections[connID]
	return conn, ok
}

// GetConnectionsBySession returns all connections for a given session ID
func (m *InMemoryConnectionManager) GetConnectionsBySession(ctx context.Context, sessionID string) []Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get connection IDs for this session
	connIDs, ok := m.sessionToConnIDs[sessionID]
	if !ok {
		return nil
	}

	// Build list of connections
	conns := make([]Connection, 0, len(connIDs))
	for _, connID := range connIDs {
		if conn, ok := m.connections[connID]; ok {
			conns = append(conns, conn)
		}
	}

	return conns
}

// Broadcast sends a message to all connections
func (m *InMemoryConnectionManager) Broadcast(ctx context.Context, message []byte) error {
	m.mu.RLock()
	// Create a copy of connections to avoid holding the lock during sends
	conns := make([]Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		conns = append(conns, conn)
	}
	m.mu.RUnlock()

	// Send to all connections
	var lastErr error
	for _, conn := range conns {
		if err := conn.Send(ctx, message); err != nil {
			lastErr = err
			// Continue sending to other connections even if one fails
		}
	}

	return lastErr
}

// BroadcastToSession sends a message to all connections for a session
func (m *InMemoryConnectionManager) BroadcastToSession(ctx context.Context, sessionID string, message []byte) error {
	conns := m.GetConnectionsBySession(ctx, sessionID)
	if len(conns) == 0 {
		return errors.New("no connections found for session")
	}

	// Send to all connections in this session
	var lastErr error
	for _, conn := range conns {
		if err := conn.Send(ctx, message); err != nil {
			lastErr = err
			// Continue sending to other connections even if one fails
		}
	}

	return lastErr
}
