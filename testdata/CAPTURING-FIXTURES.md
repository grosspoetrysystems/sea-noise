# Capturing fixture golden files

This document describes the procedure for capturing parity golden files from the Python prototype against the fixture vault. Run this procedure when the fixture vault contents change, or when the prototype's parity-critical behavior changes and you want the golden files to reflect the new behavior.

## What's in here

- [`fixture-vault/`](./fixture-vault/) — 34 markdown notes copied from the live iCloud vault at the snapshot time of this directory. Pinned; never edited; never re-synced from the live vault. Each note is in its original vault path under `fixture-vault/` (e.g., `notes/assertions/2026-03-10-learning-music-at-thirty-six.md`).
- [`scripts/capture-parity.py`](./scripts/capture-parity.py) — the capture script. Imports nothing third-party; reproduces the prototype's parity functions verbatim.
- [`expected/frontmatter/index.json`](./expected/frontmatter/) — what the prototype's indexer would extract per note: `{id, path, title, tags, body_preview}`. Notes under `signal/` and `bloom/` are excluded (per `INDEXER_EXCLUDE_DIRS` in `references/scripts/_rag/indexer.py`).
- [`expected/links/per-note.json`](./expected/links/) — what the prototype's link auditor extracts per note: `{source, links: [target, ...]}`. Notes under `_rag/`, `templates/`, etc. are excluded (per `LINKS_EXCLUDE_DIRS` in `references/scripts/audit-links.py`); notes under `signal/` and `bloom/` ARE included (different exclude set than the indexer — different surfaces, different rules).
- [`expected/links/broken.json`](./expected/links/) — links whose target stem is not in the fixture vault's stem set. Most of the fixture's links are "broken" relative to the fixture because the fixture is a 34-note subset of a 460-note vault; this is expected and is part of the parity contract (the Go port should report the same broken-link set).

## What the golden files are NOT

