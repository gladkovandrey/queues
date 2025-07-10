#!/bin/bash

echo "🚀 Запуск Kafka кластера..."

# Запускаем Kafka кластер
docker compose up -d

echo "⏳ Ждем запуска Kafka (30 секунд)..."
sleep 30

# Создаем топик
echo "📝 Создание топика application-logs..."
docker compose exec kafka-1 /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 \
    --create --if-not-exists \
    --topic application-logs \
    --partitions 3 \
    --replication-factor 3

echo "✅ Kafka кластер готов!"
echo "📈 Веб-интерфейс: http://localhost:8180" 