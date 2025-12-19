-- Таблица напоминаний
CREATE TABLE IF NOT EXISTS reminders (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    name TEXT UNIQUE NOT NULL,
    message TEXT NOT NULL,
    cron_expr TEXT NOT NULL,  -- всегда cron, даже если введено hh:mm
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reminder_users (
    reminder_id INT REFERENCES reminders(id) ON DELETE CASCADE,
    username TEXT NOT NULL,
    PRIMARY KEY (reminder_id, username)
);