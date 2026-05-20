# mind-palace

> **Full system architecture, skills, and scripts:** see `AGENTS.md` and `.agents/`
> - `.agents/system-architecture.md` — complete reference (paths, env vars, quirks, data flows)
> - `.agents/skills/SKILL_MANIFEST.json` — registry of all tools and pipelines
> - `.agents/skills/<name>/SKILL.md` — detailed spec per skill
> - `.agents/go-rewrite-plan.md` — Go rewrite plan (active)

## Vault Structure
- inbox/                    — raw voice captures, temporary landing zone
- notes/fleeting/           — Stage 1 processed, awaiting daily review
- notes/ideas/              — promoted original thinking
- notes/todos/              — extracted action items
- notes/references/         — promoted references, web clips, enriched notes
- notes/references/briefs/  — auto-generated research briefs (enrich-topics)
- notes/entities/           — auto-generated entity profiles (profile-entities)
- notes/assertions/         — active intentions, goals, strategic threads
- atlas/                    — Maps of Content, one per domain
- bloom/                    — elaboration tier: weekly synthesis, assertion tracker, bridge questions
- signal/                   — generated read queue dashboard (signal/queue.md)
- daily/                    — daily notes YYYY-MM-DD
- _meta/                    — agent logs, exhaust, boneyard

## Tag Taxonomy
Tags are organized in tiers. Every note MUST have exactly one
Tier 1 tag (determines directory). Add Tier 2+ tags as relevant.

### Tier 1 — Type (maps to directory, mutually exclusive)
- #idea        → notes/ideas/
- #todo        → notes/todos/
- #reference   → notes/references/
- #entity      → notes/entities/
- #assertion   → notes/assertions/
- #fleeting    → notes/fleeting/ (default; awaiting promotion)
- #question    → notes/fleeting/ (stays until answered)

### Tier 2 — Domain / Topic (nested, slash-separated, multiple OK)
Domains grow organically. Current domains:
- #tech/ai, #tech/ai-agents, #tech/context-engineering,
  #tech/dev-tools, #tech/rag, #tech/knowledge-graphs
- #tech/software-design, #tech/software-engineering
- #life/cooking, #life/health, #life/fitness, #life/sports,
  #life/music, #life/personal-growth
- #culture/criticism, #culture/rationalism, #culture/tech
- #science/... (expand as needed)
- #learning/... (expand as needed)
- #business/strategy, #business/ops

### Tier 3 — People (only when central to the note)
- #people/<name-slug> (e.g. #people/scott-alexander)

### Tier 4 — Context / Source
- #work, #personal (for todo routing to Todoist)
- #entity-profile, #reference/brief (generated content markers)
- #web-clipper, #voice-capture, #voice-todo, #clippings

### Retired tags
- #recipe → use #life/cooking on #reference or #idea notes
- notes/recipes/ directory is deprecated — do not use

## Command: "process my inbox"
For each file in inbox/, first detect the source type:

**Web Clipper** (has frontmatter with source: "https://..." or tags: clippings):
- Duplicates should already be discarded by `fetch-stub-content.py`
  before this step. If a duplicate somehow reaches inbox processing,
  move it to `_meta/discarded/` with reason "duplicate".
- Stubs (from iOS Share Sheet, minimal body) should already be
  hydrated by `_scripts/fetch-stub-content.py` before this step.
  If a stub still has no body, skip enrichment but still triage.
- Preserve the original content — do not rewrite or clean
- Extract a descriptive title from the article headline
- Keep the source URL in frontmatter
- Write a 1-2 sentence summary above the ## Raw section
- Tag as #reference (plus any relevant topic tags)
- Preserve consumed: false from web clipper frontmatter (read queue)
- Set source: web-clipper in frontmatter
- If source URL is a YouTube video: after inbox processing,
  run `python3 _scripts/fetch-youtube-transcript.py` to fetch
  and append the video transcript under ## Transcript

**Voice Capture** (raw dictation, filler words, no frontmatter or
has source: voice-capture):
- Clean transcript — remove filler words, false starts
- Preserve raw original under ## Raw section

**Ad Hoc Note** (typed note — clean prose, no filler words, no
source URL, may have user-written frontmatter):
- Do NOT rewrite, rephrase, or clean the content
- Preserve exactly as written — this is intentional prose
- Only add frontmatter fields (tags, date) if missing

