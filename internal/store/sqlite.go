// This file is the SQLite implementation of the Store interface. It is the
// only place in the whole project that contains raw SQL. Keeping all
// database code in one file makes it easy to audit for the two things that
// matter most for correctness and security here: that passwords are never
// stored in plain text, and that every ticket query filters by the owning
// user.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/decode2211/evaassignment/internal/models"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// timeLayout is how we store timestamps as text in SQLite. RFC3339 in UTC
// is human-readable, sorts correctly as a string, and is exactly the format
// the API contract requires in JSON responses.
const timeLayout = time.RFC3339

// SQLiteStore implements the Store interface on top of a SQLite database
// file. SQLite is a great fit for a small assignment like this: it needs no
// separate database server, the whole database is just one file, and
// modernc.org/sqlite is a pure-Go driver so the app can be compiled into a
// tiny, fully static Docker image with no C toolchain required.
type SQLiteStore struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at dbPath, creating its parent
// directory if necessary, and makes sure the required tables exist. This is
// the one function the rest of the app calls to get a working Store.
func New(dbPath string) (*SQLiteStore, error) {
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// SQLite only allows one writer at a time. Limiting the pool to a
	// single connection avoids "database is locked" errors under
	// concurrent requests, at the cost of serializing writes - a perfectly
	// fine trade-off for a small ticket system.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return s, nil
}

// Close releases the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// migrate creates the users and tickets tables if they do not already
// exist. Using "CREATE TABLE IF NOT EXISTS" keeps this idempotent, so it is
// safe to run every time the server starts.
func (s *SQLiteStore) migrate() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		email         TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		name          TEXT NOT NULL DEFAULT '',
		created_at    TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS tickets (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id     INTEGER NOT NULL REFERENCES users(id),
		title       TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL,
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets(user_id);
	`
	_, err := s.db.Exec(schema)
	return err
}

// CreateUser inserts a new user row. The caller is responsible for hashing
// the password before it ever reaches this function - this function only
// ever sees and stores the hash, never the original password.
func (s *SQLiteStore) CreateUser(ctx context.Context, email, passwordHash, name string) (models.User, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (email, password_hash, name, created_at) VALUES (?, ?, ?, ?)`,
		email, passwordHash, name, now.Format(timeLayout),
	)
	if err != nil {
		// SQLite reports a UNIQUE constraint violation as a plain error
		// string; checking its text is the standard way to detect this
		// with the database/sql package (there is no portable typed error).
		if strings.Contains(err.Error(), "UNIQUE") {
			return models.User{}, ErrDuplicateEmail
		}
		return models.User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.User{}, err
	}
	return models.User{
		ID:           id,
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
		CreatedAt:    now,
	}, nil
}

// scanUser reads one row from a *sql.Row/*sql.Rows into a models.User.
func scanUser(row interface{ Scan(...any) error }) (models.User, error) {
	var (
		u         models.User
		createdAt string
	)
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &createdAt); err != nil {
		return models.User{}, err
	}
	t, err := time.Parse(timeLayout, createdAt)
	if err != nil {
		return models.User{}, err
	}
	u.CreatedAt = t
	return u, nil
}

// GetUserByEmail finds a user by their email address, used during login.
func (s *SQLiteStore) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, name, created_at FROM users WHERE email = ?`,
		email,
	)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	return u, err
}

// GetUserByID finds a user by primary key, used by the auth middleware to
// re-check (on every request) that the user encoded in a JWT still exists.
func (s *SQLiteStore) GetUserByID(ctx context.Context, id int64) (models.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, name, created_at FROM users WHERE id = ?`,
		id,
	)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	return u, err
}

