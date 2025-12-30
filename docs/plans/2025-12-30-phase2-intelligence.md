# Phase 2 Intelligence Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add LLM-powered insight extraction and community-based scoring to identify high-value blog posts.

**Architecture:** Extract command uses Ollama API to parse posts and generate insights/references. Score command queries HN Algolia API for community signals, computes relevance against user interests, and calculates novelty using TF-IDF. All scores stored in SQLite scores table.

**Tech Stack:** Go, Ollama API (HTTP), HN Algolia API, TF-IDF (custom), SQLite

---

## Task 1: LLM Client Package

**Files:**
- Create: `internal/llm/client.go`
- Create: `internal/llm/client_test.go`

**Step 1: Write test for LLM client**

```go
// internal/llm/client_test.go
package llm

import (
	"context"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:11434", "llama3.2", 30*time.Second)

	if client.baseURL != "http://localhost:11434" {
		t.Errorf("expected baseURL http://localhost:11434, got %s", client.baseURL)
	}
	if client.model != "llama3.2" {
		t.Errorf("expected model llama3.2, got %s", client.model)
	}
}

func TestGeneratePrompt(t *testing.T) {
	client := NewClient("http://localhost:11434", "llama3.2", 30*time.Second)

	prompt := client.BuildExtractionPrompt("Test Title", "Test content about Go programming.")

	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/julienpequegnot/Code/blogmon
go test ./internal/llm/... -v
```

Expected: FAIL - package doesn't exist

**Step 3: Implement LLM client**

```go
// internal/llm/client.go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type ExtractionResult struct {
	Takeaways  []string `json:"takeaways"`
	References []struct {
		URL     string `json:"url"`
		Title   string `json:"title"`
		Context string `json:"context"`
	} `json:"references"`
	Topics []string `json:"topics"`
}

func NewClient(baseURL, model string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) BuildExtractionPrompt(title, content string) string {
	return fmt.Sprintf(`Analyze this blog post and extract structured information.

Title: %s

Content:
%s

Return a JSON object with:
1. "takeaways": Array of 3-5 key insights (short sentences)
2. "references": Array of objects with "url", "title", "context" for any links mentioned
3. "topics": Array of 2-4 topic tags (e.g., "golang", "distributed-systems", "performance")

Return ONLY valid JSON, no other text.`, title, content)
}

func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	reqBody := GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API error %d: %s", resp.StatusCode, string(body))
	}

	var result GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Response, nil
}

func (c *Client) ExtractInsights(ctx context.Context, title, content string) (*ExtractionResult, error) {
	prompt := c.BuildExtractionPrompt(title, content)

	response, err := c.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Try to parse JSON from response
	var result ExtractionResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// Try to find JSON in response
		start := bytes.IndexByte([]byte(response), '{')
		end := bytes.LastIndexByte([]byte(response), '}')
		if start >= 0 && end > start {
			if err := json.Unmarshal([]byte(response[start:end+1]), &result); err != nil {
				return nil, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
			}
		} else {
			return nil, fmt.Errorf("no JSON found in LLM response")
		}
	}

	return &result, nil
}
```

**Step 4: Run tests**

Run:
```bash
go test ./internal/llm/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add .
git commit -m "feat: add LLM client package for Ollama integration"
```

---

## Task 2: Insights Repository

**Files:**
- Create: `internal/insight/repository.go`
- Create: `internal/insight/repository_test.go`

**Step 1: Write tests**

```go
// internal/insight/repository_test.go
package insight

import (
	"path/filepath"
	"testing"

	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/source"
	"github.com/julienpequegnot/blogmon/internal/post"
	"time"
)

func setupTestDB(t *testing.T) (*database.DB, int64) {
	tmpDir := t.TempDir()
	db, err := database.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create a source and post for testing
	srcRepo := source.NewRepository(db)
	src, _ := srcRepo.Add("https://test.com", "Test", "")

	postRepo := post.NewRepository(db)
	p, _ := postRepo.Add(src.ID, "https://test.com/p1", "Test Post", "Author", time.Now(), "content")

	return db, p.ID
}

func TestAddInsight(t *testing.T) {
	db, postID := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	insight, err := repo.Add(postID, "takeaway", "Go is great for concurrency", 5)
	if err != nil {
		t.Fatalf("failed to add insight: %v", err)
	}

	if insight.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestListInsightsForPost(t *testing.T) {
	db, postID := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	repo.Add(postID, "takeaway", "Insight 1", 5)
	repo.Add(postID, "takeaway", "Insight 2", 4)

	insights, err := repo.ListForPost(postID)
	if err != nil {
		t.Fatalf("failed to list insights: %v", err)
	}

	if len(insights) != 2 {
		t.Errorf("expected 2 insights, got %d", len(insights))
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/insight/... -v
```

