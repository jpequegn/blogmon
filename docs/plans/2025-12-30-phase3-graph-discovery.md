# Phase 3 Graph & Discovery Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a concept graph linking posts by topics, discover new blogs from references, and detect trending topics across the knowledge base.

**Architecture:** Link command builds post-to-post relationships using shared topics from insights. Discover command analyzes blog references to suggest new sources. Trends command aggregates topic frequencies over time windows to surface trending topics.

**Tech Stack:** Go, SQLite (existing links table), topic extraction from insights

---

## Task 1: Link Repository

**Files:**
- Create: `internal/link/repository.go`
- Create: `internal/link/repository_test.go`

**Step 1: Write tests**

```go
// internal/link/repository_test.go
package link

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/julienpequegnot/blogmon/internal/source"
)

func setupTestDB(t *testing.T) (*database.DB, int64, int64) {
	tmpDir := t.TempDir()
	db, err := database.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	srcRepo := source.NewRepository(db)
	src, _ := srcRepo.Add("https://test.com", "Test", "")

	postRepo := post.NewRepository(db)
	p1, _ := postRepo.Add(src.ID, "https://test.com/p1", "Post 1", "Author", time.Now(), "content")
	p2, _ := postRepo.Add(src.ID, "https://test.com/p2", "Post 2", "Author", time.Now(), "content")

	return db, p1.ID, p2.ID
}

func TestAddLink(t *testing.T) {
	db, postA, postB := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	link, err := repo.Add(postA, postB, "shared_topic", 0.8)
	if err != nil {
		t.Fatalf("failed to add link: %v", err)
	}

	if link.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestGetLinksForPost(t *testing.T) {
	db, postA, postB := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	repo.Add(postA, postB, "shared_topic", 0.8)

	links, err := repo.GetForPost(postA)
	if err != nil {
		t.Fatalf("failed to get links: %v", err)
	}

	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}
}

func TestUpsertLink(t *testing.T) {
	db, postA, postB := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	// First insert
	repo.Upsert(postA, postB, "shared_topic", 0.5)

	// Update strength
	err := repo.Upsert(postA, postB, "shared_topic", 0.9)
	if err != nil {
		t.Fatalf("failed to upsert link: %v", err)
	}

	links, _ := repo.GetForPost(postA)
	if links[0].Strength != 0.9 {
		t.Errorf("expected strength 0.9, got %f", links[0].Strength)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go test ./internal/link/... -v
```

Expected: FAIL - package doesn't exist

**Step 3: Implement repository**

```go
// internal/link/repository.go
package link

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type Link struct {
	ID           int64
	PostIDA      int64
	PostIDB      int64
	Relationship string
	Strength     float64
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Add(postIDA, postIDB int64, relationship string, strength float64) (*Link, error) {
	// Ensure consistent ordering (smaller ID first)
	if postIDA > postIDB {
		postIDA, postIDB = postIDB, postIDA
	}

	result, err := r.db.Exec(
		`INSERT INTO links (post_id_a, post_id_b, relationship, strength) VALUES (?, ?, ?, ?)`,
		postIDA, postIDB, relationship, strength,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert link: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Link{
		ID:           id,
		PostIDA:      postIDA,
		PostIDB:      postIDB,
		Relationship: relationship,
		Strength:     strength,
	}, nil
}

func (r *Repository) Upsert(postIDA, postIDB int64, relationship string, strength float64) error {
	// Ensure consistent ordering
	if postIDA > postIDB {
		postIDA, postIDB = postIDB, postIDA
	}

	_, err := r.db.Exec(`
		INSERT INTO links (post_id_a, post_id_b, relationship, strength)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(post_id_a, post_id_b, relationship) DO UPDATE SET
			strength = excluded.strength
	`, postIDA, postIDB, relationship, strength)
	return err
}

func (r *Repository) GetForPost(postID int64) ([]Link, error) {
	rows, err := r.db.Query(`
		SELECT id, post_id_a, post_id_b, relationship, strength
		FROM links
		WHERE post_id_a = ? OR post_id_b = ?
		ORDER BY strength DESC
	`, postID, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.PostIDA, &l.PostIDB, &l.Relationship, &l.Strength); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (r *Repository) GetRelatedPosts(postID int64, limit int) ([]int64, error) {
	rows, err := r.db.Query(`
		SELECT CASE WHEN post_id_a = ? THEN post_id_b ELSE post_id_a END as related_id
		FROM links
		WHERE post_id_a = ? OR post_id_b = ?
		ORDER BY strength DESC
		LIMIT ?
	`, postID, postID, postID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) DeleteForPost(postID int64) error {
	_, err := r.db.Exec(`DELETE FROM links WHERE post_id_a = ? OR post_id_b = ?`, postID, postID)
	return err
}
```

