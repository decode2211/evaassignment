// Package store is the only part of the program that talks to the
// database. Everything else (handlers, auth) works through the small
// Store interface defined here, never with raw SQL directly. That makes it
// possible to swap the database technology later, or to write tests with a
// fake in-memory store, without touching any other package.
package store

import (
	"context"
	"errors"

	"github.com/decode2211/evaassignment/internal/models"
)

// ErrNotFound is returned whenever a lookup (by ID, or by ID+owner) finds no
// matching row. Handlers translate this into an HTTP 404 response.
var ErrNotFound = errors.New("not found")

// ErrDuplicateEmail is returned by CreateUser when the email address is
// already registered. Handlers translate this into an HTTP 409 Conflict.
var ErrDuplicateEmail = errors.New("email already registered")

// Store is the interface every handler depends on for persistence. It
// describes *what* the application needs to store and retrieve, without
// saying *how* - the SQLite implementation lives in sqlite.go, but a test
// could provide its own in-memory implementation of this same interface.
type Store interface {
	// CreateUser saves a new user with an already-hashed password and
	// returns the created record (including its generated ID and
	// creation time). Returns ErrDuplicateEmail if the email is taken.
	CreateUser(ctx context.Context, email, passwordHash, name string) (models.User, error)

	// GetUserByEmail looks up a user for login. Returns ErrNotFound if no
	// user has that email.
	GetUserByEmail(ctx context.Context, email string) (models.User, error)

	// GetUserByID looks up a user by primary key, used by the auth
	// middleware to confirm the user in a JWT still exists. Returns
	// ErrNotFound if there is no such user (e.g. deleted after the token
	// was issued).
	GetUserByID(ctx context.Context, id int64) (models.User, error)

	// CreateTicket saves a brand-new ticket owned by userID. The ticket
	// always starts with status "open" - callers must not pass any other
	// starting status.
	CreateTicket(ctx context.Context, userID int64, title, description string) (models.Ticket, error)

	// ListTicketsByUser returns every ticket owned by userID, newest first.
	// Ownership filtering happens in the SQL query itself (not just in Go),
	// so it is impossible to accidentally return another user's tickets.
	ListTicketsByUser(ctx context.Context, userID int64) ([]models.Ticket, error)

	// GetTicketByIDForUser returns the ticket with the given ID, but only
	// if it is owned by userID. Returns ErrNotFound both when the ticket
	// does not exist at all AND when it belongs to someone else - the
	// caller cannot tell those two cases apart, which is intentional (see
	// the handler comments for why).
	GetTicketByIDForUser(ctx context.Context, id, userID int64) (models.Ticket, error)

	// UpdateTicketStatus changes a ticket's status, but only if it is
	// owned by userID. Returns ErrNotFound if no such owned ticket exists.
	// The implementation must perform the ownership+existence check and
	// the write atomically (e.g. inside one SQL statement or one
	// transaction) so two simultaneous requests can't race each other.
	UpdateTicketStatus(ctx context.Context, id, userID int64, newStatus models.Status) (models.Ticket, error)
}
