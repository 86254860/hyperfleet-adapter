package health

import (
	"context"
	"net/http"
	"time"

	"github.com/openshift-hyperfleet/hyperfleet-adapter/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsServer provides HTTP metrics endpoint for Prometheus.
type MetricsServer struct {
	server *http.Server
	log    logger.Logger
	port   string
}

// NewMetricsServer creates a new metrics server.
func NewMetricsServer(log logger.Logger, port string) *MetricsServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &MetricsServer{
		log:  log,
		port: port,
		server: &http.Server{
			Addr:              ":" + port,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

// Start starts the metrics server in a goroutine.
func (s *MetricsServer) Start(ctx context.Context) error {
	s.log.Infof(ctx, "Starting metrics server on port %s", s.port)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCtx := logger.WithErrorField(ctx, err)
			s.log.Errorf(errCtx, "Metrics server error")
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the metrics server.
func (s *MetricsServer) Shutdown(ctx context.Context) error {
	s.log.Info(ctx, "Shutting down metrics server...")
	return s.server.Shutdown(ctx)
}
