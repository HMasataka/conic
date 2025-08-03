package domain

import "context"

type MessageHandler func(message []byte) error

type Client interface {
	ID() string

	Send(ctx context.Context, message []byte) error

	SetHandler(handler MessageHandler) error

	Close() error

	Context() context.Context
}
