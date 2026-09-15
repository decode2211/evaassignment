// Package models holds the core data types: User, Ticket, and status rules.
package models

import (
	"errors"
	"time"
)

// User is one registered account.
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Status is a ticket's lifecycle stage.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

// IsValid reports whether s is one of the known statuses.
func (s Status) IsValid() bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusClosed:
		return true
	default:
		return false
	}
}

// Ticket is one issue raised by a user.
type Ticket struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	UserID      int64     `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ErrInvalidTransition means the requested status change isn't allowed.
var ErrInvalidTransition = errors.New("invalid status transition")

// CanTransition checks if a ticket can move from one status to another.
// Closed tickets can't reopen - once resolved, a ticket stays a fixed record.
func CanTransition(from, to Status) error {
	if !from.IsValid() || !to.IsValid() {
		return ErrInvalidTransition
	}
	if from == to {
		return errors.New("ticket already has status " + string(to))
	}

	allowed := map[Status]map[Status]bool{
		StatusOpen: {
			StatusInProgress: true,
			StatusClosed:     true,
		},
		StatusInProgress: {
			StatusClosed: true,
		},
		StatusClosed: {},
	}

	if allowed[from][to] {
		return nil
	}
	return ErrInvalidTransition
}
