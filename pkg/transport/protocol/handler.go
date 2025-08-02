package protocol

import (
	"context"

	"github.com/HMasataka/conic/pkg/domain"
)

// Handler defines the interface for handling protocol messages
type Handler interface {
	// Handle processes a message and returns a response
	Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error)

	// CanHandle checks if the handler can handle a specific message type
	CanHandle(messageType domain.MessageType) bool
}

// HandlerFunc is a function adapter for Handler
type HandlerFunc func(ctx context.Context, msg *domain.Message) (*domain.Message, error)

// HandlerRegistry manages message handlers
type HandlerRegistry interface {
	// Register registers a handler for a message type
	Register(messageType domain.MessageType, handler Handler)

	// Get retrieves a handler for a message type
	Get(messageType domain.MessageType) (Handler, bool)

	// Handle routes a message to the appropriate handler
	Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error)
}

// DefaultHandlerRegistry is the default implementation of HandlerRegistry
type DefaultHandlerRegistry struct {
	handlers map[domain.MessageType]Handler
}

// NewHandlerRegistry creates a new handler registry
func NewHandlerRegistry() *DefaultHandlerRegistry {
	return &DefaultHandlerRegistry{
		handlers: make(map[domain.MessageType]Handler),
	}
}

// Register implements HandlerRegistry
func (r *DefaultHandlerRegistry) Register(messageType domain.MessageType, handler Handler) {
	r.handlers[messageType] = handler
}

// Get implements HandlerRegistry
func (r *DefaultHandlerRegistry) Get(messageType domain.MessageType) (Handler, bool) {
	handler, ok := r.handlers[messageType]
	return handler, ok
}

// Handle implements HandlerRegistry
func (r *DefaultHandlerRegistry) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	handler, ok := r.Get(msg.Type)
	if !ok {
		return nil, domain.NewDomainError(
			domain.ErrCodeNotFound,
			"no handler found for message type",
			nil,
		)
	}

	return handler.Handle(ctx, msg)
}