Expected: FAIL

**Step 3: Implement repository**

```go
// internal/insight/repository.go
package insight

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type Insight struct {
	ID         int64
	PostID     int64
	Type       string // "takeaway", "code_example", "quote", "definition"
	Content    string
	Importance int
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Add(postID int64, insightType, content string, importance int) (*Insight, error) {
	result, err := r.db.Exec(
		`INSERT INTO insights (post_id, type, content, importance) VALUES (?, ?, ?, ?)`,
		postID, insightType, content, importance,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert insight: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Insight{
		ID:         id,
		PostID:     postID,
		Type:       insightType,
		Content:    content,
		Importance: importance,
	}, nil
}

func (r *Repository) ListForPost(postID int64) ([]Insight, error) {
	rows, err := r.db.Query(
		`SELECT id, post_id, type, content, importance FROM insights WHERE post_id = ? ORDER BY importance DESC`,
		postID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var insights []Insight
	for rows.Next() {
		var i Insight
		if err := rows.Scan(&i.ID, &i.PostID, &i.Type, &i.Content, &i.Importance); err != nil {
			return nil, err
		}
		insights = append(insights, i)
	}
	return insights, rows.Err()
}

func (r *Repository) DeleteForPost(postID int64) error {
	_, err := r.db.Exec(`DELETE FROM insights WHERE post_id = ?`, postID)
	return err
}
```

**Step 4: Run tests**

Run:
```bash
go test ./internal/insight/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add .
git commit -m "feat: add insight repository"
```

---

## Task 3: References Repository

**Files:**
- Create: `internal/reference/repository.go`
- Create: `internal/reference/repository_test.go`

**Step 1: Write tests**

```go
// internal/reference/repository_test.go
package reference

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/julienpequegnot/blogmon/internal/source"
)

func setupTestDB(t *testing.T) (*database.DB, int64) {
	tmpDir := t.TempDir()
	db, err := database.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	srcRepo := source.NewRepository(db)
	src, _ := srcRepo.Add("https://test.com", "Test", "")

	postRepo := post.NewRepository(db)
	p, _ := postRepo.Add(src.ID, "https://test.com/p1", "Test Post", "Author", time.Now(), "content")

	return db, p.ID
}

func TestAddReference(t *testing.T) {
	db, postID := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	ref, err := repo.Add(postID, "https://example.com", "Example Site", "mentioned in context", true)
	if err != nil {
		t.Fatalf("failed to add reference: %v", err)
	}

	if ref.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if !ref.IsBlog {
		t.Error("expected IsBlog to be true")
	}
}

func TestListReferencesForPost(t *testing.T) {
	db, postID := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	repo.Add(postID, "https://example1.com", "Example 1", "", false)
	repo.Add(postID, "https://example2.com", "Example 2", "", true)

	refs, err := repo.ListForPost(postID)
	if err != nil {
		t.Fatalf("failed to list references: %v", err)
	}

	if len(refs) != 2 {
		t.Errorf("expected 2 references, got %d", len(refs))
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/reference/... -v
```

Expected: FAIL

**Step 3: Implement repository**

```go
// internal/reference/repository.go
package reference

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type Reference struct {
	ID      int64
	PostID  int64
	URL     string
	Title   string
	Context string
	IsBlog  bool
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Add(postID int64, url, title, context string, isBlog bool) (*Reference, error) {
	result, err := r.db.Exec(
		`INSERT INTO refs (post_id, url, title, context, is_blog) VALUES (?, ?, ?, ?, ?)`,
		postID, url, title, context, isBlog,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert reference: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Reference{
		ID:      id,
		PostID:  postID,
		URL:     url,
		Title:   title,
		Context: context,
		IsBlog:  isBlog,
	}, nil
}

func (r *Repository) ListForPost(postID int64) ([]Reference, error) {
	rows, err := r.db.Query(
		`SELECT id, post_id, url, title, context, is_blog FROM refs WHERE post_id = ?`,
		postID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []Reference
	for rows.Next() {
		var ref Reference
		if err := rows.Scan(&ref.ID, &ref.PostID, &ref.URL, &ref.Title, &ref.Context, &ref.IsBlog); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (r *Repository) DeleteForPost(postID int64) error {
	_, err := r.db.Exec(`DELETE FROM refs WHERE post_id = ?`, postID)
	return err
}

func (r *Repository) ListBlogReferences() ([]Reference, error) {
	rows, err := r.db.Query(`SELECT id, post_id, url, title, context, is_blog FROM refs WHERE is_blog = TRUE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []Reference
	for rows.Next() {
		var ref Reference
		if err := rows.Scan(&ref.ID, &ref.PostID, &ref.URL, &ref.Title, &ref.Context, &ref.IsBlog); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}
