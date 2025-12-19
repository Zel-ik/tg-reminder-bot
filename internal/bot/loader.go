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
	       ARRAY_AGG(ru.username) AS usernames
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
		var usernames []sql.NullString
		rows.Scan(&r.ID, &r.ChatID, &r.Name, &r.Message, &r.CronExpr, &usernames)

		for _, u := range usernames {
			if u.Valid {
				r.Usernames = append(r.Usernames, u.String)
			}
		}
		job := &scheduler.Job{
			Bot:       b,
			ChatID:    r.ChatID,
			Usernames: r.Usernames,
			Message:   r.Message,
		}

		if err := sch.AddJob(r.Name, r.CronExpr, job); err != nil {
			log.Printf("Failed to add job %q: %v", r.Name, err)
		} else {
			log.Printf("Loaded reminder: %s (%s)", r.Name, r.CronExpr)
		}
	}
	return nil
}
