// Password hashing and checking.
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword hashes a plain-text password with bcrypt.
// bcrypt is slow on purpose and salts automatically, so a stolen database
// can't be brute-forced quickly and matching passwords don't produce matching hashes.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether password matches the given bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
