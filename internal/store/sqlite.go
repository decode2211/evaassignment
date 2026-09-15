// SQLite implementation of the Store interface.
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

	_ "modernc.org/sqlite"
)

// timeLayout stores timestamps as RFC3339 UTC text.
const timeLayout = time.RFC3339

// SQLiteStore implements Store on top of a SQLite database file.
type SQLiteStore struct {
	db *sql.DB
}

// New opens or creates the SQLite database at dbPath and runs migrations.
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

	// SQLite allows only one writer at a time; a single connection avoids
	// "database is locked" errors under concurrent requests.
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

// migrate creates the users and tickets tables if they don't already exist.
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

// CreateUser inserts a new user row. Only the bcrypt hash is stored, never the raw password.
func (s *SQLiteStore) CreateUser(ctx context.Context, email, passwordHash, name string) (models.User, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (email, password_hash, name, created_at) VALUES (?, ?, ?, ?)`,
		email, passwordHash, name, now.Format(timeLayout),
	)
	if err != nil {
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

// GetUserByEmail finds a user by email, used during login.
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

// GetUserByID finds a user by primary key, used to re-check a token's user still exists.
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

// CreateTicket inserts a new ticket for userID, always starting as "open".
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

// ListTicketsByUser returns all tickets owned by userID, newest first.
// The WHERE clause is what enforces ownership, not application code.
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

// GetTicketByIDForUser returns ErrNotFound both when the ticket doesn't
// exist and when it belongs to someone else, so a lookup can't confirm
// another user's ticket ID exists.
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

// UpdateTicketStatus checks ownership and the transition rules, then
// updates the row. Runs in one transaction so two concurrent requests
// can't both act on the same stale status.
func (s *SQLiteStore) UpdateTicketStatus(ctx context.Context, id, userID int64, newStatus models.Status) (models.Ticket, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Ticket{}, err
	}
	defer tx.Rollback()

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
