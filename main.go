package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

type Message struct {
	SenderID   int    `json:"sender_id"`
	ReceiverID int    `json:"receiver_id"`
	Content    string `json:"content"`
	Timestamp  string `json:"timestamp"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}
var broadcast = make(chan Message)
var clients = make(map[*websocket.Conn]int)
var db *sql.DB
var log = logrus.New()
var jwtSecret string

func main() {
	if err := godotenv.Load(); err != nil {
		log.Info("No .env file found")
	}

	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	jwtSecret = os.Getenv("JWT_SECRET")

	var err error
	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true", dbUser, dbPassword, dbHost, dbName))
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.WithError(err).Fatal("Database ping failed")
	}

	log.SetFormatter(&logrus.JSONFormatter{})
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.Out = logFile
	}

	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/health", healthCheck)
	go handleMessages()

	server := &http.Server{Addr: ":8081", Handler: nil}
	go func() {
		log.Info("Application up and running")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Server failed")
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.WithError(err).Error("Shutdown error")
	}
	log.Info("Server stopped")
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	log.WithFields(logrus.Fields{"remote_addr": r.RemoteAddr}).Info("New WebSocket connection attempt")

	tokenString := r.Header.Get("Authorization")
	if tokenString == "" || !strings.HasPrefix(tokenString, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("invalid signing method")
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, "Invalid token claims", http.StatusUnauthorized)
		return
	}
	userIDFloat, ok := claims["id"].(float64)
	if !ok {
		http.Error(w, "Invalid user ID", http.StatusUnauthorized)
		return
	}
	userID := int(userIDFloat)

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.WithError(err).Error("WebSocket upgrade failed")
		return
	}
	defer ws.Close()

	clients[ws] = userID
	defer delete(clients, ws)
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client, userID := range clients {
			if userID == msg.ReceiverID {
				err := client.WriteJSON(msg)
				if err != nil {
					log.WithError(err).Warn("Write to client failed")
					delete(clients, client)
					client.Close()
				}
			}
		}
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		log.WithError(err).Error("Health check failed")
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}
