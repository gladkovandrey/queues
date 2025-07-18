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

// Структура статистики ошибок (из aggregator)
type ErrorStats struct {
	WindowStart string         `json:"window_start"`
	WindowEnd   string         `json:"window_end"`
	Services    map[string]int `json:"services"`
	TotalErrors int            `json:"total_errors"`
	GeneratedAt string         `json:"generated_at"`
}

func main() {
	// Получаем настройки
	servers := getEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:19092")
	topic := getEnvOrDefault("KAFKA_TOPIC", "error-stats")
	consumerGroup := getEnvOrDefault("CONSUMER_GROUP", "stats-display")

	log.Printf("📊 Stats Consumer запущен")
	log.Printf("📥 Читаем из: %s", topic)

	brokers := strings.Split(servers, ",")

	// Создаем reader для чтения статистики
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: consumerGroup,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	log.Printf("✅ Подключение к Kafka установлено")
	log.Printf("\n🔍 Ожидаем статистику ошибок...\n")

	// Бесконечный цикл чтения статистики
	for {
		message, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("❌ Ошибка чтения: %v", err)
			continue
		}

		// Парсим статистику
		var stats ErrorStats
		err = json.Unmarshal(message.Value, &stats)
		if err != nil {
			log.Printf("❌ Ошибка JSON: %v", err)
			continue
		}

		// Красиво отображаем статистику
		displayStats(&stats)
	}
}

func displayStats(stats *ErrorStats) {
	fmt.Printf("\n" + strings.Repeat("=", 70) + "\n")
	fmt.Printf("📊 СТАТИСТИКА ОШИБОК ЗА ПЕРИОД\n")
	fmt.Printf("🕐 Период: %s → %s\n", stats.WindowStart, stats.WindowEnd)
	fmt.Printf("📈 Всего ошибок: %d\n", stats.TotalErrors)
	fmt.Printf("⏰ Сгенерировано: %s\n", stats.GeneratedAt)
	fmt.Printf(strings.Repeat("-", 70) + "\n")

	if len(stats.Services) == 0 {
		fmt.Printf("✅ Ошибок не обнаружено\n")
	} else {
		fmt.Printf("🔍 Детализация по сервисам:\n")
		
		// Сортируем сервисы по количеству ошибок (простая сортировка)
		type serviceError struct {
			name  string
			count int
		}
		
		var sorted []serviceError
		for service, count := range stats.Services {
			sorted = append(sorted, serviceError{service, count})
		}
		
		// Простая сортировка по убыванию
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].count > sorted[i].count {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		for i, se := range sorted {
			percentage := float64(se.count) / float64(stats.TotalErrors) * 100
			bar := generateBar(percentage)
			
			var icon string
			switch i {
			case 0:
				icon = "🥇" // Первое место
			case 1:
				icon = "🥈" // Второе место
			case 2:
				icon = "🥉" // Третье место
			default:
				icon = "📌"
			}
			
			fmt.Printf("   %s %-18s: %3d ошибок (%5.1f%%) %s\n",
				icon, se.name, se.count, percentage, bar)
		}
	}

	fmt.Printf(strings.Repeat("=", 70) + "\n\n")
}

func generateBar(percentage float64) string {
	barLength := 20
	filled := int(percentage / 100.0 * float64(barLength))
	
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

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
} 