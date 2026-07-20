package http

import (
	"net/http"
)

// ReadinessStatus indicates whether server is ready to accept requests
type ReadinessStatus struct {
	Ready   bool            `json:"ready"`
	Checks  map[string]bool `json:"checks"`
	Message string          `json:"message,omitempty"`
}

// readyHandler implements GET /ready endpoint
func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]bool)

	// Check if config loaded
	checks["config_loaded"] = s.configMgr != nil && s.configMgr.Get() != nil

	// Check if TCP listener started (dfhack client listener should be non-nil)
	// We can infer this by checking if client was initialized
	checks["tcp_listener"] = s.dfhackClient != nil

	// Check if HTTP server started (self-check - if we're responding, it's started)
	checks["http_server"] = true

	// Check if logger initialized
	checks["logger"] = s.logger != nil

	// Determine overall readiness
	ready := true
	var message string

	for name, check := range checks {
		if !check {
			ready = false
			if message == "" {
				message = "Not ready: " + name + " check failed"
			}
		}
	}

	readiness := ReadinessStatus{
		Ready:   ready,
		Checks:  checks,
		Message: message,
	}

	// Return 200 if ready, 503 if not ready
	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	s.writeJSON(w, statusCode, readiness)
}
