// Package entityprofile ports the manual-entity-profile path of
// references/scripts/profile-entities.py. Auto-mode (regex NER +
// candidate filtering via a second Claude call) is deferred to a
// follow-up.
//
// Manual mode: given an entity name, scan the vault for notes that
// mention it, render the generate-entity-profile skill with those
// mentions as context, and write the response to notes/entities/<date>-profile-<slug>.md.
package entityprofile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"sea-noise/internal/ai/skill"
	"sea-noise/internal/markdown"
	"sea-noise/internal/vault"
)

// Sentinel errors.
var (
	// ErrEntityNotMentioned is returned when no notes in the vault
	// contain the entity name (case-insensitive substring match).
	ErrEntityNotMentioned = errors.New("entity not mentioned in vault")
)

// Input configures a profile-entity run.
type Input struct {
	VaultRoot string
	Entity    string
	// EntityType is the optional classification (person, company,
	// project, concept, other) that GenerateAuto's filter step
	// produces. When empty, "concept" is used so the manual-mode flow
	// still writes a well-formed entity-type frontmatter key.
	EntityType string
	// Today is injected for deterministic file naming in tests. Zero
	// value -> time.Now().
	Today time.Time
}

// Result reports outcomes.
type Result struct {
	ProfilePath string // vault-relative path of the new profile note
	LogPath     string // vault-relative path of the log entry
	Mentions    int    // number of notes that mentioned the entity
}

// Mention is one note that mentions the entity, with up to 3 context
// sentences that contain the entity.
type Mention struct {
	Path    string   // vault-relative path
	Stem    string   // filename without .md
	Title   string   // from frontmatter, or stem fallback
	Context []string // up to 3 sentences containing the entity
}

// GenerateEntityProfileInput is the data the generate-entity-profile
// skill template expects. Exported so parity-discovery callers can
// render the same prompt the live workflow would assemble without
// invoking the LLM.
type GenerateEntityProfileInput struct {
	Entity       string
	MentionsText string
	MentionCount int
}

// Generate scans the vault for mentions of in.Entity, invokes the
// generate-entity-profile skill, and writes the profile note. Returns
// ErrEntityNotMentioned if no notes match.
func Generate(ctx context.Context, exec *skill.Executor, in Input) (Result, error) {
	if in.Today.IsZero() {
		in.Today = time.Now()
	}

	mentions, err := findMentions(in.VaultRoot, in.Entity)
	if err != nil {
		return Result{}, err
	}
	if len(mentions) == 0 {
		return Result{}, fmt.Errorf("%w: %q", ErrEntityNotMentioned, in.Entity)
	}

	mentionsText := formatMentions(mentions)
	body, err := exec.Execute(ctx, "generate-entity-profile", GenerateEntityProfileInput{
		Entity:       in.Entity,
		MentionsText: mentionsText,
		MentionCount: len(mentions),
	})
	if err != nil {
		return Result{}, fmt.Errorf("generate-entity-profile: %w", err)
	}

	entityType := in.EntityType
	if entityType == "" {
		entityType = "concept"
	}
	profileRel, err := writeProfile(in.VaultRoot, in.Entity, entityType, mentions, body, in.Today)
	if err != nil {
		return Result{}, err
	}

	logRel, err := appendLog(in.VaultRoot, in.Entity, profileRel, in.Today)
	if err != nil {
		return Result{}, err
	}

	return Result{
		ProfilePath: profileRel,
		LogPath:     logRel,
		Mentions:    len(mentions),
	}, nil
}

