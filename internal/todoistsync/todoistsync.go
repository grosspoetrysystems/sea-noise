// Package todoistsync ports the Sea Noise ↔ Todoist note-sync
// workflows from references/scripts/sync-todoist.py and
// references/scripts/sync-todoist-completions.py.
//
// Two surfaces:
//
//   - PushTasks scans notes/todos/*.md, finds notes without a
//     `synced-todoist` frontmatter key, POSTs a task to Todoist, and
//     records the task id back into the note's frontmatter.
//   - PullCompletions queries Todoist for tasks completed in a recent
//     window, matches them back to vault notes via the recorded
//     `synced-todoist` id, and marks the matching notes as done.
//
// Both workflows mutate frontmatter byte-locally (string surgery
// against the existing block) so unrelated keys, ordering, and
// formatting survive untouched.
package todoistsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"sea-noise/internal/markdown"
	"sea-noise/internal/todoist"
)

// TodosDir is the vault-relative directory the prototype scripts read.
const TodosDir = "notes/todos"

// MindPalaceLabel is the Todoist label the prototype tags every
// synced task with. PullCompletions also filters on it.
const MindPalaceLabel = "mind-palace"

// DefaultVaultName is the Obsidian vault name the prototype hardcodes
// for the `obsidian://open?vault=...` link inside task content. The
// flag on the subcommand overrides it.
const DefaultVaultName = "mind-palace"

// DefaultCompletionsLookback mirrors the prototype's hard-coded 14-day
// window for pulling completions.
const DefaultCompletionsLookback = 14 * 24 * time.Hour

// ProjectIDs holds the Todoist project routing config. Empty fields
// mean "use Todoist default inbox for that route." Mirrors the
// prototype's TODOIST_*_ID env-driven config.
type ProjectIDs struct {
	Inbox    string
	Work     string
	Personal string
}

// PushTasksInput configures a PushTasks run.
type PushTasksInput struct {
	VaultRoot string
	VaultName string // Obsidian vault name for the deep-link, defaults to DefaultVaultName.
	Projects  ProjectIDs
}

// PushTasksResult reports per-note outcomes. All paths are
// vault-relative.
type PushTasksResult struct {
	Synced  []string
	Skipped []string
	Failed  []string
}

// PullCompletionsInput configures a PullCompletions run.
type PullCompletionsInput struct {
	VaultRoot string
	Lookback  time.Duration // window from now, defaults to DefaultCompletionsLookback
	Now       time.Time     // injected clock; defaults to time.Now() when zero
}

// PullCompletionsResult reports counts plus the relative paths of
// notes that flipped from open to done.
type PullCompletionsResult struct {
	Completed []string
	Skipped   []string // notes whose synced-todoist id didn't match any completed task, or that were already done
}

// PushTasks walks notes/todos/*.md, pushes new ones to Todoist, and
// records the returned task id back into each note's frontmatter.
func PushTasks(ctx context.Context, client *todoist.Client, in PushTasksInput) (PushTasksResult, error) {
	var result PushTasksResult
	if in.VaultName == "" {
		in.VaultName = DefaultVaultName
	}
	dir := filepath.Join(in.VaultRoot, TodosDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("todoistsync: read %s: %w", dir, err)
	}
	paths := mdPaths(entries, dir)
	for _, p := range paths {
		rel, _ := filepath.Rel(in.VaultRoot, p)
		outcome, err := pushOne(ctx, client, in, p)
		if err != nil {
			return result, err
		}
		switch outcome {
		case outcomeSynced:
			result.Synced = append(result.Synced, rel)
		case outcomeSkipped:
			result.Skipped = append(result.Skipped, rel)
		case outcomeFailed:
			result.Failed = append(result.Failed, rel)
		}
	}
	return result, nil
}

// PullCompletions pulls recent completed tasks from Todoist and marks
// matching vault notes as done.
func PullCompletions(ctx context.Context, client *todoist.Client, in PullCompletionsInput) (PullCompletionsResult, error) {
	var result PullCompletionsResult
	if in.Lookback == 0 {
		in.Lookback = DefaultCompletionsLookback
	}
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	since := now.Add(-in.Lookback)
	tasks, err := client.CompletedRange(ctx, since, now, 200)
	if err != nil {
		return result, fmt.Errorf("todoistsync: fetch completions: %w", err)
	}
	idToTask := make(map[string]todoist.CompletedTask, len(tasks))
	for _, t := range tasks {
		if !hasLabel(t.Labels, MindPalaceLabel) {
			continue
		}
		idToTask[t.ID] = t
	}

	dir := filepath.Join(in.VaultRoot, TodosDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("todoistsync: read %s: %w", dir, err)
	}
	paths := mdPaths(entries, dir)
	for _, p := range paths {
		rel, _ := filepath.Rel(in.VaultRoot, p)
		outcome, err := completeOne(p, idToTask)
		if err != nil {
			return result, err
		}
		switch outcome {
		case outcomeSynced:
			result.Completed = append(result.Completed, rel)
		case outcomeSkipped:
			result.Skipped = append(result.Skipped, rel)
		}
	}
	return result, nil
}

type outcomeKind int

const (
	outcomeSkipped outcomeKind = iota
	outcomeSynced
	outcomeFailed
)

