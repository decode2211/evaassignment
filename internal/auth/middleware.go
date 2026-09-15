// Auth middleware: checks the bearer token on protected routes.
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/decode2211/evaassignment/internal/httpx"
	"github.com/decode2211/evaassignment/internal/store"
)

type contextKey string

const userIDContextKey contextKey = "userID"

// Middleware checks bearer tokens against a signing secret and a user store.
type Middleware struct {
	jwtSecret string
	users     store.Store
}

// NewMiddleware creates an auth Middleware.
func NewMiddleware(jwtSecret string, users store.Store) *Middleware {
	return &Middleware{jwtSecret: jwtSecret, users: users}
}

// RequireAuth rejects requests without a valid "Authorization: Bearer <token>"
// header. Every failure case (missing header, bad signature, expired,
// user no longer exists) gets the same 401 message, so nothing leaks
// about which part of a forged token was wrong.
func (m *Middleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			httpx.WriteError(w, http.StatusUnauthorized, "missing or invalid authorization header")
			return
		}
		tokenString := strings.TrimSpace(strings.TrimPrefix(header, prefix))

		userID, _, err := VerifyToken(m.jwtSecret, tokenString)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// Re-check the user still exists, in case the account was removed
		// after the token was issued.
		if _, err := m.users.GetUserByID(r.Context(), userID); err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next(w, r.WithContext(ctx))
	}
}

// UserIDFromContext returns the logged-in user's ID set by RequireAuth.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDContextKey).(int64)
	return id, ok
}
