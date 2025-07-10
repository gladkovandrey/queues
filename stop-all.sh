#!/bin/bash

set -e

echo "🛑 Остановка всех сервисов..."

# Останавливаем приложения
echo "📱 Остановка приложений..."
docker compose -f docker-compose.apps.yml down --remove-orphans 2>/dev/null || true

# Останавливаем Kafka кластер
echo "☕ Остановка Kafka кластера..."
docker compose down --remove-orphans

echo "🧹 Очистка неиспользуемых ресурсов..."
docker system prune -f --volumes 2>/dev/null || true

echo "✅ Все сервисы остановлены!" 