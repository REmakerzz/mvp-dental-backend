package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"MVP_ChatBot/models"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB(dbPath string, migrationsPath string) *sql.DB {
	// Создаем директорию для базы данных, если она не существует
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatal(err)
	}

	// Открываем соединение с базой данных
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	// Читаем и выполняем миграции
	migrations, err := os.ReadFile(migrationsPath)
	if err != nil {
		log.Fatal(err)
	}

	// Разделяем миграции на отдельные запросы
	queries := strings.Split(string(migrations), ";")
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		_, err := db.Exec(query)
		if err != nil {
			log.Printf("Error executing migration: %v\nQuery: %s", err, query)
		}
	}

	return db
}

func GetUserState(db *sql.DB, telegramID int64) (*models.UserState, error) {
	var s models.UserState
	err := db.QueryRow("SELECT telegram_id, step, service, doctor_id, date, time, phone, created_at FROM user_states WHERE telegram_id = ?", telegramID).
		Scan(&s.TelegramID, &s.Step, &s.Service, &s.DoctorID, &s.Date, &s.Time, &s.Phone, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error getting user state: %v", err)
	}
	return &s, nil
}

func SetUserState(db *sql.DB, s *models.UserState) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO user_states (telegram_id, step, service, doctor_id, date, time, phone, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, s.TelegramID, s.Step, s.Service, s.DoctorID, s.Date, s.Time, s.Phone, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("error setting user state: %v", err)
	}
	return nil
}

func ClearUserState(db *sql.DB, telegramID int64) error {
	_, err := db.Exec("DELETE FROM user_states WHERE telegram_id = ?", telegramID)
	if err != nil {
		return fmt.Errorf("error clearing user state: %v", err)
	}
	return nil
}

func GetAdminTelegramIDs(db *sql.DB) ([]int64, error) {
	rows, err := db.Query(`SELECT telegram_id FROM users WHERE is_admin = true AND telegram_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
