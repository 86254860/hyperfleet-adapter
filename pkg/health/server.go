package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/openshift-hyperfleet/hyperfleet-adapter/pkg/logger"
)

// Response represents the JSON response for health endpoints.
type Response struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Server provides HTTP health check endpoints.
type Server struct {
	server    *http.Server
	ready     atomic.Bool
	log       logger.Logger
	port      string
	component string
}

// NewServer creates a new health check server.
func NewServer(log logger.Logger, port string, component string) *Server {
	s := &Server{
		log:       log,
		port:      port,
		component: component,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthzHandler)
	mux.HandleFunc("/readyz", s.readyzHandler)

	s.server = &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

// Start starts the health server in a goroutine.
func (s *Server) Start(ctx context.Context) error {
	s.log.Infof(ctx, "Starting health server on port %s", s.port)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCtx := logger.WithErrorField(ctx, err)
			s.log.Errorf(errCtx, "Health server error")
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the health server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info(ctx, "Shutting down health server...")
	return s.server.Shutdown(ctx)
}

// SetReady marks the server as ready to accept traffic.
func (s *Server) SetReady(ready bool) {
	s.ready.Store(ready)
}

// IsReady returns the current readiness state.
func (s *Server) IsReady() bool {
	return s.ready.Load()
}

// healthzHandler handles liveness probe requests.
// Returns 200 OK if the process is alive.
func (s *Server) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{Status: "ok"})
}

// readyzHandler handles readiness probe requests.
// Returns 200 OK if the server is ready to accept traffic,
// 503 Service Unavailable otherwise.
func (s *Server) readyzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.ready.Load() {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Response{Status: "ok"})
		return
	}

	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(Response{
		Status:  "error",
		Message: "not ready",
	})
}
