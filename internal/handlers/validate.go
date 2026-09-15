// This file holds small, shared input-validation helpers used by more than
// one handler, so the rules for "what counts as a valid email" or "what
// counts as a valid password" are defined exactly once.
package handlers

import (
	"regexp"
	"strings"
)

// emailPattern is a deliberately simple check: something, an @ sign,
// something, a dot, something. It is not a full RFC 5322 validator (which
// would be far more complex than this project needs) - it just catches
// obviously wrong input like "not-an-email" before it reaches the database.
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// minPasswordLength is the shortest password we accept. Six characters is
// a low bar, but the assignment asks for a simple system, not a full
// password-strength policy.
const minPasswordLength = 6

// maxTitleLength is the longest a ticket title may be.
const maxTitleLength = 200

// normalizeEmail trims surrounding whitespace and lower-cases an email
// address, so "  Bob@Example.com " and "bob@example.com" are treated as
// the same login identifier both when registering and when logging in.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// isValidEmail reports whether email looks like a real email address.
func isValidEmail(email string) bool {
	return emailPattern.MatchString(email)
}
