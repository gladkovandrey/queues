package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/segmentio/kafka-go"
)

// Простая структура лога (такая же как в продюсере)
type LogMessage struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Service   string `json:"service"`
	Message   string `json:"message"`
}

func main() {
	// Получаем настройки
	servers := os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	if servers == "" {
		servers = "localhost:19092"
	}

	// Добавляем отладочную информацию
	log.Printf("Исходная строка серверов: '%s'", servers)
	brokers := strings.Split(servers, ",")
	log.Printf("Массив серверов: %v (длина: %d)", brokers, len(brokers))

	topic := os.Getenv("KAFKA_TOPIC")
	if topic == "" {
		topic = "application-logs"
	}

	groupID := os.Getenv("CONSUMER_GROUP")
	if groupID == "" {
		groupID = "log-processors"
	}

	// Создаем подключение к Kafka
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: groupID,
	})

	log.Printf("Консьюмер запущен, читаем из топика: %s", topic)

	// Бесконечный цикл чтения сообщений
	for {
		// Читаем сообщение
		message, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("Ошибка чтения: %v", err)
			continue
		}

		// Превращаем JSON обратно в структуру
		var logMsg LogMessage
		err = json.Unmarshal(message.Value, &logMsg)
		if err != nil {
			log.Printf("Ошибка JSON: %v", err)
			continue
		}

		// Выводим сообщение с цветом в зависимости от уровня
		switch logMsg.Level {
		case "ERROR":
			log.Printf("\033[91m[%s] %s: %s\033[0m", logMsg.Timestamp, logMsg.Service, logMsg.Message)
		case "WARN":
			log.Printf("\033[93m[%s] %s: %s\033[0m", logMsg.Timestamp, logMsg.Service, logMsg.Message)
		case "INFO":
			log.Printf("\033[92m[%s] %s: %s\033[0m", logMsg.Timestamp, logMsg.Service, logMsg.Message)
		default:
			log.Printf("[%s] %s: %s", logMsg.Timestamp, logMsg.Service, logMsg.Message)
		}
	}
}
