FROM golang:1.23

WORKDIR /app

COPY . .

RUN go mod tidy && go build -o tg-reminder .

RUN ls -la /app

CMD ["/app/tg-reminder"]
