package models

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username,omitempty"`
	FullName string `json:"full_name,omitempty"`
}

// internal/models/models.go
type Reminder struct {
	ID       int64   `json:"id"`
	ChatID   int64   `json:"chat_id"`
	Name     string  `json:"name"`
	Message  string  `json:"message"`
	CronExpr string  `json:"cron_expr"` // всегда cron
	UserIDs  []int64 `json:"user_ids"`
}
