// This file is the "bouncer at the door" for protected routes. It checks
// that every request to a protected endpoint carries a valid login token,
// and makes the identity of the logged-in user available to the handler
// that runs afterwards.
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/decode2211/evaassignment/internal/httpx"
	"github.com/decode2211/evaassignment/internal/store"
)

// contextKey is a private type for context keys defined in this package.
// Using a custom type (instead of a plain string) avoids collisions with
// context keys set by other packages.
type contextKey string

const userIDContextKey contextKey = "userID"

// Middleware builds the authentication middleware. It needs access to the
// store so it can confirm, on every request, that the user named in the
// token still actually exists (for example, in case a user could be
// deleted in the future - this check makes that safe by construction).
type Middleware struct {
	jwtSecret string
	users     store.Store
}

// NewMiddleware creates an auth Middleware using the given signing secret
// and user store.
func NewMiddleware(jwtSecret string, users store.Store) *Middleware {
	return &Middleware{jwtSecret: jwtSecret, users: users}
}

// RequireAuth wraps a handler so that it only runs for requests carrying a
// valid "Authorization: Bearer <token>" header. Otherwise, it responds with
// 401 Unauthorized and never calls the wrapped handler at all.
//
// All of these situations are treated the same way (401, generic message):
// the header is missing, it isn't in the "Bearer <token>" shape, the token
// signature doesn't check out, the token has expired, or the user it names
// no longer exists. Giving the same response for every failure avoids
// leaking hints about which part of a forged token was wrong.
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

		// Re-check the user still exists. A token issued yesterday should
		// stop working the moment the account behind it is gone, even
		// though the token itself has not "expired" yet.
		if _, err := m.users.GetUserByID(r.Context(), userID); err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next(w, r.WithContext(ctx))
	}
}

// UserIDFromContext retrieves the logged-in user's ID that RequireAuth
// stored on the request context. Handlers call this to know whose data to
// read or write. The bool return is false if called on a request that
// never went through RequireAuth (which should never happen in practice,
// since every /tickets route is wrapped with it).
func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDContextKey).(int64)
	return id, ok
}
