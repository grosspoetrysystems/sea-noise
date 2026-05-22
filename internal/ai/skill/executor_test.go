package skill

import (
	"context"
	"errors"
	"strings"
	"testing"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/transport"
)

func TestExecutor_Execute(t *testing.T) {
	skill := &Skill{
		Name: "test",
		Kind: "inference",
		Body: "Hello, {{.Name}}!",
	}
	mock := &transport.MockTransport{}
	mock.EnqueueResponses("Hi there!")

	exec := NewExecutor(
		map[string]*Skill{"test": skill},
		inferencerouter.NewLocalRouter(),
		mock,
	)

	type input struct{ Name string }
	got, err := exec.Execute(context.Background(), "test", input{Name: "world"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "Hi there!" {
		t.Errorf("Execute = %q, want %q", got, "Hi there!")
	}

	// Confirm the rendered prompt was sent.
	if len(mock.Calls) != 1 {
		t.Fatalf("Calls length = %d, want 1", len(mock.Calls))
	}
	if mock.Calls[0].Prompt != "Hello, world!" {
		t.Errorf("Prompt = %q, want %q", mock.Calls[0].Prompt, "Hello, world!")
	}
	if mock.Calls[0].Decision.Tier != inferencerouter.TierMedium {
		t.Errorf("Decision.Tier = %v, want Medium", mock.Calls[0].Decision.Tier)
	}
}

func TestExecutor_SkillNotFound(t *testing.T) {
	exec := NewExecutor(map[string]*Skill{}, inferencerouter.NewLocalRouter(), &transport.MockTransport{})
	_, err := exec.Execute(context.Background(), "nope", nil)
	if !errors.Is(err, ErrSkillNotFound) {
		t.Errorf("err = %v, want errors.Is(err, ErrSkillNotFound)", err)
	}
}

func TestExecutor_TransportError(t *testing.T) {
	skill := &Skill{Name: "test", Kind: "inference", Body: "prompt"}
	mock := &transport.MockTransport{}
	mock.EnqueueError(transport.ErrTransportUnavailable)

	exec := NewExecutor(
		map[string]*Skill{"test": skill},
		inferencerouter.NewLocalRouter(),
		mock,
	)
	_, err := exec.Execute(context.Background(), "test", nil)
	if !errors.Is(err, transport.ErrTransportUnavailable) {
		t.Errorf("err = %v, want errors.Is(err, ErrTransportUnavailable)", err)
	}
}

func TestExecutor_RealSkillFile_RoundTrip(t *testing.T) {
	// Load the actual summarize-content skill from the repo's
	// .agents/shared/skills/inference/ tree to verify the on-disk
	// skill files parse and render under realistic inputs.
	root := repoRoot(t)
	skills, err := LoadFromDir(root + "/.agents/shared/skills/inference")
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	s, ok := skills["summarize-content"]
	if !ok {
		t.Fatalf("summarize-content not found; loaded: %v", keys(skills))
	}

	type input struct {
		Content           string
		TitleContext      string
		SourceDescription string
	}
	prompt, err := s.Render(input{
		Content:           "sample content",
		TitleContext:      " about 'Sample Title'",
		SourceDescription: "a YouTube video transcript",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(prompt, "sample content") {
		t.Error("rendered prompt missing content")
	}
	if !strings.Contains(prompt, "Sample Title") {
		t.Error("rendered prompt missing title context")
	}
	if !strings.Contains(prompt, "### Key Takeaways") {
		t.Error("rendered prompt missing Key Takeaways section header")
	}
}

func keys(m map[string]*Skill) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	// Walk up from CWD until go.mod is found.
	wd, err := getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := wd; d != "/" && d != "."; d = parentDir(d) {
		if exists(d + "/go.mod") {
			return d
		}
	}
	t.Fatal("could not locate repo root")
	return ""
}

// Tiny helpers to avoid pulling in path/filepath into this test file's
// header — they're equivalent and keep the test focused.
func getwd() (string, error) { return osGetwd() }
func parentDir(d string) string {
	for i := len(d) - 1; i >= 0; i-- {
		if d[i] == '/' {
			if i == 0 {
				return "/"
			}
			return d[:i]
		}
	}
	return d
}
func exists(p string) bool {
	_, err := osStat(p)
	return err == nil
}
