package handlers

import (
	"context"
	"net/http"

	"github.com/chatapp/internal/service"
	"github.com/rs/zerolog"
)

func AuthMiddleware(authService service.AuthService, logger *zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractToken(r)
			if tokenString == "" {
				logger.Warn().Msg("Missing authorization token")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			userID, err := authService.ValidateToken(tokenString)
			if err != nil {
				logger.Warn().Err(err).Msg("Invalid authorization token")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
