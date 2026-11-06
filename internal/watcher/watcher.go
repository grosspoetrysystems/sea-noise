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
// fsnotify quirks worth knowing (see .tempor/KNOWN-DELTA.md for the
// long form):
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

	"sea-noise/internal/vault"
)

// DefaultDebounce is the per-path debounce window. Picked to be long
// enough to collapse a typical editor's save burst (vim writes a
// .swp, then the file, then chmods) but short enough that "drop in
// inbox, see processing start" still feels live.
const DefaultDebounce = 200 * time.Millisecond

// DefaultInboxSettle is the quiet period used to ensure a synced inbox
// file has stopped changing before the pipeline reads it.
const DefaultInboxSettle = 500 * time.Millisecond

// DefaultInboxMaxWait bounds how long a continuously syncing inbox can
// defer the pipeline. The pipeline's own vault reads still apply their
// materialization retry policy after this deadline.
const DefaultInboxMaxWait = 30 * time.Second

type inboxStamp struct {
	size    int64
	modTime time.Time
}

func (w *Watcher) inboxStable(ctx context.Context) bool {
	before, ok := inboxSnapshot(w)
	if !ok {
		return false
	}
	timer := time.NewTimer(w.cfg.InboxSettle)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
	}
	after, ok := inboxSnapshot(w)
	return ok && inboxSnapshotsEqual(before, after)
}

var inboxSnapshot = func(w *Watcher) (map[string]inboxStamp, bool) {
	inbox := filepath.Join(w.cfg.VaultRoot, w.cfg.InboxSubdir)
	entries, err := os.ReadDir(inbox)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]inboxStamp{}, true
	}
	if err != nil {
		return nil, false
	}
	stamps := make(map[string]inboxStamp)
	for _, entry := range entries {
		if entry.IsDir() || !isMarkdown(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, false
		}
		stamps[entry.Name()] = inboxStamp{
			size:    info.Size(),
			modTime: info.ModTime(),
		}
	}
	return stamps, true
}

func inboxSnapshotsEqual(a, b map[string]inboxStamp) bool {
	if len(a) != len(b) {
		return false
	}
	for name, before := range a {
		if after, ok := b[name]; !ok || after != before {
			return false
		}
	}
	return true
}

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

	// OnSourcesChange fires when bridge membership or an external source
	// root is created, replaced, or removed. Callers should re-resolve
	// vernacular.Sources, then call ReloadSources before rebuilding.
	OnSourcesChange func(ctx context.Context)
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
	// InboxSettle is the quiet period used to confirm synced inbox
	// files stopped changing. Zero falls back to DefaultInboxSettle.
	InboxSettle time.Duration
	// Sources are explicit external source mappings. They are watched in
	// addition to VaultRoot and are never considered the vault inbox.
	Sources []vault.Source
	// Actions are the user-supplied callbacks. All optional.
	Actions Actions
}
type sourceSentinel struct {
	parent string
	target string
}

// Watcher is the resident fsnotify-backed observer.
type Watcher struct {
	cfg Config

	mu      sync.Mutex
	fsw     *fsnotify.Watcher
	started bool
	stopped bool
	wg      sync.WaitGroup

	sourceMu        sync.RWMutex
	sources         []vault.Source
	sourceSentinels []sourceSentinel
	watchMu         sync.Mutex
	watched         map[string]struct{}

	timerMu sync.Mutex
	timers  map[string]*time.Timer // per-path debounce (OnNoteModify)
	// inboxTimer coalesces all OnInboxCreate fires within the
	// debounce window into a single pipeline run. Multiple inbox
	// files arriving in quick succession (e.g., iCloud syncing a
	// batch) collapse to one pipeline fire that processes all of
	// them in one walk. Without this, every file produced its own
	// concurrent pipeline run — wasteful and racy on shared vault
	// state like _meta/topic-frequencies.md.
	inboxTimer *time.Timer
	// sourceTimer coalesces membership/target boundary changes.
	sourceTimer *time.Timer
	// inboxSince bounds repeated settle retries caused by a continuously
	// syncing file or a steady stream of cloud events.
	inboxSince time.Time
	// inboxMu serialises actual pipeline execution. Even when the
	// coalescing timer collapses correctly, a slow pipeline + a
	// timer firing during that pipeline could still overlap. The
	// mutex ensures no two OnInboxCreate callbacks run concurrently.
	inboxMu sync.Mutex
}

