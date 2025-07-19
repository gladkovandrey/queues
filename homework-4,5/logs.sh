#!/bin/bash

# Скрипт для просмотра логов компонентов системы

if [ $# -eq 0 ]; then
    echo "📋 Использование: ./logs.sh [компонент]"
    echo ""
    echo "🚀 Доступные компоненты:"
    echo "  • data-generator     - генератор пользователей и платежей"
    echo "  • outbox-publisher   - публикатор событий из outbox"
    echo "  • inbox-processor    - обработчик входящих событий"
    echo "  • postgres-a         - база данных A"
    echo "  • postgres-b         - база данных B"
    echo "  • nats-a-1,2,3       - NATS кластер A"
    echo "  • nats-b-1,2,3       - NATS кластер B"
    echo ""
    echo "📖 Примеры:"
    echo "  ./logs.sh data-generator"
    echo "  ./logs.sh outbox-publisher -f"
    echo "  ./logs.sh inbox-processor --tail=50"
    exit 1
fi

COMPONENT=$1
shift  # Убираем первый аргумент, оставляем остальные для docker logs

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}📋 Логи компонента: ${GREEN}$COMPONENT${NC}"
echo ""

# Проверяем, что контейнер существует
if ! docker ps -a --format "table {{.Names}}" | grep -q "^$COMPONENT$"; then
    echo "❌ Контейнер '$COMPONENT' не найден"
    echo ""
    echo "🔍 Доступные контейнеры:"
    docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "(nats-|postgres-|data-generator|outbox-publisher|inbox-processor)"
    exit 1
fi

# Показываем логи
docker logs "$COMPONENT" "$@" 