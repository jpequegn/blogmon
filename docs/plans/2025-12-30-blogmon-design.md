# Blogmon Design Document

**Date:** 2025-12-30
**Status:** Approved
**Author:** Design session with Claude

## Overview

Blogmon is a CLI tool for monitoring developer blogs, extracting insights, scoring content value, and building a searchable knowledge base with concept graph connections.

### Goals

1. **Research Archive** — Searchable knowledge base of ideas, references, and insights
2. **Trend Radar** — Track what top developers are discussing
3. **Content Curation** — Filter signal from noise to surface highest-value posts

### Non-Goals

- Web UI (CLI-first)
- Real-time notifications
- Social features

## Architecture

### Stack

| Component | Technology |
|-----------|------------|
| Language | Go |
| Primary Storage | SQLite |
| Vector Store | ChromaDB (optional, Phase 4) |
| LLM | Ollama (local) or OpenAI/Anthropic API |

### Pipeline Architecture

```
fetch → extract → score → link → query
```

Each stage is an independent command. Data flows through SQLite. Stages can be re-run independently.

## CLI Commands

### Setup & Configuration

```bash
blogmon init                    # Create ~/.blogmon with config and SQLite DB
blogmon add <url>               # Add blog/RSS feed to monitor
blogmon interests add <topic>   # Add personal interest for relevance scoring
```

### Pipeline Stages

```bash
blogmon fetch                   # Download new posts from all feeds
blogmon extract                 # Parse content, pull references, extract key points
blogmon score                   # Calculate value scores
blogmon link                    # Build concept graph connections
blogmon sync                    # Run full pipeline: fetch → extract → score → link
```

### Query & Browse

```bash
blogmon list [--top N] [--topic X] [--since DATE]
blogmon show <post-id>          # Full post with extracted insights
blogmon search <query>          # Full-text search
blogmon related <post-id>       # Show connected posts in concept graph
blogmon trends [--period 7d]    # What topics are heating up
blogmon discover                # Suggested new blogs from cross-references
```

### Daemon Mode

```bash
blogmon daemon start            # Background sync every N hours
blogmon daemon stop
```

## Data Model

### SQLite Schema

```sql
-- Sources (blogs/feeds to monitor)
CREATE TABLE sources (
    id INTEGER PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    name TEXT,
    feed_url TEXT,
    discovered_from INTEGER REFERENCES posts(id),  -- NULL if manual
    last_fetched DATETIME,
    active BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Posts (individual articles)
CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    source_id INTEGER NOT NULL REFERENCES sources(id),
    url TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    author TEXT,
    published_at DATETIME,
    fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    content_raw TEXT,           -- Original HTML/markdown
    content_clean TEXT,         -- Extracted text
    word_count INTEGER
);

-- Extracted insights per post
CREATE TABLE insights (
    id INTEGER PRIMARY KEY,
    post_id INTEGER NOT NULL REFERENCES posts(id),
    type TEXT NOT NULL,         -- 'takeaway', 'code_example', 'quote', 'definition'
    content TEXT NOT NULL,
    importance INTEGER          -- 1-5 ranking from LLM extraction
);

-- References found in posts (outbound links)
CREATE TABLE references (
    id INTEGER PRIMARY KEY,
    post_id INTEGER NOT NULL REFERENCES posts(id),
    url TEXT NOT NULL,
    title TEXT,
    context TEXT,               -- Surrounding text explaining the link
    is_blog BOOLEAN DEFAULT FALSE
);

-- Scoring data
CREATE TABLE scores (
    post_id INTEGER PRIMARY KEY REFERENCES posts(id),
    community_score REAL,       -- HN/Reddit signals
    relevance_score REAL,       -- Match to interests
    novelty_score REAL,         -- Distinctness from existing posts
    final_score REAL,           -- Weighted combination
    scored_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Concept graph edges
CREATE TABLE links (
    id INTEGER PRIMARY KEY,
    post_id_a INTEGER NOT NULL REFERENCES posts(id),
    post_id_b INTEGER NOT NULL REFERENCES posts(id),
    relationship TEXT NOT NULL, -- 'references', 'similar_topic', 'same_author', 'responds_to'
    strength REAL,              -- 0.0-1.0
    UNIQUE(post_id_a, post_id_b, relationship)
);

-- Personal interests for relevance scoring
CREATE TABLE interests (
    id INTEGER PRIMARY KEY,
    topic TEXT NOT NULL UNIQUE,
    weight REAL DEFAULT 1.0,
    keywords TEXT               -- JSON array of related keywords
);

-- Indexes
CREATE INDEX idx_posts_source ON posts(source_id);
CREATE INDEX idx_posts_published ON posts(published_at);
CREATE INDEX idx_scores_final ON scores(final_score DESC);
CREATE INDEX idx_links_post_a ON links(post_id_a);
CREATE INDEX idx_links_post_b ON links(post_id_b);
```

## Pipeline Stages Detail

### Stage 1: Fetch

**Input:** `sources` table
**Output:** New rows in `posts` table

- Iterate active sources, check RSS/Atom feeds
- Parse feed, dedupe by URL against existing posts
- Download full article content (not just feed summary)
- Store raw HTML in `content_raw`
- Parallel fetching with configurable concurrency
- Respect rate limits, handle errors gracefully

### Stage 2: Extract

**Input:** Posts where `content_clean IS NULL`
**Output:** `content_clean`, `insights`, `references`

- Clean HTML → readable text (strip nav, ads, footers)
- LLM call to extract:
  - Key takeaways (3-5 per post)
  - Code examples worth saving
  - Definitions of concepts introduced
- Parse all outbound links → `references` table
- Flag references pointing to blogs (`is_blog = TRUE`)

