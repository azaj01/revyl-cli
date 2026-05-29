package analytics

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const (
	maxOutputEvents = 30
	maxOutputTail   = 20
)

type CommandRun struct {
	rec       *Recorder
	startedAt time.Time
	commandID string
	props     map[string]interface{}

	mu          sync.Mutex
	outputCount int
	outputTail  []map[string]interface{}
}

// CommandCompletion describes a command that analytically completed even if it
// intentionally returns a non-zero exit code for callers such as CI.
type CommandCompletion struct {
	ExitCode     int
	Domain       string
	DomainStatus string
	Properties   map[string]interface{}
}

type CompletedError struct {
	err        error
	completion CommandCompletion
}

func CompletedWithExitCode(err error, completion CommandCompletion) error {
	if err == nil {
		err = errors.New("command completed with non-zero exit")
	}
	return &CompletedError{err: err, completion: completion}
}

func (e *CompletedError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *CompletedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *CompletedError) Completion() CommandCompletion {
	if e == nil {
		return CommandCompletion{}
	}
	return copyCommandCompletion(e.completion)
}

func (r *Recorder) StartCommand(cmd *cobra.Command, args []string) *CommandRun {
	if !r.Enabled() || cmd == nil {
		return nil
	}
	run := &CommandRun{
		rec:       r,
		startedAt: time.Now(),
		commandID: uuid.NewString(),
	}
	run.props = r.commandProps(cmd, args, run.commandID)
	run.capture("cli_command_started", nil)
	return run
}

func (r *CommandRun) Complete(err error) {
	if r == nil || !r.rec.Enabled() {
		return
	}
	props := map[string]interface{}{
		"duration_ms": time.Since(r.startedAt).Milliseconds(),
	}
	var completedErr *CompletedError
	if errors.As(err, &completedErr) {
		completion := completedErr.Completion()
		props["exit_code"] = completion.ExitCode
		if domain := strings.TrimSpace(completion.Domain); domain != "" {
			props["domain"] = domain
		}
		if status := strings.TrimSpace(completion.DomainStatus); status != "" {
			props["domain_status"] = status
		}
		for key, value := range completion.Properties {
			props[key] = value
		}
		r.capture("cli_command_completed", props)
		return
	}
	if err != nil {
		props["error"] = true
		props["exit_code"] = 1
		props["error_message"] = sanitizeString(err.Error())
		props["output_tail"] = r.outputTailSnapshot()
		r.capture("cli_command_failed", props)
	} else {
		props["exit_code"] = 0
		r.capture("cli_command_completed", props)
	}
}

func (r *CommandRun) Flush() {
	if r == nil || !r.rec.Enabled() {
		return
	}
	r.rec.Flush()
}

func (r *CommandRun) ObserveOutput(level, message string) {
	if r == nil || !r.rec.Enabled() {
		return
	}
	level = strings.TrimSpace(level)
	message = sanitizeString(message)
	if message == "" {
		return
	}

	r.mu.Lock()
	r.outputCount++
	index := r.outputCount
	offset := time.Since(r.startedAt).Milliseconds()
	tailEntry := map[string]interface{}{
		"level":     level,
		"message":   message,
		"offset_ms": offset,
	}
	r.outputTail = append(r.outputTail, tailEntry)
	if len(r.outputTail) > maxOutputTail {
		r.outputTail = r.outputTail[len(r.outputTail)-maxOutputTail:]
	}
	shouldCapture := index <= maxOutputEvents
	r.mu.Unlock()

	if !shouldCapture {
		return
	}
	r.capture("cli_output", map[string]interface{}{
		"level":     level,
		"message":   message,
		"offset_ms": offset,
		"index":     index,
	})
}

func (r *CommandRun) outputTailSnapshot() []map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]map[string]interface{}, len(r.outputTail))
	copy(out, r.outputTail)
	return out
}

func copyCommandCompletion(completion CommandCompletion) CommandCompletion {
	copied := completion
	if len(completion.Properties) > 0 {
		copied.Properties = make(map[string]interface{}, len(completion.Properties))
		for key, value := range completion.Properties {
			copied.Properties[key] = value
		}
	}
	return copied
}

type TelemetryPayload struct {
	Events []TelemetryEvent `json:"events"`
}

type TelemetryEvent struct {
	Event      string                 `json:"event"`
	Timestamp  time.Time              `json:"timestamp"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

func (r *CommandRun) capture(event string, props map[string]interface{}) {
	if r == nil || !r.rec.Enabled() || strings.TrimSpace(event) == "" {
		return
	}
	merged := r.rec.eventProps(r)
	for key, value := range props {
		merged[key] = value
	}

	evt := TelemetryEvent{
		Event:      event,
		Timestamp:  time.Now(),
		Properties: merged,
	}

	r.rec.mu.Lock()
	r.rec.events = append(r.rec.events, evt)
	r.rec.mu.Unlock()
}
