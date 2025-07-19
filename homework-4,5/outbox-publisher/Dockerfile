FROM golang:1.23.3-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN go build -o outbox-publisher main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/outbox-publisher .

CMD ["./outbox-publisher"] 