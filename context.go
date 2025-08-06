package conic

import (
	"context"

	"github.com/gorilla/websocket"
)

const connectionKey = "connection"

func WithConnection(ctx context.Context, conn *websocket.Conn) context.Context {
	return context.WithValue(ctx, connectionKey, conn)
}

func ConnectionFromContext(ctx context.Context) (*websocket.Conn, bool) {
	conn, ok := ctx.Value(connectionKey).(*websocket.Conn)
	if !ok || conn == nil {
		return nil, false
	}

	return conn, true
}
