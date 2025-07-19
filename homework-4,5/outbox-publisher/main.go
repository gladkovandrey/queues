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
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	_ "github.com/lib/pq"
)

type OutboxEvent struct {
	ID            uuid.UUID       `json:"id"`
	AggregateID   uuid.UUID       `json:"aggregate_id"`
	AggregateType string          `json:"aggregate_type"`
	EventType     string          `json:"event_type"`
	EventData     json.RawMessage `json:"event_data"`
	CreatedAt     time.Time       `json:"created_at"`
}

type ReplicationMessage struct {
	EventID       uuid.UUID       `json:"event_id"`
	AggregateID   uuid.UUID       `json:"aggregate_id"`
	AggregateType string          `json:"aggregate_type"`
	EventType     string          `json:"event_type"`
	EventData     json.RawMessage `json:"event_data"`
	OriginalTime  time.Time       `json:"original_time"`
	PublishedAt   time.Time       `json:"published_at"`
}

func main() {
	log.Println("📤 Outbox Publisher запускается...")

	// Получаем настройки из environment
	dbHost := getEnvOrDefault("DB_HOST", "localhost")
	dbPort := getEnvOrDefault("DB_PORT", "5432")
	dbUser := getEnvOrDefault("DB_USER", "postgres")
	dbPassword := getEnvOrDefault("DB_PASSWORD", "password")
	dbName := getEnvOrDefault("DB_NAME", "transactions")

	natsURL := getEnvOrDefault("NATS_URL", "nats://localhost:4222")
	streamName := getEnvOrDefault("STREAM_NAME", "REPLICATION")
	subject := getEnvOrDefault("SUBJECT", "replication.events")

	pollIntervalStr := getEnvOrDefault("POLL_INTERVAL", "1")
	pollInterval, err := strconv.Atoi(pollIntervalStr)
	if err != nil {
		pollInterval = 1
	}

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

	log.Printf("✅ Подключение к Database A установлено")

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

	log.Printf("✅ Подключение к NATS A установлено (%s)", natsURL)

	// Создаем JetStream контекст
	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("❌ Ошибка создания JetStream: %v", err)
	}

	// Создаем или получаем stream
	stream, err := js.CreateOrUpdateStream(context.Background(), jetstream.StreamConfig{
		Name:        streamName,
		Subjects:    []string{subject},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      24 * time.Hour, // Храним события 24 часа
		MaxMsgs:     100000,         // Максимум 100k сообщений
		Duplicates:  5 * time.Minute, // Защита от дубликатов 5 минут
	})
	if err != nil {
		log.Fatalf("❌ Ошибка создания stream: %v", err)
	}

	log.Printf("✅ JetStream создан: %s", stream.CachedInfo().Config.Name)
	log.Printf("⏱️ Интервал опроса: %d секунд", pollInterval)
	log.Printf("📦 Размер пакета: %d событий", batchSize)
	log.Println("🔄 Начинаем обработку outbox...")

	// Основной цикл обработки outbox
	ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			processOutboxBatch(db, js, subject, batchSize)
		}
	}
}

func processOutboxBatch(db *sql.DB, js jetstream.JetStream, subject string, batchSize int) {
	// Получаем необработанные события из outbox
	events, err := getUnprocessedEvents(db, batchSize)
	if err != nil {
		log.Printf("❌ Ошибка чтения outbox: %v", err)
		return
	}

	if len(events) == 0 {
		return // Нет новых событий
	}

	log.Printf("📥 Найдено событий для обработки: %d", len(events))

	// Обрабатываем события
	var processedIDs []uuid.UUID

	for _, event := range events {
		message := ReplicationMessage{
			EventID:       event.ID,
			AggregateID:   event.AggregateID,
			AggregateType: event.AggregateType,
			EventType:     event.EventType,
			EventData:     event.EventData,
			OriginalTime:  event.CreatedAt,
			PublishedAt:   time.Now(),
		}

		messageBytes, err := json.Marshal(message)
		if err != nil {
			log.Printf("❌ Ошибка сериализации события %s: %v", event.ID, err)
			continue
		}

		// Отправляем в NATS с уникальным ID для дедупликации
		_, err = js.Publish(context.Background(), subject, messageBytes,
			jetstream.WithMsgID(event.ID.String()),
		)
		if err != nil {
			log.Printf("❌ Ошибка отправки события %s в NATS: %v", event.ID, err)
			continue
		}

		processedIDs = append(processedIDs, event.ID)
		log.Printf("✅ Событие отправлено: %s/%s (%s)", 
			event.AggregateType, event.EventType, event.ID)
	}

	// Отмечаем события как обработанные
	if len(processedIDs) > 0 {
		err = markEventsProcessed(db, processedIDs)
		if err != nil {
			log.Printf("❌ Ошибка отметки событий как обработанных: %v", err)
		} else {
			log.Printf("✅ Отмечено как обработанных: %d событий", len(processedIDs))
		}
	}
}

func getUnprocessedEvents(db *sql.DB, limit int) ([]OutboxEvent, error) {
	query := `
		SELECT id, aggregate_id, aggregate_type, event_type, event_data, created_at
		FROM outbox 
		WHERE processed = false 
		ORDER BY created_at ASC 
		LIMIT $1`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []OutboxEvent

	for rows.Next() {
		var event OutboxEvent
		err := rows.Scan(
			&event.ID,
			&event.AggregateID,
			&event.AggregateType,
			&event.EventType,
			&event.EventData,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

func markEventsProcessed(db *sql.DB, eventIDs []uuid.UUID) error {
	if len(eventIDs) == 0 {
		return nil
	}

	// Используем цикл для обновления каждого события отдельно
	query := `
		UPDATE outbox 
		SET processed = true, processed_at = NOW()
		WHERE id = $1 AND processed = false`

	for _, id := range eventIDs {
		_, err := db.Exec(query, id)
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