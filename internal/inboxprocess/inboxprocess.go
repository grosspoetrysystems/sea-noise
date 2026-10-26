// Package inboxprocess is the thin driver for the `inbox-process`
// skill.
//
// Unlike the other AI workflows (summarize-transcripts, enrich-topics,
// profile-entities), inbox-process is fully LLM-orchestrated: the
// skill prompt instructs Claude to walk the vault's inbox/ folder
// directly (using its own tool-use capabilities) and to write the
// resulting state changes — `_meta/topic-frequencies.md`,
// `_meta/rejected-connections.md`, `atlas/*` MOC updates, and per-
// note promotions — without the Go process enumerating files.
//
// This driver loads + invokes the skill via the configured executor
// and reports back the skill's textual summary. It does not
// post-process the vault.
package inboxprocess

import (
	"context"
	"fmt"

	"sea-noise/internal/ai/skill"
)

// SkillName is the name of the inbox-process skill in
// .agents/shared/skills/inference/.
const SkillName = "inbox-process"

// Input configures a Run call. SkipPaths is a newline-separated list of
// inbox-relative files that hydration could not safely materialize.
type Input struct {
	VaultRoot string
	SkipPaths string
}

// Result captures the skill's report. ReportPath is unset for now —
// the prototype does not write the summary to a file. Future work may
// route it through `_meta/inbox-runs/`.
type Result struct {
	Report string
}

type inboxProcessInput struct {
	VaultRoot string
	SkipPaths string
}

// Run invokes the inbox-process skill with the configured vault root.
// Returns the textual report the skill produced.
func Run(ctx context.Context, exec *skill.Executor, in Input) (Result, error) {
	report, err := exec.Execute(ctx, SkillName, inboxProcessInput{VaultRoot: in.VaultRoot, SkipPaths: in.SkipPaths})
	if err != nil {
		return Result{}, fmt.Errorf("inboxprocess: %w", err)
	}
	return Result{Report: report}, nil
}
