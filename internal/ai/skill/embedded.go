package skill

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

// embeddedSkills holds the inference skill files baked into the
// binary at build time. The source of truth lives at
// .agents/shared/skills/inference/ (per AGENTS.md repo layout); the
// embed/inference/ subdirectory here is a tracked mirror, kept in
// sync via scripts/sync-skills.sh.
//
// Embedding the skills makes the shipped binary self-contained — no
// repo-checkout dependency at runtime. `sea-noise serve` and friends
// fall back to these embedded skills when --skills-dir is not set.
//
//go:embed embed/inference/*.md
var embeddedSkills embed.FS

// LoadEmbedded returns the inference skills baked into this binary
// at build time. Equivalent to LoadFromDir(.agents/shared/skills/inference)
// against the on-disk canonical location, but works regardless of
// CWD or repo presence — that's what makes the binary shippable as a
// standalone artifact (drag-to-Applications, brew install, etc.).
//
// Callers that need development-time skill iteration should pass
// --skills-dir explicitly to read live edits from .agents/shared/
// skills/inference/ instead.
func LoadEmbedded() (map[string]*Skill, error) {
	entries, err := embeddedSkills.ReadDir("embed/inference")
	if err != nil {
		return nil, fmt.Errorf("read embedded skill dir: %w", err)
	}
	out := make(map[string]*Skill, len(entries))
	var firstErr error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		path := "embed/inference/" + e.Name()
		s, err := loadEmbeddedFile(path)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		out[s.Name] = s
	}
	return out, firstErr
}

// loadEmbeddedFile reads one skill from the embedded FS and parses
// it via the shared parseSkillBytes helper.
func loadEmbeddedFile(path string) (*Skill, error) {
	raw, err := fs.ReadFile(embeddedSkills, path)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", path, err)
	}
	return parseSkillBytes(raw, "embedded://"+path)
}
