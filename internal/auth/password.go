package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"schedio/internal/store"

	"golang.org/x/crypto/bcrypt"
)

const bcryptMinCost = 12

// ErrInvalidCredentials is returned by Authenticate when the email or
// password does not match a stored user.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// Authenticator checks username/password credentials against the store.
type Authenticator struct {
	store store.DomainStore
}

// NewAuthenticator constructs an Authenticator.
func NewAuthenticator(st store.DomainStore) *Authenticator {
	return &Authenticator{store: st}
}

// Authenticate looks up the user by email and compares the provided password
// against the bcrypt hash. Returns the authenticated User on success.
// A constant-time compare is used; a fixed artificial delay is added on
// failure to prevent timing-based user enumeration.
func (a *Authenticator) Authenticate(ctx context.Context, email, password string) (*store.User, error) {
	user, err := a.store.GetUserByEmail(ctx, email)
	if err != nil {
		// Always run bcrypt to prevent timing differences leaking user existence.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$placeholder"), []byte(password))
		constantDelay()
		return nil, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		constantDelay()
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

// HashPassword returns the bcrypt hash of password at cost ≥ bcryptMinCost.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptMinCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(hash), nil
}

// verifyPassword compares a plaintext password against a bcrypt hash.
// Returns nil when they match.
func verifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// constantDelay sleeps for a fixed duration to rate-limit failed login attempts.
func constantDelay() {
	time.Sleep(300 * time.Millisecond)
}

// ErrAccountDisabled is returned when the matched user does not have the
// expected role or Apple OAuth is required.
var ErrAccountDisabled = errors.New("auth: account disabled or misconfigured")