For all source types:
1. Extract a descriptive title (3-5 words, no generic terms like
   "capture" or "test") — skip if file already has a good title
2. Assign tags per the Tag Taxonomy above. Every note gets exactly
   one Tier 1 tag. Add 1-3 Tier 2 domain tags based on content.
   Add Tier 3 people tags only when a person is central to the note.

   Tier 1 Tag Selection (CRITICAL — read carefully):
   - **#reference** — content created by someone else that the user
     saved to read/watch/listen to later. Articles, videos, threads,
     documentation. The user is a consumer, not the author.
   - **#idea** — the user's original thinking. Even a single sentence
     of opinion, observation, or speculation is an idea. If the user
     wrote it (not clipped it), default to #idea over #reference.
     Product ideas, shower thoughts, "what if..." musings, reactions
     to things they read — all #idea.
   - **#assertion** — a directional intention. "I want to...",
     "I'm going to...", "my goal is...", "I plan to...",
     "I need to learn...", "I want to build...". These express what
     the user wants to become, build, or pursue. If a note has
     `tags: [assertion]` in frontmatter, ALWAYS respect that — the
     user explicitly marked it. Also detect assertion-like language
     in voice captures and ad hoc notes.
   - **#question** — the user is asking something they don't know.
   - **#todo** — a concrete task with a clear done state.
   - **#fleeting** — genuinely unsure. Use sparingly.

   Content Type Classification (set `content-type` in frontmatter):
   - **deep-read** — essays, long-form articles, threads with a thesis,
     talks/videos with substantive argument. The user sits down and
     engages. This is the core of the signal queue.
   - **tool-reference** — manuals, documentation, API docs, system design
     case studies, framework READMEs, how-to guides. Consulted when
     needed, not "read" in the traditional sense.
   - **listen-watch** — podcasts, video essays, interviews. Different
     mode and time budget. Detect by source URL (youtube.com, spotify,
     podcast hosts) or explicit media markers.
   - **anchor** — see `mood` field below.
   - **stub** — landing pages, profile bios, thin tweets, product
     homepages with no article content, GitHub profiles. Set
     `low-signal: true`. Excluded from signal queue entirely.

   The `mood` frontmatter field:
   - The user may set `mood: true` on web clips from the iOS clipper.
     This means: "I clipped this to have it in the vault, not to read
     later." It might be inspiration, a cultural reference, a critical
     element, an ideological anchor, an institution they care about —
     the reason doesn't matter. The point is it's already served its
     purpose by being captured. Index it as a node, link it in atlas
     MOCs, but skip the signal queue. Set `consumed: true` during
     triage. Set `content-type: anchor`.

   Heuristics for content-type (when not mood):
   - Source is youtube.com or spotify or podcast host → listen-watch
   - Body is docs/manual/API reference with code examples → tool-reference
   - Body is <100 chars after stripping frontmatter → stub
   - Person's Twitter bio or GitHub profile → stub
   - Long-form prose with a thesis and argument → deep-read
   - When unsure, default to deep-read (better to include than exclude)

   Todo Detection:
   - If transcript contains "add a todo", "todo:", "task:", or similar:
     - Extract task text verbatim after the trigger phrase
     - If transcript contains "work" → add tag #work
     - Otherwise → add tag #personal
     - Always add tag #todo
     - Set source: voice-todo in frontmatter
     - If the task text contains a temporal phrase (e.g. "next Monday",
       "at 6 PM", "tomorrow", "next week", "on Friday", "in 3 days"):
       - Normalize into a clean Todoist-compatible due string:
         - Format: "<weekday or relative day> at <HH:MM am/pm>"
         - Examples: "next Wednesday at 6pm", "Friday at 5pm", "tomorrow at 9am"
         - "next week on Wednesday at 6 PM" → "next Wednesday at 6pm"
         - "before 5pm" / "by 5p" → drop "before"/"by", use "5pm"
         - Normalize "5p" → "5pm", "6P" → "6pm" (always append m)
         - Never use "next week at X on Y" — always resolve to specific day
       - Set due: "<normalized string>" in frontmatter
       - Strip the temporal phrase from the title — keep only the action
         - "tell the client next week at 6 PM on Wednesday that we need
           a new PRD" → title: "tell the client that we need a new PRD"
         - "grab soccer jerseys on Friday before 5p"
           → title: "grab soccer jerseys"
     - Set title to the cleaned task text — do not rephrase or summarize,
       only remove the temporal phrase

