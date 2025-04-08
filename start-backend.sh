#!/bin/bash

# Configurações
BACKEND_PORT=3333
ROOT_DIR=$(pwd)
LOG_DIR="$ROOT_DIR/logs"
LOG_FILE="$LOG_DIR/backend.log"

# Função de erro
function erro() {
  echo "❌ Erro: $1"
  exit 1
}

# Cria diretório de logs
mkdir -p "$LOG_DIR"

# Verifica ferramentas
command -v docker &> /dev/null || erro "Docker não instalado."
docker compose version &> /dev/null || erro "Docker Compose não disponível."

# Verifica se porta está ocupada
if lsof -i :$BACKEND_PORT &> /dev/null; then
  erro "Porta $BACKEND_PORT em uso. Encerre o processo ou altere a porta do backend."
fi

# Reinicia ambiente
echo "♻️ Encerrando containers antigos..."
docker compose down

echo "🚀 Iniciando containers backend..."
docker compose up -d || erro "Falha ao iniciar os containers."

# Aguardar subida dos serviços
sleep 5

# Status
echo "📦 Containers ativos:"
docker compose ps

# Log persistente
echo "📝 Registrando logs em $LOG_FILE"
docker compose logs -f backend &> "$LOG_FILE" &

echo -e "\n✅ Backend em execução!"
echo "👉 Porta: $BACKEND_PORT"
echo "👉 Logs persistentes: $LOG_FILE"
