package daemon

import (
	"context"
	"testing"
	"time"
)

func TestMaybeFire_FiresInboxEveryHalfHour(t *testing.T) {
	calls := 0
	srv := New(Config{VaultRoot: t.TempDir()}, Registry{
		Inbox: []Step{{Name: "inbox", Fn: func(context.Context) error {
			calls++
			return nil
		}}},
	})

	srv.maybeFire(context.Background(), time.Date(2026, 10, 5, 12, 30, 0, 0, time.UTC))
	if calls != 1 {
		t.Fatalf("inbox calls at :30 = %d, want 1", calls)
	}

	srv.maybeFire(context.Background(), time.Date(2026, 10, 5, 12, 45, 0, 0, time.UTC))
	if calls != 1 {
		t.Fatalf("inbox calls at :45 = %d, want 1", calls)
	}
}

func TestMaybeFire_CatchesMissedInboxBoundary(t *testing.T) {
	calls := 0
	srv := New(Config{VaultRoot: t.TempDir()}, Registry{
		Inbox: []Step{{Name: "inbox", Fn: func(context.Context) error {
			calls++
			return nil
		}}},
	})

	srv.maybeFireBetween(
		context.Background(),
		time.Date(2026, 10, 5, 12, 29, 0, 0, time.UTC),
		time.Date(2026, 10, 5, 12, 31, 0, 0, time.UTC),
	)
	if calls != 1 {
		t.Fatalf("inbox calls after crossing :30 = %d, want 1", calls)
	}

	srv.maybeFireBetween(
		context.Background(),
		time.Date(2026, 10, 5, 12, 31, 0, 0, time.UTC),
		time.Date(2026, 10, 5, 12, 32, 0, 0, time.UTC),
	)
	if calls != 1 {
		t.Fatalf("inbox calls without crossing boundary = %d, want 1", calls)
	}
}

func TestMaybeFire_CatchesMissedDailyBoundary(t *testing.T) {
	calls := 0
	srv := New(Config{
		VaultRoot: t.TempDir(),
		DailyAt:   time.Date(0, 1, 1, 0, 5, 0, 0, time.UTC),
	}, Registry{
		Daily: []Step{{Name: "daily", Fn: func(context.Context) error {
			calls++
			return nil
		}}},
	})

	srv.maybeFireBetween(
		context.Background(),
		time.Date(2026, 10, 5, 0, 4, 0, 0, time.UTC),
		time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC),
	)
	if calls != 1 {
		t.Fatalf("daily calls after crossing 00:05 = %d, want 1", calls)
	}
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time { return c.now }

func TestStart_CatchesUpMissedDailyBoundary(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	srv := New(Config{
		VaultRoot: t.TempDir(),
		DailyAt:   time.Date(0, 1, 1, 0, 5, 0, 0, time.UTC),
	}, Registry{
		Daily: []Step{{Name: "daily", Fn: func(context.Context) error {
			calls++
			close(started)
			<-release
			return nil
		}}},
	}).WithClock(fixedClock{now: time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC)})

	if err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("startup catch-up did not run")
	}
	close(release)
	if calls != 1 {
		t.Fatalf("daily calls after startup catch-up = %d, want 1", calls)
	}
}
