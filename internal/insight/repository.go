package insight

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type Insight struct {
	ID         int64
	PostID     int64
	Type       string // "takeaway", "code_example", "quote", "definition"
	Content    string
	Importance int
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Add(postID int64, insightType, content string, importance int) (*Insight, error) {
	result, err := r.db.Exec(
		`INSERT INTO insights (post_id, type, content, importance) VALUES (?, ?, ?, ?)`,
		postID, insightType, content, importance,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert insight: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Insight{
		ID:         id,
		PostID:     postID,
		Type:       insightType,
		Content:    content,
		Importance: importance,
	}, nil
}

func (r *Repository) ListForPost(postID int64) ([]Insight, error) {
	rows, err := r.db.Query(
		`SELECT id, post_id, type, content, importance FROM insights WHERE post_id = ? ORDER BY importance DESC`,
		postID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var insights []Insight
	for rows.Next() {
		var i Insight
		if err := rows.Scan(&i.ID, &i.PostID, &i.Type, &i.Content, &i.Importance); err != nil {
			return nil, err
		}
		insights = append(insights, i)
	}
	return insights, rows.Err()
}

func (r *Repository) DeleteForPost(postID int64) error {
	_, err := r.db.Exec(`DELETE FROM insights WHERE post_id = ?`, postID)
	return err
}
