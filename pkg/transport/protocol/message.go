package protocol

import (
	"encoding/json"
	"time"

	"github.com/HMasataka/conic/pkg/domain"
)

// Frame represents a transport-level message frame
type Frame struct {
	Version   string          `json:"version"`
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// NewFrame creates a new frame
func NewFrame(messageType string, payload interface{}) (*Frame, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Frame{
		Version:   "1.0",
		Type:      messageType,
		ID:        generateID(),
		Timestamp: time.Now(),
		Payload:   data,
	}, nil
}

// Decode decodes the frame payload into the provided interface
func (f *Frame) Decode(v interface{}) error {
	return json.Unmarshal(f.Payload, v)
}

// Marshal marshals the frame to bytes
func (f *Frame) Marshal() ([]byte, error) {
	return json.Marshal(f)
}

// Unmarshal unmarshals bytes into a frame
func Unmarshal(data []byte) (*Frame, error) {
	var f Frame
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// generateID generates a unique ID for a frame
func generateID() string {
	return time.Now().Format("20060102150405.999999999")
}

// Codec defines the interface for message encoding/decoding
type Codec interface {
	// Encode encodes a domain message to bytes
	Encode(msg domain.Message) ([]byte, error)

	// Decode decodes bytes to a domain message
	Decode(data []byte) (*domain.Message, error)
}

// JSONCodec implements Codec using JSON
type JSONCodec struct{}

// NewJSONCodec creates a new JSON codec
func NewJSONCodec() *JSONCodec {
	return &JSONCodec{}
}

// Encode implements the Codec interface
func (c *JSONCodec) Encode(msg domain.Message) ([]byte, error) {
	return json.Marshal(msg)
}

// Decode implements the Codec interface
func (c *JSONCodec) Decode(data []byte) (*domain.Message, error) {
	var msg domain.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}