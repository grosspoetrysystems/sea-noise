# RAG Index

## Purpose
Embed vault notes into a vector database for semantic search. MCP server exposes `search_vault` tool to Claude Code.

## Scripts
- `_rag/indexer.py` (149 lines) — embedding + indexing
- `_rag/mcp_server.py` (115 lines) — MCP server

## Indexer
1. Load model: `BAAI/bge-base-en-v1.5` (768-dim, cosine similarity)
2. Open LanceDB at `_rag/vault.db`
3. Walk vault .md files, excluding: `_rag, _meta, templates, .git, .obsidian, signal, bloom`
4. Parse frontmatter (title, tags)
5. Embed text: `title + tags + body[:2000]`
6. Drop + recreate "notes" table
7. Create PQ index if row count ≥256

## MCP Server
- **Tool:** `search_vault(query: str, top_k: int = 5)`
- Embeds query with same model
- Cosine similarity search on LanceDB table
- Returns top-k results: title, path, tags, excerpt, score
- Registered as project-scoped MCP server (`vault-rag`)

## Database Schema
| Field | Type | Description |
|-------|------|-------------|
| id | string | Filename stem |
| path | string | Relative path from vault root |
| title | string | From frontmatter |
| tags | string | Comma-joined tags |
| content | string | First 4000 chars of body |
| vector | float[768] | BAAI/bge-base-en-v1.5 embedding |

## Environment
- `HF_TOKEN` — HuggingFace token for model download

## Known Quirks (LanceDB on Python 3.14)
- `db.list_tables()` returns `ListTablesResponse` — use `.tables`
- ANN index requires ≥256 rows
- `table.to_pandas()` requires pandas installed separately

## Go Rewrite Target
`mind-palace index` / `mind-palace search <query>` — Phase 5 in go-rewrite-plan.md.
Replacing LanceDB + sentence-transformers with SQLite + sqlite-vec + OpenAI/Ollama embeddings.
Maps to `internal/embed/` + `internal/search/`.
