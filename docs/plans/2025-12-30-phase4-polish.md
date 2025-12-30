# Phase 4 Polish Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add full-text search for fast content queries, daemon mode for automated background fetching, and a search command that combines FTS with scoring.

**Architecture:** SQLite FTS5 virtual table for full-text indexing. Daemon command runs fetch/extract/score/link pipeline on configurable interval. Search command queries FTS and ranks by combined relevance + final score.

**Tech Stack:** Go, SQLite FTS5, OS signals for daemon lifecycle

---

## Task 1: FTS5 Schema Migration

**Files:**
- Modify: `internal/database/database.go`

**Step 1: Read current database.go**

Read `/Users/julienpequegnot/Code/blogmon/internal/database/database.go`

**Step 2: Add FTS5 virtual table to schema**

Add after the existing CREATE INDEX statements in initSchema():
```go
	-- Full-text search virtual table
	CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
		title,
		content,
		content='posts',
		content_rowid='id'
	);

	-- Triggers to keep FTS in sync
	CREATE TRIGGER IF NOT EXISTS posts_ai AFTER INSERT ON posts BEGIN
		INSERT INTO posts_fts(rowid, title, content) VALUES (new.id, new.title, COALESCE(new.content_clean, new.content_raw, ''));
	END;

	CREATE TRIGGER IF NOT EXISTS posts_ad AFTER DELETE ON posts BEGIN
		INSERT INTO posts_fts(posts_fts, rowid, title, content) VALUES('delete', old.id, old.title, COALESCE(old.content_clean, old.content_raw, ''));
	END;

	CREATE TRIGGER IF NOT EXISTS posts_au AFTER UPDATE ON posts BEGIN
		INSERT INTO posts_fts(posts_fts, rowid, title, content) VALUES('delete', old.id, old.title, COALESCE(old.content_clean, old.content_raw, ''));
		INSERT INTO posts_fts(rowid, title, content) VALUES (new.id, new.title, COALESCE(new.content_clean, new.content_raw, ''));
	END;
```

**Step 3: Build and test**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go build -o blogmon . && go test ./internal/database/... -v
```

Expected: Build and tests pass

**Step 4: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: add FTS5 full-text search schema"
```

---

## Task 2: Search Repository

**Files:**
- Create: `internal/search/repository.go`
- Create: `internal/search/repository_test.go`

**Step 1: Write tests**

```go
// internal/search/repository_test.go
package search

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/julienpequegnot/blogmon/internal/source"
)

func setupTestDB(t *testing.T) *database.DB {
	tmpDir := t.TempDir()
	db, err := database.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	srcRepo := source.NewRepository(db)
	src, _ := srcRepo.Add("https://test.com", "Test", "")

	postRepo := post.NewRepository(db)
	postRepo.Add(src.ID, "https://test.com/p1", "Learning Golang Concurrency", "Author", time.Now(), "This post covers goroutines and channels in Go programming")
	postRepo.Add(src.ID, "https://test.com/p2", "Rust Memory Safety", "Author", time.Now(), "Understanding ownership and borrowing in Rust")
	postRepo.Add(src.ID, "https://test.com/p3", "Python Data Science", "Author", time.Now(), "Using pandas and numpy for data analysis")

	return db
}

func TestSearchByQuery(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	results, err := repo.Search("golang", 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one result for 'golang'")
	}
}

func TestSearchNoResults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	results, err := repo.Search("kubernetes", 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected no results for 'kubernetes', got %d", len(results))
	}
}

func TestSearchWithSnippet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	results, err := repo.Search("goroutines", 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}

	if results[0].Snippet == "" {
		t.Error("expected snippet to be populated")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go test ./internal/search/... -v
```

Expected: FAIL - package doesn't exist

**Step 3: Implement repository**

