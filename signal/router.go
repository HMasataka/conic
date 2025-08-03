package signal

import (
	"context"

	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
	"github.com/HMasataka/conic/registry"
)

// Router implements domain.Router for signaling messages
type Router struct {
	reg    *registry.DefaultHandlerRegistry
	logger *logging.Logger
}

// NewRouter creates a new signaling router
func NewRouter(hub domain.Hub, logger *logging.Logger) *Router {
	reg := registry.NewHandlerRegistry()

	// Register handlers
	reg.Register(domain.MessageTypeRegister, NewRegisterHandler(hub, logger))
	reg.Register(domain.MessageTypeSDP, NewSDPHandler(hub, logger))
	reg.Register(domain.MessageTypeCandidate, NewICECandidateHandler(hub, logger))
	reg.Register(domain.MessageTypeDataChannel, NewDataChannelHandler(hub, logger))

	return &Router{
		reg:    reg,
		logger: logger,
	}
}

func (r *Router) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	return r.reg.Handle(ctx, msg)
}
