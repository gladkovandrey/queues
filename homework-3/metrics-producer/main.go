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

// Структура метрик сервиса
type ServiceMetrics struct {
	Timestamp    string  `json:"timestamp"`
	Service      string  `json:"service"`
	CPUUsage     float64 `json:"cpu_usage"`     // процент
	MemoryUsage  float64 `json:"memory_usage"`  // процент
	LatencyMs    int     `json:"latency_ms"`    // миллисекунды
	RequestCount int     `json:"request_count"` // запросов в секунду
	GeneratedAt  string  `json:"generated_at"`
}

func main() {
	// Получаем настройки
	servers := getEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:19092")
	topic := getEnvOrDefault("KAFKA_TOPIC", "service-metrics")
	intervalSeconds := 10 // Генерируем метрики каждые 10 секунд

	log.Printf("📊 Metrics Producer запущен")
	log.Printf("📤 Записываем в: %s", topic)
	log.Printf("⏰ Интервал: %d секунд", intervalSeconds)

	brokers := strings.Split(servers, ",")

	// Создаем writer для записи метрик
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  brokers,
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	})
	defer writer.Close()

	log.Printf("✅ Подключение к Kafka установлено")

	services := []string{"user-service", "order-service", "payment-service"}

	// Бесконечный цикл генерации метрик
	for {
		for _, service := range services {
			metrics := ServiceMetrics{
				Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
				Service:      service,
				CPUUsage:     generateCPUUsage(service),
				MemoryUsage:  generateMemoryUsage(service),
				LatencyMs:    generateLatency(service),
				RequestCount: generateRequestCount(service),
				GeneratedAt:  time.Now().Format("2006-01-02 15:04:05"),
			}

			// Сериализуем метрики
			metricsBytes, err := json.Marshal(metrics)
			if err != nil {
				log.Printf("❌ Ошибка сериализации: %v", err)
				continue
			}

			// Записываем метрики
			err = writer.WriteMessages(context.Background(), kafka.Message{
				Key:   []byte(service), // Ключ по сервису
				Value: metricsBytes,
			})

			if err != nil {
				log.Printf("❌ Ошибка записи: %v", err)
			} else {
				log.Printf("📊 %s: CPU=%.1f%% MEM=%.1f%% LAT=%dms REQ=%d/s",
					service, metrics.CPUUsage, metrics.MemoryUsage,
					metrics.LatencyMs, metrics.RequestCount)
			}
		}

		time.Sleep(time.Duration(intervalSeconds) * time.Second)
	}
}

// Генерируем CPU usage с разным поведением для разных сервисов
func generateCPUUsage(service string) float64 {
	base := map[string]float64{
		"user-service":    30.0,
		"order-service":   45.0,
		"payment-service": 25.0,
	}
	
	// Добавляем случайный шум ±20%
	noise := (rand.Float64() - 0.5) * 40
	usage := base[service] + noise
	
	// Ограничиваем 0-100%
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	
	return usage
}

// Генерируем Memory usage
func generateMemoryUsage(service string) float64 {
	base := map[string]float64{
		"user-service":    40.0,
		"order-service":   60.0,
		"payment-service": 35.0,
	}
	
	noise := (rand.Float64() - 0.5) * 30
	usage := base[service] + noise
	
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	
	return usage
}

// Генерируем Latency в миллисекундах
func generateLatency(service string) int {
	base := map[string]int{
		"user-service":    50,
		"order-service":   120,
		"payment-service": 200,
	}
	
	// Добавляем случайный шум ±50%
	noise := int((rand.Float64() - 0.5) * float64(base[service]))
	latency := base[service] + noise
	
	if latency < 1 {
		latency = 1
	}
	
	return latency
}

// Генерируем количество запросов в секунду
func generateRequestCount(service string) int {
	base := map[string]int{
		"user-service":    100,
		"order-service":   50,
		"payment-service": 30,
	}
	
	noise := int((rand.Float64() - 0.5) * float64(base[service]) * 0.8)
	count := base[service] + noise
	
	if count < 0 {
		count = 0
	}
	
	return count
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
} 