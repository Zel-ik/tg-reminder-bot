FROM golang:1.23.0

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod tidy

COPY . .

RUN go build -o bot main.go

CMD ["./bot"]