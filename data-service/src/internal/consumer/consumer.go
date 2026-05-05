package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"data-service/internal/model"
)

type MessageReader interface {
	Read(ctx context.Context) ([]byte, error)
}

type DataStore interface {
	InsertPost(ctx context.Context, post model.PostData, createdAt *time.Time) error
	InsertComment(ctx context.Context, comment model.CommentData, createdAt *time.Time) error
}

func Run(ctx context.Context, reader MessageReader, st DataStore) {
	for {
		message, err := reader.Read(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Printf("kafka read failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if err := processMessage(ctx, st, message); err != nil {
			log.Printf("kafka message rejected: %v", err)
		}
	}
}

func processMessage(ctx context.Context, st DataStore, raw []byte) error {
	var msg model.DataMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}

	switch msg.Type {
	case "post":
		if msg.Post == nil {
			return errors.New("post payload is required")
		}
		if strings.TrimSpace(msg.Post.Author) == "" || strings.TrimSpace(msg.Post.Title) == "" || strings.TrimSpace(msg.Post.Body) == "" {
			return errors.New("post.author, post.title and post.body are required")
		}

		createdAt, err := parseCreatedAt(msg.Post.CreatedAt)
		if err != nil {
			return err
		}

		return st.InsertPost(ctx, *msg.Post, createdAt)
	case "comment":
		if msg.Comment == nil {
			return errors.New("comment payload is required")
		}
		if msg.Comment.PostID <= 0 || strings.TrimSpace(msg.Comment.Author) == "" || strings.TrimSpace(msg.Comment.Text) == "" {
			return errors.New("comment.post_id, comment.author and comment.text are required")
		}

		createdAt, err := parseCreatedAt(msg.Comment.CreatedAt)
		if err != nil {
			return err
		}

		return st.InsertComment(ctx, *msg.Comment, createdAt)
	default:
		return errors.New("unknown message type")
	}
}

func parseCreatedAt(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, errors.New("created_at must be RFC3339")
	}

	return &parsed, nil
}
