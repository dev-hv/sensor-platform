package middleware

import (
	"net/http"
	"strings"
)

// Auth wraps a handler to require X-API-Key. readKey is used for GET, writeKey for POST.
func Auth(readKey, writeKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("X-API-Key"))
		required := readKey
		if r.Method == http.MethodPost {
			required = writeKey
		}
		if key == "" || key != required {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
