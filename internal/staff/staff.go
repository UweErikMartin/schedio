// Package staff loads the YAML staff/user configuration file at startup and
// calls DomainStore.SyncUsers to write the entries into the active backend.
//
// File format (YAML):
//
//	users:
//	  - email: alice@example.com
//	    name: Alice
//	    role: staff                   # "staff" | "administrator"
//	    password_hash: "$2a$12$..."   # bcrypt hash (cost ≥ 12)
//	    apple_oauth_enabled: true
//	    apple_subject: "001234.abc…"  # Apple id_token `sub`
//
// The password_hash field may be omitted for Apple-only users. At least one
// user with role "administrator" should exist.
package staff

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	"schedio/internal/store"

	"gopkg.in/yaml.v3"
)

// fileConfig is the top-level structure of the YAML staff config file.
type fileConfig struct {
	Users []userEntry `yaml:"users"`
}

// userEntry mirrors one list item in the YAML file.
type userEntry struct {
	Email             string `yaml:"email"`
	Name              string `yaml:"name"`
	Role              string `yaml:"role"`
	PasswordHash      string `yaml:"password_hash"`
	AppleOAuthEnabled bool   `yaml:"apple_oauth_enabled"`
	AppleSubject      string `yaml:"apple_subject"`
}

// Load reads the YAML file at path, converts the entries to [store.User]
// values, and calls st.SyncUsers. It is intended to be called once at startup,
// before the HTTP server begins accepting requests.
func Load(ctx context.Context, path string, st store.DomainStore) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("staff: read config %q: %w", path, err)
	}

	var cfg fileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("staff: parse config %q: %w", path, err)
	}

	users := make([]*store.User, 0, len(cfg.Users))
	for i, e := range cfg.Users {
		if e.Email == "" {
			return fmt.Errorf("staff: user[%d] has no email", i)
		}
		role := store.UserRole(e.Role)
		if role != store.UserRoleStaff && role != store.UserRoleAdministrator {
			return fmt.Errorf("staff: user %q has invalid role %q", e.Email, e.Role)
		}
		users = append(users, &store.User{
			ID:                deterministicID(e.Email),
			Email:             e.Email,
			Name:              e.Name,
			Role:              role,
			PasswordHash:      e.PasswordHash,
			AppleOAuthEnabled: e.AppleOAuthEnabled,
			AppleSubject:      e.AppleSubject,
		})
	}

	if err := st.SyncUsers(ctx, users); err != nil {
		return fmt.Errorf("staff: sync users: %w", err)
	}
	return nil
}

// deterministicID returns a UUID-shaped identifier derived from email so that
// staff records receive a stable ID across restarts. The ID is the SHA-256 of
// the DNS-namespace UUID concatenated with the email, formatted as a UUID string.
func deterministicID(email string) string {
	// DNS namespace UUID as defined in RFC 4122 appendix C.
	const ns = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	sum := sha256.Sum256([]byte(ns + email))
	h := fmt.Sprintf("%x", sum)
	// Format: 8-4-4-4-12 (UUID shape; stable and collision-resistant for this use case).
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}
