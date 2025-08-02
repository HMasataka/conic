package webrtc

import (
	"context"
	"sync"

	"github.com/HMasataka/conic/internal/eventbus"
	"github.com/HMasataka/conic/internal/logging"
	"github.com/HMasataka/conic/pkg/errors"
	"github.com/pion/webrtc/v4"
)

// Manager manages WebRTC peer connections
type Manager struct {
	peers    map[string]*PeerConnection
	mu       sync.RWMutex
	logger   *logging.Logger
	eventBus eventbus.Bus
	options  PeerConnectionOptions
}

// NewManager creates a new WebRTC manager
func NewManager(logger *logging.Logger, eventBus eventbus.Bus, options PeerConnectionOptions) *Manager {
	if logger == nil {
		logger = logging.New(logging.Config{Level: "info", Format: "text"})
	}

	return &Manager{
		peers:    make(map[string]*PeerConnection),
		logger:   logger,
		eventBus: eventBus,
		options:  options,
	}
}

// CreatePeerConnection creates a new peer connection
func (m *Manager) CreatePeerConnection(peerID string) (*PeerConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if peer already exists
	if _, exists := m.peers[peerID]; exists {
		return nil, errors.New(errors.ErrorTypeWebRTC, "PEER_EXISTS", "peer connection already exists")
	}

	// Create peer connection
	pc, err := NewPeerConnection(peerID, m.options)
	if err != nil {
		return nil, err
	}

	// Store peer connection
	m.peers[peerID] = pc

	m.logger.Info("created peer connection", "peer_id", peerID)

	return pc, nil
}

// GetPeerConnection retrieves a peer connection
func (m *Manager) GetPeerConnection(peerID string) (*PeerConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pc, exists := m.peers[peerID]
	if !exists {
		return nil, errors.New(errors.ErrorTypeNotFound, "PEER_NOT_FOUND", "peer connection not found")
	}

	return pc, nil
}

// RemovePeerConnection removes and closes a peer connection
func (m *Manager) RemovePeerConnection(peerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pc, exists := m.peers[peerID]
	if !exists {
		return errors.New(errors.ErrorTypeNotFound, "PEER_NOT_FOUND", "peer connection not found")
	}

	// Close peer connection
	if err := pc.Close(); err != nil {
		m.logger.Error("failed to close peer connection", "peer_id", peerID, "error", err)
	}

	// Remove from map
	delete(m.peers, peerID)

	m.logger.Info("removed peer connection", "peer_id", peerID)

	return nil
}

// CloseAll closes all peer connections
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for peerID, pc := range m.peers {
		if err := pc.Close(); err != nil {
			m.logger.Error("failed to close peer connection", "peer_id", peerID, "error", err)
		}
	}

	// Clear the map
	m.peers = make(map[string]*PeerConnection)

	m.logger.Info("closed all peer connections")
}

// GetPeerCount returns the number of active peer connections
func (m *Manager) GetPeerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.peers)
}

// GetPeerIDs returns all peer IDs
func (m *Manager) GetPeerIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.peers))
	for id := range m.peers {
		ids = append(ids, id)
	}

	return ids
}

// HandleOffer handles an incoming SDP offer
func (m *Manager) HandleOffer(ctx context.Context, peerID string, offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	// Get or create peer connection
	pc, err := m.GetPeerConnection(peerID)
	if err != nil {
		// Create new peer connection if not exists
		pc, err = m.CreatePeerConnection(peerID)
		if err != nil {
			return webrtc.SessionDescription{}, err
		}
	}

	// Set remote description
	if err := pc.SetRemoteDescription(offer); err != nil {
		return webrtc.SessionDescription{}, err
	}

	// Create answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	return answer, nil
}

// HandleAnswer handles an incoming SDP answer
func (m *Manager) HandleAnswer(ctx context.Context, peerID string, answer webrtc.SessionDescription) error {
	pc, err := m.GetPeerConnection(peerID)
	if err != nil {
		return err
	}

	return pc.SetRemoteDescription(answer)
}

// HandleICECandidate handles an incoming ICE candidate
func (m *Manager) HandleICECandidate(ctx context.Context, peerID string, candidate webrtc.ICECandidateInit) error {
	pc, err := m.GetPeerConnection(peerID)
	if err != nil {
		return err
	}

	return pc.AddICECandidate(candidate)
}
