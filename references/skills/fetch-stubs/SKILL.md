# Fetch Stubs

## Purpose
Hydrate mobile web clipper stubs with full content. Uses platform-specific
APIs first (faster, no rate limits), falls back to Cloudflare /crawl.
Deduplicates web clips against vault before hydration.

## Scripts
- `_scripts/fetch-stub-content.py` — main orchestrator (hydrate, dedup, retry)
- `_scripts/platform_apis.py` — platform API registry (Substack, future others)

## Fetch Priority
1. **Platform APIs** (`platform_apis.py`) — checked first by URL pattern match.
   Faster, more reliable, richer data. Currently: Substack.
2. **Cloudflare /crawl** — fallback for URLs without a known API.
3. If both fail → logged to `_meta/crawl-failures.json` for retry.

See `_meta/platform-apis.md` for the full registry and candidates.

## Modes
- **Default:** Scan `inbox/` for stubs, hydrate + dedup
- **`--retry`:** Retry entries from `_meta/crawl-failures.json` past their backoff window

## Stub Detection
A note is a stub if:
- Has a source URL in frontmatter
- Has `clippings` in tags
- Body is <200 characters

## Deduplication
1. Normalize URL: strip tracking params (`utm_*`, `ref`, `fbclid`), fragments, trailing slashes
2. Scan `notes/fleeting/`, `notes/references/`, `notes/ideas/` for existing source URLs
3. Also catch inbox-internal dupes (same URL clipped twice)
4. Duplicates → `_meta/discarded/` with reason logged

## Platform APIs (in `platform_apis.py`)
Registry of platform-specific fetchers. Each is a function that takes a URL
and returns content or None. Matched by regex on the source URL.

| Platform | Pattern | API | Auth |
|----------|---------|-----|------|
| Substack | `*substack.com/p/*` | `<pub>.substack.com/api/v1/posts/<slug>` | None |

When a new platform is discovered (rate-limited by Cloudflare, or has a
better API route), add a fetcher to `platform_apis.py` and document in
`_meta/platform-apis.md`.

## Cloudflare /crawl (fallback)
- POST `https://api.cloudflare.com/client/v4/accounts/{id}/browser-rendering/crawl`
- Poll job status at 5s intervals, max 30 attempts
- Returns page content as markdown

## Retry System
- Failed crawls logged to `_meta/crawl-failures.json`
- Backoff schedule: 6h → 24h → 72h → 168h
- Abandon after 4 failures
- On retry success: find note by source URL (may have moved), hydrate in place
- Retries also check platform APIs first (a previously-unknown API may now exist)

## Environment
- `CF_ACCOUNT_ID` — Cloudflare account ID (optional)
- `CF_API_TOKEN` — Cloudflare API token (optional)
- Platform APIs require no credentials

## Go commands
`sea-noise fetch` hydrates stubs; `sea-noise fetch --retry` retries recorded failures.
