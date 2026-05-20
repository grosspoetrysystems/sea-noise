You are running the /research skill for the mind-palace vault. This triggers a deep research enrichment pass.

Determine the mode from the arguments:
- If a topic name is given (e.g. "ai-agents"), run: `python3 _scripts/enrich-topics.py "TOPIC"`
- If a note path is given (e.g. "notes/references/some-note.md"), run: `python3 _scripts/enrich-topics.py --note "PATH"`
- If no argument, run: `python3 _scripts/enrich-topics.py` (auto mode — enriches all topics above threshold)

After the script runs:
1. Read the generated research brief or enriched note
2. Present a summary of what was found to the user
3. Highlight the most interesting follow-up questions and surprise connections
4. Ask if the user wants to pull on any thread further

If the Cloudflare crawl is not configured (no CF_ACCOUNT_ID), the script still works — it just uses vault context + Claude without web crawling. Mention this to the user so they know they can set up crawling for richer results.

$ARGUMENTS
