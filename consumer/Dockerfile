# Простой Dockerfile для Go консьюмера
FROM golang:1.23.3-alpine

WORKDIR /app

# Копируем файлы
COPY . .

# Скачиваем зависимости
RUN go mod download

# Собираем приложение
RUN go build -o consumer main.go

# Запускаем
CMD ["./consumer"] 