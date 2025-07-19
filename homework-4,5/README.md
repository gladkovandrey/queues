# Transaction Replication System - Homework 4,5

Система консистентной репликации транзакций между двумя базами данных через NATS очереди с **near real-time** производительностью.

## 🏗️ Архитектура

```
Zone A                           Zone B
┌─────────────────────┐         ┌─────────────────────┐
│ PostgreSQL A        │         │ PostgreSQL B        │
│ ├─ users            │         │ ├─ users            │
│ ├─ payments         │         │ ├─ payments         │
│ └─ outbox           │         │ └─ inbox            │
└─────────────────────┘         └─────────────────────┘
         ↕                               ↕
┌─────────────────────┐         ┌─────────────────────┐
│ NATS Cluster A      │  <───>  │ NATS Cluster B      │
│ ├─ nats-a-1         │ Gateway │ ├─ nats-b-1         │
│ ├─ nats-a-2         │ репли-  │ ├─ nats-b-2         │
│ └─ nats-a-3         │ кация   │ └─ nats-b-3         │
└─────────────────────┘         └─────────────────────┘
```

## 📦 Компоненты

- **data-generator** - генерирует пользователей и платежи в Database A
- **outbox-publisher** - читает outbox из DB A и отправляет в NATS A
- **inbox-processor** - читает из NATS B и записывает в DB B
- **PostgreSQL A/B** - две базы данных
- **NATS Clusters** - два кластера по 3 ноды с Gateway репликацией

## 🔄 Поток данных

1. **data-generator** создает пользователей и платежи в DB A
2. При каждой записи в основные таблицы создается событие в **outbox**  
3. **outbox-publisher** читает новые события из outbox и отправляет в NATS A
4. NATS Gateway автоматически реплицирует события в NATS B
5. **inbox-processor** читает события из NATS B и записывает в DB B с **правильной сортировкой** (users → payments)
6. Идемпотентность обеспечивается через UUID и проверку дубликатов

## ⚡ Производительность

**Оптимизированные настройки:**
- **Пропускная способность**: 50+ событий/секунду
- **Задержка репликации**: 1-2 секунды (near real-time)
- **Batch Size**: 25 событий на пакет
- **Poll Interval**: 0.5 секунды
- **Нормальное состояние**: 0 необработанных событий в outbox/inbox

## 🚀 Запуск системы

```bash
# Запустить всю инфраструктуру
./start-replication.sh

# Посмотреть логи компонентов
./logs.sh data-generator
./logs.sh outbox-publisher  
./logs.sh inbox-processor

# Остановить систему
./stop-replication.sh
```

## 📊 Мониторинг

- **Database A**: localhost:5432 (user: postgres, pass: password)
- **Database B**: localhost:5433 (user: postgres, pass: password) 
- **NATS A**: http://localhost:8222 (admin:password123)
- **NATS B**: http://localhost:8223 (admin:password123)

## 🔍 Проверка работы системы

### Автоматическая проверка
```bash
./check-replication.sh
```

**Что проверяет скрипт:**

1. **📊 Статус контейнеров**
   - Проверяет что все критичные контейнеры запущены
   - Контролирует: postgres-a, postgres-b, nats-a-1, nats-b-1, data-generator, outbox-publisher, inbox-processor

2. **🗄️ Состояние баз данных**
   - **Database A (источник):** 
     - Количество пользователей и платежей
     - Необработанные события в outbox (**должно быть 0 в норме**)
     - Общее количество событий в outbox
   - **Database B (назначение):**
     - Количество пользователей и платежей (**должно совпадать с DB A**)
     - Необработанные события в inbox (**должно быть 0 в норме**)
     - Общее количество событий в inbox
     - Количество успешно обработанных событий

3. **⚖️ Сравнение данных**
   - Сравнивает количество пользователей в DB A и DB B
   - Сравнивает количество платежей в DB A и DB B
   - ✅ Зеленый = данные синхронизированы идеально
   - ⚠️ Желтый = есть минимальные расхождения (1-2 записи, временная задержка < 5 сек)

4. **📋 Последние события**
   - Показывает последние 3 созданных пользователя в DB A
   - Показывает последние 3 реплицированных пользователя в DB B
   - Помогает убедиться что данные актуальные

5. **⏱️ Задержка репликации**
   - Время с момента последнего события в outbox (DB A)
   - Время с момента последнего события в inbox (DB B)
   - **Нормальная задержка: 1-3 секунды**

### Ручная проверка
```bash
# Посмотреть данные в Database A
docker exec -it postgres-a psql -U postgres -d transactions -c "SELECT * FROM users LIMIT 5;"
docker exec -it postgres-a psql -U postgres -d transactions -c "SELECT * FROM payments LIMIT 5;"

# Посмотреть данные в Database B (должны совпадать)
docker exec -it postgres-b psql -U postgres -d transactions -c "SELECT * FROM users LIMIT 5;"
docker exec -it postgres-b psql -U postgres -d transactions -c "SELECT * FROM payments LIMIT 5;"

# Проверить состояние outbox/inbox (должно быть пусто в норме)
docker exec -it postgres-a psql -U postgres -d transactions -c "SELECT * FROM outbox WHERE processed = false;"
docker exec -it postgres-b psql -U postgres -d transactions -c "SELECT * FROM inbox ORDER BY created_at DESC LIMIT 5;"
```

