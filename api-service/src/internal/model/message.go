package model

import (
	"errors"
	"strconv"
	"strings"
)

type PostData struct {
	Author    string `json:"author"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at,omitempty"`
}

type CommentData struct {
	PostID    int64  `json:"post_id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at,omitempty"`
}

type DataMessage struct {
	Type    string       `json:"type"`
	Post    *PostData    `json:"post,omitempty"`
	Comment *CommentData `json:"comment,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func BuildKafkaMessageKey(msg DataMessage) []byte {
	switch msg.Type {
	case "comment":
		return []byte("post:" + strconv.FormatInt(msg.Comment.PostID, 10))
	case "post":
		return []byte("author:" + strings.ToLower(strings.TrimSpace(msg.Post.Author)))
	default:
		return []byte("unknown")
	}
}

func ValidateDataMessage(msg DataMessage) error {
	switch msg.Type {
	case "post":
		if msg.Post == nil {
			return errors.New("field post is required for type=post")
		}
		if strings.TrimSpace(msg.Post.Author) == "" || strings.TrimSpace(msg.Post.Title) == "" || strings.TrimSpace(msg.Post.Body) == "" {
			return errors.New("post.author, post.title and post.body are required")
		}
		if msg.Comment != nil {
			return errors.New("comment must be empty for type=post")
		}
	case "comment":
		if msg.Comment == nil {
			return errors.New("field comment is required for type=comment")
		}
		if msg.Comment.PostID <= 0 || strings.TrimSpace(msg.Comment.Author) == "" || strings.TrimSpace(msg.Comment.Text) == "" {
			return errors.New("comment.post_id, comment.author and comment.text are required")
		}
		if msg.Post != nil {
			return errors.New("post must be empty for type=comment")
		}
	default:
		return errors.New("type must be post or comment")
	}

	return nil
}
