package insight

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/source"
	"github.com/julienpequegnot/blogmon/internal/post"
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
