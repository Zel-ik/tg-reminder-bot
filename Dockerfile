# Этап сборки
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем только go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod tidy

# Копируем весь исходный код
COPY . .

# Собираем приложение
RUN go build -o bot main.go

# Финальный этап
FROM alpine:latest

WORKDIR /root/

# Копируем собранный бинарник
COPY --from=builder /app/bot .

# Запускаем бота
CMD ["./bot"]
