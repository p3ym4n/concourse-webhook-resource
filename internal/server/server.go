package server

import (
	"net/http"

	"github.com/p3ym4n/concourse-webhook-resource/internal/storage"
)

// Server handles incoming webhooks and exposes an internal API for the resource scripts.
type Server struct {
	mux   *http.ServeMux
	store storage.Storage
	token string // empty = no auth required
}

func New(store storage.Storage, token string) *Server {
	s := &Server{
		mux:   http.NewServeMux(),
		store: store,
		token: token,
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/webhook", s.handleWebhook)
	s.mux.HandleFunc("/api/payloads", s.handleListPayloads)
	s.mux.HandleFunc("/api/payloads/", s.handlePayload)
}
