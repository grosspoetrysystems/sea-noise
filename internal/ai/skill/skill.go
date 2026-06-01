// Package skill loads inference skills from .agents/shared/skills/inference/
// and executes them via the inference router seam.
//
// A skill is a single markdown file with YAML frontmatter declaring
// inputs and outputs, plus a body that is a text/template rendering
// of the prompt with input variables. Any caller (CLI subcommand,
// future API handler, future MCP server) invokes a skill by name
// with input values; the executor renders the prompt, asks the
// inference router for a routing decision, and dispatches via the
// transport.
//
// Skill files live under .agents/shared/skills/inference/<name>.md.
// They are agent-neutral; no provider-specific assumptions baked in.
package skill

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"sea-noise/internal/markdown"
)

// Sentinel errors. Wrap with %w; callers check via errors.Is.
var (
	// ErrSkillNotFound is returned when a named skill doesn't exist in
	// the configured skill directory.
	ErrSkillNotFound = errors.New("skill not found")

	// ErrMissingRequiredInput is returned when a required input wasn't
	// provided to Render or Execute.
	ErrMissingRequiredInput = errors.New("required input missing")

	// ErrInputTooLarge is returned when an input exceeds the skill's
	// declared max_chars.
	ErrInputTooLarge = errors.New("input exceeds max_chars")

	// ErrInvalidEnumValue is returned when an input's value isn't in
	// the skill's declared enum list.
	ErrInvalidEnumValue = errors.New("input value not in enum")

	// ErrInvalidSkill is returned when a skill file fails to parse
	// (malformed frontmatter, etc.).
	ErrInvalidSkill = errors.New("invalid skill file")
)

// Skill is one inference skill loaded from disk.
type Skill struct {
	Name        string
	Description string
	Kind        string // expected "inference"
	Inputs      []Input
	Outputs     Outputs
	Body        string // the text/template body (post-frontmatter)
	Source      string // path the skill was loaded from
}

// Input is one declared input parameter for a skill.
type Input struct {
	Name        string
	Required    bool
	MaxChars    int      // 0 = no limit
	Enum        []string // empty = no constraint
	Default     string   // applied when input is absent and not required
}

// Outputs describes the skill's expected output shape. Currently
// declarative only — the executor doesn't validate output format.
// Callers parse the output per the skill's declaration.
type Outputs struct {
	Format   string   // "markdown" | "json" | other
	Sections []string // for format=markdown
	Schema   string   // for format=json
}

// LoadFromDir reads all *.md files under dir and returns a map of skill
// name → Skill. Files that fail to parse are reported via the error
// (the partially-loaded map is returned alongside).
func LoadFromDir(dir string) (map[string]*Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read skill dir %s: %w", dir, err)
	}
	out := make(map[string]*Skill, len(entries))
	var firstErr error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		s, err := LoadFile(path)
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

// LoadFile loads a single skill from a markdown file.
func LoadFile(path string) (*Skill, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parseSkillBytes(raw, path)
}

// parseSkillBytes is the shared parse path for both on-disk and
// embedded skill files. source is used in the returned Skill's
// Source field and in error messages — pass the file path for
// LoadFile, or `embedded://<rel-path>` for the embedded loader.
func parseSkillBytes(raw []byte, source string) (*Skill, error) {
	fm, err := markdown.ParseFrontmatterYAML(string(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrInvalidSkill, source, err)
	}

	s := &Skill{Source: source, Body: fm.Body}

	if v, ok := fm.Fields["name"].(string); ok {
		s.Name = v
	}
	if v, ok := fm.Fields["description"].(string); ok {
		s.Description = v
	}
	if v, ok := fm.Fields["kind"].(string); ok {
		s.Kind = v
	}
	if s.Name == "" {
		return nil, fmt.Errorf("%w: %s: missing name", ErrInvalidSkill, source)
	}
	if s.Kind != "inference" {
		return nil, fmt.Errorf("%w: %s: kind=%q, want %q", ErrInvalidSkill, source, s.Kind, "inference")
	}

	s.Inputs = parseInputs(fm.Fields["inputs"])
	s.Outputs = parseOutputs(fm.Fields["outputs"])
	return s, nil
}

func parseInputs(raw any) []Input {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]Input, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		var in Input
		if v, ok := m["name"].(string); ok {
			in.Name = v
		}
		if v, ok := m["required"].(bool); ok {
			in.Required = v
		}
		if v, ok := m["max_chars"].(int); ok {
			in.MaxChars = v
		}
		if v, ok := m["default"].(string); ok {
			in.Default = v
		}
		if v, ok := m["enum"].([]any); ok {
			for _, e := range v {
				if s, ok := e.(string); ok {
					in.Enum = append(in.Enum, s)
				}
			}
		}
		if in.Name != "" {
			out = append(out, in)
		}
	}
	return out
}

func parseOutputs(raw any) Outputs {
	m, ok := raw.(map[string]any)
	if !ok {
		return Outputs{}
	}
	var o Outputs
	if v, ok := m["format"].(string); ok {
		o.Format = v
	}
	if v, ok := m["schema"].(string); ok {
		o.Schema = v
	}
	if v, ok := m["sections"].([]any); ok {
		for _, s := range v {
			if str, ok := s.(string); ok {
				o.Sections = append(o.Sections, str)
			}
		}
	}
	return o
}

// Render validates inputs against the skill's declarations and renders
// the prompt template with them. Returns the rendered prompt string.
//
// The template uses Go's text/template syntax. Input names are
// available as fields on a struct passed to the template — callers
// pass a struct or map[string]any that the template can address.
func (s *Skill) Render(data any) (string, error) {
	if err := s.validateInputs(data); err != nil {
		return "", err
	}
	tmpl, err := template.New(s.Name).Parse(s.Body)
	if err != nil {
		return "", fmt.Errorf("parse skill %s template: %w", s.Name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute skill %s template: %w", s.Name, err)
	}
	return buf.String(), nil
}

// validateInputs checks required-ness, max_chars, and enum constraints.
// Operates on map[string]any forms only at this scaffold stage; callers
// who pass typed structs skip validation (their compiler enforces shape).
func (s *Skill) validateInputs(data any) error {
	m, ok := data.(map[string]any)
	if !ok {
		// Typed struct: skip dynamic validation. Template execution
		// will surface missing-field errors.
		return nil
	}
	for _, in := range s.Inputs {
		v, present := m[in.Name]
		if !present {
			if in.Required {
				return fmt.Errorf("%w: %s", ErrMissingRequiredInput, in.Name)
			}
			continue
		}
		str, _ := v.(string)
		if in.MaxChars > 0 && len(str) > in.MaxChars {
			return fmt.Errorf("%w: %s (%d > %d)", ErrInputTooLarge, in.Name, len(str), in.MaxChars)
		}
		if len(in.Enum) > 0 && str != "" {
			ok := false
			for _, e := range in.Enum {
				if str == e {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("%w: %s=%q, want one of %v", ErrInvalidEnumValue, in.Name, str, in.Enum)
			}
		}
	}
	return nil
}
