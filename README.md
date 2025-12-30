# Blogmon

Monitor developer blogs, extract insights, and build a knowledge base.

## Installation

```bash
go install github.com/julienpequegnot/blogmon@latest
```

Or build from source:

```bash
git clone https://github.com/julienpequegnot/blogmon
cd blogmon
go build -tags sqlite_fts5 -o blogmon .
```

Note: The `sqlite_fts5` build tag is required for full-text search support.

## Quick Start

```bash
# Initialize configuration and database
blogmon init

# Add blogs to monitor
blogmon add https://jvns.ca
blogmon add https://fasterthanli.me
blogmon add https://brooker.co.za

# Fetch posts
blogmon fetch

# List posts
blogmon list

# Show post details
blogmon show 1
```

## Commands

| Command | Description |
|---------|-------------|
| `blogmon init` | Initialize config and database |
| `blogmon add <url>` | Add a blog to monitor |
| `blogmon fetch` | Download new posts from feeds |
| `blogmon extract` | Extract insights from posts using LLM |
| `blogmon score` | Calculate community/relevance/novelty scores |
| `blogmon link` | Build concept graph by linking related posts |
| `blogmon discover` | Discover new blogs from references |
| `blogmon trends` | Show trending topics |
| `blogmon list` | List posts (--sort: date/score/source) |
| `blogmon show <id>` | Show post details |
| `blogmon sources` | List monitored sources |
| `blogmon search <query>` | Full-text search across posts |
| `blogmon daemon` | Run in daemon mode for auto-fetching |
| `blogmon reindex` | Rebuild full-text search index |

## Configuration

Config is stored in `~/.blogmon/config.yaml`

```yaml
interests:
  - topic: "distributed-systems"
    weight: 1.0
  - topic: "rust"
    weight: 0.9

scoring:
  community: 0.3
  relevance: 0.4
  novelty: 0.3

apis:
  llm_provider: "ollama"
  llm_model: "llama3.2"

fetch:
  concurrency: 5
  timeout_seconds: 30

daemon:
  interval_hours: 6
```

## Architecture

Pipeline architecture:

```
fetch → extract → score → link → search
         ↑                        ↓
      daemon (scheduled)      query results
```

- **fetch**: Download posts from RSS feeds
- **extract**: Parse content, extract insights using LLM
- **score**: Calculate community/relevance/novelty scores
- **link**: Build concept graph linking related posts
- **search**: Full-text search with BM25 ranking

## Development Status

### Phase 1 (MVP) - Complete
- [x] Project setup
- [x] init command
- [x] add command (with RSS feed auto-discovery)
- [x] fetch command (with concurrent fetching)
- [x] list command (with styled output)
- [x] show command
- [x] sources command

### Phase 2 (Intelligence) - Complete
- [x] LLM-powered insight extraction (Ollama)
- [x] Community scoring (HN API)
- [x] Relevance scoring (keyword matching)
- [x] Novelty scoring (TF-IDF)

### Phase 3 (Graph & Discovery) - Complete
- [x] Concept graph (topic-based post linking)
- [x] Blog discovery (from references)
- [x] Trend detection (topic trending analysis)

### Phase 4 (Polish) - Complete
- [x] Full-text search (SQLite FTS5 with BM25 ranking)
- [x] Daemon mode (scheduled background processing)
- [x] Search command with ranked results
- [x] Reindex command for index maintenance
- [x] Enhanced list command with sorting
