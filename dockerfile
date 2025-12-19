FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Копируем init.sql в контейнер
COPY init.sql .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bot .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Копируем бинарник и init.sql
COPY --from=builder /app/bot .
COPY --from=builder /app/init.sql .

CMD ["./bot"]
