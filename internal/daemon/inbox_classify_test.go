package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyInbox_DetectsStub(t *testing.T) {
	path := writeFixtureNote(t, `---
title: A clipped article
tags:
  - clippings
source: https://example.com/article
---
short body
`)
	kind, err := ClassifyInbox(path)
	if err != nil {
		t.Fatalf("ClassifyInbox: %v", err)
	}
	if !kind.NeedsHydrate {
		t.Errorf("NeedsHydrate = false, want true (stub-shaped note)")
	}
	if kind.NeedsTranscript {
		t.Errorf("NeedsTranscript = true, want false (not a YouTube URL)")
	}
}

func TestClassifyInbox_DetectsYouTube(t *testing.T) {
	path := writeFixtureNote(t, `---
title: Some video
source: https://www.youtube.com/watch?v=abc123
---
Video description.
`)
	kind, err := ClassifyInbox(path)
	if err != nil {
		t.Fatalf("ClassifyInbox: %v", err)
	}
	if !kind.NeedsTranscript {
		t.Errorf("NeedsTranscript = false, want true (YouTube source)")
	}
	if kind.NeedsHydrate {
		t.Errorf("NeedsHydrate = true, want false (no clippings tag)")
	}
}

func TestClassifyInbox_SkipsYouTubeIfTranscriptPresent(t *testing.T) {
	path := writeFixtureNote(t, `---
source: https://youtu.be/xyz
---
# Video

## Transcript
some content already here
`)
	kind, err := ClassifyInbox(path)
	if err != nil {
		t.Fatalf("ClassifyInbox: %v", err)
	}
	if kind.NeedsTranscript {
		t.Errorf("NeedsTranscript = true, want false (already has transcript)")
	}
}

func TestClassifyInbox_PlainNote_NoFlags(t *testing.T) {
	path := writeFixtureNote(t, `---
title: Just a thought
tags:
  - idea
---
A regular note, neither a stub nor a video. The body is long enough
to not look like a stub even if there was a clippings tag.
`)
	kind, err := ClassifyInbox(path)
	if err != nil {
		t.Fatalf("ClassifyInbox: %v", err)
	}
	if kind.NeedsHydrate || kind.NeedsTranscript {
		t.Errorf("plain note flagged for special processing: %+v", kind)
	}
}

func TestClassifyInbox_MissingFile(t *testing.T) {
	kind, err := ClassifyInbox(filepath.Join(t.TempDir(), "gone.md"))
	if err != nil {
		t.Fatalf("ClassifyInbox on missing file: %v", err)
	}
	if !kind.FileNotFound {
		t.Errorf("FileNotFound = false on missing path")
	}
}

func TestFilterInboxSteps_GatesOnKind(t *testing.T) {
	all := []Step{
		{Name: "hydrate-stubs"},
		{Name: "process-inbox"},
		{Name: "youtube-transcript"},
	}

	cases := []struct {
		name string
		kind InboxKind
		want []string
	}{
		{"plain note", InboxKind{}, []string{"process-inbox"}},
		{"stub only", InboxKind{NeedsHydrate: true}, []string{"hydrate-stubs", "process-inbox"}},
		{"youtube only", InboxKind{NeedsTranscript: true}, []string{"process-inbox", "youtube-transcript"}},
		{"both", InboxKind{NeedsHydrate: true, NeedsTranscript: true}, []string{"hydrate-stubs", "process-inbox", "youtube-transcript"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterInboxSteps(all, tc.kind)
			gotNames := make([]string, len(got))
			for i, st := range got {
				gotNames[i] = st.Name
			}
			if !equalStringSlices(gotNames, tc.want) {
				t.Errorf("got = %v, want = %v", gotNames, tc.want)
			}
		})
	}
}

func writeFixtureNote(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
