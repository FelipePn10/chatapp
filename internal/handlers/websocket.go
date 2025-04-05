package handlers

import (
	"net/http"

	"github.com/chatapp/internal/service"
	"github.com/chatapp/internal/websocket"
	"github.com/rs/zerolog"
)

func HandleWebSocket(hub *websocket.Hub, authService service.AuthService, logger *zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, ok := ctx.Value(userIDKey).(int)
		if !ok {
			logger.Error().Msg("User ID not found in context")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error().Err(err).Msg("WebSocket upgrade failed")
			return
		}

		client := websocket.NewClient(hub, conn, userID)
		hub.Register <- client

		go client.WritePump()
		go client.ReadPump()
	}
}
