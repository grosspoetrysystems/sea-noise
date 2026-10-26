package parity

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"sea-noise/internal/markdown"
)

// Difference is one cell of disagreement between two vault trees.
//
// Path is relative to the vault roots. Axis is one of "presence",
// "frontmatter", "wikilinks", "body". Want and Got describe the
// observed values; their concrete types depend on Axis.
type Difference struct {
	Path string
	Axis string
	Want any
	Got  any
}

func (d Difference) String() string {
	return fmt.Sprintf("%s [%s]: want %v, got %v", d.Path, d.Axis, d.Want, d.Got)
}

// Diff walks the two vault roots and returns every structural
// difference between them under the supplied Equivalence. An empty
// slice means the two trees are equivalent.
//
// Only files ending in .md are compared. Non-markdown files do not
// gate parity — the LLM-touching workflows in scope all write
// markdown. IgnorePaths is consulted on every axis.
func Diff(wantRoot, gotRoot string, eq Equivalence) ([]Difference, error) {
	ignore := make(map[string]bool, len(eq.IgnorePaths))
	for _, p := range eq.IgnorePaths {
		ignore[p] = true
	}

	wantFiles, err := listMarkdown(wantRoot, ignore)
	if err != nil {
		return nil, fmt.Errorf("parity: list want %s: %w", wantRoot, err)
	}
	gotFiles, err := listMarkdown(gotRoot, ignore)
	if err != nil {
		return nil, fmt.Errorf("parity: list got %s: %w", gotRoot, err)
	}

	var diffs []Difference

	if eq.FilesPresent {
		for f := range wantFiles {
			if _, ok := gotFiles[f]; !ok {
				diffs = append(diffs, Difference{Path: f, Axis: "presence", Want: "present", Got: "missing"})
			}
		}
		for f := range gotFiles {
			if _, ok := wantFiles[f]; !ok {
				diffs = append(diffs, Difference{Path: f, Axis: "presence", Want: "missing", Got: "present"})
			}
		}
	}

	for f, wantContent := range wantFiles {
		gotContent, ok := gotFiles[f]
		if !ok {
			continue
		}

		if eq.FrontmatterKeys {
			ds, err := diffFrontmatter(f, wantContent, gotContent)
			if err != nil {
				return nil, err
			}
			diffs = append(diffs, ds...)
		}

		if eq.WikilinkSet {
			wantLinks := normSet(markdown.ExtractWikilinks(wantContent))
			gotLinks := normSet(markdown.ExtractWikilinks(gotContent))
			if !reflect.DeepEqual(wantLinks, gotLinks) {
				diffs = append(diffs, Difference{
					Path: f, Axis: "wikilinks",
					Want: wantLinks, Got: gotLinks,
				})
			}
		}

		if eq.CompareBody {
			wfm, _ := markdown.ParseFrontmatterYAML(wantContent)
			gfm, _ := markdown.ParseFrontmatterYAML(gotContent)
			if wfm.Body != gfm.Body {
				diffs = append(diffs, Difference{
					Path: f, Axis: "body",
					Want: wfm.Body, Got: gfm.Body,
				})
			}
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].Path != diffs[j].Path {
			return diffs[i].Path < diffs[j].Path
		}
		return diffs[i].Axis < diffs[j].Axis
	})
	return diffs, nil
}

func diffFrontmatter(rel, wantContent, gotContent string) ([]Difference, error) {
	wfm, err := markdown.ParseFrontmatterYAML(wantContent)
	if err != nil {
		return nil, fmt.Errorf("parity: parse frontmatter want %s: %w", rel, err)
	}
	gfm, err := markdown.ParseFrontmatterYAML(gotContent)
	if err != nil {
		return nil, fmt.Errorf("parity: parse frontmatter got %s: %w", rel, err)
	}

	var diffs []Difference
	for k, wv := range wfm.Fields {
		gv, ok := gfm.Fields[k]
		if !ok {
			diffs = append(diffs, Difference{
				Path: rel, Axis: "frontmatter",
				Want: fmt.Sprintf("key %q present", k),
				Got:  "key missing",
			})
			continue
		}
		if !reflect.DeepEqual(wv, gv) {
			diffs = append(diffs, Difference{
				Path: rel, Axis: "frontmatter",
				Want: fmt.Sprintf("%s=%v", k, wv),
				Got:  fmt.Sprintf("%s=%v", k, gv),
			})
		}
	}
	for k := range gfm.Fields {
		if _, ok := wfm.Fields[k]; !ok {
			diffs = append(diffs, Difference{
				Path: rel, Axis: "frontmatter",
				Want: "key absent",
				Got:  fmt.Sprintf("key %q present", k),
			})
		}
	}
	return diffs, nil
}

func listMarkdown(root string, ignore map[string]bool) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if ignore[rel] {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(b)
		return nil
	})
	return out, err
}

// normSet returns a sorted, deduplicated copy of s. Comparisons via
// reflect.DeepEqual are then order-independent.
func normSet(s []string) []string {
	if len(s) == 0 {
		return s
	}
	out := append([]string(nil), s...)
	sort.Strings(out)
	j := 1
	for i := 1; i < len(out); i++ {
		if out[i] != out[i-1] {
			out[j] = out[i]
			j++
		}
	}
	return out[:j]
}