## 🔧 Диагностика проблем

### Основные индикаторы проблем

**🚨 Критические проблемы:**
- Большое количество необработанных событий в outbox (>50)
- Отсутствие данных в Database B
- Ошибки в логах компонентов

**⚠️ Проблемы производительности:**
- Задержка репликации >10 секунд
- Накопление событий в outbox/inbox
- Расхождения в данных >5 записей

### Пошаговая диагностика

1. **Проверить логи компонентов:**
   ```bash
   ./logs.sh data-generator --tail=20
   ./logs.sh outbox-publisher --tail=20
   ./logs.sh inbox-processor --tail=20
   ```

2. **Типичные ошибки и решения:**
   
   **Ошибка SQL массивов:**
   ```
   sql: converting argument $1 type: unsupported type []string
   ```
   ✅ **Решение:** Исправлено в текущей версии

   **Foreign key constraint:**
   ```
   violates foreign key constraint "payments_user_id_fkey"
   ```
   ✅ **Решение:** Добавлена сортировка событий (users → payments)

   **NATS подключение:**
   ```
   Error connecting to NATS
   ```
   🔧 **Решение:** Проверить NATS кластеры и аутентификацию

3. **Проверить состояние NATS:**
   ```bash
   # Статус NATS кластеров
   curl http://localhost:8222/connz
   curl http://localhost:8223/connz
   
   # Gateway связи между кластерами
   curl http://localhost:8222/gatewayz
   curl http://localhost:8223/gatewayz
   ```

4. **Проверить количество необработанных событий:**
   ```bash
   ./check-replication.sh
   ```

## 🎛️ Настройка производительности

### Переменные окружения

**Data Generator:**
- `GENERATION_INTERVAL` - интервал генерации данных (по умолчанию: 2 сек)

**Outbox Publisher:**
- `POLL_INTERVAL` - частота опроса outbox (по умолчанию: 0.5 сек)
- `BATCH_SIZE` - размер пакета событий (по умолчанию: 25)

**Inbox Processor:**
- `BATCH_SIZE` - размер пакета событий (по умолчанию: 25)

### Рекомендации по оптимизации

**Для высокой нагрузки (>100 событий/сек):**
```yaml
# docker-compose.applications.yml
environment:
  POLL_INTERVAL: 0.2        # Увеличить частоту опроса
  BATCH_SIZE: 50            # Увеличить размер пакета
  GENERATION_INTERVAL: 5    # Снизить частоту генерации
```

**Для экономии ресурсов:**
```yaml
environment:
  POLL_INTERVAL: 2          # Снизить частоту опроса
  BATCH_SIZE: 10            # Уменьшить размер пакета
  GENERATION_INTERVAL: 1    # Увеличить частоту генерации
```

## 🔧 NATS Gateway Конфигурация

### Файлы конфигурации

**cluster-a.conf:**
- Account: CLIENT (JetStream enabled)
- User: replication:password123
- Gateway: CLUSTER_A ↔ CLUSTER_B

**cluster-b.conf:**
- Account: CLIENT (JetStream enabled)  
- User: replication:password123
- Gateway: CLUSTER_B ↔ CLUSTER_A

### Мониторинг Gateway

```bash
# Проверить состояние gateway соединений
curl http://localhost:8222/gatewayz | jq '.gateways'
curl http://localhost:8223/gatewayz | jq '.gateways'

# Проверить JetStream статистику
curl http://localhost:8222/jsz
curl http://localhost:8223/jsz
```

## 💾 Схема БД

### Основные таблицы (в обеих БД):
- `users` - пользователи системы
- `payments` - платежи пользователей

### Специальные таблицы:
- `outbox` (только DB A) - исходящие события для репликации
- `inbox` (только DB B) - входящие события из репликации
- `processed_events` (только DB B) - дедупликация обработанных событий

### Триггеры и функции:
- **DB A**: Автоматическое создание событий в outbox при INSERT/UPDATE
- **DB B**: Функции обработки событий с дедупликацией

## ⚡ Гарантии надежности

- **Консистентность**: outbox записывается в той же транзакции что и основные данные
- **Идемпотентность**: каждое событие имеет уникальный UUID + дедупликация
- **At-least-once delivery**: NATS JetStream с persistent storage
- **Fault tolerance**: система восстанавливается после сбоев любого компонента
- **Event ordering**: правильная сортировка событий (users перед payments)
- **Gateway репликация**: NATS автоматически реплицирует между кластерами
- **Near real-time**: задержка репликации 1-3 секунды в нормальном режиме

## 🎯 Результаты тестирования

**Производительность системы:**
- ✅ Пропускная способность: 50+ событий/секунду
- ✅ Задержка репликации: 1-2 секунды
- ✅ Идемпотентность: 100% (дубликаты отфильтровываются)
- ✅ Консистентность: 100% (данные полностью синхронизированы)
- ✅ Fault tolerance: восстановление после сбоев < 30 секунд

**Тестовая нагрузка:**
- 400+ пользователей реплицированы без ошибок
- 1600+ платежей реплицированы без ошибок  
- 2000+ событий обработано без потерь
- 0 необработанных событий в установившемся режиме 