- They are NOT a comprehensive snapshot of every prototype behavior. They cover the markdown-engine + vault-abstraction surface in P1-W3 scope (T03). When later waves port additional prototype scripts (T05's pipeline commands, etc.), this capture script grows to cover their surfaces too.
- They are NOT machine-translated regression tests. They are the prototype's actual outputs on real notes. A failing parity test means the Go port produces different output for the same input, which is a real bug (or a deliberate documented delta in `KNOWN-DELTA.md`).
- They are NOT meant to be hand-edited. Edit the prototype's source or the fixture vault, then re-run the capture script.

## Capture environment

The capture script in P1-W3 scope is pure stdlib — no `requirements.txt` deps needed. The venv exists as the established substrate for future waves where heavier scripts (lancedb-backed indexer, frontmatter package, etc.) become parity targets.

Python version pinned for capture: **3.12+ recommended; 3.14 verified.**

Setup (once per machine):

```sh
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt  # currently sparse; adds deps in later waves
```

`requirements.txt` is at the repo root and committed. `.venv/` is gitignored.

## Capture procedure

From the repo root:

```sh
.venv/bin/python3 testdata/scripts/capture-parity.py
```

Expected output:

```
frontmatter: 27 records -> .../expected/frontmatter/index.json
links: 34 notes, 426 links, 367 broken -> .../expected/links/
```

Then check the diff:

```sh
git diff testdata/expected/
```

If the diff is non-empty, decide whether the change is intentional:

- **Intentional** (e.g., new fixture notes, prototype behavior update): commit the new golden files alongside the change that caused them. The commit subject should describe the change in the code/data, not the recapture itself (e.g., `add: 3 entity notes to fixture vault`, not `update: golden files`).
- **Unintentional** (e.g., Python version drift, env contamination): investigate before committing. Pin a Python version, fix the env, recapture.

## Note on selection from the live vault

Notes were selected deterministically: a fixed N per directory, sorted by filename, taking the head or tail of the listing. Selection details are not committed as a script because the fixture is now pinned — the selection happened once. Re-selecting from the live vault is a deliberate operation that requires re-running the original selection (or a new one) and re-running the capture, with both diff sets reviewed together.

Top-level fixture composition:

| Source dir | Notes copied | Selection |
|-----------|--------------|-----------|
| `HOME.md`, `AGENTS.md` (top-level) | 2 | top-level |
| `daily/` | 2 | sort, tail 2 (most recent) |
| `bloom/` | 6 | all (small directory) |
| `atlas/` | 4 | all (small directory) |
| `signal/` | 1 | all (small directory) |
| `plunge/` | 1 | all (small directory) |
| `notes/assertions/` | 5 | all (small directory) |
| `notes/ideas/` | 3 | sort, head 3 |
| `notes/entities/` | 3 | sort, head 3 |
| `notes/references/` | 3 | sort, head 2 + 1 deliberate add (`2026-04-12-rich-hickey-simple-made-easy-talk.md`, the transcript-bearing fixture for T-PROVE on `summarize-transcripts`; see `.tempor/exec/P1-W2-kickoff.md`) |
| `notes/references/briefs/` | 2 | sort, head 2 |
| `notes/todos/` | 2 | sort, head 2 |
| `export/` | 1 | sort, head 1 |
| **total** | **35** | |

## Capturing LLM envelopes (for parity tests of LLM-touching workflows)

Stdlib-only `capture-parity.py` covers the deterministic surfaces (frontmatter, links, signal, retrieval index). LLM-touching workflows (`summarize-transcripts` first, more in T-SCALE) need a separate capture pass that records the prototype's LLM response per invocation so the Go port can replay it via `internal/parity.RecordedTransport`.

That capture lives at [`scripts/capture-llm-envelopes.py`](./scripts/capture-llm-envelopes.py) and uses the [`scripts/claude-shim.py`](./scripts/claude-shim.py) PATH-shim. The shape (hybrid two-pass capture — "path A" in the W2 spike):

1. **Discover Go-side prompts.** Build `sea-noise`, run `sea-noise parity-discover --skill <name> --vault testdata/fixture-vault`. This walks the fixture with the same qualification rules the live workflow applies and prints the rendered Go skill prompt per qualifying note — without invoking the LLM.
2. **Run the prototype with PATH-shim.** Copy the fixture vault to a temp dir, drop the prototype script alongside (so its hard-coded `VAULT = Path(__file__).parent.parent` resolves to the temp vault), put `claude-shim.py` as `claude` first on PATH, run the prototype via the project venv. Each `claude -p` call is intercepted: the shim records `(prompt, response, skill_sha)` and passes through to the real `claude` binary.
3. **Pair and emit.** For each Go prompt, find the matching shim-captured envelope by transcript-content overlap. Re-emit under the Go prompt's SHA256-hex-trunc-16 hash at `testdata/envelopes/<workflow>/<hash>.json` (matches `internal/parity.HashPrompt`).
4. **Snapshot post-state.** Save the post-Python vault state to `testdata/expected/<workflow>/post-state/`. The parity test diffs the Go workflow's post-state against this snapshot under `internal/parity.DefaultEquivalence` (files present + frontmatter keys + wikilink set; LLM-authored body prose is not compared per `.tempor/PARITY.md` decision 2).

Run when fixture content, prototype source, or the Go skill template changes:

```sh
.venv/bin/python testdata/scripts/capture-llm-envelopes.py
```

Real `claude -p` calls happen during this capture. Keep the fixture small.

Required venv state: `python-frontmatter` from `requirements.txt`. The capture script also needs a usable `claude` binary on PATH (or `REAL_CLAUDE=/path/to/claude`).

### profile-entities (two-phase workflow)

`profile-entities` fires two distinct skills per run: a single `filter-entity-candidates` call that classifies which regex-NER candidates are real entities, followed by up to `MAX_PROFILES_PER_RUN` (= 5) `generate-entity-profile` calls — one per kept entity in ranked-by-NER-count order. `testdata/scripts/capture-profile-entities.py` extends the W2 hybrid shape with two-pass discovery to handle this:

1. **Discover the filter prompt.** `sea-noise parity-discover --skill profile-entities --mode filter --vault testdata/fixture-vault` — emits a single `{kind: "filter", prompt}` entry rendered against the fixture's ranked candidates.
2. **Run the prototype.** Same PATH-shim setup as `summarize-transcripts`, but the shim's `SEA_NOISE_CAPTURE_SKILL` is set to a placeholder (`profile-entities-pending`) because the shim can't see which skill each `claude -p` call serves. The capture script fixes up `skill_name` + `skill_sha` post-pairing.
3. **Classify and pair envelopes.** The capture script loads every shim envelope, classifies each by a prompt-content marker (`You are filtering entity candidates` → filter; `Generate a profile note for '<entity>'` → profile, entity extracted from the prompt). One filter envelope; up to five profile envelopes keyed by entity.
4. **Parse the filter response → keep set.** The capture script extracts the `{name, keep, type}` JSON array from the filter envelope's response (same regex as the prototype's `filter_candidates_with_claude`), yielding the name + type for each kept entity.
5. **Second-pass discover.** `sea-noise parity-discover --skill profile-entities --mode profile --keep-names <...> --keep-types <...>` — emits `[{kind: "profile", rel: <entity>, prompt}, ...]` one per kept entity in ranked order, capped at `MaxPerRun=5`.
6. **Re-key + emit.** Each profile envelope from the prototype is paired with its Go counterpart by entity name (the natural key for this workflow — content-overlap matching is unnecessary). Re-emit under the Go prompt hash at `testdata/envelopes/profile-entities/<hash>.json` with `skill_name` and `skill_sha` corrected.
7. **Snapshot post-state.** Save to `testdata/expected/profile-entities/post-state/` as usual.

Run with:

```sh
.venv/bin/python testdata/scripts/capture-profile-entities.py
```

Re-run when the fixture vault's entity counts change, when `references/scripts/profile-entities.py` changes, or when either `filter-entity-candidates.md` or `generate-entity-profile.md` skill template changes. Burns 1 filter + up to 5 profile `claude -p` calls.

The Go parity test pins `Today: 2026-06-01` to match the captured snapshot's profile filenames (`notes/entities/2026-06-01-profile-<slug>.md`). Re-capturing on a different date requires re-pinning the test's `Today` constant in lockstep.

## Live-vault drift

The live iCloud vault evolves; the fixture does not. When the rewrite ships and parity is demonstrated against this fixture, the fixture remains valid. Future expansions (more case-types, edge cases discovered later) should add to the fixture and re-run capture — not replace the existing notes.