// New wires a Watcher. Call Start to begin observing.
func New(cfg Config) (*Watcher, error) {
	if cfg.VaultRoot == "" {
		return nil, errors.New("watcher: VaultRoot required")
	}
	abs, err := canonicalPath(cfg.VaultRoot)
	if err != nil {
		return nil, fmt.Errorf("watcher: resolve vault root: %w", err)
	}
	cfg.VaultRoot = abs
	sources, err := normalizeSources(cfg.VaultRoot, cfg.Sources)
	if err != nil {
		return nil, err
	}
	cfg.Sources = nil
	if cfg.InboxSubdir == "" {
		cfg.InboxSubdir = "inbox"
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = DefaultDebounce
	}
	if cfg.InboxSettle <= 0 {
		cfg.InboxSettle = DefaultInboxSettle
	}
	return &Watcher{
		cfg:     cfg,
		sources: append([]vault.Source(nil), sources...),
		timers:  map[string]*time.Timer{},
		watched: map[string]struct{}{},
	}, nil
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
		w.mu.Lock()
		w.started = false
		w.fsw = nil
		w.mu.Unlock()
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
	if w.inboxTimer != nil {
		w.inboxTimer.Stop()
		w.inboxTimer = nil
	}
	if w.sourceTimer != nil {
		w.sourceTimer.Stop()
		w.sourceTimer = nil
	}
	w.inboxSince = time.Time{}
	w.timerMu.Unlock()
	return nil
}

// registerAll walks the vault and each explicitly registered source, adding
// every directory to fsnotify. fsnotify does not recurse, so new directories
// are added by handle on create.
func (w *Watcher) registerAll() error {
	w.watchMu.Lock()
	defer w.watchMu.Unlock()
	w.watched = map[string]struct{}{}
	return w.registerAllLocked()
}

func (w *Watcher) registerAllLocked() error {
	if err := w.registerTreeLocked(w.cfg.VaultRoot); err != nil {
		return err
	}
	for _, source := range w.sourceSnapshot() {
		if err := w.registerTreeLocked(source.PhysicalRoot); err != nil {
			return err
		}
	}
	for _, sentinel := range w.sentinelSnapshot() {
		if err := w.addWatchExactLocked(sentinel.parent); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
	}
	return nil
}

func (w *Watcher) registerTreeLocked(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("watcher: inspect %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("watcher: symlink watch root %s", root)
	}
	if !info.IsDir() {
		if err := w.addWatchLocked(filepath.Dir(root)); err != nil {
			return err
		}
		return w.addWatchLocked(root)
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if isExcluded(d.Name()) && path != root {
			return filepath.SkipDir
		}
		return w.addWatchLocked(path)
	})
}

func (w *Watcher) addWatchLocked(path string) error {
	path, err := canonicalPath(path)
	if err != nil {
		return fmt.Errorf("watcher: resolve watch path: %w", err)
	}
	if _, ok := w.watched[path]; ok {
		return nil
	}
	if err := w.fsw.Add(path); err != nil {
		return fmt.Errorf("watcher: add %s: %w", path, err)
	}
	w.watched[path] = struct{}{}
	return nil
}

func (w *Watcher) addWatchExactLocked(path string) error {
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("watcher: inspect exact watch path %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("watcher: exact watch path %s is not a real directory", path)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("watcher: resolve exact watch path %s: %w", path, err)
	}
	if filepath.Clean(resolved) != path {
		return fmt.Errorf("watcher: exact watch path %s resolves to %s", path, resolved)
	}
	if _, ok := w.watched[path]; ok {
		return nil
	}
	if err := w.fsw.Add(path); err != nil {
		return fmt.Errorf("watcher: add exact watch %s: %w", path, err)
	}
	w.watched[path] = struct{}{}
	return nil
}

// ReloadSources atomically replaces external source mappings and their
// watches. An empty source list removes every external watch and sentinel
// while retaining the vault root.
func (w *Watcher) ReloadSources(sources []vault.Source) error {
	normalized, err := normalizeSources(w.cfg.VaultRoot, sources)
	if err != nil {
		return err
	}
	return w.replaceSourceState(normalized, nil)
}

// SuspendSources disables invalid external mappings while retaining parent
// sentinels for the last validated targets. A later target restoration can
// therefore trigger OnSourcesChange and re-resolution.
func (w *Watcher) SuspendSources() error {
	active := w.sourceSnapshot()
	targets := make([]string, 0, len(active))
	if len(active) > 0 {
		for _, source := range active {
			targets = append(targets, source.PhysicalRoot)
		}
	} else {
		for _, sentinel := range w.sentinelSnapshot() {
			targets = append(targets, sentinel.target)
		}
	}
	return w.replaceSourceState(nil, sourceSentinels(targets))
}

