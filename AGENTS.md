# Sea Noise

Parity-first Go port of the Python "mind-palace" prototype. A local retrieval and context-assembly runtime over a markdown vault. Single static binary, CGO disabled, no Python in shipped output.

This file is the team-shared agent entry point. `CLAUDE.md` imports it via `@AGENTS.md` so Claude Code picks up the same content. Personal/IC-local agent context lives under `.agents/local/` (gitignored) and **supplements** — never replaces — what's here.

## Where things live

- **Architecture** — [ARCHITECTURE.md](./ARCHITECTURE.md): retrieval substrate, vault SQL schema, retrieval router interface, Embedder interface, inference router seam, hard binary constraints, fallback chain. Authoritative on design.
- **Python prototype reference** — [`references/scripts/`](./references/) and `references/scripts/_rag/`: the canonical specification for parity. When ported behavior is ambiguous, run the Python script against the fixture vault and match its output.
- **Project docs** — [`docs/`](./docs/): notes accrued during planning (migration, language/product shape, etc.). Reference material, not load-bearing.

## Repo layout for agent infra

Two parallel trees, each split into `shared/` (checked in) and `local/` (gitignored):

- `.agents/shared/` — agent-neutral commons: rule-skills, loader, metrics. Consumed by any runtime.
- `.agents/local/` — IC-personal agent-neutral stuff (metric buffer, drafts). Gitignored.
- `.claude/shared/` — Claude-specific shared wiring (the adapter hook). Thin wrapper around `.agents/shared/`.
- `.claude/` (root) — Claude Code's required fixed paths: `settings.json` (checked in), plus `commands/` (checked in for now while we're using Tempor bootstrap skills).

## Rule-skills

Narrow, prescriptive guardrails scoped by file path. Each rule fires only when the file you're editing matches its `globs` field. The PreToolUse hook at `.claude/shared/hooks/inject-rules.mjs` injects the matching rules as context.

Rules live at `.agents/shared/skills/rules/*.md` — always checked in, never per-IC.

### Existing rules

- [`test-discipline`](./.agents/shared/skills/rules/test-discipline.md) — fires on `**/*_test.go`. Eight rules for non-brittle, composable Go tests.

### Adding a new rule

See [`.agents/shared/skills/rules/README.md`](./.agents/shared/skills/rules/README.md). TL;DR: one markdown file with frontmatter (`name`, `description`, `globs`, `kind: rule`); no hook changes needed; the loader picks it up automatically.

### Inspecting hook output

```sh
echo '{"tool_name":"Edit","tool_input":{"file_path":"'"$PWD"'/internal/config/config_test.go"}}' \
  | node .claude/shared/hooks/inject-rules.mjs
```

Empty output = no rules matched. JSON with `additionalContext` = those rules will be injected.

## Build, run, test

```sh
./scripts/build.sh          # CGO_ENABLED=0 go build ./...
./scripts/test.sh           # go test ./...
./scripts/grep-gates.sh     # architectural invariants (no model literals, no Python, no MCP)
node --test .agents/shared/hooks/*.test.mjs .claude/shared/hooks/*.test.mjs   # rule-injector tests
```

Vault path is resolved from `MIND_PALACE_VAULT` env var or `vault.path` in `mind-palace.toml`. Env var wins.

## Commit discipline

- **One-line subjects.** No body. No Co-Authored-By trailer. Under ~60 chars.
- **Declarative and imperative about source/infra.** Never reference Tempor planning vocabulary (no `Phase`, `Wave`, `T01`, `kickoff`, `lookahead`, `v8 plan`, etc.).
- **Subject describes what's changing in the codebase.** Use prefixes when they help (`init:`, `add:`, `fix:`, `refactor:`, `chore:`).
- **Explanation belongs in code/docs, not in commit messages.** If a change needs a "why," put it in a comment, the relevant kickoff, or the architecture doc.

Examples: `add: ARCHITECTURE.md`, `fix: sentinel errors in config; tests use errors.Is`, `chore: gitignore .agents/local`.

## Parity-first discipline

Until parity against the Python prototype is demonstrated on the golden fixture vault, the rewrite does not sharpen, redesign, or simplify the prototype's behavior. Authority precedence when sources conflict:

1. Current Python runtime behavior (canonical specification)
2. Prompts in `references/prompts/`
3. Docs in `references/docs/`

Parity is judged behaviorally — `go test` diffs Go output against captured Python output on the fixture vault. There are no metric thresholds (no Recall@K, MRR, latency). See ARCHITECTURE.md §9 and §11.

## Product positioning

Sea Noise is end-user runtime software, not a developer SDK or agent-harness component. The vault lives on the user's machine; LLM access is whatever the user already has. The runtime stays agent-agnostic — does not couple to any specific agent harness — and the transport layer is structured so future plurality (other CLIs, direct APIs) is a natural extension after parity.

## Worktree-first orchestration

Per `orchestration_contract.parallelism_model = worktree-first` in the v8 plan: each track gets its own git worktree under `.worktrees/<track-id>/`. Branch convention is `swarm/<track-id>`. Worktrees are gitignored at the repo level so the main tree stays clean.
