CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    chat_id BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS reminders (
    id SERIAL PRIMARY KEY,
    text TEXT NOT NULL,
    send_time TEXT NOT NULL
);

-- Убираем уникальность с chat_id в таблице users
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_chat_id_key;

-- Добавляем chat_id в таблицу reminders
ALTER TABLE reminders ADD COLUMN IF NOT EXISTS chat_id BIGINT;

-- Если столбец chat_id уже существует, то можно сделать его обязательным
-- Если нужно, добавьте ограничение, чтобы для существующих записей это поле имело значение
-- ALTER TABLE reminders ALTER COLUMN chat_id SET NOT NULL;

