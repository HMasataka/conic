package conic

import (
	"context"
	"encoding/json"

	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
	"github.com/pion/webrtc/v4"
	"github.com/rs/xid"
)

type RegisterResponseHandler struct {
	logger *logging.Logger
}

func NewRegisterHandler(logger *logging.Logger) *RegisterResponseHandler {
	return &RegisterResponseHandler{
		logger: logger,
	}
}

func (h *RegisterResponseHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	h.logger.Debug("message data", "data", string(msg.Data))
	return nil, nil
}

func (h *RegisterResponseHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeRegisterResponse
}

type UnregisterResponseHandler struct {
	logger *logging.Logger
}

func NewUnregisterHandler(logger *logging.Logger) *UnregisterResponseHandler {
	return &UnregisterResponseHandler{
		logger: logger,
	}
}

func (h *UnregisterResponseHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	h.logger.Debug("message data", "data", string(msg.Data))
	return nil, nil
}

func (h *UnregisterResponseHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeUnregisterResponse
}

type SessionDescriptionHandler struct {
	clientID string
	pc       *PeerConnection
	logger   *logging.Logger
}

func NewSessionDescriptionHandler(clientID string, pc *PeerConnection, logger *logging.Logger) *SessionDescriptionHandler {
	return &SessionDescriptionHandler{
		clientID: clientID,
		pc:       pc,
		logger:   logger,
	}
}

func (h *SessionDescriptionHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	var sdpMsg domain.SDPMessage

	if err := json.Unmarshal(msg.Data, &sdpMsg); err != nil {
		return nil, err
	}

	if err := h.pc.SetRemoteDescription(sdpMsg.SessionDescription); err != nil {
		return nil, err
	}

	h.pc.SetTargetID(sdpMsg.FromID)

	if sdpMsg.SessionDescription.Type == webrtc.SDPTypeAnswer {
		return nil, nil
	}

	answer, err := h.pc.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(domain.SDPMessage{
		FromID:             h.clientID,
		ToID:               sdpMsg.FromID,
		SessionDescription: answer,
	})
	if err != nil {
		return nil, err
	}

	response := &domain.Message{
		ID:   xid.New().String(),
		Type: domain.MessageTypeSDP,
		Data: data,
	}

	h.logger.Debug("message data", "data", string(msg.Data))

	return response, nil
}

func (h *SessionDescriptionHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeSDP
}

type CandidateHandler struct {
	clientID string
	pc       *PeerConnection
	logger   *logging.Logger
}

func NewCandidateHandler(pc *PeerConnection, logger *logging.Logger) *CandidateHandler {
	return &CandidateHandler{
		pc:     pc,
		logger: logger,
	}
}

func (h *CandidateHandler) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	var candidateMsg domain.ICECandidateMessage

	if err := json.Unmarshal(msg.Data, &candidateMsg); err != nil {
		return nil, err
	}

	if err := h.pc.AddICECandidate(candidateMsg.Candidate); err != nil {
		return nil, err
	}

	h.logger.Debug("message data", "data", string(msg.Data))

	return nil, nil
}

func (h *CandidateHandler) CanHandle(messageType domain.MessageType) bool {
	return messageType == domain.MessageTypeCandidate
}
