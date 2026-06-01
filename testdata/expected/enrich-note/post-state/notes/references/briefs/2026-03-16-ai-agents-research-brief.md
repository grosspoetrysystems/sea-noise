---
title: "Research Brief: ai-agents"
tags:
  - reference
  - reference/brief
  - research-brief
  - ai-agents
date: 2026-03-16
source: enrich-topics
---

# Research Brief: ai-agents

Auto-generated enrichment for topic cluster **ai-agents** (29 notes, threshold crossed).

## Vault Notes in This Cluster
- [[ai-agents-and-tools]]
- [[2026-03-14-openviking-context-database]]
- [[2026-03-12-getting-started-openai-symphony]]
- [[2026-03-12-openclaw-lessons-agent-teams]]
- [[2026-03-12-github-moment-for-agents]]
- [[2026-03-12-anti-fragile-infrastructure-coding-agents]]
- [[2026-03-12-harness-engineering-codex-agent-first]]
- [[2026-03-12-process-compression-bret-taylor]]
- [[2026-03-15-ry-walker-ai-agent-research-index]]
- [[2026-03-12-amp-coding-agent-manual]]
- [[2026-03-13-autonomous-verified-optimization]]
- [[2026-03-16-mem0-scalable-agent-memory]]
- [[2026-03-12-mcp-security-argument]]
- [[2026-03-12-building-claude-code-boris-cherny]]
- [[2026-03-12-mcporter-mcp-typescript-toolkit]]
- [[2026-03-12-mcp-dynamic-loading-response]]
- [[2026-03-11-anthropic-people-products-role]]
- [[2026-03-12-garry-tan-mcp-criticism]]
- [[2026-03-12-google-a2ui-agent-generated-uis]]
- [[2026-03-11-intent-engineering-beyond-context]]
- [[2026-03-11-intent-engineering-framework-agents]]
- [[2026-03-12-consumer-agents-mental-load]]
- [[2026-03-14-agentmail-email-for-agents]]
- [[2026-03-12-raising-an-agent-podcast]]
- [[2026-03-13-harness-first-agent-engineering]]
- [[2026-03-12-governance-liveness-agentic-era]]
- [[2026-03-12-product-ideas]]
- [[2026-03-13-invisible-castles]]
- [[2026-03-12-ai-tool-sprawl-orchestration]]