### Stage 3: Score

**Input:** Posts without scores or stale scores
**Output:** `scores` table

**Community Score (0-100):**
- HN Algolia API: search by URL for points/comments
- Reddit API: check relevant subreddits
- Formula: `log(1 + hn_points*2 + hn_comments*3 + reddit_score) * 10`

**Relevance Score (0-100):**
- Match post content against `interests` table
- v1: Keyword density calculation
- v2: Embedding similarity

**Novelty Score (0-100):**
- Compute TF-IDF vector for new post
- Find max cosine similarity to existing posts
- `novelty = (1 - max_similarity) * 100`

**Final Score:**
```
final = (community * 0.3) + (relevance * 0.4) + (novelty * 0.3)
```

### Stage 4: Link

**Input:** `posts`, `references`, `scores`
**Output:** `links` table (concept graph edges)

| Link Type | Detection Method | Strength |
|-----------|------------------|----------|
| `references` | URL in post body points to monitored post | 1.0 |
| `similar_topic` | Embedding/TF-IDF similarity > 0.6 | similarity value |
| `same_author` | Author field matches | 1.0 |
| `responds_to` | Within 14 days + references + similar topic | 0.8 |

## Discovery Engine

Surfaces frequently referenced domains not yet monitored:

```sql
SELECT domain, COUNT(*) as refs
FROM references
WHERE is_blog = TRUE
  AND domain NOT IN (SELECT domain FROM sources)
GROUP BY domain
ORDER BY refs DESC
LIMIT 10
```

## Configuration

### Directory Structure

```
~/.blogmon/
├── config.yaml          # Main configuration
├── blogmon.db           # SQLite database
├── vectors/             # Vector store (optional)
└── cache/               # Cached API responses
```

### config.yaml

```yaml
interests:
  - topic: "distributed-systems"
    weight: 1.0
    keywords: ["consensus", "raft", "paxos", "replication"]
  - topic: "rust"
    weight: 0.9
  - topic: "performance"
    weight: 0.8
    keywords: ["latency", "throughput", "optimization"]

scoring:
  community: 0.3
  relevance: 0.4
  novelty: 0.3

apis:
  llm_provider: "ollama"
  llm_model: "llama3.2"
  openai_key: "${OPENAI_API_KEY}"

fetch:
  concurrency: 5
  timeout_seconds: 30
  user_agent: "blogmon/1.0"

daemon:
  interval_hours: 6

reddit:
  subreddits: ["programming", "golang", "rust", "systems"]
```

## Output Formats

### List Output

```
 #  SCORE  DATE        SOURCE              TITLE
 1   87    2025-12-28  fasterthanli.me     Understanding Rust's ownership deeply
 2   82    2025-12-27  jvns.ca             How DNS works (for real this time)
 3   79    2025-12-26  brooker.co.za       Why distributed consensus is hard
```

### Show Output

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Understanding Rust's ownership deeply
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Source:    fasterthanli.me
Published: 2025-12-28
Score:     87 (community: 72, relevance: 95, novelty: 88)
URL:       https://fasterthanli.me/articles/rust-ownership

KEY TAKEAWAYS:
  • Ownership is compile-time memory management
  • The borrow checker prevents data races at compile time
  • Interior mutability (RefCell, Mutex) is the escape hatch

REFERENCES:
  → The Rust Book, Ch 4 (rust-lang.org)
  → "Fearless Concurrency" - Aaron Turon

RELATED POSTS:
  → #12 "Lifetimes explained" (same_author, 0.9)
  → #45 "Zero-cost abstractions" (similar_topic, 0.7)
```

### Trends Output

```
TRENDING TOPICS (last 14 days):
  1. ▲ rust-async       (+340%)  12 posts
  2. ▲ sqlite           (+180%)   8 posts
  3. ▲ llm-inference    (+120%)   6 posts
  4. ─ distributed-sys  (steady)  9 posts
  5. ▼ kubernetes       (-40%)    3 posts
```

## Implementation Phases

### Phase 1: Core Pipeline (MVP)

- `init`, `add`, `fetch`, `list`, `show` commands
- SQLite schema, RSS parsing
- Store raw content
- Manual source management
- **Deliverable:** Working blog reader

### Phase 2: Intelligence

- `extract` with LLM integration (Ollama)
- Insights and references extraction
- `score` with HN community signals
- Keyword-based relevance scoring
- **Deliverable:** Ranked, annotated posts

### Phase 3: Graph & Discovery

- `link` command builds concept graph
- `related` and `discover` queries
- Novelty scoring with TF-IDF
- **Deliverable:** Connected knowledge base

### Phase 4: Polish

- `trends` analysis
- `search` with SQLite FTS5
- `daemon` mode
- Vector store + semantic search
- **Deliverable:** Complete system

## Dependencies

### Go Libraries (suggested)

- `github.com/gocolly/colly` — Web scraping
- `github.com/mmcdole/gofeed` — RSS/Atom parsing
- `github.com/mattn/go-sqlite3` — SQLite driver
- `github.com/spf13/cobra` — CLI framework
- `github.com/charmbracelet/lipgloss` — Terminal styling
- `github.com/ollama/ollama/api` — Ollama client

### External APIs

- HN Algolia API (no auth required)
- Reddit API (OAuth required for higher rate limits)
- Ollama (local) or OpenAI/Anthropic (API key)

## Open Questions

1. **RSS discovery:** Should `blogmon add <url>` auto-discover RSS feed URLs?
2. **Duplicate handling:** How to handle cross-posts or syndicated content?
3. **Export format:** Should there be `blogmon export` for backup/migration?

---

*Design approved 2025-12-30*
