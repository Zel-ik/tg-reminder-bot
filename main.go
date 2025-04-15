package main

import (
	"errors"
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
	telebot "gopkg.in/telebot.v3"
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
	bot.Use(AdminMiddleware)
	if err != nil {
		log.Fatal(err)
	}

	// Регулярное выражение для поиска времени в формате HH:MM
	timeRegex := regexp.MustCompile(`\b(\d{1,2}):(\d{2})\b`)

	// Функция отправки сообщений всем пользователям в конкретном чате
	sendReminderToUsers := func(text string, chatID int64) {
		var users []User
		getUsers(&users, chatID)

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

	// Команда для добавления нескольких пользователей
	bot.Handle("/addusers", func(m telebot.Context) error {
		args := strings.Split(m.Text(), " ")[1:] // Берем все аргументы после команды
		if len(args) == 0 {
			bot.Send(m.Chat(), "Используй: /addusers @user1 @user2 @user3")
			return errors.New("неправильный формат")
		}

		var addedUsers []string

		for _, username := range args {
			username = strings.TrimPrefix(username, "@") // Убираем @ перед ником
			if username == "" {
				continue
			}

			// Добавляем пользователя в базу данных без проверки уникальности
			user := &User{Username: username, ChatID: m.Chat().ID}
			_, err := db.Model(user).Insert()
			if err != nil {
				log.Println("Ошибка при добавлении пользователя:", err)
				continue
			}

			addedUsers = append(addedUsers, "@"+username)
		}

		if len(addedUsers) == 0 {
			bot.Send(m.Chat(), "Не удалось добавить пользователей.")
			return errors.New("не удалость добавить пользователя")
		}

		bot.Send(m.Chat(), fmt.Sprintf("Добавлены пользователи: %s", strings.Join(addedUsers, ", ")))
		return nil
	})

	// Команда для добавления пользователя
	bot.Handle("/adduser", func(m telebot.Context) error {
		addUser(m.Sender().Username, m.Chat().ID)
		bot.Send(m.Chat(), "Ты был добавлен в список пользователей!")
		return nil
	})

	// Команда для установки напоминания с временем и chat_id
	bot.Handle("/setreminder", func(m telebot.Context) error {
		reminderText := m.Text()
		chatID := m.Chat().ID

		// Ищем время в сообщении
		matches := timeRegex.FindStringSubmatch(reminderText)
		if len(matches) != 3 {
			bot.Send(m.Chat(), "Неверный формат! Используй: /setreminder HH:MM текст напоминания")
			return errors.New("неверный формат! Используй: /setreminder HH:MM текст напоминания")
		}

		// Преобразуем время в числа
		hour, _ := strconv.Atoi(matches[1])
		minute, _ := strconv.Atoi(matches[2])
		if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			bot.Send(m.Chat(), "Неверное время! Используй формат HH:MM, например, 09:30")
			return errors.New("неверное время! Используй формат HH:MM, например, 09:30")
		}

		// Убираем время и '/setreminder' из текста
		cleanReminder := strings.TrimSpace(strings.Replace(reminderText, matches[0], "", 1))
		cleanReminder = strings.Replace(cleanReminder, "/setreminder", "", 1)

		// Сохраняем напоминание в БД
		addReminder(cleanReminder, fmt.Sprintf("%02d:%02d", hour, minute), chatID)

		bot.Send(m.Chat(), fmt.Sprintf("Напоминание сохранено! Оно будет отправлено в %02d:%02d по московскому времени.", hour, minute))

		now := time.Now().UTC().Unix()
		mskTime := time.Unix(now, 0).UTC()
		sendTimeUTC := time.Date(mskTime.Year(), mskTime.Month(), mskTime.Day(), hour-3, minute, 0, 0, time.UTC)

		// Добавляем задачу в cron (по локальному времени сервера)
		cronTime := fmt.Sprintf("%d %d * * 1-5", sendTimeUTC.Minute(), sendTimeUTC.Hour())

		c.AddFunc(cronTime, func() {
			sendReminderToUsers(cleanReminder, chatID)
		})
		return nil
	})

	// Команда для удаления напоминания по ID
	bot.Handle("/deletereminder", func(m telebot.Context) error {
		args := strings.Fields(strings.TrimSpace(m.Text()))
		if len(args) < 2 {
			bot.Send(m.Chat(), "Укажите ID напоминания, которое хотите удалить.")
			return errors.New("укажите ID напоминания, которое хотите удалить")
		}

		reminderID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			bot.Send(m.Chat(), "Неверный формат ID напоминания.")
			return errors.New("неверный формат ID напоминания")
		}

		deleteReminder(reminderID)
		bot.Send(m.Chat(), fmt.Sprintf("Напоминание с ID %d удалено.", reminderID))
		return nil
	})
	// Команда для удаления напоминания по ID
	bot.Handle("/deleteuser", func(m telebot.Context) error {
		args := strings.Fields(strings.TrimSpace(m.Text()))
		if len(args) < 2 {
			bot.Send(m.Chat(), "Укажите ID напоминания, которое хотите удалить.")
			return errors.New("укажите ID напоминания, которое хотите удалить")
		}

		userID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			bot.Send(m.Chat(), "Неверный формат ID пользователя.")
			return errors.New("неверный формат ID пользователя")
		}

		deleteUser(userID)
		bot.Send(m.Chat(), fmt.Sprintf("Пользователь с ID %d удален.", userID))
		return nil
	})

	// Обработчик команды /updatecron
	bot.Handle("/updatecron", func(m telebot.Context) error {
		updateCron(c, db, bot)
		bot.Send(m.Chat(), "Расписание напоминаний обновлено")
		return nil
	})

	// Команда для получения списка пользователей
	bot.Handle("/listusers", func(m telebot.Context) error {
		message, err := listUsers(m.Chat().ID)
		if err != nil {
			log.Println("Ошибка при получении пользователей:", err)
			return errors.New("ошибка при получении пользователей")
		}
		bot.Send(m.Chat(), message)
		return nil
	})

	// Команда для получения списка напоминаний
	bot.Handle("/listreminders", func(m telebot.Context) error {
		message, err := listReminders(m.Chat().ID)
		if err != nil {
			log.Println("Ошибка при получении напоминаний:", err)
			bot.Send(m.Chat(), "Произошла ошибка при получении напоминаний.")
			return errors.New("произошла ошибка при получении напоминаний")
		}
		bot.Send(m.Chat(), message)
		return nil
	})

	// Запускаем бота
	bot.Start()
}

