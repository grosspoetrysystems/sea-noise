You are generating a curated reading recommendation from the signal queue.

First, run `python3 _scripts/generate-signal.py` to refresh the queue.
Then read `signal/queue.md` for the full queue state.

If the user provided a time budget (e.g. `/signal 15m`), filter to items that fit within that time. Otherwise, recommend a balanced mix:
- 1-2 quick hits for momentum
- 1 medium read aligned with active assertions or rising topics
- 1 deep dive if time allows

For each recommendation, explain *why* it's relevant right now — connect it to active assertions, recent notes, or rising topics.

End with a count of remaining unconsumed items and any stale items that should be triaged (read or discard).

$ARGUMENTS
