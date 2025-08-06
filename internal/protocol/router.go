package protocol

import (
	"context"

	"github.com/HMasataka/conic/domain"
	webrtcinternal "github.com/HMasataka/conic/internal/webrtc"
	"github.com/HMasataka/conic/logging"
	"github.com/HMasataka/conic/registry"
)

type Router struct {
	handlerRegistry registry.HandlerRegistry
	logger          *logging.Logger
}

func NewRouter(logger *logging.Logger) *Router {
	return &Router{
		handlerRegistry: registry.NewHandlerRegistry(),
		logger:          logger,
	}
}

func (r *Router) Register(messageType domain.MessageType, handler registry.Handler) {
	r.handlerRegistry.Register(messageType, handler)
}

func (r *Router) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	return r.handlerRegistry.Handle(ctx, msg)
}

func NewPeerRouter(pc *webrtcinternal.PeerConnection, logger *logging.Logger) *Router {
	router := NewRouter(logger)

	router.Register(domain.MessageTypeRegisterResponse, NewRegisterHandler(logger))
	router.Register(domain.MessageTypeUnregisterResponse, NewUnregisterHandler(logger))
	router.Register(domain.MessageTypeSDP, NewSessionDescriptionHandler(pc, logger))
	router.Register(domain.MessageTypeCandidate, NewCandidateHandler(pc, logger))

	return router
}
