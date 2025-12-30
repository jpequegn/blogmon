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
	p1, _ := postRepo.Add(src.ID, "https://test.com/p1", "Learning Golang Concurrency", "Author", time.Now(), "This post covers goroutines and channels in Go programming")
	p2, _ := postRepo.Add(src.ID, "https://test.com/p2", "Rust Memory Safety", "Author", time.Now(), "Understanding ownership and borrowing in Rust")
	p3, _ := postRepo.Add(src.ID, "https://test.com/p3", "Python Data Science", "Author", time.Now(), "Using pandas and numpy for data analysis")

	// Update posts with clean content to trigger FTS index update
	postRepo.UpdateContentClean(p1.ID, "This post covers goroutines and channels in Go programming", 10)
	postRepo.UpdateContentClean(p2.ID, "Understanding ownership and borrowing in Rust", 7)
	postRepo.UpdateContentClean(p3.ID, "Using pandas and numpy for data analysis", 8)

	// Manually rebuild FTS index since external content mode may have issues
	searchRepo := NewRepository(db)
	if err := searchRepo.RebuildIndex(); err != nil {
		t.Fatalf("failed to rebuild index: %v", err)
	}

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
