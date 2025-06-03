package main

import (
	"database/sql"
	"io/ioutil"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type UserState struct {
	TelegramID int64
	Step       string
	Service    string
	Date       string
	Time       string
	Phone      string
}

func InitDB(path string, migrationsPath string) *sql.DB {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}

	migrations, err := ioutil.ReadFile(migrationsPath)
	if err != nil {
		log.Fatalf("failed to read migrations: %v", err)
	}

	_, err = db.Exec(string(migrations))
	if err != nil {
		log.Fatalf("failed to apply migrations: %v", err)
	}

	return db
}

func GetUserState(db *sql.DB, telegramID int64) (*UserState, error) {
	row := db.QueryRow(`SELECT telegram_id, step, service, date, time, phone FROM user_states WHERE telegram_id = ?`, telegramID)
	var s UserState
	err := row.Scan(&s.TelegramID, &s.Step, &s.Service, &s.Date, &s.Time, &s.Phone)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func SetUserState(db *sql.DB, s *UserState) error {
	_, err := db.Exec(`INSERT INTO user_states (telegram_id, step, service, date, time, phone) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(telegram_id) DO UPDATE SET step=excluded.step, service=excluded.service, date=excluded.date, time=excluded.time, phone=excluded.phone`,
		s.TelegramID, s.Step, s.Service, s.Date, s.Time, s.Phone)
	return err
}

func ClearUserState(db *sql.DB, telegramID int64) error {
	_, err := db.Exec(`DELETE FROM user_states WHERE telegram_id = ?`, telegramID)
	return err
}

func GetAdminTelegramIDs(db *sql.DB) ([]int64, error) {
	rows, err := db.Query(`SELECT telegram_id FROM users WHERE is_admin = 1 AND telegram_id IS NOT NULL`)
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
 