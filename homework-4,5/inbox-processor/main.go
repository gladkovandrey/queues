package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type ReplicationMessage struct {
	EventID       uuid.UUID       `json:"event_id"`
	AggregateID   uuid.UUID       `json:"aggregate_id"`
	AggregateType string          `json:"aggregate_type"`
	EventType     string          `json:"event_type"`
	EventData     json.RawMessage `json:"event_data"`
	OriginalTime  time.Time       `json:"original_time"`
	PublishedAt   time.Time       `json:"published_at"`
}

type InboxEvent struct {
	ID            uuid.UUID       `json:"id"`
	EventID       uuid.UUID       `json:"event_id"`
	AggregateID   uuid.UUID       `json:"aggregate_id"`
	AggregateType string          `json:"aggregate_type"`
	EventType     string          `json:"event_type"`
	EventData     json.RawMessage `json:"event_data"`
	CreatedAt     time.Time       `json:"created_at"`
}

func main() {
	log.Println("📥 Inbox Processor запускается...")

	// Получаем настройки из environment
	dbHost := getEnvOrDefault("DB_HOST", "localhost")
	dbPort := getEnvOrDefault("DB_PORT", "5433")
	dbUser := getEnvOrDefault("DB_USER", "postgres")
	dbPassword := getEnvOrDefault("DB_PASSWORD", "password")
	dbName := getEnvOrDefault("DB_NAME", "transactions")

	natsURL := getEnvOrDefault("NATS_URL", "nats://localhost:4225")
	streamName := getEnvOrDefault("STREAM_NAME", "REPLICATION")
	subject := getEnvOrDefault("SUBJECT", "replication.events")
	consumerName := getEnvOrDefault("CONSUMER_NAME", "inbox-processor")

	batchSizeStr := getEnvOrDefault("BATCH_SIZE", "10")
	batchSize, err := strconv.Atoi(batchSizeStr)
	if err != nil {
		batchSize = 10
	}

	// Подключение к базе данных
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("❌ Ошибка подключения к БД: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ БД недоступна: %v", err)
	}

	log.Printf("✅ Подключение к Database B установлено")

	// Подключение к NATS
	nc, err := nats.Connect(natsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		log.Fatalf("❌ Ошибка подключения к NATS: %v", err)
	}
	defer nc.Close()

	log.Printf("✅ Подключение к NATS B установлено (%s)", natsURL)

	// Создаем JetStream контекст
	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("❌ Ошибка создания JetStream: %v", err)
	}

	// Получаем или создаем consumer
	consumer, err := js.CreateOrUpdateConsumer(context.Background(), streamName, jetstream.ConsumerConfig{
		Name:          consumerName,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		MaxDeliver:    3, // Максимум 3 попытки доставки
		AckWait:       30 * time.Second,
	})
	if err != nil {
		log.Fatalf("❌ Ошибка создания consumer: %v", err)
	}

	log.Printf("✅ Consumer создан: %s", consumer.CachedInfo().Name)
	log.Printf("📦 Размер пакета: %d событий", batchSize)
	log.Println("🔄 Начинаем обработку входящих событий...")

	// Основной цикл обработки сообщений
	for {
		// Получаем пакет сообщений
		messages, err := consumer.Fetch(batchSize, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if err == nats.ErrTimeout {
				continue // Таймаут - это нормально, продолжаем
			}
			log.Printf("❌ Ошибка получения сообщений: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Получаем срез сообщений из MessageBatch
		var msgList []jetstream.Msg
		for msg := range messages.Messages() {
			msgList = append(msgList, msg)
		}

		if len(msgList) == 0 {
			continue
		}

		log.Printf("📥 Получено сообщений: %d", len(msgList))

		// Обрабатываем пакет сообщений
		processMessageBatch(db, msgList)
	}
}

func processMessageBatch(db *sql.DB, messages []jetstream.Msg) {
	// Сортируем сообщения: сначала users, потом payments
	var userMessages []jetstream.Msg
	var paymentMessages []jetstream.Msg

	for _, msg := range messages {
		var replicationMsg ReplicationMessage
		if err := json.Unmarshal(msg.Data(), &replicationMsg); err != nil {
			log.Printf("❌ Ошибка парсинга для сортировки: %v", err)
			continue
		}

		if replicationMsg.AggregateType == "user" {
			userMessages = append(userMessages, msg)
		} else if replicationMsg.AggregateType == "payment" {
			paymentMessages = append(paymentMessages, msg)
		}
	}

	// Обрабатываем сначала пользователей, потом платежи
	allMessages := append(userMessages, paymentMessages...)

	for _, msg := range allMessages {
		if processMessage(db, msg) {
			// Подтверждаем успешную обработку
			if err := msg.Ack(); err != nil {
				log.Printf("❌ Ошибка подтверждения сообщения: %v", err)
			}
		} else {
			// Отклоняем сообщение (оно будет переотправлено)
			if err := msg.Nak(); err != nil {
				log.Printf("❌ Ошибка отклонения сообщения: %v", err)
			}
		}
	}
}

func processMessage(db *sql.DB, msg jetstream.Msg) bool {
	// Парсим сообщение
	var replicationMsg ReplicationMessage
	err := json.Unmarshal(msg.Data(), &replicationMsg)
	if err != nil {
		log.Printf("❌ Ошибка парсинга сообщения: %v", err)
		return false // Некорректное сообщение, не переотправляем
	}

	// Проверяем, не обработано ли уже это событие
	processed, err := isEventProcessed(db, replicationMsg.EventID)
	if err != nil {
		log.Printf("❌ Ошибка проверки дубликата %s: %v", replicationMsg.EventID, err)
		return false // Ошибка БД, попробуем позже
	}

	if processed {
		log.Printf("⚠️ Событие %s уже обработано, пропускаем", replicationMsg.EventID)
		return true // Дубликат, подтверждаем обработку
	}

	// Начинаем транзакцию
	tx, err := db.Begin()
	if err != nil {
		log.Printf("❌ Ошибка начала транзакции: %v", err)
		return false
	}
	defer tx.Rollback() // Откатываем в случае ошибки

	// Сохраняем событие в inbox
	inboxEvent := InboxEvent{
		ID:            uuid.New(),
		EventID:       replicationMsg.EventID,
		AggregateID:   replicationMsg.AggregateID,
		AggregateType: replicationMsg.AggregateType,
		EventType:     replicationMsg.EventType,
		EventData:     replicationMsg.EventData,
		CreatedAt:     time.Now(),
	}

	err = saveToInbox(tx, inboxEvent)
	if err != nil {
		log.Printf("❌ Ошибка сохранения в inbox %s: %v", replicationMsg.EventID, err)
		return false
	}

	// Обрабатываем событие в зависимости от типа
	var processResult bool
	if replicationMsg.AggregateType == "user" {
		processResult, err = processUserEvent(tx, replicationMsg.EventID, replicationMsg.EventType, replicationMsg.EventData)
	} else if replicationMsg.AggregateType == "payment" {
		processResult, err = processPaymentEvent(tx, replicationMsg.EventID, replicationMsg.EventType, replicationMsg.EventData)
	} else {
		log.Printf("❌ Неизвестный тип агрегата: %s", replicationMsg.AggregateType)
		return false
	}

	if err != nil {
		log.Printf("❌ Ошибка обработки события %s: %v", replicationMsg.EventID, err)
		return false
	}

	if !processResult {
		log.Printf("⚠️ Событие %s уже было обработано ранее", replicationMsg.EventID)
	}

	// Отмечаем событие inbox как обработанное
	err = markInboxProcessed(tx, []uuid.UUID{inboxEvent.ID})
	if err != nil {
		log.Printf("❌ Ошибка отметки inbox как обработанного %s: %v", replicationMsg.EventID, err)
		return false
	}

	// Коммитим транзакцию
	err = tx.Commit()
	if err != nil {
		log.Printf("❌ Ошибка коммита транзакции %s: %v", replicationMsg.EventID, err)
		return false
	}

	log.Printf("✅ Событие обработано: %s/%s (%s)",
		replicationMsg.AggregateType, replicationMsg.EventType, replicationMsg.EventID)

	return true
}

func isEventProcessed(db *sql.DB, eventID uuid.UUID) (bool, error) {
	query := "SELECT is_event_processed($1)"
	var processed bool
	err := db.QueryRow(query, eventID).Scan(&processed)
	return processed, err
}

func saveToInbox(tx *sql.Tx, event InboxEvent) error {
	query := `
		INSERT INTO inbox (id, event_id, aggregate_id, aggregate_type, event_type, event_data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := tx.Exec(query, event.ID, event.EventID, event.AggregateID,
		event.AggregateType, event.EventType, event.EventData, event.CreatedAt)
	return err
}

func processUserEvent(tx *sql.Tx, eventID uuid.UUID, eventType string, eventData json.RawMessage) (bool, error) {
	query := "SELECT process_user_event($1, $2, $3)"
	var processed bool
	err := tx.QueryRow(query, eventID, eventType, eventData).Scan(&processed)
	return processed, err
}

func processPaymentEvent(tx *sql.Tx, eventID uuid.UUID, eventType string, eventData json.RawMessage) (bool, error) {
	query := "SELECT process_payment_event($1, $2, $3)"
	var processed bool
	err := tx.QueryRow(query, eventID, eventType, eventData).Scan(&processed)
	return processed, err
}

func markInboxProcessed(tx *sql.Tx, eventIDs []uuid.UUID) error {
	if len(eventIDs) == 0 {
		return nil
	}

	// Используем простой UPDATE запрос вместо массива
	query := `
		UPDATE inbox 
		SET processed = true, processed_at = NOW()
		WHERE id = $1 AND processed = false`

	for _, id := range eventIDs {
		_, err := tx.Exec(query, id)
		if err != nil {
			return err
		}
	}

	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
