// Shared input validation helpers.
package handlers

import (
	"regexp"
	"strings"
)

// emailPattern is a simple check, not a full RFC 5322 validator.
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

const minPasswordLength = 6
const maxTitleLength = 200

// normalizeEmail trims and lower-cases an email so it's a stable login key.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// isValidEmail reports whether email looks like a real email address.
func isValidEmail(email string) bool {
	return emailPattern.MatchString(email)
}
