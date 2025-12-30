// internal/post/repository_test.go
package post

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/source"
)

func setupTestDB(t *testing.T) (*database.DB, *source.Source) {
	tmpDir := t.TempDir()
	db, err := database.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	srcRepo := source.NewRepository(db)
	src, err := srcRepo.Add("https://test.com", "Test Blog", "https://test.com/feed")
	if err != nil {
		t.Fatalf("failed to add source: %v", err)
	}

	return db, src
}

func TestAddPost(t *testing.T) {
	db, src := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	post, err := repo.Add(src.ID, "https://test.com/post1", "Test Post", "Author", time.Now(), "<p>Content</p>")
	if err != nil {
		t.Fatalf("failed to add post: %v", err)
	}

	if post.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestListPosts(t *testing.T) {
	db, src := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	repo.Add(src.ID, "https://test.com/post1", "Post 1", "Author", time.Now(), "")
	repo.Add(src.ID, "https://test.com/post2", "Post 2", "Author", time.Now(), "")

	posts, err := repo.List(10, 0)
	if err != nil {
		t.Fatalf("failed to list posts: %v", err)
	}

	if len(posts) != 2 {
		t.Errorf("expected 2 posts, got %d", len(posts))
	}
}

func TestPostExists(t *testing.T) {
	db, src := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	repo.Add(src.ID, "https://test.com/post1", "Post 1", "Author", time.Now(), "")

	exists, err := repo.Exists("https://test.com/post1")
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}
	if !exists {
		t.Error("expected post to exist")
	}

	exists, err = repo.Exists("https://test.com/nonexistent")
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}
	if exists {
		t.Error("expected post to not exist")
	}
}
