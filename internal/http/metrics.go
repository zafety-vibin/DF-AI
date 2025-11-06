package http

import (
	"net/http"
	"time"

	"github.com/df-ai/orchestrator/internal/dfhack"
)

// MetricsSnapshot aggregates operational metrics
type MetricsSnapshot struct {
	ServerUptimeSeconds int64                   `json:"server_uptime_seconds"`
	DFHackConnected     bool                    `json:"dfhack_connected"`
	Session             *dfhack.SessionMetrics  `json:"session,omitempty"`
	ConfigReloads       uint64                  `json:"config_reloads"`
	HttpRequests        uint64                  `json:"http_requests"`
	Timestamp           time.Time               `json:"timestamp"`
}

// metricsHandler implements GET /metrics endpoint
func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	uptime := now.Sub(s.startTime)

	// Get connection status
	connected := s.dfhackClient.IsConnected()

	// Get session metrics from DFHack client
	var sessionMetrics *dfhack.SessionMetrics
	if connected {
		metrics := s.dfhackClient.GetSessionMetrics()
		sessionMetrics = &metrics
	}

	// Get HTTP request count
	s.mu.Lock()
	httpReqs := s.httpRequests
	s.mu.Unlock()

	// Get config reload count from ConfigManager
	configReloads := s.configMgr.GetConfigReloads()

	snapshot := MetricsSnapshot{
		ServerUptimeSeconds: int64(uptime.Seconds()),
		DFHackConnected:     connected,
		Session:             sessionMetrics,
		ConfigReloads:       configReloads,
		HttpRequests:        httpReqs,
		Timestamp:           now,
	}

	s.writeJSON(w, http.StatusOK, snapshot)
}
