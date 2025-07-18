#!/bin/bash

set -e

echo "🚀 Запуск Kafka Streams Processing..."

# Проверяем, что Kafka кластер запущен
echo "🔍 Проверка Kafka кластера..."
if ! docker compose -f ../docker-compose.yml ps kafka-1 | grep -q "Up"; then
    echo "❌ Kafka кластер не запущен!"
    echo "💡 Сначала запустите Kafka: cd .. && ./start-kafka.sh"
    exit 1
fi

# Дополнительная проверка готовности Kafka
echo "⏳ Проверка готовности Kafka..."
sleep 5

# Проверяем доступность Kafka UI как индикатор готовности
if ! docker compose -f ../docker-compose.yml ps kafka-ui | grep -q "Up"; then
    echo "⚠️  Kafka UI не запущен, но продолжаем..."
fi

# Проверяем, что основные producer/consumer запущены
echo "🔍 Проверка базовых приложений..."
if ! docker compose -f ../docker-compose.apps.yml ps producer | grep -q "Up"; then
    echo "⚠️  Базовые приложения не запущены"
    echo "💡 Запускаем базовые приложения..."
    cd ..
    ./scale-apps.sh
    cd homework-3
    sleep 5
fi

echo "🔄 Сборка и запуск Stream Processing компонентов..."

# Строим и запускаем все компоненты
docker compose -f docker-compose.streams.yml up -d --build

echo "⏳ Ожидание запуска компонентов..."
sleep 15

# Проверяем что компоненты подключились к Kafka
echo "🔄 Проверка подключения к Kafka..."
sleep 5

# Проверяем статус компонентов
echo ""
echo "📊 Статус компонентов:"
docker compose -f docker-compose.streams.yml ps

echo ""
echo "✅ Stream Processing запущен!"
echo ""
echo "🔍 Для просмотра логов используйте:"
echo "   📊 Статистика ошибок:    docker compose -f docker-compose.streams.yml logs -f stats-consumer"
echo "   🔗 Обогащенные ошибки:   docker compose -f docker-compose.streams.yml logs -f enriched-consumer"
echo "   🔄 Все компоненты:       docker compose -f docker-compose.streams.yml logs -f"
echo ""
echo "🌐 Kafka UI: http://localhost:8180"
echo ""
echo "🛑 Для остановки: cd .. && ./stop-all.sh" 