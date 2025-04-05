package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/chatapp/internal/models"
	"github.com/chatapp/internal/service"
	"github.com/rs/zerolog"
)

func HandleMessageHistory(messageService service.MessageService, logger *zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := ctx.Value("userID").(int)

		otherUserIDStr := r.URL.Query().Get("user_id")
		limitStr := r.URL.Query().Get("limit")
		if limitStr == "" {
			limitStr = "50"
		}

		otherUserID, err := strconv.Atoi(otherUserIDStr)
		if err != nil {
			logger.Warn().Err(err).Msg("Invalid other_user_id parameter")
			http.Error(w, "Invalid user_id parameter", http.StatusBadRequest)
			return
		}

		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 1000 {
			logger.Warn().Err(err).Msg("Invalid limit parameter")
			http.Error(w, "Invalid limit parameter (1-1000)", http.StatusBadRequest)
			return
		}

		var messages []*models.Message
		if otherUserID > 0 {
			messages, err = messageService.GetConversation(ctx, userID, otherUserID, limit)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to get conversation history")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if err := messageService.MarkMessagesAsRead(ctx, otherUserID, userID); err != nil {
				logger.Error().Err(err).Msg("Failed to mark messages as read")
			}
		} else {
			messages, err = messageService.GetUserMessages(ctx, userID, limit)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to get user messages")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(messages); err != nil {
			logger.Error().Err(err).Msg("Failed to encode messages response")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}
