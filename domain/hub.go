package domain

import "context"

type HubStats struct {
	ConnectedClients int     `json:"connected_clients"`
	MessagesSent     int64   `json:"messages_sent"`
	MessagesReceived int64   `json:"messages_received"`
	Uptime           float64 `json:"uptime_seconds"`
}

type Hub interface {
	// Start starts the hub
	Start(ctx context.Context) error

	// Stop stops the hub gracefully
	Stop() error

	// Register registers a new client
	Register(client Client) error

	// Unregister removes a client
	Unregister(clientID string) error

	// Broadcast sends a message to all connected clients
	Broadcast(message []byte) error

	// SendTo sends a message to a specific client
	SendTo(clientID string, message []byte) error

	// SendToMultiple sends a message to multiple clients
	SendToMultiple(clientIDs []string, message []byte) error

	// GetClient retrieves a client by ID
	GetClient(clientID string) (Client, bool)

	// GetClients returns all connected clients
	GetClients() []Client
}
