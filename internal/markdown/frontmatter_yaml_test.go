package markdown

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestParseFrontmatterYAML(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantFields map[string]any
		wantBody   string
	}{
		{
			name:       "no frontmatter",
			in:         "just body\n",
			wantFields: map[string]any{},
			wantBody:   "just body\n",
		},
		{
			name:       "empty frontmatter block",
			in:         "---\n\n---\nbody\n",
			wantFields: map[string]any{},
			wantBody:   "body\n",
		},
		{
			name: "inline list tags",
			in: `---
tags: [alpha, beta]
title: A
---
body`,
			wantFields: map[string]any{
				"tags":  []any{"alpha", "beta"},
				"title": "A",
			},
			wantBody: "body",
		},
		{
			name: "block list tags",
			in: `---
tags:
  - alpha
  - beta
  - gamma
title: A
---
body`,
			wantFields: map[string]any{
				"tags":  []any{"alpha", "beta", "gamma"},
				"title": "A",
			},
			wantBody: "body",
		},
		{
			// YAML 1.1 / 1.2 typing: bare YYYY-MM-DD is a date, not a
			// string. yaml.v3 (and PyYAML / python-frontmatter) both
			// surface it as a typed timestamp. Consumers that need a
			// string form quote it in the frontmatter ("2026-03-10").
			name: "typed scalars",
			in: `---
consumed: false
low-signal: true
score: 7
date: 2026-03-10
---
body`,
			wantFields: map[string]any{
				"consumed":   false,
				"low-signal": true,
				"score":      7,
				"date":       time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
			},
			wantBody: "body",
		},
		{
			name: "unterminated frontmatter is left as body",
			in: `---
title: T
no close
`,
			wantFields: map[string]any{},
			wantBody:   "---\ntitle: T\nno close\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseFrontmatterYAML(c.in)
			if err != nil {
				t.Fatalf("ParseFrontmatterYAML: %v", err)
			}
			if !reflect.DeepEqual(got.Fields, c.wantFields) {
				t.Errorf("Fields = %#v, want %#v", got.Fields, c.wantFields)
			}
			if got.Body != c.wantBody {
				t.Errorf("Body = %q, want %q", got.Body, c.wantBody)
			}
		})
	}
}

func TestParseFrontmatterYAML_Invalid(t *testing.T) {
	in := "---\ntags: [unbalanced\n---\nbody\n"
	_, err := ParseFrontmatterYAML(in)
	if !errors.Is(err, ErrInvalidYAML) {
		t.Errorf("err = %v, want errors.Is(err, ErrInvalidYAML)", err)
	}
}

func TestStringTags(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]any
		want   []string
	}{
		{
			name:   "absent",
			fields: map[string]any{},
			want:   nil,
		},
		{
			name:   "yaml list",
			fields: map[string]any{"tags": []any{"a", "b"}},
			want:   []string{"a", "b"},
		},
		{
			name:   "comma string",
			fields: map[string]any{"tags": "a, b,  c "},
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "trims whitespace in list",
			fields: map[string]any{"tags": []any{" a ", "b"}},
			want:   []string{"a", "b"},
		},
		{
			name:   "ignores non-string list items",
			fields: map[string]any{"tags": []any{"a", 42, "b"}},
			want:   []string{"a", "b"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fm := YAMLFrontmatter{Fields: c.fields}
			got := fm.StringTags()
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("StringTags() = %#v, want %#v", got, c.want)
			}
		})
	}
}
