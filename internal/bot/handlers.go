// internal/bot/handlers.go
package bot

import (
	"database/sql"
	"reminder/internal/scheduler"

	"gopkg.in/telebot.v3"
)

func RegisterHandlers(b *telebot.Bot, db *sql.DB, sch *scheduler.Scheduler) {
	b.Handle("/create", func(c telebot.Context) error {
		onCreate(b, c.Message(), db, sch)
		return nil
	})

	b.Handle("/list", func(c telebot.Context) error {
		onList(b, c.Message(), db, sch)
		return nil
	})

	b.Handle("/delete", func(c telebot.Context) error {
		onDeleteStart(b, c.Message(), db, sch)
		return nil
	})

	b.Handle("/editMessage", func(c telebot.Context) error {
		onEditStart(b, c.Message(), db, sch, "edit_message")
		return nil
	})

	b.Handle("/editTime", func(c telebot.Context) error {
		onEditStart(b, c.Message(), db, sch, "edit_time")
		return nil
	})

	b.Handle("/editUsers", func(c telebot.Context) error {
		onEditStart(b, c.Message(), db, sch, "edit_users")
		return nil
	})

	b.Handle(telebot.OnText, func(c telebot.Context) error {
		if !isMessageToBot(b, c.Message()) {
			return nil
		}
		handleState(b, db, sch, c.Message())
		return nil
	})
}
