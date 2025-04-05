package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/chatapp/internal/models"
	"github.com/chatapp/internal/service"
	"github.com/rs/zerolog"
)

func HandleUserStatus(statusService service.StatusService, logger *zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userIDStr := r.URL.Query().Get("user_id")
		var statuses []*models.UserStatus
		var err error

		if userIDStr != "" {
			userID, err := strconv.Atoi(userIDStr)
			if err != nil {
				logger.Warn().Err(err).Msg("Invalid user_id parameter")
				http.Error(w, "Invalid user_id parameter", http.StatusBadRequest)
				return
			}

			status, err := statusService.GetUserStatus(ctx, userID)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to get user status")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			statuses = []*models.UserStatus{status}
		} else {
			statuses, err = statusService.GetAllStatuses(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to get all user statuses")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(statuses); err != nil {
			logger.Error().Err(err).Msg("Failed to encode status response")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}
