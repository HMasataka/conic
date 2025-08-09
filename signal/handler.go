package signal

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/HMasataka/conic"
	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
	"github.com/rs/xid"
)

type RegisterRequestHandler struct {
	hub    domain.Hub
	logger *logging.Logger
}

func NewRegisterRequestHandler(hub domain.Hub, logger *logging.Logger) *RegisterRequestHandler {
	return &RegisterRequestHandler{
		hub:    hub,
		logger: logger,
	}
}

func (h *RegisterRequestHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	var req domain.RegisterRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.logger.Error("failed to unmarshal register request", "error", err)
		return nil, errors.New("failed to unmarshal register request")
	}

	conn, ok := conic.ConnectionFromContext(ctx)
	if !ok || conn == nil {
		h.logger.Error("connection not found in context")
		return nil, errors.New("connection not found")
	}

	socket := domain.NewClient(req.ClientID, conn)

	if err := h.hub.Register(socket); err != nil {
		h.logger.Error("failed to register client", "client_id", req.ClientID, "error", err)
		return nil, errors.New("failed to register client")
	}

	resp := domain.RegisterResponse{
		ClientID: req.ClientID,
		Success:  true,
	}

	respData, err := json.Marshal(resp)
	if err != nil {
		return nil, errors.New("failed to marshal register response: " + err.Error())
	}

	response := &domain.Message{
		ID:        xid.New().String(),
		Type:      domain.MessageTypeRegisterResponse,
		Timestamp: time.Now(),
		Data:      respData,
	}

	h.logger.Info("client registered", "client_id", req.ClientID)

	return response, nil
}

// CanHandle implements protocol.Handler
func (h *RegisterRequestHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeRegisterRequest
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
func (h *SDPHandler) Handle(ctx context.Context, message *domain.Message) (*domain.Message, error) {
	var sdpMsg domain.SDPMessage
	if err := json.Unmarshal(message.Data, &sdpMsg); err != nil {
		return nil, errors.New("failed to unmarshal SDP message")
	}

	// TODO domain.MessageがFromID, ToIDを持つようにする
	m, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("failed to marshal SDP message", "error", err)
		return nil, errors.New("failed to marshal SDP message")
	}

	if err := h.hub.SendTo(sdpMsg.ToID, m); err != nil {
		h.logger.Error("failed to send SDP message", "error", err, "to_id", sdpMsg.ToID)
		return nil, errors.New("failed to send SDP message")
	}

	h.logger.Debug("SDP forwarded",
		"from", sdpMsg.FromID,
		"to", sdpMsg.ToID,
		"type", sdpMsg.SessionDescription.Type,
	)

	return nil, nil
}

func (h *SDPHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeSDP
}

type ICECandidateHandler struct {
	hub    domain.Hub
	logger *logging.Logger
}

func NewICECandidateHandler(hub domain.Hub, logger *logging.Logger) *ICECandidateHandler {
	return &ICECandidateHandler{
		hub:    hub,
		logger: logger,
	}
}

func (h *ICECandidateHandler) Handle(ctx context.Context, message *domain.Message) (*domain.Message, error) {
	var iceMsg domain.ICECandidateMessage
	if err := json.Unmarshal(message.Data, &iceMsg); err != nil {
		h.logger.Error("failed to unmarshal ICE candidate message", "error", err)
		return nil, errors.New("failed to unmarshal ICE candidate message")
	}

	// TODO domain.MessageがFromID, ToIDを持つようにする
	m, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("failed to marshal SDP message", "error", err)
		return nil, errors.New("failed to marshal SDP message")
	}

	if err := h.hub.SendTo(iceMsg.ToID, m); err != nil {
		h.logger.Error("failed to send ICE candidate", "error", err, "to_id", iceMsg.ToID)
		return nil, errors.New("failed to send ICE candidate")
	}

	h.logger.Debug("ICE candidate forwarded",
		"from", iceMsg.FromID,
		"to", iceMsg.ToID,
	)

	return nil, nil
}

func (h *ICECandidateHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeCandidate
}

type DataChannelHandler struct {
	hub    domain.Hub
	logger *logging.Logger
}

func NewDataChannelHandler(hub domain.Hub, logger *logging.Logger) *DataChannelHandler {
	return &DataChannelHandler{
		hub:    hub,
		logger: logger,
	}
}

func (h *DataChannelHandler) Handle(ctx context.Context, message *domain.Message) (*domain.Message, error) {
	var dcMsg domain.DataChannelMessage
	if err := json.Unmarshal(message.Data, &dcMsg); err != nil {
		h.logger.Error("failed to unmarshal data channel message", "error", err)
		return nil, errors.New("failed to unmarshal data channel message")
	}

	// TODO domain.MessageがFromID, ToIDを持つようにする
	m, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("failed to marshal SDP message", "error", err)
		return nil, errors.New("failed to marshal SDP message")
	}

	if err := h.hub.SendTo(dcMsg.ToID, m); err != nil {
		h.logger.Error("failed to send data channel message", "error", err, "to_id", dcMsg.ToID)
		return nil, errors.New("failed to send data channel message")
	}

	h.logger.Debug("data channel message forwarded",
		"from", dcMsg.FromID,
		"to", dcMsg.ToID,
		"label", dcMsg.Label,
		"size", len(dcMsg.Payload),
	)

	return nil, nil
}

func (h *DataChannelHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeDataChannel
}