### Follow-up Questions
1. Your vault has extensive coverage of agent *infrastructure* (orchestration, memory, protocols) but almost no notes on agent *evaluation* — how do you measure whether an agent actually did a good job? The Datadog harness-first pieces gesture at this, but what does a general-purpose agent eval framework look like outside of coding tasks?
2. Mem0 and OpenViking both tackle agent memory, but from different angles (session persistence vs. hierarchical context delivery). Is there a convergence point, or are these fundamentally different problems? What would a unified agent memory architecture look like?
3. The MCP debate (Garry Tan's criticism vs. the security rebuttal) surfaces a real tension: protocol overhead vs. simplicity. But neither side addresses what happens when agents need to compose *each other's* tools — is MCP even the right abstraction layer for agent-to-agent collaboration?
4. Bret Taylor's "process compression" thesis and Claire Vo's "mental load" piece point at the same insight from different angles. If AI agents compress processes rather than replace people, who owns the *decomposition* — the user or the agent? What are the failure modes when agents decompose processes the user doesn't fully understand?
5. Your governance/liveness assertion connects to agent autonomy but hasn't accumulated much evidence yet. What specific governance mechanisms from DAO design (token-weighted voting, conviction voting, rage quit) have direct analogs in agent permission systems?
6. The harness-first engineering pieces from Datadog describe verification for *deterministic* outputs. How do you build verification harnesses for agents doing *creative* or *ambiguous* work — summarization, research, design decisions — where there's no formal spec to check against?
7. Several notes reference agents operating on codebases (Codex, Amp, Claude Code), but none address the organizational dynamics: what happens to code review culture, knowledge distribution, and team learning when 60%+ of PRs are agent-generated?

### Related Materials
1. **"Practices for Governing Agentic AI Systems"** — OpenAI whitepaper (2023). Foundational framework for agent governance including monitoring, delegation boundaries, and human oversight patterns. https://openai.com/index/practices-for-governing-agentic-ai-systems/
2. **"The Landscape of Emerging AI Agent Architectures for Reasoning, Planning, and Tool Calling"** — Masterman et al. (2024). Survey paper categorizing single-agent, multi-agent, and planning architectures. https://arxiv.org/abs/2404.11584
3. **"Voyager: An Open-Ended Embodied Agent with Large Language Models"** — Fan et al. (Nvidia, 2023). Seminal paper on agents that build their own skill library — directly relevant to the self-evolution thread in OpenViking. https://arxiv.org/abs/2305.16291
4. **"METR Task Standard"** — Model Evaluation & Threat Research. Open standard for evaluating autonomous agent capabilities on real-world tasks. https://github.com/METR/task-standard
5. **"Gorilla: Large Language Model Connected with Massive APIs"** — Patil et al. (UC Berkeley, 2023). Addresses the tool-calling accuracy problem that underlies the MCP debate. https://arxiv.org/abs/2305.15334
6. **"Agent Protocol"** — AI Engineer Foundation. Open spec for agent-to-agent and agent-to-human communication, alternative framing to MCP. https://github.com/AI-Engineer-Foundation/agent-protocol
7. **"Cognitive Architectures for Language Agents"** — Sumers et al. (2024). Proposes CoALA framework unifying memory, action, and decision-making in language agents. Directly relevant to your Mem0 and context engineering notes. https://arxiv.org/abs/2309.02427
8. **"SWE-bench"** — Jimenez et al. (Princeton, 2024). The benchmark that drove coding agent development — important context for evaluating the Codex/Amp/Claude Code claims in your vault. https://arxiv.org/abs/2310.06770
9. **"AutoGen: Enabling Next-Gen LLM Applications via Multi-Agent Conversation"** — Wu et al. (Microsoft, 2023). Multi-agent conversation framework — relevant to the orchestration thread (Symphony, OpenClaw). https://arxiv.org/abs/2308.08155
10. **Anthropic's "Building effective agents"** guide (2024). Practical patterns for agent design from the Claude team — workflows vs. autonomous agents, when to use which. https://www.anthropic.com/engineering/building-effective-agents

### Book & Podcast Recommendations

**Books:**
1. **"Designing Autonomous AI"** by Kence Anderson (O'Reilly, 2024) — Practical engineering guide to building production agent systems, covers planning, tool use, and human-in-the-loop patterns.
2. **"The Alignment Problem"** by Brian Christian (2020) — The governance and liveness questions in your assertion connect directly to alignment; this is the best accessible treatment of the problem space.
3. **"Thinking in Systems"** by Donella Meadows (2008) — Your process compression and orchestration notes are fundamentally systems thinking problems; this is the canonical primer on feedback loops, leverage points, and emergent behavior in complex systems.
4. **"A Philosophy of Software Design"** by John Ousterhout (2nd ed., 2021) — Relevant to the harness-first and anti-fragile infrastructure threads; Ousterhout's complexity management principles apply directly to agent-generated code governance.

**Podcasts:**
1. **Latent Space** — "The Agent Landscape" episodes (Swyx & Alessio, ongoing 2024-2026). The most technically dense agent coverage; their interviews with agent framework builders directly complement your orchestration notes.
2. **Cognitive Revolution** — hosted by Nathan Labenz. Regularly features agent researchers; the Voyager episode and multi-agent coordination episodes are particularly relevant.
3. **Practical AI** (Changelog) — "Agentic AI Patterns" episodes. More ops-focused, pairs well with your Datadog harness-first notes.
4. **Gradient Dissent** (Weights & Biases) — interviews with ML engineers building agent evaluation and observability, relevant to the verification gap in your vault.
5. **Raising an Agent** — Quinn Slack & Thorsten Ball (already in your vault). Continue following — they're documenting the coding agent buildout in real time from the Sourcegraph/Amp perspective.

### Suggested Deep Dives

1. **Agent evaluation and benchmarking beyond code** — Your vault is heavy on coding agents but has no notes on how to evaluate agents doing non-deterministic work (research, summarization, planning). Start with the METR task standard (https://github.com/METR/task-standard) and the "AgentBench" paper (https://arxiv.org/abs/2308.03688), then explore how Braintrust, LangSmith, and Arize are building eval tooling for production agents. Search: `"agent evaluation" "non-deterministic" benchmark 2024 2025`.

2. **Agent economics and delegation theory** — Bret Taylor's process compression and Claire Vo's mental load pieces raise economic questions your vault hasn't addressed: pricing models for agent work, principal-agent problems when the agent is literal, and how agent labor changes org structure. Start with Ethan Mollick's research at Wharton on AI and organizational productivity (https://www.oneusefulthing.org/) and search: `"AI agents" "principal agent" economics delegation 2024 2025`.

3. **Multi-agent coordination protocols** — Your vault covers MCP (tool integration) and some orchestration (Symphony, OpenClaw) but doesn't address the emerging protocol layer for agents coordinating with *each other*. The Agent Protocol spec, Google's Agent2Agent (A2A) protocol, and the academic work on multi-agent communication are the starting points. Search: `"agent to agent" protocol coordination "multi-agent" A2A 2025`. Start at https://google.github.io/A2A/.
