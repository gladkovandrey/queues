# Простой Dockerfile для Go продюсера
FROM golang:1.23.3-alpine

WORKDIR /app

# Копируем файлы
COPY . .

# Скачиваем зависимости
RUN go mod download

# Собираем приложение
RUN go build -o producer main.go

# Запускаем
CMD ["./producer"] 