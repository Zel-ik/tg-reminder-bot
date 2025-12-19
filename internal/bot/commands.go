// internal/bot/commands.go
package bot

import (
	"database/sql"

	"gopkg.in/telebot.v3"
)

func onCreate(b *telebot.Bot, msg *telebot.Message) {
	userID := msg.Sender.ID
	ClearUserState(userID)
	SetUserState(userID, &State{
		Step: "create_waiting_schedule",
		Data: make(map[string]interface{}),
	})
	b.Send(msg.Chat, "Введите время (hh:mm) или cron-выражение (например: 0 9 * * 1):")
}

func onList(b *telebot.Bot, msg *telebot.Message, db *sql.DB) {
	sendRemindersList(b, db, msg.Chat)
}

func onDeleteStart(b *telebot.Bot, msg *telebot.Message, db *sql.DB) {
	sendRemindersList(b, db, msg.Chat)
	b.Send(msg.Chat, "введите уникальное название напоминания, которое хотите удалить")

	SetUserState(msg.Sender.ID, &State{
		Step: "delete_waiting_name",
		Data: make(map[string]interface{}),
	})
}

func onEditStart(b *telebot.Bot, msg *telebot.Message, db *sql.DB, editType string) {
	sendRemindersList(b, db, msg.Chat)

	b.Send(msg.Chat, "Введите уникальное название напоминания, которое хотите изменить:")

	SetUserState(msg.Sender.ID, &State{
		Step: "edit_waiting_name",
		Data: map[string]interface{}{"edit_type": editType},
	})
}
