package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"data-service/internal/model"
	"github.com/lib/pq"
)

func (st *Store) InsertPost(ctx context.Context, post model.PostData, createdAt *time.Time) error {
	query := `
		INSERT INTO posts (author, title, body, created_at)
		VALUES ($1, $2, $3, COALESCE($4, NOW()))
	`
	_, err := st.db.ExecContext(ctx, query, post.Author, post.Title, post.Body, createdAt)
	return err
}

func (st *Store) InsertComment(ctx context.Context, comment model.CommentData, createdAt *time.Time) error {
	query := `
		INSERT INTO comments (post_id, author, text, created_at)
		VALUES ($1, $2, $3, COALESCE($4, NOW()))
	`
	_, err := st.db.ExecContext(ctx, query, comment.PostID, comment.Author, comment.Text, createdAt)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && string(pgErr.Code) == "23503" {
			return fmt.Errorf("post with id=%d does not exist", comment.PostID)
		}
	}
	return err
}
