package analytics

import (
	"errors"
	"testing"
	"time"
)

func TestCompleteUsesCompletedErrorForDomainFailure(t *testing.T) {
	rec := testRecorder()
	run := testCommandRun(rec)
	err := CompletedWithExitCode(errors.New("test failed"), CommandCompletion{
		ExitCode:     1,
		Domain:       "test_run",
		DomainStatus: "failed",
		Properties: map[string]interface{}{
			"test_task_id": "task-123",
		},
	})

	run.Complete(err)

	event := lastEvent(t, rec)
	if event.Event != "cli_command_completed" {
		t.Fatalf("event = %q, want cli_command_completed", event.Event)
	}
	if got := event.Properties["exit_code"]; got != 1 {
		t.Fatalf("exit_code = %v, want 1", got)
	}
	if got := event.Properties["domain"]; got != "test_run" {
		t.Fatalf("domain = %v, want test_run", got)
	}
	if got := event.Properties["domain_status"]; got != "failed" {
		t.Fatalf("domain_status = %v, want failed", got)
	}
	if got := event.Properties["test_task_id"]; got != "task-123" {
		t.Fatalf("test_task_id = %v, want task-123", got)
	}
	if _, ok := event.Properties["error"]; ok {
		t.Fatalf("completed domain result should not set command error property")
	}
}

func TestCompleteWithoutOverrideKeepsCommandFailure(t *testing.T) {
	rec := testRecorder()
	run := testCommandRun(rec)

	run.Complete(errors.New("test not found"))

	event := lastEvent(t, rec)
	if event.Event != "cli_command_failed" {
		t.Fatalf("event = %q, want cli_command_failed", event.Event)
	}
	if got := event.Properties["exit_code"]; got != 1 {
		t.Fatalf("exit_code = %v, want 1", got)
	}
	if got := event.Properties["error"]; got != true {
		t.Fatalf("error = %v, want true", got)
	}
}

func TestCompleteSuccessIncludesZeroExitCode(t *testing.T) {
	rec := testRecorder()
	run := testCommandRun(rec)

	run.Complete(nil)

	event := lastEvent(t, rec)
	if event.Event != "cli_command_completed" {
		t.Fatalf("event = %q, want cli_command_completed", event.Event)
	}
	if got := event.Properties["exit_code"]; got != 0 {
		t.Fatalf("exit_code = %v, want 0", got)
	}
}

func TestCompletedErrorUnwrapsOriginalError(t *testing.T) {
	original := errors.New("workflow had 1 failed tests")
	err := CompletedWithExitCode(original, CommandCompletion{
		ExitCode:     1,
		Domain:       "workflow_run",
		DomainStatus: "failed",
	})

	if !errors.Is(err, original) {
		t.Fatalf("expected completed error to unwrap original error")
	}
	if err.Error() != original.Error() {
		t.Fatalf("error text = %q, want %q", err.Error(), original.Error())
	}
}

func TestCompleteUsesWorkflowCompletedError(t *testing.T) {
	rec := testRecorder()
	run := testCommandRun(rec)
	err := CompletedWithExitCode(errors.New("workflow had 1 failed tests"), CommandCompletion{
		ExitCode:     1,
		Domain:       "workflow_run",
		DomainStatus: "failed",
	})
	run.Complete(err)

	event := lastEvent(t, rec)
	if event.Event != "cli_command_completed" {
		t.Fatalf("event = %q, want cli_command_completed", event.Event)
	}
	if got := event.Properties["domain"]; got != "workflow_run" {
		t.Fatalf("domain = %v, want workflow_run", got)
	}
}

func testRecorder() *Recorder {
	return &Recorder{
		enabled:   true,
		flush:     func(TelemetryPayload) {},
		baseProps: map[string]interface{}{"service": "revyl-cli"},
	}
}

func testCommandRun(rec *Recorder) *CommandRun {
	return &CommandRun{
		rec:       rec,
		startedAt: time.Now(),
		commandID: "command-123",
		props:     map[string]interface{}{"command": "revyl test run"},
	}
}

func lastEvent(t *testing.T, rec *Recorder) TelemetryEvent {
	t.Helper()
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.events) == 0 {
		t.Fatal("expected recorded event")
	}
	return rec.events[len(rec.events)-1]
}