```go
// internal/search/repository.go
package search

import (
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type SearchResult struct {
	PostID      int64
	Title       string
	SourceName  string
	PublishedAt time.Time
	Snippet     string
	Rank        float64
	FinalScore  float64
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Search(query string, limit int) ([]SearchResult, error) {
	rows, err := r.db.Query(`
		SELECT
			p.id,
			p.title,
			s.name,
			p.published_at,
			snippet(posts_fts, 1, '<b>', '</b>', '...', 32) as snippet,
			bm25(posts_fts) as rank,
			COALESCE(sc.final_score, 0) as final_score
		FROM posts_fts
		JOIN posts p ON posts_fts.rowid = p.id
		JOIN sources s ON p.source_id = s.id
		LEFT JOIN scores sc ON p.id = sc.post_id
		WHERE posts_fts MATCH ?
		ORDER BY bm25(posts_fts)
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.PostID, &r.Title, &r.SourceName, &r.PublishedAt, &r.Snippet, &r.Rank, &r.FinalScore); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (r *Repository) SearchWithScore(query string, limit int) ([]SearchResult, error) {
	// Search combining FTS rank with final score
	rows, err := r.db.Query(`
		SELECT
			p.id,
			p.title,
			s.name,
			p.published_at,
			snippet(posts_fts, 1, '<b>', '</b>', '...', 32) as snippet,
			bm25(posts_fts) as rank,
			COALESCE(sc.final_score, 0) as final_score
		FROM posts_fts
		JOIN posts p ON posts_fts.rowid = p.id
		JOIN sources s ON p.source_id = s.id
		LEFT JOIN scores sc ON p.id = sc.post_id
		WHERE posts_fts MATCH ?
		ORDER BY (COALESCE(sc.final_score, 0) * 0.3 - bm25(posts_fts) * 0.7) DESC
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.PostID, &r.Title, &r.SourceName, &r.PublishedAt, &r.Snippet, &r.Rank, &r.FinalScore); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (r *Repository) RebuildIndex() error {
	// Rebuild FTS index from posts table
	_, err := r.db.Exec(`
		DELETE FROM posts_fts;
		INSERT INTO posts_fts(rowid, title, content)
		SELECT id, title, COALESCE(content_clean, content_raw, '') FROM posts;
	`)
	return err
}
```

**Step 4: Run tests**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go test ./internal/search/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: add search repository with FTS5"
```

---

## Task 3: Search Command

**Files:**
- Create: `cmd/search.go`

**Step 1: Create search command**

```go
// cmd/search.go
package cmd

import (
	"fmt"
	"strings"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/search"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search posts by content",
	Long:  `Full-text search across all post titles and content.`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSearch,
}

var (
	searchLimit    int
	searchUseScore bool
)

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 20, "Maximum results to show")
	searchCmd.Flags().BoolVar(&searchUseScore, "ranked", false, "Rank by combined relevance and score")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")

	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	searchRepo := search.NewRepository(db)

	var results []search.SearchResult
	if searchUseScore {
		results, err = searchRepo.SearchWithScore(query, searchLimit)
	} else {
		results, err = searchRepo.Search(query, searchLimit)
	}

	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No results found for '%s'\n", query)
		return nil
	}

	// Display results
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	snippetStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	fmt.Printf("\n%s '%s' (%d results)\n\n", titleStyle.Render("SEARCH:"), query, len(results))

	for _, r := range results {
		fmt.Printf("%s %s\n", idStyle.Render(fmt.Sprintf("[%d]", r.PostID)), r.Title)
		fmt.Printf("    %s", sourceStyle.Render(r.SourceName))
		if r.FinalScore > 0 {
			fmt.Printf(" • Score: %.1f", r.FinalScore)
		}
		fmt.Println()

		if r.Snippet != "" {
			// Clean up HTML tags from snippet
			snippet := strings.ReplaceAll(r.Snippet, "<b>", "")
			snippet = strings.ReplaceAll(snippet, "</b>", "")
			fmt.Printf("    %s\n", snippetStyle.Render(snippet))
		}
		fmt.Println()
	}

	return nil
}
```

**Step 2: Build and test**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go build -o blogmon . && ./blogmon search --help
```

Expected: Help message displays

**Step 3: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: add search command with full-text search"
```

---

## Task 4: Daemon Mode

**Files:**
- Create: `cmd/daemon.go`

**Step 1: Create daemon command**

```go
// cmd/daemon.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/feed"
	"github.com/julienpequegnot/blogmon/internal/graph"
	"github.com/julienpequegnot/blogmon/internal/insight"
	"github.com/julienpequegnot/blogmon/internal/link"
	"github.com/julienpequegnot/blogmon/internal/llm"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/julienpequegnot/blogmon/internal/reference"
	"github.com/julienpequegnot/blogmon/internal/score"
	"github.com/julienpequegnot/blogmon/internal/scorer"
	"github.com/julienpequegnot/blogmon/internal/source"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run in daemon mode",
	Long:  `Runs blogmon in the background, periodically fetching and processing new posts.`,
	RunE:  runDaemon,
}

var (
	daemonInterval int
	daemonOnce     bool
)

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.Flags().IntVar(&daemonInterval, "interval", 0, "Override interval in hours (0 = use config)")
	daemonCmd.Flags().BoolVar(&daemonOnce, "once", false, "Run once and exit")
}

