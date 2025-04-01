package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var broadcast = make(chan Message)
var clients = make(map[*websocket.Conn]bool)

func main() {
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)
	http.HandleFunc("/ws", handleConnections)

	go handleMessages()

	fmt.Println("Application up and running")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		panic(err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}

	defer ws.Close()
	clients[ws] = true

	for {
		var msg Message
		if err := ws.ReadJSON(&msg); err != nil {
			delete(clients, ws)
			break
		}

		broadcast <- msg
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				delete(clients, client)
				client.Close()
			}
		}
	}
}
