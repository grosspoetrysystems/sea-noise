#!/bin/sh
# run-weekly-catchup.sh — runs weekly synthesis if it hasn't completed this week.
# Called at the end of the inbox loop (every 30 min) and by the weekly plist.
# Uses a marker file to avoid double-runs. Only runs on Sunday or later if missed.
# Each step runs independently — failures are logged but don't block others.

VAULT_DIR="$HOME/Library/Mobile Documents/iCloud~md~obsidian/Documents/mind-palace"
MARKER_DIR="$VAULT_DIR/_meta"
TODAY=$(date +%Y-%m-%d)
DOW=$(date +%u)  # 1=Monday ... 7=Sunday
MARKER="$MARKER_DIR/.weekly-last-run"
HEALTH="$MARKER_DIR/pipeline-health.md"
FAILURES=0

# Determine the Monday that starts the current ISO week
if [ "$DOW" -eq 7 ]; then
    DAYS_SINCE_MONDAY=6
else
    DAYS_SINCE_MONDAY=$((DOW - 1))
fi
WEEK_START=$(date -v-${DAYS_SINCE_MONDAY}d +%Y-%m-%d)

# Check if weekly already ran this week
if [ -f "$MARKER" ] && [ "$(cat "$MARKER")" \> "$WEEK_START" -o "$(cat "$MARKER")" = "$WEEK_START" ]; then
    echo "Weekly synthesis already ran this week (marker: $(cat "$MARKER"), week of $WEEK_START), skipping."
    exit 0
fi

# Only run on Sunday (preferred) or Monday (catch-up grace day)
if [ "$DOW" -ne 7 ] && [ "$DOW" -ne 1 ]; then
    echo "Weekly synthesis: not Sunday or Monday ($TODAY, dow=$DOW), skipping."
    exit 0
fi

echo "=== Weekly synthesis catchup: $TODAY ($(date)) ==="
cd "$VAULT_DIR"

run_step() {
    STEP_NAME="$1"
    shift
    echo "--- [$STEP_NAME] start: $(date) ---"
    if "$@" 2>&1; then
        echo "--- [$STEP_NAME] OK ---"
    else
        CODE=$?
        echo "--- [$STEP_NAME] FAILED (exit $CODE) ---"
        FAILURES=$((FAILURES + 1))
        echo "- **$TODAY $(date +%H:%M)** — weekly \`$STEP_NAME\` failed (exit $CODE)" >> "$HEALTH"
    fi
}

run_step "retry-crawls" python3 _scripts/fetch-stub-content.py --retry
run_step "weekly-synthesis" env CLAUDECODE= claude -p "weekly synthesis" --dangerously-skip-permissions
run_step "index" python3 _rag/indexer.py
run_step "signal" python3 _scripts/generate-signal.py
run_step "audit-links" python3 _scripts/audit-links.py
run_step "enrich-topics" python3 _scripts/enrich-topics.py
run_step "profile-entities" python3 _scripts/profile-entities.py

# Mark as complete even with partial failures
echo "$TODAY" > "$MARKER"

if [ "$FAILURES" -gt 0 ]; then
    echo "Weekly synthesis for week of $WEEK_START completed with $FAILURES failure(s). See _meta/pipeline-health.md."
else
    echo "Weekly synthesis complete for week of $WEEK_START — all steps OK."
fi
