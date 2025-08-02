package signaling

import (
	"context"
	"encoding/json"

	"github.com/HMasataka/conic/internal/eventbus"
	"github.com/HMasataka/conic/internal/logging"
	"github.com/HMasataka/conic/pkg/domain"
	"github.com/HMasataka/conic/pkg/errors"
	"github.com/rs/xid"
)

// RegisterHandler handles client registration
type RegisterHandler struct {
	hub      domain.Hub
	logger   *logging.Logger
	eventBus eventbus.Bus
}

// NewRegisterHandler creates a new register handler
func NewRegisterHandler(hub domain.Hub, logger *logging.Logger, eventBus eventbus.Bus) *RegisterHandler {
	return &RegisterHandler{
		hub:      hub,
		logger:   logger,
		eventBus: eventBus,
	}
}

// Handle implements protocol.Handler
func (h *RegisterHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	var req domain.RegisterRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeValidation, "INVALID_REQUEST", "failed to unmarshal register request")
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
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "MARSHAL_ERROR", "failed to marshal response")
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
	hub      domain.Hub
	logger   *logging.Logger
	eventBus eventbus.Bus
}

// NewSDPHandler creates a new SDP handler
func NewSDPHandler(hub domain.Hub, logger *logging.Logger, eventBus eventbus.Bus) *SDPHandler {
	return &SDPHandler{
		hub:      hub,
		logger:   logger,
		eventBus: eventBus,
	}
}

// Handle implements protocol.Handler
func (h *SDPHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	var sdpMsg domain.SDPMessage
	if err := json.Unmarshal(msg.Data, &sdpMsg); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeValidation, "INVALID_SDP", "failed to unmarshal SDP message")
	}

	// Forward SDP to target client
	if err := h.hub.SendTo(sdpMsg.ToID, msg.Data); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "FORWARD_ERROR", "failed to forward SDP")
	}

	// Publish event
	if h.eventBus != nil {
		event := eventbus.NewEvent(
			eventbus.EventSDPReceived,
			"sdp-handler",
			sdpMsg,
		).WithMetadata("from_id", sdpMsg.FromID).
			WithMetadata("to_id", sdpMsg.ToID).
			WithMetadata("sdp_type", string(sdpMsg.SessionDescription.Type))

		h.eventBus.PublishAsync(event)
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
	hub      domain.Hub
	logger   *logging.Logger
	eventBus eventbus.Bus
}

// NewICECandidateHandler creates a new ICE candidate handler
func NewICECandidateHandler(hub domain.Hub, logger *logging.Logger, eventBus eventbus.Bus) *ICECandidateHandler {
	return &ICECandidateHandler{
		hub:      hub,
		logger:   logger,
		eventBus: eventBus,
	}
}

// Handle implements protocol.Handler
func (h *ICECandidateHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	var iceMsg domain.ICECandidateMessage
	if err := json.Unmarshal(msg.Data, &iceMsg); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeValidation, "INVALID_ICE", "failed to unmarshal ICE candidate")
	}

	// Forward ICE candidate to target client
	if err := h.hub.SendTo(iceMsg.ToID, msg.Data); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "FORWARD_ERROR", "failed to forward ICE candidate")
	}

	// Publish event
	if h.eventBus != nil {
		event := eventbus.NewEvent(
			eventbus.EventICECandidate,
			"ice-handler",
			iceMsg,
		).WithMetadata("from_id", iceMsg.FromID).
			WithMetadata("to_id", iceMsg.ToID)

		h.eventBus.PublishAsync(event)
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
	hub      domain.Hub
	logger   *logging.Logger
	eventBus eventbus.Bus
}

// NewDataChannelHandler creates a new data channel handler
func NewDataChannelHandler(hub domain.Hub, logger *logging.Logger, eventBus eventbus.Bus) *DataChannelHandler {
	return &DataChannelHandler{
		hub:      hub,
		logger:   logger,
		eventBus: eventBus,
	}
}

// Handle implements protocol.Handler
func (h *DataChannelHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	var dcMsg domain.DataChannelMessage
	if err := json.Unmarshal(msg.Data, &dcMsg); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeValidation, "INVALID_DC_MSG", "failed to unmarshal data channel message")
	}

	// Forward message to target client
	if err := h.hub.SendTo(dcMsg.ToID, msg.Data); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "FORWARD_ERROR", "failed to forward data channel message")
	}

	// Publish event
	if h.eventBus != nil {
		event := eventbus.NewEvent(
			eventbus.EventDataChannelMessage,
			"datachannel-handler",
			dcMsg,
		).WithMetadata("from_id", dcMsg.FromID).
			WithMetadata("to_id", dcMsg.ToID).
			WithMetadata("label", dcMsg.Label)

		h.eventBus.PublishAsync(event)
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
