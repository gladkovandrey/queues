package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// Структура ERROR лога (из mapper)
type ErrorLog struct {
	Timestamp   string `json:"timestamp"`
	Service     string `json:"service"`
	Error       string `json:"error"`
	ProcessedAt string `json:"processed_at"`
}

// Структура статистики ошибок
type ErrorStats struct {
	WindowStart string            `json:"window_start"`
	WindowEnd   string            `json:"window_end"`
	Services    map[string]int    `json:"services"`
	TotalErrors int               `json:"total_errors"`
	GeneratedAt string            `json:"generated_at"`
}

func main() {
	// Получаем настройки
	servers := getEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:19092")
	inputTopic := getEnvOrDefault("INPUT_TOPIC", "error-logs")
	outputTopic := getEnvOrDefault("OUTPUT_TOPIC", "error-stats")
	consumerGroup := getEnvOrDefault("CONSUMER_GROUP", "error-aggregator")
	windowSeconds := 60 // 1-минутные окна

	log.Printf("📊 Aggregator запущен")
	log.Printf("📥 Читаем из: %s", inputTopic)
	log.Printf("📤 Записываем в: %s", outputTopic)
	log.Printf("⏰ Окно агрегации: %d секунд", windowSeconds)

	brokers := strings.Split(servers, ",")

	// Создаем reader для чтения ERROR логов
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   inputTopic,
		GroupID: consumerGroup,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})
	defer reader.Close()

	// Создаем writer для записи статистики
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: brokers,
		Topic:   outputTopic,
		Balancer: &kafka.LeastBytes{},
	})
	defer writer.Close()

	log.Printf("✅ Подключение к Kafka установлено")

	// Счетчики для агрегации
	errorCounts := make(map[string]int)
	windowStart := time.Now()

	// Ticker для отправки статистики каждую минуту
	ticker := time.NewTicker(time.Duration(windowSeconds) * time.Second)
	defer ticker.Stop()

	// Канал для чтения сообщений
	messageChan := make(chan kafka.Message, 100)

	// Горутина для чтения сообщений
	go func() {
		for {
			message, err := reader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("❌ Ошибка чтения: %v", err)
				continue
			}
			messageChan <- message
		}
	}()

	// Основной цикл агрегации
	for {
		select {
		case message := <-messageChan:
			// Парсим ERROR лог
			var errorLog ErrorLog
			err := json.Unmarshal(message.Value, &errorLog)
			if err != nil {
				log.Printf("❌ Ошибка JSON: %v", err)
				continue
			}

			// Увеличиваем счетчик для сервиса
			errorCounts[errorLog.Service]++
			log.Printf("📈 %s: %d ошибок", errorLog.Service, errorCounts[errorLog.Service])

		case <-ticker.C:
			// Время отправить статистику
			if len(errorCounts) > 0 {
				windowEnd := time.Now()
				totalErrors := 0
				for _, count := range errorCounts {
					totalErrors += count
				}

				stats := ErrorStats{
					WindowStart: windowStart.Format("2006-01-02 15:04:05"),
					WindowEnd:   windowEnd.Format("2006-01-02 15:04:05"),
					Services:    copyMap(errorCounts),
					TotalErrors: totalErrors,
					GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
				}

				// Сериализуем статистику
				statsBytes, err := json.Marshal(stats)
				if err != nil {
					log.Printf("❌ Ошибка сериализации: %v", err)
					continue
				}

				// Записываем статистику
				err = writer.WriteMessages(context.Background(), kafka.Message{
					Key:   []byte("error-stats"),
					Value: statsBytes,
				})

				if err != nil {
					log.Printf("❌ Ошибка записи: %v", err)
				} else {
					log.Printf("📊 Отправлена статистика: %d ошибок за окно", totalErrors)
					for service, count := range errorCounts {
						log.Printf("   %s: %d ошибок", service, count)
					}
				}

				// Сбрасываем счетчики для нового окна
				errorCounts = make(map[string]int)
				windowStart = windowEnd
			}
		}
	}
}

func copyMap(original map[string]int) map[string]int {
	copy := make(map[string]int)
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
} 