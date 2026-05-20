# Product Vision

> References: [go-rewrite-plan.md](go-rewrite-plan.md), [system-architecture.md](system-architecture.md)
> Suite context: `~/Local/chops/.agents/SUITE_VISION.md` — how mind-palace fits as a standalone tool in the broader agent ecosystem (chops platform, tempor planning, zeroclaw orchestration). Mind-palace is independent; the suite doc captures the composed vision.

## The Idea

A second brain that grows with you. Not a platform. Not social. Not algorithmic.
Audience of one.

The more you feed it — clips, voice memos, todos, stray thoughts — the more
it mirrors back. Connections you didn't see. Patterns in what you care about.
Questions you haven't asked yet. The flywheel output is the product: not the
tool, but what emerges from sustained use. Like Sublime or Are.na for stashing
culture and energy, but private, personal, and shaped by your own thinking
rather than a feed.

The "Her" insight isn't about conversational AI. It's that a system which
genuinely knows what you've been thinking about becomes compelling — almost
addictive — to use. A Tamagotchi for your intellectual life. The mirror
is the draw, not the assistant.

## Three Layers

### Layer 1: Core Engine (Go CLI)

The rewrite. Single binary. Personal tool. No account, no server, no platform.

```
brew install mind-palace
mind-palace init ~/my-vault
```

- Capture → triage → synthesize → enrich → connect
- Claude for AI reasoning, everything else deterministic
- BYOK (bring your own key) for Claude/OpenAI/Ollama
- Obsidian-compatible but not Obsidian-dependent (any markdown editor works)
- Plugin-shaped: works standalone OR as a module inside agent orchestrators

**Milestone:** Dogfood the Go rewrite for 30 days on our own vault before
anything else. The tool has to be good for us before it's good for anyone.

See: [go-rewrite-plan.md](go-rewrite-plan.md)

### Layer 2: The Engine

The closed loop: **vault ↔ logic ↔ frontend (Obsidian).**

No external API surface. No MCP server for other agents. No REST API.
This is a party of one. The vault is private, the logic is local, the
frontend is Obsidian. The tool is reusable (install it, point it at
your vault), but the running instance is not — it doesn't serve
external consumers.

```go
vault := mindpalace.Open("~/my-vault")
results := vault.Search("context engineering", 5)
vault.Ingest(note)
vault.Synthesize()
assertions := vault.Assertions()
signal := vault.Signal(15 * time.Minute)
```

The Go package is the API. Import it if you're building on top of it.
But the default mode is a single binary that owns the vault lifecycle.

Note: the current Python prototype uses an MCP server (`_rag/mcp_server.py`)
as scaffolding — Claude Code needs it to call vault search during sessions.
This is a pragmatic hack, not architecture. The Go binary won't carry it
forward as a design surface.

Model-agnostic — AI calls are behind interfaces. Claude is the default.
Swap in GPT, Gemini, Ollama, whatever. The engine doesn't care.

#### Landscape Context

The agentic orchestration space is moving fast. For context on where
this fits (not as integration targets, but to understand the terrain):

- **Agent orchestrators:** OpenClaw ecosystem (*claw variants), GasTown,
  Paperclip, Antfarm, Symphony, CrewAI, AutoGen, LangGraph, etc.
- **Agent memory layers:** Mem0, Letta, Zep, LangMem, ACE
- **Coordination protocols:** A2A (Google), ACP, ANP, Summoner
- **Skills frameworks:** AGENTS.md convention, Anthropic Skills, BMAD Method

Mind-palace occupies the **knowledge accumulation and synthesis** niche.
Not task orchestration, not agent routing, not tool execution. The thing
that remembers what you've been thinking about and tells you what you're
missing. Any of the above could use it as a memory layer. Or not. It
stands alone just fine.

