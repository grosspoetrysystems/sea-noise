---
title: "Governance, Liveness, and the Agentic Era"
tags:
  - assertion
  - tech/ai-agents
  - tech/context-engineering
  - tech/software-design
  - people/venkatesh-rao
date: 2026-03-12
status: active
source: voice-capture
---

## Assertion

I want to build a public speaking presentation exploring governance and DAOs through the lens of design and AI — focusing on incentives, liveness, and what those concepts mean in the agentic era, both passively and as applied practice for the human in the loop.

## Why

This sits at the intersection of my working identity: front-end engineer, product engineer in AI and crypto, design practitioner. The liveness concept from Venkatesh Rao and the emerging frame of intent engineering feel like they're converging. Governance — spanning data, privacy, finance, community, politics, globalization — is the umbrella where these threads meet. Success looks like a coherent talk that maps how AI and coordination coalesce under governance across all those domains.

## Evidence
- [[2026-03-12-ai-voting-proxies-private-governance]] — AI voting proxies and private governance
- [[2026-03-13-invisible-castles]] — invisible castles as compound agent systems
- [[2026-03-12-engineering-liveness]] — Venkatesh Rao on engineering liveness
- [[2026-03-12-ai-tool-sprawl-orchestration]] — personal experience of agentic tool sprawl and the need for orchestration/governance across accounts and workflows
- [[2026-03-16-systems-thinking-before-analytical-thinking]] — systems thinking (feedback loops, stakeholder context) must precede analytical execution — directly applicable to structuring the governance talk
- [[2026-03-17-how-complexity-accumulates]] — systems thinking essay on how complexity creeps in through locally rational decisions — procedural layering, workarounds, and technical debt. Directly relevant to how governance complexity accumulates in organizations and DAOs
- [[2026-03-17-ai-adoption-lessons-from-history]] — MIT research on AI adoption: trust-driven adoption, human-in-the-loop design principles, the gap between tool capability and organizational transformation — maps to the "applied practice for the human in the loop" dimension
- [[2026-03-18-visa-agentic-commerce-cli]] — agentic commerce arms race: Visa, Stripe, Mastercard, Circle all shipping AI agent payment infrastructure simultaneously. Concrete governance questions: authorization models, agent liability, KYC/AML for agents, trad-fi trust layers vs. crypto-native rails. Maps to "governance in finance" dimension
- [[2026-03-20-agent-zero-framework]] — operational framework for AI-assisted development with explicit approval gates, state machine (PLAN→BUILD→DIFF→QA→APPROVAL→APPLY→DOCS), and "reuse over creation" governance principles. Concrete example of human-in-the-loop governance in an agentic dev context
- [[2026-03-22-secure-ai-agent-api-credentials]] — credential vault pattern, proxy-based credential injection, access policies, and human-in-the-loop approval for sensitive agent actions. Concrete governance infrastructure: least privilege, audit trails, per-agent RBAC. Complements the MCP security argument on auth identity chains
- [[2026-03-25-ai-constitution-claude]] — Jill Lepore on Claude's Constitution as product of constitutional democracy's failure: AI governance filling the vacuum left by state abdication. Amanda Askell's shift from rules to virtues. Democratic legitimacy deficit of corporate AI constitutions
- [[2026-03-25-posthog-building-ai-agents]] — PostHog's 3 harness iterations: from coordinator routing to single-agent modes to Claude Agent SDK + MCP. Context engineering as governance layer. "Traces hour" as human-in-the-loop observability ritual
- [[2026-03-25-karpathy-autoresearch-repo]] — program.md as "research org code" — governance of autonomous agents through natural language instructions, not code constraints. Concrete example: agent-as-researcher with fixed evaluation metric as the only guardrail
- [[2026-03-27-agentorchestra-tea-protocol]] — TEA protocol as unified governance layer above MCP/A2A: explicit lifecycle management, versioned interfaces, protocol transformations enabling dynamic role reconfiguration. Hierarchical orchestration as governance pattern — central planner delegates to specialized sub-agents with localized tool ownership, keeping decision scope bounded. Self-evolution module treats components as evolvable variables, raising questions about who governs the evolution loop
- [[2026-03-30-hegelian-dialectic-skill]] — Claude Code skill grounded in Venkatesh Rao's Electric Monks framework (belief offloading), Boyd's destructive deduction/creative induction, and Alexander's semi-lattice theory. Structured dialectical thinking as a governance-of-reasoning tool: two committed agents debate, orchestrator decomposes and synthesizes. Directly applicable to the talk's method — and potentially to its content (governance as dialectical process)
- [[2026-03-31-opencode-workflow-senior-engineer]] — practical multi-agent team orchestration: team lead delegates to product manager, backend dev, tester, code reviewer sub-agents. Credential security via JIT tokens (Descope). Real-world friction report: "95% okay", constant prompt tuning needed. Concrete case study for the "applied practice for human in the loop" dimension
- [[2026-03-31-worktrunk-git-worktree-cli]] — infrastructure for parallel AI agent workflows: git worktree management enabling 5-10+ concurrent agents. Hooks, merge workflows, and CI integration as coordination governance at the tooling layer
- [[2026-04-19-12-agentic-harness-patterns]] — Bilgin Ibryam catalog of 12 reusable agentic harness patterns observed in Claude Code. Patterns are the unit of governance composition at the harness layer (stub awaiting hydration)
- [[2026-04-27-ai-sandboxes-engineercodex]] — sandboxing primitives every dev should know for AI agents. Direct safety/governance infrastructure: bounded execution environments as the precondition for trustworthy autonomy (stub awaiting hydration)
- [[2026-04-30-agentic-harness-engineering-arxiv]] — arXiv: observability-driven automatic evolution of coding-agent harnesses. Self-evolving governance — the harness itself as a learning artifact, not a fixed scaffold (stub awaiting hydration)
- [[2026-05-05-charlie-daemons-vs-agents]] — Charlie Labs frames DAEMON.md as in-repo policy contract: watch/routines/deny/schedule semantics. "Operational debt" as the governance category that agents create and daemons pay down. Role-based (continuous) vs. task-based (discrete) autonomy
- [[2026-05-06-how-charlie-daemons-work]] — daemon execution model: 7-stage loop (intake→context→plan→execute→verify→report→continue) gated by an autonomy/approval matrix. Routine read-only work runs automatic; scope-expanding actions hit human approval. DAEMON.md as versioned policy-as-code analog to OPA/Sentinel for agents
- [[2026-05-07-zed-parallel-agents]] — Zed's Threads Sidebar for parallel agent orchestration in one editor window. Per-thread folder/repo access control as governance primitive at the IDE layer. "Agentic engineering" (Nathan Sobo) as middle position between vibe-coding and AI-purism — relevant framing for the human-in-the-loop dimension
- [[2026-05-08-rtk-cli-llm-token-proxy]] — single Rust binary that intercepts dev commands and trims output before LLM, claiming 60-90% token reduction. Context governance / cost discipline at the tooling layer
- [[2026-05-15-ai-daemons-engineering-maintenance]] — Riley Tomasek's manifesto for "daemons" as a new AI category alongside agents. Tasks vs roles distinction is the governance primitive. Agents create work; daemons maintain it. Daemon-as-flywheel: zero marginal attention, learned norms, earned trust, predictable behavior earning autonomy
- [[2026-05-15-structured-prompt-driven-development]] — Thoughtworks' SPDD: prompts as first-class delivery artifacts (versioned, reviewed, reused). REASONS Canvas (Requirements/Entities/Approach/Structure/Operations/Norms/Safeguards) as governance schema for AI-generated code. Thesis: AI-driven local speed doesn't translate to system-level throughput without governance — directly applicable to the talk's "human in the loop" dimension

## Open Questions

- How do incentives and liveness connect formally in DAO governance?
- What does "intent engineering" mean concretely for human-in-the-loop agentic systems?
- How does Venkatesh Rao's liveness framework translate to applied design practice?
- What are the passive vs. applied dimensions of liveness in agentic contexts?
- How do data/privacy, finance, community, and political governance share structural patterns?

## Raw

I want to work on building a public speaking presentation around governance and DAOs with design and AI that focuses on incentives and liveness and what that means in the agentic era, both in the passive and applied sense for the human in the loop. I'm curious to explore this in general as a design practice, as a front-end engineer, as a product engineer working in AI and crypto. I'm excited to think about that and I think a lot about the liveness piece from Venkatech Rao. I think a lot about intent engineering now.

What are these kinds of words and how do they coalesce as we try to use AI and coordination broadly under the auspice of governance in all of its myriad applications, from data and privacy and finance and community to modern day politics and globalization?
