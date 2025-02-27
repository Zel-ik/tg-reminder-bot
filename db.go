package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/go-pg/pg/v10"
	"github.com/joho/godotenv"
)

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
	db = pg.Connect(&pg.Options{
		Addr:     fmt.Sprintf("%s:%s", dbHost, dbPort),
		User:     dbUser,
		Password: dbPassword,
		Database: dbName,
	})

	log.Printf("Подключение к базе данных: host=%s port=%s user=%s dbname=%s", dbHost, dbPort, dbUser, dbName)

	// Проверяем соединение с базой
	_, err = db.Exec("SELECT 1")
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}

	return db
}

var db *pg.DB

// Функция добавления пользователя в базу данных
func addUser(username string, chatID int64) {
	user := &User{Username: username, ChatID: chatID}
	_, err := db.Model(user).OnConflict("(username, chat_id) DO NOTHING").Insert() // Учитываем chat_id при уникальности
	if err != nil {
		log.Println("Ошибка при добавлении пользователя:", err)
	}
}

// Функция получения списка напоминаний для конкретного чата
func listReminders(chatID int64) (string, error) {
	var reminders []Reminder
	err := db.Model(&reminders).Where("chat_id = ?", chatID).Select()
	if err != nil {
		return "", err
	}

	if len(reminders) == 0 {
		return "У вас нет напоминаний.", nil
	}

	var message strings.Builder
	message.WriteString("Ваши напоминания:\n")

	var timeString []string
	for _, reminder := range reminders {
		timeString = strings.Split(reminder.SendTime, ":")
		hour, _ := strconv.Atoi(timeString[0])
		minute, _ := strconv.Atoi(timeString[0])
		message.WriteString(fmt.Sprintf("- ID: %d, Время: %02d:%02d, Текст: %s\n", reminder.ID, hour, minute, reminder.Text))
	}

	return message.String(), nil
}

// Функция получения списка пользователей для конкретного чата
func listUsers(chatID int64) (string, error) {
	var users []User
	err := db.Model(&users).Where("chat_id = ?", chatID).Select()
	if err != nil {
		log.Println("Ошибка при получении пользователей:", err)
		return "", err
	}

	if len(users) == 0 {
		log.Println("Нет пользователей в этом чате.")
		return "Нет пользователей в этом чате.", nil
	}
	var message strings.Builder
	message.WriteString("Список пользователей:\n")

	for _, user := range users {
		message.WriteString(fmt.Sprintf("- ID: %d, Username: %s\n", user.ID, user.Username))
	}

	return message.String(), nil
}

// Функция удаления напоминания по ID
func deleteReminder(id int64) {
	reminder := &Reminder{ID: id}
	_, err := db.Model(reminder).Where("id = ?", id).Delete()
	if err != nil {
		log.Println("Ошибка при удалении напоминания:", err)
	}
}

// Функция удаления напоминания по ID
func deleteUser(id int64) {
	user := &User{ID: id}
	_, err := db.Model(user).Where("id = ?", id).Delete()
	if err != nil {
		log.Println("Ошибка при удалении напоминания:", err)
	}
}

// Функция добавления напоминания с временем и chat_id
func addReminder(text string, sendTime string, chatID int64) {
	reminder := &Reminder{Text: text, SendTime: sendTime, ChatID: chatID}
	_, err := db.Model(reminder).Insert()
	if err != nil {
		log.Println("Ошибка при добавлении напоминания:", err)
	}
}

func getUsers(users *[]User, chatID int64) {
	err := db.Model(users).Where("chat_id = ?", chatID).Select() // Фильтруем пользователей по chat_id
	if err != nil {
		log.Println("Ошибка при получении пользователей:", err)
		return
	}
}
