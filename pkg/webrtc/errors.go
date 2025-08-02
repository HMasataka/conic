package webrtc

import "errors"

// Common WebRTC errors
var (
	// ErrDataChannelNotOpen is returned when trying to send on a closed data channel
	ErrDataChannelNotOpen = errors.New("data channel is not open")
	
	// ErrPeerConnectionClosed is returned when operating on a closed peer connection
	ErrPeerConnectionClosed = errors.New("peer connection is closed")
	
	// ErrNoRemoteDescription is returned when remote description is not set
	ErrNoRemoteDescription = errors.New("remote description not set")
	
	// ErrInvalidSDP is returned when SDP is invalid
	ErrInvalidSDP = errors.New("invalid SDP")
	
	// ErrInvalidICECandidate is returned when ICE candidate is invalid
	ErrInvalidICECandidate = errors.New("invalid ICE candidate")
)