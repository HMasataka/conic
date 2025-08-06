package signal

import (
	"github.com/HMasataka/conic/domain"
	"github.com/HMasataka/conic/internal/protocol"
	"github.com/HMasataka/conic/logging"
)

func NewRouter(hub domain.Hub, logger *logging.Logger) *protocol.Router {
	router := protocol.NewRouter(logger)

	router.Register(domain.MessageTypeRegisterRequest, NewRegisterRequestHandler(hub, logger))
	router.Register(domain.MessageTypeSDP, NewSDPHandler(hub, logger))
	router.Register(domain.MessageTypeCandidate, NewICECandidateHandler(hub, logger))
	router.Register(domain.MessageTypeDataChannel, NewDataChannelHandler(hub, logger))

	return router
}
