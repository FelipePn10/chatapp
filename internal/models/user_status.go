package models

import "time"

type UserStatus struct {
	UserID   int       `json:"user_id"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"last_seen,omitempty"`
}

type UserStatusResponse struct {
	UserID   int       `json:"user_id"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"last_seen,omitempty"`
}

type StatusUpdate struct {
	UserID int    `json:"user_id"`
	Status string `json:"status"`
}
