package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey struct{}

// WithUserID records a verified identity. Only the middleware should call this
// on a real request; handlers and tests read it back through UserID.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKey{}, strings.TrimSpace(userID))
}

// UserID returns the verified caller, or "" when the request was not
// authenticated. It deliberately does not fall back to a request header: that
// fallback was the vulnerability.
func UserID(r *http.Request) string {
	if id, ok := r.Context().Value(contextKey{}).(string); ok {
		return id
	}
	return ""
}

// Middleware verifies the bearer token and attaches its subject.
//
// WebSocket clients cannot set headers, so a token may also arrive as
// ?access_token=. That puts it in access logs, which is why these tokens are
// short lived and carry nothing but a subject and an expiry.
func Middleware(signer *Signer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := signer.Verify(bearerToken(r))
			if err != nil {
				writeUnauthorized(w)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), claims.Subject)))
		})
	}
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(header) > 7 && strings.EqualFold(header[:7], "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return strings.TrimSpace(r.URL.Query().Get("access_token"))
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
}
