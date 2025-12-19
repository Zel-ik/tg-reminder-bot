-- Таблица напоминаний
CREATE TABLE IF NOT EXISTS reminders (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    name TEXT UNIQUE NOT NULL,
    message TEXT NOT NULL,
    cron_expr TEXT NOT NULL,  -- всегда cron, даже если введено hh:mm
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Таблица пользователей (опционально, но удобно)
-- Можно хранить только ID, но для отладки и читаемости — имя
CREATE TABLE IF NOT EXISTS users (
    user_id BIGINT PRIMARY KEY,
    username TEXT,
    full_name TEXT
);

-- Связь многие-ко-многим: напоминание → пользователи
CREATE TABLE IF NOT EXISTS reminder_users (
    reminder_id INT REFERENCES reminders(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(user_id) ON DELETE CASCADE,
    PRIMARY KEY (reminder_id, user_id)
);