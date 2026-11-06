package transport

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"sea-noise/internal/ai/inferencerouter"
)

func TestCLITransport_Run_Success(t *testing.T) {
	// Use /bin/echo to simulate a "prompt in, response out" CLI.
	// echo prints its args and exits 0.
	tx := &CLITransport{
		Command: "echo",
		Args:    []string{"{PROMPT}"},
		Timeout: 5 * time.Second,
	}
	got, err := tx.Run(context.Background(), inferencerouter.InferenceDecision{}, "hello world")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "hello world" {
		t.Errorf("Run = %q, want %q", got, "hello world")
	}
}

func TestCLITransport_Run_EmptyOutput(t *testing.T) {
	// /usr/bin/true exits 0 with no stdout.
	tx := &CLITransport{
		Command: "true",
		Args:    []string{},
		Timeout: 5 * time.Second,
	}
	_, err := tx.Run(context.Background(), inferencerouter.InferenceDecision{}, "prompt")
	if !errors.Is(err, ErrEmptyResponse) {
		t.Errorf("err = %v, want errors.Is(err, ErrEmptyResponse)", err)
	}
}

func TestCLITransport_Run_NonZeroExit(t *testing.T) {
	// /usr/bin/false exits 1 with no output.
	tx := &CLITransport{
		Command: "false",
		Args:    []string{},
		Timeout: 5 * time.Second,
	}
	_, err := tx.Run(context.Background(), inferencerouter.InferenceDecision{}, "prompt")
	if !errors.Is(err, ErrNonZeroExit) {
		t.Errorf("err = %v, want errors.Is(err, ErrNonZeroExit)", err)
	}
}

func TestCLITransport_Run_NotFound(t *testing.T) {
	tx := &CLITransport{
		Command: "this-command-definitely-does-not-exist-anywhere",
		Args:    []string{},
		Timeout: 5 * time.Second,
	}
	_, err := tx.Run(context.Background(), inferencerouter.InferenceDecision{}, "prompt")
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Errorf("err = %v, want errors.Is(err, ErrTransportUnavailable)", err)
	}
}

func TestCLITransport_Run_Timeout(t *testing.T) {
	// `sh -c "sleep 10"` blocks past the 100ms timeout. sh ignores any
	// trailing positional args that substitutePrompt appends, so the
	// prompt parameter doesn't interfere with the sleep.
	tx := &CLITransport{
		Command: "sh",
		Args:    []string{"-c", "sleep 10"},
		Timeout: 100 * time.Millisecond,
	}
	_, err := tx.Run(context.Background(), inferencerouter.InferenceDecision{}, "prompt")
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("err = %v, want errors.Is(err, ErrTimeout)", err)
	}
}

func TestCLITransport_Run_StdoutLimitCancelsBeforeDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tx := &CLITransport{
		Command:        "sh",
		Args:           []string{"-c", "printf 12345; exec sleep 30"},
		Timeout:        5 * time.Second,
		MaxStdoutBytes: 4,
	}
	_, err := tx.Run(ctx, inferencerouter.InferenceDecision{}, "prompt")
	if !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("Run error = %v, want ErrOutputLimit", err)
	}
	if ctx.Err() != nil {
		t.Fatalf("Run waited for the caller deadline: %v", ctx.Err())
	}
}

func TestCLITransport_Run_StdoutLimitAtBoundaryAccepted(t *testing.T) {
	tx := &CLITransport{
		Command:        "sh",
		Args:           []string{"-c", "printf 1234"},
		Timeout:        5 * time.Second,
		MaxStdoutBytes: 4,
	}
	got, err := tx.Run(context.Background(), inferencerouter.InferenceDecision{}, "prompt")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "1234" {
		t.Errorf("Run = %q, want %q", got, "1234")
	}
}

func TestCLITransport_Run_StderrLimitBoundsRetainedPrefix(t *testing.T) {
	run := func(script string, maxStderrBytes int) error {
		tx := &CLITransport{
			Command:        "sh",
			Args:           []string{"-c", script},
			Timeout:        5 * time.Second,
			MaxStdoutBytes: 2,
			MaxStderrBytes: maxStderrBytes,
		}
		_, err := tx.Run(context.Background(), inferencerouter.InferenceDecision{}, "prompt")
		return err
	}

	boundedErr := run("printf ok; printf 1234567890 >&2; exit 1", 4)
	unboundedErr := run("printf ok; printf 1234567890 >&2; exit 1", 0)
	for name, err := range map[string]error{
		"bounded":   boundedErr,
		"unbounded": unboundedErr,
	} {
		if !errors.Is(err, ErrNonZeroExit) {
			t.Fatalf("%s stderr case error = %v, want ErrNonZeroExit", name, err)
		}
	}
	if !strings.Contains(boundedErr.Error(), "1234") || strings.Contains(boundedErr.Error(), "12345") {
		t.Errorf("bounded stderr did not retain exactly the four-byte diagnostic prefix: %v", boundedErr)
	}
	if !strings.Contains(unboundedErr.Error(), "1234567890") {
		t.Errorf("zero stderr limit did not retain the full diagnostic: %v", unboundedErr)
	}
}

func TestSubstitutePrompt(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		prompt string
		want   []string
	}{
		{
			name:   "placeholder substituted",
			args:   []string{"-p", "{PROMPT}", "--flag"},
			prompt: "hello",
			want:   []string{"-p", "hello", "--flag"},
		},
		{
			name:   "no placeholder appends",
			args:   []string{"exec"},
			prompt: "hello",
			want:   []string{"exec", "hello"},
		},
		{
			name:   "empty args appends",
			args:   []string{},
			prompt: "hello",
			want:   []string{"hello"},
		},
		{
			name:   "first placeholder only substituted",
			args:   []string{"{PROMPT}", "{PROMPT}"},
			prompt: "hello",
			want:   []string{"hello", "{PROMPT}"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := substitutePrompt(c.args, c.prompt)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("substitutePrompt(%v, %q) = %v, want %v", c.args, c.prompt, got, c.want)
			}
		})
	}
}

func TestMergeEnv(t *testing.T) {
	base := []string{"FOO=1", "BAR=2"}
	overrides := map[string]string{"BAR": "3", "BAZ": ""}
	got := mergeEnv(base, overrides)

	// Expect: FOO=1 (untouched), BAR=3 (overridden), BAZ= (added, empty value).
	want := map[string]string{"FOO": "1", "BAR": "3", "BAZ": ""}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	gotMap := make(map[string]string, len(got))
	for _, kv := range got {
		for i, b := range kv {
			if b == '=' {
				gotMap[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	if !reflect.DeepEqual(gotMap, want) {
		t.Errorf("mergeEnv = %v, want %v", gotMap, want)
	}
}
