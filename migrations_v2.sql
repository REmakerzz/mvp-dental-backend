-- Таблица врачей
CREATE TABLE IF NOT EXISTS doctors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    specialization TEXT NOT NULL,
    description TEXT,
    photo_url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Таблица связи врачей с услугами
CREATE TABLE IF NOT EXISTS doctor_services (
    doctor_id INTEGER,
    service_id INTEGER,
    FOREIGN KEY(doctor_id) REFERENCES doctors(id) ON DELETE CASCADE,
    FOREIGN KEY(service_id) REFERENCES services(id) ON DELETE CASCADE,
    PRIMARY KEY(doctor_id, service_id)
);

-- Таблица расписания врачей
CREATE TABLE IF NOT EXISTS doctor_schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    doctor_id INTEGER,
    weekday INTEGER NOT NULL, -- 0-6 (воскресенье-суббота)
    start_time TEXT NOT NULL, -- HH:MM
    end_time TEXT NOT NULL, -- HH:MM
    break_start TEXT, -- HH:MM
    break_end TEXT, -- HH:MM
    FOREIGN KEY(doctor_id) REFERENCES doctors(id) ON DELETE CASCADE
);

-- Добавляем поле doctor_id в таблицу bookings
ALTER TABLE bookings ADD COLUMN doctor_id INTEGER REFERENCES doctors(id);

-- Создаем индексы для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_bookings_date ON bookings(date);
CREATE INDEX IF NOT EXISTS idx_bookings_doctor ON bookings(doctor_id);
CREATE INDEX IF NOT EXISTS idx_doctor_schedules_doctor ON doctor_schedules(doctor_id);
CREATE INDEX IF NOT EXISTS idx_doctor_schedules_weekday ON doctor_schedules(weekday);

-- Добавляем базовых врачей
INSERT INTO doctors (name, specialization, description) VALUES
('Иванов Иван Иванович', 'Терапевт', 'Опытный стоматолог-терапевт с 10-летним стажем'),
('Петрова Анна Сергеевна', 'Ортодонт', 'Специалист по исправлению прикуса и выравниванию зубов'),
('Сидоров Алексей Петрович', 'Хирург', 'Стоматолог-хирург, специалист по имплантации');

-- Добавляем базовое расписание для всех врачей
INSERT INTO doctor_schedules (doctor_id, weekday, start_time, end_time, break_start, break_end)
SELECT 
    d.id,
    w.weekday,
    '09:00',
    '18:00',
    '13:00',
    '14:00'
FROM doctors d
CROSS JOIN (
    SELECT 1 as weekday UNION SELECT 2 UNION SELECT 3 
    UNION SELECT 4 UNION SELECT 5
) w; 