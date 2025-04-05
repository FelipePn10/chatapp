package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/chatapp/internal/service"
	"github.com/chatapp/internal/websocket"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

func SetupRoutes(
	router *mux.Router,
	hub *websocket.Hub,
	authService service.AuthService,
	messageService service.MessageService,
	statusService service.StatusService,
	logger *zerolog.Logger,
) {
	authMiddleware := AuthMiddleware(authService, logger)

	router.Handle("/ws", authMiddleware(HandleWebSocket(hub, authService, logger))).Methods("GET")

	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(authMiddleware)

	apiRouter.HandleFunc("/messages/history", HandleMessageHistory(messageService, logger)).Methods("GET")
	apiRouter.HandleFunc("/users/status", HandleUserStatus(statusService, logger)).Methods("GET")

	router.HandleFunc("/health", healthCheck).Methods("GET")
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
