package transport

import (
	"context"
	"errors"
	"reflect"
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
