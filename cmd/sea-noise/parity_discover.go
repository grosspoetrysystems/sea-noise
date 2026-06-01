package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sea-noise/internal/ai/skill"
	"sea-noise/internal/crawl"
	"sea-noise/internal/enrichtopics"
	"sea-noise/internal/entityprofile"
	"sea-noise/internal/summarizer"
)

// runParityDiscover implements `sea-noise parity-discover`. It walks
// the supplied vault directory using the same qualification rules the
// live workflow applies, renders the Go skill prompt(s) for each
// qualifying note WITHOUT invoking the LLM, and prints a JSON array
// of {kind, rel, prompt} entries to stdout.
//
// The --skill flag names a workflow alias (not necessarily a single
// skill file): `summarize-content` for summarize-transcripts (one
// skill), `profile-entities` for the profile-entities two-phase
// workflow (filter + per-entity).
//
// Used by testdata/scripts/capture-*.py to obtain Go-side prompts
// for envelope keying (per the W2 spike, path A — hybrid two-pass
// capture, extended to multi-skill workflows in W3). Test/parity
// infrastructure only; never wired into a daemon code path.
func runParityDiscover(args []string) error {
	fs := flag.NewFlagSet("parity-discover", flag.ExitOnError)
	skillName := fs.String("skill", "", "workflow alias to render: summarize-content | profile-entities | enrich-topics")
	vaultRoot := fs.String("vault", "", "absolute path to the vault root to walk")
	skillsDir := fs.String("skills-dir", "", "directory containing inference skill files (defaults to .agents/shared/skills/inference relative to repo root or binary)")
	mode := fs.String("mode", "", "discovery mode for multi-phase workflows: filter | profile (profile-entities only)")
	keepNamesArg := fs.String("keep-names", "", "comma-separated list of entity names to render profile prompts for (--mode profile only)")
	keepTypesArg := fs.String("keep-types", "", "comma-separated list of name:type pairs assigning entity-type to each keep-name (--mode profile only)")
	crawlStubsDir := fs.String("crawl-stubs-dir", "", "directory of *.md crawl-stub blobs for enrich-topics StubCrawler (defaults to testdata/crawl-stubs/enrich-topics relative to repo root or binary)")
	notePath := fs.String("note", "", "vault-relative path of the note to render (enrich-note workflow only)")
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

	switch *skillName {
	case "summarize-content":
		s, ok := skills["summarize-content"]
		if !ok {
			return fmt.Errorf("skill %q not found in %s", "summarize-content", skillsPath)
		}
		return discoverSummarizeContent(s, *vaultRoot)
	case "profile-entities":
		filterSkill, ok := skills["filter-entity-candidates"]
		if !ok {
			return fmt.Errorf("skill %q not found in %s", "filter-entity-candidates", skillsPath)
		}
		profileSkill, ok := skills["generate-entity-profile"]
		if !ok {
			return fmt.Errorf("skill %q not found in %s", "generate-entity-profile", skillsPath)
		}
		keepNames := splitCSV(*keepNamesArg)
		keepTypes, err := parseKeepTypes(*keepTypesArg)
		if err != nil {
			return err
		}
		return discoverProfileEntities(filterSkill, profileSkill, *vaultRoot, *mode, keepNames, keepTypes)
	case "enrich-topics":
		briefSkill, ok := skills["generate-research-brief"]
		if !ok {
			return fmt.Errorf("skill %q not found in %s", "generate-research-brief", skillsPath)
		}
		stubsDir := *crawlStubsDir
		if stubsDir == "" {
			stubsDir = guessCrawlStubsDir()
		}
		return discoverEnrichTopics(briefSkill, *vaultRoot, stubsDir)
	case "enrich-note":
		noteSkill, ok := skills["enrich-single-note"]
		if !ok {
			return fmt.Errorf("skill %q not found in %s", "enrich-single-note", skillsPath)
		}
		if *notePath == "" {
			return errors.New("--note required for enrich-note workflow")
		}
		stubsDir := *crawlStubsDir
		if stubsDir == "" {
			stubsDir = guessCrawlStubsDir()
		}
		return discoverEnrichNote(noteSkill, *vaultRoot, *notePath, stubsDir)
	default:
		return fmt.Errorf("parity-discover does not yet support workflow %q (only summarize-content, profile-entities, enrich-topics, enrich-note)", *skillName)
	}
}

type discoveredPrompt struct {
	Kind   string `json:"kind"`
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
		out = append(out, discoveredPrompt{Kind: "summarize", Rel: c.Rel, Prompt: prompt})
	}
	return emitDiscovered(out)
}

