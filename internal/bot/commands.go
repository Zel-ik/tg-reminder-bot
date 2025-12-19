// internal/bot/commands.go
package bot

import (
	"database/sql"
	"reminder/internal/scheduler"

	"gopkg.in/telebot.v3"
)

func onCreate(b *telebot.Bot, msg *telebot.Message, db *sql.DB, sch *scheduler.Scheduler) {
	userID := msg.Sender.ID
	ClearUserState(userID)
	SetUserState(userID, &State{
		Step: "create_waiting_schedule",
		Data: make(map[string]interface{}),
	})
	b.Send(msg.Chat, "Введите время (hh:mm) или cron-выражение (например: 0 9 * * 1):")
}

func onList(b *telebot.Bot, msg *telebot.Message, db *sql.DB, sch *scheduler.Scheduler) {
	sendRemindersList(b, db, msg.Chat)
}

func onDeleteStart(b *telebot.Bot, msg *telebot.Message, db *sql.DB, sch *scheduler.Scheduler) {
	sendRemindersList(b, db, msg.Chat)
	b.Send(msg.Chat, "Ответьте мне (@GoRemind) названием напоминания для удаления:")

	SetUserState(msg.Sender.ID, &State{
		Step: "delete_waiting_name",
		Data: make(map[string]interface{}),
	})
}

func onEditStart(b *telebot.Bot, msg *telebot.Message, db *sql.DB, sch *scheduler.Scheduler, editType string) {
	sendRemindersList(b, db, msg.Chat)

	var prompt string
	switch editType {
	case "edit_message":
		prompt = "название → изменить сообщение"
	case "edit_time":
		prompt = "название → изменить время/cron"
	case "edit_users":
		prompt = "название → изменить пользователей"
	}
	b.Send(msg.Chat, "Ответьте мне (@GoRemind) "+prompt+":")

	SetUserState(msg.Sender.ID, &State{
		Step: "edit_waiting_name",
		Data: map[string]interface{}{"edit_type": editType},
	})
}
