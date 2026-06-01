---
title: "Research Brief: agent-orchestration"
tags:
  - reference
  - reference/brief
  - research-brief
  - agent-orchestration
date: 2026-03-16
source: enrich-topics
---

# Research Brief: agent-orchestration

Auto-generated enrichment for topic cluster **agent-orchestration** (1 notes, threshold crossed).

## Vault Notes in This Cluster
- [[2026-03-11-tmux-agent-orchestration]]

Now I have comprehensive context from the vault and current research. Let me generate the brief.

### Follow-up Questions

1. **What's the right granularity for agent delegation?** Your tmux note hints at workspace management, but the real question is: when you split work across parallel Claude Code agents, what's the minimum viable context each agent needs to avoid merge conflicts and redundant work? The Meta engineer tips note says sub-agents are best for "atomic, isolated work" — where's the boundary between atomic and coupled?

2. **Orchestrator-as-code vs. orchestrator-as-agent?** Claude Code Agent Teams use a git-based coordination model where agents claim tasks. OpenAI Symphony routes from a Linear board. These are fundamentally different philosophies — declarative task boards vs. emergent agent negotiation. Which breaks down first at scale, and for what kinds of work?

3. **How do you verify agent-generated code when you're running 5+ agents in parallel?** Datadog's formal verification (TLA+, Kani proofs) works for isolated optimization tasks. But your vault has no notes on verification strategies for *orchestrated* multi-agent output where agents' changes interact. What does a "merge harness" look like?

4. **What's the governance layer between agent autonomy and human oversight?** Your assertion on governance/liveness in the agentic era touches this obliquely — but concretely, when you have 20 agents running via `claude_code_agent_farm`, who approves what? GitHub Agent HQ is betting on centralized guardrails. Is that the right abstraction, or does it recreate the bottlenecks agents are supposed to eliminate?

5. **Is MCP sufficient as the coordination protocol for agent-to-agent communication?** Garry Tan's criticism (context bloat, poor auth) and the emerging A2A protocol from Google suggest MCP was designed for human-to-agent tool access, not agent-to-agent orchestration. What's the protocol layer that's actually missing?

6. **What happens to developer skill when orchestration abstracts away implementation?** Your "AI code comprehension crisis" note flags shipping code you don't understand. Scale that to orchestrating agents writing code you don't understand across 10 parallel sessions. Is the meta-skill "agent management" or is it something more specific — like writing better CLAUDE.md files and test harnesses?

7. **Where does agent memory fit in orchestration?** Mem0 and OpenViking solve per-agent memory, but orchestrated systems need *shared* memory — what one agent learned should inform another's work. Your vault has these as separate threads. What's the convergence architecture?

### Related Materials

