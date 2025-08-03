package conic

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/HMasataka/conic/logging"
	"github.com/pion/webrtc/v4"
)

// PeerConnectionOptions represents options for peer connection
type PeerConnectionOptions struct {
	ICEServers          []webrtc.ICEServer
	Logger              *logging.Logger
	ICECandidateTimeout time.Duration
}

// DefaultPeerConnectionOptions returns default options
func DefaultPeerConnectionOptions() PeerConnectionOptions {
	return PeerConnectionOptions{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
		ICECandidateTimeout: 30 * time.Second,
	}
}

// PeerConnection wraps a WebRTC peer connection
type PeerConnection struct {
	id      string
	pc      *webrtc.PeerConnection
	logger  *logging.Logger
	options PeerConnectionOptions

	pendingCandidates []webrtc.ICECandidateInit
	candidatesMu      sync.Mutex

	onICECandidate    func(*webrtc.ICECandidate) error
	onDataChannel     func(*webrtc.DataChannel)
	onConnectionState func(webrtc.PeerConnectionState)

	ctx    context.Context
	cancel context.CancelFunc
}

// NewPeerConnection creates a new peer connection
func NewPeerConnection(id string, options PeerConnectionOptions) (*PeerConnection, error) {
	config := webrtc.Configuration{
		ICEServers: options.ICEServers,
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, errors.New("failed to create peer connection: " + err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &PeerConnection{
		id:                id,
		pc:                pc,
		logger:            options.Logger,
		options:           options,
		pendingCandidates: make([]webrtc.ICECandidateInit, 0),
		ctx:               ctx,
		cancel:            cancel,
	}

	// Set up event handlers
	p.setupEventHandlers()

	return p, nil
}

// ID returns the peer connection ID
func (p *PeerConnection) ID() string {
	return p.id
}

// Close closes the peer connection
func (p *PeerConnection) Close() error {
	p.cancel()
	return p.pc.Close()
}

// CreateOffer creates an SDP offer
func (p *PeerConnection) CreateOffer(options *webrtc.OfferOptions) (webrtc.SessionDescription, error) {
	offer, err := p.pc.CreateOffer(options)
	if err != nil {
		return webrtc.SessionDescription{}, errors.New("")
	}

	if err := p.pc.SetLocalDescription(offer); err != nil {
		return webrtc.SessionDescription{}, errors.New("")
	}

	p.logger.Debug("created offer", "peer_id", p.id)

	return offer, nil
}

// CreateAnswer creates an SDP answer
func (p *PeerConnection) CreateAnswer(options *webrtc.AnswerOptions) (webrtc.SessionDescription, error) {
	answer, err := p.pc.CreateAnswer(options)
	if err != nil {
		return webrtc.SessionDescription{}, errors.New("")
	}

	if err := p.pc.SetLocalDescription(answer); err != nil {
		return webrtc.SessionDescription{}, errors.New("")
	}

	p.logger.Debug("created answer", "peer_id", p.id)

	return answer, nil
}

// SetRemoteDescription sets the remote SDP
func (p *PeerConnection) SetRemoteDescription(sdp webrtc.SessionDescription) error {
	if err := p.pc.SetRemoteDescription(sdp); err != nil {
		return errors.New("")
	}

	p.logger.Debug("set remote description", "peer_id", p.id, "type", sdp.Type)

	// Process pending ICE candidates if any
	p.processPendingCandidates()

	return nil
}

// AddICECandidate adds an ICE candidate
func (p *PeerConnection) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	// If remote description is not set yet, queue the candidate
	if p.pc.RemoteDescription() == nil {
		p.candidatesMu.Lock()
		p.pendingCandidates = append(p.pendingCandidates, candidate)
		p.candidatesMu.Unlock()
		p.logger.Debug("queued ICE candidate", "peer_id", p.id)
		return nil
	}

	if err := p.pc.AddICECandidate(candidate); err != nil {
		return errors.New("")
	}

	p.logger.Debug("added ICE candidate", "peer_id", p.id)

	return nil
}

// CreateDataChannel creates a new data channel
func (p *PeerConnection) CreateDataChannel(label string, options *webrtc.DataChannelInit) (*DataChannel, error) {
	dc, err := p.pc.CreateDataChannel(label, options)
	if err != nil {
		return nil, errors.New("")
	}

	dataChannel := NewDataChannel(dc, p.logger)

	p.logger.Info("created data channel", "peer_id", p.id, "label", label)

	return dataChannel, nil
}

// OnICECandidate sets the ICE candidate handler
func (p *PeerConnection) OnICECandidate(handler func(*webrtc.ICECandidate) error) {
	p.onICECandidate = handler
}

// OnDataChannel sets the data channel handler
func (p *PeerConnection) OnDataChannel(handler func(*webrtc.DataChannel)) {
	p.onDataChannel = handler
}

// OnConnectionStateChange sets the connection state change handler
func (p *PeerConnection) OnConnectionStateChange(handler func(webrtc.PeerConnectionState)) {
	p.onConnectionState = handler
}

// GetStats returns peer connection statistics
func (p *PeerConnection) GetStats() webrtc.StatsReport {
	return p.pc.GetStats()
}

// ConnectionState returns the current connection state
func (p *PeerConnection) ConnectionState() webrtc.PeerConnectionState {
	return p.pc.ConnectionState()
}

// setupEventHandlers sets up WebRTC event handlers
func (p *PeerConnection) setupEventHandlers() {
	// ICE candidate handler
	p.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		p.logger.Debug("ICE candidate gathered", "peer_id", p.id)

		if p.onICECandidate != nil {
			if err := p.onICECandidate(candidate); err != nil {
				p.logger.Error("ICE candidate handler error", "peer_id", p.id, "error", err)
			}
		}
	})

	// Data channel handler
	p.pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		p.logger.Info("data channel received", "peer_id", p.id, "label", dc.Label())

		if p.onDataChannel != nil {
			p.onDataChannel(dc)
		}

	})

	// Connection state handler
	p.pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		p.logger.Info("connection state changed", "peer_id", p.id, "state", state.String())

		if p.onConnectionState != nil {
			p.onConnectionState(state)
		}

	})

	// ICE connection state handler
	p.pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		p.logger.Debug("ICE connection state changed", "peer_id", p.id, "state", state.String())
	})

	// Signaling state handler
	p.pc.OnSignalingStateChange(func(state webrtc.SignalingState) {
		p.logger.Debug("signaling state changed", "peer_id", p.id, "state", state.String())
	})
}

// processPendingCandidates processes queued ICE candidates
func (p *PeerConnection) processPendingCandidates() {
	p.candidatesMu.Lock()
	candidates := p.pendingCandidates
	p.pendingCandidates = nil
	p.candidatesMu.Unlock()

	for _, candidate := range candidates {
		if err := p.pc.AddICECandidate(candidate); err != nil {
			p.logger.Error("failed to add pending ICE candidate", "peer_id", p.id, "error", err)
		} else {
			p.logger.Debug("added pending ICE candidate", "peer_id", p.id)
		}
	}
}
