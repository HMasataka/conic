package conic

import (
	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
	"github.com/HMasataka/conic/registry"
	"github.com/HMasataka/conic/router"
)

func NewRouter(pc *PeerConnection, logger *logging.Logger) *router.Router {
	reg := registry.NewHandlerRegistry()

	reg.Register(domain.MessageTypeRegisterResponse, NewRegisterHandler(logger))
	reg.Register(domain.MessageTypeUnregisterResponse, NewUnregisterHandler(logger))
	reg.Register(domain.MessageTypeSDP, NewSessionDescriptionHandler(pc, logger))
	reg.Register(domain.MessageTypeCandidate, NewCandidateHandler(pc, logger))

	return router.NewRouter(reg, logger)
}
