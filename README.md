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
go build -o blogmon .
```

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
| `blogmon list` | List posts |
| `blogmon show <id>` | Show post details |
| `blogmon sources` | List monitored sources |

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
```

## Architecture

Pipeline architecture:

```
fetch → extract → score → link → query
```

- **fetch**: Download posts from RSS feeds
- **extract**: Parse content, extract insights (Phase 2)
- **score**: Calculate value scores (Phase 2)
- **link**: Build concept graph (Phase 3)
- **query**: Search and browse

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

### Phase 3 (Graph & Discovery)
- [ ] Concept graph
- [ ] Blog discovery
- [ ] Trend detection

### Phase 4 (Polish)
- [ ] Semantic search
- [ ] Daemon mode
- [ ] Full-text search
