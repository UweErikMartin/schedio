package auth

import (
	"context"
	"fmt"
	"net/http"
)

// AppleSignIn handles the Apple OAuth 2.0 / OIDC flow.
// Required environment variables: APPLE_CLIENT_ID, APPLE_TEAM_ID,
// APPLE_KEY_ID, APPLE_PRIVATE_KEY (ES256 PEM).
type AppleSignIn struct {
	clientID   string
	teamID     string
	keyID      string
	privateKey []byte // PEM-encoded ES256 private key
	store      interface {
		GetUserByEmail(ctx context.Context, email string) (interface{}, error)
	}
}

// NewAppleSignIn constructs an AppleSignIn handler.
// Returns an error when any required environment variable is missing.
func NewAppleSignIn(clientID, teamID, keyID string, privateKeyPEM []byte) (*AppleSignIn, error) {
	if clientID == "" || teamID == "" || keyID == "" || len(privateKeyPEM) == 0 {
		return nil, fmt.Errorf("auth: Apple Sign-In requires APPLE_CLIENT_ID, APPLE_TEAM_ID, APPLE_KEY_ID and APPLE_PRIVATE_KEY")
	}
	return &AppleSignIn{
		clientID:   clientID,
		teamID:     teamID,
		keyID:      keyID,
		privateKey: privateKeyPEM,
	}, nil
}

// Redirect handles GET /auth/apple — redirects the browser to Apple's
// authorization endpoint.
func (a *AppleSignIn) Redirect(w http.ResponseWriter, r *http.Request) {
	// TODO: build Apple authorization URL with state parameter and redirect.
	http.Error(w, "Apple Sign-In not yet implemented", http.StatusNotImplemented)
}

// Callback handles GET /auth/apple/callback — validates the Apple id_token,
// looks up the user, and sets a session cookie.
func (a *AppleSignIn) Callback(sm *SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: parse Apple callback, validate id_token, look up user, set cookie.
		http.Error(w, "Apple Sign-In callback not yet implemented", http.StatusNotImplemented)
	}
}
