package watcher_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sea-noise/internal/watcher"
)

// eventTimeout caps any single "did the callback fire?" wait. Has to
// be long enough for fsnotify to deliver kernel events on a loaded
// CI machine — 2s is the empirical sweet spot.
const eventTimeout = 2 * time.Second

// quiescenceWindow is how long we wait after a write to assert
// "nothing fired." Has to be longer than the debounce window plus
// fsnotify delivery latency. Used only by tests that assert non-
// occurrence — for those, an elapsed-time check is fundamental: you
// cannot signal the absence of an event with a channel.
const quiescenceWindow = 300 * time.Millisecond

// shortDebounce is the per-test debounce window. Long enough for
// fsnotify to deliver the underlying event before the timer fires;
// short enough that tests don't drag. 30ms is the sweet spot on
// both darwin/kqueue and linux/inotify.
const shortDebounce = 30 * time.Millisecond

// fixture wires a test vault, a channel-based event sink for each
// callback class, and a Watcher. Tests do `f := newFixture(t)` and
// assert on `f.inbox` / `f.modify` channels.
type fixture struct {
	root   string
	inbox  chan string
	modify chan string
	w      *watcher.Watcher
	cancel context.CancelFunc
}

// newFixture stands up a minimal vault under t.TempDir, wires the
// watcher with channel-backed callbacks, and registers cleanup.
// Debounce defaults to shortDebounce; pass overrides via opts.
func newFixture(t *testing.T, opts ...fixtureOpt) *fixture {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"inbox", "topics", "_meta"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	f := &fixture{
		root:   root,
		inbox:  make(chan string, 32),
		modify: make(chan string, 32),
	}

	cfg := watcher.Config{
		VaultRoot: root,
		Debounce:  shortDebounce,
		Actions: watcher.Actions{
			OnInboxCreate: func(_ context.Context, path string) {
				select {
				case f.inbox <- path:
				default:
				}
			},
			OnNoteModify: func(_ context.Context, path string) {
				select {
				case f.modify <- path:
				default:
				}
			},
		},
	}
	for _, o := range opts {
		o(&cfg)
	}

	w, err := watcher.New(cfg)
	if err != nil {
		t.Fatalf("watcher.New: %v", err)
	}
	f.w = w
	return f
}

type fixtureOpt func(*watcher.Config)

func withDebounce(d time.Duration) fixtureOpt {
	return func(c *watcher.Config) { c.Debounce = d }
}

// start launches the watcher and arranges teardown. Returns once
// Start has completed (so startup-scan callbacks have been
// dispatched).
func (f *fixture) start(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	t.Cleanup(func() {
		cancel()
		_ = f.w.Stop()
	})
	if err := f.w.Start(ctx); err != nil {
		t.Fatalf("watcher.Start: %v", err)
	}
}

