// internal/bot/validation.go
package bot

import (
	"database/sql"
	"strconv"
	"strings"
)

func parseUserIDs(input string) ([]int64, error) {
	parts := strings.Split(input, ",")
	var ids []int64
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func isReminderNameUnique(db *sql.DB, name string) bool {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM reminders WHERE name = $1)", name).Scan(&exists)
	return err == nil && !exists
}

func isReminderExists(db *sql.DB, name string) bool {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM reminders WHERE name = $1)", name).Scan(&exists)
	return err == nil && exists
}
