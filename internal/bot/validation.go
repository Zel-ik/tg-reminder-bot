// internal/bot/validation.go
package bot

import (
	"database/sql"
	"fmt"
	"strings"
)

func parseUsernames(input string) ([]string, error) {
	parts := strings.Fields(input) // split by any whitespace
	var usernames []string
	for _, u := range parts {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !strings.HasPrefix(u, "@") {
			u = "@" + u
		}
		usernames = append(usernames, u)
	}
	if len(usernames) == 0 {
		return nil, fmt.Errorf("нет корректных username")
	}
	return usernames, nil
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
