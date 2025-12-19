// internal/bot/state_handlers.go
package bot

import (
	"database/sql"
	"reminder/internal/scheduler"
	"reminder/internal/utils"
	"strings"

	"gopkg.in/telebot.v3"
)

func handleState(b *telebot.Bot, db *sql.DB, sch *scheduler.Scheduler, msg *telebot.Message) bool {
	state := GetUserState(msg.Sender.ID)
	if state == nil {
		return false
	}

	text := strings.TrimSpace(extractPlainTextAfterBotMention(b, msg))

	if text == "cancel" {
		ClearUserState(msg.Sender.ID)
		b.Send(msg.Chat, "Операция отменена.")
		return true
	}

	switch state.Step {
	case "create_waiting_schedule":
		if cronExpr, err := utils.NormalizeToCron(text); err != nil {
			b.Send(msg.Chat, "❌ Неверный формат. Примеры: 14:30 или 0 8 * * 1")
		} else {
			state.Data["cron_expr"] = cronExpr
			state.Step = "create_waiting_users"
			b.Send(msg.Chat, "Укажите username. Пример: @one @magma @gsgs")
		}
		return true

	case "create_waiting_users":
		if usernames, err := parseUsernames(text); err != nil || len(usernames) == 0 {
			b.Send(msg.Chat, "❌ Укажите username. Пример: @one @magma @gsgs")
		} else {
			state.Data["usernames"] = usernames
			state.Step = "create_waiting_message"
			b.Send(msg.Chat, "Введите сообщение для напоминания:")
		}
		return true

	case "create_waiting_message":
		state.Data["message"] = text
		state.Step = "create_waiting_name"
		b.Send(msg.Chat, "Введите уникальное название напоминания:")
		return true

	case "create_waiting_name":
		if text == "" {
			b.Send(msg.Chat, "❌ Название не может быть пустым.")
		} else if !isReminderNameUnique(db, text) {
			b.Send(msg.Chat, "❌ Название уже существует. Введите другое:")
		} else {
			state.Data["name"] = text
			if err := saveReminder(db, sch, b, msg.Chat.ID, state.Data); err != nil {
				b.Send(msg.Chat, "❌ Ошибка: "+err.Error())
			} else {
				b.Send(msg.Chat, "✅ Напоминание создано!")
			}
			ClearUserState(msg.Sender.ID)
		}
		return true

	case "delete_waiting_name":
		if !isReminderExists(db, text) {
			b.Send(msg.Chat, "❌ Напоминание не найдено. Попробуйте снова:")
		} else if err := deleteReminder(db, sch, text); err != nil {
			b.Send(msg.Chat, "❌ Ошибка удаления: "+err.Error())
		} else {
			b.Send(msg.Chat, "✅ Удалено.")
			ClearUserState(msg.Sender.ID)
		}
		return true

	case "edit_waiting_name":
		if !isReminderExists(db, text) {
			b.Send(msg.Chat, "❌ Напоминание не найдено. Введите название:")
		} else {
			state.Data["target_name"] = text
			state.Step = "edit_waiting_value"
			editType := state.Data["edit_type"].(string)
			switch editType {
			case "edit_message":
				b.Send(msg.Chat, "Введите новое сообщение:")
			case "edit_time":
				b.Send(msg.Chat, "Введите новое время или cron:")
			case "edit_users":
				b.Send(msg.Chat, "Введите пользователей для добавление через add (Пример: add @Zel @alko) или через delete для удаления(Пример: delete @Zel)")
			}
		}
		return true

	case "edit_waiting_value":
		target := state.Data["target_name"].(string)
		editType := state.Data["edit_type"].(string)
		var err error

		switch editType {
		case "edit_message":
			err = updateReminderMessage(db, sch, target, text)
		case "edit_time":
			if cron, e := utils.NormalizeToCron(text); e != nil {
				b.Send(msg.Chat, "❌ Неверный формат времени/cron.")
				return true
			} else {
				err = updateReminderCron(db, sch, target, cron)
			}
		case "edit_users":
			// text теперь содержит "@username @othername"
			if len(text) == 0 {
				b.Send(msg.Chat, "❌ Укажите хотя бы одного пользователя через пробел (@username).")
				return true
			}
			err = updateReminderUsers(db, sch, target, text)
		}

		if err != nil {
			b.Send(msg.Chat, "❌ Ошибка: "+err.Error())
		} else {
			b.Send(msg.Chat, "✅ Обновлено.")
		}
		ClearUserState(msg.Sender.ID)
		return true

	default:
		return false
	}
}
