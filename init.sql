CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    chat_id BIGINT NOT NULL -- Убираем уникальность, так как пользователь может быть в нескольких чатах
);

CREATE TABLE IF NOT EXISTS reminders (
    id SERIAL PRIMARY KEY,
    text TEXT NOT NULL,
    send_time TIME NOT NULL,
    chat_id BIGINT NOT NULL -- Добавляем chat_id в таблицу reminders, чтобы напоминания были привязаны к чатам
);