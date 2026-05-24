package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"sensor-backend/internal/schema"
)

// IngestHandler unmarshals JSON into map[string]any, validates against schema bounds, then INSERTs.
func IngestHandler(db *sql.DB, s *schema.Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		if !validateAndInsert(db, s, payload) {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func validateAndInsert(db *sql.DB, s *schema.Schema, payload map[string]any) bool {
	// Mandatory fields
	var timestamp time.Time
	switch v := payload["timestamp"]; vv := v.(type) {
	case string:
		var err error
		timestamp, err = time.Parse(time.RFC3339, vv)
		if err != nil {
			log.Printf("ingest: invalid timestamp %q: %v", v, err)
			return false
		}
	default:
		log.Printf("ingest: missing or invalid timestamp")
		return false
	}
	serial, _ := payload["serial_number"].(string)
	if serial == "" {
		log.Printf("ingest: missing or invalid serial_number")
		return false
	}

	// Dynamic columns: validate bounds and collect for INSERT
	colNames := []string{"timestamp", "serial_number"}
	colPlaceholders := []string{"$1", "$2"}
	args := []any{timestamp, serial}
	argIdx := 3

	for _, dc := range s.DynamicColumns {
		v, ok := payload[dc.Name]
		if !ok {
			continue
		}
		f, err := toFloat64(v)
		if err != nil {
			log.Printf("ingest: invalid %s: %v", dc.Name, err)
			return false
		}
		if dc.MinValue != nil && f < *dc.MinValue {
			log.Printf("ingest: %s %v below min %v", dc.Name, f, *dc.MinValue)
			return false
		}
		if dc.MaxValue != nil && f > *dc.MaxValue {
			log.Printf("ingest: %s %v above max %v", dc.Name, f, *dc.MaxValue)
			return false
		}
		colNames = append(colNames, dc.Name)
		colPlaceholders = append(colPlaceholders, fmt.Sprintf("$%d", argIdx))
		args = append(args, f)
		argIdx++
	}

	// Build INSERT (only columns we have values for)
	query := "INSERT INTO " + quoteIdent(s.TableName) + " (" + strings.Join(quoteIdents(colNames), ", ") + ") VALUES (" + strings.Join(colPlaceholders, ", ") + ")"
	_, err := db.Exec(query, args...)
	if err != nil {
		log.Printf("ingest: insert error: %v", err)
		return false
	}
	return true
}

func toFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	default:
		return 0, &typeErr{v}
	}
}

type typeErr struct{ v any }

func (e *typeErr) Error() string { return "not a number" }

func quoteIdents(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = quoteIdent(n)
	}
	return out
}
