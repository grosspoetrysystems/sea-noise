package daemon_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sea-noise/internal/daemon"
)

func TestRunPipeline_ExecutesStepsInOrder(t *testing.T) {
	vault := t.TempDir()
	var order []string
	reg := daemon.Registry{Daily: []daemon.Step{
		{Name: "a", Fn: func(ctx context.Context) error { order = append(order, "a"); return nil }},
		{Name: "b", Fn: func(ctx context.Context) error { order = append(order, "b"); return nil }},
		{Name: "c", Fn: func(ctx context.Context) error { order = append(order, "c"); return nil }},
	}}
	srv := daemon.New(daemon.Config{VaultRoot: vault}, reg)
	if err := srv.RunPipeline(context.Background(), daemon.PipelineDaily, fixedNow()); err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	want := []string{"a", "b", "c"}
	if len(order) != 3 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestRunPipeline_MarkerGatesSecondInvocation(t *testing.T) {
	vault := t.TempDir()
	var calls int32
	reg := daemon.Registry{Daily: []daemon.Step{
		{Name: "noop", Fn: func(ctx context.Context) error { atomic.AddInt32(&calls, 1); return nil }},
	}}
	srv := daemon.New(daemon.Config{VaultRoot: vault}, reg)
	now := fixedNow()

	if err := srv.RunPipeline(context.Background(), daemon.PipelineDaily, now); err != nil {
		t.Fatalf("first RunPipeline: %v", err)
	}
	if err := srv.RunPipeline(context.Background(), daemon.PipelineDaily, now); !errors.Is(err, daemon.ErrSkippedAlreadyRan) {
		t.Errorf("second RunPipeline err = %v, want ErrSkippedAlreadyRan", err)
	}
	if c := atomic.LoadInt32(&calls); c != 1 {
		t.Errorf("step ran %d times, want 1", c)
	}

	// Crossing the day boundary unblocks the gate.
	nextDay := now.Add(24 * time.Hour)
	if err := srv.RunPipeline(context.Background(), daemon.PipelineDaily, nextDay); err != nil {
		t.Errorf("next-day RunPipeline err = %v, want nil", err)
	}
	if c := atomic.LoadInt32(&calls); c != 2 {
		t.Errorf("step ran %d times across two days, want 2", c)
	}
}

func TestRunPipeline_WeeklyMarkerGatesWithinSameISOWeek(t *testing.T) {
	vault := t.TempDir()
	var calls int32
	reg := daemon.Registry{Weekly: []daemon.Step{
		{Name: "noop", Fn: func(ctx context.Context) error { atomic.AddInt32(&calls, 1); return nil }},
	}}
	srv := daemon.New(daemon.Config{VaultRoot: vault}, reg)
	// 2026-05-26 is a Tuesday, ISO week 22.
	tuesday := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	if err := srv.RunPipeline(context.Background(), daemon.PipelineWeekly, tuesday); err != nil {
		t.Fatalf("first RunPipeline: %v", err)
	}
	// Wednesday same week — still gated.
	wednesday := tuesday.Add(24 * time.Hour)
	if err := srv.RunPipeline(context.Background(), daemon.PipelineWeekly, wednesday); !errors.Is(err, daemon.ErrSkippedAlreadyRan) {
		t.Errorf("mid-week RunPipeline err = %v, want ErrSkippedAlreadyRan", err)
	}
	// Next Monday — new ISO week, unblocked.
	nextMonday := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := srv.RunPipeline(context.Background(), daemon.PipelineWeekly, nextMonday); err != nil {
		t.Errorf("next-week RunPipeline err = %v", err)
	}
	if c := atomic.LoadInt32(&calls); c != 2 {
		t.Errorf("step ran %d times, want 2 (one per ISO week)", c)
	}
}

func TestRunPipeline_DryRunSkipsStepFns(t *testing.T) {
	vault := t.TempDir()
	var calls int32
	reg := daemon.Registry{Daily: []daemon.Step{
		{Name: "noop", Fn: func(ctx context.Context) error { atomic.AddInt32(&calls, 1); return nil }},
	}}
	srv := daemon.New(daemon.Config{VaultRoot: vault, DryRun: true}, reg)
	if err := srv.RunPipeline(context.Background(), daemon.PipelineDaily, fixedNow()); err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	if c := atomic.LoadInt32(&calls); c != 0 {
		t.Errorf("step ran %d times under DryRun, want 0", c)
	}
	if _, err := os.Stat(filepath.Join(vault, "_meta/.daily-last-run")); err == nil {
		t.Errorf("marker file written under DryRun")
	}
	log := readDayLog(t, vault, fixedNow())
	if !strings.Contains(log, "[DRY-RUN] noop") {
		t.Errorf("dry-run log missing entry:\n%s", log)
	}
}

func TestRunPipeline_DeferredStepLogsAndWritesMarker(t *testing.T) {
	vault := t.TempDir()
	reg := daemon.Registry{Daily: []daemon.Step{
		{Name: "shipped", Fn: func(ctx context.Context) error { return nil }},
		{Name: "not-yet-ported", Fn: nil},
	}}
	srv := daemon.New(daemon.Config{VaultRoot: vault}, reg)
	if err := srv.RunPipeline(context.Background(), daemon.PipelineDaily, fixedNow()); err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "_meta/.daily-last-run")); err != nil {
		t.Fatalf("marker missing after deferred step: %v", err)
	}
	log := readDayLog(t, vault, fixedNow())
	if !strings.Contains(log, "not-yet-ported — DEFERRED") {
		t.Errorf("missing deferred line:\n%s", log)
	}
}

