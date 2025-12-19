// internal/bot/db_operations.go
package bot

import (
	"database/sql"
	"fmt"
	"strings"

	"reminder/internal/scheduler"

	"gopkg.in/telebot.v3"
)

// Отправка списка напоминаний
func sendRemindersList(b *telebot.Bot, db *sql.DB, chat *telebot.Chat) {
	rows, err := db.Query(`
		SELECT r.name, r.message, r.cron_expr,
		       COALESCE(STRING_AGG(ru.username, ', '), '—') AS usernames
		FROM reminders r
		LEFT JOIN reminder_users ru ON r.id = ru.reminder_id
		GROUP BY r.id
		ORDER BY r.name
	`)
	if err != nil {
		b.Send(chat, "❌ Ошибка загрузки списка.")
		return
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var name, message, cron, users string
		if err := rows.Scan(&name, &message, &cron, &users); err != nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("🔹 <b>%s</b>\n   ⏰ %s\n   👥 %s\n   📝 %s",
			name, cron, users, message))
	}

	if len(lines) == 0 {
		b.Send(chat, "Список напоминаний пуст.")
	} else {
		b.Send(chat, strings.Join(lines, "\n\n"), &telebot.SendOptions{ParseMode: "HTML"})
	}
}

// Сохранение нового напоминания
func saveReminder(db *sql.DB, sch *scheduler.Scheduler, b *telebot.Bot, chatID int64, data map[string]interface{}) error {
	name := data["name"].(string)
	message := data["message"].(string)
	cronExpr := data["cron_expr"].(string)
	usernames := data["usernames"].([]string) // <-- теперь username

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var remID int64
	err = tx.QueryRow(`
		INSERT INTO reminders (chat_id, name, message, cron_expr)
		VALUES ($1, $2, $3, $4) RETURNING id`,
		chatID, name, message, cronExpr).Scan(&remID)
	if err != nil {
		return err
	}

	for _, u := range usernames {
		_, err = tx.Exec("INSERT INTO reminder_users (reminder_id, username) VALUES ($1, $2)", remID, u)
		if err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	job := &scheduler.Job{
		Bot:       b,
		ChatID:    chatID,
		Usernames: usernames, // <-- поле Usernames
		Message:   message,
	}
	return sch.AddJob(name, cronExpr, job)
}

// Удаление
func deleteReminder(db *sql.DB, sch *scheduler.Scheduler, name string) error {
	// Сначала удалим из планировщика
	sch.RemoveJob(name)

	// Затем из БД (CASCADE удалит из reminder_users)
	_, err := db.Exec("DELETE FROM reminders WHERE name = $1", name)
	return err
}

// Обновление сообщения
func updateReminderMessage(db *sql.DB, sch *scheduler.Scheduler, name, newMsg string) error {
	_, err := db.Exec("UPDATE reminders SET message = $1 WHERE name = $2", newMsg, name)
	if err != nil {
		return err
	}
	// Обновим задачу в планировщике (пересоздадим)
	return reloadJob(db, sch, name)
}

// Обновление cron
func updateReminderCron(db *sql.DB, sch *scheduler.Scheduler, name, newCron string) error {
	_, err := db.Exec("UPDATE reminders SET cron_expr = $1 WHERE name = $2", newCron, name)
	if err != nil {
		return err
	}
	return reloadJob(db, sch, name)
}

// Обновление пользователей (поддержка "add 123,456" / "delete 789")
func updateReminderUsers(db *sql.DB, sch *scheduler.Scheduler, name, input string) error {
	var newUsers []string
	var err error

	if strings.HasPrefix(input, "add ") {
		newUsers, err = parseUsernames(strings.TrimPrefix(input, "add "))
	} else if strings.HasPrefix(input, "delete ") {
		newUsers, err = parseUsernames(strings.TrimPrefix(input, "delete "))
	} else {
		newUsers, err = parseUsernames(input)
	}

	if err != nil {
		return fmt.Errorf("ошибка обработки usernames: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var remID int64
	if err := tx.QueryRow("SELECT id FROM reminders WHERE name=$1", name).Scan(&remID); err != nil {
		return err
	}

	if strings.HasPrefix(input, "add ") {
		for _, u := range newUsers {
			tx.Exec("INSERT INTO reminder_users (reminder_id, username) VALUES ($1,$2) ON CONFLICT DO NOTHING", remID, u)
		}
	} else if strings.HasPrefix(input, "delete ") {
		for _, u := range newUsers {
			tx.Exec("DELETE FROM reminder_users WHERE reminder_id=$1 AND username=$2", remID, u)
		}
	} else {
		tx.Exec("DELETE FROM reminder_users WHERE reminder_id=$1", remID)
		for _, u := range newUsers {
			tx.Exec("INSERT INTO reminder_users (reminder_id, username) VALUES ($1,$2)", remID, u)
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return reloadJob(db, sch, name)
}

// Вспомогательная функция: перезагрузка задачи
func reloadJob(db *sql.DB, sch *scheduler.Scheduler, name string) error {
	sch.RemoveJob(name)

	// Загружаем обновлённые данные
	row := db.QueryRow(`
	SELECT r.chat_id, r.message, r.cron_expr, ARRAY_AGG(ru.username)
	FROM reminders r
	LEFT JOIN reminder_users ru ON r.id = ru.reminder_id
	WHERE r.name = $1
	GROUP BY r.id
`, name)

	var chatID int64
	var message, cronExpr string
	var usernames string // <- сканируем массив как string

	if err := row.Scan(&chatID, &message, &cronExpr, &usernames); err != nil {
		return err
	}

	// usernames приходит как "{@one,@two,@three}"
	usernames = strings.Trim(usernames, "{}")
	usernameList := strings.Split(usernames, ",")
	for i, u := range usernameList {
		usernameList[i] = strings.TrimSpace(u)
	}

	job := &scheduler.Job{
		Bot:       sch.Bot,
		ChatID:    chatID,
		Usernames: usernameList,
		Message:   message,
	}
	return sch.AddJob(name, cronExpr, job)
}