func sourceSentinels(targets []string) []sourceSentinel {
	sentinels := make([]sourceSentinel, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if !filepath.IsAbs(target) || filepath.Clean(target) != target {
			continue
		}
		parent, err := nearestExistingAncestor(filepath.Dir(target))
		if err != nil {
			continue
		}
		sentinel := sourceSentinel{parent: parent, target: target}
		key := sentinel.parent + "\x00" + sentinel.target
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sentinels = append(sentinels, sentinel)
	}
	return sentinels
}

func (w *Watcher) replaceSourceState(normalized []vault.Source, sentinels []sourceSentinel) error {
	w.mu.Lock()
	started, stopped, fsw := w.started, w.stopped, w.fsw
	w.mu.Unlock()
	if !started || stopped || fsw == nil {
		w.sourceMu.Lock()
		w.sources = append([]vault.Source(nil), normalized...)
		w.sourceSentinels = append([]sourceSentinel(nil), sentinels...)
		w.sourceMu.Unlock()
		return nil
	}

	w.watchMu.Lock()
	defer w.watchMu.Unlock()
	old := w.sourceSnapshot()
	oldSentinels := w.sentinelSnapshot()
	for path := range w.watched {
		_ = fsw.Remove(path)
	}
	w.watched = map[string]struct{}{}
	w.sourceMu.Lock()
	w.sources = append([]vault.Source(nil), normalized...)
	w.sourceSentinels = append([]sourceSentinel(nil), sentinels...)
	w.sourceMu.Unlock()
	if err := w.registerAllLocked(); err != nil {
		for path := range w.watched {
			_ = fsw.Remove(path)
		}
		w.watched = map[string]struct{}{}
		w.sourceMu.Lock()
		w.sources = old
		w.sourceSentinels = oldSentinels
		w.sourceMu.Unlock()
		_ = w.registerAllLocked()
		return err
	}
	return nil
}

func (w *Watcher) sourceSnapshot() []vault.Source {
	w.sourceMu.RLock()
	defer w.sourceMu.RUnlock()
	return append([]vault.Source(nil), w.sources...)
}

func (w *Watcher) sentinelSnapshot() []sourceSentinel {
	w.sourceMu.RLock()
	defer w.sourceMu.RUnlock()
	return append([]sourceSentinel(nil), w.sourceSentinels...)
}

// LogicalPath maps an absolute watcher event path to its stable logical ID.
// It returns false for paths outside the vault and registered external roots.
func (w *Watcher) LogicalPath(abs string) (string, bool) {
	logical, _, ok := w.mapPath(abs)
	return logical, ok
}

func (w *Watcher) mapPath(abs string) (string, bool, bool) {
	clean, err := canonicalEventPath(abs)
	if err != nil {
		return "", false, false
	}
	w.sourceMu.RLock()
	defer w.sourceMu.RUnlock()
	for _, source := range w.sources {
		physical := source.PhysicalRoot
		if clean == physical {
			return filepath.ToSlash(source.LogicalRoot), true, true
		}
		if !pathWithin(physical, clean) {
			continue
		}
		rel, err := filepath.Rel(physical, clean)
		if err != nil {
			continue
		}
		return filepath.ToSlash(filepath.Join(source.LogicalRoot, filepath.FromSlash(rel))), true, true
	}
	rel, err := filepath.Rel(w.cfg.VaultRoot, clean)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false, false
	}
	if rel == "." {
		return "", false, true
	}
	return filepath.ToSlash(rel), false, true
}

func (w *Watcher) isExternalPath(path string) bool {
	_, external, ok := w.mapPath(path)
	return ok && external
}
func (w *Watcher) isSourceSentinelPath(path string) bool {
	abs, err := canonicalEventPath(path)
	if err != nil {
		return false
	}
	w.sourceMu.RLock()
	defer w.sourceMu.RUnlock()
	for _, sentinel := range w.sourceSentinels {
		if pathWithin(abs, sentinel.target) || pathWithin(sentinel.target, abs) {
			return true
		}
	}
	return false
}

