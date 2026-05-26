// Package daemon is the resident-process runtime for Sea Noise.
//
// The daemon owns three responsibilities:
//
//   - Scheduling — fire the daily and weekly pipeline chains on Go
//     timers (not cron) so the macOS TCC / launchd churn that plagues
//     the Python prototype never applies. See DAEMON.md for the
//     load-bearing rationale.
//   - Pipeline execution — run a list of named Steps in order, log
//     each step's start, duration, and exit status, and write a
//     marker file after a successful run so re-fires within the same
//     period skip.
//   - Developer ergonomics — the same in-process Step list is
//     reachable from the one-shot `sea-noise run inbox|daily|weekly`
//     CLI path for debugging.
//
// Subsystems (Unix-socket JSON-RPC IPC, fsnotify vault watcher) are
// separate tracks; this package exposes the seams (Server.Trigger,
// Server.Notify) so they can plug in without modifying the core.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Pipeline names. The scheduler fires them on a Go timer; callers
// can also invoke them directly via Server.RunPipeline.
const (
	PipelineDaily  = "daily"
	PipelineWeekly = "weekly"
	PipelineInbox  = "inbox"
)

// Marker file paths under the vault. Mirrors run-daily-catchup.sh +
// run-weekly-catchup.sh marker behavior.
const (
	dailyMarkerRel  = "_meta/.daily-last-run"
	weeklyMarkerRel = "_meta/.weekly-last-run"
	logsDirRel      = "_meta/processing-logs"
)

// Sentinel errors. Wrap with %w when surfacing.
var (
	// ErrSkippedAlreadyRan is returned by RunPipeline when the marker
	// file shows the pipeline already ran in the current period.
	ErrSkippedAlreadyRan = errors.New("daemon: pipeline already ran in current period")
	// ErrUnknownPipeline is returned when RunPipeline gets a name it
	// doesn't recognize.
	ErrUnknownPipeline = errors.New("daemon: unknown pipeline")
)

// StepFn is the executable body of a Step. Receives a context for
// cancellation/timeout propagation; returns nil on success or an
// error that gets recorded in the processing log.
type StepFn func(ctx context.Context) error

// Step is one item in a pipeline chain.
type Step struct {
	// Name is the prototype's step name (e.g. "retry-crawls",
	// "summarize", "index"). Used in marker files, processing logs,
	// and pipeline-health entries.
	Name string
	// Fn is the executable body. A nil Fn means the step is named but
	// not yet ported to Go; the daemon logs it as deferred and moves
	// on. This keeps the daily/weekly step lists byte-identical to
	// the prototype's catchup scripts even when some steps haven't
	// landed yet.
	Fn StepFn
}

// Registry pairs the daily and weekly step lists. Built by the
// caller (typically cmd/sea-noise) so internal/daemon stays free of
// imports against the workflow packages.
type Registry struct {
	Daily  []Step
	Weekly []Step
	Inbox  []Step
}

// Config captures user-tunable daemon behaviour. Defaults match the
// prototype's catchup scripts: daily fires at 00:05 UTC, weekly on
// Sunday at 01:00 UTC.
type Config struct {
	VaultRoot string
	// DailyAt is the time-of-day to fire the daily pipeline. Only the
	// hour and minute fields are read.
	DailyAt time.Time
	// WeeklyDay is the weekday the weekly pipeline fires on.
	WeeklyDay time.Weekday
	// WeeklyAt is the time-of-day to fire the weekly pipeline.
	WeeklyAt time.Time
	// DryRun causes the daemon to log every Step it *would* run
	// without invoking the StepFn. Marker files are NOT written under
	// dry-run.
	DryRun bool
}

// DefaultConfig returns the baseline config matching the prototype's
// catchup scripts. The caller still needs to fill in VaultRoot.
func DefaultConfig() Config {
	return Config{
		DailyAt:   time.Date(0, 1, 1, 0, 5, 0, 0, time.UTC),
		WeeklyDay: time.Sunday,
		WeeklyAt:  time.Date(0, 1, 1, 1, 0, 0, 0, time.UTC),
	}
}

// Clock is the time source the daemon uses. Injectable so tests can
// pin "now" without sleeping for real intervals.
type Clock interface {
	Now() time.Time
}

// realClock returns wall-clock time. Production default.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// Server is a resident daemon instance.
type Server struct {
	cfg      Config
	registry Registry
	clock    Clock

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// New wires a Server with the supplied config + step registry.
// Override the clock with WithClock before Start for tests.
func New(cfg Config, reg Registry) *Server {
	return &Server{
		cfg:      cfg,
		registry: reg,
		clock:    realClock{},
	}
}

// WithClock replaces the time source. Tests use this to pin Now()
// without sleeping for real intervals.
func (s *Server) WithClock(c Clock) *Server {
	s.clock = c
	return s
}

// Start launches the daemon. It returns nil after the scheduler is
// running; the daemon stays up until ctx is cancelled or Stop is
// called.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("daemon: already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.started = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.scheduleLoop(runCtx)
	return nil
}

// Stop signals the scheduler to drain and waits for in-flight steps
// to finish. Idempotent.
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.cancel()
	s.started = false
	s.mu.Unlock()
	s.wg.Wait()
	return nil
}

