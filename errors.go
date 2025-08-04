package conic

import "errors"

var (
	// ErrDataChannelNotOpen is returned when trying to send on a closed data channel
	ErrDataChannelNotOpen = errors.New("data channel is not open")

	// ErrPeerConnectionClosed is returned when peer connection is closed
	ErrPeerConnectionClosed = errors.New("peer connection is closed")

	// ErrPeerNotFound is returned when peer is not found
	ErrPeerNotFound = errors.New("peer not found")
)
