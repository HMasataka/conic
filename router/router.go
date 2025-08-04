package router

import (
	"context"

	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
	"github.com/HMasataka/conic/registry"
)

type Router struct {
	reg    registry.HandlerRegistry
	logger *logging.Logger
}

func NewRouter(reg registry.HandlerRegistry, logger *logging.Logger) *Router {
	return &Router{
		reg:    reg,
		logger: logger,
	}
}

func (r *Router) Handle(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	return r.reg.Handle(ctx, msg)
}
