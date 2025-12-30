package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
	path string
}

func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_fts5=true")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn, path: path}
	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Exec(query string, args ...any) (sql.Result, error) {
	return db.conn.Exec(query, args...)
}

func (db *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return db.conn.Query(query, args...)
}

func (db *DB) QueryRow(query string, args ...any) *sql.Row {
	return db.conn.QueryRow(query, args...)
}

func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sources (
		id INTEGER PRIMARY KEY,
		url TEXT NOT NULL UNIQUE,
		name TEXT,
		feed_url TEXT,
		discovered_from INTEGER REFERENCES posts(id),
		last_fetched DATETIME,
		active BOOLEAN DEFAULT TRUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY,
		source_id INTEGER NOT NULL REFERENCES sources(id),
		url TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL,
		author TEXT,
		published_at DATETIME,
		fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		content_raw TEXT,
		content_clean TEXT,
		word_count INTEGER
	);

	CREATE TABLE IF NOT EXISTS insights (
		id INTEGER PRIMARY KEY,
		post_id INTEGER NOT NULL REFERENCES posts(id),
		type TEXT NOT NULL,
		content TEXT NOT NULL,
		importance INTEGER
	);

	CREATE TABLE IF NOT EXISTS refs (
		id INTEGER PRIMARY KEY,
		post_id INTEGER NOT NULL REFERENCES posts(id),
		url TEXT NOT NULL,
		title TEXT,
		context TEXT,
		is_blog BOOLEAN DEFAULT FALSE
	);

	CREATE TABLE IF NOT EXISTS scores (
		post_id INTEGER PRIMARY KEY REFERENCES posts(id),
		community_score REAL,
		relevance_score REAL,
		novelty_score REAL,
		final_score REAL,
		scored_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS links (
		id INTEGER PRIMARY KEY,
		post_id_a INTEGER NOT NULL REFERENCES posts(id),
		post_id_b INTEGER NOT NULL REFERENCES posts(id),
		relationship TEXT NOT NULL,
		strength REAL,
		UNIQUE(post_id_a, post_id_b, relationship)
	);

	CREATE TABLE IF NOT EXISTS interests (
		id INTEGER PRIMARY KEY,
		topic TEXT NOT NULL UNIQUE,
		weight REAL DEFAULT 1.0,
		keywords TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_posts_source ON posts(source_id);
	CREATE INDEX IF NOT EXISTS idx_posts_published ON posts(published_at);
	CREATE INDEX IF NOT EXISTS idx_scores_final ON scores(final_score DESC);

	CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
		title,
		content
	);

	CREATE TRIGGER IF NOT EXISTS posts_ai AFTER INSERT ON posts BEGIN
		INSERT INTO posts_fts(rowid, title, content) VALUES (new.id, new.title, COALESCE(new.content_clean, new.content_raw, ''));
	END;

	CREATE TRIGGER IF NOT EXISTS posts_ad AFTER DELETE ON posts BEGIN
		INSERT INTO posts_fts(posts_fts, rowid, title, content) VALUES('delete', old.id, old.title, COALESCE(old.content_clean, old.content_raw, ''));
	END;

	CREATE TRIGGER IF NOT EXISTS posts_au AFTER UPDATE ON posts BEGIN
		INSERT INTO posts_fts(posts_fts, rowid, title, content) VALUES('delete', old.id, old.title, COALESCE(old.content_clean, old.content_raw, ''));
		INSERT INTO posts_fts(rowid, title, content) VALUES (new.id, new.title, COALESCE(new.content_clean, new.content_raw, ''));
	END;
	`

	_, err := db.conn.Exec(schema)
	return err
}
