package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// Message representa uma mensagem trocada entre os usuários
type Message struct {
	ID         int64  `json:"id,omitempty"`
	SenderID   int    `json:"sender_id"`
	ReceiverID int    `json:"receiver_id"`
	Content    string `json:"content"`
	Timestamp  string `json:"timestamp"`
	Status     string `json:"status"` // "sent", "delivered", "read"
}

// UserStatus representa o status de um usuário
type UserStatus struct {
	UserID   int    `json:"user_id"`
	Status   string `json:"status"` // "online", "offline"
	LastSeen string `json:"last_seen,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Implementação no futuro...
		return true
	},
}

var (
	broadcast  = make(chan Message)            // Canal para transmissão de mensagens
	clients    = make(map[*websocket.Conn]int) // Mapeia conexões WebSocket para IDs de usuários
	clientsMu  sync.Mutex                      // Mutex para proteger o mapa clients
	userStatus = make(map[int]UserStatus)      // Mapeia status dos usuários
	statusMu   sync.Mutex                      // Mutex para proteger o mapa userStatus
	db         *sql.DB
	log        = logrus.New()
	jwtSecret  string
)

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
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true", dbUser, dbPassword, dbHost, dbName)
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Configurar propriedades do pool de conexões
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verifica se a conexão com o banco de dados está ativa
	if err = db.Ping(); err != nil {
		log.WithError(err).Fatal("Database ping failed")
	}

	// Inicializa as tabelas se necessário
	initDatabase()

	// Configuração de logging
	log.SetFormatter(&logrus.JSONFormatter{})
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.Out = logFile
	}

	// Define rotas HTTP
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/health", healthCheck)
	http.HandleFunc("/messages/history", handleMessageHistory)
	http.HandleFunc("/users/status", handleUserStatus)

	go handleMessages()

	// Inicia o servidor com timeout configurado
	server := &http.Server{
		Addr:         ":8081",
		Handler:      nil,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("Application up and running on port 8081")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Server failed")
		}
	}()

	// Captura sinais de interrupção para desligamento gracioso
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down server...")

	// Atualiza status de todos os usuários para offline
	updateAllUsersOffline()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.WithError(err).Error("Shutdown error")
	}

	log.Info("Server stopped")
}

// Inicializa as tabelas do banco de dados
func initDatabase() {
	// Tabela de mensagens
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS messages (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		sender_id INT NOT NULL,
		receiver_id INT NOT NULL,
		content TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		status VARCHAR(10) NOT NULL DEFAULT 'sent',
		INDEX idx_sender (sender_id),
		INDEX idx_receiver (receiver_id),
		INDEX idx_timestamp (timestamp)
	)
	`)
	if err != nil {
		log.WithError(err).Fatal("Failed to create messages table")
	}

	// Tabela de status dos usuários
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS user_status (
		user_id INT PRIMARY KEY,
		status VARCHAR(10) NOT NULL DEFAULT 'offline',
		last_seen DATETIME NOT NULL
	)
	`)
	if err != nil {
		log.WithError(err).Fatal("Failed to create user_status table")
	}
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

	// Registra o cliente na lista de conexões ativas
	clientsMu.Lock()
	clients[ws] = userID
	clientsMu.Unlock()

	// Atualiza o status do usuário para online
	updateUserStatus(userID, "online")

	// Envia mensagens não entregues para o usuário que acabou de conectar
	sendPendingMessages(userID, ws)

	// Notifica outros usuários sobre a mudança de status
	broadcastStatusChange(userID, "online")

	// Quando a conexão for fechada
	defer func() {
		clientsMu.Lock()
		delete(clients, ws)
		clientsMu.Unlock()
		ws.Close()

		// Atualiza o status do usuário para offline
		updateUserStatus(userID, "offline")

		// Notifica outros usuários sobre a mudança de status
		broadcastStatusChange(userID, "offline")
	}()

	// Loop principal para leitura de mensagens
	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.WithError(err).Warn("Error reading message, closing connection")
			break
		}

		// Garante que o ID do remetente seja o do usuário autenticado
		msg.SenderID = userID
		msg.Timestamp = time.Now().Format(time.RFC3339)
		msg.Status = "sent"

		// Salva a mensagem no banco de dados
		msgID, err := saveMessage(msg)
		if err != nil {
			log.WithError(err).Error("Failed to save message")
		} else {
			msg.ID = msgID
		}

		// Envia a mensagem para o destinatário
		broadcast <- msg
	}
}

// Envia mensagens pendentes para um usuário que acabou de se conectar
func sendPendingMessages(userID int, conn *websocket.Conn) {
	rows, err := db.Query(`
		SELECT id, sender_id, receiver_id, content, timestamp, status 
		FROM messages 
		WHERE receiver_id = ? AND status = 'sent'
		ORDER BY timestamp ASC
	`, userID)
	if err != nil {
		log.WithError(err).Error("Failed to fetch pending messages")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var msg Message
		var timestamp time.Time
		err := rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Content, &timestamp, &msg.Status)
		if err != nil {
			log.WithError(err).Error("Failed to scan message row")
			continue
		}
		msg.Timestamp = timestamp.Format(time.RFC3339)

		// Envia a mensagem para o usuário
		if err := conn.WriteJSON(msg); err != nil {
			log.WithError(err).Error("Failed to send pending message")
			return
		}

		// Atualiza o status da mensagem para "delivered"
		_, err = db.Exec("UPDATE messages SET status = 'delivered' WHERE id = ?", msg.ID)
		if err != nil {
			log.WithError(err).Error("Failed to update message status")
		}
	}
}

// Salva uma mensagem no banco de dados
func saveMessage(msg Message) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO messages (sender_id, receiver_id, content, timestamp, status)
		VALUES (?, ?, ?, ?, ?)
	`, msg.SenderID, msg.ReceiverID, msg.Content, time.Now(), msg.Status)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Atualiza o status de um usuário
