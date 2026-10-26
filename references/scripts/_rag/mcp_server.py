#!/usr/bin/env python3
"""
Vault RAG MCP server.
Tool: search_vault(query, top_k=5) — semantic search over indexed notes.
"""
import asyncio
import os
from pathlib import Path
from typing import Any

os.environ.setdefault("TRANSFORMERS_VERBOSITY", "error")

import lancedb
from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp import types
from sentence_transformers import SentenceTransformer

DB_PATH = Path(__file__).parent / "vault.db"
MODEL_NAME = "BAAI/bge-base-en-v1.5"

# Model loaded once (expensive). Table re-opened each call (cheap)
# so we always read the latest data after reindexing.
_model: SentenceTransformer | None = None


def get_resources():
    global _model
    if _model is None:
        _model = SentenceTransformer(MODEL_NAME)
    db = lancedb.connect(str(DB_PATH))
    table = db.open_table("notes")
    return _model, table


app = Server("vault-rag")


@app.list_tools()
async def list_tools() -> list[types.Tool]:
    return [
        types.Tool(
            name="search_vault",
            description=(
                "Semantic search over the Obsidian mind-palace vault. "
                "Returns the most relevant notes for a natural-language query."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "Natural language search query",
                    },
                    "top_k": {
                        "type": "integer",
                        "description": "Number of results to return (default 5)",
                        "default": 5,
                    },
                },
                "required": ["query"],
            },
        )
    ]


@app.call_tool()
async def call_tool(name: str, arguments: dict) -> list[types.TextContent]:
    if name != "search_vault":
        raise ValueError(f"Unknown tool: {name}")

    query = arguments["query"]
    top_k = int(arguments.get("top_k", 5))

    model, table = get_resources()

    # Embed query
    vec = model.encode(query, normalize_embeddings=True).tolist()

    # Search
    results = (
        table.search(vec)
        .metric("cosine")
        .limit(top_k)
        .select(["path", "title", "tags", "content"])
        .to_list()
    )

    if not results:
        return [types.TextContent(type="text", text="No results found.")]

    lines = []
    for i, r in enumerate(results, 1):
        excerpt = (r["content"] or "")[:200].replace("\n", " ").strip()
        score = r.get("_distance", None)
        score_str = f" (score: {1 - score:.3f})" if score is not None else ""
        lines.append(
            f"{i}. **{r['title']}**{score_str}\n"
            f"   path: {r['path']}\n"
            f"   tags: {r['tags'] or 'none'}\n"
            f"   excerpt: {excerpt}"
        )

    return [types.TextContent(type="text", text="\n\n".join(lines))]


async def main():
    async with stdio_server() as (read_stream, write_stream):
        await app.run(read_stream, write_stream, app.create_initialization_options())


if __name__ == "__main__":
    asyncio.run(main())
