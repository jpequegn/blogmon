package reference

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/database"
)

type Reference struct {
	ID      int64
	PostID  int64
	URL     string
	Title   string
	Context string
	IsBlog  bool
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Add(postID int64, url, title, context string, isBlog bool) (*Reference, error) {
	result, err := r.db.Exec(
		`INSERT INTO refs (post_id, url, title, context, is_blog) VALUES (?, ?, ?, ?, ?)`,
		postID, url, title, context, isBlog,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert reference: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Reference{
		ID:      id,
		PostID:  postID,
		URL:     url,
		Title:   title,
		Context: context,
		IsBlog:  isBlog,
	}, nil
}

func (r *Repository) ListForPost(postID int64) ([]Reference, error) {
	rows, err := r.db.Query(
		`SELECT id, post_id, url, title, context, is_blog FROM refs WHERE post_id = ?`,
		postID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []Reference
	for rows.Next() {
		var ref Reference
		if err := rows.Scan(&ref.ID, &ref.PostID, &ref.URL, &ref.Title, &ref.Context, &ref.IsBlog); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (r *Repository) DeleteForPost(postID int64) error {
	_, err := r.db.Exec(`DELETE FROM refs WHERE post_id = ?`, postID)
	return err
}

func (r *Repository) ListBlogReferences() ([]Reference, error) {
	rows, err := r.db.Query(`SELECT id, post_id, url, title, context, is_blog FROM refs WHERE is_blog = TRUE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []Reference
	for rows.Next() {
		var ref Reference
		if err := rows.Scan(&ref.ID, &ref.PostID, &ref.URL, &ref.Title, &ref.Context, &ref.IsBlog); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}
