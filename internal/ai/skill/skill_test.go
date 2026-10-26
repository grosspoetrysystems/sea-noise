package skill

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleSkill = `---
name: test-skill
description: A skill used in tests.
kind: inference
inputs:
  - name: content
    required: true
    max_chars: 100
  - name: mode
    required: false
    enum: [a, b, c]
    default: a
outputs:
  format: markdown
  sections:
    - Result
---

You are doing mode {{.Mode}}.

Content:
{{.Content}}
`

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-skill.md")
	if err := os.WriteFile(path, []byte(sampleSkill), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if s.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", s.Name, "test-skill")
	}
	if s.Kind != "inference" {
		t.Errorf("Kind = %q, want %q", s.Kind, "inference")
	}
	if len(s.Inputs) != 2 {
		t.Fatalf("len(Inputs) = %d, want 2", len(s.Inputs))
	}
	if !s.Inputs[0].Required {
		t.Errorf("Inputs[0].Required = false, want true")
	}
	if s.Inputs[0].MaxChars != 100 {
		t.Errorf("Inputs[0].MaxChars = %d, want 100", s.Inputs[0].MaxChars)
	}
	if got := s.Inputs[1].Enum; len(got) != 3 || got[0] != "a" {
		t.Errorf("Inputs[1].Enum = %v, want [a b c]", got)
	}
	if s.Outputs.Format != "markdown" {
		t.Errorf("Outputs.Format = %q, want markdown", s.Outputs.Format)
	}
}

func TestLoadFile_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	body := "---\nkind: inference\n---\nbody"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFile(path)
	if !errors.Is(err, ErrInvalidSkill) {
		t.Errorf("err = %v, want errors.Is(err, ErrInvalidSkill)", err)
	}
}

func TestLoadFile_WrongKind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	body := "---\nname: x\nkind: rule\n---\nbody"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFile(path)
	if !errors.Is(err, ErrInvalidSkill) {
		t.Errorf("err = %v, want errors.Is(err, ErrInvalidSkill)", err)
	}
}

func TestLoadFromDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "one.md"), []byte(sampleSkill), 0o644); err != nil {
		t.Fatal(err)
	}
	other := strings.Replace(sampleSkill, "name: test-skill", "name: other-skill", 1)
	if err := os.WriteFile(filepath.Join(dir, "other.md"), []byte(other), 0o644); err != nil {
		t.Fatal(err)
	}
	// README.md should be skipped.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# docs"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("len(skills) = %d, want 2", len(skills))
	}
	if _, ok := skills["test-skill"]; !ok {
		t.Error("expected test-skill in catalog")
	}
	if _, ok := skills["other-skill"]; !ok {
		t.Error("expected other-skill in catalog")
	}
}

func TestRender_Map(t *testing.T) {
	s := &Skill{
		Name: "test",
		Body: "Mode {{.Mode}}, content: {{.Content}}",
		Inputs: []Input{
			{Name: "content", Required: true, MaxChars: 50},
			{Name: "mode", Enum: []string{"a", "b"}},
		},
	}

	got, err := s.Render(map[string]any{"Content": "hi", "Mode": "a", "content": "hi", "mode": "a"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != "Mode a, content: hi" {
		t.Errorf("Render = %q, want %q", got, "Mode a, content: hi")
	}
}

func TestRender_MissingRequired(t *testing.T) {
	s := &Skill{
		Name:   "test",
		Body:   "{{.Content}}",
		Inputs: []Input{{Name: "content", Required: true}},
	}
	_, err := s.Render(map[string]any{})
	if !errors.Is(err, ErrMissingRequiredInput) {
		t.Errorf("err = %v, want errors.Is(err, ErrMissingRequiredInput)", err)
	}
}

func TestRender_TooLarge(t *testing.T) {
	s := &Skill{
		Name:   "test",
		Body:   "{{.Content}}",
		Inputs: []Input{{Name: "content", MaxChars: 5}},
	}
	_, err := s.Render(map[string]any{"content": "way too long"})
	if !errors.Is(err, ErrInputTooLarge) {
		t.Errorf("err = %v, want errors.Is(err, ErrInputTooLarge)", err)
	}
}

func TestRender_InvalidEnum(t *testing.T) {
	s := &Skill{
		Name:   "test",
		Body:   "{{.Mode}}",
		Inputs: []Input{{Name: "mode", Enum: []string{"a", "b"}}},
	}
	_, err := s.Render(map[string]any{"mode": "z"})
	if !errors.Is(err, ErrInvalidEnumValue) {
		t.Errorf("err = %v, want errors.Is(err, ErrInvalidEnumValue)", err)
	}
}

func TestRender_TypedStructBypassesValidation(t *testing.T) {
	// Typed struct passed directly: template handles its own errors.
	type input struct{ Content string }
	s := &Skill{
		Name:   "test",
		Body:   "{{.Content}}",
		Inputs: []Input{{Name: "content", Required: true}},
	}
	got, err := s.Render(input{Content: "hello"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != "hello" {
		t.Errorf("Render = %q, want %q", got, "hello")
	}
}