// findMentions walks the vault and returns one Mention per note that
// contains the entity name (case-insensitive substring match in body
// or title).
func findMentions(vaultRoot, entity string) ([]Mention, error) {
	notes, err := vault.Walk(vaultRoot, vault.LinksExcludeDirs)
	if err != nil {
		return nil, fmt.Errorf("walk vault: %w", err)
	}

	pattern := regexp.MustCompile("(?i)" + regexp.QuoteMeta(entity))

	out := make([]Mention, 0, len(notes))
	for _, n := range notes {
		full, err := vault.ReadFileRetry(filepath.Join(vaultRoot, n.Path))
		if err != nil {
			continue
		}
		fm, err := markdown.ParseFrontmatterYAML(string(full))
		if err != nil {
			continue
		}
		title, _ := fm.Fields["title"].(string)
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(n.Path), ".md")
		}

		searchText := fm.Body + " " + title
		if !pattern.MatchString(searchText) {
			continue
		}

		context := extractContextSentences(fm.Body, pattern, 3)
		stem := strings.TrimSuffix(filepath.Base(n.Path), ".md")
		out = append(out, Mention{
			Path:    n.Path,
			Stem:    stem,
			Title:   title,
			Context: context,
		})
	}

	return out, nil
}

var sentenceSplit = regexp.MustCompile(`[.!?\n]`)

// extractContextSentences returns up to maxSentences sentences that
// match pattern, in order of appearance.
func extractContextSentences(body string, pattern *regexp.Regexp, maxSentences int) []string {
	sentences := sentenceSplit.Split(body, -1)
	out := make([]string, 0, maxSentences)
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if pattern.MatchString(s) {
			out = append(out, s)
			if len(out) >= maxSentences {
				break
			}
		}
	}
	return out
}

// formatMentions formats the mentions list for the prompt template
// per the prototype's shape:
//
//   - **<title>** (<path>)
//     Context: <s1> | <s2>
func formatMentions(mentions []Mention) string {
	var sb strings.Builder
	for i, m := range mentions {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("- **%s** (%s)\n", m.Title, m.Path))
		ctxLines := m.Context
		if len(ctxLines) > 2 {
			ctxLines = ctxLines[:2]
		}
		sb.WriteString("  Context: " + strings.Join(ctxLines, " | "))
	}
	return sb.String()
}

// slugify produces the kebab-case slug the prototype uses for filenames.
var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// writeProfile writes the new entity profile to
// notes/entities/<date>-profile-<slug>.md. Returns the vault-relative
// path of the written file.
func writeProfile(vaultRoot, entity, entityType string, mentions []Mention, body string, today time.Time) (string, error) {
	date := today.Format("2006-01-02")
	slug := slugify(entity)
	rel := filepath.Join("notes", "entities", fmt.Sprintf("%s-profile-%s.md", date, slug))
	full := filepath.Join(vaultRoot, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(full), err)
	}

	var backlinks strings.Builder
	for i, m := range mentions {
		if i > 0 {
			backlinks.WriteString("\n")
		}
		backlinks.WriteString(fmt.Sprintf("- [[%s]] — %s", m.Stem, m.Title))
	}

	content := fmt.Sprintf(`---
title: "%s"
tags:
  - entity
  - entity/%s
  - entity-profile
date: %s
source: profile-entities
entity-type: %s
mention-count: %d
---

# %s

%s

## Vault Backlinks
%s
`, entity, entityType, date, entityType, len(mentions), entity, body, backlinks.String())

	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", full, err)
	}
	return rel, nil
}

// appendLog appends a line to _meta/entity-profiles-log.md.
func appendLog(vaultRoot, entity, profileRel string, today time.Time) (string, error) {
	rel := filepath.Join("_meta", "entity-profiles-log.md")
	full := filepath.Join(vaultRoot, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(full), err)
	}

	date := today.Format("2006-01-02")
	line := fmt.Sprintf("- %s — %s — %s\n", entity, date, profileRel)

	var existing string
	if b, err := vault.ReadFileRetry(full); err == nil {
		existing = string(b)
	} else {
		existing = "# Entity Profiles Log\n\n"
	}

	if err := os.WriteFile(full, []byte(existing+line), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", full, err)
	}
	return rel, nil
}
