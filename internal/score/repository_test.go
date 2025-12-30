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
