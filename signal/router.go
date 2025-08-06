package signal

import (
	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/logging"
	"github.com/HMasataka/conic/registry"
	"github.com/HMasataka/conic/router"
)

func NewRouter(hub domain.Hub, logger *logging.Logger) *router.Router {
	reg := registry.NewHandlerRegistry()

	reg.Register(domain.MessageTypeRegisterRequest, NewRegisterRequestHandler(hub, logger))
	reg.Register(domain.MessageTypeSDP, NewSDPHandler(hub, logger))
	reg.Register(domain.MessageTypeCandidate, NewICECandidateHandler(hub, logger))
	reg.Register(domain.MessageTypeDataChannel, NewDataChannelHandler(hub, logger))

	return router.NewRouter(reg, logger)
}
