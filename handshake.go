package conic

import (
	"sync"

	"github.com/pion/webrtc/v4"
)

func NewHandshake(config webrtc.Configuration, signalCandidate func(candidate *webrtc.ICECandidate) error) (*Handshake, error) {
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	pendingCandidates := make([]*webrtc.ICECandidate, 0)

	return &Handshake{
		peerConnection:    peerConnection,
		pendingCandidates: pendingCandidates,
		signalCandidate:   signalCandidate,
	}, nil
}

type Handshake struct {
	peerConnection    *webrtc.PeerConnection
	pendingCandidates []*webrtc.ICECandidate
	candidateMux      sync.Mutex
	signalCandidate   func(candidate *webrtc.ICECandidate) error
}

func (h *Handshake) Close() error {
	return h.peerConnection.Close()
}

func (h *Handshake) CreateOffer(op *webrtc.OfferOptions) (webrtc.SessionDescription, error) {
	return h.peerConnection.CreateOffer(op)
}

func (h *Handshake) SetRemoteDescription(s webrtc.SessionDescription) error {
	return h.peerConnection.SetRemoteDescription(s)
}

func (h *Handshake) SetLocalDescription(s webrtc.SessionDescription) error {
	return h.peerConnection.SetLocalDescription(s)
}

func (h *Handshake) AddIceCandidate(candidate webrtc.ICECandidateInit) error {
	return h.peerConnection.AddICECandidate(candidate)
}

func (h *Handshake) OnIceCandidate() {
	h.peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		h.candidateMux.Lock()
		defer h.candidateMux.Unlock()

		desc := h.peerConnection.RemoteDescription()
		if desc == nil {
			h.pendingCandidates = append(h.pendingCandidates, candidate)
		} else if onICECandidateErr := h.signalCandidate(candidate); onICECandidateErr != nil {
			panic(onICECandidateErr)
		}
	})
}

func (h *Handshake) SetOnConnectionStateChange(fn func(state webrtc.PeerConnectionState)) {
	h.peerConnection.OnConnectionStateChange(fn)
}

func (h *Handshake) HandlePendingCandidate() error {
	h.candidateMux.Lock()
	defer h.candidateMux.Unlock()

	for _, c := range h.pendingCandidates {
		if err := h.signalCandidate(c); err != nil {
			return err
		}
	}

	return nil
}

func (h *Handshake) GetPeerConnection() *webrtc.PeerConnection {
	return h.peerConnection
}
