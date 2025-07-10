#!/bin/bash

set -e

# Значения по умолчанию
PRODUCERS=1
CONSUMERS=1
MESSAGES_PER_SECOND=1
DURATION=0

# Функция помощи
show_help() {
    echo "Использование: $0 [ОПЦИИ]"
    echo ""
    echo "Опции:"
    echo "  -p, --producers NUM         Количество продюсеров (по умолчанию: 1)"
    echo "  -c, --consumers NUM         Количество консьюмеров (по умолчанию: 1)"
    echo "  -r, --rate NUM              Сообщений в секунду на продюсер (по умолчанию: 1)"
    echo "  -d, --duration NUM          Длительность в секундах (0 = бесконечно, по умолчанию: 0)"
    echo "  -h, --help                  Показать эту справку"
    echo ""
    echo "Примеры:"
    echo "  $0 -p 3 -c 2 -r 5           Запустить 3 продюсера и 2 консьюмера, 5 сообщений/сек"
    echo "  $0 -p 5 -c 3 -r 10 -d 300   Запустить на 5 минут с высокой нагрузкой"
    echo "  $0 -p 0 -c 5                Запустить только консьюмеры"
}

# Парсинг аргументов
while [[ $# -gt 0 ]]; do
    case $1 in
        -p|--producers)
            PRODUCERS="$2"
            shift 2
            ;;
        -c|--consumers)
            CONSUMERS="$2"
            shift 2
            ;;
        -r|--rate)
            MESSAGES_PER_SECOND="$2"
            shift 2
            ;;
        -d|--duration)
            DURATION="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "Неизвестная опция: $1"
            show_help
            exit 1
            ;;
    esac
done

echo "🔧 Настройки масштабирования:"
echo "   Продюсеры: $PRODUCERS"
echo "   Консьюмеры: $CONSUMERS"
echo "   Скорость: $MESSAGES_PER_SECOND сообщений/сек на продюсер"
echo "   Длительность: $([ $DURATION -eq 0 ] && echo "бесконечно" || echo "$DURATION сек")"

# Проверяем что Kafka запущен
if ! docker compose ps | grep -q "kafka-1.*Up"; then
    echo "❌ Kafka кластер не запущен!"
    echo "💡 Запустите сначала: ./start-kafka.sh"
    exit 1
fi

# Экспортируем переменные окружения
export MESSAGES_PER_SECOND
export DURATION_SECONDS=$DURATION

echo ""
echo "🚀 Запуск приложений..."

# Останавливаем существующие приложения если есть
docker compose -f docker-compose.apps.yml down 2>/dev/null || true

# Запускаем приложения с масштабированием
if [ $PRODUCERS -gt 0 ]; then
    echo "📤 Запуск $PRODUCERS продюсер(ов)..."
    docker compose -f docker-compose.apps.yml up -d --build --scale producer=$PRODUCERS producer
fi

if [ $CONSUMERS -gt 0 ]; then
    echo "📥 Запуск $CONSUMERS консьюмер(ов)..."
    docker compose -f docker-compose.apps.yml up -d --build --scale consumer=$CONSUMERS consumer
fi

echo ""
echo "✅ Приложения запущены!"
echo ""
echo "📊 Статус сервисов:"
docker compose -f docker-compose.apps.yml ps

echo ""
echo "📝 Полезные команды:"
echo "   Просмотр логов продюсеров:  docker compose -f docker-compose.apps.yml logs -f producer"
echo "   Просмотр логов консьюмеров: docker compose -f docker-compose.apps.yml logs -f consumer"
echo "   Остановка приложений:       docker compose -f docker-compose.apps.yml down"
echo "   Kafka UI:                   http://localhost:8180"

# Если указана длительность, ожидаем завершения
if [ $DURATION -gt 0 ]; then
    echo ""
    echo "⏱️  Работаем $DURATION секунд..."
    sleep $DURATION
    echo "⏹️  Время истекло, останавливаем приложения..."
    docker compose -f docker-compose.apps.yml down
    echo "🏁 Готово!"
fi 