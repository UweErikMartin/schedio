// Package token signs and verifies management-link tokens using HMAC-SHA256.
//
// Token format (before base64url encoding):
//
//	<bookingID> ":" <unix-timestamp> ":" <hex-HMAC>
//
// Tokens do not carry an expiry by themselves; they are invalidated by
// replacing the HMAC secret via DomainStore.SetHMACSecret.
package token

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"schedio/internal/store"

	"k8s.io/klog/v2"
)

// ErrInvalid is returned by Verify when the token is malformed or the HMAC
// does not match.
var ErrInvalid = errors.New("token: invalid or tampered")

// ErrExpired is returned by VerifyExpiring when the token's embedded timestamp
// is older than the allowed duration.
var ErrExpired = errors.New("token: expired")

// Signer holds the HMAC secret and signs / verifies tokens.
type Signer struct {
	secret []byte
}

// NewSigner loads the HMAC secret from the store. If no secret is stored yet,
// a new 32-byte random secret is generated, persisted, and then used.
func NewSigner(ctx context.Context, st store.DomainStore) (*Signer, error) {
	secret, err := st.GetHMACSecret(ctx)
	if err == nil {
		klog.V(2).Info("token: loaded existing HMAC secret from store")
		return &Signer{secret: secret}, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("token: load HMAC secret: %w", err)
	}

	// Generate a new 32-byte secret.
	secret = make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("token: generate HMAC secret: %w", err)
	}
	if err := st.SetHMACSecret(ctx, secret); err != nil {
		return nil, fmt.Errorf("token: persist HMAC secret: %w", err)
	}
	klog.Info("token: generated and stored new HMAC secret")
	return &Signer{secret: secret}, nil
}

// Sign returns a base64url-encoded token embedding bookingID and the current
// Unix timestamp.
func (s *Signer) Sign(bookingID string) string {
	ts := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	payload := bookingID + ":" + ts
	mac := s.computeMAC(payload)
	raw := payload + ":" + hex.EncodeToString(mac)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// Verify decodes and authenticates the token. On success it returns the
// embedded bookingID.
func (s *Signer) Verify(token string) (bookingID string, err error) {
	bookingID, _, err = s.decode(token)
	return
}

// VerifyExpiring is like Verify but additionally checks that the token was
// issued within maxAge. Returns ErrExpired when the timestamp is too old.
func (s *Signer) VerifyExpiring(token string, maxAge time.Duration) (bookingID string, err error) {
	bookingID, issuedAt, err := s.decode(token)
	if err != nil {
		return "", err
	}
	if time.Since(issuedAt) > maxAge {
		return "", ErrExpired
	}
	return bookingID, nil
}

// decode parses and validates the token, returning the bookingID and issue time.
func (s *Signer) decode(token string) (bookingID string, issuedAt time.Time, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", time.Time{}, ErrInvalid
	}
	// Format: <bookingID>:<timestamp>:<hex-mac>
	// bookingID may itself contain colons (UUID does not, but we split from the right).
	parts := strings.Split(string(raw), ":")
	if len(parts) < 3 {
		return "", time.Time{}, ErrInvalid
	}
	hexMAC := parts[len(parts)-1]
	tsStr := parts[len(parts)-2]
	bookingID = strings.Join(parts[:len(parts)-2], ":")

	payload := bookingID + ":" + tsStr
	expectedMAC := s.computeMAC(payload)
	gotMAC, err := hex.DecodeString(hexMAC)
	if err != nil || !hmac.Equal(expectedMAC, gotMAC) {
		return "", time.Time{}, ErrInvalid
	}

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return "", time.Time{}, ErrInvalid
	}
	return bookingID, time.Unix(ts, 0).UTC(), nil
}

// computeMAC returns the HMAC-SHA256 of msg using the signer's secret.
func (s *Signer) computeMAC(msg string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}
