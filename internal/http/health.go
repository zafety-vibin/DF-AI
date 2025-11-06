package http

import (
	"net/http"
	"time"
)

// HealthStatus represents current operational state
type HealthStatus struct {
	Status                string     `json:"status"`
	UptimeSeconds         int64      `json:"uptime_seconds"`
	DFHackConnected       bool       `json:"dfhack_connected"`
	DFHackConnectionTime  *time.Time `json:"dfhack_connection_time,omitempty"`
	LastHeartbeat         *time.Time `json:"last_heartbeat,omitempty"`
	Timestamp             time.Time  `json:"timestamp"`
}

// healthHandler implements GET /health endpoint
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	uptime := now.Sub(s.startTime)

	// Get connection status from DFHack client
	connected := s.dfhackClient.IsConnected()

	// Get session metrics to check heartbeat recency
	metrics := s.dfhackClient.GetSessionMetrics()

	// Determine health status
	status := "healthy"
	var connTime *time.Time
	var lastHeartbeat *time.Time

	if connected {
		connTime = &metrics.ConnectionTime

		// Check heartbeat freshness
		if !metrics.ConnectionTime.IsZero() {
			// If heartbeats exist, check recency
			if metrics.HeartbeatsReceived > 0 {
				// Estimate last heartbeat time (we don't track it directly yet)
				// For now, consider healthy if connected
				lastHeartbeat = &metrics.ConnectionTime
			}
		}
	} else {
		status = "degraded"
	}

	// If connected but no heartbeat in 30s, status is degraded
	if connected && lastHeartbeat != nil && time.Since(*lastHeartbeat) > 30*time.Second {
		status = "degraded"
	}

	healthStatus := HealthStatus{
		Status:               status,
		UptimeSeconds:        int64(uptime.Seconds()),
		DFHackConnected:      connected,
		DFHackConnectionTime: connTime,
		LastHeartbeat:        lastHeartbeat,
		Timestamp:            now,
	}

	// Always return 200 if server is running (even if degraded)
	s.writeJSON(w, http.StatusOK, healthStatus)
}