func runDaemon(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	interval := cfg.Daemon.IntervalHours
	if daemonInterval > 0 {
		interval = daemonInterval
	}

	fmt.Printf("Blogmon daemon starting (interval: %d hours)\n", interval)

	// Run immediately on start
	if err := runPipeline(cfg); err != nil {
		fmt.Printf("Pipeline error: %v\n", err)
	}

	if daemonOnce {
		fmt.Println("Single run complete.")
		return nil
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(interval) * time.Hour)
	defer ticker.Stop()

	fmt.Printf("Daemon running. Next run in %d hours. Press Ctrl+C to stop.\n", interval)

	for {
		select {
		case <-ticker.C:
			fmt.Printf("\n[%s] Running scheduled pipeline...\n", time.Now().Format("2006-01-02 15:04:05"))
			if err := runPipeline(cfg); err != nil {
				fmt.Printf("Pipeline error: %v\n", err)
			}
			fmt.Printf("Next run in %d hours.\n", interval)

		case sig := <-sigChan:
			fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
			cancel()
			return nil

		case <-ctx.Done():
			return nil
		}
	}
}

func runPipeline(cfg *config.Config) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	// Stage 1: Fetch
	fmt.Println("→ Fetching new posts...")
	srcRepo := source.NewRepository(db)
	postRepo := post.NewRepository(db)

	sources, err := srcRepo.List()
	if err != nil {
		return err
	}

	fetcher := feed.NewFetcher(cfg.Fetch.Concurrency, cfg.Fetch.TimeoutSeconds, cfg.Fetch.UserAgent)
	newPosts := 0

	for _, src := range sources {
		if src.FeedURL == "" {
			continue
		}

		items, err := fetcher.Fetch(src.FeedURL)
		if err != nil {
			fmt.Printf("  Error fetching %s: %v\n", src.Name, err)
			continue
		}

		for _, item := range items {
			exists, _ := postRepo.Exists(item.Link)
			if exists {
				continue
			}

			_, err := postRepo.Add(src.ID, item.Link, item.Title, item.Author, item.Published, item.Content)
			if err == nil {
				newPosts++
			}
		}

		srcRepo.UpdateLastFetched(src.ID)
	}
	fmt.Printf("  Fetched %d new posts\n", newPosts)

	if newPosts == 0 {
		fmt.Println("→ No new posts to process")
		return nil
	}

	// Stage 2: Extract (limit to new posts)
	fmt.Println("→ Extracting insights...")
	insightRepo := insight.NewRepository(db)
	refRepo := reference.NewRepository(db)

	llmClient := llm.NewClient("http://localhost:11434", cfg.APIs.LLMModel, 2*time.Minute)

	unextracted, _ := postRepo.GetUnextracted(newPosts)
	extracted := 0
	for _, p := range unextracted {
		content := stripHTMLTags(p.ContentRaw)
		if len(content) < 100 {
			continue
		}
		if len(content) > 8000 {
			content = content[:8000]
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		result, err := llmClient.ExtractInsights(ctx, p.Title, content)
		cancel()

		if err != nil {
			continue
		}

		for i, takeaway := range result.Takeaways {
			importance := 5 - i
			if importance < 1 {
				importance = 1
			}
			insightRepo.Add(p.ID, "takeaway", takeaway, importance)
		}

		for _, ref := range result.References {
			isBlog := isBlogURL(ref.URL)
			refRepo.Add(p.ID, ref.URL, ref.Title, ref.Context, isBlog)
		}

		wordCount := len(content) / 5 // rough estimate
		postRepo.UpdateContentClean(p.ID, content, wordCount)
		extracted++
	}
	fmt.Printf("  Extracted insights from %d posts\n", extracted)

	// Stage 3: Score
	fmt.Println("→ Scoring posts...")
	scoreRepo := score.NewRepository(db)
	hnScorer := scorer.NewHNScorer()
	relevanceScorer := scorer.NewRelevanceScorer(cfg.Interests)
	noveltyScorer := scorer.NewNoveltyScorer()

	allPosts, _ := postRepo.List(1000, 0)
	for _, p := range allPosts {
		content := p.ContentClean
		if content == "" {
			content = p.Title
		}
		noveltyScorer.AddDocument(p.ID, content)
	}

	unscoredIDs, _ := scoreRepo.GetUnscoredPostIDs(newPosts)
	scored := 0
	for _, postID := range unscoredIDs {
		p, err := postRepo.Get(postID)
		if err != nil {
			continue
		}

		var communityScore float64
		hit, _ := hnScorer.SearchByURL(p.URL)
		if hit != nil {
			communityScore = scorer.CalculateCommunityScore(hit.Points, hit.NumComments, 0)
		}

		content := p.ContentClean
		if content == "" {
			content = p.Title
		}
		relevanceScore := relevanceScorer.Score(p.Title, content)
		noveltyScore := noveltyScorer.Score(content)

		finalScore := (communityScore * cfg.Scoring.Community) +
			(relevanceScore * cfg.Scoring.Relevance) +
			(noveltyScore * cfg.Scoring.Novelty)

		scoreRepo.Upsert(postID, communityScore, relevanceScore, noveltyScore, finalScore)
		scored++
	}
	fmt.Printf("  Scored %d posts\n", scored)

	// Stage 4: Link
	fmt.Println("→ Updating concept graph...")
	linkRepo := link.NewRepository(db)

	postTopics := make(map[int64][]string)
	for _, p := range allPosts {
		content := p.Title + " " + p.ContentClean
		topics := graph.ExtractTopics(content)
		postTopics[p.ID] = topics
	}

	linked := 0
	for i := 0; i < len(allPosts); i++ {
		for j := i + 1; j < len(allPosts); j++ {
			topicsA := postTopics[allPosts[i].ID]
			topicsB := postTopics[allPosts[j].ID]

			if len(topicsA) == 0 || len(topicsB) == 0 {
				continue
			}

			similarity := graph.ComputeTopicSimilarity(topicsA, topicsB)
			if similarity >= 0.3 {
				sharedTopics := findSharedTopics(topicsA, topicsB)
				relationship := "shared_topics:" + joinTopics(sharedTopics)
				if err := linkRepo.Upsert(allPosts[i].ID, allPosts[j].ID, relationship, similarity); err == nil {
					linked++
				}
			}
		}
	}
	fmt.Printf("  Created/updated %d links\n", linked)

	fmt.Println("→ Pipeline complete")
	return nil
}
```

**Step 2: Build and test**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go build -o blogmon . && ./blogmon daemon --help
```

