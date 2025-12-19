package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"reminder/internal/bot"
	"reminder/internal/scheduler"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"gopkg.in/telebot.v3"
)

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	dbURL := os.Getenv("DATABASE_URL")

	if token == "" || dbURL == "" {
		log.Fatal("TELEGRAM_TOKEN and DATABASE_URL are required")
	}

	// Подключение к БД через pgx драйвер
	database, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Failed to open DB connection: %v", err)
	}

	// Проверка соединения с БД
	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping DB: %v", err)
	}
	defer database.Close()

	// --- ИНИЦИАЛИЗАЦИЯ БОТА ---
	b, err := telebot.NewBot(telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// --- ИНИЦИАЛИЗАЦИЯ ПЛАНИРОВЩИКА ---
	sch := scheduler.NewScheduler(b)
	sch.Start()

	// Загрузка напоминаний из базы
	if err := bot.LoadRemindersFromDB(b, sch, database); err != nil {
		log.Printf("Failed to load reminders: %v", err)
	}

	// Регистрация обработчиков
	bot.RegisterHandlers(b, database, sch)

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Println("Bot started")
	b.Start()

	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	sch.Stop(ctx)
}
