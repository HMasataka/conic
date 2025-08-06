package signal

import (
	"net/http"

	"github.com/HMasataka/conic"
	"github.com/HMasataka/conic/logging"
	"github.com/HMasataka/conic/internal/protocol"
	"github.com/HMasataka/conic/internal/transport"
	ws "github.com/gorilla/websocket"
)

type ServerOptions struct {
	transport.ConnectionOptions
}

func DefaultServerOptions() ServerOptions {
	return ServerOptions{
		ConnectionOptions: transport.DefaultConnectionOptions(),
	}
}

type Server struct {
	upgrader ws.Upgrader
	router   *protocol.Router
	logger   *logging.Logger
	options  ServerOptions
}

func NewServer(router *protocol.Router, logger *logging.Logger, options ServerOptions) *Server {
	upgrader := ws.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		ReadBufferSize:  int(options.ReadBufferSize),
		WriteBufferSize: int(options.WriteBufferSize),
	}

	return &Server{
		upgrader: upgrader,
		router:   router,
		logger:   logger,
		options:  options,
	}
}

func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("failed to upgrade connection", "error", err)
		return
	}

	s.logger.Info("websocket connection established")

	ctx := conic.WithConnection(r.Context(), conn)
	connection := transport.NewConnection(conn, s.router, s.logger, s.options.ConnectionOptions)

	connection.Start(ctx)
}
