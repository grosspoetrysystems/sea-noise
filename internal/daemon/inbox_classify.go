package daemon

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"sea-noise/internal/markdown"
	"sea-noise/internal/vault"
)

// InboxKind summarises what the inbox pipeline's downstream steps
// need to do for a specific triggering file. Used to short-circuit
// steps that don't match the file's shape — e.g., hydrate-stubs
// shouldn't run for a fully-formed note, youtube-transcript shouldn't
// run for a non-video note.
type InboxKind struct {
	// NeedsHydrate is true when the file looks like a clipper stub:
	// a clippings-tagged note with an http source URL and a body
	// shorter than fetchstub.MinBodyChars (200 chars).
	NeedsHydrate bool

	// NeedsTranscript is true when the file's source URL points at
	// YouTube and the file doesn't yet have a `## Transcript` section.
	// Re-evaluated AFTER process-inbox to catch tags Claude may have
	// added based on URL inspection.
	NeedsTranscript bool

	// FileNotFound is true when the path doesn't exist (e.g., file
	// was moved/deleted between fsnotify event and classification).
	// All Needs* flags will be false; the pipeline can skip everything.
	FileNotFound bool
}

// ClassifyInbox inspects a single inbox file at path and reports
// which downstream pipeline steps need to run. Uses readFileRetry
// internally to handle macOS iCloud Drive's "resource deadlock
// avoided" — fsnotify can fire before iCloud finishes materializing
// the file from the cloud, and the kernel returns EDEADLK; we retry
// with linear backoff up to ~5s.
//
// Returns InboxKind{FileNotFound: true} if the file genuinely doesn't
// exist (not an iCloud delay).
func ClassifyInbox(path string) (InboxKind, error) {
	raw, err := vault.ReadFileRetry(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return InboxKind{FileNotFound: true}, nil
		}
		return InboxKind{}, fmt.Errorf("daemon: classify %s: %w", path, err)
	}
	content := string(raw)

	fm, err := markdown.ParseFrontmatterYAML(content)
	if err != nil {
		// Unparseable frontmatter → assume regular note; process-inbox
		// will classify via Claude.
		return InboxKind{}, nil
	}

	source, _ := fm.Fields["source"].(string)
	tagsAny := fm.Fields["tags"]
	hasClippings := hasTagValue(tagsAny, "clippings")
	bodyLen := len(strings.TrimSpace(fm.Body))

	kind := InboxKind{}
	if hasClippings && strings.HasPrefix(source, "http") && bodyLen < 200 {
		kind.NeedsHydrate = true
	}
	if isYouTubeURL(source) && !strings.Contains(content, "## Transcript") {
		kind.NeedsTranscript = true
	}
	return kind, nil
}

func hasTagValue(raw any, want string) bool {
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(s), "#"), want) {
				return true
			}
		}
	case string:
		// Allow comma-separated inline form.
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(part), "#"), want) {
				return true
			}
		}
	}
	return false
}

func isYouTubeURL(source string) bool {
	if source == "" {
		return false
	}
	s := strings.ToLower(source)
	return strings.Contains(s, "youtube.com/") || strings.Contains(s, "youtu.be/")
}

// FilterInboxSteps returns the steps from the full inbox step list
// that should run given the supplied kind. process-inbox always runs
// (it's the classifier-of-last-resort); other steps gate on Needs*
// flags.
//
// allSteps is the unfiltered registry list; the slice returned
// preserves the original order.
func FilterInboxSteps(allSteps []Step, kind InboxKind) []Step {
	out := make([]Step, 0, len(allSteps))
	for _, st := range allSteps {
		switch st.Name {
		case "hydrate-stubs":
			if kind.NeedsHydrate {
				out = append(out, st)
			}
		case "youtube-transcript":
			if kind.NeedsTranscript {
				out = append(out, st)
			}
		default:
			out = append(out, st)
		}
	}
	return out
}

