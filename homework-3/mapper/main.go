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

// Исходная структура лога (из основного producer)
type LogMessage struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Service   string `json:"service"`
	Message   string `json:"message"`
}

// Упрощенная структура для ERROR логов
type ErrorLog struct {
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Error     string `json:"error"`
	ProcessedAt string `json:"processed_at"`
}

func main() {
	// Получаем настройки
	servers := getEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:19092")
	inputTopic := getEnvOrDefault("INPUT_TOPIC", "application-logs")
	outputTopic := getEnvOrDefault("OUTPUT_TOPIC", "error-logs")
	consumerGroup := getEnvOrDefault("CONSUMER_GROUP", "error-mapper")

	log.Printf("🔄 Mapper запущен")
	log.Printf("📥 Читаем из: %s", inputTopic)
	log.Printf("📤 Записываем в: %s", outputTopic)

	brokers := strings.Split(servers, ",")

	// Создаем reader для чтения исходных логов
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   inputTopic,
		GroupID: consumerGroup,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})
	defer reader.Close()

	// Создаем writer для записи ERROR логов  
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: brokers,
		Topic:   outputTopic,
		Balancer: &kafka.LeastBytes{},
	})
	defer writer.Close()

	log.Printf("✅ Подключение к Kafka установлено")

	// Основной цикл обработки
	for {
		// Читаем сообщение
		message, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("❌ Ошибка чтения: %v", err)
			continue
		}

		// Парсим исходный лог
		var logMsg LogMessage
		err = json.Unmarshal(message.Value, &logMsg)
		if err != nil {
			log.Printf("❌ Ошибка JSON: %v", err)
			continue
		}

		// Фильтруем только ERROR логи
		if logMsg.Level != "ERROR" {
			continue // Пропускаем не-ERROR логи
		}

		// Преобразуем в упрощенный формат
		errorLog := ErrorLog{
			Timestamp:   logMsg.Timestamp,
			Service:     logMsg.Service,
			Error:       logMsg.Message,
			ProcessedAt: time.Now().Format("2006-01-02 15:04:05"),
		}

		// Сериализуем в JSON
		errorBytes, err := json.Marshal(errorLog)
		if err != nil {
			log.Printf("❌ Ошибка сериализации: %v", err)
			continue
		}

		// Записываем в выходной топик
		err = writer.WriteMessages(context.Background(), kafka.Message{
			Key:   []byte(errorLog.Service), // Ключ по сервису для партицирования
			Value: errorBytes,
		})

		if err != nil {
			log.Printf("❌ Ошибка записи: %v", err)
		} else {
			log.Printf("🔄 Обработан ERROR: %s - %s", errorLog.Service, errorLog.Error)
		}
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
} 