package models

import "time"

type Message struct {
	ID         int64     `json:"id,omitempty"`
	SenderID   int       `json:"sender_id"`
	ReceiverID int       `json:"receiver_id"`
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
	Status     string    `json:"status"`
	Type       string    `json:"type,omitempty"` // Para mensagens de sistema
}

type MessageRequest struct {
	ReceiverID int    `json:"receiver_id" validate:"required,gt=0"`
	Content    string `json:"content" validate:"required,min=1,max=1000"`
}

type MessageResponse struct {
	ID         int64     `json:"id"`
	SenderID   int       `json:"sender_id"`
	ReceiverID int       `json:"receiver_id"`
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
	Status     string    `json:"status"`
}

type MessageHistoryRequest struct {
	OtherUserID int `json:"other_user_id" validate:"required,gt=0"`
	Limit       int `json:"limit" validate:"gte=1,lte=1000"`
}
