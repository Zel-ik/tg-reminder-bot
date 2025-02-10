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
		_, err := db.Model(user).OnConflict("(username) DO NOTHING").Insert()
		if err != nil {
			log.Println("Ошибка при добавлении пользователя:", err)
		}
	}

	// Функция добавления напоминания с временем
	addReminder := func(text string, sendTime string) {
		reminder := &Reminder{Text: text, SendTime: sendTime}
		_, err := db.Model(reminder).Insert()
		if err != nil {
			log.Println("Ошибка при добавлении напоминания:", err)
		}
	}

	// Функция отправки сообщений всем пользователям
	sendReminderToUsers := func(text string) {
		var users []User
		err := db.Model(&users).Select()
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
		chatID := users[0].ChatID // Предполагаем, что все пользователи из одного чата

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
		for _, r := range reminders { // Используем локальную переменную r
			cronTime := fmt.Sprintf("%s %s * * *", r.SendTime[3:5], r.SendTime[0:2]) // Минуты Часы

			// Используем замыкание, чтобы захватить r.Text
			c.AddFunc(cronTime, func(text string) func() {
				return func() {
					sendReminderToUsers(text)
				}
			}(r.Text))
		}

	}

	c.Start()

	// Команда для добавления пользователя
	bot.Handle("/adduser", func(m *telebot.Message) {
		addUser(m.Sender.Username, m.Chat.ID)
		bot.Send(m.Sender, "Ты был добавлен в список пользователей!")
	})

	// Команда для установки напоминания с временем
	bot.Handle("/setreminder", func(m *telebot.Message) {
		reminderText := m.Text

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
		addReminder(cleanReminder, sendTime.Format("15:04")) // Сохраняем уже скорректированное время
		bot.Send(m.Sender, fmt.Sprintf("Напоминание сохранено! Оно будет отправлено в %s", sendTime.Format("15:04")))

		// Добавляем задачу в cron (учитывая только будние дни 1-5)
		cronTime := fmt.Sprintf("%d %d * * 1-5", sendTime.Minute(), sendTime.Hour())

		c.AddFunc(cronTime, func() {
			sendReminderToUsers(cleanReminder)
		})
	})

	// Команда для получения списка всех напоминаний
	bot.Handle("/listreminders", func(m *telebot.Message) {
		var reminders []Reminder
		err := db.Model(&reminders).Select()
		if err != nil {
			bot.Send(m.Chat, "Ошибка при получении списка напоминаний!")
			return
		}

		if len(reminders) == 0 {
			bot.Send(m.Chat, "Нет сохранённых напоминаний!")
			return
		}

		var response string
		for _, rem := range reminders {
			response += fmt.Sprintf("ID: %d | Время: %s | Напоминание: %s\n", rem.ID, rem.SendTime, rem.Text)
		}

		bot.Send(m.Chat, response)
	})

	// Команда для получения списка пользователей
	bot.Handle("/listusers", func(m *telebot.Message) {
		var users []User
		err := db.Model(&users).Select()
		if err != nil {
			bot.Send(m.Chat, "Ошибка при получении списка пользователей.")
			return
		}

		if len(users) == 0 {
			bot.Send(m.Chat, "В базе данных пока нет пользователей.")
			return
		}

		// Формируем список пользователей
		var userList []string
		for i, user := range users {
			userList = append(userList, fmt.Sprintf("%d. @%s", i+1, user.Username))
		}

		response := "Список пользователей:\n" + strings.Join(userList, "\n")
		bot.Send(m.Chat, response)
	})

	// Команда для удаления напоминания по ID
	bot.Handle("/deletereminder", func(m *telebot.Message) {
		args := strings.Split(m.Text, " ")
		if len(args) != 2 {
			bot.Send(m.Chat, "Используй: /deletereminder <ID>")
			return
		}

		id, err := strconv.Atoi(args[1])
		if err != nil {
			bot.Send(m.Chat, "Некорректный ID!")
			return
		}

		// Удаляем напоминание из БД
		res, err := db.Model((*Reminder)(nil)).Where("id = ?", id).Delete()
		if err != nil || res.RowsAffected() == 0 {
			bot.Send(m.Chat, "Ошибка! Напоминание с таким ID не найдено.")
			return
		}

		bot.Send(m.Chat, fmt.Sprintf("Напоминание с ID %d удалено!", id))
	})

	// Команда для добавления списка пользователей
	bot.Handle("/addusers", func(m *telebot.Message) {
		args := strings.Split(m.Text, " ")[1:] // Берём все аргументы после команды
		if len(args) == 0 {
			bot.Send(m.Chat, "Используй: /addusers @user1 @user2 @user3")
			return
		}

		var addedUsers []string

		for _, username := range args {
			username = strings.TrimPrefix(username, "@") // Убираем @ перед ником
			if username == "" {
				continue
			}

			user := &User{Username: username, ChatID: m.Chat.ID}
			_, err := db.Model(user).OnConflict("(username) DO NOTHING").Insert()
			if err != nil {
				log.Println("Ошибка при добавлении пользователя:", err)
				continue
			}

			addedUsers = append(addedUsers, "@"+username)
		}

		if len(addedUsers) == 0 {
			bot.Send(m.Chat, "Не удалось добавить пользователей. Возможно, они уже в списке.")
			return
		}

		bot.Send(m.Chat, fmt.Sprintf("Добавлены пользователи: %s", strings.Join(addedUsers, ", ")))
	})

	bot.Handle("/deleteuser", func(m *telebot.Message) {
		args := strings.Split(m.Text, " ")[1:] // Берем все аргументы после команды
		if len(args) != 1 {
			bot.Send(m.Chat, "Используй: /deleteuser @username")
			return
		}

		username := strings.TrimPrefix(args[0], "@") // Убираем @ перед ником
		if username == "" {
			bot.Send(m.Chat, "Неверный формат! Убедитесь, что вы указали правильный ник.")
			return
		}

		// Ищем пользователя в базе данных и удаляем
		res, err := db.Model((*User)(nil)).Where("username = ?", username).Delete()
		if err != nil || res.RowsAffected() == 0 {
			bot.Send(m.Chat, fmt.Sprintf("Пользователь @%s не найден в базе.", username))
			return
		}

		bot.Send(m.Chat, fmt.Sprintf("Пользователь @%s удален из базы данных.", username))
	})

	// Запускаем бота
	bot.Start()
}
