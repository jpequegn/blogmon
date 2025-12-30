package source

import (
	"path/filepath"
	"testing"

	"github.com/julienpequegnot/blogmon/internal/database"
)

func setupTestDB(t *testing.T) *database.DB {
	tmpDir := t.TempDir()
	db, err := database.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	return db
}

func TestAddSource(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	src, err := repo.Add("https://jvns.ca", "Julia Evans", "https://jvns.ca/atom.xml")
	if err != nil {
		t.Fatalf("failed to add source: %v", err)
	}

	if src.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if src.URL != "https://jvns.ca" {
		t.Errorf("expected URL https://jvns.ca, got %s", src.URL)
	}
}

func TestAddDuplicateSource(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	_, err := repo.Add("https://jvns.ca", "Julia Evans", "")
	if err != nil {
		t.Fatalf("failed to add source: %v", err)
	}

	_, err = repo.Add("https://jvns.ca", "Julia Evans", "")
	if err == nil {
		t.Error("expected error for duplicate source")
	}
}

func TestListSources(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)

	repo.Add("https://jvns.ca", "Julia Evans", "")
	repo.Add("https://fasterthanli.me", "fasterthanlime", "")

	sources, err := repo.List()
	if err != nil {
		t.Fatalf("failed to list sources: %v", err)
	}

	if len(sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(sources))
	}
}