func pushOne(ctx context.Context, client *todoist.Client, in PushTasksInput, path string) (outcomeKind, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("todoistsync: read %s: %w", path, err)
	}
	content := string(raw)
	fm, err := markdown.ParseFrontmatterYAML(content)
	if err != nil {
		return outcomeSkipped, nil
	}
	if _, ok := fm.Fields["synced-todoist"]; ok {
		return outcomeSkipped, nil
	}

	stem := strings.TrimSuffix(filepath.Base(path), ".md")
	title, _ := fm.Fields["title"].(string)
	if title == "" {
		title = titleFromStem(stem)
	}
	link := fmt.Sprintf("obsidian://open?vault=%s&file=notes/todos/%s", in.VaultName, stem)
	taskContent := fmt.Sprintf("%s [→ vault](%s)", title, link)

	req := todoist.CreateTaskRequest{
		Content: taskContent,
		Labels:  []string{MindPalaceLabel},
	}
	switch routeTagsToProject(fm.StringTags()) {
	case projectWork:
		req.ProjectID = in.Projects.Work
	case projectPersonal:
		req.ProjectID = in.Projects.Personal
	default:
		req.ProjectID = in.Projects.Inbox
	}
	if due, _ := fm.Fields["due"].(string); strings.TrimSpace(due) != "" {
		req.DueString = normalizeDueString(due)
		req.DueLang = "en"
	}

	task, err := client.CreateTask(ctx, req)
	if err != nil {
		return outcomeFailed, nil
	}

	newContent, err := appendFrontmatterKeys(content, []kv{
		{key: "synced-todoist", value: "'" + task.ID + "'"},
		{key: "synced-todoist-url", value: "'" + task.URL + "'"},
	})
	if err != nil {
		return outcomeFailed, nil
	}
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return 0, fmt.Errorf("todoistsync: write %s: %w", path, err)
	}
	return outcomeSynced, nil
}

func completeOne(path string, idToTask map[string]todoist.CompletedTask) (outcomeKind, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("todoistsync: read %s: %w", path, err)
	}
	content := string(raw)
	fm, err := markdown.ParseFrontmatterYAML(content)
	if err != nil {
		return outcomeSkipped, nil
	}
	id := stringifyFrontmatterID(fm.Fields["synced-todoist"])
	if id == "" {
		return outcomeSkipped, nil
	}
	task, ok := idToTask[id]
	if !ok {
		return outcomeSkipped, nil
	}
	if done, _ := fm.Fields["done"].(bool); done {
		return outcomeSkipped, nil
	}

	completedDate := ""
	if task.CompletedAt != "" && len(task.CompletedAt) >= 10 {
		completedDate = task.CompletedAt[:10]
	}
	newContent, err := appendFrontmatterKeys(content, []kv{
		{key: "done", value: "true"},
		{key: "completed-date", value: "'" + completedDate + "'"},
	})
	if err != nil {
		return outcomeSkipped, nil
	}
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return 0, fmt.Errorf("todoistsync: write %s: %w", path, err)
	}
	return outcomeSynced, nil
}

// --- helpers ---

type kv struct {
	key, value string
}

// appendFrontmatterKeys inserts each kv as a new line at the end of
// the content's YAML frontmatter block, just before the closing "---".
// Existing keys and ordering are preserved verbatim. Values are
// written as-given — the caller is responsible for quoting.
func appendFrontmatterKeys(content string, kvs []kv) (string, error) {
	if !strings.HasPrefix(content, "---") {
		return "", errors.New("todoistsync: note has no frontmatter")
	}
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", errors.New("todoistsync: frontmatter not closed")
	}
	insertAt := 3 + idx
	var b strings.Builder
	b.WriteString(strings.TrimRight(content[:insertAt], " \t\n"))
	for _, kv := range kvs {
		b.WriteString("\n")
		b.WriteString(kv.key)
		b.WriteString(": ")
		b.WriteString(kv.value)
	}
	b.WriteString(content[insertAt:])
	return b.String(), nil
}

type projectRoute int

const (
	projectInbox projectRoute = iota
	projectWork
	projectPersonal
)

func routeTagsToProject(tags []string) projectRoute {
	for _, t := range tags {
		switch strings.ToLower(t) {
		case "#work", "work":
			return projectWork
		case "#personal", "personal":
			return projectPersonal
		}
	}
	return projectInbox
}

var (
	datePrefix       = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-?`)
	dueShortPM       = regexp.MustCompile(`(?i)(\d)(p)\b`)
	dueShortAM       = regexp.MustCompile(`(?i)(\d)(a)\b`)
	dueBeforeBy      = regexp.MustCompile(`(?i)\b(before|by)\s+`)
	titleCaseSpace   = regexp.MustCompile(`\s+`)
	titleCaseInitial = regexp.MustCompile(`(^|\s)(\w)`)
)

func titleFromStem(stem string) string {
	s := datePrefix.ReplaceAllString(stem, "")
	s = strings.ReplaceAll(s, "-", " ")
	s = titleCaseSpace.ReplaceAllString(strings.TrimSpace(s), " ")
	return titleCaseInitial.ReplaceAllStringFunc(s, strings.ToUpper)
}

func normalizeDueString(s string) string {
	out := dueShortPM.ReplaceAllString(s, "${1}pm")
	out = dueShortAM.ReplaceAllString(out, "${1}am")
	out = dueBeforeBy.ReplaceAllString(out, "")
	return strings.TrimSpace(out)
}

func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if l == want {
			return true
		}
	}
	return false
}

// stringifyFrontmatterID coerces a synced-todoist field value to a
// string id. Accepts both string ("7000") and integer (7000) forms,
// since YAML round-trips numbers as int when unquoted.
func stringifyFrontmatterID(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case float64:
		return fmt.Sprintf("%d", int64(t))
	}
	return ""
}

func mdPaths(entries []os.DirEntry, dir string) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out
}
