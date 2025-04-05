package service

import (
	"context"
	"errors"
	"time"

	"github.com/chatapp/internal/models"
	"github.com/chatapp/internal/repository"
)

type MessageService interface {
	SendMessage(ctx context.Context, msg *models.Message) (*models.Message, error)
	GetConversation(ctx context.Context, user1ID, user2ID, limit int) ([]*models.Message, error)
	GetUserMessages(ctx context.Context, userID, limit int) ([]*models.Message, error)
	GetUndeliveredMessages(ctx context.Context, userID int) ([]*models.Message, error)
	MarkMessagesAsDelivered(ctx context.Context, receiverID int) error
	MarkMessagesAsRead(ctx context.Context, senderID, receiverID int) error
}

type messageService struct {
	repo repository.MessageRepository
}

func NewMessageService(repo repository.MessageRepository) MessageService {
	return &messageService{repo: repo}
}

func (s *messageService) SendMessage(ctx context.Context, msg *models.Message) (*models.Message, error) {
	if msg.SenderID <= 0 || msg.ReceiverID <= 0 {
		return nil, errors.New("invalid user ID")
	}

	msg.Timestamp = time.Now()
	msg.Status = "sent"

	id, err := s.repo.Create(ctx, msg)
	if err != nil {
		return nil, err
	}

	msg.ID = id
	return msg, nil
}

func (s *messageService) GetConversation(ctx context.Context, user1ID, user2ID, limit int) ([]*models.Message, error) {
	if user1ID <= 0 || user2ID <= 0 {
		return nil, errors.New("invalid user ID")
	}

	return s.repo.GetConversation(ctx, user1ID, user2ID, limit)
}

func (s *messageService) GetUserMessages(ctx context.Context, userID, limit int) ([]*models.Message, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user ID")
	}

	return s.repo.GetUserMessages(ctx, userID, limit)
}

func (s *messageService) GetUndeliveredMessages(ctx context.Context, userID int) ([]*models.Message, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user ID")
	}

	return s.repo.GetUndeliveredMessages(ctx, userID)
}

func (s *messageService) MarkMessagesAsDelivered(ctx context.Context, receiverID int) error {
	if receiverID <= 0 {
		return errors.New("invalid user ID")
	}

	return s.repo.MarkAsDelivered(ctx, receiverID)
}

func (s *messageService) MarkMessagesAsRead(ctx context.Context, senderID, receiverID int) error {
	if senderID <= 0 || receiverID <= 0 {
		return errors.New("invalid user ID")
	}

	return s.repo.MarkAsRead(ctx, senderID, receiverID)
}
