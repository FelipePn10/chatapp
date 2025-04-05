package websocket

import (
	"context"
	"time"

	"github.com/chatapp/internal/models"
	"github.com/chatapp/internal/service"
	"github.com/rs/zerolog"
)

type Hub struct {
	Clients      map[int]*Client
	Register     chan *Client
	Unregister   chan *Client
	Broadcast    chan *models.Message
	ShutdownChan chan struct{}

	MessageService service.MessageService
	StatusService  service.StatusService
	Logger         *zerolog.Logger
}

func NewHub(
	messageService service.MessageService,
	statusService service.StatusService,
	logger *zerolog.Logger,
) *Hub {
	return &Hub{
		Clients:        make(map[int]*Client),
		Register:       make(chan *Client),
		Unregister:     make(chan *Client),
		Broadcast:      make(chan *models.Message),
		ShutdownChan:   make(chan struct{}),
		MessageService: messageService,
		StatusService:  statusService,
		Logger:         logger,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.handleRegister(client)
		case client := <-h.Unregister:
			h.handleUnregister(client)
		case message := <-h.Broadcast:
			h.handleBroadcast(message)
		case <-h.ShutdownChan:
			h.handleShutdown()
			return
		}
	}
}

func (h *Hub) handleRegister(client *Client) {
	h.Clients[client.UserID] = client
	h.StatusService.UpdateUserStatus(context.Background(), client.UserID, "online")
	h.notifyStatusChange(client.UserID, "online")
	h.sendPendingMessages(client)
}

func (h *Hub) handleUnregister(client *Client) {
	if _, ok := h.Clients[client.UserID]; ok {
		delete(h.Clients, client.UserID)
		close(client.Send)
		h.StatusService.UpdateUserStatus(context.Background(), client.UserID, "offline")
		h.notifyStatusChange(client.UserID, "offline")
	}
}

func (h *Hub) handleBroadcast(message *models.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, err := h.MessageService.SendMessage(ctx, message)
	if err != nil {
		h.Logger.Error().Err(err).Msg("Failed to save message")
		return
	}

	if client, ok := h.Clients[msg.ReceiverID]; ok {
		select {
		case client.Send <- msg:
			if err := h.MessageService.MarkMessagesAsDelivered(ctx, msg.ReceiverID); err != nil {
				h.Logger.Error().Err(err).Int("user_id", msg.ReceiverID).Msg("Failed to mark messages as delivered")
			}
		default:
			close(client.Send)
			delete(h.Clients, client.UserID)
		}
	}
}

func (h *Hub) notifyStatusChange(userID int, status string) {
	for _, client := range h.Clients {
		if client.UserID != userID {
			select {
			case client.Send <- &models.Message{
				Type:      "status_update",
				SenderID:  userID,
				Status:    status,
				Timestamp: time.Now(),
			}:
			default:
				close(client.Send)
				delete(h.Clients, client.UserID)
			}
		}
	}
}

func (h *Hub) sendPendingMessages(client *Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages, err := h.MessageService.GetUndeliveredMessages(ctx, client.UserID)
	if err != nil {
		h.Logger.Error().Err(err).Int("user_id", client.UserID).Msg("Failed to fetch pending messages")
		return
	}

	for _, msg := range messages {
		select {
		case client.Send <- msg:
			if err := h.MessageService.MarkMessagesAsDelivered(ctx, client.UserID); err != nil {
				h.Logger.Error().Err(err).Int("user_id", client.UserID).Msg("Failed to mark messages as delivered")
			}
		default:
			close(client.Send)
			delete(h.Clients, client.UserID)
			return
		}
	}
}

func (h *Hub) handleShutdown() {
	for _, client := range h.Clients {
		close(client.Send)
		shutdownMsg := &models.Message{
			Type:      "system",
			Content:   "Server is shutting down",
			Timestamp: time.Now(),
		}
		client.Conn.WriteJSON(shutdownMsg)
		client.Conn.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := h.StatusService.UpdateAllUsersOffline(ctx); err != nil {
		h.Logger.Error().Err(err).Msg("Failed to update all users to offline during shutdown")
	}
}

func (h *Hub) Shutdown() {
	close(h.ShutdownChan)
}
