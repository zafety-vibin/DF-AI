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

// saveHandler implements POST /ai/save endpoint
// Triggers manual save of modifications to disk
func (s *Server) saveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "Method not allowed, use POST",
		})
		return
	}

	// Get save callback
	s.mu.Lock()
	saveFn := s.saveModificationsFunc
	s.mu.Unlock()

	if saveFn == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error": "Save system not initialized",
		})
		return
	}

	// Trigger save
	if err := saveFn(); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"message": "Modifications saved to disk",
	})
}
