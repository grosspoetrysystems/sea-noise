package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"sea-noise/internal/ai/inferencerouter"
)

// CLITransport invokes a local "prompt in, response out" CLI as a
// subprocess. Provider-agnostic: works with any CLI that accepts a
// prompt argument and writes the response to stdout. The user chooses
// which CLI to invoke via config; Sea Noise never names a specific
// provider in code.
//
// The Sea Noise runtime does not assume a specific provider. End users
// configure which CLI to invoke (and which args) via their config
// file. This matches the product positioning that Sea Noise meets
// users at whatever LLM access they already have.
//
// The prototype that this runtime ports uses `claude -p`; for parity
// testing the default config matches that invocation. Production users
// override the config to point at a different CLI.
type CLITransport struct {
	// Command is the executable name or absolute path of the CLI.
	// Provider-agnostic: configured per user environment.
	Command string

	// Args is the argv template passed to the CLI, with the literal
	// token "{PROMPT}" replaced by the actual prompt at call time.
	// Examples (illustrative — actual args depend on the configured CLI):
	//   ["-p", "{PROMPT}", "--dangerously-skip-permissions"]
	//   ["exec", "{PROMPT}"]
	// If "{PROMPT}" doesn't appear in Args, the prompt is appended
	// as the final positional arg.
	Args []string

	// Timeout is the per-call deadline. Zero defaults to 5 minutes
	// (matches the prototype's claude -p timeout).
	Timeout time.Duration

	// EnvOverrides are extra env vars merged on top of the parent
	// process's environment. An empty-string value sets the var to
	// empty (rather than unsetting). e.g., {"CLAUDECODE": ""} for
	// the claude-nested-invocation parity.
	EnvOverrides map[string]string
}

// Run invokes the configured CLI with the prompt substituted in,
// returns the stripped stdout. The decision is currently ignored —
// future versions may map decision.Target to a model-selection arg.
func (c *CLITransport) Run(ctx context.Context, _ inferencerouter.InferenceDecision, prompt string) (string, error) {
	if c.Command == "" {
		return "", fmt.Errorf("%w: no command configured", ErrTransportUnavailable)
	}
	if _, err := exec.LookPath(c.Command); err != nil {
		return "", fmt.Errorf("%w: %s not found on PATH", ErrTransportUnavailable, c.Command)
	}

	timeout := c.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	args := substitutePrompt(c.Args, prompt)

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(callCtx, c.Command, args...)
	cmd.Env = mergeEnv(os.Environ(), c.EnvOverrides)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("%w: %v", ErrTimeout, err)
		}
		if exit, ok := err.(*exec.ExitError); ok {
			snippet := strings.TrimSpace(stderr.String())
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			return "", fmt.Errorf("%w (exit %d): %s", ErrNonZeroExit, exit.ExitCode(), snippet)
		}
		return "", fmt.Errorf("%w: %v", ErrTransportUnavailable, err)
	}

	response := strings.TrimSpace(stdout.String())
	if response == "" {
		return "", ErrEmptyResponse
	}
	return response, nil
}

// substitutePrompt replaces the first occurrence of "{PROMPT}" in args
// with the prompt. If "{PROMPT}" doesn't appear, the prompt is appended.
// Returns a fresh slice; the caller's Args slice is not mutated.
func substitutePrompt(args []string, prompt string) []string {
	out := make([]string, 0, len(args)+1)
	substituted := false
	for _, a := range args {
		if !substituted && strings.Contains(a, "{PROMPT}") {
			out = append(out, strings.ReplaceAll(a, "{PROMPT}", prompt))
			substituted = true
		} else {
			out = append(out, a)
		}
	}
	if !substituted {
		out = append(out, prompt)
	}
	return out
}

// mergeEnv overlays the override map on top of base. An empty-string
// value sets the var to empty (rather than unsetting).
func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	// Build a map of base for fast override.
	merged := make(map[string]string, len(base)+len(overrides))
	keys := make([]string, 0, len(base))
	for _, kv := range base {
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			continue
		}
		k, v := kv[:i], kv[i+1:]
		if _, seen := merged[k]; !seen {
			keys = append(keys, k)
		}
		merged[k] = v
	}
	for k, v := range overrides {
		if _, seen := merged[k]; !seen {
			keys = append(keys, k)
		}
		merged[k] = v
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+merged[k])
	}
	return out
}