**Step 4: Run tests**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go test ./internal/link/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: add link repository for concept graph"
```

---

## Task 2: Topic Extractor

**Files:**
- Create: `internal/graph/topics.go`
- Create: `internal/graph/topics_test.go`

**Step 1: Write tests**

```go
// internal/graph/topics_test.go
package graph

import (
	"testing"
)

func TestExtractTopics(t *testing.T) {
	content := "This article discusses golang concurrency patterns and goroutines for distributed systems"

	topics := ExtractTopics(content)

	if len(topics) == 0 {
		t.Error("expected topics to be extracted")
	}

	// Should find "golang" as a topic
	found := false
	for _, topic := range topics {
		if topic == "golang" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'golang' to be extracted as a topic")
	}
}

func TestComputeTopicSimilarity(t *testing.T) {
	topics1 := []string{"golang", "concurrency", "performance"}
	topics2 := []string{"golang", "performance", "rust"}

	similarity := ComputeTopicSimilarity(topics1, topics2)

	if similarity <= 0 {
		t.Error("expected positive similarity for overlapping topics")
	}
	if similarity > 1.0 {
		t.Error("expected similarity <= 1.0")
	}
}

func TestComputeTopicSimilarityNoOverlap(t *testing.T) {
	topics1 := []string{"golang", "concurrency"}
	topics2 := []string{"python", "machine-learning"}

	similarity := ComputeTopicSimilarity(topics1, topics2)

	if similarity != 0 {
		t.Errorf("expected 0 similarity for non-overlapping topics, got %f", similarity)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go test ./internal/graph/... -v
```

Expected: FAIL

**Step 3: Implement topic extractor**

```go
// internal/graph/topics.go
package graph

import (
	"regexp"
	"strings"
)

// Common tech topics to detect
var techTopics = map[string][]string{
	"golang":              {"go", "golang", "goroutine", "goroutines"},
	"rust":                {"rust", "rustlang", "cargo", "ownership"},
	"python":              {"python", "django", "flask", "pytorch"},
	"javascript":          {"javascript", "typescript", "nodejs", "react", "vue"},
	"distributed-systems": {"distributed", "consensus", "raft", "paxos", "microservices"},
	"databases":           {"database", "sql", "postgresql", "mysql", "redis", "mongodb"},
	"kubernetes":          {"kubernetes", "k8s", "docker", "containers", "helm"},
	"performance":         {"performance", "optimization", "latency", "throughput", "benchmark"},
	"security":            {"security", "authentication", "encryption", "vulnerability"},
	"machine-learning":    {"machine learning", "ml", "neural", "tensorflow", "pytorch"},
	"devops":              {"devops", "ci/cd", "jenkins", "github actions", "terraform"},
	"architecture":        {"architecture", "design patterns", "solid", "clean architecture"},
	"testing":             {"testing", "unit test", "integration test", "tdd"},
	"concurrency":         {"concurrency", "parallel", "async", "threads", "mutex"},
	"api":                 {"api", "rest", "graphql", "grpc", "openapi"},
}

func ExtractTopics(content string) []string {
	content = strings.ToLower(content)
	var found []string

	for topic, keywords := range techTopics {
		for _, keyword := range keywords {
			if strings.Contains(content, keyword) {
				found = append(found, topic)
				break
			}
		}
	}

	return found
}

func ExtractTopicsFromInsights(insights []string) []string {
	combined := strings.Join(insights, " ")
	return ExtractTopics(combined)
}

func ComputeTopicSimilarity(topics1, topics2 []string) float64 {
	if len(topics1) == 0 || len(topics2) == 0 {
		return 0
	}

	set1 := make(map[string]bool)
	for _, t := range topics1 {
		set1[t] = true
	}

	set2 := make(map[string]bool)
	for _, t := range topics2 {
		set2[t] = true
	}

	// Jaccard similarity: intersection / union
	intersection := 0
	for t := range set1 {
		if set2[t] {
			intersection++
		}
	}

	union := len(set1)
	for t := range set2 {
		if !set1[t] {
			union++
		}
	}

	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// ExtractKeywords extracts significant words from content
func ExtractKeywords(content string, minLen int) []string {
	content = strings.ToLower(content)

	// Remove common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"this": true, "that": true, "these": true, "those": true,
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "what": true, "which": true, "who": true,
		"when": true, "where": true, "why": true, "how": true,
		"all": true, "each": true, "every": true, "both": true, "few": true,
		"more": true, "most": true, "other": true, "some": true, "such": true,
		"no": true, "not": true, "only": true, "same": true, "so": true,
		"than": true, "too": true, "very": true, "just": true, "also": true,
	}

	// Extract words
	re := regexp.MustCompile(`[a-z]+`)
	words := re.FindAllString(content, -1)

	seen := make(map[string]bool)
	var keywords []string

	for _, word := range words {
		if len(word) >= minLen && !stopWords[word] && !seen[word] {
			seen[word] = true
			keywords = append(keywords, word)
		}
	}

	return keywords
}
```

**Step 4: Run tests**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go test ./internal/graph/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: add topic extraction for concept graph"
```

---

## Task 3: Link Command

**Files:**
- Create: `cmd/link.go`

**Step 1: Create link command**

```go
// cmd/link.go
package cmd

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/graph"
	"github.com/julienpequegnot/blogmon/internal/insight"
	"github.com/julienpequegnot/blogmon/internal/link"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Build concept graph by linking related posts",
	Long:  `Analyzes posts and creates links between those sharing similar topics.`,
	RunE:  runLink,
}

var (
	linkMinSimilarity float64
	linkRebuild       bool
)

func init() {
	rootCmd.AddCommand(linkCmd)
	linkCmd.Flags().Float64Var(&linkMinSimilarity, "min-similarity", 0.3, "Minimum similarity threshold for linking")
	linkCmd.Flags().BoolVar(&linkRebuild, "rebuild", false, "Rebuild all links from scratch")
}

func runLink(cmd *cobra.Command, args []string) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	postRepo := post.NewRepository(db)
	insightRepo := insight.NewRepository(db)
	linkRepo := link.NewRepository(db)

	// Get all posts
	posts, err := postRepo.List(1000, 0)
	if err != nil {
		return err
	}

	if len(posts) < 2 {
		fmt.Println("Need at least 2 posts to build links.")
		return nil
	}

	fmt.Printf("Analyzing %d posts for topic similarity\n\n", len(posts))

	// Extract topics for each post
	postTopics := make(map[int64][]string)
	for _, p := range posts {
		// Get insights for this post
		insights, _ := insightRepo.ListForPost(p.ID)

		var insightTexts []string
		for _, ins := range insights {
			insightTexts = append(insightTexts, ins.Content)
		}

		// Extract topics from title + content + insights
		content := p.Title + " " + p.ContentClean
		if len(insightTexts) > 0 {
			content += " " + fmt.Sprintf("%v", insightTexts)
		}

		topics := graph.ExtractTopics(content)
		postTopics[p.ID] = topics

		if len(topics) > 0 {
			fmt.Printf("  %s: %v\n", truncate(p.Title, 40), topics)
		}
	}

	fmt.Println()

	// Compare all pairs and create links
	linksCreated := 0
	for i := 0; i < len(posts); i++ {
		for j := i + 1; j < len(posts); j++ {
			postA := posts[i]
			postB := posts[j]

			topicsA := postTopics[postA.ID]
			topicsB := postTopics[postB.ID]

			if len(topicsA) == 0 || len(topicsB) == 0 {
				continue
			}

			similarity := graph.ComputeTopicSimilarity(topicsA, topicsB)

			if similarity >= linkMinSimilarity {
				// Find shared topics for relationship description
				sharedTopics := findSharedTopics(topicsA, topicsB)
				relationship := "shared_topics:" + joinTopics(sharedTopics)

				if err := linkRepo.Upsert(postA.ID, postB.ID, relationship, similarity); err != nil {
					fmt.Printf("Error linking posts: %v\n", err)
					continue
				}

				fmt.Printf("Linked: '%s' <-> '%s' (%.2f)\n",
					truncate(postA.Title, 25),
					truncate(postB.Title, 25),
					similarity)
				linksCreated++
			}
		}
	}

	fmt.Printf("\nCreated %d links\n", linksCreated)
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func findSharedTopics(a, b []string) []string {
	setB := make(map[string]bool)
	for _, t := range b {
		setB[t] = true
	}

	var shared []string
	for _, t := range a {
		if setB[t] {
			shared = append(shared, t)
		}
	}
	return shared
}

func joinTopics(topics []string) string {
	if len(topics) == 0 {
		return "general"
	}
	result := topics[0]
	for i := 1; i < len(topics) && i < 3; i++ {
		result += "," + topics[i]
	}
	return result
}
```

**Step 2: Build and test**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go build -o blogmon . && ./blogmon link --help
```

Expected: Help message displays

**Step 3: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: add link command for concept graph building"
```

---

## Task 4: Related Posts in Show Command

**Files:**
- Modify: `cmd/show.go`

**Step 1: Read current show.go**

Read the file to understand current structure.

**Step 2: Add related posts section**

Add import:
```go
"github.com/julienpequegnot/blogmon/internal/link"
```

Add after references section in runShow:
```go
	// Show related posts
	linkRepo := link.NewRepository(db)
	relatedIDs, _ := linkRepo.GetRelatedPosts(id, 5)
	if len(relatedIDs) > 0 {
		fmt.Printf("\n%s\n", labelStyle.Render("RELATED POSTS:"))
		for _, relID := range relatedIDs {
			if relPost, err := postRepo.Get(relID); err == nil {
				fmt.Printf("  → [%d] %s\n", relPost.ID, relPost.Title)
			}
		}
	}
```

**Step 3: Build and test**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go build -o blogmon .
```

Expected: Build succeeds

**Step 4: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: show related posts in show command"
```

---

## Task 5: Discover Command (Blog Discovery)

**Files:**
- Create: `cmd/discover.go`
- Modify: `internal/source/repository.go` (add GetByURL method)

**Step 1: Add GetByURL to source repository**

First read `/Users/julienpequegnot/Code/blogmon/internal/source/repository.go`.

Add this method:
```go
func (r *Repository) GetByURL(url string) (*Source, error) {
	var s Source
	err := r.db.QueryRow(`
		SELECT id, url, name, feed_url, discovered_from, last_fetched, active, created_at
		FROM sources WHERE url = ?
	`, url).Scan(&s.ID, &s.URL, &s.Name, &s.FeedURL, &s.DiscoveredFrom, &s.LastFetched, &s.Active, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) AddDiscovered(url, name, feedURL string, discoveredFromPostID int64) (*Source, error) {
	result, err := r.db.Exec(
		`INSERT INTO sources (url, name, feed_url, discovered_from, active) VALUES (?, ?, ?, ?, TRUE)`,
		url, name, feedURL, discoveredFromPostID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert source: %w", err)
	}

	id, _ := result.LastInsertId()
	return &Source{
		ID:             id,
		URL:            url,
		Name:           name,
		FeedURL:        feedURL,
		DiscoveredFrom: &discoveredFromPostID,
		Active:         true,
	}, nil
}
```

**Step 2: Create discover command**

```go
// cmd/discover.go
package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/feed"
	"github.com/julienpequegnot/blogmon/internal/reference"
	"github.com/julienpequegnot/blogmon/internal/source"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover new blogs from references",
	Long:  `Analyzes blog references in posts to discover and optionally add new sources.`,
	RunE:  runDiscover,
}

var (
	discoverAutoAdd bool
	discoverLimit   int
)

func init() {
	rootCmd.AddCommand(discoverCmd)
	discoverCmd.Flags().BoolVar(&discoverAutoAdd, "auto-add", false, "Automatically add discovered blogs")
	discoverCmd.Flags().IntVarP(&discoverLimit, "limit", "l", 20, "Maximum blogs to discover")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	refRepo := reference.NewRepository(db)
	srcRepo := source.NewRepository(db)

	// Get all blog references
	blogRefs, err := refRepo.ListBlogReferences()
	if err != nil {
		return err
	}

	if len(blogRefs) == 0 {
		fmt.Println("No blog references found. Run 'blogmon extract' first.")
		return nil
	}

	fmt.Printf("Analyzing %d blog references\n\n", len(blogRefs))

	// Group by domain and count
	domainCounts := make(map[string]int)
	domainPostIDs := make(map[string]int64) // Track which post referenced it
	domainURLs := make(map[string]string)

	for _, ref := range blogRefs {
		parsed, err := url.Parse(ref.URL)
		if err != nil {
			continue
		}

		domain := parsed.Host
		// Normalize domain
		domain = strings.TrimPrefix(domain, "www.")

		domainCounts[domain]++
		if _, exists := domainPostIDs[domain]; !exists {
			domainPostIDs[domain] = ref.PostID
			domainURLs[domain] = "https://" + domain
		}
	}

	// Sort by count and filter to limit
	type domainEntry struct {
		domain string
		count  int
	}
	var entries []domainEntry
	for domain, count := range domainCounts {
		entries = append(entries, domainEntry{domain, count})
	}

	// Simple bubble sort by count (descending)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].count > entries[i].count {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	if len(entries) > discoverLimit {
		entries = entries[:discoverLimit]
	}

	discovered := 0
	added := 0

	for _, entry := range entries {
		domain := entry.domain
		count := entry.count
		refURL := domainURLs[domain]

		// Check if already a source
		if _, err := srcRepo.GetByURL(refURL); err == nil {
			continue // Already exists
		}

		fmt.Printf("Discovered: %s (referenced %d times)\n", domain, count)
		discovered++

		if discoverAutoAdd {
			// Try to find RSS feed
			feedURL, err := feed.Discover(refURL)
			if err != nil || feedURL == "" {
				fmt.Printf("  → Could not find RSS feed\n")
				continue
			}

			// Add as discovered source
			postID := domainPostIDs[domain]
			_, err = srcRepo.AddDiscovered(refURL, domain, feedURL, postID)
			if err != nil {
				fmt.Printf("  → Error adding: %v\n", err)
				continue
			}

			fmt.Printf("  → Added with feed: %s\n", feedURL)
			added++
		}
	}

	fmt.Printf("\nDiscovered %d new blogs", discovered)
	if discoverAutoAdd {
		fmt.Printf(", added %d", added)
	}
	fmt.Println()

	if !discoverAutoAdd && discovered > 0 {
		fmt.Println("\nRun with --auto-add to automatically add discovered blogs.")
	}

	return nil
}
```

**Step 3: Build and test**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go build -o blogmon . && ./blogmon discover --help
```

Expected: Help message displays

**Step 4: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: add discover command for blog discovery"
```

---

## Task 6: Trends Command

**Files:**
- Create: `internal/graph/trends.go`
- Create: `internal/graph/trends_test.go`
- Create: `cmd/trends.go`

**Step 1: Write trend analyzer tests**

```go
// internal/graph/trends_test.go
package graph

import (
	"testing"
	"time"
)

func TestTrendAnalyzer(t *testing.T) {
	analyzer := NewTrendAnalyzer()

	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, -1, 0)

	// Add recent posts about golang
	analyzer.AddPost(1, []string{"golang", "performance"}, now)
	analyzer.AddPost(2, []string{"golang", "concurrency"}, now.AddDate(0, 0, -1))
	analyzer.AddPost(3, []string{"golang"}, now.AddDate(0, 0, -2))

	// Add older posts about rust
	analyzer.AddPost(4, []string{"rust", "performance"}, monthAgo)
	analyzer.AddPost(5, []string{"rust"}, monthAgo)

	trends := analyzer.GetTrends(7, 5)

	if len(trends) == 0 {
		t.Error("expected trends to be returned")
	}

	// Golang should be trending (3 recent posts)
	if trends[0].Topic != "golang" {
		t.Errorf("expected 'golang' to be top trend, got '%s'", trends[0].Topic)
	}
}

func TestTrendScore(t *testing.T) {
	analyzer := NewTrendAnalyzer()

	now := time.Now()

	// Add posts
	analyzer.AddPost(1, []string{"trending"}, now)
	analyzer.AddPost(2, []string{"old"}, now.AddDate(0, -2, 0))

	trends := analyzer.GetTrends(7, 10)

	// "trending" should have higher score than "old"
	var trendingScore, oldScore float64
	for _, t := range trends {
		if t.Topic == "trending" {
			trendingScore = t.Score
		}
		if t.Topic == "old" {
			oldScore = t.Score
		}
	}

	if trendingScore <= oldScore {
		t.Errorf("expected trending score (%f) > old score (%f)", trendingScore, oldScore)
	}
}
```

**Step 2: Implement trend analyzer**

```go
// internal/graph/trends.go
package graph

import (
	"sort"
	"time"
)

type Trend struct {
	Topic      string
	Count      int
	Score      float64
	RecentPosts []int64
}

type TrendAnalyzer struct {
	postTopics map[int64]topicEntry
}

type topicEntry struct {
	topics    []string
	timestamp time.Time
}

func NewTrendAnalyzer() *TrendAnalyzer {
	return &TrendAnalyzer{
		postTopics: make(map[int64]topicEntry),
	}
}

func (ta *TrendAnalyzer) AddPost(postID int64, topics []string, publishedAt time.Time) {
	ta.postTopics[postID] = topicEntry{
		topics:    topics,
		timestamp: publishedAt,
	}
}

func (ta *TrendAnalyzer) GetTrends(days int, limit int) []Trend {
	cutoff := time.Now().AddDate(0, 0, -days)

	// Count topics and track recency
	topicCounts := make(map[string]int)
	topicPosts := make(map[string][]int64)
	topicRecency := make(map[string]time.Time)

	for postID, entry := range ta.postTopics {
		for _, topic := range entry.topics {
			topicCounts[topic]++
			topicPosts[topic] = append(topicPosts[topic], postID)

			// Track most recent post for each topic
			if existing, ok := topicRecency[topic]; !ok || entry.timestamp.After(existing) {
				topicRecency[topic] = entry.timestamp
			}
		}
	}

	// Calculate trend scores
	var trends []Trend
	for topic, count := range topicCounts {
		// Count recent posts
		recentCount := 0
		var recentPosts []int64
		for _, postID := range topicPosts[topic] {
			if entry, ok := ta.postTopics[postID]; ok && entry.timestamp.After(cutoff) {
				recentCount++
				recentPosts = append(recentPosts, postID)
			}
		}

		// Score: recent count weighted higher than total count
		// Also factor in how recent the most recent post is
		recencyBoost := 1.0
		if mostRecent, ok := topicRecency[topic]; ok {
			daysSince := time.Since(mostRecent).Hours() / 24
			if daysSince < float64(days) {
				recencyBoost = 1.0 + (float64(days)-daysSince)/float64(days)
			}
		}

		score := (float64(recentCount)*2 + float64(count)) * recencyBoost

		trends = append(trends, Trend{
			Topic:       topic,
			Count:       count,
			Score:       score,
			RecentPosts: recentPosts,
		})
	}

	// Sort by score
	sort.Slice(trends, func(i, j int) bool {
		return trends[i].Score > trends[j].Score
	})

	if len(trends) > limit {
		trends = trends[:limit]
	}

	return trends
}
```

**Step 3: Run tests**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go test ./internal/graph/... -v
```

Expected: All tests PASS

**Step 4: Create trends command**

```go
// cmd/trends.go
package cmd

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/graph"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var trendsCmd = &cobra.Command{
	Use:   "trends",
	Short: "Show trending topics",
	Long:  `Analyzes posts to identify trending topics based on recency and frequency.`,
	RunE:  runTrends,
}

var (
	trendsDays  int
	trendsLimit int
)

func init() {
	rootCmd.AddCommand(trendsCmd)
	trendsCmd.Flags().IntVar(&trendsDays, "days", 30, "Time window in days")
	trendsCmd.Flags().IntVarP(&trendsLimit, "limit", "l", 10, "Maximum trends to show")
}

func runTrends(cmd *cobra.Command, args []string) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	postRepo := post.NewRepository(db)

	// Get all posts
	posts, err := postRepo.List(1000, 0)
	if err != nil {
		return err
	}

	if len(posts) == 0 {
		fmt.Println("No posts found.")
		return nil
	}

	// Build trend analyzer
	analyzer := graph.NewTrendAnalyzer()

	for _, p := range posts {
		content := p.Title + " " + p.ContentClean
		topics := graph.ExtractTopics(content)
		analyzer.AddPost(p.ID, topics, p.PublishedAt)
	}

	trends := analyzer.GetTrends(trendsDays, trendsLimit)

	if len(trends) == 0 {
		fmt.Println("No trending topics found.")
		return nil
	}

	// Display trends
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	fmt.Printf("\n%s (last %d days)\n\n", titleStyle.Render("TRENDING TOPICS"), trendsDays)

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	maxScore := trends[0].Score

	for i, trend := range trends {
		// Create bar visualization
		barWidth := int((trend.Score / maxScore) * 20)
		bar := ""
		for j := 0; j < barWidth; j++ {
			bar += "█"
		}

		fmt.Printf("%2d. %-20s %s %.1f (%d posts, %d recent)\n",
			i+1,
			trend.Topic,
			barStyle.Render(bar),
			trend.Score,
			trend.Count,
			len(trend.RecentPosts))
	}

	fmt.Println()
	return nil
}
```

**Step 5: Build and test**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go build -o blogmon . && ./blogmon trends --help
```

