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
