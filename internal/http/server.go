package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/config"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/logging"
)

// Server manages the HTTP monitoring API
type Server struct {
	httpServer           *http.Server
	logger               *logging.Logger
	dfhackClient         *dfhack.Client
	configMgr            *config.ConfigManager
	getHazardMetricsFunc func() *HazardMetrics
	startTime            time.Time
	configReloads        uint64
	httpRequests         uint64
	mu                   sync.Mutex
}

// NewServer creates a new HTTP monitoring server
func NewServer(logger *logging.Logger, dfhackClient *dfhack.Client, configMgr *config.ConfigManager) *Server {
	return &Server{
		logger:       logger,
		dfhackClient: dfhackClient,
		configMgr:    configMgr,
		startTime:    time.Now(),
	}
}

// SetGetHazardMetrics sets the callback function for retrieving hazard metrics
func (s *Server) SetGetHazardMetrics(fn func() *HazardMetrics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getHazardMetricsFunc = fn
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	cfg := s.configMgr.Get()

	// Create mux and register handlers
	mux := http.NewServeMux()

	// Wrap handlers with request counting middleware
	mux.HandleFunc("/health", s.withMetrics(s.healthHandler))
	mux.HandleFunc("/ready", s.withMetrics(s.readyHandler))
	mux.HandleFunc("/metrics", s.withMetrics(s.metricsHandler))

	// Create HTTP server
	addr := fmt.Sprintf(":%d", cfg.HttpPort)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	s.logger.Info("HTTP server starting",
		logging.Field{Key: "address", Value: addr})

	// Start server in goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", err)
		}
	}()

	s.logger.Info("HTTP server started",
		logging.Field{Key: "port", Value: cfg.HttpPort})

	return nil
}

// Stop gracefully shuts down the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	s.logger.Info("HTTP server shutting down...")

	// Graceful shutdown with context timeout
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP server shutdown error: %w", err)
	}

	s.logger.Info("HTTP server stopped")
	return nil
}

// withMetrics is middleware that counts HTTP requests
func (s *Server) withMetrics(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.httpRequests++
		s.mu.Unlock()

		handler(w, r)
	}
}

// writeJSON writes JSON response with proper headers
func (s *Server) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode JSON response", err)
	}
}
