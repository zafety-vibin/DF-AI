package http

import (
	"net/http"
)

// aiHistoryHandler implements GET /ai/history endpoint
// Returns recent AI decision history with context, responses, commands, and outcomes
func (s *Server) aiHistoryHandler(w http.ResponseWriter, r *http.Request) {
	// Get AI history callback
	s.mu.Lock()
	getHistory := s.getAIHistoryFunc
	s.mu.Unlock()

	if getHistory == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error": "AI system not initialized",
		})
		return
	}

	// Call callback to get history
	history := getHistory()

	if history == nil {
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"turns":        []interface{}{},
			"total_turns":  0,
			"message":      "No AI decisions yet",
		})
		return
	}

	// Return history as JSON
	s.writeJSON(w, http.StatusOK, history)
}