// CreateTicket inserts a new ticket for userID. Every new ticket starts
// life with status "open" - this function does not accept a status
// parameter at all, so there is no way to accidentally create a ticket in
// any other state.
func (s *SQLiteStore) CreateTicket(ctx context.Context, userID int64, title, description string) (models.Ticket, error) {
	now := time.Now().UTC()
	nowStr := now.Format(timeLayout)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO tickets (user_id, title, description, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		userID, title, description, string(models.StatusOpen), nowStr, nowStr,
	)
	if err != nil {
		return models.Ticket{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.Ticket{}, err
	}
	return models.Ticket{
		ID:          id,
		Title:       title,
		Description: description,
		Status:      models.StatusOpen,
		UserID:      userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// scanTicket reads one row into a models.Ticket.
func scanTicket(row interface{ Scan(...any) error }) (models.Ticket, error) {
	var (
		t                    models.Ticket
		status               string
		createdAt, updatedAt string
	)
	if err := row.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &status, &createdAt, &updatedAt); err != nil {
		return models.Ticket{}, err
	}
	t.Status = models.Status(status)

	c, err := time.Parse(timeLayout, createdAt)
	if err != nil {
		return models.Ticket{}, err
	}
	u, err := time.Parse(timeLayout, updatedAt)
	if err != nil {
		return models.Ticket{}, err
	}
	t.CreatedAt = c
	t.UpdatedAt = u
	return t, nil
}

// ListTicketsByUser returns all tickets belonging to userID, newest first.
// The "WHERE user_id = ?" clause is what actually enforces ownership here -
// it is impossible for this query to return another user's tickets, no
// matter what a caller further up the stack does or forgets to do.
func (s *SQLiteStore) ListTicketsByUser(ctx context.Context, userID int64) ([]models.Ticket, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, title, description, status, created_at, updated_at
		 FROM tickets WHERE user_id = ? ORDER BY id DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Start with an empty (non-nil) slice so that a user with zero tickets
	// gets back "[]" in JSON, never "null" - the API contract requires this.
	tickets := make([]models.Ticket, 0)
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

// GetTicketByIDForUser fetches one ticket, but the SQL query itself
// requires both the ID to match AND the owner to match. If the ticket
// belongs to someone else, this query finds zero rows - exactly the same
// result as if the ticket never existed at all. That is deliberate: it
// means a curious user cannot even tell whether ticket #42 belongs to
// another real user or simply does not exist, which avoids leaking
// information about other accounts.
func (s *SQLiteStore) GetTicketByIDForUser(ctx context.Context, id, userID int64) (models.Ticket, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, title, description, status, created_at, updated_at
		 FROM tickets WHERE id = ? AND user_id = ?`,
		id, userID,
	)
	t, err := scanTicket(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Ticket{}, ErrNotFound
	}
	return t, err
}

// UpdateTicketStatus changes a ticket's status after checking ownership and
// the allowed-transition rules. Everything happens inside one database
// transaction so that two requests changing the same ticket at the same
// moment can never both succeed based on stale information (no race
// condition): the transaction locks the row for the duration of the
// read-check-write, so a second concurrent request simply waits its turn
// and then sees the already-updated status.
func (s *SQLiteStore) UpdateTicketStatus(ctx context.Context, id, userID int64, newStatus models.Status) (models.Ticket, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Ticket{}, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	row := tx.QueryRowContext(ctx,
		`SELECT id, user_id, title, description, status, created_at, updated_at
		 FROM tickets WHERE id = ? AND user_id = ?`,
		id, userID,
	)
	current, err := scanTicket(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Ticket{}, ErrNotFound
	}
	if err != nil {
		return models.Ticket{}, err
	}

	if err := models.CanTransition(current.Status, newStatus); err != nil {
		return models.Ticket{}, err
	}

	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx,
		`UPDATE tickets SET status = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		string(newStatus), now.Format(timeLayout), id, userID,
	)
	if err != nil {
		return models.Ticket{}, err
	}

	if err := tx.Commit(); err != nil {
		return models.Ticket{}, err
	}

	current.Status = newStatus
	current.UpdatedAt = now
	return current, nil
}
