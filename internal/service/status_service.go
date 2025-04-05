package service

import (
	"context"
	"errors"
	"time"

	"github.com/chatapp/internal/models"
	"github.com/chatapp/internal/repository"
)

type StatusService interface {
	UpdateUserStatus(ctx context.Context, userID int, status string) error
	GetUserStatus(ctx context.Context, userID int) (*models.UserStatus, error)
	GetAllStatuses(ctx context.Context) ([]*models.UserStatus, error)
	UpdateAllUsersOffline(ctx context.Context) error
}

type statusService struct {
	repo repository.StatusRepository
}

func NewStatusService(repo repository.StatusRepository) StatusService {
	return &statusService{repo: repo}
}

func (s *statusService) UpdateUserStatus(ctx context.Context, userID int, status string) error {
	if userID <= 0 {
		return errors.New("invalid user ID")
	}

	userStatus := &models.UserStatus{
		UserID:   userID,
		Status:   status,
		LastSeen: time.Now(),
	}
	return s.repo.Update(ctx, userStatus)
}

func (s *statusService) GetUserStatus(ctx context.Context, userID int) (*models.UserStatus, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user ID")
	}

	return s.repo.GetByUserID(ctx, userID)
}

func (s *statusService) GetAllStatuses(ctx context.Context) ([]*models.UserStatus, error) {
	return s.repo.GetAll(ctx)
}

func (s *statusService) UpdateAllUsersOffline(ctx context.Context) error {
	return s.repo.UpdateAllOffline(ctx)
}
