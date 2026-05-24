package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sensor-backend/internal/schema"
)

// MetricsHandler queries the DB for mandatory + dynamic columns with optional device and time range filters.
// Query params: device (serial_number), start and end (ISO8601, both required if either is set).
func MetricsHandler(db *sql.DB, s *schema.Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		device := strings.TrimSpace(q.Get("device"))
		startStr := strings.TrimSpace(q.Get("start"))
		endStr := strings.TrimSpace(q.Get("end"))

		var startTime, endTime time.Time
		hasTimeRange := false
		if startStr != "" || endStr != "" {
			if startStr == "" || endStr == "" {
				http.Error(w, "both start and end are required when filtering by time", http.StatusBadRequest)
				return
			}
			var err error
			startTime, err = parseISO8601(startStr)
			if err != nil {
				http.Error(w, "invalid start: must be ISO8601", http.StatusBadRequest)
				return
			}
			endTime, err = parseISO8601(endStr)
			if err != nil {
				http.Error(w, "invalid end: must be ISO8601", http.StatusBadRequest)
				return
			}
			hasTimeRange = true
		}

		query, args, err := buildMetricsSelect(s, device, hasTimeRange, startTime, endTime)
		if err != nil {
			log.Printf("metrics build query: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("metrics query error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		cols, _ := rows.Columns()
		var result []map[string]any
		for rows.Next() {
			dest := make([]any, len(cols))
			destPtrs := make([]any, len(cols))
			for i := range dest {
				destPtrs[i] = &dest[i]
			}
			if err := rows.Scan(destPtrs...); err != nil {
				log.Printf("metrics scan error: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			row := make(map[string]any)
			for i, c := range cols {
				v := dest[i]
				if b, ok := v.([]byte); ok {
					v = string(b)
				}
				row[c] = v
			}
			result = append(result, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("metrics rows error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Printf("metrics encode error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

func parseISO8601(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z07:00",
	}
	var last error
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t, nil
		}
		last = err
	}
	return time.Time{}, last
}

// buildMetricsSelect returns SQL and args using only $n placeholders for values.
func buildMetricsSelect(s *schema.Schema, device string, hasTimeRange bool, startTime, endTime time.Time) (string, []any, error) {
	var names []string
	for _, c := range s.MandatoryColumns {
		names = append(names, quoteIdent(c.Name))
	}
	for _, c := range s.DynamicColumns {
		names = append(names, quoteIdent(c.Name))
	}

	var sb strings.Builder
	sb.WriteString("SELECT ")
	for i, n := range names {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(n)
	}
	sb.WriteString(" FROM ")
	sb.WriteString(quoteIdent(s.TableName))

	var args []any
	argN := 1
	var conds []string

	if device != "" {
		serialCol, err := mandatoryColumnName(s, "serial_number")
		if err != nil {
			return "", nil, err
		}
		conds = append(conds, quoteIdent(serialCol)+" = $"+strconv.Itoa(argN))
		args = append(args, device)
		argN++
	}

	if hasTimeRange {
		tsCol, err := mandatoryColumnName(s, "timestamp")
		if err != nil {
			return "", nil, err
		}
		conds = append(conds, quoteIdent(tsCol)+" >= $"+strconv.Itoa(argN))
		args = append(args, startTime)
		argN++
		conds = append(conds, quoteIdent(tsCol)+" <= $"+strconv.Itoa(argN))
		args = append(args, endTime)
		argN++
	}

	if len(conds) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conds, " AND "))
	}

	tsOrder, err := mandatoryColumnName(s, "timestamp")
	if err != nil {
		return "", nil, err
	}
	sb.WriteString(" ORDER BY ")
	sb.WriteString(quoteIdent(tsOrder))
	sb.WriteString(" ASC")

	return sb.String(), args, nil
}

func mandatoryColumnName(s *schema.Schema, want string) (string, error) {
	for _, c := range s.MandatoryColumns {
		if c.Name == want {
			return c.Name, nil
		}
	}
	return "", errUnknownMandatory(want)
}

type errUnknownMandatory string

func (e errUnknownMandatory) Error() string {
	return "schema missing mandatory column: " + string(e)
}

