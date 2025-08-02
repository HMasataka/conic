package signaling

import (
	"context"

	"github.com/HMasataka/conic/internal/eventbus"
	"github.com/HMasataka/conic/internal/logging"
	"github.com/HMasataka/conic/pkg/domain"
	"github.com/HMasataka/conic/pkg/transport/protocol"
)

// Router implements domain.Router for signaling messages
type Router struct {
	registry *protocol.DefaultHandlerRegistry
	logger   *logging.Logger
}

// NewRouter creates a new signaling router
func NewRouter(hub domain.Hub, logger *logging.Logger, eventBus eventbus.Bus) *Router {
	registry := protocol.NewHandlerRegistry()

	// Register handlers
	registry.Register(domain.MessageTypeRegister, NewRegisterHandler(hub, logger, eventBus))
	registry.Register(domain.MessageTypeSDP, NewSDPHandler(hub, logger, eventBus))
	registry.Register(domain.MessageTypeCandidate, NewICECandidateHandler(hub, logger, eventBus))
	registry.Register(domain.MessageTypeDataChannel, NewDataChannelHandler(hub, logger, eventBus))

	return &Router{
		registry: registry,
		logger:   logger,
	}
}

// Route implements domain.Router
func (r *Router) Route(ctx context.Context, message domain.Message) error {
	_, err := r.registry.Handle(ctx, &message)
	return err
}

// RegisterHandler implements domain.Router
func (r *Router) RegisterHandler(messageType domain.MessageType, handler domain.MessageHandlerFunc) {
	r.registry.Register(messageType, &handlerAdapter{fn: handler})
}

// UnregisterHandler implements domain.Router
func (r *Router) UnregisterHandler(messageType domain.MessageType) {
	// Not implemented in current registry
	r.logger.Warn("UnregisterHandler not implemented", "message_type", messageType)
}

// Handle implements the compatibility method for protocol.HandlerRegistry
func (r *Router) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	r.logger.Info("router handling message", "type", msg.Type)
	result, err := r.registry.Handle(ctx, msg)
	if err != nil {
		r.logger.Error("router handler error", "error", err)
	} else if result != nil {
		r.logger.Info("router returning response", "type", result.Type)
	} else {
		r.logger.Info("router returning no response")
	}
	return result, err
}

// handlerAdapter adapts domain.MessageHandlerFunc to protocol.Handler
type handlerAdapter struct {
	fn domain.MessageHandlerFunc
}

func (a *handlerAdapter) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	return nil, a.fn(ctx, *msg)
}

func (a *handlerAdapter) CanHandle(messageType domain.MessageType) bool {
	return true
}
