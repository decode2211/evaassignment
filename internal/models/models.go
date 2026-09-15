// Package models describes the "nouns" of this application: what a User is,
// what a Ticket is, and what states a ticket is allowed to move through.
//
// Nothing in this file talks to a database or the network. It is pure,
// plain Go data plus one important business rule (status transitions), kept
// separate from storage and HTTP code so it is easy to read, easy to test,
// and impossible to bypass by accident from elsewhere in the codebase.
package models

import (
	"errors"
	"time"
)

// User represents one registered person who can log in and own tickets.
//
// Note that there is no "Password" field with the plain-text password here.
// We only ever keep the bcrypt hash (see PasswordHash), and that field is
// tagged "-" for JSON so it can never accidentally be sent back to a client,
// even if a developer later passes a User value straight into a JSON
// response by mistake.
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Status is the lifecycle stage of a ticket. We use a named string type
// (instead of a plain string) so the compiler helps us catch typos like
// Status("oepn") being used where a real status constant was expected.
type Status string

// The only three statuses a ticket can ever be in. Keeping this list short
// and closed (no "pending", no "cancelled", etc.) is a deliberate choice
// from the assignment: keep the system simple.
const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

// IsValid reports whether s is one of the three known statuses. Any value
// coming from outside the program (an HTTP request body, for example) must
// be checked with this before we trust it.
func (s Status) IsValid() bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusClosed:
		return true
	default:
		return false
	}
}

// Ticket represents one support/issue ticket created by a user.
//
// UserID records who owns this ticket. Every place in the code that reads
// or changes a ticket must also check UserID against the logged-in user -
// that is how we guarantee "you can only see and change your own tickets".
type Ticket struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	UserID      int64     `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ErrInvalidTransition is returned by CanTransition when a status change is
// not allowed. Handlers turn this into an HTTP 400 response with a clear
// message, so the caller understands exactly why their request was rejected.
var ErrInvalidTransition = errors.New("invalid status transition")

// CanTransition is the single source of truth for "which status changes are
// allowed". Every other part of the program (the HTTP handler, the tests)
// calls this function instead of re-implementing the rule, so the business
// logic lives in exactly one place and can't drift out of sync.
//
// The allowed moves are, in plain English:
//   - A ticket that is "open" can move to "in_progress" or straight to "closed".
//   - A ticket that is "in_progress" can move to "closed".
//   - A ticket that is "closed" can NEVER move anywhere else. Once work is
//     marked done, it is done for good - this mirrors how most real ticket
//     systems treat a closed ticket as an audit record, not something to be
//     silently reopened and edited.
//   - Setting the status to the value it already has is rejected too. This
//     keeps every successful PATCH meaningful: if it succeeds, something
//     actually changed.
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
		StatusClosed: {
			// A closed ticket has no allowed next states.
		},
	}

	if allowed[from][to] {
		return nil
	}
	return ErrInvalidTransition
}
