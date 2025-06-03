-- Таблица пользователей
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id INTEGER UNIQUE,
    name TEXT,
    phone TEXT,
    is_admin BOOLEAN DEFAULT 0
);

-- Таблица услуг
CREATE TABLE IF NOT EXISTS services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    duration INTEGER NOT NULL, -- в минутах
    price INTEGER
);

-- Таблица записей
CREATE TABLE IF NOT EXISTS bookings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    service_id INTEGER,
    date TEXT NOT NULL, -- YYYY-MM-DD
    time TEXT NOT NULL, -- HH:MM
    status TEXT NOT NULL DEFAULT 'Ожидание', -- Ожидание, Подтверждено, Отменено
    phone_confirmed BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES users(id),
    FOREIGN KEY(service_id) REFERENCES services(id)
);

-- Таблица для хранения состояния пользователя (сценарий записи)
CREATE TABLE IF NOT EXISTS user_states (
    telegram_id INTEGER PRIMARY KEY,
    step TEXT,
    service TEXT,
    date TEXT,
    time TEXT,
    phone TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
