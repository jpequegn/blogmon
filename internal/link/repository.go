package link

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type Link struct {
	ID           int64
	PostIDA      int64
	PostIDB      int64
	Relationship string
	Strength     float64
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Add(postIDA, postIDB int64, relationship string, strength float64) (*Link, error) {
	// Ensure consistent ordering (smaller ID first)
	if postIDA > postIDB {
		postIDA, postIDB = postIDB, postIDA
	}

	result, err := r.db.Exec(
		`INSERT INTO links (post_id_a, post_id_b, relationship, strength) VALUES (?, ?, ?, ?)`,
		postIDA, postIDB, relationship, strength,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert link: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Link{
		ID:           id,
		PostIDA:      postIDA,
		PostIDB:      postIDB,
		Relationship: relationship,
		Strength:     strength,
	}, nil
}

func (r *Repository) Upsert(postIDA, postIDB int64, relationship string, strength float64) error {
	// Ensure consistent ordering
	if postIDA > postIDB {
		postIDA, postIDB = postIDB, postIDA
	}

	_, err := r.db.Exec(`
		INSERT INTO links (post_id_a, post_id_b, relationship, strength)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(post_id_a, post_id_b, relationship) DO UPDATE SET
			strength = excluded.strength
	`, postIDA, postIDB, relationship, strength)
	return err
}

func (r *Repository) GetForPost(postID int64) ([]Link, error) {
	rows, err := r.db.Query(`
		SELECT id, post_id_a, post_id_b, relationship, strength
		FROM links
		WHERE post_id_a = ? OR post_id_b = ?
		ORDER BY strength DESC
	`, postID, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.PostIDA, &l.PostIDB, &l.Relationship, &l.Strength); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (r *Repository) GetRelatedPosts(postID int64, limit int) ([]int64, error) {
	rows, err := r.db.Query(`
		SELECT CASE WHEN post_id_a = ? THEN post_id_b ELSE post_id_a END as related_id
		FROM links
		WHERE post_id_a = ? OR post_id_b = ?
		ORDER BY strength DESC
		LIMIT ?
	`, postID, postID, postID, limit)
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

func (r *Repository) DeleteForPost(postID int64) error {
	_, err := r.db.Exec(`DELETE FROM links WHERE post_id_a = ? OR post_id_b = ?`, postID, postID)
	return err
}
