#!/bin/bash

echo "🚀 Запуск системы репликации транзакций..."

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Функция для проверки успешности выполнения
check_success() {
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ $1${NC}"
    else
        echo -e "${RED}❌ $1${NC}"
        exit 1
    fi
}

# Остановка старых контейнеров (если есть)
echo -e "${YELLOW}🛑 Остановка старых контейнеров...${NC}"
docker compose -f docker-compose.infrastructure.yml down > /dev/null 2>&1
docker compose -f docker-compose.applications.yml down > /dev/null 2>&1

# Запуск инфраструктуры (NATS + PostgreSQL)
echo -e "${YELLOW}🏗️ Запуск инфраструктуры...${NC}"
docker compose -f docker-compose.infrastructure.yml up -d
check_success "Инфраструктура запущена"

# Ждем инициализации БД
echo -e "${YELLOW}⏳ Ожидание инициализации баз данных (30 секунд)...${NC}"
sleep 30

# Проверяем доступность PostgreSQL A
echo -e "${YELLOW}🔍 Проверка доступности PostgreSQL A...${NC}"
for i in {1..10}; do
    if docker exec postgres-a pg_isready -U postgres > /dev/null 2>&1; then
        echo -e "${GREEN}✅ PostgreSQL A готов${NC}"
        break
    fi
    if [ $i -eq 10 ]; then
        echo -e "${RED}❌ PostgreSQL A недоступен${NC}"
        exit 1
    fi
    sleep 2
done

# Проверяем доступность PostgreSQL B
echo -e "${YELLOW}🔍 Проверка доступности PostgreSQL B...${NC}"
for i in {1..10}; do
    if docker exec postgres-b pg_isready -U postgres > /dev/null 2>&1; then
        echo -e "${GREEN}✅ PostgreSQL B готов${NC}"
        break
    fi
    if [ $i -eq 10 ]; then
        echo -e "${RED}❌ PostgreSQL B недоступен${NC}"
        exit 1
    fi
    sleep 2
done

# Проверяем доступность NATS
echo -e "${YELLOW}🔍 Проверка доступности NATS...${NC}"
sleep 10

# Запуск приложений
echo -e "${YELLOW}🚀 Запуск приложений...${NC}"
docker compose -f docker-compose.applications.yml up -d
check_success "Приложения запущены"

# Ждем стабилизации
echo -e "${YELLOW}⏳ Ожидание стабилизации системы (10 секунд)...${NC}"
sleep 10

# Проверяем статус всех контейнеров
echo -e "${YELLOW}📊 Статус контейнеров:${NC}"
echo ""

# Инфраструктура
echo -e "${YELLOW}🏗️ Инфраструктура:${NC}"
docker compose -f docker-compose.infrastructure.yml ps

echo ""

# Приложения
echo -e "${YELLOW}🚀 Приложения:${NC}"
docker compose -f docker-compose.applications.yml ps

echo ""
echo -e "${GREEN}✅ Система репликации запущена!${NC}"
echo ""
echo -e "${YELLOW}📊 Мониторинг:${NC}"
echo "  • Database A: localhost:5432 (postgres/password)"
echo "  • Database B: localhost:5433 (postgres/password)"
echo "  • NATS A: http://localhost:8222"
echo "  • NATS B: http://localhost:8223"
echo ""
echo -e "${YELLOW}🔍 Полезные команды:${NC}"
echo "  • Логи генератора: ./logs.sh data-generator"
echo "  • Логи publisher: ./logs.sh outbox-publisher"
echo "  • Логи processor: ./logs.sh inbox-processor"
echo "  • Остановить систему: ./stop-replication.sh"
echo "  • Проверить данные: ./check-replication.sh"
echo "" 