package markdown

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrInvalidYAML is returned when a frontmatter block is present but
// its YAML cannot be parsed. Wrap with %w; check with errors.Is.
var ErrInvalidYAML = errors.New("invalid yaml frontmatter")

// YAMLFrontmatter is the parsed result of a richer YAML frontmatter
// block. Used by surfaces that need PyYAML-equivalent parsing — block
// lists, nested values, typed scalars. The simpler ParseFrontmatter
// (which mirrors references/scripts/_rag/indexer.py byte-for-byte)
// remains the canonical parser for the indexing surface.
type YAMLFrontmatter struct {
	// Fields holds the top-level keys and their parsed values. Values
	// are whatever yaml.v3 produces: string, int, bool, []any, map[string]any.
	// Callers convert per their own needs.
	Fields map[string]any

	// Body is the note content with the frontmatter block removed and
	// leading whitespace stripped. Equals the full input when no
	// frontmatter is present.
	Body string
}

// ParseFrontmatterYAML parses a note's frontmatter as full YAML and
// returns the parsed fields plus the post-frontmatter body. Mirrors
// python-frontmatter's behavior closely enough for parity-port consumers:
//
//   - Frontmatter is delimited by leading "---" and a closing "\n---".
//   - The body after the closing "\n---" is left-stripped.
//   - If no frontmatter is present, Fields is empty and Body equals
//     the full input.
//   - Malformed YAML inside a frontmatter block returns ErrInvalidYAML.
//
// Consumers needing the byte-for-byte indexer parser (title + tags only)
// should use ParseFrontmatter, not this function.
func ParseFrontmatterYAML(content string) (YAMLFrontmatter, error) {
	out := YAMLFrontmatter{Fields: map[string]any{}, Body: content}

	if !strings.HasPrefix(content, "---") {
		return out, nil
	}
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return out, nil
	}
	block := rest[:idx]
	body := rest[idx+4:]
	out.Body = strings.TrimLeft(body, " \t\n\v\f\r")

	if strings.TrimSpace(block) == "" {
		return out, nil
	}

	if err := yaml.Unmarshal([]byte(block), &out.Fields); err != nil {
		return out, fmt.Errorf("%w: %v", ErrInvalidYAML, err)
	}
	if out.Fields == nil {
		out.Fields = map[string]any{}
	}
	return out, nil
}

// StringTags is a convenience: pull a "tags" field out of YAMLFrontmatter
// as a normalized []string. Handles both YAML list form (`tags: [a, b]`
// and block form) and a comma-separated string form (`tags: "a, b"`).
// Returns nil if no tags field exists. Tags are NOT lowercased; callers
// normalize as needed.
func (f YAMLFrontmatter) StringTags() []string {
	raw, ok := f.Fields["tags"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case string:
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	return nil
}
