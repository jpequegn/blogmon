// internal/post/repository.go
package post

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type Post struct {
	ID           int64
	SourceID     int64
	SourceName   string
	URL          string
	Title        string
	Author       string
	PublishedAt  *time.Time
	FetchedAt    time.Time
	ContentRaw   string
	ContentClean string
	WordCount    int
	FinalScore   *float64
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Add(sourceID int64, url, title, author string, publishedAt time.Time, contentRaw string) (*Post, error) {
	result, err := r.db.Exec(
		`INSERT INTO posts (source_id, url, title, author, published_at, content_raw) VALUES (?, ?, ?, ?, ?, ?)`,
		sourceID, url, title, author, publishedAt, contentRaw,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert post: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Post{
		ID:          id,
		SourceID:    sourceID,
		URL:         url,
		Title:       title,
		Author:      author,
		PublishedAt: &publishedAt,
		ContentRaw:  contentRaw,
	}, nil
}

func (r *Repository) Exists(url string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM posts WHERE url = ?`, url).Scan(&count)
	return count > 0, err
}

func (r *Repository) List(limit, offset int) ([]Post, error) {
	rows, err := r.db.Query(`
		SELECT p.id, p.source_id, s.name, p.url, p.title, p.author, p.published_at, p.fetched_at,
		       COALESCE(sc.final_score, 0) as final_score
		FROM posts p
		JOIN sources s ON p.source_id = s.id
		LEFT JOIN scores sc ON p.id = sc.post_id
		ORDER BY p.published_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		var score sql.NullFloat64
		if err := rows.Scan(&p.ID, &p.SourceID, &p.SourceName, &p.URL, &p.Title, &p.Author, &p.PublishedAt, &p.FetchedAt, &score); err != nil {
			return nil, err
		}
		if score.Valid {
			p.FinalScore = &score.Float64
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (r *Repository) ListSorted(limit, offset int, sortBy string) ([]Post, error) {
	orderClause := "ORDER BY p.published_at DESC"
	switch sortBy {
	case "score":
		orderClause = "ORDER BY COALESCE(sc.final_score, 0) DESC, p.published_at DESC"
	case "source":
		orderClause = "ORDER BY s.name ASC, p.published_at DESC"
	case "date":
		orderClause = "ORDER BY p.published_at DESC"
	}

	query := fmt.Sprintf(`
		SELECT p.id, p.source_id, s.name, p.url, p.title, p.author, p.published_at, p.fetched_at,
		       COALESCE(sc.final_score, 0) as final_score
		FROM posts p
		JOIN sources s ON p.source_id = s.id
		LEFT JOIN scores sc ON p.id = sc.post_id
		%s
		LIMIT ? OFFSET ?
	`, orderClause)

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		var score sql.NullFloat64
		if err := rows.Scan(&p.ID, &p.SourceID, &p.SourceName, &p.URL, &p.Title, &p.Author, &p.PublishedAt, &p.FetchedAt, &score); err != nil {
			return nil, err
		}
		if score.Valid {
			p.FinalScore = &score.Float64
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (r *Repository) Get(id int64) (*Post, error) {
	var p Post
	var score sql.NullFloat64
	err := r.db.QueryRow(`
		SELECT p.id, p.source_id, s.name, p.url, p.title, p.author, p.published_at, p.fetched_at,
		       p.content_raw, p.content_clean, COALESCE(p.word_count, 0),
		       COALESCE(sc.final_score, 0)
		FROM posts p
		JOIN sources s ON p.source_id = s.id
		LEFT JOIN scores sc ON p.id = sc.post_id
		WHERE p.id = ?
	`, id).Scan(&p.ID, &p.SourceID, &p.SourceName, &p.URL, &p.Title, &p.Author, &p.PublishedAt, &p.FetchedAt,
		&p.ContentRaw, &p.ContentClean, &p.WordCount, &score)
	if err != nil {
		return nil, err
	}
	if score.Valid {
		p.FinalScore = &score.Float64
	}
	return &p, nil
}

func (r *Repository) GetUnextracted(limit int) ([]Post, error) {
	rows, err := r.db.Query(`
		SELECT p.id, p.source_id, s.name, p.url, p.title, p.author, p.published_at, p.fetched_at,
		       p.content_raw, COALESCE(p.content_clean, '') as content_clean, COALESCE(p.word_count, 0)
		FROM posts p
		JOIN sources s ON p.source_id = s.id
		WHERE p.content_clean IS NULL OR p.content_clean = ''
		ORDER BY p.published_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.SourceID, &p.SourceName, &p.URL, &p.Title, &p.Author,
			&p.PublishedAt, &p.FetchedAt, &p.ContentRaw, &p.ContentClean, &p.WordCount); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (r *Repository) UpdateContentClean(id int64, contentClean string, wordCount int) error {
	_, err := r.db.Exec(
		`UPDATE posts SET content_clean = ?, word_count = ? WHERE id = ?`,
		contentClean, wordCount, id,
	)
	return err
}
