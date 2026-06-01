package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"sea-noise/internal/ai/skill"
	"sea-noise/internal/summarizer"
)

// runParityDiscover implements `sea-noise parity-discover`. It walks
// the supplied vault directory using the same qualification rules the
// live workflow applies, renders the Go skill prompt for each
// qualifying note WITHOUT invoking the LLM, and prints a JSON array
// of {rel, prompt} entries to stdout.
//
// Used by testdata/scripts/capture-llm-envelopes.py to obtain Go-side
// prompts for envelope keying (per the W2 spike, path A — hybrid
// two-pass capture). Test/parity infrastructure only; never wired
// into a daemon code path.
func runParityDiscover(args []string) error {
	fs := flag.NewFlagSet("parity-discover", flag.ExitOnError)
	skillName := fs.String("skill", "", "skill name to render (currently only summarize-content)")
	vaultRoot := fs.String("vault", "", "absolute path to the vault root to walk")
	skillsDir := fs.String("skills-dir", "", "directory containing inference skill files (defaults to .agents/shared/skills/inference relative to repo root or binary)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *skillName == "" {
		return errors.New("--skill required")
	}
	if *vaultRoot == "" {
		return errors.New("--vault required")
	}

	skillsPath := *skillsDir
	if skillsPath == "" {
		skillsPath = guessSkillsDir()
	}
	skills, err := skill.LoadFromDir(skillsPath)
	if err != nil {
		return fmt.Errorf("load skills from %s: %w", skillsPath, err)
	}
	s, ok := skills[*skillName]
	if !ok {
		return fmt.Errorf("skill %q not found in %s", *skillName, skillsPath)
	}

	switch *skillName {
	case "summarize-content":
		return discoverSummarizeContent(s, *vaultRoot)
	default:
		return fmt.Errorf("parity-discover does not yet support skill %q (only summarize-content)", *skillName)
	}
}

type discoveredPrompt struct {
	Rel    string `json:"rel"`
	Prompt string `json:"prompt"`
}

func discoverSummarizeContent(s *skill.Skill, vaultRoot string) error {
	candidates, err := summarizer.WalkTranscriptCandidates(summarizer.EnrichTranscriptsInput{VaultRoot: vaultRoot})
	if err != nil {
		return err
	}

	out := make([]discoveredPrompt, 0, len(candidates))
	for _, c := range candidates {
		prompt, err := s.Render(c.SkillInput)
		if err != nil {
			return fmt.Errorf("render skill for %s: %w", c.Rel, err)
		}
		out = append(out, discoveredPrompt{Rel: c.Rel, Prompt: prompt})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
