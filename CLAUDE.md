# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Conic is a Go-based WebRTC signaling server that facilitates real-time peer-to-peer communications. It uses WebSocket connections for signaling and includes both client and server implementations.

## Development Commands

### Task Runner Commands (using Taskfile)

- **Run server**: `task server` or `go run cmd/server/main.go`
- **Run client**: `task client` or `go run cmd/client/main.go`
- **Run signal**: `task signal` or `go run cmd/signal/main.go`
- **View available tasks**: `task --list`
- **Build all**: `task build`
- **Run tests**: `task test`
- **Format code**: `task fmt`
- **Vet code**: `task vet`
- **Tidy dependencies**: `task tidy`
- **Clean build artifacts**: `task clean`
- **Install development tools**: `task install-tools`
- **Dev server with hot reload**: `task dev-server` (requires air)

Note: No test files currently exist in the codebase.

## Architecture Overview

### Core Components

1. **Hub (`hub.go`)** - Central message routing system
   - Manages client registration/unregistration
   - Routes messages between connected clients
   - Uses channels for concurrent operation
   - Interface: `Hub` with implementation `hub`

2. **WebSocket Server (`websocket.go`)** - WebSocket connection handling
   - Upgrades HTTP connections to WebSocket
   - Handles client registration and message routing
   - Supports WebRTC signaling messages (SDP, ICE candidates)
   - Interface: `Server` and `Socket` with implementations

3. **Client (`client.go`)** - WebSocket client implementation
   - Connects to WebSocket server
   - Handles registration process
   - Implements `HubClient` interface

4. **Handshake (`handshake.go`)** - WebRTC peer connection management
   - Wraps Pion WebRTC PeerConnection
   - Manages ICE candidate collection and signaling
   - Handles offer/answer workflow

### Message Types

- `register` - Client registration
- `unregister` - Client disconnection
- `sdp` - Session Description Protocol for WebRTC
- `candidate` - ICE candidate exchange

### Key Dependencies

- `github.com/gorilla/websocket` - WebSocket implementation
- `github.com/pion/webrtc/v4` - WebRTC stack
- `github.com/go-chi/chi/v5` - HTTP router
- `github.com/rs/xid` - ID generation

## Project Structure

```
/
├── cmd/
│   ├── client/main.go    # WebSocket client application
│   ├── server/main.go    # WebSocket server (signal server)
│   └── signal/main.go    # Alternative client implementation
├── client.go             # Client implementation
├── hub.go               # Message hub and routing
├── websocket.go         # WebSocket server and socket handling
├── handshake.go         # WebRTC handshake management
└── Taskfile.yml         # Task runner configuration
```

## Development Notes

- Server runs on port 3000 by default (`localhost:3000`)
- WebSocket endpoint: `/ws`
- Client connects to `ws://localhost:3000/ws`
- Uses unique IDs for client identification (generated via `xid.New()`)
- All components implement clean interfaces for testability
- Concurrent message handling via Go channels and goroutines