```

**Step 4: Run tests**

Run:
```bash
go test ./internal/reference/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add .
git commit -m "feat: add reference repository"
```

---

## Task 4: Extract Command

**Files:**
- Create: `cmd/extract.go`
- Modify: `internal/post/repository.go` (add GetUnextracted method)

**Step 1: Add GetUnextracted to post repository**

```go
// Add to internal/post/repository.go

func (r *Repository) GetUnextracted(limit int) ([]Post, error) {
	rows, err := r.db.Query(`
		SELECT p.id, p.source_id, s.name, p.url, p.title, p.author, p.published_at, p.fetched_at,
		       p.content_raw, p.content_clean, COALESCE(p.word_count, 0)
		FROM posts p
		JOIN sources s ON p.source_id = s.id
		WHERE p.content_clean IS NULL OR p.content_clean = ''
		ORDER BY p.published_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.SourceID, &p.SourceName, &p.URL, &p.Title, &p.Author,
			&p.PublishedAt, &p.FetchedAt, &p.ContentRaw, &p.ContentClean, &p.WordCount); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (r *Repository) UpdateContentClean(id int64, contentClean string, wordCount int) error {
	_, err := r.db.Exec(
		`UPDATE posts SET content_clean = ?, word_count = ? WHERE id = ?`,
		contentClean, wordCount, id,
	)
	return err
}
```

**Step 2: Create extract command**

```go
// cmd/extract.go
package cmd

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/insight"
	"github.com/julienpequegnot/blogmon/internal/llm"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/julienpequegnot/blogmon/internal/reference"
	"github.com/spf13/cobra"
)

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract insights from posts using LLM",
	Long:  `Analyzes unprocessed posts and extracts key takeaways, references, and topics.`,
	RunE:  runExtract,
}

var (
	extractLimit int
	extractSkipErrors bool
)

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.Flags().IntVarP(&extractLimit, "limit", "l", 10, "Maximum posts to process")
	extractCmd.Flags().BoolVar(&extractSkipErrors, "skip-errors", false, "Continue on extraction errors")
}

func runExtract(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	postRepo := post.NewRepository(db)
	insightRepo := insight.NewRepository(db)
	refRepo := reference.NewRepository(db)

	// Get unextracted posts
	posts, err := postRepo.GetUnextracted(extractLimit)
	if err != nil {
		return err
	}

	if len(posts) == 0 {
		fmt.Println("No unprocessed posts found.")
		return nil
	}

	fmt.Printf("Found %d posts to process\n\n", len(posts))

	// Initialize LLM client
	llmClient := llm.NewClient(
		"http://localhost:11434",
		cfg.APIs.LLMModel,
		2*time.Minute,
	)

	processed := 0
	for _, p := range posts {
		fmt.Printf("Extracting: %s\n", p.Title)

		// Clean HTML content
		cleanContent := stripHTMLTags(p.ContentRaw)
		if len(cleanContent) < 100 {
			fmt.Printf("  Skipping (content too short)\n")
			continue
		}

		// Truncate for LLM context
		if len(cleanContent) > 8000 {
			cleanContent = cleanContent[:8000]
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		result, err := llmClient.ExtractInsights(ctx, p.Title, cleanContent)
		cancel()

		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			if !extractSkipErrors {
				return err
			}
			continue
		}

		// Save insights
		for i, takeaway := range result.Takeaways {
			importance := 5 - i // First takeaway most important
			if importance < 1 {
				importance = 1
			}
			insightRepo.Add(p.ID, "takeaway", takeaway, importance)
		}

		// Save references
		for _, ref := range result.References {
			isBlog := isBlogURL(ref.URL)
			refRepo.Add(p.ID, ref.URL, ref.Title, ref.Context, isBlog)
		}

		// Update post with clean content
		wordCount := len(strings.Fields(cleanContent))
		postRepo.UpdateContentClean(p.ID, cleanContent, wordCount)

		fmt.Printf("  Extracted %d takeaways, %d references\n", len(result.Takeaways), len(result.References))
		processed++
	}

	fmt.Printf("\nProcessed %d posts\n", processed)
	return nil
}

func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	clean := re.ReplaceAllString(s, " ")
	clean = strings.Join(strings.Fields(clean), " ")
	return strings.TrimSpace(clean)
}

func isBlogURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	// Common blog indicators
	blogPatterns := []string{
		"blog", "post", "article", "writing",
	}

	host := strings.ToLower(parsed.Host)
	path := strings.ToLower(parsed.Path)

	for _, pattern := range blogPatterns {
		if strings.Contains(host, pattern) || strings.Contains(path, pattern) {
			return true
		}
	}

	// Known blog platforms
	blogDomains := []string{
		"medium.com", "dev.to", "hashnode", "substack.com",
	}

	for _, domain := range blogDomains {
		if strings.Contains(host, domain) {
			return true
		}
	}

	return false
}
```

**Step 3: Build and test**

Run:
```bash
go build -o blogmon .
./blogmon extract --help
```

Expected: Help message displays

**Step 4: Commit**

```bash
git add .
git commit -m "feat: add extract command with LLM integration"
```

---

## Task 5: Score Repository

**Files:**
- Create: `internal/score/repository.go`
- Create: `internal/score/repository_test.go`

**Step 1: Write tests**

```go
// internal/score/repository_test.go
package score

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/julienpequegnot/blogmon/internal/source"
)

func setupTestDB(t *testing.T) (*database.DB, int64) {
	tmpDir := t.TempDir()
	db, err := database.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	srcRepo := source.NewRepository(db)
	src, _ := srcRepo.Add("https://test.com", "Test", "")

	postRepo := post.NewRepository(db)
	p, _ := postRepo.Add(src.ID, "https://test.com/p1", "Test Post", "Author", time.Now(), "content")

	return db, p.ID
}

func TestUpsertScore(t *testing.T) {
	db, postID := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	err := repo.Upsert(postID, 75.0, 80.0, 60.0, 72.0)
	if err != nil {
		t.Fatalf("failed to upsert score: %v", err)
	}

	score, err := repo.Get(postID)
	if err != nil {
		t.Fatalf("failed to get score: %v", err)
	}

	if score.FinalScore != 72.0 {
		t.Errorf("expected final score 72.0, got %f", score.FinalScore)
	}
}

func TestGetUnscored(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	unscored, err := repo.GetUnscoredPostIDs(10)
	if err != nil {
		t.Fatalf("failed to get unscored: %v", err)
	}

	if len(unscored) != 1 {
		t.Errorf("expected 1 unscored post, got %d", len(unscored))
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/score/... -v
```

Expected: FAIL

**Step 3: Implement repository**

```go
// internal/score/repository.go
package score

import (
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type Score struct {
	PostID         int64
	CommunityScore float64
	RelevanceScore float64
	NoveltyScore   float64
	FinalScore     float64
	ScoredAt       time.Time
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Upsert(postID int64, community, relevance, novelty, final float64) error {
	_, err := r.db.Exec(`
		INSERT INTO scores (post_id, community_score, relevance_score, novelty_score, final_score, scored_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(post_id) DO UPDATE SET
			community_score = excluded.community_score,
			relevance_score = excluded.relevance_score,
			novelty_score = excluded.novelty_score,
			final_score = excluded.final_score,
			scored_at = CURRENT_TIMESTAMP
	`, postID, community, relevance, novelty, final)
	return err
}

func (r *Repository) Get(postID int64) (*Score, error) {
	var s Score
	err := r.db.QueryRow(`
		SELECT post_id, community_score, relevance_score, novelty_score, final_score, scored_at
		FROM scores WHERE post_id = ?
	`, postID).Scan(&s.PostID, &s.CommunityScore, &s.RelevanceScore, &s.NoveltyScore, &s.FinalScore, &s.ScoredAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) GetUnscoredPostIDs(limit int) ([]int64, error) {
	rows, err := r.db.Query(`
		SELECT p.id FROM posts p
		LEFT JOIN scores s ON p.id = s.post_id
		WHERE s.post_id IS NULL
		LIMIT ?
	`, limit)
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
```

**Step 4: Run tests**

Run:
```bash
go test ./internal/score/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add .
git commit -m "feat: add score repository"
```

---

## Task 6: HN Scorer

**Files:**
- Create: `internal/scorer/hn.go`
- Create: `internal/scorer/hn_test.go`

**Step 1: Write tests**

```go
// internal/scorer/hn_test.go
package scorer

import (
	"testing"
)

func TestCalculateCommunityScore(t *testing.T) {
	// Test score calculation formula
	score := CalculateCommunityScore(100, 50, 0)

	if score <= 0 {
		t.Error("expected positive score")
	}
	if score > 100 {
		t.Error("expected score <= 100")
	}
}

func TestCalculateCommunityScoreZero(t *testing.T) {
	score := CalculateCommunityScore(0, 0, 0)

	if score != 0 {
		t.Errorf("expected 0 score for no signals, got %f", score)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/scorer/... -v
```

Expected: FAIL

**Step 3: Implement HN scorer**

```go
// internal/scorer/hn.go
package scorer

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"
)

type HNSearchResponse struct {
	Hits []HNHit `json:"hits"`
}

type HNHit struct {
	ObjectID   string `json:"objectID"`
	Title      string `json:"title"`
	Points     int    `json:"points"`
	NumComments int   `json:"num_comments"`
	URL        string `json:"url"`
}

type HNScorer struct {
	client *http.Client
}

func NewHNScorer() *HNScorer {
	return &HNScorer{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *HNScorer) SearchByURL(postURL string) (*HNHit, error) {
	// HN Algolia API
	searchURL := fmt.Sprintf(
		"https://hn.algolia.com/api/v1/search?query=%s&restrictSearchableAttributes=url",
		url.QueryEscape(postURL),
	)

	resp, err := s.client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query HN API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HN API returned %d", resp.StatusCode)
	}

	var result HNSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode HN response: %w", err)
	}

	if len(result.Hits) == 0 {
		return nil, nil // Not found on HN
	}

	// Return the hit with the highest score
	var best *HNHit
	for i := range result.Hits {
		if best == nil || result.Hits[i].Points > best.Points {
			best = &result.Hits[i]
		}
	}

	return best, nil
}

func CalculateCommunityScore(hnPoints, hnComments, redditScore int) float64 {
	if hnPoints == 0 && hnComments == 0 && redditScore == 0 {
		return 0
	}

	// Weighted log scale to prevent viral posts from dominating
	// Formula: log(1 + hn_points*2 + hn_comments*3 + reddit_score) * 10
	rawScore := float64(hnPoints*2 + hnComments*3 + redditScore)
	score := math.Log(1+rawScore) * 10

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}
```

**Step 4: Run tests**

Run:
```bash
go test ./internal/scorer/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add .
git commit -m "feat: add HN community scorer"
```

---

## Task 7: Relevance Scorer

**Files:**
- Modify: `internal/scorer/relevance.go`

**Step 1: Create relevance scorer**

```go
// internal/scorer/relevance.go
package scorer

import (
	"strings"

	"github.com/julienpequegnot/blogmon/internal/config"
)

type RelevanceScorer struct {
	interests []config.Interest
}

func NewRelevanceScorer(interests []config.Interest) *RelevanceScorer {
	return &RelevanceScorer{interests: interests}
}

func (s *RelevanceScorer) Score(title, content string) float64 {
	if len(s.interests) == 0 {
		return 50.0 // Neutral score if no interests configured
	}

	text := strings.ToLower(title + " " + content)
	wordCount := len(strings.Fields(text))
	if wordCount == 0 {
		return 0
	}

	var totalScore float64
	var totalWeight float64

	for _, interest := range s.interests {
		topic := strings.ToLower(interest.Topic)
		keywords := append([]string{topic}, interest.Keywords...)

		matchCount := 0
		for _, keyword := range keywords {
			matchCount += strings.Count(text, strings.ToLower(keyword))
		}

		// Normalize by word count and apply weight
		if matchCount > 0 {
			density := float64(matchCount) / float64(wordCount) * 1000
			totalScore += density * interest.Weight
		}
		totalWeight += interest.Weight
	}

	if totalWeight == 0 {
		return 50.0
	}

	// Normalize to 0-100 scale
	score := (totalScore / totalWeight) * 10
	if score > 100 {
		score = 100
	}

	return score
}
```

**Step 2: Add test**

```go
// internal/scorer/relevance_test.go
package scorer

import (
	"testing"

	"github.com/julienpequegnot/blogmon/internal/config"
)

func TestRelevanceScorer(t *testing.T) {
	interests := []config.Interest{
		{Topic: "golang", Weight: 1.0, Keywords: []string{"go", "goroutine"}},
		{Topic: "rust", Weight: 0.8},
	}

	scorer := NewRelevanceScorer(interests)

	// Post about Go should score high
	score := scorer.Score("Learning Go", "This is about golang and goroutines and go programming")
	if score <= 0 {
		t.Errorf("expected positive score for Go content, got %f", score)
	}

	// Unrelated post should score low
	lowScore := scorer.Score("Cooking Tips", "How to make pasta and pizza")
	if lowScore >= score {
		t.Errorf("expected cooking score (%f) < Go score (%f)", lowScore, score)
	}
}

func TestRelevanceScorerNoInterests(t *testing.T) {
	scorer := NewRelevanceScorer([]config.Interest{})
	score := scorer.Score("Any Title", "Any content")

	if score != 50.0 {
		t.Errorf("expected neutral score 50, got %f", score)
	}
}
```

**Step 3: Run tests**

Run:
```bash
go test ./internal/scorer/... -v
```

Expected: PASS

**Step 4: Commit**

```bash
git add .
git commit -m "feat: add relevance scorer with keyword matching"
```

---

## Task 8: Novelty Scorer (TF-IDF)

**Files:**
- Create: `internal/scorer/novelty.go`
- Create: `internal/scorer/novelty_test.go`

**Step 1: Write tests**

```go
// internal/scorer/novelty_test.go
package scorer

import (
	"testing"
)

func TestNoveltyScorer(t *testing.T) {
	scorer := NewNoveltyScorer()

	// Add some existing content
	scorer.AddDocument(1, "golang programming concurrency")
	scorer.AddDocument(2, "rust memory safety ownership")

	// Similar content should have low novelty
	similarScore := scorer.Score("golang programming goroutines concurrency")

	// Different content should have high novelty
	differentScore := scorer.Score("machine learning neural networks tensorflow")

	if similarScore >= differentScore {
		t.Errorf("expected similar score (%f) < different score (%f)", similarScore, differentScore)
	}
}

func TestNoveltyEmptyCorpus(t *testing.T) {
	scorer := NewNoveltyScorer()
	score := scorer.Score("any content here")

	if score != 100.0 {
		t.Errorf("expected 100 novelty for empty corpus, got %f", score)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/scorer/... -v -run TestNovelty
```

Expected: FAIL

**Step 3: Implement novelty scorer**

```go
// internal/scorer/novelty.go
package scorer

import (
	"math"
	"strings"
)

type NoveltyScorer struct {
	documents    map[int64]map[string]float64 // postID -> term -> tf
	documentFreq map[string]int               // term -> doc count
	docCount     int
}

func NewNoveltyScorer() *NoveltyScorer {
	return &NoveltyScorer{
		documents:    make(map[int64]map[string]float64),
		documentFreq: make(map[string]int),
		docCount:     0,
	}
}

func (s *NoveltyScorer) AddDocument(id int64, content string) {
	terms := tokenize(content)
	if len(terms) == 0 {
		return
	}

	tf := make(map[string]float64)
	termCounts := make(map[string]int)

	for _, term := range terms {
		termCounts[term]++
	}

	// Calculate term frequency
	for term, count := range termCounts {
		tf[term] = float64(count) / float64(len(terms))
	}

	// Update document frequency (only count each term once per doc)
	for term := range termCounts {
		s.documentFreq[term]++
	}

	s.documents[id] = tf
	s.docCount++
}

func (s *NoveltyScorer) Score(content string) float64 {
	if s.docCount == 0 {
		return 100.0 // Everything is novel when corpus is empty
	}

	terms := tokenize(content)
	if len(terms) == 0 {
		return 100.0
	}

	// Calculate TF-IDF vector for new content
	newTF := make(map[string]float64)
	termCounts := make(map[string]int)

	for _, term := range terms {
		termCounts[term]++
	}

	for term, count := range termCounts {
		newTF[term] = float64(count) / float64(len(terms))
	}

	// Find max cosine similarity with existing documents
	maxSimilarity := 0.0
	for _, docTF := range s.documents {
		similarity := s.cosineSimilarity(newTF, docTF)
		if similarity > maxSimilarity {
			maxSimilarity = similarity
		}
	}

	// Novelty = 1 - max_similarity, scaled to 0-100
	novelty := (1 - maxSimilarity) * 100
	return novelty
}

func (s *NoveltyScorer) cosineSimilarity(tf1, tf2 map[string]float64) float64 {
	var dotProduct, norm1, norm2 float64

	for term, tfidf1 := range tf1 {
		idf := s.idf(term)
		v1 := tfidf1 * idf
		norm1 += v1 * v1

		if tfidf2, ok := tf2[term]; ok {
			v2 := tfidf2 * idf
			dotProduct += v1 * v2
		}
	}

	for term, tfidf2 := range tf2 {
		idf := s.idf(term)
		v2 := tfidf2 * idf
		norm2 += v2 * v2
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

func (s *NoveltyScorer) idf(term string) float64 {
	df := s.documentFreq[term]
	if df == 0 {
		return 0
	}
	return math.Log(float64(s.docCount+1) / float64(df+1))
}

func tokenize(content string) []string {
	content = strings.ToLower(content)

	// Simple tokenization: split on non-alphanumeric
	var tokens []string
	var current strings.Builder

	for _, r := range content {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			token := current.String()
			if len(token) > 2 { // Skip very short tokens
				tokens = append(tokens, token)
			}
			current.Reset()
		}
	}

	if current.Len() > 2 {
		tokens = append(tokens, current.String())
	}

	return tokens
}
```

**Step 4: Run tests**

Run:
```bash
go test ./internal/scorer/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add .
git commit -m "feat: add novelty scorer with TF-IDF"
```

---

## Task 9: Score Command

**Files:**
- Create: `cmd/score.go`

**Step 1: Create score command**

```go
// cmd/score.go
package cmd

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/julienpequegnot/blogmon/internal/score"
	"github.com/julienpequegnot/blogmon/internal/scorer"
	"github.com/spf13/cobra"
)

var scoreCmd = &cobra.Command{
	Use:   "score",
	Short: "Calculate scores for posts",
	Long:  `Calculates community, relevance, and novelty scores for unscored posts.`,
	RunE:  runScore,
}

var (
	scoreLimit     int
	scoreSkipHN    bool
)

func init() {
	rootCmd.AddCommand(scoreCmd)
	scoreCmd.Flags().IntVarP(&scoreLimit, "limit", "l", 50, "Maximum posts to score")
	scoreCmd.Flags().BoolVar(&scoreSkipHN, "skip-hn", false, "Skip HN API calls (for testing)")
}

func runScore(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	postRepo := post.NewRepository(db)
	scoreRepo := score.NewRepository(db)

	// Get unscored post IDs
	unscoredIDs, err := scoreRepo.GetUnscoredPostIDs(scoreLimit)
	if err != nil {
		return err
	}

	if len(unscoredIDs) == 0 {
		fmt.Println("No unscored posts found.")
		return nil
	}

	fmt.Printf("Scoring %d posts\n\n", len(unscoredIDs))

	// Initialize scorers
	hnScorer := scorer.NewHNScorer()
	relevanceScorer := scorer.NewRelevanceScorer(cfg.Interests)
	noveltyScorer := scorer.NewNoveltyScorer()

	// Build novelty corpus from existing posts
	allPosts, _ := postRepo.List(1000, 0)
	for _, p := range allPosts {
		content := p.ContentClean
		if content == "" {
			content = p.Title
		}
		noveltyScorer.AddDocument(p.ID, content)
	}

	weights := cfg.Scoring

	for _, postID := range unscoredIDs {
		p, err := postRepo.Get(postID)
		if err != nil {
			fmt.Printf("Error getting post %d: %v\n", postID, err)
			continue
		}

		fmt.Printf("Scoring: %s\n", p.Title)

		// Community score (HN)
		var communityScore float64
		if !scoreSkipHN {
			hit, err := hnScorer.SearchByURL(p.URL)
			if err != nil {
				fmt.Printf("  HN API error: %v\n", err)
			} else if hit != nil {
				communityScore = scorer.CalculateCommunityScore(hit.Points, hit.NumComments, 0)
				fmt.Printf("  HN: %d points, %d comments\n", hit.Points, hit.NumComments)
			}
		}

		// Relevance score
		content := p.ContentClean
		if content == "" {
			content = p.Title
		}
		relevanceScore := relevanceScorer.Score(p.Title, content)

		// Novelty score
		noveltyScore := noveltyScorer.Score(content)

		// Final score
		finalScore := (communityScore * weights.Community) +
			(relevanceScore * weights.Relevance) +
			(noveltyScore * weights.Novelty)

		// Save score
		if err := scoreRepo.Upsert(postID, communityScore, relevanceScore, noveltyScore, finalScore); err != nil {
			fmt.Printf("  Error saving score: %v\n", err)
			continue
		}

		fmt.Printf("  Community: %.1f, Relevance: %.1f, Novelty: %.1f -> Final: %.1f\n",
			communityScore, relevanceScore, noveltyScore, finalScore)
	}

	fmt.Println("\nScoring complete")
	return nil
}
```

**Step 2: Build and test**

Run:
```bash
go build -o blogmon .
./blogmon score --help
```

Expected: Help message displays

**Step 3: Commit**

```bash
git add .
git commit -m "feat: add score command with community/relevance/novelty scoring"
```

---

## Task 10: Update Show Command for Insights

**Files:**
- Modify: `cmd/show.go`

**Step 1: Update show command to display insights and scores**

Add to the runShow function after displaying URL:

```go
// Add imports
import (
	"github.com/julienpequegnot/blogmon/internal/insight"
	"github.com/julienpequegnot/blogmon/internal/reference"
	"github.com/julienpequegnot/blogmon/internal/score"
)

// Add after URL display in runShow:

	// Show score breakdown if available
	scoreRepo := score.NewRepository(db)
	if s, err := scoreRepo.Get(id); err == nil {
		fmt.Printf("\n%s\n", labelStyle.Render("SCORES:"))
		fmt.Printf("  Community: %.1f  Relevance: %.1f  Novelty: %.1f  → Final: %.1f\n",
			s.CommunityScore, s.RelevanceScore, s.NoveltyScore, s.FinalScore)
	}

	// Show insights
	insightRepo := insight.NewRepository(db)
	insights, _ := insightRepo.ListForPost(id)
	if len(insights) > 0 {
		fmt.Printf("\n%s\n", labelStyle.Render("KEY TAKEAWAYS:"))
		for _, ins := range insights {
			if ins.Type == "takeaway" {
				fmt.Printf("  • %s\n", ins.Content)
			}
		}
	}

	// Show references
	refRepo := reference.NewRepository(db)
	refs, _ := refRepo.ListForPost(id)
	if len(refs) > 0 {
		fmt.Printf("\n%s\n", labelStyle.Render("REFERENCES:"))
		for _, ref := range refs {
			if ref.Title != "" {
				fmt.Printf("  → %s (%s)\n", ref.Title, ref.URL)
			} else {
				fmt.Printf("  → %s\n", ref.URL)
			}
		}
	}
```

**Step 2: Build and test**

Run:
```bash
go build -o blogmon .
./blogmon show 1
```

Expected: Shows post with insights and scores if available

**Step 3: Commit**

```bash
git add .
git commit -m "feat: enhance show command with insights and score display"
```

---

## Task 11: Update README and Final Tests

**Files:**
- Modify: `README.md`

**Step 1: Update README**

Add to Development Status section:

```markdown
### Phase 2 (Intelligence) - Complete
- [x] LLM-powered insight extraction (Ollama)
- [x] Community scoring (HN API)
- [x] Relevance scoring (keyword matching)
- [x] Novelty scoring (TF-IDF)
```

Update Commands table:

```markdown
| `blogmon extract` | Extract insights from posts using LLM |
| `blogmon score` | Calculate community/relevance/novelty scores |
```

**Step 2: Run all tests**

Run:
```bash
go test ./... -v
```

Expected: All tests pass

**Step 3: Final commit**

```bash
git add .
git commit -m "docs: mark Phase 2 Intelligence as complete"
git push
```

---

## Summary

**Phase 2 delivers:**
- LLM client for Ollama integration
- Extract command with insight/reference extraction
- Score command with three scoring dimensions:
  - Community (HN API)
  - Relevance (keyword matching)
  - Novelty (TF-IDF)
- Enhanced show command with insights display

**New commands:**
- `blogmon extract` - Extract insights using LLM
- `blogmon score` - Calculate post scores

**Next: Phase 3 (Graph & Discovery)**
- Concept graph linking posts
- Blog discovery from references
- Trend detection
