package source

import (
	"fmt"
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type Source struct {
	ID             int64
	URL            string
	Name           string
	FeedURL        string
	DiscoveredFrom *int64
	LastFetched    *time.Time
	Active         bool
	CreatedAt      time.Time
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Add(url, name, feedURL string) (*Source, error) {
	result, err := r.db.Exec(
		`INSERT INTO sources (url, name, feed_url, active) VALUES (?, ?, ?, TRUE)`,
		url, name, feedURL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert source: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Source{
		ID:      id,
		URL:     url,
		Name:    name,
		FeedURL: feedURL,
		Active:  true,
	}, nil
}

func (r *Repository) List() ([]Source, error) {
	rows, err := r.db.Query(`SELECT id, url, name, feed_url, last_fetched, active, created_at FROM sources WHERE active = TRUE ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.ID, &s.URL, &s.Name, &s.FeedURL, &s.LastFetched, &s.Active, &s.CreatedAt); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

func (r *Repository) UpdateLastFetched(id int64) error {
	_, err := r.db.Exec(`UPDATE sources SET last_fetched = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}
