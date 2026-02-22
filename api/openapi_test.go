package api

import (
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestOpenApiSpec_IsValidYAML(t *testing.T) {
	if len(OpenApiSpec) == 0 {
		t.Fatal("OpenApiSpec is empty")
	}

	var doc map[string]any
	if err := yaml.Unmarshal([]byte(OpenApiSpec), &doc); err != nil {
		t.Fatalf("OpenApiSpec is not valid YAML: %v", err)
	}

	// Basic sanity checks for OpenAPI structure
	if _, ok := doc["openapi"]; !ok {
		t.Fatalf("OpenApiSpec missing required 'openapi' field")
	}
	if info, ok := doc["info"]; !ok || info == nil {
		t.Fatalf("OpenApiSpec missing 'info' section")
	}
	if paths, ok := doc["paths"]; !ok || paths == nil {
		t.Fatalf("OpenApiSpec missing 'paths' section")
	}
}
