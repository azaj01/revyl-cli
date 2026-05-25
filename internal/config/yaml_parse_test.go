package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLocalTestParsesCanonicalYAMLBlockShapes(t *testing.T) {
	testsDir := t.TempDir()
	path := filepath.Join(testsDir, "all-step-types.yaml")
	content := []byte(`test:
  metadata:
    name: all-step-types
    platform: ios
  build:
    name: Demo App
  variables:
    email: user@example.com
  env_vars:
    - API_URL
  location:
    latitude: 37.7749
    longitude: -122.4194
  blocks:
    - type: instructions
      step_description: Tap Sign in.
    - type: validation
      step_description: Sign in screen is visible.
    - type: extraction
      step_description: Capture the confirmation code.
      variable_name: confirmation_code
    - type: code_execution
      script: Normalize Confirmation Code
      script_id: script-uuid
    - type: module_import
      module: Shared Login
    - type: if
      condition: user is logged out
      then:
        - type: instructions
          step_description: Complete login.
      else:
        - type: validation
          step_description: Dashboard is already visible.
    - type: while
      condition: loading spinner is visible
      body:
        - type: manual
          step_type: wait
          step_description: "1"
    - type: manual
      step_type: open_app
    - type: manual
      step_type: kill_app
    - type: manual
      step_type: go_home
    - type: manual
      step_type: navigate
      step_description: app://settings
    - type: manual
      step_type: set_location
      step_description: "37.7749,-122.4194"
    - type: manual
      step_type: set_orientation
      step_description: landscape
    - type: manual
      step_type: set_appearance
      step_description: dark
    - type: manual
      step_type: download_file
      file: Terms.pdf
      file_uri: revyl-file://file-uuid
    - type: manual
      step_type: end
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := LoadLocalTest(path)
	if err != nil {
		t.Fatalf("LoadLocalTest() error = %v", err)
	}

	if got.Test.Metadata.Name != "all-step-types" {
		t.Fatalf("metadata.name = %q", got.Test.Metadata.Name)
	}
	if got.Test.Build.Name != "Demo App" {
		t.Fatalf("build.name = %q", got.Test.Build.Name)
	}
	if got.Test.Variables["email"] != "user@example.com" {
		t.Fatalf("variables.email = %q", got.Test.Variables["email"])
	}
	if len(got.Test.EnvVars) != 1 || got.Test.EnvVars[0] != "API_URL" {
		t.Fatalf("env_vars = %#v", got.Test.EnvVars)
	}
	if got.Test.Location == nil || got.Test.Location.Latitude != 37.7749 || got.Test.Location.Longitude != -122.4194 {
		t.Fatalf("location = %#v", got.Test.Location)
	}

	blocks := got.Test.Blocks
	if len(blocks) != 16 {
		t.Fatalf("len(blocks) = %d", len(blocks))
	}
	if blocks[3].Type != "code_execution" || blocks[3].Script != "Normalize Confirmation Code" || blocks[3].ScriptID != "script-uuid" {
		t.Fatalf("code_execution block = %#v", blocks[3])
	}
	if blocks[4].Type != "module_import" || blocks[4].Module != "Shared Login" {
		t.Fatalf("module_import block = %#v", blocks[4])
	}
	if blocks[5].Type != "if" || len(blocks[5].Then) != 1 || len(blocks[5].Else) != 1 {
		t.Fatalf("if block = %#v", blocks[5])
	}
	if blocks[6].Type != "while" || len(blocks[6].Body) != 1 || blocks[6].Body[0].StepType != "wait" {
		t.Fatalf("while block = %#v", blocks[6])
	}
	if blocks[14].StepType != "download_file" || blocks[14].File != "Terms.pdf" || blocks[14].FileURI != "revyl-file://file-uuid" {
		t.Fatalf("download_file block = %#v", blocks[14])
	}
}

func TestLoadLocalTestReportsSyntaxErrorsWithLineContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.yaml")
	content := []byte(`test:
  metadata:
    name: broken
  blocks:
    - type: instructions
      step_description: works
    - type: validation
      step_description: [unterminated
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadLocalTest(path)
	if err == nil {
		t.Fatal("LoadLocalTest() error = nil, want parse error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "failed to parse test file") || !strings.Contains(msg, "line") {
		t.Fatalf("LoadLocalTest() error = %q, want parse context with line", msg)
	}
}

func TestLoadLocalTestDoesNotValidateSemanticSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.yaml")
	content := []byte(`test:
  metadata:
    name: semantic
  blocks:
    - type: made_up_step
      step_description: This is structurally valid YAML.
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := LoadLocalTest(path)
	if err != nil {
		t.Fatalf("LoadLocalTest() error = %v", err)
	}
	if got.Test.Blocks[0].Type != "made_up_step" {
		t.Fatalf("block type = %q", got.Test.Blocks[0].Type)
	}
}