func updateUserStatus(userID int, status string) {
	statusMu.Lock()
	now := time.Now()
	lastSeen := now.Format(time.RFC3339)
	userStatus[userID] = UserStatus{
		UserID:   userID,
		Status:   status,
		LastSeen: lastSeen,
	}
	statusMu.Unlock()

	// Atualiza o status no banco de dados
	_, err := db.Exec(`
		INSERT INTO user_status (user_id, status, last_seen)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
		status = VALUES(status),
		last_seen = VALUES(last_seen)
	`, userID, status, now)
	if err != nil {
		log.WithError(err).Error("Failed to update user status")
	}
}

// Marca todos os usuários como offline quando o servidor é desligado
func updateAllUsersOffline() {
	_, err := db.Exec("UPDATE user_status SET status = 'offline', last_seen = ? WHERE status = 'online'", time.Now())
	if err != nil {
		log.WithError(err).Error("Failed to set all users offline")
	}
}

// Transmite a mudança de status para todos os usuários conectados
func broadcastStatusChange(userID int, status string) {
	statusUpdate := UserStatus{
		UserID:   userID,
		Status:   status,
		LastSeen: time.Now().Format(time.RFC3339),
	}

	clientsMu.Lock()
	for client := range clients {
		if err := client.WriteJSON(map[string]interface{}{
			"type":   "status_update",
			"status": statusUpdate,
		}); err != nil {
			log.WithError(err).Warn("Error sending status update")
		}
	}
	clientsMu.Unlock()
}

// Gerencia o envio de mensagens entre usuários
func handleMessages() {
	for {
		msg := <-broadcast

		clientsMu.Lock()
		delivered := false
		for client, userID := range clients {
			if userID == msg.ReceiverID {
				err := client.WriteJSON(msg)
				if err != nil {
					log.WithError(err).Warn("Write to client failed")
					client.Close()
					delete(clients, client)
				} else {
					delivered = true
					// Atualiza o status da mensagem para "delivered"
					_, err = db.Exec("UPDATE messages SET status = 'delivered' WHERE id = ?", msg.ID)
					if err != nil {
						log.WithError(err).Error("Failed to update message status")
					}
				}
			}
		}
		clientsMu.Unlock()

		// Se a mensagem não foi entregue, ela permanecerá com status "sent" no banco
		if !delivered {
			log.WithFields(logrus.Fields{
				"message_id":  msg.ID,
				"receiver_id": msg.ReceiverID,
			}).Info("Message not delivered, user offline")
		}
	}
}

