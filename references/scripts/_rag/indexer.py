#!/usr/bin/env python3
"""
Vault indexer: walks .md files, embeds with sentence-transformers, upserts into LanceDB.

Uses merge-upsert instead of drop+recreate so the table stays valid for
any process (like the MCP server) holding a reference to it.
"""
import os
import re
import sys
from pathlib import Path

os.environ.setdefault("TRANSFORMERS_VERBOSITY", "error")

import lancedb
import pyarrow as pa
from sentence_transformers import SentenceTransformer

VAULT_ROOT = Path(__file__).parent.parent
DB_PATH = Path(__file__).parent / "vault.db"
MODEL_NAME = "BAAI/bge-base-en-v1.5"
EXCLUDE_DIRS = {"_rag", "_meta", "templates", ".git", ".obsidian", "signal", "bloom"}
PQ_INDEX_THRESHOLD = 256


def parse_frontmatter(content: str) -> tuple[str, str, str]:
    """Extract title and tags from YAML frontmatter. Returns (title, tags, body)."""
    title = ""
    tags = ""
    body = content

    if content.startswith("---"):
        end = content.find("\n---", 3)
        if end != -1:
            fm = content[3:end]
            body = content[end + 4:].lstrip()

            for line in fm.splitlines():
                if line.startswith("title:"):
                    title = line[6:].strip().strip('"').strip("'")
                elif line.startswith("tags:"):
                    raw = line[5:].strip()
                    raw = raw.strip("[]")
                    tags = " ".join(t.strip() for t in raw.split(",") if t.strip())

    if not title:
        m = re.search(r"^#\s+(.+)", body, re.MULTILINE)
        if m:
            title = m.group(1).strip()

    return title, tags, body


def walk_vault(root: Path):
    for path in root.rglob("*.md"):
        if any(part in EXCLUDE_DIRS for part in path.parts):
            continue
        yield path


def main():
    print(f"Loading model {MODEL_NAME}...")
    model = SentenceTransformer(MODEL_NAME)

    print(f"Opening LanceDB at {DB_PATH}...")
    db = lancedb.connect(str(DB_PATH))

    schema = pa.schema([
        pa.field("id", pa.string()),
        pa.field("path", pa.string()),
        pa.field("title", pa.string()),
        pa.field("tags", pa.string()),
        pa.field("content", pa.string()),
        pa.field("vector", pa.list_(pa.float32(), 768)),
    ])

    existing_tables = db.list_tables().tables
    has_table = "notes" in existing_tables

    md_files = list(walk_vault(VAULT_ROOT))
    current_ids = set()
    print(f"Found {len(md_files)} markdown files.")

    rows = []
    for path in md_files:
        rel = str(path.relative_to(VAULT_ROOT))
        file_id = rel

        try:
            content_raw = path.read_text(encoding="utf-8", errors="ignore")
        except Exception as e:
            print(f"  SKIP {rel}: {e}", file=sys.stderr)
            continue

        if not content_raw.strip():
            continue

        title, tags, body = parse_frontmatter(content_raw)
        if not title:
            title = path.stem.replace("-", " ").replace("_", " ")

        text_to_embed = f"{title}\n{tags}\n{body[:2000]}"
        vector = model.encode(text_to_embed, normalize_embeddings=True).tolist()

        rows.append({
            "id": file_id,
            "path": rel,
            "title": title,
            "tags": tags,
            "content": body[:4000],
            "vector": vector,
        })
        current_ids.add(file_id)

    if not rows:
        print("No files to index.")
        return

    if has_table:
        table = db.open_table("notes")

        # Merge upsert: update existing rows, insert new ones
        table.merge_insert("id") \
            .when_matched_update_all() \
            .when_not_matched_insert_all() \
            .execute(rows)

        # Remove rows for files that no longer exist (moved/deleted)
        existing_ids = set(r["id"] for r in table.search().select(["id"]).limit(10000).to_list())
        stale_ids = existing_ids - current_ids
        if stale_ids:
            # Delete stale rows using a filter
            filter_expr = " OR ".join(f'id = "{sid}"' for sid in stale_ids)
            table.delete(filter_expr)
            print(f"Removed {len(stale_ids)} stale entries.")
    else:
        table = db.create_table("notes", data=rows, schema=schema)

    row_count = table.count_rows()
    if row_count >= PQ_INDEX_THRESHOLD:
        print(f"Building PQ index ({row_count} rows >= {PQ_INDEX_THRESHOLD})...")
        table.create_index(metric="cosine", replace=True)
        print("PQ index created.")
    else:
        print(f"Exact search enabled ({row_count} rows < {PQ_INDEX_THRESHOLD} threshold).")

    print(f"\nDone: {len(rows)} indexed, {len(stale_ids) if has_table and stale_ids else 0} removed.")
    print(f"Total in DB: {row_count}")


if __name__ == "__main__":
    main()
