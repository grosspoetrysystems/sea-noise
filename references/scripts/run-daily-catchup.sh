#!/bin/sh
# run-daily-catchup.sh — runs daily synthesis if it hasn't completed today.
# Called at the end of the inbox loop (every 30 min) and by the daily plist.
# Uses a marker file to avoid double-runs.
# Each step runs independently — failures are logged but don't block others.

VAULT_DIR="$HOME/Library/Mobile Documents/iCloud~md~obsidian/Documents/mind-palace"
MARKER_DIR="$VAULT_DIR/_meta"
TODAY=$(date +%Y-%m-%d)
MARKER="$MARKER_DIR/.daily-last-run"
HEALTH="$MARKER_DIR/pipeline-health.md"
FAILURES=0

# Check if daily already ran today
if [ -f "$MARKER" ] && [ "$(cat "$MARKER")" = "$TODAY" ]; then
    echo "Daily synthesis already ran today ($TODAY), skipping."
    exit 0
fi

echo "=== Daily synthesis catchup: $TODAY ($(date)) ==="
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
        echo "- **$TODAY $(date +%H:%M)** — daily \`$STEP_NAME\` failed (exit $CODE)" >> "$HEALTH"
    fi
}

run_step "retry-crawls" python3 _scripts/fetch-stub-content.py --retry
run_step "summarize" python3 _scripts/summarize-transcripts.py
run_step "index" python3 _rag/indexer.py
sleep 3
run_step "daily-synthesis" env CLAUDECODE= claude -p "daily synthesis" --dangerously-skip-permissions
run_step "sync-consumed" python3 _scripts/sync-consumed.py
run_step "signal" python3 _scripts/generate-signal.py
run_step "plunge" python3 _scripts/generate-plunge.py
run_step "home" python3 _scripts/generate-home.py
run_step "todoist-push" python3 _scripts/sync-todoist.py
run_step "todoist-pull" python3 _scripts/sync-todoist-completions.py

# Mark as complete even with partial failures — marker prevents re-running
# a broken pipeline 48 times a day. Failures are in pipeline-health.md.
echo "$TODAY" > "$MARKER"

if [ "$FAILURES" -gt 0 ]; then
    echo "Daily synthesis for $TODAY completed with $FAILURES failure(s). See _meta/pipeline-health.md."
else
    echo "Daily synthesis complete for $TODAY — all steps OK."
fi
