package barertc

import (
	"net/http"
	"sync"
)

// Server is the primary back-end server struct for BareRTC, see main.go
type Server struct {
	// HTTP router.
	mux *http.ServeMux

	// Max number of messages we'll buffer for a subscriber
	subscriberMessageBuffer int

	// Connected WebSocket subscribers.
	subscribersMu sync.RWMutex
	subscribers   map[*Subscriber]struct{}
}

// NewServer initializes the Server.
func NewServer() *Server {
	return &Server{
		subscriberMessageBuffer: 16,
		subscribers:             make(map[*Subscriber]struct{}),
	}
}

// Setup the server: configure HTTP routes, etc.
func (s *Server) Setup() error {
	var mux = http.NewServeMux()

	mux.Handle("/", IndexPage())
	mux.Handle("/ws", s.WebSocket())
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	s.mux = mux

	return nil
}

// ListenAndServe starts the web server.
func (s *Server) ListenAndServe(address string) error {
	return http.ListenAndServe(address, s.mux)
}
