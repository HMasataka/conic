package signal

import (
	"encoding/json"

	"github.com/HMasataka/conic/hub"
	"github.com/rs/xid"
)

type MessageHandler interface {
	Handle(raw []byte, socket *socket) error
}

type RegisterHandler struct{}

func (h *RegisterHandler) Handle(raw []byte, s *socket) error {
	id := xid.New().String()

	s.hub.Register(hub.RegisterRequest{
		ID:     id,
		Client: s,
	})

	res := RegisterResponse{
		ID: id,
	}

	message, err := json.Marshal(res)
	if err != nil {
		return err
	}

	_, err = s.Write(message)
	return err
}

type UnregisterHandler struct{}

func (h *UnregisterHandler) Handle(raw []byte, s *socket) error {
	var unregister UnRegisterRequest

	if err := json.Unmarshal(raw, &unregister); err != nil {
		return err
	}

	s.hub.Unregister(hub.UnRegisterRequest{
		ID: unregister.ID,
	})

	return nil
}

type SDPHandler struct{}

func (h *SDPHandler) Handle(raw []byte, s *socket) error {
	var sdpRequest SessionDescriptionRequest

	if err := json.Unmarshal(raw, &sdpRequest); err != nil {
		return err
	}

	// 元のメッセージ全体を再構築
	fullMessage := struct {
		Type string `json:"type"`
		Raw  []byte `json:"raw"`
	}{
		Type: RequestTypeSDP,
		Raw:  raw,
	}

	message, err := json.Marshal(fullMessage)
	if err != nil {
		return err
	}

	s.hub.SendMessage(hub.MessageRequest{
		ID:       sdpRequest.ID,
		TargetID: sdpRequest.TargetID,
		Message:  message,
	})

	return nil
}

type CandidateHandler struct{}

func (h *CandidateHandler) Handle(raw []byte, s *socket) error {
	var candidateRequest CandidateRequest

	if err := json.Unmarshal(raw, &candidateRequest); err != nil {
		return err
	}

	// 元のメッセージ全体を再構築
	fullMessage := struct {
		Type string `json:"type"`
		Raw  []byte `json:"raw"`
	}{
		Type: RequestTypeCandidate,
		Raw:  raw,
	}

	message, err := json.Marshal(fullMessage)
	if err != nil {
		return err
	}

	s.hub.SendMessage(hub.MessageRequest{
		ID:       candidateRequest.ID,
		TargetID: candidateRequest.TargetID,
		Message:  message,
	})

	return nil
}
