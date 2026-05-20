# Mind-Palace Go Rewrite Plan

> Rewrite the mind-palace automation system from Python/shell scripts
> into a single Go binary with CLI installer, scheduler management,
> and distribution via `go install` / Homebrew.

## Reference Documents

- **System architecture:** `.agents/system-architecture.md` — full inventory of all components, paths, env vars, quirks
- **Skill manifest:** `.agents/skills/SKILL_MANIFEST.json` — registry of all skills/tools with scripts, deps, env vars
- **Skill definitions:** `.agents/skills/<name>/SKILL.md` — detailed spec per skill (11 skills total)
- **AGENTS.md** — project overview, pipeline diagrams, script table
- **CLAUDE.md** — Claude system instructions (triage, synthesis, tag taxonomy)

Each Go subcommand maps 1:1 to a skill definition. The SKILL.md files are the source-of-truth specs for the rewrite.

## Current State

- ~2,261 lines across 11 Python scripts + 2 shell scripts + 2 RAG modules
- Scattered across: _scripts/, _rag/, ~/Library/LaunchAgents/, .claude/
- Tightly coupled to vault path (iCloud), macOS launchd, Python runtime
- Now fully documented in `.agents/` with skill manifest + architecture reference

## Design Principles

1. **Separate runtime from vault** — Go binary is the harness (all business logic, scheduling, export, sync). Vault is just markdown — no scripts, no RAG infra, no agent configs. Open it in Obsidian and it's a clean knowledge base.
2. **Single binary** — no Python, no pip, no venv, no node. Just `mind-palace`
3. **Config-driven** — vault path, API keys, schedule preferences, agent paths all in one config file (lives with the binary, not the vault)
4. **Claude stays Claude** — AI tasks (inbox triage, synthesis, enrichment) still shell out to `claude -p`. Go handles all deterministic orchestration, HTTP, file I/O, and parsing
5. **Platform-aware** — macOS launchd first, with room for cron/systemd later
6. **Graceful degradation** — Cloudflare, Todoist, embeddings, agent sync are all optional features
7. **Export as interface** — the runtime generates read-only research packs from the vault. External agents (ZeroClaw, etc.) consume these packs without touching the vault. The vault never knows about the agents; the agents never write to the vault. User clips valuable output back through the normal inbox pipeline.

## Architecture

### Core Separation

```
mind-palace (Go binary)              vault (pure markdown)
├── all business logic                ├── inbox/
├── all scheduling                    ├── notes/
├── pipeline orchestration            ├── atlas/
├── research pack export              ├── bloom/
├── agent sync                        ├── signal/
├── embedded templates                ├── daily/
└── mind-palace.toml                  ├── _meta/
                                      ├── .claude/
                                      └── .obsidian/

          export/                     agent ecosystem (ZeroClaw/mise)
          (read-only packs)           ├── agent-a/context/ ← packs land here
          generated outside vault     ├── agent-b/context/
          by the runtime              └── ...
```

### Binary Layout

```
mind-palace-cli/                    (new repo, e.g. ~/repos/mind-palace-cli)
├── cmd/
│   └── mind-palace/
│       └── main.go                 — Kong root command
├── internal/
│   ├── config/                     — TOML config, env var loading, vault path resolution
│   ├── vault/                      — Vault abstraction: walk notes, read/write, move, discard
│   ├── markdown/                   — Frontmatter parser (YAML), markdown section manipulation
│   ├── crawl/                      — Cloudflare /crawl HTTP client + retry/backoff logic
│   ├── todoist/                    — Todoist REST v2 + Sync v9 client (push + completions)
│   ├── youtube/                    — YouTube transcript fetching (HTTP, no library needed)
│   ├── signal/                     — Signal queue scoring + dashboard generation
│   ├── embed/                      — Embedding client (OpenAI API or Ollama)
│   ├── search/                     — Vector store (SQLite + vec extension, or Qdrant lite)
│   ├── audit/                      — Broken wikilink scanner
│   ├── enrich/                     — Topic enrichment + entity profiling orchestration
│   ├── export/                     — Research pack generation (theme detection, bundling)
│   ├── agentsync/                  — Push packs to external agent context/ dirs
│   ├── claude/                     — Claude CLI wrapper (exec subprocess, parse output)
│   ├── scheduler/                  — LaunchAgent plist generation + launchctl management
│   └── pipeline/                   — Chain orchestration: inbox, daily, weekly runners
├── templates/                      — Embedded plist templates, CLAUDE.md template, etc.
├── go.mod
├── go.sum
├── Makefile
├── goreleaser.yml                  — Cross-platform release builds
└── README.md
```