// scheduleLoop ticks every minute and fires daily/weekly pipelines
// when the clock crosses the configured boundary. One-minute
// granularity is enough — neither pipeline cares about sub-minute
// timing.
func (s *Server) scheduleLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := s.clock.Now()
			s.maybeFire(ctx, now)
		}
	}
}

// maybeFire runs each pipeline whose schedule window includes now,
// gated by the marker file.
func (s *Server) maybeFire(ctx context.Context, now time.Time) {
	if shouldFireDaily(now, s.cfg.DailyAt) {
		_ = s.RunPipeline(ctx, PipelineDaily, now)
	}
	if shouldFireWeekly(now, s.cfg.WeeklyDay, s.cfg.WeeklyAt) {
		_ = s.RunPipeline(ctx, PipelineWeekly, now)
	}
}

func shouldFireDaily(now, dailyAt time.Time) bool {
	return now.Hour() == dailyAt.Hour() && now.Minute() == dailyAt.Minute()
}

func shouldFireWeekly(now time.Time, dow time.Weekday, weeklyAt time.Time) bool {
	return now.Weekday() == dow && now.Hour() == weeklyAt.Hour() && now.Minute() == weeklyAt.Minute()
}

// RunPipeline executes the named pipeline once, honoring the marker
// file gate. Returns ErrSkippedAlreadyRan when the marker shows the
// period has already had a run. Returns ErrUnknownPipeline if name
// isn't registered.
func (s *Server) RunPipeline(ctx context.Context, name string, now time.Time) error {
	steps, marker, err := s.lookupPipeline(name)
	if err != nil {
		return err
	}
	if marker != "" && s.markerActive(name, marker, now) {
		return fmt.Errorf("%w: %s", ErrSkippedAlreadyRan, name)
	}

	logPath, err := s.openProcessingLog(now)
	if err != nil {
		return err
	}
	defer logPath.Close()

	fmt.Fprintf(logPath, "\n## %s %s\n\n", name, now.Format(time.RFC3339))
	if s.cfg.DryRun {
		for _, st := range steps {
			fmt.Fprintf(logPath, "- [DRY-RUN] %s — would run\n", st.Name)
		}
		return nil
	}

	for _, st := range steps {
		start := s.clock.Now()
		switch {
		case st.Fn == nil:
			fmt.Fprintf(logPath, "- %s — DEFERRED (no Go port yet)\n", st.Name)
		default:
			err := st.Fn(ctx)
			elapsed := s.clock.Now().Sub(start)
			status := "OK"
			if err != nil {
				status = fmt.Sprintf("FAILED (%v)", err)
			}
			fmt.Fprintf(logPath, "- %s — %s — %s (%s)\n", st.Name, start.Format("15:04:05"), elapsed.Round(time.Millisecond), status)
		}
	}

	if marker != "" {
		if err := s.writeMarker(marker, now); err != nil {
			return err
		}
	}
	return nil
}

// lookupPipeline returns the step list + marker-file path for name.
// Inbox has no marker (it's event-fired, not period-gated).
func (s *Server) lookupPipeline(name string) ([]Step, string, error) {
	switch name {
	case PipelineDaily:
		return s.registry.Daily, dailyMarkerRel, nil
	case PipelineWeekly:
		return s.registry.Weekly, weeklyMarkerRel, nil
	case PipelineInbox:
		return s.registry.Inbox, "", nil
	default:
		return nil, "", fmt.Errorf("%w: %q", ErrUnknownPipeline, name)
	}
}

// markerActive returns true when the marker file's recorded run is
// still valid for the current period (same day for daily, same ISO
// week for weekly).
func (s *Server) markerActive(name, markerRel string, now time.Time) bool {
	raw, err := os.ReadFile(filepath.Join(s.cfg.VaultRoot, markerRel))
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil {
		return false
	}
	previous, err := time.Parse("2006-01-02", string(bytesTrim(raw)))
	if err != nil {
		return false
	}
	switch name {
	case PipelineDaily:
		return sameDay(previous, now)
	case PipelineWeekly:
		return sameISOWeek(previous, now)
	}
	return false
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func sameISOWeek(a, b time.Time) bool {
	ya, wa := a.ISOWeek()
	yb, wb := b.ISOWeek()
	return ya == yb && wa == wb
}

func (s *Server) writeMarker(markerRel string, now time.Time) error {
	path := filepath.Join(s.cfg.VaultRoot, markerRel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("daemon: mkdir marker dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(now.Format("2006-01-02")+"\n"), 0o644); err != nil {
		return fmt.Errorf("daemon: write marker: %w", err)
	}
	return nil
}

// openProcessingLog returns a writer pointed at the day's log file
// in append mode, creating the parent dir if needed.
func (s *Server) openProcessingLog(now time.Time) (*os.File, error) {
	dir := filepath.Join(s.cfg.VaultRoot, logsDirRel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("daemon: mkdir log dir: %w", err)
	}
	path := filepath.Join(dir, now.Format("2006-01-02")+".md")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("daemon: open log: %w", err)
	}
	return f, nil
}

func bytesTrim(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t' || b[0] == '\n' || b[0] == '\r') {
		b = b[1:]
	}
	for len(b) > 0 {
		last := b[len(b)-1]
		if last == ' ' || last == '\t' || last == '\n' || last == '\r' {
			b = b[:len(b)-1]
			continue
		}
		break
	}
	return b
}
