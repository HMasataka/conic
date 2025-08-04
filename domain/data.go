package domain

import (
	"encoding/json"
	"time"

	"github.com/pion/webrtc/v4"
)

// MessageType represents the type of signaling message
type MessageType string

const (
	MessageTypeRegisterRequest    MessageType = "register_request"
	MessageTypeRegisterResponse   MessageType = "register_response"
	MessageTypeUnregisterRequest  MessageType = "unregister_request"
	MessageTypeUnregisterResponse MessageType = "unregister_response"
	MessageTypeSDP                MessageType = "sdp"
	MessageTypeCandidate          MessageType = "candidate"
	MessageTypeDataChannel        MessageType = "data_channel"
)

// Message represents a generic signaling message
type Message struct {
	ID        string          `json:"id"`
	Type      MessageType     `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// RegisterRequest represents a client registration request
type RegisterRequest struct {
	ClientID string `json:"client_id,omitempty"`
}

// RegisterResponse represents a registration response
type RegisterResponse struct {
	ClientID string `json:"client_id"`
	Success  bool   `json:"success"`
}

// SDPMessage represents an SDP exchange message
type SDPMessage struct {
	FromID             string                    `json:"from_id"`
	ToID               string                    `json:"to_id"`
	SessionDescription webrtc.SessionDescription `json:"session_description"`
}

// ICECandidateMessage represents an ICE candidate message
type ICECandidateMessage struct {
	FromID    string                  `json:"from_id"`
	ToID      string                  `json:"to_id"`
	Candidate webrtc.ICECandidateInit `json:"candidate"`
}

// DataChannelMessage represents a data channel message
type DataChannelMessage struct {
	FromID  string `json:"from_id"`
	ToID    string `json:"to_id"`
	Label   string `json:"label"`
	Payload []byte `json:"payload"`
}
