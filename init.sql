CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    chat_id BIGINT NOT NULL,
    CONSTRAINT unique_user_chat UNIQUE (username, chat_id)
);

CREATE TABLE IF NOT EXISTS reminders (
    id SERIAL PRIMARY KEY,
    text TEXT NOT NULL,
    send_time TEXT NOT NULL,
    chat_id BIGINT NOT NULL
);

ALTER TABLE reminders 
ADD COLUMN IF NOT EXISTS
crown_task_id INT NOT NULL;

CREATE TABLE IF NOT EXISTS reminders_users (
    reminder_id INT REFERENCES reminders(id) ON DELETE CASCADE,
    user_id INT REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (reminder_id, user_id)
)