package webrtc

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
func DefaultPeerConnectionOptions(logger *logging.Logger) PeerConnectionOptions {
	return PeerConnectionOptions{
		Logger: logger,
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
	id       string
	targetID string
	pc       *webrtc.PeerConnection
	logger   *logging.Logger
	options  PeerConnectionOptions

	pendingCandidates []webrtc.ICECandidateInit
	candidatesMu      sync.Mutex

	audioTracks   map[string]*AudioTrack
	audioTracksMu sync.RWMutex

	onICECandidate    func(*webrtc.ICECandidate) error
	onDataChannel     func(*webrtc.DataChannel)
	onConnectionState func(webrtc.PeerConnectionState)
	onTrack           func(*webrtc.TrackRemote, *webrtc.RTPReceiver)

	ctx    context.Context
	cancel context.CancelFunc
}

// NewPeerConnection creates a new peer connection
func NewPeerConnection(id string, options PeerConnectionOptions) (*PeerConnection, error) {
	// Create media engine with Opus support
	mediaEngine, err := CreateOpusMediaEngine()
	if err != nil {
		return nil, errors.New("failed to create media engine: " + err.Error())
	}

	// Create settings engine with larger buffer sizes
	settingEngine := webrtc.SettingEngine{}
	settingEngine.SetReceiveMTU(8192) // Increase MTU for larger packets

	// Create API with custom media and setting engines
	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithSettingEngine(settingEngine),
	)

	config := webrtc.Configuration{
		ICEServers: options.ICEServers,
	}

	pc, err := api.NewPeerConnection(config)
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
		audioTracks:       make(map[string]*AudioTrack),
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

func (p *PeerConnection) TargetID() string {
	return p.targetID
}

func (p *PeerConnection) SetTargetID(id string) {
	p.targetID = id
	p.logger.Debug("set target ID", "peer_id", p.id, "target_id", id)
}

// Close closes the peer connection
func (p *PeerConnection) Close() error {
	p.cancel()

	// Close all audio tracks
	p.audioTracksMu.Lock()
	for _, track := range p.audioTracks {
		track.Close()
	}
	p.audioTracks = nil
	p.audioTracksMu.Unlock()

	return p.pc.Close()
}

// CreateOffer creates an SDP offer
func (p *PeerConnection) CreateOffer(options *webrtc.OfferOptions) (webrtc.SessionDescription, error) {
	p.logger.Debug("creating offer", "peer_id", p.id)
	offer, err := p.pc.CreateOffer(options)
	if err != nil {
		return webrtc.SessionDescription{}, errors.New("failed to create offer: " + err.Error())
	}

	if err := p.pc.SetLocalDescription(offer); err != nil {
		return webrtc.SessionDescription{}, errors.New("failed to set local description: " + err.Error())
	}

	<-webrtc.GatheringCompletePromise(p.pc)

	p.logger.Debug("created offer", "peer_id", p.id)

	return offer, nil
}

// CreateAnswer creates an SDP answer
func (p *PeerConnection) CreateAnswer(options *webrtc.AnswerOptions) (webrtc.SessionDescription, error) {
	answer, err := p.pc.CreateAnswer(options)
	if err != nil {
		return webrtc.SessionDescription{}, errors.New("failed to create answer: " + err.Error())
	}

	if err := p.pc.SetLocalDescription(answer); err != nil {
		return webrtc.SessionDescription{}, errors.New("failed to set local description: " + err.Error())
	}

	p.logger.Debug("created answer", "peer_id", p.id)

	return answer, nil
}

// SetRemoteDescription sets the remote SDP
func (p *PeerConnection) SetRemoteDescription(sdp webrtc.SessionDescription) error {
	if err := p.pc.SetRemoteDescription(sdp); err != nil {
		return errors.New("failed to set remote description: " + err.Error())
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
		return errors.New("failed to add ICE candidate: " + err.Error())
	}

	p.logger.Debug("added ICE candidate", "peer_id", p.id)

	return nil
}

// CreateDataChannel creates a new data channel
func (p *PeerConnection) CreateDataChannel(label string, options *webrtc.DataChannelInit) (*DataChannel, error) {
	dc, err := p.pc.CreateDataChannel(label, options)
	if err != nil {
		return nil, errors.New("failed to create data channel: " + err.Error())
	}

	dataChannel := NewDataChannel(dc, p.logger)

	p.logger.Info("created data channel", "peer_id", p.id, "label", label)

	return dataChannel, nil
}

// AddAudioTrack adds an audio track to the peer connection
func (p *PeerConnection) AddAudioTrack(track *AudioTrack) (*webrtc.RTPSender, error) {
	sender, err := p.pc.AddTrack(track.LocalTrack())
	if err != nil {
		return nil, errors.New("failed to add audio track: " + err.Error())
	}

	p.audioTracksMu.Lock()
	p.audioTracks[track.ID()] = track
	p.audioTracksMu.Unlock()

	p.logger.Info("added audio track", "peer_id", p.id, "track_id", track.ID())

	return sender, nil
}

// RemoveAudioTrack removes an audio track from the peer connection
func (p *PeerConnection) RemoveAudioTrack(trackID string) error {
	p.audioTracksMu.Lock()
	track, exists := p.audioTracks[trackID]
	if !exists {
		p.audioTracksMu.Unlock()
		return errors.New("audio track not found")
	}
	delete(p.audioTracks, trackID)
	p.audioTracksMu.Unlock()

	track.Close()
	p.logger.Info("removed audio track", "peer_id", p.id, "track_id", trackID)

	return nil
}

// GetAudioTrack returns an audio track by ID
func (p *PeerConnection) GetAudioTrack(trackID string) (*AudioTrack, bool) {
	p.audioTracksMu.RLock()
	defer p.audioTracksMu.RUnlock()
	track, exists := p.audioTracks[trackID]
	return track, exists
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

// OnTrack sets the track handler for incoming media
func (p *PeerConnection) OnTrack(handler func(*webrtc.TrackRemote, *webrtc.RTPReceiver)) {
	p.onTrack = handler
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

	// Track handler for incoming audio/video
	p.pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		p.logger.Info("track received",
			"peer_id", p.id,
			"track_id", track.ID(),
			"kind", track.Kind().String(),
			"codec", track.Codec().MimeType,
		)

		if track.Kind() == webrtc.RTPCodecTypeAudio {
			// Create audio track wrapper
			audioTrack, err := NewAudioTrack(track.ID(), track.Codec().RTPCodecCapability)
			if err != nil {
				p.logger.Error("failed to create audio track", "error", err)
				return
			}

			audioTrack.SetRemoteTrack(track)

			p.audioTracksMu.Lock()
			p.audioTracks[track.ID()] = audioTrack
			p.audioTracksMu.Unlock()

			// Start reading samples in background
			go func() {
				if err := audioTrack.ReadSamples(p.ctx); err != nil {
					p.logger.Error("error reading audio samples", "error", err)
				}
			}()
		}

		if p.onTrack != nil {
			p.onTrack(track, receiver)
		}
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
