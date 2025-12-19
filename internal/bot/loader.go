package bot

import (
	"database/sql"
	"fmt"
	"log"

	"reminder/internal/models"
	"reminder/internal/scheduler"

	"gopkg.in/telebot.v3"
)

func LoadRemindersFromDB(b *telebot.Bot, sch *scheduler.Scheduler, db *sql.DB) error {
	rows, err := db.Query(`
        SELECT r.id, r.chat_id, r.name, r.message, r.cron_expr,
               ARRAY_AGG(ru.user_id) AS user_ids
        FROM reminders r
        LEFT JOIN reminder_users ru ON r.id = ru.reminder_id
        GROUP BY r.id
    `)
	if err != nil {
		return fmt.Errorf("query reminders: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r models.Reminder
		var userIDs []sql.NullInt64

		if err := rows.Scan(&r.ID, &r.ChatID, &r.Name, &r.Message, &r.CronExpr, &userIDs); err != nil {
			log.Printf("Failed to scan reminder: %v", err)
			continue
		}

		// Преобразуем []sql.NullInt64 → []int64
		for _, uid := range userIDs {
			if uid.Valid {
				r.UserIDs = append(r.UserIDs, uid.Int64)
			}
		}

		if len(r.UserIDs) == 0 {
			log.Printf("Skipping reminder %q: no users", r.Name)
			continue
		}

		job := &scheduler.Job{
			Bot:     b,
			ChatID:  r.ChatID,
			UserIDs: r.UserIDs,
			Message: r.Message,
		}

		if err := sch.AddJob(r.Name, r.CronExpr, job); err != nil {
			log.Printf("Failed to add job %q: %v", r.Name, err)
		} else {
			log.Printf("Loaded reminder: %s (%s)", r.Name, r.CronExpr)
		}
	}
	return nil
}
