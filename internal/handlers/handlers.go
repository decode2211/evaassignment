// This file defines the Handlers type, which bundles together everything
// the HTTP handlers need (a database store and the JWT signing secret) so
// individual handler functions can be methods on it instead of relying on
// global variables.
package handlers

import (
	"time"

	"github.com/decode2211/evaassignment/internal/store"
)

// rfc3339 is the timestamp format used in every JSON response, as required
// by the API contract ("Timestamps in RFC3339 UTC").
const rfc3339 = time.RFC3339

// Handlers holds the shared dependencies used by every request handler in
// this package.
type Handlers struct {
	Store     store.Store
	JWTSecret string
}

// New creates a Handlers value wired up with the given store and JWT
// secret.
func New(s store.Store, jwtSecret string) *Handlers {
	return &Handlers{Store: s, JWTSecret: jwtSecret}
}