Expected: Help message displays

**Step 6: Commit**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "feat: add trends command for trend detection"
```

---

## Task 7: Update README and Final Tests

**Files:**
- Modify: `README.md`

**Step 1: Update README**

Update Development Status section:
```markdown
### Phase 3 (Graph & Discovery) - Complete
- [x] Concept graph (topic-based post linking)
- [x] Blog discovery (from references)
- [x] Trend detection (topic trending analysis)
```

Add new commands to the Commands table:
```markdown
| `blogmon link` | Build concept graph by linking related posts |
| `blogmon discover` | Discover new blogs from references |
| `blogmon trends` | Show trending topics |
```

**Step 2: Run all tests**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon && go test ./... -v
```

Expected: All tests pass

**Step 3: Final commit and push**

```bash
cd /Users/julienpequegnot/Code/blogmon && git add . && git commit -m "docs: mark Phase 3 Graph & Discovery as complete"
cd /Users/julienpequegnot/Code/blogmon && git push
```

---

## Summary

**Phase 3 delivers:**
- Link repository for concept graph storage
- Topic extractor with tech topic detection
- Link command to build post-to-post relationships
- Related posts display in show command
- Discover command for finding new blogs from references
- Trend analyzer with recency-weighted scoring
- Trends command for viewing trending topics

**New commands:**
- `blogmon link` - Build concept graph
- `blogmon discover` - Find new blogs from references
- `blogmon trends` - Show trending topics

**Next: Phase 4 (Polish)**
- Semantic search
- Daemon mode
- Full-text search
