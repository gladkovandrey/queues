package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type User struct {
	ID    uuid.UUID
	Name  string
	Email string
}

type Payment struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Amount      float64
	Currency    string
	Description string
	Status      string
}

var (
	firstNames = []string{
		"Александр", "Мария", "Дмитрий", "Елена", "Сергей", "Ольга", "Андрей", "Наталья",
		"Максим", "Анна", "Игорь", "Светлана", "Алексей", "Татьяна", "Владимир", "Ирина",
	}

	lastNames = []string{
		"Иванов", "Петров", "Сидоров", "Козлов", "Соколов", "Попов", "Новиков", "Морозов",
		"Волков", "Зайцев", "Михайлов", "Федоров", "Смирнов", "Васильев", "Николаев", "Романов",
	}

	domains = []string{
		"example.com", "test.org", "demo.net", "sample.io", "email.ru",
	}

	paymentDescriptions = []string{
		"Покупка товара в интернет-магазине",
		"Оплата услуг доставки",
		"Пополнение счета мобильного телефона",
		"Оплата коммунальных услуг",
		"Покупка подписки на сервис",
		"Перевод другу",
		"Оплата такси",
		"Покупка билетов в кино",
		"Заказ еды",
		"Оплата парковки",
	}
)

func main() {
	log.Println("🚀 Data Generator запускается...")

	// Получаем настройки из environment
	dbHost := getEnvOrDefault("DB_HOST", "localhost")
	dbPort := getEnvOrDefault("DB_PORT", "5432")
	dbUser := getEnvOrDefault("DB_USER", "postgres")
	dbPassword := getEnvOrDefault("DB_PASSWORD", "password")
	dbName := getEnvOrDefault("DB_NAME", "transactions")

	intervalStr := getEnvOrDefault("GENERATION_INTERVAL", "1")
	interval, err := strconv.Atoi(intervalStr)
	if err != nil {
		interval = 1
	}

	// Подключение к базе данных
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("❌ Ошибка подключения к БД: %v", err)
	}
	defer db.Close()

	// Проверяем подключение
	if err := db.Ping(); err != nil {
		log.Fatalf("❌ БД недоступна: %v", err)
	}

	log.Printf("✅ Подключение к Database A установлено")
	log.Printf("⏱️ Интервал генерации: %d секунд", interval)
	log.Println("📊 Начинаем генерацию данных...")

	// Инициализируем генератор случайных чисел
	rand.Seed(time.Now().UnixNano())

	// Создаем несколько начальных пользователей
	users := createInitialUsers(db, 5)
	log.Printf("👥 Создано начальных пользователей: %d", len(users))

	// Основной цикл генерации
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 70% вероятность создать платеж для существующего пользователя
			// 30% вероятность создать нового пользователя (и платеж для него)
			if rand.Float32() < 0.7 && len(users) > 0 {
				// Создаем платеж для существующего пользователя
				user := users[rand.Intn(len(users))]
				payment := generatePayment(user.ID)
				if createPayment(db, payment) {
					log.Printf("💳 Создан платеж: %.2f %s для %s",
						payment.Amount, payment.Currency, user.Name)
				}
			} else {
				// Создаем нового пользователя и платеж для него
				user := generateUser()
				if createUser(db, user) {
					users = append(users, user)
					log.Printf("👤 Создан пользователь: %s (%s)", user.Name, user.Email)

					// Создаем платеж для нового пользователя
					payment := generatePayment(user.ID)
					if createPayment(db, payment) {
						log.Printf("💳 Создан платеж: %.2f %s для %s",
							payment.Amount, payment.Currency, user.Name)
					}
				}
			}
		}
	}
}

func createInitialUsers(db *sql.DB, count int) []User {
	var users []User

	for i := 0; i < count; i++ {
		user := generateUser()
		if createUser(db, user) {
			users = append(users, user)
		}
	}

	return users
}

func generateUser() User {
	firstName := firstNames[rand.Intn(len(firstNames))]
	lastName := lastNames[rand.Intn(len(lastNames))]
	domain := domains[rand.Intn(len(domains))]

	return User{
		ID:   uuid.New(),
		Name: firstName + " " + lastName,
		Email: fmt.Sprintf("%s.%s@%s",
			transliterate(firstName), transliterate(lastName), domain),
	}
}

func generatePayment(userID uuid.UUID) Payment {
	amounts := []float64{50.0, 100.0, 250.0, 500.0, 1000.0, 1500.0, 2500.0}
	currencies := []string{"USD", "EUR", "RUB"}
	statuses := []string{"pending", "completed", "failed"}

	return Payment{
		ID:          uuid.New(),
		UserID:      userID,
		Amount:      amounts[rand.Intn(len(amounts))],
		Currency:    currencies[rand.Intn(len(currencies))],
		Description: paymentDescriptions[rand.Intn(len(paymentDescriptions))],
		Status:      statuses[rand.Intn(len(statuses))],
	}
}

func createUser(db *sql.DB, user User) bool {
	query := `
		INSERT INTO users (id, name, email) 
		VALUES ($1, $2, $3)`

	_, err := db.Exec(query, user.ID, user.Name, user.Email)
	if err != nil {
		log.Printf("❌ Ошибка создания пользователя: %v", err)
		return false
	}

	return true
}

func createPayment(db *sql.DB, payment Payment) bool {
	query := `
		INSERT INTO payments (id, user_id, amount, currency, description, status) 
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := db.Exec(query, payment.ID, payment.UserID, payment.Amount,
		payment.Currency, payment.Description, payment.Status)
	if err != nil {
		log.Printf("❌ Ошибка создания платежа: %v", err)
		return false
	}

	return true
}

func transliterate(text string) string {
	// Полная транслитерация для email адресов (заглавные и строчные буквы)
	translitMap := map[rune]string{
		// Заглавные буквы
		'А': "a", 'Б': "b", 'В': "v", 'Г': "g", 'Д': "d", 'Е': "e", 'Ж': "zh",
		'З': "z", 'И': "i", 'Й': "y", 'К': "k", 'Л': "l", 'М': "m", 'Н': "n",
		'О': "o", 'П': "p", 'Р': "r", 'С': "s", 'Т': "t", 'У': "u", 'Ф': "f",
		'Х': "h", 'Ц': "ts", 'Ч': "ch", 'Ш': "sh", 'Щ': "sch", 'Ы': "y", 'Э': "e",
		'Ю': "yu", 'Я': "ya", 'Ь': "", 'Ъ': "",
		// Строчные буквы
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ж': "zh",
		'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n",
		'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f",
		'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sch", 'ы': "y", 'э': "e",
		'ю': "yu", 'я': "ya", 'ь': "", 'ъ': "",
	}

	result := ""
	for _, char := range text {
		if latin, exists := translitMap[char]; exists {
			result += latin
		} else {
			result += string(char)
		}
	}

	return result
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
