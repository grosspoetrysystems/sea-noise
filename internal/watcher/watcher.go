// Package watcher is the fsnotify-backed vault watcher. It turns
// filesystem events on the markdown vault into typed actions:
//
//   - A new file under inbox/ fires the inbox pipeline.
//   - Any markdown file modified (create or write) under the vault
//     fires a per-note re-index callback.
//
// The watcher is one of three subsystems the resident daemon owns
// (the other two are the Go-timer scheduler and the Unix-socket
// JSON-RPC IPC surface). It is built around the existing
// daemon.Server: callers wire actions to in-process workflow
// packages without watcher needing to know what those workflows do.
//
// Why fsnotify and not polling: the user-visible feature is "drop a
// note in inbox/, see it processed within seconds." Polling at that
// cadence wastes battery on a personal machine; fsnotify uses kqueue
// on Darwin and inotify on Linux and only wakes when something
// actually changes.
//
// fsnotify quirks worth knowing (see KNOWN-DELTA.md for the long
// form):
//
//   - Atomic-rename editors (vim, Obsidian's mobile sync) fire
//     delete + create rather than write. We treat create-of-existing
//     -note as a modify so callers don't have to special-case it.
//   - macOS coalesces rapid writes into a single event but
//     occasionally emits 2–3 events for one user action; the
//     debounce window collapses these.
//   - Recursive watching is not supported by the underlying APIs.
//     The watcher walks the vault on Start to register every
//     directory and adds new dirs on create events.
package watcher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDebounce is the per-path debounce window. Picked to be long
// enough to collapse a typical editor's save burst (vim writes a
// .swp, then the file, then chmods) but short enough that "drop in
// inbox/, see processing start" still feels live.
const DefaultDebounce = 200 * time.Millisecond

// ExcludeDirs are directory names skipped during the initial walk
// and never registered with fsnotify. _meta is the daemon's own
// log/marker dump — re-indexing on those churns the watcher.
var ExcludeDirs = []string{".git", ".obsidian", ".trash", "_meta", "node_modules"}

// Actions is the set of side-effects the watcher invokes. Both
// callbacks are optional — a nil callback turns the corresponding
// event class into a no-op (handy when a test only wants to assert
// one channel).
type Actions struct {
	// OnInboxCreate fires when a new .md file lands under <root>/inbox/.
	// The path argument is absolute. Implementations typically call
	// daemon.Server.RunPipeline(ctx, daemon.PipelineInbox, time.Now()).
	OnInboxCreate func(ctx context.Context, path string)

	// OnNoteModify fires when an existing .md file under the vault is
	// written (or atomically replaced). The path argument is absolute.
	// Implementations typically schedule a per-note re-index.
	OnNoteModify func(ctx context.Context, path string)
}

// Config tunes watcher behavior.
type Config struct {
	// VaultRoot is the absolute path to the vault.
	VaultRoot string
	// InboxSubdir is the path under VaultRoot considered "inbox" for
	// the OnInboxCreate callback. Defaults to "inbox".
	InboxSubdir string
	// Debounce is the per-path event coalescing window. Zero falls
	// back to DefaultDebounce.
	Debounce time.Duration
	// Actions are the user-supplied callbacks. Both optional.
	Actions Actions
}

// Watcher is the resident fsnotify-backed observer.
type Watcher struct {
	cfg Config

	mu      sync.Mutex
	fsw     *fsnotify.Watcher
	started bool
	stopped bool
	wg      sync.WaitGroup

	timerMu sync.Mutex
	timers  map[string]*time.Timer // per-path debounce
}

// New wires a Watcher. Call Start to begin observing.
func New(cfg Config) (*Watcher, error) {
	if cfg.VaultRoot == "" {
		return nil, errors.New("watcher: VaultRoot required")
	}
	abs, err := filepath.Abs(cfg.VaultRoot)
	if err != nil {
		return nil, fmt.Errorf("watcher: resolve vault root: %w", err)
	}
	cfg.VaultRoot = abs
	if cfg.InboxSubdir == "" {
		cfg.InboxSubdir = "inbox"
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = DefaultDebounce
	}
	return &Watcher{cfg: cfg, timers: map[string]*time.Timer{}}, nil
}

// Start registers every directory under the vault, performs a
// startup scan of the inbox (so notes that landed while the daemon
// was off are processed), and begins the event loop. Start returns
// once the initial registration completes; the event loop runs in
// the background until Stop is called or ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return errors.New("watcher: already started")
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.mu.Unlock()
		return fmt.Errorf("watcher: new fsnotify: %w", err)
	}
	w.fsw = fsw
	w.started = true
	w.mu.Unlock()

	if err := w.registerAll(); err != nil {
		_ = fsw.Close()
		return err
	}

	// Startup inbox scan — anything already sitting in inbox/ counts
	// as "new" since the daemon last ran. The marker file gate on
	// the inbox pipeline is the prototype's catch-up convention; we
	// just fire the callback and let downstream code decide.
	w.scanInbox(ctx)

	w.wg.Add(1)
	go w.eventLoop(ctx)
	return nil
}

