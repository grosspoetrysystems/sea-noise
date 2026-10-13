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
