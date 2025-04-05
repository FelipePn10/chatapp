package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chatapp/internal/config"
	"github.com/chatapp/internal/handlers"
	"github.com/chatapp/internal/repository"
	"github.com/chatapp/internal/service"
	"github.com/chatapp/internal/websocket"
	"github.com/chatapp/pkg/jwt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/zerolog"
)

func main() {
	cfg := config.LoadConfig()
	logger := config.SetupLogger(cfg)

	db, err := setupDatabase(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	messageRepo := repository.NewMessageRepository(db, &logger.Logger)
	statusRepo := repository.NewStatusRepository(db, &logger.Logger)

	jwtService := jwt.NewJWTService(cfg.JWTSecret)
	authService := service.NewAuthService(jwtService)
	messageService := service.NewMessageService(messageRepo)
	statusService := service.NewStatusService(statusRepo)

	hub := websocket.NewHub(messageService, statusService, &logger.Logger)
	go hub.Run()

	router := mux.NewRouter()
	router.Use(handlers.LoggingMiddleware(&logger.Logger))

	handlers.SetupRoutes(router, hub, authService, messageService, statusService, &logger.Logger)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info().Str("port", cfg.Port).Msg("Server starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	gracefulShutdown(server, hub, statusService, &logger.Logger)
}

func setupDatabase(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return db, nil
}

func gracefulShutdown(server *http.Server, hub *websocket.Hub, statusService service.StatusService, logger *zerolog.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("Shutting down server...")

	if err := statusService.UpdateAllUsersOffline(context.Background()); err != nil {
		logger.Error().Err(err).Msg("Failed to update all users to offline")
	}

	hub.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown error")
	}

	logger.Info().Msg("Server stopped")
}
