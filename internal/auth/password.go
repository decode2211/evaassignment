// This file handles turning a plain-text password into something safe to
// store, and checking a plain-text password against that stored value. The
// actual password a user types is never written to the database, logged,
// or sent back in any response - only its bcrypt hash is kept.
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword turns a plain-text password into a bcrypt hash suitable for
// storing in the database.
//
// Why bcrypt: unlike a plain hash such as SHA-256, bcrypt is deliberately
// slow and includes a random "salt" automatically. That means (1) an
// attacker who steals the database cannot quickly try millions of guesses
// per second against it, and (2) two users with the same password get
// different stored hashes, so the database itself can't reveal that they
// share a password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether password matches the given bcrypt hash. It
// returns true only for an exact match; any error (corrupt hash, mismatch)
// is treated as "not matching" from the caller's point of view.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
