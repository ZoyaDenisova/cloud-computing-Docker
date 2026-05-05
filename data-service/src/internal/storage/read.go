package storage

import (
	"context"
	"database/sql"
	"time"

	"data-service/internal/model"
)

func (st *Store) PostExists(ctx context.Context, postID int64) (bool, error) {
	var exists bool
	err := st.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM posts WHERE id = $1)", postID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (st *Store) SearchPosts(ctx context.Context, queryValue string) ([]model.SearchItem, error) {
	var (
		query string
		rows  *sql.Rows
		err   error
	)

	base := `
		SELECT p.id, p.author, p.title, p.body, p.created_at, COUNT(c.id) AS comments_count
		FROM posts p
		LEFT JOIN comments c ON c.post_id = p.id
	`

	if queryValue == "" {
		query = base + `
		GROUP BY p.id
		ORDER BY p.id
	`
		rows, err = st.db.QueryContext(ctx, query)
	} else {
		query = base + `
		WHERE p.title ILIKE '%' || $1 || '%'
		   OR p.body ILIKE '%' || $1 || '%'
		   OR p.author ILIKE '%' || $1 || '%'
		   OR EXISTS (
				SELECT 1
				FROM comments c2
				WHERE c2.post_id = p.id
				  AND (c2.text ILIKE '%' || $1 || '%' OR c2.author ILIKE '%' || $1 || '%')
			)
		GROUP BY p.id
		ORDER BY p.id
	`
		rows, err = st.db.QueryContext(ctx, query, queryValue)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]model.SearchItem, 0)
	for rows.Next() {
		var item model.SearchItem
		if err := rows.Scan(&item.PostID, &item.Author, &item.Title, &item.Body, &item.CreatedAt, &item.CommentsCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (st *Store) ReportTopPostsByComments(ctx context.Context) ([]model.TopPostReportItem, error) {
	query := `
		SELECT p.id, p.title, COUNT(c.id) AS comments_count
		FROM posts p
		LEFT JOIN comments c ON c.post_id = p.id
		GROUP BY p.id
		ORDER BY comments_count DESC, p.id
		LIMIT 10
	`
	rows, err := st.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]model.TopPostReportItem, 0)
	for rows.Next() {
		var item model.TopPostReportItem
		if err := rows.Scan(&item.PostID, &item.Title, &item.CommentsCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (st *Store) ReportPostsByDay(ctx context.Context) ([]model.DailyReportItem, error) {
	query := `
		SELECT DATE(created_at) AS day, COUNT(*) AS total
		FROM posts
		GROUP BY DATE(created_at)
		ORDER BY day
	`
	return st.runDailyReport(ctx, query)
}

func (st *Store) ReportCommentsByDay(ctx context.Context) ([]model.DailyReportItem, error) {
	query := `
		SELECT DATE(created_at) AS day, COUNT(*) AS total
		FROM comments
		GROUP BY DATE(created_at)
		ORDER BY day
	`
	return st.runDailyReport(ctx, query)
}

func (st *Store) runDailyReport(ctx context.Context, query string) ([]model.DailyReportItem, error) {
	rows, err := st.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]model.DailyReportItem, 0)
	for rows.Next() {
		var (
			day   time.Time
			count int64
		)
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		result = append(result, model.DailyReportItem{
			Day:   day.Format("2006-01-02"),
			Count: count,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
