package watcher

import (
	"context"
	"testing"
	"time"
)

func TestInboxStableRejectsWriteDuringSettle(t *testing.T) {
	previous := inboxSnapshot
	t.Cleanup(func() { inboxSnapshot = previous })

	w := &Watcher{cfg: Config{InboxSettle: time.Nanosecond}}
	before := map[string]inboxStamp{"mobile.md": {size: 7}}
	after := map[string]inboxStamp{"mobile.md": {size: 8}}
	calls := 0
	inboxSnapshot = func(*Watcher) (map[string]inboxStamp, bool) {
		calls++
		if calls == 1 {
			return before, true
		}
		return after, true
	}
	if w.inboxStable(context.Background()) {
		t.Fatal("inboxStable returned true while the snapshot was changing")
	}

	after = before
	calls = 0
	if !w.inboxStable(context.Background()) {
		t.Fatal("inboxStable returned false after the snapshot stopped changing")
	}
}
