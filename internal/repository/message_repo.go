package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/chatapp/internal/models"
	"github.com/rs/zerolog"
)

type MessageRepository interface {
	Create(ctx context.Context, message *models.Message) (int64, error)
	GetByID(ctx context.Context, id int64) (*models.Message, error)
	GetConversation(ctx context.Context, user1ID, user2ID int, limit int) ([]*models.Message, error)
	GetUserMessages(ctx context.Context, userID int, limit int) ([]*models.Message, error)
	GetUndeliveredMessages(ctx context.Context, userID int) ([]*models.Message, error)
	UpdateStatus(ctx context.Context, id int64, status string) error
	MarkAsDelivered(ctx context.Context, receiverID int) error
	MarkAsRead(ctx context.Context, senderID, receiverID int) error
}

type messageRepository struct {
	db     *sql.DB
	logger *zerolog.Logger
}

func NewMessageRepository(db *sql.DB, logger *zerolog.Logger) MessageRepository {
	return &messageRepository{db: db, logger: logger}
}

func (r *messageRepository) Create(ctx context.Context, message *models.Message) (int64, error) {
	query := `
		INSERT INTO messages (sender_id, receiver_id, content, timestamp, status)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query,
		message.SenderID,
		message.ReceiverID,
		message.Content,
		message.Timestamp,
		message.Status,
	)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to create message")
		return 0, err
	}
	return result.LastInsertId()
}

func (r *messageRepository) GetByID(ctx context.Context, id int64) (*models.Message, error) {
	query := `
		SELECT id, sender_id, receiver_id, content, timestamp, status
		FROM messages
		WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, query, id)

	var msg models.Message
	var timestamp time.Time
	err := row.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Content, &timestamp, &msg.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error().Err(err).Int64("message_id", id).Msg("Failed to get message by ID")
		return nil, err
	}

	msg.Timestamp = timestamp
	return &msg, nil
}

func (r *messageRepository) GetConversation(ctx context.Context, user1ID, user2ID int, limit int) ([]*models.Message, error) {
	query := `
		SELECT id, sender_id, receiver_id, content, timestamp, status
		FROM messages
		WHERE (sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, user1ID, user2ID, user2ID, user1ID, limit)
	if err != nil {
		r.logger.Error().Err(err).
			Int("user1_id", user1ID).
			Int("user2_id", user2ID).
			Msg("Failed to get conversation")
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var msg models.Message
		var timestamp time.Time
		err := rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Content, &timestamp, &msg.Status)
		if err != nil {
			r.logger.Error().Err(err).Msg("Failed to scan message row")
			continue
		}
		msg.Timestamp = timestamp
		messages = append(messages, &msg)
	}

	return messages, nil
}

func (r *messageRepository) GetUserMessages(ctx context.Context, userID int, limit int) ([]*models.Message, error) {
	query := `
		SELECT id, sender_id, receiver_id, content, timestamp, status
		FROM messages
		WHERE sender_id = ? OR receiver_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, userID, userID, limit)
	if err != nil {
		r.logger.Error().Err(err).Int("user_id", userID).Msg("Failed to get user messages")
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var msg models.Message
		var timestamp time.Time
		err := rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Content, &timestamp, &msg.Status)
		if err != nil {
			r.logger.Error().Err(err).Msg("Failed to scan message row")
			continue
		}
		msg.Timestamp = timestamp
		messages = append(messages, &msg)
	}

	return messages, nil
}

func (r *messageRepository) GetUndeliveredMessages(ctx context.Context, userID int) ([]*models.Message, error) {
	query := `
		SELECT id, sender_id, receiver_id, content, timestamp, status
		FROM messages
		WHERE receiver_id = ? AND status = 'sent'
		ORDER BY timestamp ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		r.logger.Error().Err(err).Int("user_id", userID).Msg("Failed to get undelivered messages")
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var msg models.Message
		var timestamp time.Time
		err := rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Content, &timestamp, &msg.Status)
		if err != nil {
			r.logger.Error().Err(err).Msg("Failed to scan message row")
			continue
		}
		msg.Timestamp = timestamp
		messages = append(messages, &msg)
	}

	return messages, nil
}

func (r *messageRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := `UPDATE messages SET status = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		r.logger.Error().Err(err).Int64("message_id", id).Str("status", status).Msg("Failed to update message status")
		return err
	}
	return nil
}

func (r *messageRepository) MarkAsDelivered(ctx context.Context, receiverID int) error {
	query := `UPDATE messages SET status = 'delivered' WHERE receiver_id = ? AND status = 'sent'`
	_, err := r.db.ExecContext(ctx, query, receiverID)
	if err != nil {
		r.logger.Error().Err(err).Int("receiver_id", receiverID).Msg("Failed to mark messages as delivered")
		return err
	}
	return nil
}

func (r *messageRepository) MarkAsRead(ctx context.Context, senderID, receiverID int) error {
	query := `
		UPDATE messages 
		SET status = 'read' 
		WHERE sender_id = ? AND receiver_id = ? AND status = 'delivered'
	`
	_, err := r.db.ExecContext(ctx, query, senderID, receiverID)
	if err != nil {
		r.logger.Error().Err(err).
			Int("sender_id", senderID).
			Int("receiver_id", receiverID).
			Msg("Failed to mark messages as read")
		return err
	}
	return nil
}
