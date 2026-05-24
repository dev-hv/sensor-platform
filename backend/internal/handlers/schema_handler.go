package handlers

import (
	"encoding/json"
	"net/http"

	"sensor-backend/internal/schema"
)

// SchemaHandler returns the parsed schema as JSON.
func SchemaHandler(s *schema.Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(s); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
