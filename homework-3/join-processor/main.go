package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
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

// Структура метрик сервиса (из metrics-producer)
type ServiceMetrics struct {
	Timestamp    string  `json:"timestamp"`
	Service      string  `json:"service"`
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryUsage  float64 `json:"memory_usage"`
	LatencyMs    int     `json:"latency_ms"`
	RequestCount int     `json:"request_count"`
	GeneratedAt  string  `json:"generated_at"`
}

// Обогащенная структура - ошибка + метрики
type EnrichedError struct {
	ErrorLog
	Metrics     *ServiceMetrics `json:"metrics,omitempty"`
	JoinedAt    string          `json:"joined_at"`
	MetricsAge  string          `json:"metrics_age,omitempty"` // Возраст метрик
}

// Кэш метрик для join операций
type MetricsCache struct {
	mu      sync.RWMutex
	metrics map[string]*ServiceMetrics // ключ: service
}

func (mc *MetricsCache) Set(service string, metrics *ServiceMetrics) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics[service] = metrics
}

func (mc *MetricsCache) Get(service string) *ServiceMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.metrics[service]
}

func main() {
	// Получаем настройки
	servers := getEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:19092")
	errorTopic := getEnvOrDefault("ERROR_TOPIC", "error-logs")
	metricsTopic := getEnvOrDefault("METRICS_TOPIC", "service-metrics")
	outputTopic := getEnvOrDefault("OUTPUT_TOPIC", "enriched-errors")
	consumerGroup := getEnvOrDefault("CONSUMER_GROUP", "join-processor")

	log.Printf("🔗 Join Processor запущен")
	log.Printf("📥 ERROR логи из: %s", errorTopic)
	log.Printf("📥 Метрики из: %s", metricsTopic)
	log.Printf("📤 Записываем в: %s", outputTopic)

	brokers := strings.Split(servers, ",")

	// Создаем кэш метрик
	cache := &MetricsCache{
		metrics: make(map[string]*ServiceMetrics),
	}

	// Создаем readers
	errorReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   errorTopic,
		GroupID: consumerGroup + "-errors",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})
	defer errorReader.Close()

	metricsReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   metricsTopic,
		GroupID: consumerGroup + "-metrics",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})
	defer metricsReader.Close()

	// Создаем writer для обогащенных данных
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: brokers,
		Topic:   outputTopic,
		Balancer: &kafka.LeastBytes{},
	})
	defer writer.Close()

	log.Printf("✅ Подключение к Kafka установлено")

	// Каналы для обработки сообщений
	errorChan := make(chan kafka.Message, 100)
	metricsChan := make(chan kafka.Message, 100)

	// Горутина для чтения ERROR логов
	go func() {
		for {
			message, err := errorReader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("❌ Ошибка чтения error: %v", err)
				continue
			}
			errorChan <- message
		}
	}()

	// Горутина для чтения метрик
	go func() {
		for {
			message, err := metricsReader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("❌ Ошибка чтения metrics: %v", err)
				continue
			}
			metricsChan <- message
		}
	}()

	// Основной цикл join обработки
	for {
		select {
		case metricsMsg := <-metricsChan:
			// Обновляем кэш метрик
			var metrics ServiceMetrics
			err := json.Unmarshal(metricsMsg.Value, &metrics)
			if err != nil {
				log.Printf("❌ Ошибка JSON metrics: %v", err)
				continue
			}

			cache.Set(metrics.Service, &metrics)
			log.Printf("📊 Обновлены метрики для %s: CPU=%.1f%% LAT=%dms",
				metrics.Service, metrics.CPUUsage, metrics.LatencyMs)

		case errorMsg := <-errorChan:
			// Обрабатываем ERROR лог
			var errorLog ErrorLog
			err := json.Unmarshal(errorMsg.Value, &errorLog)
			if err != nil {
				log.Printf("❌ Ошибка JSON error: %v", err)
				continue
			}

			// Получаем метрики для данного сервиса
			metrics := cache.Get(errorLog.Service)

			// Создаем обогащенную запись
			enriched := EnrichedError{
				ErrorLog: errorLog,
				Metrics:  metrics,
				JoinedAt: time.Now().Format("2006-01-02 15:04:05"),
			}

			// Если нашли метрики, вычисляем их возраст
			if metrics != nil {
				metricsTime, err := time.Parse("2006-01-02 15:04:05", metrics.Timestamp)
				if err == nil {
					age := time.Since(metricsTime)
					enriched.MetricsAge = age.String()
				}
			}

			// Сериализуем обогащенные данные
			enrichedBytes, err := json.Marshal(enriched)
			if err != nil {
				log.Printf("❌ Ошибка сериализации: %v", err)
				continue
			}

			// Записываем обогащенные данные
			err = writer.WriteMessages(context.Background(), kafka.Message{
				Key:   []byte(errorLog.Service),
				Value: enrichedBytes,
			})

			if err != nil {
				log.Printf("❌ Ошибка записи: %v", err)
			} else {
				if metrics != nil {
					log.Printf("🔗 JOIN: %s ошибка + метрики (CPU=%.1f%%, LAT=%dms)",
						errorLog.Service, metrics.CPUUsage, metrics.LatencyMs)
				} else {
					log.Printf("🔗 JOIN: %s ошибка БЕЗ метрик", errorLog.Service)
				}
			}
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