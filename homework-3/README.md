# Kafka Streams Processing - Домашнее задание 3

Система потоковой обработки данных на базе существующего Kafka кластера.

## 🏗️ Архитектура

```
application-logs (исходные логи)
    ↓
[Mapper] → error-logs (только ERROR логи)
    ↓
[Aggregator] → error-stats (статистика ошибок по сервисам)

service-metrics (метрики производительности)
    ↓
error-logs + service-metrics → [Join Processor] → enriched-errors (ошибки + метрики)
```

## 📦 Компоненты

- **mapper/** - фильтрует ERROR логи и преобразует формат
- **aggregator/** - подсчитывает ошибки по сервисам за 1-минутные окна
- **join-processor/** - объединяет ошибки с метриками производительности 
- **metrics-producer/** - генерирует метрики сервисов (CPU, память, latency)
- **stats-consumer/** - читает и отображает статистику ошибок
- **enriched-consumer/** - читает обогащенные логи ошибок

## 🚀 Запуск

1. **Запустить основной Kafka кластер:**
```bash
cd ..
./start-kafka.sh
```

2. **Запустить базовые producer/consumer:**
```bash
cd ..
./scale-apps.sh
```

3. **Запустить Stream Processing компоненты:**
```bash
docker compose -f docker-compose.streams.yml up -d
```

4. **Посмотреть результаты:**
- Статистика ошибок: `docker compose -f docker-compose.streams.yml logs -f stats-consumer`
- Обогащенные ошибки: `docker compose -f docker-compose.streams.yml logs -f enriched-consumer`
- Kafka UI: http://localhost:8180

## 🛑 Остановка

```bash
docker compose -f docker-compose.streams.yml down
cd ..
./stop-all.sh
```

## 📊 Топики

- `application-logs` - исходные логи (из основного проекта)
- `error-logs` - отфильтрованные ERROR логи
- `service-metrics` - метрики производительности сервисов  
- `error-stats` - агрегированная статистика ошибок
- `enriched-errors` - ошибки, обогащенные метриками

## 🔍 Что происходит

1. **Mapper** читает из `application-logs`, фильтрует только ERROR и записывает в `error-logs`
2. **Aggregator** читает из `error-logs`, считает ошибки по сервисам и записывает в `error-stats`
3. **Metrics Producer** генерирует метрики сервисов в `service-metrics`
4. **Join Processor** объединяет `error-logs` + `service-metrics` → `enriched-errors`
5. **Consumers** читают и красиво отображают результаты 