// Функция обновления Cron с учетом временных зон
func updateCron(c *cron.Cron, db *pg.DB, bot *telebot.Bot) {
	c.Stop()
	entries := c.Entries()
	for _, val := range entries {
		c.Remove(val.ID)
	}

	var reminders []Reminder
	err := db.Model(&reminders).Select()
	if err != nil {
		log.Println("Ошибка при получении напоминаний:", err)
		return
	}

	// Смещение Москвы (UTC+3)
	mskOffset := 3 * 60 * 60

	var users []User

	for _, r := range reminders {
		// Парсим сохранённое время (формат "HH:MM")
		parsedTime, err := time.Parse("15:04", r.SendTime)
		if err != nil {
			log.Println("Ошибка при парсинге времени напоминания:", err)
			continue
		}

		// Устанавливаем московское время
		now := time.Now().UTC().Unix() + int64(mskOffset)
		mskTime := time.Unix(now, 0).UTC()
		sendTimeUTC := time.Date(mskTime.Year(), mskTime.Month(), mskTime.Day(), parsedTime.Hour()-3, parsedTime.Minute(), 0, 0, time.UTC)

		// Формируем cron-выражение
		cronTime := fmt.Sprintf("%d %d * * 1-5", sendTimeUTC.Minute(), sendTimeUTC.Hour())
		getUsers(&users, r.ChatID)
		var message strings.Builder
		for _, user := range users {
			message.WriteString(fmt.Sprintf("@%s,", user.Username))
		}
		message.WriteString(r.Text)
		// Добавляем в cron
		c.AddFunc(cronTime, func(chatID int64, text string) func() {
			return func() {
				day := time.Now().Weekday()
				if day < time.Monday || day > time.Friday {
					log.Println("Пропускаем задачу, так как сегодня выходной:", day)
					return
				}
				log.Println("Отправка напоминания:", text)
				bot.Send(&telebot.Chat{ID: chatID}, text)
			}
		}(r.ChatID, message.String()))
	}
	c.Start()
	log.Println("✅ Cron обновлён для корректного времени!")
}