// touch writes content to root/rel.
func (f *fixture) touch(t *testing.T, rel, content string) {
	t.Helper()
	full := filepath.Join(f.root, rel)
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

// awaitInbox blocks until OnInboxCreate fires (or eventTimeout
// elapses). Returns the path the callback received.
func (f *fixture) awaitInbox(t *testing.T) string {
	t.Helper()
	select {
	case p := <-f.inbox:
		return p
	case m := <-f.modify:
		t.Fatalf("expected inbox callback, got modify for %s", m)
		return ""
	case <-time.After(eventTimeout):
		t.Fatalf("OnInboxCreate did not fire within %v", eventTimeout)
		return ""
	}
}

// awaitModify blocks until OnNoteModify fires (or eventTimeout
// elapses).
func (f *fixture) awaitModify(t *testing.T) string {
	t.Helper()
	select {
	case p := <-f.modify:
		return p
	case p := <-f.inbox:
		t.Fatalf("expected modify callback, got inbox for %s", p)
		return ""
	case <-time.After(eventTimeout):
		t.Fatalf("OnNoteModify did not fire within %v", eventTimeout)
		return ""
	}
}

// assertNoEvent waits quiescenceWindow and fails if any callback
// arrived. Used to assert non-occurrence — an elapsed-time check is
// fundamental for proving "nothing happened."
func (f *fixture) assertNoEvent(t *testing.T) {
	t.Helper()
	select {
	case p := <-f.inbox:
		t.Fatalf("unexpected OnInboxCreate for %s", p)
	case p := <-f.modify:
		t.Fatalf("unexpected OnNoteModify for %s", p)
	case <-time.After(quiescenceWindow):
		// Expected — quiet window elapsed cleanly.
	}
}

// drain consumes any pending callbacks until the channels go quiet
// for one quiescenceWindow. Used after a known-firing setup step
// (e.g., startup scan) to reset state before the assertion phase.
func (f *fixture) drain() {
	for {
		select {
		case <-f.inbox:
		case <-f.modify:
		case <-time.After(quiescenceWindow):
			return
		}
	}
}

func TestWatcher_InboxCreateFiresAction(t *testing.T) {
	f := newFixture(t)
	f.start(t)

	f.touch(t, "inbox/note-1.md", "# hello\n")
	// OnInboxCreate is coalesced — fires once per debounce window
	// without a specific path (the inbox pipeline walks the whole
	// inbox dir, so the trigger path is irrelevant at this layer).
	_ = f.awaitInbox(t)
}

func TestWatcher_InboxCoalescesBurst(t *testing.T) {
	f := newFixture(t, withDebounce(100*time.Millisecond))
	f.start(t)

	// Five inbox files in quick succession — should collapse to a
	// single OnInboxCreate callback rather than firing five
	// concurrent pipeline runs.
	for i := 0; i < 5; i++ {
		f.touch(t, fmt.Sprintf("inbox/burst-%d.md", i), "# burst\n")
	}
	_ = f.awaitInbox(t)
	// No additional inbox callbacks should follow within the
	// quiescence window — coalescing should have collapsed the rest.
	f.assertNoEvent(t)
}

func TestWatcher_TopicModifyFiresAction(t *testing.T) {
	f := newFixture(t)
	// Topic exists before Start so the subsequent write is a Write,
	// not a Create.
	f.touch(t, "topics/foo.md", "# foo\n")
	f.start(t)

	f.touch(t, "topics/foo.md", "# foo updated\n")
	got := f.awaitModify(t)
	if filepath.Base(got) != "foo.md" {
		t.Fatalf("modify callback path: got %q want foo.md", got)
	}
}

func TestWatcher_DebounceCoalescesBurst(t *testing.T) {
	f := newFixture(t, withDebounce(100*time.Millisecond))
	f.touch(t, "topics/foo.md", "# initial\n")
	f.start(t)

	// Five rapid writes — should collapse to a single callback.
	for i := 0; i < 5; i++ {
		f.touch(t, "topics/foo.md", "# burst\n")
	}
	// First callback should arrive.
	_ = f.awaitModify(t)
	// No further callbacks should follow within the quiescence
	// window — debounce should have collapsed the rest.
	f.assertNoEvent(t)
}

func TestWatcher_StartupScanFiresInboxForPreexistingNotes(t *testing.T) {
	f := newFixture(t)
	// Inbox notes that existed before the daemon came up. The watcher
	// must still hand these off to OnInboxCreate so a daemon restart
	// doesn't leave them stuck — but coalesced to a single fire (the
	// inbox pipeline's walk picks up all the notes in one run).
	f.touch(t, "inbox/pre-1.md", "")
	f.touch(t, "inbox/pre-2.md", "")
	f.start(t)

	_ = f.awaitInbox(t)
	// Single fire only — no second callback within the quiescence
	// window.
	f.assertNoEvent(t)
}

func TestWatcher_IgnoresNonMarkdownFiles(t *testing.T) {
	f := newFixture(t)
	f.start(t)

	f.touch(t, "inbox/note.txt", "not markdown")
	f.touch(t, "topics/log.json", "{}")
	f.assertNoEvent(t)
}

func TestWatcher_IgnoresExcludedDirs(t *testing.T) {
	f := newFixture(t)
	// _meta is excluded; the daemon writes processing logs and marker
	// files there. Watcher must not echo those back as modify events.
	f.start(t)

	f.touch(t, "_meta/notes.md", "# log\n")
	f.assertNoEvent(t)
}

func TestWatcher_RegistersNewSubdirectories(t *testing.T) {
	f := newFixture(t)
	f.start(t)

	// Create a brand-new subdirectory after Start, then write a note
	// inside it. The watcher's create-handler must add the new dir
	// to the watch set before the inner write fires.
	//
	// fsnotify itself does not expose a "registered" signal — the
	// only way to observe registration is to retry the write until
	// the resulting event makes it through. The retry loop below
	// converts the inherent setup race into a bounded retry instead
	// of a fixed sleep.
	newDir := filepath.Join(f.root, "projects")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	deadline := time.Now().Add(eventTimeout)
	notePath := filepath.Join(newDir, "x.md")
	for time.Now().Before(deadline) {
		if err := os.WriteFile(notePath, []byte("# x\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		select {
		case <-f.modify:
			return
		case <-time.After(100 * time.Millisecond):
			// fsnotify hadn't registered the new dir yet on this try;
			// next iteration will write again.
		}
	}
	t.Fatalf("no modify event after %v of retries — new subdir not registered", eventTimeout)
}

func TestWatcher_StopIsIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := f.w.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := f.w.Stop(); err != nil {
		t.Fatalf("first stop: %v", err)
	}
	if err := f.w.Stop(); err != nil {
		t.Fatalf("second stop: %v", err)
	}
}

func TestWatcher_NewRejectsEmptyVaultRoot(t *testing.T) {
	if _, err := watcher.New(watcher.Config{}); err == nil {
		t.Fatalf("expected error for empty VaultRoot")
	}
}

func TestWatcher_DefaultDebounceWhenUnset(t *testing.T) {
	// Config{} with no Debounce field set should fall back to
	// DefaultDebounce. Exercise via behavior — a write should
	// produce exactly one callback within the default window.
	root := t.TempDir()
	for _, d := range []string{"topics"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "topics/d.md"), nil, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got := make(chan string, 8)
	w, err := watcher.New(watcher.Config{
		VaultRoot: root,
		Actions: watcher.Actions{
			OnNoteModify: func(_ context.Context, p string) { got <- p },
		},
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(filepath.Join(root, "topics/d.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	select {
	case <-got:
	case <-time.After(watcher.DefaultDebounce + eventTimeout):
		t.Fatalf("default-debounce write produced no callback")
	}
	// Within one more quiet window, no further callbacks should
	// arrive (one write → one callback).
	select {
	case p := <-got:
		t.Fatalf("unexpected extra callback for %s", p)
	case <-time.After(quiescenceWindow):
	}
}
