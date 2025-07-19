#!/bin/bash

echo "🛑 Остановка системы репликации транзакций..."

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Остановка приложений
echo -e "${YELLOW}🛑 Остановка приложений...${NC}"
docker compose -f docker-compose.applications.yml down

# Остановка инфраструктуры
echo -e "${YELLOW}🛑 Остановка инфраструктуры...${NC}"
docker compose -f docker-compose.infrastructure.yml down

# Опциональная очистка volumes (раскомментировать при необходимости)
# echo -e "${YELLOW}🗑️ Очистка данных...${NC}"
# docker compose -f docker-compose.infrastructure.yml down -v

echo -e "${GREEN}✅ Система остановлена${NC}"

# Показываем оставшиеся контейнеры (если есть)
REMAINING=$(docker ps -q --filter "name=nats-" --filter "name=postgres-" --filter "name=data-generator" --filter "name=outbox-publisher" --filter "name=inbox-processor")
if [ ! -z "$REMAINING" ]; then
    echo -e "${YELLOW}⚠️ Оставшиеся контейнеры:${NC}"
    docker ps --filter "name=nats-" --filter "name=postgres-" --filter "name=data-generator" --filter "name=outbox-publisher" --filter "name=inbox-processor"
    echo ""
    echo -e "${YELLOW}💡 Для полной очистки выполните:${NC}"
    echo "  docker stop \$(docker ps -q --filter \"name=nats-\" --filter \"name=postgres-\" --filter \"name=data-generator\" --filter \"name=outbox-publisher\" --filter \"name=inbox-processor\")"
fi 