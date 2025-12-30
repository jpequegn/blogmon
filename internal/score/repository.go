package score

import (
	"time"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type Score struct {
	PostID         int64
	CommunityScore float64
	RelevanceScore float64
	NoveltyScore   float64
	FinalScore     float64
	ScoredAt       time.Time
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Upsert(postID int64, community, relevance, novelty, final float64) error {
	_, err := r.db.Exec(`
		INSERT INTO scores (post_id, community_score, relevance_score, novelty_score, final_score, scored_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(post_id) DO UPDATE SET
			community_score = excluded.community_score,
			relevance_score = excluded.relevance_score,
			novelty_score = excluded.novelty_score,
			final_score = excluded.final_score,
			scored_at = CURRENT_TIMESTAMP
	`, postID, community, relevance, novelty, final)
	return err
}

func (r *Repository) Get(postID int64) (*Score, error) {
	var s Score
	err := r.db.QueryRow(`
		SELECT post_id, community_score, relevance_score, novelty_score, final_score, scored_at
		FROM scores WHERE post_id = ?
	`, postID).Scan(&s.PostID, &s.CommunityScore, &s.RelevanceScore, &s.NoveltyScore, &s.FinalScore, &s.ScoredAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) GetUnscoredPostIDs(limit int) ([]int64, error) {
	rows, err := r.db.Query(`
		SELECT p.id FROM posts p
		LEFT JOIN scores s ON p.id = s.post_id
		WHERE s.post_id IS NULL
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