4. Rename file to YYYY-MM-DD-descriptive-slug.md
5. Move to notes/fleeting/
   - Exception: #todo moves directly to notes/todos/ (needs
     immediate Todoist sync)
   Fleeting is the active workbench — all notes land here first
   so the user can annotate, add memos, connect thoughts
   throughout the day before daily synthesis files them.
6. Log to _meta/processing-logs/YYYY-MM-DD.md:
   - filename, tags assigned, one-line summary, timestamp

Do not create wikilinks yet. Do not update MOCs yet.
Stage 1 is triage only — fast and non-destructive.

## Command: "daily synthesis"
Catch-up check: before starting, scan daily/ for the most recent
daily note. If there are gaps (missing dates between then and today),
process notes/fleeting/ files from those missing days too — use the
file's frontmatter date or date prefix to determine which day it
belongs to. Create a daily/YYYY-MM-DD.md for each missed day.

Review all files in notes/fleeting/ from the last 24 hours
(or longer if catching up):

### Step 1: Re-triage (fix misclassifications)
Before anything else, scan every note in fleeting/ and check:
- Does it have `tags: [assertion, ...]` in frontmatter? → it's an
  assertion. Ensure it has ## Assertion / ## Why structure. Move to
  notes/assertions/ immediately.
- Does it contain "I want to...", "I plan to...", "my goal is...",
  "I need to learn..." language? → suggest upgrading to #assertion.
  If the intent is clear, create the assertion structure and move it.
- Is it tagged #idea but still in fleeting/? → promote to notes/ideas/
- Is it tagged #reference but contains the user's original thinking
  (not just a clip)? → re-tag as #idea
- Is it a low-signal stub (personal page, GitHub profile, <100 chars
  of body)? → add `low-signal: true` if not already set

### Step 2: Assertion tracking
Read all notes in notes/assertions/ (including any just moved).
For each active assertion:
- Scan today's new notes for evidence — does any note provide
  support, context, counter-evidence, or relate to this assertion?
- If yes, add a [[wikilink]] under the assertion's ## Evidence section
- Update bloom/assertion-tracker.md with current state

### Step 3: Connect and link
For each note in today's batch:
1. Scan the vault for related existing notes and add [[wikilinks]]
   where genuinely relevant
2. If a link was considered but rejected, log to
   _meta/rejected-connections.md with reason
3. Update _meta/topic-frequencies.md — increment count for each
   topic/tag seen, update last_seen, set trend to "rising" if
   count increased >2 in last 7 days
4. Find or create the relevant atlas/ MOC and add a [[wikilink]]
   to the note under the appropriate section
5. Create a new atlas/ MOC if any tag cluster has 5+ notes
   without an existing MOC
6. Split any atlas/ MOC section with 15+ links into its own sub-MOC

### Step 4: Re-enrich modified notes
For each note about to leave fleeting, check if the user has modified
it since initial processing (added memos, annotations, connected
thoughts). Signs: content added outside ## Raw or ## Research Context,
new sections, inline comments. If modified, re-run enrichment.

### Step 5: Promote (MANDATORY — do not skip)
Move notes out of fleeting to their final directory by Tier 1 tag.
This is not optional. Every note with a clear Tier 1 tag MUST move:
- #idea → notes/ideas/
- #reference → notes/references/
- #reference with #reference/brief → notes/references/briefs/
- #entity → notes/entities/
- #assertion → notes/assertions/
- #question — stays in notes/fleeting/ (unresolved)
- #fleeting — stays in notes/fleeting/ (needs deliberation)
Low-signal notes (empty, single word, test captures) → _meta/discarded/