func normalizeSources(vaultRoot string, sources []vault.Source) ([]vault.Source, error) {
	out := make([]vault.Source, 0, len(sources))
	seenLogical := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		rawLogical := filepath.FromSlash(source.LogicalRoot)
		if filepath.IsAbs(rawLogical) || strings.HasPrefix(source.LogicalRoot, "/") {
			return nil, fmt.Errorf("watcher: invalid source logical root %q", source.LogicalRoot)
		}
		logical := filepath.ToSlash(strings.Trim(source.LogicalRoot, "/"))
		if logical == "" || logical == "." || strings.HasPrefix(logical, "../") || strings.Contains(logical, "/../") || strings.Contains(logical, "\\") {
			return nil, fmt.Errorf("watcher: invalid source logical root %q", source.LogicalRoot)
		}
		if _, ok := seenLogical[logical]; ok {
			return nil, fmt.Errorf("watcher: duplicate source logical root %q", logical)
		}
		seenLogical[logical] = struct{}{}
		physical, err := canonicalPath(source.PhysicalRoot)
		if err != nil {
			return nil, fmt.Errorf("watcher: resolve source %s: %w", source.LogicalRoot, err)
		}
		info, err := os.Stat(physical)
		if err != nil {
			return nil, fmt.Errorf("watcher: source %s: %w", source.LogicalRoot, err)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("watcher: source %s is not a file or directory", source.LogicalRoot)
		}
		if pathWithin(vaultRoot, physical) || pathWithin(physical, vaultRoot) {
			return nil, fmt.Errorf("watcher: source %s overlaps vault", source.LogicalRoot)
		}
		out = append(out, vault.Source{LogicalRoot: logical, PhysicalRoot: physical})
	}
	return out, nil
}

func canonicalPath(name string) (string, error) {
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved), nil
	} else {
		lastErr := err
		current := abs
		missing := make([]string, 0, 2)
		for {
			if _, statErr := os.Lstat(current); statErr == nil {
				resolved, resolveErr := filepath.EvalSymlinks(current)
				if resolveErr != nil {
					return "", resolveErr
				}
				for i := len(missing) - 1; i >= 0; i-- {
					resolved = filepath.Join(resolved, missing[i])
				}
				return filepath.Clean(resolved), nil
			} else if !errors.Is(statErr, fs.ErrNotExist) {
				return "", statErr
			}
			parent := filepath.Dir(current)
			if parent == current {
				return "", lastErr
			}
			missing = append(missing, filepath.Base(current))
			current = parent
		}
	}
}

func canonicalEventPath(name string) (string, error) {
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	if info, err := os.Lstat(abs); err == nil && info.Mode()&os.ModeSymlink != 0 {
		parent, err := canonicalPath(filepath.Dir(abs))
		if err != nil {
			return "", err
		}
		return filepath.Join(parent, filepath.Base(abs)), nil
	}
	return canonicalPath(abs)
}

func nearestExistingAncestor(name string) (string, error) {
	current := filepath.Clean(name)
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
				resolved, resolveErr := filepath.EvalSymlinks(current)
				if resolveErr == nil && filepath.Clean(resolved) == current {
					return current, nil
				}
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("watcher: no safe source sentinel ancestor for %s", name)
		}
		current = parent
	}
}

func pathWithin(base, candidate string) bool {
	rel, err := filepath.Rel(base, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// scanInbox fires OnInboxCreate ONCE if the inbox has any .md notes
// at startup. Notes the user dropped while the daemon was offline
// get processed in a single pipeline run rather than N concurrent
// runs. Uses the coalescing schedule path so behaviour matches
// runtime inbox events.
func (w *Watcher) scanInbox(ctx context.Context) {
	if w.cfg.Actions.OnInboxCreate == nil {
		return
	}
	inbox := filepath.Join(w.cfg.VaultRoot, w.cfg.InboxSubdir)
	entries, err := os.ReadDir(inbox)
	if err != nil {
		return // No inbox dir is fine — nothing to scan.
	}
	hasMd := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			hasMd = true
			break
		}
	}
	if hasMd {
		w.scheduleInbox(ctx)
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
	if canonical, err := canonicalEventPath(ev.Name); err == nil {
		ev.Name = canonical
	}
	sentinelPath := w.isSourceSentinelPath(ev.Name)
	if w.isSourceControlPath(ev.Name) && (ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write) || ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename)) {
		w.scheduleSources(ctx)
	}
	if w.isExternalPath(ev.Name) && (ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename)) {
		w.scheduleSources(ctx)
	}
	if sentinelPath && (ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write) || ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename)) {
		w.scheduleSources(ctx)
	}

	// Newly created directories must be added to the watcher so their
	// children become observable. This applies to vault and explicit
	// external roots alike.
	if ev.Has(fsnotify.Create) {
		info, err := os.Lstat(ev.Name)
		if err == nil && info.IsDir() {
			if sentinelPath {
				w.watchMu.Lock()
				_ = w.addWatchExactLocked(ev.Name)
				w.watchMu.Unlock()
				return
			}
			if _, _, indexable := w.mapPath(ev.Name); indexable && !isExcluded(filepath.Base(ev.Name)) {
				w.watchMu.Lock()
				_ = w.registerTreeLocked(ev.Name)
				w.watchMu.Unlock()
				w.schedule(ctx, ev.Name, w.cfg.Actions.OnNoteModify)
			}
			return
		}
	}
	if sentinelPath {
		return
	}

	if !isMarkdown(ev.Name) {
		return
	}

	// Only the physical vault inbox can trigger inbox processing. An
	// external project directory named "inbox" is always a note modify.
	if w.isInbox(ev.Name) && (ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write)) {
		w.scheduleInbox(ctx)
		return
	}

	if ev.Has(fsnotify.Write) || ev.Has(fsnotify.Create) {
		w.schedule(ctx, ev.Name, w.cfg.Actions.OnNoteModify)
	}
}

