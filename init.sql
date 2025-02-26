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