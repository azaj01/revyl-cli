//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestCLILocal tests CLI commands that need zero backend access.
// These always pass regardless of authentication or backend availability.
func TestCLILocal(t *testing.T) {
	step(t, "version_output", func(st *testing.T) {
		result := runCLI(t, "version")
		// revyl version outputs via --version flag
		if result.ExitCode != 0 {
			result = runCLI(t, "--version")
		}
		combined := result.Stdout + result.Stderr
		if !strings.Contains(strings.ToLower(combined), "revyl") && !strings.Contains(combined, "version") {
			st.Fatalf("version output missing 'revyl' or 'version': %s", combined)
		}
	})

	step(t, "help_exits_zero", func(st *testing.T) {
		result := runCLI(t, "--help")
		if result.ExitCode != 0 {
			st.Fatalf("--help exited %d", result.ExitCode)
		}
		if len(result.Stdout) == 0 {
			st.Fatal("--help produced no output")
		}
	})

	step(t, "unknown_command_exits_nonzero", func(st *testing.T) {
		result := runCLI(t, "nonexistent-command-12345")
		if result.ExitCode == 0 {
			st.Fatal("unknown command should exit non-zero")
		}
	})

	step(t, "config_path", func(st *testing.T) {
		result := runCLI(t, "config", "path")
		if result.ExitCode != 0 {
			st.Skipf("config path not supported: %s", result.Stderr)
		}
		out := strings.TrimSpace(result.Stdout)
		if out == "" {
			st.Fatal("config path returned empty output")
		}
	})

}
