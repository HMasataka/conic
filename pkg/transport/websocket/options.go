package websocket

// WithRouter sets the message router for the server
func WithRouter(router MessageRouter) ServerOption {
	return func(o *ServerOptions) {
		o.Router = router
	}
}