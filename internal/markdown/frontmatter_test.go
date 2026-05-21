package markdown

import "testing"

func TestParseFrontmatter(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantTitle string
		wantTags  string
		wantBody  string
	}{
		{
			name:      "no frontmatter, no heading",
			in:        "Just plain text.\n",
			wantTitle: "",
			wantTags:  "",
			wantBody:  "Just plain text.\n",
		},
		{
			name:      "no frontmatter, heading falls back",
			in:        "# A Title\n\nbody here\n",
			wantTitle: "A Title",
			wantTags:  "",
			wantBody:  "# A Title\n\nbody here\n",
		},
		{
			name: "frontmatter title and tags",
			in: `---
title: My Note
tags: [alpha, beta, gamma]
---
body content`,
			wantTitle: "My Note",
			wantTags:  "alpha beta gamma",
			wantBody:  "body content",
		},
		{
			name: "frontmatter title quoted",
			in: `---
title: "Quoted Title"
---
body`,
			wantTitle: "Quoted Title",
			wantTags:  "",
			wantBody:  "body",
		},
		{
			name: "frontmatter title single-quoted",
			in: `---
title: 'Single Quoted'
---
body`,
			wantTitle: "Single Quoted",
			wantTags:  "",
			wantBody:  "body",
		},
		{
			name: "tags with whitespace and empty entries",
			in: `---
tags: [  one ,, two  ,]
---
`,
			wantTitle: "",
			wantTags:  "one two",
			wantBody:  "",
		},
		{
			name: "frontmatter without title falls back to heading",
			in: `---
tags: [x]
---
# Heading Title
`,
			wantTitle: "Heading Title",
			wantTags:  "x",
			wantBody:  "# Heading Title\n",
		},
		{
			name: "unterminated frontmatter is left as body",
			in: `---
title: x
no close marker
`,
			wantTitle: "",
			wantTags:  "",
			wantBody: `---
title: x
no close marker
`,
		},
		{
			name: "body starts after lstrip",
			in: `---
title: T
---


   leading blanks
`,
			wantTitle: "T",
			wantTags:  "",
			wantBody:  "leading blanks\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fm := ParseFrontmatter(c.in)
			if fm.Title != c.wantTitle {
				t.Errorf("Title = %q, want %q", fm.Title, c.wantTitle)
			}
			if fm.Tags != c.wantTags {
				t.Errorf("Tags = %q, want %q", fm.Tags, c.wantTags)
			}
			if fm.Body != c.wantBody {
				t.Errorf("Body = %q, want %q", fm.Body, c.wantBody)
			}
		})
	}
}

func TestTitleFromFilenameStem(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain-note", "plain note"},
		{"snake_case_note", "snake case note"},
		{"mixed-snake_case", "mixed snake case"},
		{"alreadyclean", "alreadyclean"},
		{"", ""},
	}
	for _, c := range cases {
		if got := TitleFromFilenameStem(c.in); got != c.want {
			t.Errorf("TitleFromFilenameStem(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
