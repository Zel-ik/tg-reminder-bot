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
		       COALESCE(STRING_AGG(ru.user_id::TEXT, ', '), '—') AS user_ids
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
	userIDs := data["user_ids"].([]int64)

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

	for _, uid := range userIDs {
		_, err = tx.Exec("INSERT INTO reminder_users (reminder_id, user_id) VALUES ($1, $2)", remID, uid)
		if err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	job := &scheduler.Job{
		Bot:     b,
		ChatID:  chatID,
		UserIDs: userIDs,
		Message: message,
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
	var newIDs []int64
	var err error

	if strings.HasPrefix(input, "add ") {
		newIDs, err = parseUserIDs(strings.TrimPrefix(input, "add "))
		if err != nil {
			return fmt.Errorf("ошибка в add: %w", err)
		}
	} else if strings.HasPrefix(input, "delete ") {
		newIDs, err = parseUserIDs(strings.TrimPrefix(input, "delete "))
		if err != nil {
			return fmt.Errorf("ошибка в delete: %w", err)
		}
	} else {
		// Полная замена
		newIDs, err = parseUserIDs(input)
		if err != nil {
			return err
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var remID int64
	err = tx.QueryRow("SELECT id FROM reminders WHERE name = $1", name).Scan(&remID)
	if err != nil {
		return err
	}

	if strings.HasPrefix(input, "add ") {
		for _, uid := range newIDs {
			tx.Exec("INSERT INTO reminder_users (reminder_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", remID, uid)
		}
	} else if strings.HasPrefix(input, "delete ") {
		for _, uid := range newIDs {
			tx.Exec("DELETE FROM reminder_users WHERE reminder_id = $1 AND user_id = $2", remID, uid)
		}
	} else {
		// Полная замена
		tx.Exec("DELETE FROM reminder_users WHERE reminder_id = $1", remID)
		for _, uid := range newIDs {
			tx.Exec("INSERT INTO reminder_users (reminder_id, user_id) VALUES ($1, $2)", remID, uid)
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
		SELECT r.chat_id, r.message, r.cron_expr, ARRAY_AGG(ru.user_id)
		FROM reminders r
		LEFT JOIN reminder_users ru ON r.id = ru.reminder_id
		WHERE r.name = $1
		GROUP BY r.id`, name)

	var chatID int64
	var message, cronExpr string
	var userIDs []sql.NullInt64
	if err := row.Scan(&chatID, &message, &cronExpr, &userIDs); err != nil {
		return err
	}

	var ids []int64
	for _, uid := range userIDs {
		if uid.Valid {
			ids = append(ids, uid.Int64)
		}
	}

	if len(ids) == 0 {
		return nil // не добавляем, если нет получателей
	}

	job := &scheduler.Job{
		Bot:     sch.Bot, // предполагается, что scheduler хранит ссылку на Bot
		ChatID:  chatID,
		UserIDs: ids,
		Message: message,
	}
	return sch.AddJob(name, cronExpr, job)
}
