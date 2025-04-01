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

// Estrutura para representar uma mensagem trocada entre os usuários
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
var broadcast = make(chan Message)          // Canal para transmissão de mensagens
var clients = make(map[*websocket.Conn]int) // Mapeia conexões WebSocket para IDs de usuários
var db *sql.DB
var log = logrus.New()
var jwtSecret string

func main() {
	// Carrega variáveis de ambiente do arquivo .env
	if err := godotenv.Load(); err != nil {
		log.Info("No .env file found")
	}

	// Obtém credenciais do banco de dados e chave JWT
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	jwtSecret = os.Getenv("JWT_SECRET")

	// Conecta ao banco de dados
	var err error
	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true", dbUser, dbPassword, dbHost, dbName))
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Verifica se a conexão com o banco de dados está ativa
	if err = db.Ping(); err != nil {
		log.WithError(err).Fatal("Database ping failed")
	}

	// Configuração de logging
	log.SetFormatter(&logrus.JSONFormatter{})
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.Out = logFile
	}

	// Define rotas HTTP
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/health", healthCheck)
	go handleMessages()

	// Inicia o servidor
	server := &http.Server{Addr: ":8081", Handler: nil}
	go func() {
		log.Info("Application up and running")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Server failed")
		}
	}()

	// Captura sinais de interrupção para desligamento gracioso
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

// Gerencia novas conexões WebSocket
func handleConnections(w http.ResponseWriter, r *http.Request) {
	log.WithFields(logrus.Fields{"remote_addr": r.RemoteAddr}).Info("New WebSocket connection attempt")

	// Verifica e extrai o token JWT do cabeçalho Authorization
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" || !strings.HasPrefix(tokenString, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	// Valida o token JWT
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

	// Extrai o ID do usuário a partir dos claims do token
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

	// Faz o upgrade da conexão HTTP para WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.WithError(err).Error("WebSocket upgrade failed")
		return
	}
	defer ws.Close()

	// Registra o cliente na lista de conexões ativas
	clients[ws] = userID
	defer delete(clients, ws)

	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.WithError(err).Warn("Error reading message, closing connection")
			break
		}
		msg.SenderID = userID
		msg.Timestamp = time.Now().Format(time.RFC3339)
		broadcast <- msg
	}
}

// Gerencia o envio de mensagens entre usuários
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

// Verifica a saúde do servidor e a conexão com o banco de dados
func healthCheck(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		log.WithError(err).Error("Health check failed")
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}
