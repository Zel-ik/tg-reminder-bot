package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"github.com/tucnak/telebot"
)

// Структура пользователя
type User struct {
	ID       int64
	Username string
	ChatID   int64
}

// Структура напоминания
type Reminder struct {
	ID       int64
	Text     string
	SendTime string // Храним время как строку "HH:MM"
	ChatID   int64
}

// Подключение к базе данных PostgreSQL
func connectDB() *pg.DB {
	err := godotenv.Load() // Загружаем .env
	if err != nil {
		log.Fatal("Ошибка загрузки .env файла")
	}

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	return pg.Connect(&pg.Options{
		Addr:     fmt.Sprintf("%s:%s", dbHost, dbPort),
		User:     dbUser,
		Password: dbPassword,
		Database: dbName,
	})
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Ошибка загрузки .env файла")
	}

	db := connectDB()
	query, err := os.ReadFile("init.sql")
	if err != nil {
		log.Fatal("Ошибка чтения init.sql:", err)
	}
	_, err = db.Exec(string(query))
	if err != nil {
		log.Fatal("Ошибка выполнения миграций:", err)
	}
	defer db.Close()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	bot, err := telebot.NewBot(telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Регулярное выражение для поиска времени в формате HH:MM
	timeRegex := regexp.MustCompile(`\b(\d{1,2}):(\d{2})\b`)

	// Функция добавления пользователя в базу данных
	addUser := func(username string, chatID int64) {
		user := &User{Username: username, ChatID: chatID}
		_, err := db.Model(user).OnConflict("(username, chat_id) DO NOTHING").Insert() // Учитываем chat_id при уникальности
		if err != nil {
			log.Println("Ошибка при добавлении пользователя:", err)
		}
	}

	// Функция добавления напоминания с временем и chat_id
	addReminder := func(text string, sendTime string, chatID int64) {
		reminder := &Reminder{Text: text, SendTime: sendTime, ChatID: chatID}
		_, err := db.Model(reminder).Insert()
		if err != nil {
			log.Println("Ошибка при добавлении напоминания:", err)
		}
	}

	// Функция отправки сообщений всем пользователям в конкретном чате
	sendReminderToUsers := func(text string, chatID int64) {
		var users []User
		err := db.Model(&users).Where("chat_id = ?", chatID).Select() // Фильтруем пользователей по chat_id
		if err != nil {
			log.Println("Ошибка при получении пользователей:", err)
			return
		}

		if len(users) == 0 {
			log.Println("Нет пользователей для отправки напоминания.")
			return
		}

		// Собираем всех пользователей в одно сообщение с тегами
		var mentions []string
		for _, user := range users {
			mentions = append(mentions, fmt.Sprintf("@%s", user.Username))
		}

		// Отправляем одно сообщение с тегами всех пользователей
		finalMessage := fmt.Sprintf("%s %s", strings.Join(mentions, " "), text)
		bot.Send(&telebot.Chat{ID: chatID}, finalMessage)
	}

	// Настройка Cron для отправки сообщений в заданное время
	c := cron.New()

	// Загружаем все напоминания и создаем расписание
	var reminders []Reminder
	err = db.Model(&reminders).Select()
	if err == nil {
		for _, r := range reminders {
			cronTime := fmt.Sprintf("%s %s * * *", r.SendTime[3:5], r.SendTime[0:2]) // Минуты Часы

			c.AddFunc(cronTime, func(text string, chatID int64) func() {
				return func() {
					day := time.Now().Weekday()
					if day < time.Monday || day > time.Friday {
						log.Println("Пропускаем задачу, так как сегодня выходной:", day)
						return
					}

					log.Println("Отправка напоминания:", text)
					sendReminderToUsers(text, chatID)
				}
			}(r.Text, r.ChatID))
		}
	}

	c.Start()

	// Команда для добавления пользователя
	bot.Handle("/adduser", func(m *telebot.Message) {
		addUser(m.Sender.Username, m.Chat.ID)
		bot.Send(m.Sender, "Ты был добавлен в список пользователей!")
	})

	// Команда для установки напоминания с временем и chat_id
	bot.Handle("/setreminder", func(m *telebot.Message) {
		reminderText := m.Text
		chatID := m.Chat.ID // Получаем chat_id из сообщения

		// Ищем время в сообщении
		matches := timeRegex.FindStringSubmatch(reminderText)
		if len(matches) != 3 {
			bot.Send(m.Sender, "Неверный формат! Используй: /setreminder HH:MM текст напоминания")
			return
		}

		// Преобразуем время в строку формата "15:04"
		hour, _ := strconv.Atoi(matches[1])
		minute, _ := strconv.Atoi(matches[2])
		if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			bot.Send(m.Sender, "Неверное время! Используй формат HH:MM, например, 09:30")
			return
		}

		// Создаем время на основе текущей даты и введенного времени
		currentTime := time.Now()
		sendTimeStr := fmt.Sprintf("%s %s", currentTime.Format("2006-01-02"), matches[0])
		sendTime, err := time.Parse("2006-01-02 15:04", sendTimeStr)
		if err != nil {
			log.Println("Ошибка при парсинге времени:", err)
			bot.Send(m.Sender, "Ошибка при обработке времени.")
			return
		}

		// Вычитаем 3 часа
		sendTime = sendTime.Add(-3 * time.Hour)

		// Убираем время и '/setreminder' из текста
		cleanReminder := strings.TrimSpace(strings.Replace(reminderText, matches[0], "", 1))
		cleanReminder = strings.Replace(cleanReminder, "/setreminder", "", 1)

		// Сохраняем напоминание в БД
		addReminder(cleanReminder, sendTime.Format("15:04"), chatID) // Добавляем chatID в напоминание
		bot.Send(m.Sender, fmt.Sprintf("Напоминание сохранено! Оно будет отправлено в %s", sendTime.Format("15:04")))

		// Добавляем задачу в cron (учитывая только будние дни 1-5)
		cronTime := fmt.Sprintf("%d %d * * 1-5", sendTime.Minute(), sendTime.Hour())

		c.AddFunc(cronTime, func() {
			sendReminderToUsers(cleanReminder, chatID) // Отправляем напоминание в конкретный чат
		})
	})

	// Запускаем бота
	bot.Start()
}