// Endpoint para obter histórico de mensagens
func handleMessageHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validação de autenticação
	userID, err := authenticateRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parâmetros de filtragem
	otherUserID := r.URL.Query().Get("user_id")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "50" // Valor padrão
	}

	var rows *sql.Rows
	if otherUserID != "" {
		// Histórico de conversa entre dois usuários específicos
		rows, err = db.Query(`
			SELECT id, sender_id, receiver_id, content, timestamp, status
			FROM messages
			WHERE (sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)
			ORDER BY timestamp DESC
			LIMIT ?
		`, userID, otherUserID, otherUserID, userID, limit)
	} else {
		// Todas as mensagens do usuário
		rows, err = db.Query(`
			SELECT id, sender_id, receiver_id, content, timestamp, status
			FROM messages
			WHERE sender_id = ? OR receiver_id = ?
			ORDER BY timestamp DESC
			LIMIT ?
		`, userID, userID, limit)
	}

	if err != nil {
		log.WithError(err).Error("Failed to query message history")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var timestamp time.Time
		err := rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Content, &timestamp, &msg.Status)
		if err != nil {
			log.WithError(err).Error("Failed to scan message row")
			continue
		}
		msg.Timestamp = timestamp.Format(time.RFC3339)
		messages = append(messages, msg)
	}

	// Marca mensagens como lidas se o usuário atual for o destinatário
	if otherUserID != "" {
		_, err = db.Exec(`
			UPDATE messages 
			SET status = 'read' 
			WHERE sender_id = ? AND receiver_id = ? AND status = 'delivered'
		`, otherUserID, userID)
		if err != nil {
			log.WithError(err).Error("Failed to mark messages as read")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// Endpoint para obter status dos usuários
func handleUserStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validação de autenticação
	_, err := authenticateRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Consulta status de todos os usuários ou de um usuário específico
	userID := r.URL.Query().Get("user_id")
	var rows *sql.Rows

	if userID != "" {
		rows, err = db.Query("SELECT user_id, status, last_seen FROM user_status WHERE user_id = ?", userID)
	} else {
		rows, err = db.Query("SELECT user_id, status, last_seen FROM user_status")
	}

	if err != nil {
		log.WithError(err).Error("Failed to query user status")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var statuses []UserStatus
	for rows.Next() {
		var status UserStatus
		var lastSeen time.Time
		err := rows.Scan(&status.UserID, &status.Status, &lastSeen)
		if err != nil {
			log.WithError(err).Error("Failed to scan user status row")
			continue
		}
		status.LastSeen = lastSeen.Format(time.RFC3339)
		statuses = append(statuses, status)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statuses)
}

// Autentica o usuário a partir do token JWT
func authenticateRequest(r *http.Request) (int, error) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" || !strings.HasPrefix(tokenString, "Bearer ") {
		return 0, fmt.Errorf("unauthorized")
	}
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("invalid signing method")
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("invalid token claims")
	}
	userIDFloat, ok := claims["id"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid user ID")
	}

	return int(userIDFloat), nil
}

// Verifica a saúde do servidor e a conexão com o banco de dados
func healthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type HealthStatus struct {
		Status    string `json:"status"`
		Database  bool   `json:"database"`
		Timestamp string `json:"timestamp"`
		Version   string `json:"version"`
	}

	health := HealthStatus{
		Status:    "OK",
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   "1.0.0",
	}

	if err := db.Ping(); err != nil {
		health.Status = "ERROR"
		health.Database = false
		log.WithError(err).Error("Health check failed")
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		health.Database = true
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}
