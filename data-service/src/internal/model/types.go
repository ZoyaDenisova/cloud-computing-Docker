package model

import "time"

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

type ExistsResponse struct {
	Exists bool `json:"exists"`
}

type SearchItem struct {
	PostID        int64     `json:"post_id"`
	Author        string    `json:"author"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	CreatedAt     time.Time `json:"created_at"`
	CommentsCount int64     `json:"comments_count"`
}

type TopPostReportItem struct {
	PostID        int64  `json:"post_id"`
	Title         string `json:"title"`
	CommentsCount int64  `json:"comments_count"`
}

type DailyReportItem struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}
