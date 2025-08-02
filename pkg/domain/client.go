package domain

import (
	"context"
	"io"
)

// Client represents a connected client interface
type Client interface {
	// ID returns the unique identifier of the client
	ID() string

	// Send sends a message to the client
	Send(ctx context.Context, message []byte) error

	// Receive sets up a message handler for incoming messages
	Receive(handler MessageHandler) error

	// Close closes the client connection
	Close() error

	// Context returns the client's context
	Context() context.Context
}

// MessageHandler is a function that handles incoming messages
type MessageHandler func(message []byte) error

// ClientManager manages client lifecycle
type ClientManager interface {
	// Add registers a new client
	Add(client Client) error

	// Remove unregisters a client
	Remove(clientID string) error

	// Get retrieves a client by ID
	Get(clientID string) (Client, bool)

	// List returns all connected clients
	List() []Client

	// Count returns the number of connected clients
	Count() int
}

// ClientFactory creates new client instances
type ClientFactory interface {
	// Create creates a new client with the given connection
	Create(conn io.ReadWriteCloser) (Client, error)
}