// discoverProfileEntities renders the profile-entities workflow's
// Go-side prompts for parity-discovery. The workflow has two LLM
// phases: a single bulk filter-entity-candidates call, then N
// per-entity generate-entity-profile calls (where N is bounded by
// MaxAutoPerRun and gated by which candidates the filter response
// classifies as keep).
//
// The capture script orchestrates the two passes: first invoke with
// --mode filter to capture the filter envelope; parse its response
// to extract the kept names and types; then invoke with
// --mode profile --keep-names "..." --keep-types "..." to render the
// profile prompts.
func discoverProfileEntities(filterSkill, profileSkill *skill.Skill, vaultRoot, mode string, keepNames []string, keepTypes map[string]string) error {
	switch mode {
	case "", "filter":
		cand, err := entityprofile.WalkEntityFilterCandidate(entityprofile.AutoInput{VaultRoot: vaultRoot})
		if err != nil {
			return err
		}
		if len(cand.Names) == 0 {
			return emitDiscovered(nil)
		}
		prompt, err := filterSkill.Render(cand.SkillInput)
		if err != nil {
			return fmt.Errorf("render filter skill: %w", err)
		}
		return emitDiscovered([]discoveredPrompt{{Kind: "filter", Rel: "", Prompt: prompt}})
	case "profile":
		if len(keepNames) == 0 {
			return errors.New("--mode profile requires --keep-names")
		}
		keep := map[string]string{}
		for _, name := range keepNames {
			etype, ok := keepTypes[name]
			if !ok || etype == "" {
				etype = "concept"
			}
			keep[name] = etype
		}
		candidates, err := entityprofile.WalkEntityProfileCandidates(entityprofile.AutoInput{VaultRoot: vaultRoot}, keep)
		if err != nil {
			return err
		}
		out := make([]discoveredPrompt, 0, len(candidates))
		for _, c := range candidates {
			prompt, err := profileSkill.Render(c.SkillInput)
			if err != nil {
				return fmt.Errorf("render profile skill for %s: %w", c.Entity, err)
			}
			out = append(out, discoveredPrompt{Kind: "profile", Rel: c.Entity, Prompt: prompt})
		}
		return emitDiscovered(out)
	default:
		return fmt.Errorf("unknown --mode %q (want filter|profile)", mode)
	}
}

// guessCrawlStubsDir resolves the default crawl-stubs directory the
// same way guessSkillsDir does for skills — walks up from the binary's
// location (and CWD) until testdata/crawl-stubs/enrich-topics shows up.
// Returns the fallback relative path when not found; the StubCrawler
// treats a missing dir as an empty stub set.
func guessCrawlStubsDir() string {
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, exe)
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	for _, c := range candidates {
		for d := c; d != "/" && d != "."; d = filepath.Dir(d) {
			candidate := filepath.Join(d, "testdata", "crawl-stubs", "enrich-topics")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}
	}
	return "testdata/crawl-stubs/enrich-topics"
}

// discoverEnrichTopics renders the enrich-topics workflow's Go-side
// prompts for parity-discovery. Single-phase: one
// generate-research-brief LLM call per qualifying topic.
//
// Uses a StubCrawler reading from stubsDir so the rendered prompt's
// WebSection is deterministic across runs (per .tempor/exec/P1-W3-kickoff.md
// decision 4: parity does NOT require the prototype's crawl content
// to match the Go test's stub — pair envelopes by topic name).
func discoverEnrichTopics(briefSkill *skill.Skill, vaultRoot, stubsDir string) error {
	stub, err := crawl.NewStubCrawler(stubsDir)
	if err != nil {
		return fmt.Errorf("load stub crawler from %s: %w", stubsDir, err)
	}
	candidates, err := enrichtopics.WalkTopicCandidates(context.Background(), enrichtopics.AutoInput{VaultRoot: vaultRoot}, stub)
	if err != nil {
		return err
	}
	out := make([]discoveredPrompt, 0, len(candidates))
	for _, c := range candidates {
		prompt, err := briefSkill.Render(c.SkillInput)
		if err != nil {
			return fmt.Errorf("render brief skill for %s: %w", c.Topic, err)
		}
		out = append(out, discoveredPrompt{Kind: "enrich", Rel: c.Topic, Prompt: prompt})
	}
	return emitDiscovered(out)
}

// discoverEnrichNote renders the enrich-single-note prompt for the
// supplied note path. Single LLM call per invocation; mirrors the W2
// summarize-transcripts shape. Uses StubCrawler so the rendered
// prompt's WebSection is deterministic.
func discoverEnrichNote(noteSkill *skill.Skill, vaultRoot, notePath, stubsDir string) error {
	stub, err := crawl.NewStubCrawler(stubsDir)
	if err != nil {
		return fmt.Errorf("load stub crawler from %s: %w", stubsDir, err)
	}
	cand, err := enrichtopics.BuildEnrichNoteInput(context.Background(), vaultRoot, notePath, stub)
	if err != nil {
		return err
	}
	prompt, err := noteSkill.Render(cand.SkillInput)
	if err != nil {
		return fmt.Errorf("render enrich-single-note skill: %w", err)
	}
	return emitDiscovered([]discoveredPrompt{{Kind: "enrich-note", Rel: cand.Rel, Prompt: prompt}})
}

func emitDiscovered(out []discoveredPrompt) error {
	if out == nil {
		out = []discoveredPrompt{}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// parseKeepTypes parses a "Name1:type1,Name2:type2" pair list. An
// entity name may contain spaces but never a colon or comma.
func parseKeepTypes(s string) (map[string]string, error) {
	out := map[string]string{}
	if s == "" {
		return out, nil
	}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		i := strings.Index(pair, ":")
		if i < 0 {
			return nil, fmt.Errorf("--keep-types pair %q missing colon", pair)
		}
		name := strings.TrimSpace(pair[:i])
		etype := strings.TrimSpace(pair[i+1:])
		if name == "" || etype == "" {
			return nil, fmt.Errorf("--keep-types pair %q has empty name or type", pair)
		}
		out[name] = etype
	}
	return out, nil
}
