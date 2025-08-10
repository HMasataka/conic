# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Conic is a Go-based WebRTC signaling server that facilitates real-time peer-to-peer communications. It provides WebSocket-based signaling for WebRTC connections with data channel support, automatic handshake processing, and real-time communication capabilities.

## Development Commands

### Task Runner Commands (using Taskfile)

- **Run signal server**: `task signal` or `go run cmd/signal/main.go`
- **Run P2P demo**: `task p2p` or `go run cmd/p2p/main.go`
- **Run P2P as offer side**: `task p2p-offer` or `go run cmd/p2p/main.go -role=offer`
- **Run P2P as answer side**: `task p2p-answer` or `go run cmd/p2p/main.go -role=answer`
- **Run audio demo (Opus)**: `task audio` or `go run cmd/audio/main.go`
- **Run video demo (VP8)**: `task video` or `go run cmd/video/main.go`
- **Run video as offer side**: `task video-offer` or `go run cmd/video/main.go -role=offer`
- **Run video as answer side**: `task video-answer` or `go run cmd/video/main.go -role=answer`
- **View available tasks**: `task --list` or `task`
- **Build all**: `task build`
- **Run tests**: `task test`
- **Format code**: `task fmt`
- **Vet code**: `task vet`
- **Tidy dependencies**: `task tidy`
- **Clean build artifacts**: `task clean`
- **Install development tools**: `task install-tools` (installs air)
- **Dev server with hot reload**: `task dev-server` (requires air)

Note: cmd/p2p/main.go is missing according to git status. Tests don't currently exist in the codebase.

## Architecture Overview

The project follows a domain-driven design with clean interfaces and modular packages.

### Core Components

1. **Domain Layer** (`domain/`) - Core interfaces and types
   - `Hub` interface - Message routing contract
   - `Client` interface - Client connection contract
   - Message types with JSON marshaling
   - Hub statistics tracking

2. **Hub Implementation** (`hub/hub.go`) - Central message routing
   - Manages client registration/unregistration
   - Routes messages between connected clients
   - Tracks client connections and statistics
   - Uses channels for concurrent operation

3. **Signal Package** (`signal/`) - WebSocket server implementation
   - `server.go` - WebSocket server with connection upgrading
   - `client.go` - Server-side client representation
   - `handler.go` - Message type handlers (register, unregister, SDP, candidate, data_channel)
   - `router.go` - HTTP routing setup

4. **Registry Package** (`registry/`) - WebRTC registration handling
   - Manages WebRTC offer/answer registration flow

5. **WebRTC Wrappers**
   - `peer.go` - PeerConnection wrapper with statistics, ICE candidate queuing, and error handling
   - `datachannel.go` - DataChannel wrapper with statistics, event handlers, and thread-safe operations
   - `audiotrack.go` - AudioTrack wrapper for Opus codec support and audio streaming
   - `videotrack.go` - VideoTrack wrapper for VP8 codec support and video streaming

6. **P2P Implementation** (`p2p.go`) - High-level P2P connection management
   - Handles complete WebRTC handshake workflow
   - Manages data channels and peer connections

7. **Logging Package** (`logging/`) - Structured logging
   - Uses Go's `slog` package
   - Context-aware logging
   - Configurable levels and formats (JSON/text)

8. **Error Handling** (`errors.go`) - Custom error types and definitions

### Message Types

- `register` - Client registration with hub
- `unregister` - Client disconnection
- `sdp` - Session Description Protocol for WebRTC offers/answers
- `candidate` - ICE candidate exchange for NAT traversal
- `data_channel` - Data channel messages between peers

### Key Dependencies

- `github.com/gorilla/websocket` - WebSocket implementation
- `github.com/pion/webrtc/v4` - WebRTC stack
- `github.com/go-chi/chi/v5` - HTTP router
- `github.com/rs/xid` - Unique ID generation
- Go 1.24.5 or later

## Project Structure

```
/
├── cmd/
│   ├── signal/main.go    # Signal server application
│   └── p2p/              # P2P demo client (missing per git status)
├── domain/               # Core interfaces and types
│   ├── client.go         # Client interface
│   ├── data.go          # Message types
│   └── hub.go           # Hub interface
├── hub/                 # Hub implementation
│   └── hub.go
├── signal/              # WebSocket server implementation
│   ├── server.go        # WebSocket server
│   ├── client.go        # Server-side client
│   ├── handler.go       # Message handlers
│   └── router.go        # HTTP routing
├── registry/            # WebRTC registration
│   └── handler.go
├── logging/             # Structured logging
│   ├── logger.go
│   └── context.go
├── peer.go             # PeerConnection wrapper
├── datachannel.go      # DataChannel wrapper
├── p2p.go             # P2P connection management
├── errors.go          # Error definitions
└── docs/              # Documentation
    ├── webrtc-terminology.md
    └── ice-explanation.md
```

## Development Workflow

### P2P Communication Demo

1. Start signal server: `task signal`
2. In separate terminals:
   - Terminal 1: `task p2p-offer`
   - Terminal 2: `task p2p-answer`
3. Enter peer ID when prompted to establish connection
4. Use interactive commands:
   - `offer <peer_id>` - Create WebRTC offer
   - `channel <label>` - Create data channel
   - `send <label> <message>` - Send message
   - `list` - List active channels
   - `quit` - Exit

## Development Notes

- Server runs on port 3000 by default (`localhost:3000`)
- WebSocket endpoint: `/ws`
- Clients connect to `ws://localhost:3000/ws`
- Uses unique IDs via `xid.New()` for client identification
- Concurrent message handling via Go channels and goroutines
- Comprehensive Japanese documentation available in README.md
- WebRTC terminology and ICE protocol explanations in docs/

## Markdown Formatting Guidelines

When editing or creating Markdown files in this repository, follow these markdownlint-compliant guidelines:

- **Use consistent heading structure** - Start with H1 (`#`) and increment hierarchically
- **Add blank lines around headings** - Always have empty lines before and after headings
- **Consistent list formatting** - Use `-` for unordered lists with proper spacing
- **Line length** - Keep lines under 80 characters when possible (MD013)
- **URL formatting** - Use `<https://example.com>` format for bare URLs or proper link syntax `[text](url)`
- **Code blocks** - Always specify language for syntax highlighting
- **Trailing punctuation** - Be consistent with punctuation in headings and lists
- **Empty lines** - Use single empty line between sections
- **No trailing spaces** - Remove trailing whitespace from lines
- **File endings** - End files with a single newline

Run `markdownlint README.md` before committing to ensure compliance with these standards.
