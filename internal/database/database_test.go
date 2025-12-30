// internal/database/database_test.go
package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestInitSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Verify tables exist by querying them
	tables := []string{"sources", "posts", "insights", "refs", "scores", "links", "interests"}
	for _, table := range tables {
		_, err := db.conn.Query("SELECT 1 FROM " + table + " LIMIT 1")
		if err != nil {
			t.Errorf("table %s does not exist: %v", table, err)
		}
	}
}
