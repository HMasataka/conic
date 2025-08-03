package signal

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
	"github.com/rs/xid"
)

// RegisterHandler handles client registration
type RegisterHandler struct {
	hub    domain.Hub
	logger *logging.Logger
}

// NewRegisterHandler creates a new register handler
func NewRegisterHandler(hub domain.Hub, logger *logging.Logger) *RegisterHandler {
	return &RegisterHandler{
		hub:    hub,
		logger: logger,
	}
}

// Handle implements protocol.Handler
func (h *RegisterHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	var req domain.RegisterRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return nil, errors.New("")
	}

	// Get client ID from context (set by websocket server)
	clientID, ok := ctx.Value("client_id").(string)
	if !ok || clientID == "" {
		clientID = xid.New().String()
	}

	// Create response
	resp := domain.RegisterResponse{
		ClientID: clientID,
		Success:  true,
	}

	respData, err := json.Marshal(resp)
	if err != nil {
		return nil, errors.New("")
	}

	response := &domain.Message{
		ID:   xid.New().String(),
		Type: domain.MessageTypeRegister,
		Data: respData,
	}

	h.logger.Info("client registered", "client_id", clientID)

	return response, nil
}

// CanHandle implements protocol.Handler
func (h *RegisterHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeRegister
}

// SDPHandler handles SDP exchange
type SDPHandler struct {
	hub    domain.Hub
	logger *logging.Logger
}

// NewSDPHandler creates a new SDP handler
func NewSDPHandler(hub domain.Hub, logger *logging.Logger) *SDPHandler {
	return &SDPHandler{
		hub:    hub,
		logger: logger,
	}
}

// Handle implements protocol.Handler
func (h *SDPHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	var sdpMsg domain.SDPMessage
	if err := json.Unmarshal(msg.Data, &sdpMsg); err != nil {
		return nil, errors.New("")
	}

	// Forward SDP to target client
	if err := h.hub.SendTo(sdpMsg.ToID, msg.Data); err != nil {
		return nil, errors.New("")
	}

	h.logger.Debug("SDP forwarded",
		"from", sdpMsg.FromID,
		"to", sdpMsg.ToID,
		"type", sdpMsg.SessionDescription.Type,
	)

	return nil, nil // No response needed for SDP forwarding
}

// CanHandle implements protocol.Handler
func (h *SDPHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeSDP
}

// ICECandidateHandler handles ICE candidate exchange
type ICECandidateHandler struct {
	hub    domain.Hub
	logger *logging.Logger
}

// NewICECandidateHandler creates a new ICE candidate handler
func NewICECandidateHandler(hub domain.Hub, logger *logging.Logger) *ICECandidateHandler {
	return &ICECandidateHandler{
		hub:    hub,
		logger: logger,
	}
}

// Handle implements protocol.Handler
func (h *ICECandidateHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	var iceMsg domain.ICECandidateMessage
	if err := json.Unmarshal(msg.Data, &iceMsg); err != nil {
		return nil, errors.New("")
	}

	// Forward ICE candidate to target client
	if err := h.hub.SendTo(iceMsg.ToID, msg.Data); err != nil {
		return nil, errors.New("")
	}

	h.logger.Debug("ICE candidate forwarded",
		"from", iceMsg.FromID,
		"to", iceMsg.ToID,
	)

	return nil, nil // No response needed
}

// CanHandle implements protocol.Handler
func (h *ICECandidateHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeCandidate
}

// DataChannelHandler handles data channel messages
type DataChannelHandler struct {
	hub    domain.Hub
	logger *logging.Logger
}

// NewDataChannelHandler creates a new data channel handler
func NewDataChannelHandler(hub domain.Hub, logger *logging.Logger) *DataChannelHandler {
	return &DataChannelHandler{
		hub:    hub,
		logger: logger,
	}
}

// Handle implements protocol.Handler
func (h *DataChannelHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	var dcMsg domain.DataChannelMessage
	if err := json.Unmarshal(msg.Data, &dcMsg); err != nil {
		return nil, errors.New("")
	}

	// Forward message to target client
	if err := h.hub.SendTo(dcMsg.ToID, msg.Data); err != nil {
		return nil, errors.New("")
	}

	h.logger.Debug("data channel message forwarded",
		"from", dcMsg.FromID,
		"to", dcMsg.ToID,
		"label", dcMsg.Label,
		"size", len(dcMsg.Payload),
	)

	return nil, nil // No response needed
}

// CanHandle implements protocol.Handler
func (h *DataChannelHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeDataChannel
}
