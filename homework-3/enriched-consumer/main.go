package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/segmentio/kafka-go"
)

// Структура ERROR лога
type ErrorLog struct {
	Timestamp   string `json:"timestamp"`
	Service     string `json:"service"`
	Error       string `json:"error"`
	ProcessedAt string `json:"processed_at"`
}

// Структура метрик сервиса
type ServiceMetrics struct {
	Timestamp    string  `json:"timestamp"`
	Service      string  `json:"service"`
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryUsage  float64 `json:"memory_usage"`
	LatencyMs    int     `json:"latency_ms"`
	RequestCount int     `json:"request_count"`
	GeneratedAt  string  `json:"generated_at"`
}

// Обогащенная структура (из join-processor)
type EnrichedError struct {
	ErrorLog
	Metrics    *ServiceMetrics `json:"metrics,omitempty"`
	JoinedAt   string          `json:"joined_at"`
	MetricsAge string          `json:"metrics_age,omitempty"`
}

func main() {
	// Получаем настройки
	servers := getEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:19092")
	topic := getEnvOrDefault("KAFKA_TOPIC", "enriched-errors")
	consumerGroup := getEnvOrDefault("CONSUMER_GROUP", "enriched-display")

	log.Printf("🔗 Enriched Consumer запущен")
	log.Printf("📥 Читаем из: %s", topic)

	brokers := strings.Split(servers, ",")

	// Создаем reader для чтения обогащенных ошибок
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  consumerGroup,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	log.Printf("✅ Подключение к Kafka установлено")
	log.Printf("\n🔍 Ожидаем обогащенные ошибки...\n")

	// Бесконечный цикл чтения обогащенных ошибок
	for {
		message, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("❌ Ошибка чтения: %v", err)
			continue
		}

		// Парсим обогащенную ошибку
		var enriched EnrichedError
		err = json.Unmarshal(message.Value, &enriched)
		if err != nil {
			log.Printf("❌ Ошибка JSON: %v", err)
			continue
		}

		// Красиво отображаем обогащенную ошибку
		displayEnrichedError(&enriched)
	}
}

func displayEnrichedError(enriched *EnrichedError) {
	fmt.Printf("\n" + strings.Repeat("=", 70) + "\n")

	// Определяем цвет и иконку для сервиса
	var serviceIcon, serviceColor string
	switch enriched.Service {
	case "user-service":
		serviceIcon = "👤"
		serviceColor = "\033[96m" // Cyan
	case "order-service":
		serviceIcon = "📦"
		serviceColor = "\033[93m" // Yellow
	case "payment-service":
		serviceIcon = "💳"
		serviceColor = "\033[95m" // Magenta
	default:
		serviceIcon = "⚙️"
		serviceColor = "\033[97m" // White
	}

	fmt.Printf("🚨 ОБОГАЩЕННАЯ ОШИБКА\n")
	fmt.Printf("%s%s %-20s\033[0m 🕐 %s\n", serviceColor, serviceIcon, enriched.Service, enriched.Timestamp)
	fmt.Printf("💥 Ошибка: \033[91m%s\033[0m\n", enriched.Error)
	fmt.Printf("⏰ Обработано: %s\n", enriched.ProcessedAt)
	fmt.Printf("🔗 Объединено: %s\n", enriched.JoinedAt)

	if enriched.Metrics != nil {
		fmt.Printf(strings.Repeat("-", 70) + "\n")
		fmt.Printf("📊 МЕТРИКИ ПРОИЗВОДИТЕЛЬНОСТИ:\n")

		// CPU индикатор
		cpuBar := generateMetricBar(enriched.Metrics.CPUUsage, 100)
		cpuColor := getColorByLevel(enriched.Metrics.CPUUsage, 60, 85)
		fmt.Printf("   🖥️  CPU:       %s%5.1f%%\033[0m %s\n", cpuColor, enriched.Metrics.CPUUsage, cpuBar)

		// Memory индикатор
		memBar := generateMetricBar(enriched.Metrics.MemoryUsage, 100)
		memColor := getColorByLevel(enriched.Metrics.MemoryUsage, 70, 90)
		fmt.Printf("   🧠 Memory:    %s%5.1f%%\033[0m %s\n", memColor, enriched.Metrics.MemoryUsage, memBar)

		// Latency
		latencyColor := getColorByLatency(enriched.Metrics.LatencyMs)
		fmt.Printf("   ⚡ Latency:   %s%4dms\033[0m\n", latencyColor, enriched.Metrics.LatencyMs)

		// Request count
		fmt.Printf("   📈 Requests:  %4d/sec\n", enriched.Metrics.RequestCount)
		fmt.Printf("   📅 Время метрик: %s", enriched.Metrics.Timestamp)

		if enriched.MetricsAge != "" {
			fmt.Printf(" (возраст: %s)", enriched.MetricsAge)
		}
		fmt.Printf("\n")

		// Анализ ситуации
		fmt.Printf(strings.Repeat("-", 70) + "\n")
		fmt.Printf("🔍 АНАЛИЗ:\n")
		analyzeErrorContext(enriched)

	} else {
		fmt.Printf(strings.Repeat("-", 70) + "\n")
		fmt.Printf("❌ Метрики недоступны - JOIN не выполнен\n")
	}

	fmt.Printf(strings.Repeat("=", 70) + "\n\n")
}

