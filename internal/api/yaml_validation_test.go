package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateYAMLPostsToBackendValidationEndpoint(t *testing.T) {
	var gotReq ValidateYAMLRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tests/yaml/validate-yaml" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"is_valid": false,
			"validation_type": "full_test",
			"errors": 1,
			"warnings": 0,
			"messages": [{
				"severity": "error",
				"code": "INVALID_BLOCK_TYPE",
				"message": "Unsupported block type: made_up_step",
				"field_path": "test.blocks[0].type",
				"line": 6,
				"suggestion": "Use one of: instructions, validation, extraction, manual, if, while, code_execution, module_import"
			}]
		}`))
	}))
	defer server.Close()

	client := NewClientWithBaseURL("test-key", server.URL)
	result, err := client.ValidateYAML(context.Background(), &ValidateYAMLRequest{
		YAMLContent:    "test:\n  blocks: []",
		ValidationType: "full_test",
	})
	if err != nil {
		t.Fatalf("ValidateYAML() error = %v", err)
	}
	if gotReq.ValidationType != "full_test" {
		t.Fatalf("ValidationType = %q", gotReq.ValidationType)
	}
	if result.IsValid {
		t.Fatal("IsValid = true, want false")
	}
	if len(result.Messages) != 1 || result.Messages[0].Code != "INVALID_BLOCK_TYPE" {
		t.Fatalf("messages = %#v", result.Messages)
	}
}
