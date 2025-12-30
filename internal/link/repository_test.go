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
