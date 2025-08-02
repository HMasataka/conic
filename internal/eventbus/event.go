package eventbus

import (
	"time"
)

// EventType represents the type of event
type EventType string

// Event types
const (
	EventClientConnected    EventType = "client.connected"
	EventClientDisconnected EventType = "client.disconnected"
	EventSDPReceived        EventType = "sdp.received"
	EventSDPSent            EventType = "sdp.sent"
	EventICECandidate       EventType = "ice.candidate"
	EventDataChannelOpen    EventType = "datachannel.open"
	EventDataChannelClose   EventType = "datachannel.close"
	EventDataChannelMessage EventType = "datachannel.message"
	EventError              EventType = "error"
)

// Event represents a system event
type Event struct {
	ID        string            `json:"id"`
	Type      EventType         `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	Source    string            `json:"source"`
	Data      interface{}       `json:"data"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// NewEvent creates a new event
func NewEvent(eventType EventType, source string, data interface{}) *Event {
	return &Event{
		ID:        generateID(),
		Type:      eventType,
		Timestamp: time.Now(),
		Source:    source,
		Data:      data,
		Metadata:  make(map[string]string),
	}
}

// WithMetadata adds metadata to the event
func (e *Event) WithMetadata(key, value string) *Event {
	if e.Metadata == nil {
		e.Metadata = make(map[string]string)
	}
	e.Metadata[key] = value
	return e
}

// generateID generates a unique event ID
func generateID() string {
	// Use a simple timestamp-based ID for now
	// In production, consider using a UUID library
	return time.Now().Format("20060102150405.999999999")
}
