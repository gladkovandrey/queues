package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// Простая структура лога
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

	// Создаем подключение к Kafka
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: brokers,
		Topic:   topic,
	})

	log.Printf("Продюсер запущен, отправляем в топик: %s", topic)

	// Бесконечный цикл отправки сообщений
	for {
		// Создаем простое сообщение
		message := LogMessage{
			Timestamp: time.Now().Format("2006-01-02 15:04:05"),
			Level:     getRandomLevel(),
			Service:   getRandomService(),
			Message:   getRandomMessage(),
		}

		// Превращаем в JSON
		messageBytes, err := json.Marshal(message)
		if err != nil {
			log.Printf("Ошибка JSON: %v", err)
			continue
		}

		// Отправляем в Kafka
		err = writer.WriteMessages(context.Background(), kafka.Message{
			Value: messageBytes,
		})
		
		if err != nil {
			log.Printf("Ошибка отправки: %v", err)
		} else {
			log.Printf("Отправлено: %s", message.Message)
		}

		// Ждем секунду
		time.Sleep(1 * time.Second)
	}
}

func getRandomLevel() string {
	levels := []string{"INFO", "WARN", "ERROR"}
	return levels[rand.Intn(len(levels))]
}

func getRandomService() string {
	services := []string{"user-service", "order-service", "payment-service"}
	return services[rand.Intn(len(services))]
}

func getRandomMessage() string {
	messages := []string{
		"Пользователь вошел в систему",
		"Заказ создан успешно", 
		"Платеж обработан",
		"Ошибка подключения к базе",
		"Сервис запущен",
	}
	return messages[rand.Intn(len(messages))]
}