1. **Claude Code Agent Teams documentation** — Official guide to orchestrating parallel Claude Code sessions with tmux integration, task claiming, and git-based coordination.
   [https://code.claude.com/docs/en/agent-teams](https://code.claude.com/docs/en/agent-teams)

2. **Addy Osmani, "Claude Code Swarms"** — Practical walkthrough of multi-agent Claude Code patterns including team coordination, split-pane monitoring, and real-world project examples.
   [https://addyosmani.com/blog/claude-code-agent-teams/](https://addyosmani.com/blog/claude-code-agent-teams/)

3. **claude_code_agent_farm** (Jeffrey Emanuel) — Open-source framework for running 20+ Claude Code agents in parallel with tmux monitoring, lock-based coordination, and automated bug fixing.
   [https://github.com/Dicklesworthstone/claude_code_agent_farm](https://github.com/Dicklesworthstone/claude_code_agent_farm)

4. **"Multi-Agent Collaboration via Evolving Orchestration"** (Dang & Qian, 2025) — Proposes a puppeteer/puppet paradigm where an RL-trained orchestrator dynamically sequences agents based on evolving task state. Strong theoretical foundation for understanding when centralized vs. emergent coordination wins.
   [https://arxiv.org/abs/2505.19591](https://arxiv.org/abs/2505.19591)

5. **"AgentOrchestra: Orchestrating Multi-Agent Intelligence with the TEA Protocol"** (2026) — Introduces the Tool-Environment-Agent protocol with explicit lifecycles and versioned interfaces. Directly relevant to your MCP concerns — this is what a next-gen coordination protocol looks like.
   [https://arxiv.org/abs/2506.12508](https://arxiv.org/abs/2506.12508)

6. **Microsoft Azure AI Agent Design Patterns** — Production-tested orchestration patterns (sequential, parallel, supervisor, handoff) with architecture diagrams. More enterprise-flavored but the pattern taxonomy is useful.
   [https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns](https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns)

7. **Google ADK Multi-Agent Patterns Guide** — Google's developer guide to multi-agent patterns in their Agent Development Kit, covering routing, delegation, and parallel execution.
   [https://developers.googleblog.com/developers-guide-to-multi-agent-patterns-in-adk/](https://developers.googleblog.com/developers-guide-to-multi-agent-patterns-in-adk/)

8. **Ry Walker's AI Agent Research Index** — Your vault already has this, but it's the most comprehensive landscape survey: 200+ tools across 16 categories including agent coordination protocols (A2A, ACP, ANP, CrewAI, AutoGen, LangGraph).
   Referenced in `notes/references/2026-03-15-ry-walker-ai-agent-research-index.md`

9. **"The Orchestration of Multi-Agent Systems: Architectures, Protocols, and Enterprise Adoption"** (2026) — Comprehensive survey paper covering the full stack from coordination protocols to enterprise deployment patterns.
   [https://arxiv.org/html/2601.13671v1](https://arxiv.org/html/2601.13671v1)

10. **Codex-Orchestrator** — Practical guide to running OpenAI Codex agents in parallel using tmux, directly comparable to your tmux orchestration idea but for a different agent runtime.
    [https://medium.com/coding-nexus/codex-orchestrator-run-openai-codex-agents-in-parallel-using-tmux-8e89edc6f6bd](https://medium.com/coding-nexus/codex-orchestrator-run-openai-codex-agents-in-parallel-using-tmux-8e89edc6f6bd)

### Book & Podcast Recommendations

**Books:**

1. **"Designing Multi-Agent Systems" by Victor Dibia** — Framework-agnostic treatment of 6 orchestration patterns (sequential, parallel, supervisor, handoff, etc.) built from first principles. Dibia is a principal researcher at Microsoft Research who built AutoGen. Directly maps to the pattern vocabulary you'll need.

2. **"Build a Multi-Agent System (From Scratch)" by Val Andrei Fajardo** (Manning) — Hands-on construction of multi-agent infrastructure including MCP and A2A protocol integration, human-in-the-loop patterns, and tool-calling orchestration. Good complement to your Go rewrite work since it teaches the plumbing.

3. **"AI Engineering" by Chip Huyen** (O'Reilly, 2025) — Covers the full production stack including model orchestration, evaluation pipelines, and scaling infrastructure. Less agent-specific but essential context for anyone building systems that coordinate multiple AI components.

4. **"Practical Multi-Agent AI Systems" by Murali Kashaboina** (Wiley) — End-to-end production project using LangChain, LangGraph, MCP, and A2A. Strongest on the enterprise deployment and error-handling concerns that toy examples skip.

**Podcasts:**

1. **"Building Claude Code with Boris Cherny"** — The Pragmatic Engineer podcast. Behind-the-scenes on how Claude Code was built, directly relevant to understanding its orchestration capabilities. Already in your vault but worth actually listening to.

2. **"10 OpenClaw Lessons for Building Agent Teams"** — AI Daily Brief. Practical lessons on task separation, coordination, security, memory, and cost management from first-wave agent team deployments. Also in your vault.

3. **"Codex Launch & OpenClaw/Moltbook Chaos"** — IBM Mixture of Experts podcast. Panelists debate whether first-party coding agents (Codex) or open orchestration layers will win, with good analysis of the competitive dynamics.
   [https://www.ibm.com/think/podcasts/mixture-of-experts/codex-launch-openclaw-moltbook-chaos-ai-agents](https://www.ibm.com/think/podcasts/mixture-of-experts/codex-launch-openclaw-moltbook-chaos-ai-agents)

4. **"The 2026 State of AI Agents in Production"** — Build Wiz AI Show. Analysis of Anthropic's production agent report, covering what's actually working in deployed agent systems vs. demo-ware.

5. **Latent Space podcast** (Swyx & Alessio) — Not a single episode but the most technically rigorous ongoing coverage of AI engineering and agent infrastructure. Their episodes on agent evaluation and compound AI systems are particularly relevant.

### Suggested Deep Dives

1. **Agent coordination protocols: A2A vs. MCP vs. ACP** — Your vault has deep MCP coverage but thin material on Google's Agent-to-Agent (A2A) protocol and the Agent Communication Protocol (ACP). These are competing visions for how agents talk to each other (not just to tools). Understanding the trade-offs will sharpen your MCP criticism thread.
   - Start: [Google A2A Protocol specification](https://github.com/google/A2A)
   - Search: `"agent to agent protocol" A2A vs MCP comparison 2026`

2. **Harness-first engineering for orchestrated agents** — Your vault covers Datadog's formal verification for single-agent output, but the gap is verification strategies when multiple agents' outputs must compose correctly. This connects your harness-first thread to your orchestration thread in a way that doesn't exist in the vault yet.
   - Start: The AgentOrchestra TEA paper's traceability model ([arxiv.org/abs/2506.12508](https://arxiv.org/abs/2506.12508))
   - Search: `"multi-agent testing" "integration verification" LLM agent output composition`

3. **Terminal-native agent orchestration: tmux vs. built-in agent teams** — Your original tmux note predates Claude Code's native Agent Teams feature. The interesting question now is whether rolling your own tmux orchestration (like `claude_code_agent_farm`) gives you capabilities the built-in system doesn't — custom monitoring, cross-project coordination, heterogeneous agent runtimes (Claude + Codex + Amp in one tmux session).
   - Start: [claude_code_agent_farm README](https://github.com/Dicklesworthstone/claude_code_agent_farm) and [Claude Code Agent Teams docs](https://code.claude.com/docs/en/agent-teams)
   - Search: `tmux AI agent orchestration multi-runtime parallel coding workflow`
