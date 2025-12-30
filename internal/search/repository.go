package search

import (
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type SearchResult struct {
	PostID      int64
	Title       string
	SourceName  string
	PublishedAt time.Time
	Snippet     string
	Rank        float64
	FinalScore  float64
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Search(query string, limit int) ([]SearchResult, error) {
	rows, err := r.db.Query(`
		SELECT
			p.id,
			p.title,
			s.name,
			p.published_at,
			snippet(posts_fts, -1, '<b>', '</b>', '...', 32) as snippet,
			bm25(posts_fts) as rank,
			COALESCE(sc.final_score, 0) as final_score
		FROM posts_fts
		JOIN posts p ON posts_fts.rowid = p.id
		JOIN sources s ON p.source_id = s.id
		LEFT JOIN scores sc ON p.id = sc.post_id
		WHERE posts_fts MATCH ?
		ORDER BY bm25(posts_fts)
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var sr SearchResult
		if err := rows.Scan(&sr.PostID, &sr.Title, &sr.SourceName, &sr.PublishedAt, &sr.Snippet, &sr.Rank, &sr.FinalScore); err != nil {
			return nil, err
		}
		results = append(results, sr)
	}
	return results, rows.Err()
}

func (r *Repository) SearchWithScore(query string, limit int) ([]SearchResult, error) {
	rows, err := r.db.Query(`
		SELECT
			p.id,
			p.title,
			s.name,
			p.published_at,
			snippet(posts_fts, -1, '<b>', '</b>', '...', 32) as snippet,
			bm25(posts_fts) as rank,
			COALESCE(sc.final_score, 0) as final_score
		FROM posts_fts
		JOIN posts p ON posts_fts.rowid = p.id
		JOIN sources s ON p.source_id = s.id
		LEFT JOIN scores sc ON p.id = sc.post_id
		WHERE posts_fts MATCH ?
		ORDER BY (COALESCE(sc.final_score, 0) * 0.3 - bm25(posts_fts) * 0.7) DESC
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var sr SearchResult
		if err := rows.Scan(&sr.PostID, &sr.Title, &sr.SourceName, &sr.PublishedAt, &sr.Snippet, &sr.Rank, &sr.FinalScore); err != nil {
			return nil, err
		}
		results = append(results, sr)
	}
	return results, rows.Err()
}

func (r *Repository) RebuildIndex() error {
	// Delete all existing FTS entries
	_, err := r.db.Exec("DELETE FROM posts_fts")
	if err != nil {
		return err
	}

	// Insert all posts into FTS index
	_, err = r.db.Exec(`
		INSERT INTO posts_fts(rowid, title, content)
		SELECT id, title, COALESCE(content_clean, content_raw, '') FROM posts
	`)
	return err
}
