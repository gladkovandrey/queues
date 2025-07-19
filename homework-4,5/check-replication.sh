#!/bin/bash

# Скрипт для проверки состояния репликации

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 Проверка состояния системы репликации${NC}"
echo ""

# Функция для выполнения SQL запроса
execute_sql() {
    local db=$1
    local query=$2
    docker exec -it $db psql -U postgres -d transactions -c "$query" 2>/dev/null
}

# Проверяем статус контейнеров
echo -e "${YELLOW}📊 Статус контейнеров:${NC}"
CONTAINERS=("postgres-a" "postgres-b" "nats-a-1" "nats-b-1" "data-generator" "outbox-publisher" "inbox-processor")

for container in "${CONTAINERS[@]}"; do
    STATUS=$(docker ps --filter "name=^${container}$" --format "{{.Status}}" 2>/dev/null)
    if [ ! -z "$STATUS" ]; then
        echo -e "  ✅ $container: ${GREEN}$STATUS${NC}"
    else
        echo -e "  ❌ $container: ${RED}Не запущен${NC}"
    fi
done

echo ""

# Проверяем данные в Database A
echo -e "${YELLOW}🗄️ Database A (источник):${NC}"
echo -e "${BLUE}Пользователи:${NC}"
execute_sql postgres-a "SELECT COUNT(*) as total_users FROM users;"

echo -e "${BLUE}Платежи:${NC}"
execute_sql postgres-a "SELECT COUNT(*) as total_payments FROM payments;"

echo -e "${BLUE}Outbox (необработанные):${NC}"
execute_sql postgres-a "SELECT COUNT(*) as unprocessed_events FROM outbox WHERE processed = false;"

echo -e "${BLUE}Outbox (всего событий):${NC}"
execute_sql postgres-a "SELECT COUNT(*) as total_events FROM outbox;"

echo ""

# Проверяем данные в Database B
echo -e "${YELLOW}🗄️ Database B (назначение):${NC}"
echo -e "${BLUE}Пользователи:${NC}"
execute_sql postgres-b "SELECT COUNT(*) as total_users FROM users;"

echo -e "${BLUE}Платежи:${NC}"
execute_sql postgres-b "SELECT COUNT(*) as total_payments FROM payments;"

echo -e "${BLUE}Inbox (необработанные):${NC}"
execute_sql postgres-b "SELECT COUNT(*) as unprocessed_events FROM inbox WHERE processed = false;"

echo -e "${BLUE}Inbox (всего событий):${NC}"
execute_sql postgres-b "SELECT COUNT(*) as total_events FROM inbox;"

echo -e "${BLUE}Обработанные события:${NC}"
execute_sql postgres-b "SELECT COUNT(*) as processed_events FROM processed_events;"

echo ""

# Сравнение данных
echo -e "${YELLOW}⚖️ Сравнение данных:${NC}"

USERS_A=$(docker exec postgres-a psql -U postgres -d transactions -t -c "SELECT COUNT(*) FROM users;" 2>/dev/null | tr -d ' ')
USERS_B=$(docker exec postgres-b psql -U postgres -d transactions -t -c "SELECT COUNT(*) FROM users;" 2>/dev/null | tr -d ' ')

PAYMENTS_A=$(docker exec postgres-a psql -U postgres -d transactions -t -c "SELECT COUNT(*) FROM payments;" 2>/dev/null | tr -d ' ')
PAYMENTS_B=$(docker exec postgres-b psql -U postgres -d transactions -t -c "SELECT COUNT(*) FROM payments;" 2>/dev/null | tr -d ' ')

if [ "$USERS_A" = "$USERS_B" ]; then
    echo -e "  ✅ Пользователи: ${GREEN}$USERS_A = $USERS_B${NC}"
else
    echo -e "  ⚠️ Пользователи: ${YELLOW}$USERS_A ≠ $USERS_B${NC}"
fi

if [ "$PAYMENTS_A" = "$PAYMENTS_B" ]; then
    echo -e "  ✅ Платежи: ${GREEN}$PAYMENTS_A = $PAYMENTS_B${NC}"
else
    echo -e "  ⚠️ Платежи: ${YELLOW}$PAYMENTS_A ≠ $PAYMENTS_B${NC}"
fi

echo ""

# Последние события
echo -e "${YELLOW}📋 Последние события:${NC}"
echo -e "${BLUE}Database A - последние пользователи:${NC}"
execute_sql postgres-a "SELECT name, email, created_at FROM users ORDER BY created_at DESC LIMIT 3;"

echo -e "${BLUE}Database B - последние пользователи:${NC}"
execute_sql postgres-b "SELECT name, email, replicated_at FROM users ORDER BY replicated_at DESC LIMIT 3;"

echo ""

# Проверяем задержку репликации
echo -e "${YELLOW}⏱️ Задержка репликации:${NC}"
LAST_OUTBOX=$(docker exec postgres-a psql -U postgres -d transactions -t -c "SELECT EXTRACT(EPOCH FROM NOW() - MAX(created_at)) FROM outbox;" 2>/dev/null | tr -d ' ')
LAST_INBOX=$(docker exec postgres-b psql -U postgres -d transactions -t -c "SELECT EXTRACT(EPOCH FROM NOW() - MAX(created_at)) FROM inbox;" 2>/dev/null | tr -d ' ')

if [ ! -z "$LAST_OUTBOX" ] && [ "$LAST_OUTBOX" != "" ]; then
    echo -e "  📤 Последнее событие в outbox: ${YELLOW}${LAST_OUTBOX%.*} секунд назад${NC}"
fi

if [ ! -z "$LAST_INBOX" ] && [ "$LAST_INBOX" != "" ]; then
    echo -e "  📥 Последнее событие в inbox: ${YELLOW}${LAST_INBOX%.*} секунд назад${NC}"
fi

echo ""
echo -e "${GREEN}✅ Проверка завершена${NC}" 