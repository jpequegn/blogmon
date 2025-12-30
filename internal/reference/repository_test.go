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