### What the vault looks like after install

The vault is pure data. No scripts, no RAG infra, no agent configs.
Copy it to another machine and it's just a folder of markdown.

```
~/my-vault/                         (user's chosen path)
├── inbox/
├── notes/{fleeting,ideas,todos,references,entities,assertions}/
├── atlas/
├── bloom/
├── signal/
├── daily/
├── _meta/                          — runtime-managed state only (markers, logs)
├── .claude/
│   ├── CLAUDE.md                   — Generated from template, customizable
│   └── commands/
│       ├── reflect.md
│       ├── signal.md
│       └── research.md
└── .obsidian/                      — Obsidian app config
```

Note: `mind-palace.toml` lives with the binary install (`~/.config/mind-palace/`
or `~/.mind-palace/`), NOT in the vault. The vault is Obsidian-portable.

### Config file (mind-palace.toml)

```toml
[vault]
path = "~/Library/Mobile Documents/iCloud~md~obsidian/Documents/mind-palace"

[claude]
command = "claude"       # path to claude CLI
timeout = "5m"           # per-invocation timeout

[todoist]
enabled = true
api_token = ""           # or env: TODOIST_API_TOKEN
work_project_id = ""
personal_project_id = ""
inbox_project_id = ""

[cloudflare]
enabled = true
account_id = ""          # or env: CF_ACCOUNT_ID
api_token = ""           # or env: CF_API_TOKEN

[embeddings]
provider = "openai"              # "openai" | "ollama" | "none"
model = "text-embedding-3-small" # or ollama model name
api_key = ""                     # or env: OPENAI_API_KEY
# ollama_url = "http://localhost:11434"

[search]
backend = "chromem"              # "chromem" | "sqlite-fts" (auto-fallback if no embeddings)

[schedule]
inbox_interval = "30m"
daily_time = "06:00"
weekly_day = "sunday"
weekly_time = "09:00"

[export]
path = "~/.mind-palace/export"   # where research packs are generated
                                 # outside the vault — runtime artifact

[agents]
enabled = false                  # opt-in: requires agent runtime (ZeroClaw/mise)
path = ""                        # root of agent instances
                                 # e.g. ~/Local/chops/domains/personal/agents
sync_cadence = "weekly"          # when to push fresh packs to agent context/ dirs
```

---

## Embedding & Vector Search Strategy (Key Decision)

The current system uses local sentence-transformers (BAAI/bge-base-en-v1.5)
+ LanceDB via Python. Both need replacing. Two separate decisions:
**how to embed** and **where to store/search vectors**.

### Embedding Providers

| Option | Pros | Cons |
|--------|------|------|
| **OpenAI API** | Simple, $0.02/M tokens, best quality | API key required, network dependency |
| **Ollama local** | Free, local, user controls model | Requires Ollama installed separately |
| **ONNX Runtime** (`yalue/onnxruntime_go`) | Local, fast, same BGE model, no API | Requires ONNX model file download, dynamic lib loading |

### Vector Store Options (researched 2026-03-15)

#### Tier 1 — Best fit for single-binary Go

