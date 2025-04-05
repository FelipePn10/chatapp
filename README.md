chat-server/
├── cmd/
│   └── server/
│       └── main.go          # Ponto de entrada da aplicação
├── internal/
│   ├── config/
│   │   └── config.go        # Configurações do aplicativo
│   ├── handlers/
│   │   ├── auth.go          # Manipuladores de autenticação
│   │   ├── messages.go      # Manipuladores de mensagens
│   │   ├── status.go        # Manipuladores de status
│   │   └── websocket.go     # Manipuladores WebSocket
│   ├── models/
│   │   ├── message.go       # Modelo de mensagem
│   │   └── user_status.go   # Modelo de status do usuário
│   ├── repository/
│   │   ├── message_repo.go  # Repositório de mensagens
│   │   └── status_repo.go   # Repositório de status
│   ├── service/
│   │   ├── auth_service.go  # Serviço de autenticação
│   │   ├── message_service.go # Serviço de mensagens
│   │   └── status_service.go # Serviço de status
│   └── websocket/
│       ├── client.go        # Estrutura do cliente WebSocket
│       ├── hub.go           # Hub WebSocket
│       └── message.go       # Mensagens WebSocket
├── pkg/
│   └── jwt/
│       └── jwt.go           # Utilitários JWT
├── go.mod
├── go.sum
└── .env

