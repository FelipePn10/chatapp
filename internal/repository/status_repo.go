package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/chatapp/internal/models"
	"github.com/rs/zerolog"
)

type StatusRepository interface {
	Update(ctx context.Context, status *models.UserStatus) error
	GetByUserID(ctx context.Context, userID int) (*models.UserStatus, error)
	GetAll(ctx context.Context) ([]*models.UserStatus, error)
	UpdateAllOffline(ctx context.Context) error
}

type statusRepository struct {
	db     *sql.DB
	logger *zerolog.Logger
}

func NewStatusRepository(db *sql.DB, logger *zerolog.Logger) StatusRepository {
	return &statusRepository{db: db, logger: logger}
}

func (r *statusRepository) Update(ctx context.Context, status *models.UserStatus) error {
	query := `
		INSERT INTO user_status (user_id, status, last_seen)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
		status = VALUES(status),
		last_seen = VALUES(last_seen)
	`
	_, err := r.db.ExecContext(ctx, query,
		status.UserID,
		status.Status,
		status.LastSeen,
	)
	if err != nil {
		r.logger.Error().Err(err).Int("user_id", status.UserID).Msg("Failed to update user status")
		return err
	}
	return nil
}

func (r *statusRepository) GetByUserID(ctx context.Context, userID int) (*models.UserStatus, error) {
	query := `SELECT user_id, status, last_seen FROM user_status WHERE user_id = ?`
	row := r.db.QueryRowContext(ctx, query, userID)

	var status models.UserStatus
	var lastSeen time.Time
	err := row.Scan(&status.UserID, &status.Status, &lastSeen)
	if err != nil {
		if err == sql.ErrNoRows {
			return &models.UserStatus{
				UserID:   userID,
				Status:   "offline",
				LastSeen: time.Now(),
			}, nil
		}
		r.logger.Error().Err(err).Int("user_id", userID).Msg("Failed to get user status")
		return nil, err
	}

	status.LastSeen = lastSeen
	return &status, nil
}

func (r *statusRepository) GetAll(ctx context.Context) ([]*models.UserStatus, error) {
	query := `SELECT user_id, status, last_seen FROM user_status`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to get all user statuses")
		return nil, err
	}
	defer rows.Close()

	var statuses []*models.UserStatus
	for rows.Next() {
		var status models.UserStatus
		var lastSeen time.Time
		err := rows.Scan(&status.UserID, &status.Status, &lastSeen)
		if err != nil {
			r.logger.Error().Err(err).Msg("Failed to scan user status row")
			continue
		}
		status.LastSeen = lastSeen
		statuses = append(statuses, &status)
	}

	return statuses, nil
}

func (r *statusRepository) UpdateAllOffline(ctx context.Context) error {
	query := `UPDATE user_status SET status = 'offline', last_seen = ? WHERE status = 'online'`
	_, err := r.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to update all users to offline")
		return err
	}
	return nil
}
