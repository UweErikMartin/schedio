package config

import (
	"os"
	"testing"
)

func TestNormalizeRootPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"/", ""},
		{"foo", "/foo"},
		{"/foo", "/foo"},
		{"foo/", "/foo"},
		{"/foo/", "/foo"},
		{"foo/bar", "/foo/bar"},
		{"/foo/bar/", "/foo/bar"},
		{"//foo//bar//", "/foo/bar"},
		{"/foo//bar/", "/foo/bar"},
		{"//foo/bar//baz//", "/foo/bar/baz"},
	}

	for _, tt := range tests {
		result := normalizeRootPath(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeRootPath(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestReadConfigFile(t *testing.T) {
	configContent := `
port: 8080
bindAddress: "0.0.0.0"
rootPath: "/"
smtpUsername: "dummy_username"
smtpPassword: "dummy_password"
smtpHost: "smtp.example.com"
smtpPort: 587
`
	configFilePath := "test_config.yaml"

	// Create a temporary config file
	err := os.WriteFile(configFilePath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}
	defer os.Remove(configFilePath)

	var args Config
	err = readConfigFile(configFilePath, &args)
	if err != nil {
		t.Errorf("error reading config file: %v", err)
	}

	if args.Port != 8080 {
		t.Errorf("expected port 8080, got %d", args.Port)
	}
	if args.BindAddress != "0.0.0.0" {
		t.Errorf("expected bindAddress '0.0.0.0', got %q", args.BindAddress)
	}
	if args.RootPath != "/" {
		t.Errorf("expected rootPath '/', got %q", args.RootPath)
	}
	if args.SmtpUsername != "dummy_username" {
		t.Errorf("expected smtpUsername 'dummy_username', got %q", args.SmtpUsername)
	}
	if args.SmtpPassword != "dummy_password" {
		t.Errorf("expected smtpPassword 'dummy_password', got %q", args.SmtpPassword)
	}
	if args.SmtpHost != "smtp.example.com" {
		t.Errorf("expected smtpHost 'smtp.example.com', got %q", args.SmtpHost)
	}
	if args.SmtpPort != 587 {
		t.Errorf("expected smtpPort '587', got %q", args.SmtpPort)
	}
}

func TestParseCommandLineArgs(t *testing.T) {
	args := []string{
		"program",
		"-port", "8080",
		"-bindAddress", "127.0.0.1",
		"-rootPath", "/app",
		"-smtpUsername", "user",
		"-smtpPassword", "password",
		"-smtpHost", "smtp.example.com",
		"-smtpPort", "587",
	}

	expected := Config{
		Port:         8080,
		BindAddress:  "127.0.0.1",
		RootPath:     "/app",
		SmtpUsername: "user",
		SmtpPassword: "password",
		SmtpHost:     "smtp.example.com",
		SmtpPort:     587,
	}

	result := ParseCommandLineArgs(args)

	if result.Port != expected.Port {
		t.Errorf("expected port %d, got %d", expected.Port, result.Port)
	}
	if result.BindAddress != expected.BindAddress {
		t.Errorf("expected bindAddress %q, got %q", expected.BindAddress, result.BindAddress)
	}
	if result.RootPath != expected.RootPath {
		t.Errorf("expected rootPath %q, got %q", expected.RootPath, result.RootPath)
	}
	if result.SmtpUsername != expected.SmtpUsername {
		t.Errorf("expected smtpUsername %q, got %q", expected.SmtpUsername, result.SmtpUsername)
	}
	if result.SmtpPassword != expected.SmtpPassword {
		t.Errorf("expected smtpPassword %q, got %q", expected.SmtpPassword, result.SmtpPassword)
	}
	if result.SmtpHost != expected.SmtpHost {
		t.Errorf("expected smtpHost %q, got %q", expected.SmtpHost, result.SmtpHost)
	}
	if result.SmtpPort != expected.SmtpPort {
		t.Errorf("expected smtpPort %q, got %q", expected.SmtpPort, result.SmtpPort)
	}
	if result.ConfigFile != expected.ConfigFile {
		t.Errorf("expected configFile %q, got %q", expected.ConfigFile, result.ConfigFile)
	}
	if result.Dummy != false {
		t.Errorf("expected dummy false by default, got %v", result.Dummy)
	}
}

func TestParseCommandLineArgsDummy(t *testing.T) {
	result := ParseCommandLineArgs([]string{"program", "-dummy"})
	if !result.Dummy {
		t.Error("expected Dummy to be true when -dummy flag is set")
	}
}

const testServicesYAML = `
services:
  - uid: "aaaaaaaa-0001-4000-8000-000000000001"
    name: TestService
    summary: A short summary.
    description: A longer description.
    price: 49.50
    duration_minutes: 60
    daily_limit: 3
  - uid: "aaaaaaaa-0002-4000-8000-000000000002"
    name: FreeService
    summary: Free of charge.
    description: No cost.
    price: 0
    duration_minutes: 30
    daily_limit: 0
`

func writeServicesFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "services-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp services file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp services file: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func TestReadServicesFile(t *testing.T) {
	path := writeServicesFile(t, testServicesYAML)

	var cfg Config
	if err := readServicesFile(path, &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}

	s0 := cfg.Services[0]
	if s0.ID != "aaaaaaaa-0001-4000-8000-000000000001" {
		t.Errorf("services[0].ID = %q; want %q", s0.ID, "aaaaaaaa-0001-4000-8000-000000000001")
	}
	if s0.Name != "TestService" {
		t.Errorf("services[0].Name = %q; want %q", s0.Name, "TestService")
	}
	if s0.Summary != "A short summary." {
		t.Errorf("services[0].Summary = %q; want %q", s0.Summary, "A short summary.")
	}
	if s0.Description != "A longer description." {
		t.Errorf("services[0].Description = %q; want %q", s0.Description, "A longer description.")
	}
	if s0.Price != 49.50 {
		t.Errorf("services[0].Price = %v; want 49.50", s0.Price)
	}
	if s0.DurationMinutes != 60 {
		t.Errorf("services[0].DurationMinutes = %d; want 60", s0.DurationMinutes)
	}
	if s0.DailyLimit != 3 {
		t.Errorf("services[0].DailyLimit = %d; want 3", s0.DailyLimit)
	}

	s1 := cfg.Services[1]
	if s1.Name != "FreeService" {
		t.Errorf("services[1].Name = %q; want %q", s1.Name, "FreeService")
	}
	if s1.Price != 0 {
		t.Errorf("services[1].Price = %v; want 0", s1.Price)
	}
	if s1.DailyLimit != 0 {
		t.Errorf("services[1].DailyLimit = %d; want 0 (unlimited)", s1.DailyLimit)
	}
}

func TestReadServicesFileNotFound(t *testing.T) {
	var cfg Config
	err := readServicesFile("nonexistent_file.yaml", &cfg)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestReadServicesFileInvalidYAML(t *testing.T) {
	path := writeServicesFile(t, "services: [invalid: yaml: :")
	var cfg Config
	err := readServicesFile(path, &cfg)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestParseCommandLineArgsServicesFile(t *testing.T) {
	path := writeServicesFile(t, testServicesYAML)

	result := ParseCommandLineArgs([]string{"program", "-servicesFile", path})

	if result.ServicesFile != path {
		t.Errorf("ServicesFile = %q; want %q", result.ServicesFile, path)
	}
	if len(result.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(result.Services))
	}
	if result.Services[0].Name != "TestService" {
		t.Errorf("Services[0].Name = %q; want %q", result.Services[0].Name, "TestService")
	}
}

func TestParseCommandLineArgsNoServicesFile(t *testing.T) {
	result := ParseCommandLineArgs([]string{"program"})
	if result.ServicesFile != "" {
		t.Errorf("expected empty ServicesFile by default, got %q", result.ServicesFile)
	}
	// When no -servicesFile flag is given, the built-in dummy services must be used.
	if len(result.Services) != len(defaultServices) {
		t.Errorf("expected %d default services, got %d", len(defaultServices), len(result.Services))
	}
	for i, want := range defaultServices {
		got := result.Services[i]
		if got.Name != want.Name {
			t.Errorf("default service[%d].Name = %q; want %q", i, got.Name, want.Name)
		}
	}
}

const testUsersYAML = `
users:
  - email: staff@example.de
    password_hash: "$2a$12$TESTHASH1.............................................."
    role: staff
    apple_oauth_enabled: false
  - email: admin@example.de
    password_hash: "$2a$12$TESTHASH2.............................................."
    role: administrator
    apple_oauth_enabled: true
    apple_subject: "apple.sub.12345"
    name: "Test Admin"
`

func writeUsersFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "users-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp users file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp users file: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func TestReadUsersFile(t *testing.T) {
	path := writeUsersFile(t, testUsersYAML)

	var cfg Config
	if err := readUsersFile(path, &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(cfg.Users))
	}

	u0 := cfg.Users[0]
	if u0.Email != "staff@example.de" {
		t.Errorf("users[0].Email = %q; want %q", u0.Email, "staff@example.de")
	}
	if u0.Role != "staff" {
		t.Errorf("users[0].Role = %q; want %q", u0.Role, "staff")
	}
	if u0.AppleOAuthEnabled {
		t.Error("users[0].AppleOAuthEnabled = true; want false")
	}

	u1 := cfg.Users[1]
	if u1.Email != "admin@example.de" {
		t.Errorf("users[1].Email = %q; want %q", u1.Email, "admin@example.de")
	}
	if u1.Role != "administrator" {
		t.Errorf("users[1].Role = %q; want %q", u1.Role, "administrator")
	}
	if !u1.AppleOAuthEnabled {
		t.Error("users[1].AppleOAuthEnabled = false; want true")
	}
	if u1.AppleSubject != "apple.sub.12345" {
		t.Errorf("users[1].AppleSubject = %q; want %q", u1.AppleSubject, "apple.sub.12345")
	}
	if u1.Name != "Test Admin" {
		t.Errorf("users[1].Name = %q; want %q", u1.Name, "Test Admin")
	}
}

func TestReadUsersFileNotFound(t *testing.T) {
	var cfg Config
	err := readUsersFile("nonexistent_users_file.yaml", &cfg)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestReadUsersFileInvalidYAML(t *testing.T) {
	path := writeUsersFile(t, "users: [invalid: yaml: :")
	var cfg Config
	err := readUsersFile(path, &cfg)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestParseCommandLineArgsUsersFile(t *testing.T) {
	path := writeUsersFile(t, testUsersYAML)

	result := ParseCommandLineArgs([]string{"program", "-usersFile", path})

	if result.UsersFile != path {
		t.Errorf("UsersFile = %q; want %q", result.UsersFile, path)
	}
	if len(result.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(result.Users))
	}
	if result.Users[0].Email != "staff@example.de" {
		t.Errorf("Users[0].Email = %q; want %q", result.Users[0].Email, "staff@example.de")
	}
	if result.Users[1].Role != "administrator" {
		t.Errorf("Users[1].Role = %q; want %q", result.Users[1].Role, "administrator")
	}
}

func TestParseCommandLineArgsNoUsersFile(t *testing.T) {
	result := ParseCommandLineArgs([]string{"program"})
	if result.UsersFile != "" {
		t.Errorf("expected empty UsersFile by default, got %q", result.UsersFile)
	}
	// When no -usersFile flag is given, Users must be nil/empty (no pre-loaded users).
	if len(result.Users) != 0 {
		t.Errorf("expected 0 users by default, got %d", len(result.Users))
	}
}