| Option | Type | CGO | Persistence | Maturity | Notes |
|--------|------|-----|-------------|----------|-------|
| **chromem-go** | Pure Go in-memory + file | No | File per doc, or gzip+AES export | Beta | Chroma-like API. Zero deps. Built-in OpenAI/local embedding support. 40ms queries at 100K docs. [github.com/philippgille/chromem-go](https://github.com/philippgille/chromem-go) |
| **sqlite-vec** (WASM bindings) | SQLite extension | No (via ncruces/go-sqlite3 WASM) | SQLite file | Prod-ready | HNSW index. SQL interface. Use with `ncruces/go-sqlite3` for pure Go. [alexgarcia.xyz/sqlite-vec](https://alexgarcia.xyz/sqlite-vec/go.html) |
| **coder/hnsw** | Pure Go HNSW | No | Manual (serialize) | Stable | ANN algorithm only — bring your own embeddings + persistence. [github.com/coder/hnsw](https://github.com/coder/hnsw) |

#### Tier 2 — Viable alternatives

| Option | Type | CGO | Notes |
|--------|------|-----|-------|
| **kelindar/search** | Pure Go brute-force | No | SIMD-optimized. Optional llama.cpp integration for local embeddings. <100K docs sweet spot. |
| **TFMV/hnsw** | Pure Go HNSW | No | Another pure Go HNSW implementation. |
| **gofaiss** | Pure Go FAISS port | No | Community port. Less mature. |
| **Bleve** | Pure Go FTS + vectors | No | Full-text search primary. Vector support exists but requires FAISS + zap v16. Over-engineered for us. |

#### Not viable for our use case

| Option | Why not |
|--------|---------|
| **OpenViking** (volcengine) | Context management for AI agents, not a vector DB. Polyglot (Python/Rust/C++). Go only for one subcomponent. |
| **zvec** (alibaba) | C++ embedded vector DB. Fast, but **no Go bindings** — Python and Node only. |
| **Weaviate embedded** | Python/TS only. Go client requires running server. |
| **Qdrant embedded** | Python only. Go client requires running server. |
| **Milvus Lite** | Python only. Go via REST/gRPC to server. |

### Recommendation

**Vector store: chromem-go** — zero dependencies, pure Go, no CGO,
file persistence, built-in embedding provider support. The beta status
is acceptable for our scale (<5000 vectors) and we can swap later if
needed since we'll wrap it behind a `Store` interface.

**Embeddings: layered approach**
1. `provider = "openai"` (default) — OpenAI `text-embedding-3-small`, simplest for distribution
2. `provider = "ollama"` — local via Ollama API, free, no network
3. `provider = "none"` — disable semantic search entirely (vault still works, FTS5 fallback)

**Fallback search: SQLite FTS5** — always available regardless of
embedding provider. Keyword search as baseline, vector search as upgrade.
Both behind the same `Store` interface.

### Config

```toml
[embeddings]
provider = "openai"              # "openai" | "ollama" | "none"
model = "text-embedding-3-small" # or ollama model name
api_key = ""                     # or env: OPENAI_API_KEY
# ollama_url = "http://localhost:11434"

[search]
backend = "chromem"              # "chromem" | "sqlite-fts" (auto-fallback if no embeddings)
```

---

## Stack Decisions (researched 2026-03-15)

Full analysis of Go library choices for every subsystem. Optimizing for:
zero CGO, single static binary, minimal deps, active maintenance.

| # | Subsystem | Choice | Over | Rationale |
|---|-----------|--------|------|-----------|
| 1 | CLI framework | **Kong** | Cobra | Struct-based, type-safe flags, less boilerplate. TOML config via kong-toml. Shell completions via kongplete. |
| 2 | YAML frontmatter | **gopkg.in/yaml.v3** (read) + **byte-level surgery** (write) | goccy/go-yaml | No YAML lib round-trips frontmatter perfectly. Parse with yaml.Node, modify via targeted string replacement, rejoin. Same pattern as Hugo/yq. |
| 3 | Markdown manipulation | **String operations** (line scanning) | goldmark AST | AST round-tripping is fragile. Line-by-line `## heading` scanning + regex wikilinks is more reliable for our ops (find section, insert after heading, extract between headings). |
| 4 | HTTP client | **hashicorp/go-retryablehttp** + **x/time/rate** | resty, req | Battle-tested thin stdlib wrapper. Auto retry + backoff. Rate limiter for API calls. Cloudflare polling is app-level (for loop + time.After). |
| 5 | Config | **koanf/v2** | Viper | 4x smaller binary (0.3 vs 9.4 MB overhead). Modular (only import TOML parser + env provider). Preserves key casing (Viper force-lowercases). |
| 6 | Interactive prompts | **charmbracelet/huh v2** | survey (archived), promptui | Beautiful, accessible (screen reader mode), actively maintained. Works standalone for init wizard, embeds into Bubble Tea if we add TUI later. |
| 7 | Logging | **log/slog** (stdlib) + **charmbracelet/log** (terminal handler) | zerolog, zap | Stdlib core. Pretty terminal via Charm handler. JSON handler for file logs. No perf-critical logging in a CLI tool. |
| 8 | Plist generation | **DHowett/go-plist** (`howett.net/plist`) | text/template | Pure Go, proper XML escaping/encoding. Build-tagged platform interface (`Scheduler { Install(); Uninstall(); Status() }`). No premature cross-platform abstraction. |
| 9 | Web crawling | **Keep Cloudflare /crawl** | colly, chromedp, rod | Zero local deps, JS rendering included. Colly is fallback for static pages if CF becomes a problem. Chromedp/rod destroy single-binary story (need Chrome). |
| 10 | YouTube transcripts | **Hand-roll InnerTube client** (~200 LOC) | horiagug/youtube-transcript-api-go (18 stars) | No mature Go library. Fetch video page → extract captions JSON → fetch timedtext XML. Stable endpoint. Python lib source as reference. |
| 11 | Todoist client | **Hand-roll with go-retryablehttp** (~100 LOC) | CnTeng/todoist-api-go | 3 endpoints (create task, complete task, list). Don't need a library for that. |
| 12 | Template embedding | **go:embed** (stdlib) | — | CLAUDE.md, slash commands, plist templates. Use `all:templates` for hidden files. |
| 13 | Testing | **stdlib testing** + **google/go-cmp** | testify | Table-driven tests, `httptest.NewServer` for HTTP, `fstest.MapFS` for filesystem, go-cmp for structural diffs. |
| 14 | Distribution | **goreleaser** + **creativeprojects/go-selfupdate** | ko | Cross-compile, GitHub releases, Homebrew tap auto-generation. Self-update: `mind-palace update` checks GitHub releases. |

### Final Dependency Set

```
github.com/alecthomas/kong        — CLI framework (struct-based)
github.com/knadh/koanf/v2         — Config (TOML + env + flags, modular)
gopkg.in/yaml.v3                  — YAML frontmatter parsing
github.com/hashicorp/go-retryablehttp — HTTP with retry + backoff
golang.org/x/time/rate            — Rate limiting for API calls
github.com/philippgille/chromem-go — Vector store (pure Go, no CGO)
modernc.org/sqlite                — SQLite FTS5 fallback (pure Go, no CGO)
github.com/charmbracelet/huh      — Interactive prompts (init wizard)
github.com/charmbracelet/log      — Pretty terminal logging (slog handler)
howett.net/plist                   — macOS plist generation (pure Go)
github.com/creativeprojects/go-selfupdate — Self-updating binary
github.com/google/go-cmp          — Test diffs (test only)
```

Zero CGO. Fully static binary. ~3-4 MB estimated.

---

## Phases

### Phase 0: Project Scaffold
**Goal:** Empty CLI that builds, runs, and has the command tree stubbed.

- [ ] `go mod init github.com/<user>/mind-palace`
- [ ] Kong CLI with root command + subcommands stubbed:
  - `mind-palace init` — interactive setup wizard
  - `mind-palace run inbox` — inbox pipeline
  - `mind-palace run daily` — daily pipeline
  - `mind-palace run weekly` — weekly pipeline
  - `mind-palace fetch` — hydrate stubs
  - `mind-palace fetch --retry` — retry failed crawls
  - `mind-palace sync todoist` — push todos
  - `mind-palace sync todoist --completions` — pull completions
  - `mind-palace index` — rebuild vector index
  - `mind-palace search <query>` — semantic search
  - `mind-palace signal [--time 15m]` — generate/view signal queue
  - `mind-palace audit links` — broken link scan
  - `mind-palace enrich [topic]` — topic enrichment
  - `mind-palace enrich --note <path>` — single note enrichment
  - `mind-palace profile [entity]` — entity profiling
  - `mind-palace export research` — generate research packs from assertions + rising themes
  - `mind-palace export research --theme <keyword>` — specific theme
  - `mind-palace export vault` — bulk vault export (domain bundles)
  - `mind-palace sync agents` — push packs to external agent context/ dirs
  - `mind-palace schedule install` — install LaunchAgents
  - `mind-palace schedule remove` — remove LaunchAgents
  - `mind-palace schedule status` — show schedule status
  - `mind-palace version`
- [ ] Config loading skeleton (`internal/config/`)
- [ ] Makefile with `build`, `install`, `test`, `lint` targets
- [ ] CI: GitHub Actions for build + test on push

**Estimated effort:** 1-2 sessions

---

### Phase 1: Markdown Engine
**Goal:** Read, parse, modify, and write Obsidian-flavored markdown with YAML frontmatter.

This is the foundation — every other module depends on it.

- [ ] `internal/markdown/frontmatter.go`
  - Parse YAML frontmatter (between `---` delimiters)
  - Read/write/update individual fields (string, []string, bool, date)
  - Preserve field order and comments where possible
  - Handle edge cases: no frontmatter, empty frontmatter, multiline values
- [ ] `internal/markdown/sections.go`
  - Find section by heading (e.g. `## Raw`, `## Transcript`, `## Research Context`)
  - Insert section before/after another section
  - Append content to end of file
  - Extract section content
- [ ] `internal/markdown/wikilinks.go`
  - Regex extract all `[[wikilinks]]` from content
  - Resolve wikilink to file path (case-insensitive stem matching)
  - Insert/remove wikilinks
- [ ] `internal/vault/vault.go`
  - `Vault` struct: root path, config
  - `Walk()` — iterate all .md files (with exclusion list)
  - `Read(path)` → `Note` struct (frontmatter + body + path)
  - `Write(note)` — serialize and write back
  - `Move(note, destDir)` — rename + move, update any references
  - `Discard(note, reason)` — move to _meta/discarded/ with reason
  - `NotesInDir(dir)` — list notes in a specific directory
  - `FindBySource(url)` — scan for notes with matching source URL (for dedup)
- [ ] `internal/vault/url.go`
  - URL normalization (strip tracking params, fragments, trailing slashes)
  - YouTube video ID extraction
- [ ] Tests for all of the above (table-driven, Go convention)

**Key libraries:**
- `gopkg.in/yaml.v3` — YAML parsing
- Standard library `regexp`, `os`, `path/filepath`

**Estimated effort:** 3-4 sessions

---

### Phase 2: HTTP Clients
**Goal:** Cloudflare /crawl, Todoist REST + Sync, YouTube transcripts, embeddings.

- [ ] `internal/crawl/client.go`
  - `Crawl(url string) (string, error)` — POST to /crawl, poll status, return markdown
  - Configurable poll interval (10s) and max attempts (60)
  - Timeout handling
- [ ] `internal/crawl/retry.go`
  - Read/write `_meta/crawl-failures.json`
  - Exponential backoff logic (6h → 24h → 72h → 168h, max 4 retries)
  - `RetryPending()` — process all entries past their backoff window
- [ ] `internal/todoist/client.go`
  - `CreateTask(title, projectID, dueString, description string) (taskID, error)`
  - REST v2 API: POST /tasks
  - Due string normalization: "5p"→"5pm", strip "before"/"by"
  - Obsidian deep link generation
- [ ] `internal/todoist/completions.go`
  - `FetchCompleted() ([]CompletedTask, error)` — Sync v9 API
  - Match completed tasks to vault notes by `synced-todoist` frontmatter
  - Update frontmatter: `done: true`, `completed-date: YYYY-MM-DD`
- [ ] `internal/youtube/transcript.go`
  - Fetch transcript via YouTube's timedtext API (no library needed,
    just HTTP + XML/JSON parsing — same approach as the Python lib)
  - Format as plaintext with timestamps
- [ ] `internal/embed/client.go`
  - Interface: `Embedder { Embed(text string) ([]float32, error) }`
  - `OpenAIEmbedder` — POST to /v1/embeddings
  - `OllamaEmbedder` — POST to /api/embeddings
  - `NoopEmbedder` — returns nil (search disabled)
- [ ] Tests with httptest mock servers

**Key libraries:**
- `net/http` (stdlib)
- `encoding/json` (stdlib)

**Estimated effort:** 3-4 sessions

---

### Phase 3: Pipeline Commands (Deterministic)
**Goal:** Port all non-AI scripts to Go commands.

These are the scripts that don't call Claude — pure file manipulation + HTTP.

- [ ] `mind-palace fetch` — port fetch-stub-content.py
  - Scan inbox/ for stubs (URL + clippings tag + <200 char body)
  - Dedup: normalize URLs, scan vault for existing sources
  - Hydrate via Cloudflare /crawl
  - Move duplicates to _meta/discarded/
- [ ] `mind-palace fetch --retry` — port retry logic
  - Read crawl-failures.json, retry entries past backoff
  - On success: find note by source URL (may have moved), hydrate in place
- [ ] `mind-palace sync todoist` — port sync-todoist.py
  - Scan notes/todos/ for notes without `synced-todoist` field
  - Route by tag: #work/#personal → project IDs
  - Create task, update frontmatter with task ID + URL
- [ ] `mind-palace sync todoist --completions` — port sync-todoist-completions.py
  - Fetch completed from Todoist
  - Match to vault notes, mark done
- [ ] `mind-palace signal` — port generate-signal.py
  - Scan for `consumed: false` notes
  - Score: assertion overlap (+3), rising topic (+2), recency (+1)
  - Estimate read time (250/150 WPM)
  - Bucket + format → signal/queue.md
- [ ] `mind-palace audit links` — port audit-links.py
  - Collect note stems, regex scan for [[links]], report broken
  - Output → _meta/broken-links.md
- [ ] `mind-palace youtube` — port fetch-youtube-transcript.py
  - Scan for YouTube source URLs without ## Transcript
  - Fetch + append transcript section

**Estimated effort:** 4-5 sessions

---

### Phase 4: Claude Integration
**Goal:** Wrapper for `claude -p` subprocess + commands that use it.

- [ ] `internal/claude/exec.go`
  - `Run(prompt string) (string, error)` — exec claude -p, capture stdout
  - Configurable command path and timeout
  - Set `CLAUDECODE=""` env var for nested invocation
  - Structured error handling (timeout, exit code, stderr)
- [ ] `mind-palace enrich [topic]` — port enrich-topics.py
  - Auto mode: read topic-frequencies.md, find topics ≥ threshold
  - Collect notes for topic, optionally crawl web
  - Call Claude for research brief generation
  - Write to notes/references/briefs/
  - Log to _meta/enrichment-log.md
- [ ] `mind-palace enrich --note <path>` — single note mode
  - Append Follow-up Questions, Related Materials, Book & Podcast Recs
- [ ] `mind-palace profile [entity]` — port profile-entities.py
  - Regex NER extraction (capitalized phrases, skip headings)
  - Claude pre-filter for false positives
  - Generate profile markdown
  - Write to notes/entities/
  - Log to _meta/entity-profiles-log.md
- [ ] Port summarize-transcripts.py logic
  - Called as part of daily pipeline, not standalone command
  - Detect transcripts (## Transcript) and web clips (≥500 chars)
  - Call Claude for ## Research Context generation
  - Auto-extract + merge Tier 2 domain tags

**Estimated effort:** 4-5 sessions

---

### Phase 5: Vector Search & Embeddings
**Goal:** Replace LanceDB + sentence-transformers with pure-Go embedded search.

See "Embedding & Vector Search Strategy" section above for full research.

- [ ] `internal/embed/embed.go`
  - Interface: `Embedder { Embed(text string) ([]float32, error); EmbedBatch(texts []string) ([][]float32, error) }`
  - `OpenAIEmbedder` — POST to /v1/embeddings (text-embedding-3-small)
  - `OllamaEmbedder` — POST to /api/embeddings (user-chosen model)
  - `NoopEmbedder` — returns nil (search falls back to FTS5)
  - Factory: `NewEmbedder(config) Embedder`
- [ ] `internal/search/store.go`
  - Interface: `Store { Index(notes []Note) error; Search(query string, topK int) ([]Result, error) }`
  - `ChromemStore` — primary backend via `chromem-go`
    - Pure Go, zero CGO, zero external deps
    - File persistence (one file per doc in `_rag/chromem/`)
    - Cosine similarity, built-in embedding provider support
  - `FTSStore` — fallback via SQLite FTS5 (`modernc.org/sqlite`)
    - Always available, no embeddings needed
    - Keyword search, BM25 ranking
    - Stored in `_rag/fts.db`
  - `HybridStore` — wraps both, merges results (vector + keyword)
- [ ] `mind-palace index` — rebuild index
  - Walk vault (same exclusions: _rag, _meta, templates, .git, .obsidian, signal, bloom)
  - Embed: title + tags + body[:2000] via configured Embedder
  - Upsert to ChromemStore + FTSStore in parallel
  - Report: indexed N notes, skipped M
- [ ] `mind-palace search <query>` — semantic + keyword search
  - Embed query if embeddings configured, else FTS5 only
  - Return top-k with scores, pretty-print to terminal
  - Flag which backend produced each result
- [ ] MCP server (stretch goal — can come later)
  - Expose `search_vault` tool over MCP protocol
  - Register as project-scoped MCP server for Claude Code
  - Can reuse same Store interface

**Key libraries:**
- `github.com/philippgille/chromem-go` — vector store (pure Go, no CGO)
- `modernc.org/sqlite` — FTS5 fallback (pure Go, no CGO)

**Fallback chain:** chromem (vector) → FTS5 (keyword) → disabled (no search)

**Estimated effort:** 2-3 sessions

---

### Phase 6: Chain Orchestration
**Goal:** Wire individual commands into the inbox/daily/weekly pipelines.

- [ ] `internal/pipeline/runner.go`
  - `Pipeline` struct: ordered list of steps, each with name + function
  - Run steps sequentially, log each step, abort on error
  - Support `--dry-run` flag (print what would happen)
- [ ] `internal/pipeline/markers.go`
  - Read/write date markers (_meta/.daily-last-run, .weekly-last-run)
  - Daily: skip if already ran today
  - Weekly: skip if same ISO week, only fire Sun/Mon
- [ ] `mind-palace run inbox` — inbox pipeline
  1. fetch (hydrate stubs + dedup)
  2. claude "process my inbox"
  3. youtube (fetch transcripts)
  4. index (rebuild search)
  5. sync todoist (push)
  6. daily catchup (if not today)
  7. weekly catchup (if Sun/Mon + not this week)
- [ ] `mind-palace run daily` — daily pipeline
  1. fetch --retry
  2. summarize (enrich transcripts + web clips via Claude)
  3. index
  4. claude "daily synthesis"
  5. signal (generate queue)
  6. sync todoist (push)
  7. sync todoist --completions (pull)
- [ ] `mind-palace run weekly` — weekly pipeline
  1. fetch --retry
  2. claude "weekly synthesis"
  3. index
  4. signal
  5. audit links
  6. enrich (auto mode)
  7. profile (auto mode)
  8. export research (generate packs for top themes)
  9. sync agents (if [agents].enabled — push packs to agent dirs)
- [ ] All pipelines: structured logging to _meta/processing-logs/

**Estimated effort:** 2-3 sessions

---

### Phase 7: Export & Agent Sync
**Goal:** Generate research packs from vault themes; optionally sync to external agents.

This is the one-way membrane between the vault and external agent runtimes.
The vault never knows about the agents. The agents never write to the vault.
Research packs are read-only snapshots that agents consume.

- [ ] `internal/export/themes.go`
  - Auto-detect strong/emerging themes from:
    - Active assertions (highest priority — user declared intent)
    - Rising topics in topic-frequencies.md (count + trend)
    - New topic clusters
  - Configurable: `--top N`, `--theme <keyword>`, `--assertion <path>`
- [ ] `internal/export/pack.go`
  - For each theme, generate a self-contained research pack:
    - `context.md` — user's thinking: ideas, assertion evidence, related refs
      (Research Context sections preferred, truncated for size)
    - `sources.md` — source URLs grouped by content type + consumption state
      (unconsumed = high-value targets for agents)
    - `brief.md` — gap analysis: ideas-vs-refs ratio, open questions from
      assertions + MOCs + briefs, suggested research angles
    - `agent-prompt.md` — ready-to-use prompt for an agent persona
  - Respects size limits (configurable, default ~500K per file)
  - Strips noisy sections (## Raw, etc.)
  - Separates ideas from references (user thinking vs. collected material)
- [ ] `internal/export/personas.go`
  - Pluggable agent personas that shape brief.md and agent-prompt.md:
    researcher, teacher, coach, critic, connector (or user-defined)
  - `--all-personas` generates a prompt variant per persona
  - Persona is a template — doesn't assume any specific agent runtime
- [ ] `internal/export/delta.go`
  - Track what changed since last export per theme
  - Delta manifest: new notes, new evidence, new questions, closed gaps
  - Allows agents to efficiently re-ground instead of re-reading everything
- [ ] `mind-palace export research` — generate packs to `[export].path`
- [ ] `mind-palace export vault` — bulk domain bundles (atlas MOC-based)
- [ ] `internal/agentsync/sync.go`
  - Read `[agents]` config: path to agent instances root
  - For each theme with a matching agent dir, copy/update context/ files
  - No assumption about agent runtime — just file placement
  - Log what was synced to _meta/agent-sync-log.md
- [ ] `mind-palace sync agents` — push packs to agent context/ dirs
- [ ] Weekly pipeline integration: export + sync as final steps

**Python spec (current prototype):**
- `_scripts/export-research-packs.py` — working prototype of pack generation
- `_scripts/export-notebooklm.py` — working prototype of bulk vault export

**Estimated effort:** 2-3 sessions

---

### Phase 8: Installer & Config Wizard (was Phase 7)
**Goal:** `mind-palace init` sets up everything interactively.

- [ ] `mind-palace init`
  - Detect platform (macOS, Linux)
  - Prompt for vault path (default: detect Obsidian vaults, or iCloud path)
  - Validate vault path exists, create directory structure if empty
  - Prompt for API keys:
    - Todoist (optional): API token, project IDs
    - Cloudflare (optional): account ID, API token
    - Embeddings: OpenAI key, or Ollama URL, or "none"
  - Write `mind-palace.toml` to vault root
  - Generate `.claude/CLAUDE.md` from embedded template
  - Generate `.claude/commands/*.md` from embedded templates
  - Create vault directories if missing (inbox/, notes/*, atlas/, etc.)
  - Offer to install scheduler (`mind-palace schedule install`)
- [ ] `mind-palace init --non-interactive` — accept all from flags/env
- [ ] Templates embedded via `embed.FS` (Go 1.16+)

**Key library:**
- `github.com/charmbracelet/huh` — beautiful interactive forms
- Or `github.com/AlecAivazis/survey/v2` — simpler alternative

**Estimated effort:** 2-3 sessions

---

### Phase 9: Scheduler Management (was Phase 8)
**Goal:** Generate and manage LaunchAgents (macOS) from the CLI.

- [ ] `internal/scheduler/launchd.go`
  - Generate plist XML from config (schedule times, vault path, binary path)
  - Template: StartCalendarInterval (not StartInterval — it's broken on Sequoia)
  - Set PATH in EnvironmentVariables
  - Set StandardOutPath / StandardErrorPath for logging
- [ ] `internal/scheduler/cron.go` (future — Linux support)
  - Generate crontab entries
  - Install/remove via `crontab -l` / `crontab -`
- [ ] `mind-palace schedule install`
  - Write plist files to ~/Library/LaunchAgents/
  - `launchctl load` each one
  - Verify with `launchctl list | grep mindpalace`
- [ ] `mind-palace schedule remove`
  - `launchctl unload` each agent
  - Remove plist files
- [ ] `mind-palace schedule status`
  - Check if agents are loaded, show next fire time
  - Show marker file dates (last daily, last weekly run)

**Estimated effort:** 2 sessions

---

### Phase 10: Distribution (was Phase 9)
**Goal:** Users can install with one command.

- [ ] `goreleaser.yml` — build for darwin/arm64, darwin/amd64, linux/arm64, linux/amd64
- [ ] GitHub Releases — goreleaser publishes binaries on tag push
- [ ] Homebrew tap:
  - `brew tap <user>/mind-palace`
  - `brew install mind-palace`
- [ ] `go install github.com/<user>/mind-palace/cmd/mind-palace@latest`
- [ ] README with install instructions + quickstart
- [ ] Demo GIF/video (optional but helpful for adoption)

**Estimated effort:** 1-2 sessions

---

## Migration Path (for your existing vault)

1. Build the Go binary (phases 0-6)
2. Run `mind-palace init` pointing at existing vault
3. Verify: `mind-palace run inbox --dry-run` matches current behavior
4. Run real pipelines alongside Python for a few days, compare outputs
5. Once confident: `mind-palace schedule install` (replaces old LaunchAgents)
6. Remove _scripts/, _rag/, .agents/, AGENTS.md from vault — they now live
   in the Go binary or its repo. The vault becomes pure markdown.
7. Python/pip/venv no longer needed
8. Optionally: configure [agents] in mind-palace.toml to enable export/sync
   to external agent runtimes (ZeroClaw/mise)

## Dependencies (Go modules)

```
github.com/spf13/cobra          — CLI framework
github.com/spf13/viper          — Config (TOML, env, flags)
gopkg.in/yaml.v3                — YAML frontmatter
github.com/BurntSushi/toml      — TOML config file
github.com/charmbracelet/huh    — Interactive prompts (init wizard)
github.com/charmbracelet/log    — Structured logging
github.com/philippgille/chromem-go — Vector store (pure Go, no CGO)
modernc.org/sqlite              — Pure Go SQLite for FTS5 fallback (no CGO)
```

No CGO required. Fully static binary.

## Total Estimated Effort

| Phase | Sessions | Lines (est.) |
|-------|----------|-------------|
| 0. Scaffold | 1-2 | ~300 |
| 1. Markdown engine | 3-4 | ~800 |
| 2. HTTP clients | 3-4 | ~600 |
| 3. Pipeline commands | 4-5 | ~900 |
| 4. Claude integration | 4-5 | ~700 |
| 5. Vector search | 2-3 | ~400 |
| 6. Chain orchestration | 2-3 | ~400 |
| 7. Export & agent sync | 2-3 | ~600 |
| 8. Installer | 2-3 | ~500 |
| 9. Scheduler | 2 | ~300 |
| 10. Distribution | 1-2 | ~100 (config) |
| **Total** | **~27-33 sessions** | **~5,600 lines** |

## Open Questions

- [ ] **Repo name?** `mind-palace`, `mind-palace-cli`, `mp`, `vault-engine`?
- [ ] **Embedding provider default?** OpenAI (simple, cheap) vs Ollama (free, local)?
- [ ] **Do we keep the MCP server?** Or defer it — `mind-palace search` from CLI may be enough
- [ ] **Todoist optional?** Any other task managers to support? (Notion, Linear, etc.)
- [ ] **Should `mind-palace init` scaffold a fresh CLAUDE.md** or ship a default and let users customize?
- [ ] **GitHub org?** Personal repo or create an org for it?
