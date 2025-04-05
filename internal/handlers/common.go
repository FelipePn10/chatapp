package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func extractToken(r *http.Request) string {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		return ""
	}
	return strings.TrimPrefix(tokenString, "Bearer ")
}

func LoggingMiddleware(logger *zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote", r.RemoteAddr).
				Dur("duration", time.Since(start)).
				Msg("Request handled")
		})
	}
}