func TestRunPipeline_FailedStepDoesNotWriteMarker(t *testing.T) {
	vault := t.TempDir()
	reg := daemon.Registry{Daily: []daemon.Step{
		{Name: "boom", Fn: func(ctx context.Context) error { return errors.New("kaboom") }},
	}}
	srv := daemon.New(daemon.Config{VaultRoot: vault}, reg)
	if err := srv.RunPipeline(context.Background(), daemon.PipelineDaily, fixedNow()); !errors.Is(err, daemon.ErrPipelineIncomplete) {
		t.Fatalf("RunPipeline error = %v, want ErrPipelineIncomplete", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "_meta/.daily-last-run")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker exists after failed step: %v", err)
	}
}

func TestRunPipeline_FailedStepRecordedInLog(t *testing.T) {
	vault := t.TempDir()
	reg := daemon.Registry{Daily: []daemon.Step{
		{Name: "ok", Fn: func(ctx context.Context) error { return nil }},
		{Name: "boom", Fn: func(ctx context.Context) error { return errors.New("kaboom") }},
		{Name: "after", Fn: func(ctx context.Context) error { return nil }},
	}}
	srv := daemon.New(daemon.Config{VaultRoot: vault}, reg)
	if err := srv.RunPipeline(context.Background(), daemon.PipelineDaily, fixedNow()); err == nil {
		t.Fatal("RunPipeline returned nil after failed step")
	}
	log := readDayLog(t, vault, fixedNow())
	if !strings.Contains(log, "boom") || !strings.Contains(log, "FAILED") || !strings.Contains(log, "kaboom") {
		t.Errorf("failure not recorded in log:\n%s", log)
	}
	if !strings.Contains(log, "after") {
		t.Errorf("post-failure step did not run:\n%s", log)
	}
}

func TestRunPipeline_UnknownNameReturnsSentinel(t *testing.T) {
	srv := daemon.New(daemon.Config{VaultRoot: t.TempDir()}, daemon.Registry{})
	err := srv.RunPipeline(context.Background(), "totally-not-a-pipeline", fixedNow())
	if !errors.Is(err, daemon.ErrUnknownPipeline) {
		t.Errorf("err = %v, want errors.Is(err, ErrUnknownPipeline)", err)
	}
}

func TestStartStop_IsClean(t *testing.T) {
	srv := daemon.New(daemon.Config{VaultRoot: t.TempDir()}, daemon.Registry{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := srv.Start(ctx); err == nil {
		t.Errorf("second Start should return error")
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := srv.Stop(); err != nil {
		t.Errorf("second Stop returned %v, want nil", err)
	}
}

// --- helpers ---

func fixedNow() time.Time {
	return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
}

func readDayLog(t *testing.T, vault string, now time.Time) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(vault, "_meta/processing-logs", now.Format("2006-01-02")+".md"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return string(b)
}
