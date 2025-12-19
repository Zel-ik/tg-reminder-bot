// internal/bot/handlers.go
package bot

import (
	"database/sql"
	"reminder/internal/scheduler"

	"gopkg.in/telebot.v3"
)

func RegisterHandlers(b *telebot.Bot, db *sql.DB, sch *scheduler.Scheduler) {
	b.Handle("/create", func(c telebot.Context) error {
		onCreate(b, c.Message())
		return nil
	})

	b.Handle("/list", func(c telebot.Context) error {
		onList(b, c.Message(), db)
		return nil
	})

	b.Handle("/delete", func(c telebot.Context) error {
		onDeleteStart(b, c.Message(), db)
		return nil
	})

	b.Handle("/message", func(c telebot.Context) error {
		onEditStart(b, c.Message(), db, "edit_message")
		return nil
	})

	b.Handle("/time", func(c telebot.Context) error {
		onEditStart(b, c.Message(), db, "edit_time")
		return nil
	})

	b.Handle("/users", func(c telebot.Context) error {
		onEditStart(b, c.Message(), db, "edit_users")
		return nil
	})

	b.Handle(telebot.OnText, func(c telebot.Context) error {
		state := GetUserState(c.Message().Sender.ID)
		if state == nil {
			// Нет состояния для пользователя — игнорируем сообщение
			return nil
		}
		handleState(b, db, sch, c.Message())
		return nil
	})
}
