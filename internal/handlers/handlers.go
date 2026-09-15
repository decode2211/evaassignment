// Handlers bundles the dependencies shared by all HTTP handlers.
package handlers

import (
	"time"

	"github.com/decode2211/evaassignment/internal/store"
)

// rfc3339 is the timestamp format used in every JSON response.
const rfc3339 = time.RFC3339

// Handlers holds the shared dependencies used by request handlers.
type Handlers struct {
	Store     store.Store
	JWTSecret string
}

// New creates a Handlers wired up with the given store and JWT secret.
func New(s store.Store, jwtSecret string) *Handlers {
	return &Handlers{Store: s, JWTSecret: jwtSecret}
}