Sources: [rywalker.com/research](https://rywalker.com/research),
[awesome-agent-orchestrators](https://github.com/andyrewlee/awesome-agent-orchestrators)

#### Export as Interface — The Agent Membrane

The runtime generates **research packs** — read-only snapshots of vault
themes (assertions, ideas, evidence, open questions, source URLs). External
agents consume these packs without touching the vault. The vault never
knows about the agents. User clips valuable output back through the normal
inbox pipeline.

```
mind-palace (runtime)          external agents (ZeroClaw, etc.)
─────────────────────          ────────────────────────────────
captures, triages, connects    researches, teaches, coaches
YOUR thinking                  extends tentacles outward
pristine, never touched        operates on exported snapshots
         │
         ▼
   research packs
   (read-only, weekly cadence)
         │
         ▼
   agent/context/              ← packs land here via `mind-palace sync agents`
   agent/memory/               ← agent accumulates its own findings
   agent/soul.md               ← agent identity (persona, values, boundaries)
```

Each theme-tentacle is its own agent instance with its own persistence,
its own rhythm, its own accumulated state. The governance researcher
deepens its understanding of DAOs and liveness over weeks. The FP teacher
builds out a curriculum. None of this touches the vault.

The sync cadence (weekly, matching synthesis) creates a compounding loop:
weekly synthesis detects what's rising → exports packs → agents work all
week → user reviews output → clips valuable findings → next synthesis
incorporates them → updated pack → agents re-ground → cycle continues.

Research packs are the **interface contract** between the vault and any
external consumer. They don't assume ZeroClaw, mise, or any specific
runtime. An agent, a notebook tool, a fresh Claude conversation, or a
human reading markdown — all valid consumers of the same pack format.

### Layer 3: Interactive Experience (App / TUI / Web)

The warm layer. What makes it feel alive.

Inspiration: Kat the Poet Engineer's digital garden — minimal, futuristic,
warm. Not a dashboard. Something you want to look at.

- **Knowledge graph visualization** — watch connections form over time.
  Animated, organic, like watching something grow.
- **Interactive reflect** — pull on a thread, see related notes surface,
  write inline, watch the graph update in real time.
- **Signal queue** — curated reading experience, not a link list.
  "15 minutes. Here's what matters and why."
- **Assertion dashboard** — goals with evidence accumulating visually.
- **Time-lapse** — scrub through vault history. The archaeology of your
  own attention.
- **Search that feels like thinking** — "notes near this thought" not
  "12 results for query."

Desktop/mobile parity for reading and reflecting. Capture is mobile-first.
Synthesis is desktop-first.

Tech TBD: Bubble Tea TUI (power users) + Tauri (warm experience), or web.

## The Addiction Loop

```
Capture (low effort, mobile)
    ↓
Triage (automatic, fast)
    ↓
Connections form ("oh, that links to that?")
    ↓
Provocations arrive ("you've been collecting but not thinking")
    ↓
Original thinking happens (reflect, assertions)
    ↓
The graph grows (visible, tangible, yours)
    ↓
You want to capture more
```

The output is surprising. Not "here's a summary" — boring. But "you've
clipped 8 articles on agent orchestration and haven't written a single
original thought — what do you actually believe?" That's the mirror.

## What This Is Not

- **Not a platform.** No social. No sharing. No feed. Audience of one.
- **Not an AI assistant.** A mirror, not a butler. Provocations, not summaries.
- **Not a note-taking app.** The engine underneath the editor.
- **Not creepy.** All data local. No telemetry. You own your vault.

## Revenue (Eventually)

Dogfood first.

- **Core CLI:** free, open source, always.
- **iOS capture app:** paid ($10 one-time?). Replaces the Shortcuts hack.
- **Layer 3 app:** paid or subscription. The warm visual experience.
- **Never:** data, telemetry, ads, platform fees.

## Sequencing

```
Now         → Dogfood Go rewrite (30 days)
Month 2-3   → Layer 1 public: open-source CLI, brew install
Month 3-4   → Layer 2: plugin API, test with *claw orchestrators
Month 4-6   → Layer 3 prototype: TUI or Tauri
Month 6+    → iOS capture app (if demand)
```

## Open Questions

- [ ] Product name
- [ ] License: MIT (max adoption) vs AGPL (protect against cloud wrappers)?
- [ ] iOS tech: SwiftUI native or cross-platform?
- [ ] Layer 3 tech: Bubble Tea, Tauri, web, or all?
- [ ] How much of CLAUDE.md becomes configurable vs opinionated defaults?
- [ ] Obsidian plugin for bidirectional sync?
- [ ] Which integration surface first: Go package, MCP, REST?
- [ ] Should the engine's memory model align with emerging conventions
      (Mem0, Letta, AGENTS.md standard) or stay opinionated?