// scheduleInbox debounces all OnInboxCreate fires through a single
// shared timer. Resets the timer on each call; when it fires, runs
// the callback under inboxMu so no concurrent pipeline executions
// occur. The path argument is dropped intentionally — the inbox
// pipeline walks the full inbox dir.
func (w *Watcher) scheduleInbox(ctx context.Context) {
	cb := w.cfg.Actions.OnInboxCreate
	if cb == nil || ctx.Err() != nil || w.isStopped() {
		return
	}
	w.timerMu.Lock()
	if w.inboxTimer != nil {
		w.inboxTimer.Stop()
	}
	if w.inboxSince.IsZero() {
		w.inboxSince = time.Now()
	}
	delay := w.cfg.Debounce
	if remaining := DefaultInboxMaxWait - time.Since(w.inboxSince); remaining < delay {
		if remaining > 0 {
			delay = remaining
		} else {
			delay = 0
		}
	}
	w.inboxTimer = time.AfterFunc(delay, func() {
		w.timerMu.Lock()
		w.inboxTimer = nil
		since := w.inboxSince
		w.timerMu.Unlock()
		if ctx.Err() != nil || w.isStopped() {
			return
		}
		// Wait for the complete inbox snapshot to remain unchanged
		// across a settle window. A synced file can appear before its
		// contents finish materializing.
		if !w.inboxStable(ctx) {
			if ctx.Err() == nil && !w.isStopped() &&
				time.Since(since) < DefaultInboxMaxWait {
				w.scheduleInbox(ctx)
				return
			}
			// At the deadline, let the pipeline's vault reads apply
			// their own materialization retry policy.
		}
		w.timerMu.Lock()
		w.inboxSince = time.Time{}
		w.timerMu.Unlock()
		if ctx.Err() != nil || w.isStopped() {
			return
		}
		// Serialise: another scheduled fire that races past the
		// debounce window still waits for this one to finish.
		w.inboxMu.Lock()
		defer w.inboxMu.Unlock()
		cb(ctx, "") // path is meaningless at coalesce time; pipeline walks the dir
	})
	w.timerMu.Unlock()
}

// scheduleSources debounces bridge membership and target-boundary changes
// into one callback so an atomic metadata replacement cannot race several
// source reloads.
func (w *Watcher) scheduleSources(ctx context.Context) {
	cb := w.cfg.Actions.OnSourcesChange
	if cb == nil || ctx.Err() != nil || w.isStopped() {
		return
	}
	w.timerMu.Lock()
	if w.sourceTimer != nil {
		w.sourceTimer.Stop()
	}
	w.sourceTimer = time.AfterFunc(w.cfg.Debounce, func() {
		w.timerMu.Lock()
		w.sourceTimer = nil
		w.timerMu.Unlock()
		if ctx.Err() != nil || w.isStopped() {
			return
		}
		cb(ctx)
	})
	w.timerMu.Unlock()
}

func (w *Watcher) isSourceControlPath(name string) bool {
	abs, err := canonicalEventPath(name)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(w.cfg.VaultRoot, abs)
	if err != nil {
		return false
	}
	return rel == "vernacular" || strings.HasPrefix(rel, "vernacular"+string(filepath.Separator))
}

func (w *Watcher) isStopped() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stopped
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
	clean, err := canonicalEventPath(path)
	if err != nil {
		return false
	}
	inbox := filepath.Join(w.cfg.VaultRoot, w.cfg.InboxSubdir)
	dir := filepath.Dir(clean)
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
