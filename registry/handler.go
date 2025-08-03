package registry

import (
	"context"
	"errors"

	"github.com/HMasataka/conic/domain"
)

type Handler interface {
	Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error)
	CanHandle(messageType domain.MessageType) bool
}

type HandlerFunc func(ctx context.Context, msg *domain.Message) (*domain.Message, error)

type HandlerRegistry interface {
	Register(messageType domain.MessageType, handler Handler)

	Get(messageType domain.MessageType) (Handler, bool)

	Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error)
}

type DefaultHandlerRegistry struct {
	handlers map[domain.MessageType]Handler
}

func NewHandlerRegistry() *DefaultHandlerRegistry {
	return &DefaultHandlerRegistry{
		handlers: make(map[domain.MessageType]Handler),
	}
}

func (r *DefaultHandlerRegistry) Register(messageType domain.MessageType, handler Handler) {
	r.handlers[messageType] = handler
}

func (r *DefaultHandlerRegistry) Get(messageType domain.MessageType) (Handler, bool) {
	handler, ok := r.handlers[messageType]
	return handler, ok
}

func (r *DefaultHandlerRegistry) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	handler, ok := r.Get(msg.Type)
	if !ok {
		return nil, errors.New("")
	}

	return handler.Handle(ctx, msg)
}