Expected: Help message displays

**Step 3: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: add daemon mode for background processing"
```

---

## Task 5: Reindex Command

**Files:**
- Create: `cmd/reindex.go`

**Step 1: Create reindex command**

```go
// cmd/reindex.go
package cmd

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/search"
	"github.com/spf13/cobra"
)

var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Rebuild search index",
	Long:  `Rebuilds the full-text search index from all posts.`,
	RunE:  runReindex,
}

func init() {
	rootCmd.AddCommand(reindexCmd)
}

func runReindex(cmd *cobra.Command, args []string) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Println("Rebuilding search index...")

	searchRepo := search.NewRepository(db)
	if err := searchRepo.RebuildIndex(); err != nil {
		return fmt.Errorf("failed to rebuild index: %w", err)
	}

	fmt.Println("Search index rebuilt successfully.")
	return nil
}
```

**Step 2: Build and test**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go build -o blogmon . && ./blogmon reindex --help
```

Expected: Help message displays

**Step 3: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: add reindex command for search index rebuild"
```

---

## Task 6: Enhanced List Command with Sorting

**Files:**
- Modify: `cmd/list.go`

**Step 1: Read current list.go**

Read `/Users/julienpequegnot/Code/blogmon/cmd/list.go`

**Step 2: Add sorting options**

Add flags and sorting logic:

Add to imports if needed:
```go
"github.com/julienpequegnot/blogmon/internal/score"
```

Add variables:
```go
var (
	listLimit  int
	listSortBy string
)
```

Update init():
```go
func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().IntVarP(&listLimit, "limit", "l", 20, "Maximum posts to show")
	listCmd.Flags().StringVar(&listSortBy, "sort", "date", "Sort by: date, score, source")
}
```

Update runList to support sorting by score.

**Step 3: Build and test**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go build -o blogmon . && ./blogmon list --help
```

Expected: Help shows --sort flag

**Step 4: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: add sorting options to list command"
```

---

## Task 7: Update README and Final Tests

**Files:**
- Modify: `README.md`

**Step 1: Update README**

Update Development Status section:
```markdown
### Phase 4 (Polish) - Complete
- [x] Full-text search (SQLite FTS5)
- [x] Daemon mode (scheduled background processing)
- [x] Search command with ranked results
```

Add new commands to the Commands table:
```markdown
| `blogmon search <query>` | Full-text search across posts |
| `blogmon daemon` | Run in daemon mode for automatic fetching |
| `blogmon reindex` | Rebuild search index |
```

**Step 2: Run all tests**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go test ./... -v
```

Expected: All tests pass

**Step 3: Final commit and push**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "docs: mark Phase 4 Polish as complete"
cd /Users/julienpequegnot/Code/blogmon && git push
```

---

## Summary

**Phase 4 delivers:**
- Full-text search using SQLite FTS5
- Search command with BM25 ranking
- Daemon mode for scheduled pipeline runs
- Reindex command for search maintenance
- Enhanced list command with sorting

**New commands:**
- `blogmon search <query>` - Full-text search
- `blogmon daemon` - Background processing mode
- `blogmon reindex` - Rebuild search index

**Final pipeline:**
```
fetch → extract → score → link → search
         ↑                        ↓
      daemon (scheduled)      query results
```