// Stop closes the underlying fsnotify watcher and waits for the
// event loop to drain. Idempotent.
func (w *Watcher) Stop() error {
	w.mu.Lock()
	if !w.started || w.stopped {
		w.mu.Unlock()
		return nil
	}
	w.stopped = true
	fsw := w.fsw
	w.mu.Unlock()

	if fsw != nil {
		_ = fsw.Close()
	}
	w.wg.Wait()

	// Drain any pending debounce timers so callbacks don't fire
	// after Stop returns.
	w.timerMu.Lock()
	for k, t := range w.timers {
		t.Stop()
		delete(w.timers, k)
	}
	w.timerMu.Unlock()
	return nil
}

// registerAll walks the vault and adds every directory (minus
// ExcludeDirs) to the fsnotify watcher. fsnotify does not recurse —
// we must register every subdir explicitly and add new ones on
// create.
func (w *Watcher) registerAll() error {
	return filepath.WalkDir(w.cfg.VaultRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if isExcluded(d.Name()) && path != w.cfg.VaultRoot {
			return filepath.SkipDir
		}
		if err := w.fsw.Add(path); err != nil {
			return fmt.Errorf("watcher: add %s: %w", path, err)
		}
		return nil
	})
}

// scanInbox fires OnInboxCreate for every .md already in inbox/. Run
// once at Start so notes the user dropped while the daemon was
// offline get processed.
func (w *Watcher) scanInbox(ctx context.Context) {
	if w.cfg.Actions.OnInboxCreate == nil {
		return
	}
	inbox := filepath.Join(w.cfg.VaultRoot, w.cfg.InboxSubdir)
	entries, err := os.ReadDir(inbox)
	if err != nil {
		return // No inbox dir is fine — nothing to scan.
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		w.cfg.Actions.OnInboxCreate(ctx, filepath.Join(inbox, e.Name()))
	}
}

// eventLoop is the main fsnotify pump. It classifies events and
// schedules debounced callbacks.
func (w *Watcher) eventLoop(ctx context.Context) {
	defer w.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handle(ctx, ev)
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			// fsnotify error stream is informational on most platforms;
			// keep running so a transient kqueue burp doesn't kill the
			// daemon. The next event will resync state.
		}
	}
}

// handle classifies one fsnotify event and either fires a callback
// (after debouncing) or registers a new directory.
func (w *Watcher) handle(ctx context.Context, ev fsnotify.Event) {
	// Newly created directories must be added to the watcher so
	// their children become observable. This is the recursive-watch
	// substitute fsnotify expects callers to manage themselves.
	if ev.Has(fsnotify.Create) {
		info, err := os.Stat(ev.Name)
		if err == nil && info.IsDir() {
			if !isExcluded(filepath.Base(ev.Name)) {
				_ = w.fsw.Add(ev.Name)
			}
			return
		}
	}

	if !isMarkdown(ev.Name) {
		return
	}

	switch {
	case ev.Has(fsnotify.Create) && w.isInbox(ev.Name):
		w.schedule(ctx, ev.Name, w.cfg.Actions.OnInboxCreate)
	case ev.Has(fsnotify.Write), ev.Has(fsnotify.Create):
		// Treat any markdown create-or-write that didn't match the
		// inbox case as a modify. Atomic-rename editors emit
		// create-of-existing-name for an in-place save.
		w.schedule(ctx, ev.Name, w.cfg.Actions.OnNoteModify)
	}
}

// schedule fires cb(ctx, path) after the debounce window, collapsing
// any earlier pending fire for the same path.
func (w *Watcher) schedule(ctx context.Context, path string, cb func(context.Context, string)) {
	if cb == nil {
		return
	}
	w.timerMu.Lock()
	if t, ok := w.timers[path]; ok {
		t.Stop()
	}
	w.timers[path] = time.AfterFunc(w.cfg.Debounce, func() {
		w.timerMu.Lock()
		delete(w.timers, path)
		w.timerMu.Unlock()
		cb(ctx, path)
	})
	w.timerMu.Unlock()
}

// isInbox reports whether path is a direct child of the inbox dir.
func (w *Watcher) isInbox(path string) bool {
	inbox := filepath.Join(w.cfg.VaultRoot, w.cfg.InboxSubdir)
	dir := filepath.Dir(path)
	return dir == inbox
}

func isMarkdown(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".md")
}

func isExcluded(name string) bool {
	for _, x := range ExcludeDirs {
		if name == x {
			return true
		}
	}
	return false
}
