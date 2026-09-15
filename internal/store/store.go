// Package store defines the persistence interface; sqlite.go implements it.
package store

import (
	"context"
	"errors"

	"github.com/decode2211/evaassignment/internal/models"
)

// ErrNotFound means a lookup by ID (or ID+owner) found no row.
var ErrNotFound = errors.New("not found")

// ErrDuplicateEmail means the email is already registered.
var ErrDuplicateEmail = errors.New("email already registered")

// Store is what handlers use for persistence, independent of the DB engine.
type Store interface {
	CreateUser(ctx context.Context, email, passwordHash, name string) (models.User, error)
	GetUserByEmail(ctx context.Context, email string) (models.User, error)
	GetUserByID(ctx context.Context, id int64) (models.User, error)

	CreateTicket(ctx context.Context, userID int64, title, description string) (models.Ticket, error)
	ListTicketsByUser(ctx context.Context, userID int64) ([]models.Ticket, error)

	// GetTicketByIDForUser returns ErrNotFound both when the ticket doesn't
	// exist and when it belongs to someone else, so a caller can't tell
	// which - see handlers/tickets.go for why that matters.
	GetTicketByIDForUser(ctx context.Context, id, userID int64) (models.Ticket, error)

	// UpdateTicketStatus checks ownership and applies the status change
	// atomically so two concurrent requests can't race each other.
	UpdateTicketStatus(ctx context.Context, id, userID int64, newStatus models.Status) (models.Ticket, error)
}