### Step 6: Write daily note
Write daily/YYYY-MM-DD.md. This is NOT a flat link dump. Structure:
```
# YYYY-MM-DD

## What came in
<2-3 sentence summary of the day's intake — count of notes by type,
dominant topics>

## Observations
<1-3 genuine observations about the day's notes. What's interesting?
What connects? What's surprising? What tension exists? This should
read like a brief journal entry, not a database report.>

## Assertions touched
<list any assertions that got new evidence today, with what changed>

## Moved
<compact list: [[note]] → directory (tag)>
```

Keep it concise. The daily note is a breadcrumb trail, not an index.

## Command: "weekly synthesis"
1. Read all daily/YYYY-MM-DD.md notes from this week — these are
   your raw material. The weekly synthesizes the dailies, not the
   individual notes.
2. Read all atlas/ MOCs as compressed vault index
3. Identify 3-5 disconnected topic clusters that share implicit
   themes — these are structural gaps
4. Scan _meta/topic-frequencies.md for topics with count >5 in
   last 30 days — these are latent priorities
5. Read any bloom/*-reflect.md companion pieces from this week —
   these are the user's original thinking from /reflect sessions.
   Let them inform this week's synthesis and provocations.
6. Write a synthesis note to bloom/YYYY-MM-DD-weekly.md:
   ```
   # Week of YYYY-MM-DD

   ## This week
   <3-5 sentence narrative of what the user was thinking about this
   week. Not a list. A story. "You spent the early part of the week
   deep in agent orchestration, then pivoted to governance and
   liveness on Wednesday after the Venkatesh Rao piece. By Friday
   you were sketching product ideas that bridge those threads...">

   ## Ideas vs References ratio
   <how many original thoughts vs clipped content? Is the user
   thinking or just collecting? Be honest.>

   ## Assertion tracker
   <for each active assertion: momentum this week (growing/steady/
   stale), new evidence count, notable connections>

   ## Connections made
   <the interesting links discovered this week — not all of them,
   just the surprising or meaningful ones>

   ## Provocations
   <3-5 evocative prompts — see rules below>
   ```

   ## Provocations rules — these are the product. Get them right.
   These are NOT dry "how might X inform Y?" questions.
   They should be specific, grounded in actual notes, and
   designed to provoke original thinking. Good provocations:
   - Name a real tension between two notes or ideas
   - Point at a gap where a note *should* exist but doesn't
   - Challenge an assumption the user seems to be making
   - Connect two threads the user hasn't connected yet
   - Ask what the user actually *believes* about a topic
     they've been collecting references on but haven't written about
   - Call out the ideas-to-references ratio if it's lopsided
   Example: "You've clipped 6 articles on context engineering
   but haven't written a single original note. What's your
   actual take? Where do you disagree with the consensus?"
6. Vault hygiene:
   - Identify orphan notes (no inbound or outbound links) and
     attempt to link or route to atlas/
   - Flag notes in notes/fleeting/ older than 7 days for review
   - Check for near-duplicate filenames and log to
     _meta/rejected-connections.md
7. Catch-up audit: check daily/ for any gaps in the last 7 days.
   If a day is missing, scan notes/fleeting/ for notes from that
   date (by frontmatter date field) and create the missing daily
   note with summaries. This ensures nothing falls through if daily
   synthesis was missed due to downtime or sleep.
8. Discarded cleanup: scan _meta/discarded/ for files. Purge any
   that are empty, single-word, or test artifacts (no meaningful
   content to preserve). For files with some content, leave them
   but log a count to the weekly insight.
9. Self-improvement: review _meta/processing-logs/ from the past week.
   If any pattern of errors, misclassifications, or missed links is
   detected, propose a specific update to this CLAUDE.md and append
   it to _meta/claude-md-changelog.md for review
10. Taxonomy evolution: scan all Tier 2 tags used in the past week.
    If any tag has been used on 3+ notes but is not listed in the
    Tag Taxonomy section above, propose it to
    _meta/taxonomy-proposals.md with: tag name, note count, example
    notes, suggested parent domain. Do not auto-add to CLAUDE.md —
    proposals require user review. Also flag if any existing domain
    has 15+ notes and should split into sub-domains.

## Atlas (Maps of Content)
The atlas/ directory holds Maps of Content — one per domain cluster.
These are the vault's navigation layer. They are NOT flat link lists.

### Structure of a good MOC
```
# Domain Name
<2-3 sentence orientation: what this domain means to the user,
why they care, what they're exploring here>

## Active Assertions
<wikilinks to any assertions tagged with this domain>

## Key Ideas (user's original thinking)
<wikilinks to #idea notes in this domain — the user's take>

## References (collected material)
<wikilinks to #reference notes, grouped by subtopic>

## Open Questions
<questions the MOC itself raises — gaps, tensions, threads to pull>

## Connections to Other Domains
<where this domain touches other MOCs — e.g. "AI agents connects
to governance via intent engineering and to creative practice via
the poet-engineer thread">
```

### MOC lifecycle
- **Creation:** when any tag cluster has 5+ notes without a MOC
- **During daily synthesis:** add new notes to the relevant MOC.
  Don't just append — place them in the right section (idea vs
  reference vs assertion). Update ## Open Questions if the new
  note raises or answers one.
- **During weekly synthesis:** review each MOC. Is the orientation
  still accurate? Are there new cross-domain connections? Update
  ## Connections to Other Domains. Flag MOCs that are all
  references with no ideas (the user is collecting, not thinking).
- **Split:** when a section has 15+ links, split into a sub-MOC
- **atlas/index.md:** master navigation. One line per MOC with a
  sentence describing what's there.

### Tier 2 tags → MOC mapping
Every Tier 2 domain tag should eventually have a corresponding MOC.
Multiple related tags can share one MOC (e.g. tech/ai + tech/ai-agents
→ atlas/ai-agents-and-tools.md). The MOC is the human-readable face
of the tag cluster.

## Assertions
Assertions are active intentions, goals, and strategic threads.
Unlike ideas (observations) or todos (tasks), assertions are
directional — they express what the user wants to become, build,
or pursue. They live in notes/assertions/ with tag #assertion.

Structure of an assertion note:
```
---
title: "..."
tags: [assertion, <tier-2 domains>]
date: YYYY-MM-DD
status: active | paused | completed
---
## Assertion
<1-2 sentence statement of intent>

## Why
<motivation, context, what success looks like>

## Evidence
<wikilinks to notes that support or advance this assertion>

## Open Questions
<what needs to be figured out>
```

During daily synthesis:
- Check each new note against active assertions — if a note
  provides evidence, adds context, or relates to an active
  assertion, add a [[wikilink]] under the assertion's ## Evidence
- Do not force connections — only link when genuinely relevant

During weekly synthesis:
- Review all active assertions. For each:
  - Count new evidence links added this week
  - Note if no new evidence has appeared (stale assertion)
  - Surface in the weekly insight under "Assertion Tracker"

Assertions can be created via voice capture ("I want to...",
"I'm thinking about building...", "my goal is...") or manually.

## Temporal Hierarchy (daily → weekly → monthly → yearly)
Synthesis compresses upward. Each level distills the one below.

- **daily/YYYY-MM-DD.md** — breadcrumb trail. What came in, what
  moved, 1-3 observations. Written during daily synthesis.
- **bloom/YYYY-MM-DD-weekly.md** — narrative synthesis. Patterns,
  connections, provocations. Distills that week's dailies. Written
  during weekly synthesis.
- **bloom/YYYY-MM-monthly.md** — monthly distillation. Written on
  the first weekly synthesis of a new month. Reads that month's
  weeklies and produces:
  - Dominant themes (what did the month look like?)
  - Assertion momentum (which goals advanced? which stalled?)
  - Ideas-to-references ratio for the month
  - What shifted from the previous month
  - 2-3 provocations for the month ahead
- **bloom/YYYY-yearly.md** — annual reflection. Written on the first
  weekly synthesis of a new year. Reads that year's monthlies.
  The arc of your thinking over 12 months.

During weekly synthesis: if the current date is in a new month
compared to the most recent monthly, generate the monthly for the
previous month first. Same logic for yearly.
Bootstrap case: if no monthly exists yet but there are weeklies
from a prior month, generate that month's monthly before proceeding.
For the "What shifted from the previous month" section, note that
it's the vault's first month and establish the baseline instead.

## Signal Queue (consumed field)
Web clips arrive with `consumed: false` in frontmatter (set by
the Obsidian Web Clipper). This tracks whether the user has
actually read/watched/listened to the content.

- Do NOT set consumed to true during processing — only the user
  marks content as consumed
- `_scripts/generate-signal.py` builds `signal/queue.md` — a
  rolling dashboard of unconsumed items scored by assertion
  overlap, rising topics, and recency, bucketed by read time
- Signal queue is regenerated during daily synthesis (after
  Claude prompt) and weekly synthesis
- The user can run `/signal [time]` for curated recommendations
  filtered by available time and current interests
- During weekly synthesis, include a "Signal Queue" section:
  - Count of unconsumed items by domain tag
  - Highlight stale items (>14 days unconsumed)
  - Summarize assertion-aligned priority items

Signal Queue Quality and Structure:
- Notes with `low-signal: true` → excluded entirely
- Notes with `mood: true` or `content-type: anchor` → excluded
  (already consumed by definition)
- Remaining items grouped by content-type, not just read time:
  ```
  ## Deep Reads (essays, articles, threads with a thesis)
  ## Listen / Watch (podcasts, videos)
  ## Tools & Reference (manuals, docs, case studies)
  ```
  Within each group, sort by score (assertion overlap + rising + recency)
  and show estimated time.
- This replaces the old Quick Hits / Medium / Deep Dives buckets.
  Content type matters more than time estimate for deciding what to
  engage with — a 5-minute essay and a 5-minute manual are different
  activities.

## Bloom (elaboration tier)
The `bloom/` directory holds higher-level synthesis output:
- `bloom/YYYY-MM-DD-weekly.md` — weekly synthesis notes with
  ## Provocations section (writing prompts, tensions, gaps)
- `bloom/assertion-tracker.md` — rolling assertion dashboard,
  updated incrementally during daily synthesis, fully regenerated
  during weekly synthesis
- `bloom/YYYY-MM-DD-reflect.md` — companion pieces from /reflect
  sessions, capturing the user's original thinking and dialogue

During daily synthesis: update bloom/assertion-tracker.md with
any new evidence links added today.

During weekly synthesis: fully regenerate assertion-tracker.md.
Read this week's reflect companion pieces to inform provocations.

## Crawl Retry System
Failed crawls are tracked in `_meta/crawl-failures.json`.
- `fetch-stub-content.py` logs failures with exponential backoff
  (6h → 24h → 72h → 168h), abandons after 4 failures
- `fetch-stub-content.py --retry` retries pending entries past
  their backoff window, hydrating notes in place (even if the
  note has moved from inbox to fleeting/references)
- Retry runs at the start of daily and weekly chains

## Web Clip Deduplication
`fetch-stub-content.py` deduplicates web clips before inbox
processing:
- Scans notes/fleeting/, notes/references/, and notes/ideas/
  for existing source URLs
- Normalizes URLs (strips tracking params, fragments, trailing
  slashes) before comparison
- Duplicates are moved to `_meta/discarded/` with reason
- Also catches inbox-internal dupes (same URL clipped twice)

## Conventions
- All dates: YYYY-MM-DD
- Filenames: kebab-case, date-prefixed, descriptive
- Never delete user content — only move to _meta/discarded/ with reason
- Keep notes atomic — one idea per note
- Prefer linking over duplicating
- Raw transcripts always preserved under ## Raw
- Context window note: always read atlas/index.md first as vault
  orientation before reading individual notes

## Vault Hygiene (imperatives)
- Every note MUST have exactly one Tier 1 tag — no exceptions
- Tier 1 tag determines directory — if a note is in the wrong
  directory for its tag, move it during daily synthesis
- Tags use lowercase, kebab-case, slash nesting: #domain/subtopic
- Never create flat tags that duplicate a nested tag
  (use #tech/ai not #ai if #tech/ai exists)
- Entity profiles go in notes/entities/, never notes/references/
- Research briefs go in notes/references/briefs/
- notes/recipes/ is deprecated — do not use or create
- Orphan notes (no links in or out) are flagged during weekly synthesis
- Generated content (entities, briefs) is support material — keep it
  clearly separated from the user's original thinking
- Profile generation should be conservative: only profile entities
  with 5+ mentions, max 5 per run, Claude pre-filter required
- _meta/taxonomy-proposals.md tracks proposed new tags — never
  auto-add tags to the taxonomy without user review