func generateMetricBar(value, max float64) string {
	barLength := 15
	filled := int(value / max * float64(barLength))

	bar := "["
	for i := 0; i < barLength; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"

	return bar
}

func getColorByLevel(value, yellowThreshold, redThreshold float64) string {
	if value >= redThreshold {
		return "\033[91m" // Red
	} else if value >= yellowThreshold {
		return "\033[93m" // Yellow
	}
	return "\033[92m" // Green
}

func getColorByLatency(latency int) string {
	if latency >= 150 {
		return "\033[91m" // Red
	} else if latency >= 100 {
		return "\033[93m" // Yellow
	}
	return "\033[92m" // Green
}

func analyzeErrorContext(enriched *EnrichedError) {
	if enriched.Metrics == nil {
		return
	}

	var issues []string
	var suggestions []string

	// Анализ CPU
	if enriched.Metrics.CPUUsage > 85 {
		issues = append(issues, "🔥 Критическая загрузка CPU")
		suggestions = append(suggestions, "   • Проверить процессы и оптимизировать алгоритмы")
	} else if enriched.Metrics.CPUUsage > 60 {
		issues = append(issues, "⚠️  Высокая загрузка CPU")
	}

	// Анализ Memory
	if enriched.Metrics.MemoryUsage > 90 {
		issues = append(issues, "💾 Критическое потребление памяти")
		suggestions = append(suggestions, "   • Проверить утечки памяти и увеличить лимиты")
	} else if enriched.Metrics.MemoryUsage > 70 {
		issues = append(issues, "⚠️  Высокое потребление памяти")
	}

	// Анализ Latency
	if enriched.Metrics.LatencyMs > 150 {
		issues = append(issues, "🐌 Критическая задержка")
		suggestions = append(suggestions, "   • Оптимизировать запросы к БД и внешним сервисам")
	} else if enriched.Metrics.LatencyMs > 100 {
		issues = append(issues, "⚠️  Повышенная задержка")
	}

	// Анализ нагрузки
	if enriched.Metrics.RequestCount < 10 {
		issues = append(issues, "📉 Низкая нагрузка - возможны проблемы с доступностью")
	}

	if len(issues) == 0 {
		fmt.Printf("   ✅ Метрики в норме\n")
	} else {
		for _, issue := range issues {
			fmt.Printf("   %s\n", issue)
		}
	}

	if len(suggestions) > 0 {
		fmt.Printf("🛠️  РЕКОМЕНДАЦИИ:\n")
		for _, suggestion := range suggestions {
			fmt.Printf("%s\n", suggestion)
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
