package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nyaruka/phonenumbers"
)

var smsCodes = make(map[int64]string)

func addCancelHint(msg *tgbotapi.MessageConfig) {
	msg.Text += "\n\nДля отмены записи отправьте /cancel"
}

func handleBooking(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB) {
	services := getStomatologyServices(db)
	if len(services) == 0 {
		msg := tgbotapi.NewMessage(chatID, "Нет доступных стоматологических услуг. Обратитесь к администратору.")
		bot.Send(msg)
		return
	}
	msg := tgbotapi.NewMessage(chatID, "Выберите стоматологическую услугу:")
	keyboard := tgbotapi.NewReplyKeyboard()
	row := []tgbotapi.KeyboardButton{}
	for _, s := range services {
		row = append(row, tgbotapi.NewKeyboardButton(s))
	}
	keyboard.Keyboard = append(keyboard.Keyboard, row)
	msg.ReplyMarkup = keyboard
	addCancelHint(&msg)
	bot.Send(msg)
}

func getStomatologyServices(db *sql.DB) []string {
	rows, err := db.Query("SELECT name FROM services WHERE category = 'Стоматология'")
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		names = append(names, name)
	}
	return names
}

func makeDateInlineKeyboard() tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	now := time.Now()
	for i := 0; i < 7; i++ {
		d := now.AddDate(0, 0, i)
		btn := tgbotapi.NewInlineKeyboardButtonData(d.Format("02.01.2006"), d.Format("2006-01-02"))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func handleDateSelection(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Выберите дату:")
	msg.ReplyMarkup = makeDateInlineKeyboard()
	addCancelHint(&msg)
	bot.Send(msg)
}

func makeTimeInlineKeyboard() tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for h := 9; h < 18; h++ {
		row := []tgbotapi.InlineKeyboardButton{}
		for m := 0; m < 60; m += 30 {
			t := fmt.Sprintf("%02d:%02d", h, m)
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(t, t))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(row...))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func handleTimeSelection(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Выберите время:")
	msg.ReplyMarkup = makeTimeInlineKeyboard()
	addCancelHint(&msg)
	bot.Send(msg)
}

func sendSMSCode(phone string, userID int64) string {
	code := fmt.Sprintf("%04d", rand.Intn(10000))
	smsCodes[userID] = code
	// Здесь должна быть интеграция с реальным SMS-API
	fmt.Printf("[DEBUG] Отправлен код %s на номер %s\n", code, phone)
	return code
}

func handleSMSConfirmation(bot *tgbotapi.BotAPI, chatID int64, phone string, userID int64) {
	sendSMSCode(phone, userID)
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("На номер %s отправлен SMS-код. Введите его для подтверждения:", phone))
	addCancelHint(&msg)
	bot.Send(msg)
}

func handleBookingConfirmed(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Ваша запись подтверждена! Спасибо!")
	bot.Send(msg)
	sendMainMenu(bot, chatID)
}

func handleServices(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB) {
	msg := tgbotapi.NewMessage(chatID, "Доступные стоматологические услуги:\n")
	services := getStomatologyServices(db)
	if len(services) == 0 {
		msg.Text = "Нет доступных стоматологических услуг. Обратитесь к администратору."
		bot.Send(msg)
		return
	}
	for _, s := range services {
		msg.Text += "- " + s + "\n"
	}
	bot.Send(msg)
}

func handleContacts(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Контакты салона:\nТелефон: +7 900 000-00-00\nАдрес: п.Агинское, ул. Комсомольская, 1")
	bot.Send(msg)
}

func handleMyBookings(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB, telegramID int64) {
	rows, err := db.Query(`
		SELECT b.id, s.name, b.date, b.time, b.status
		FROM bookings b
		LEFT JOIN services s ON b.service_id = s.id
		LEFT JOIN users u ON b.user_id = u.id
		WHERE u.telegram_id = ?
		AND s.name IS NOT NULL
		AND b.date IS NOT NULL
		AND b.time IS NOT NULL
		AND b.status IS NOT NULL
		ORDER BY b.date DESC, b.time DESC
		LIMIT 10
	`, telegramID)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении записей."))
		return
	}
	defer rows.Close()

	msg := tgbotapi.NewMessage(chatID, "Ваши записи:")
	bot.Send(msg)

	found := false
	for rows.Next() {
		var id int
		var service, date, time, status string
		if err := rows.Scan(&id, &service, &date, &time, &status); err != nil {
			continue // Skip invalid records
		}

		// Skip if any required field is empty
		if service == "" || date == "" || time == "" || status == "" {
			continue
		}

		text := fmt.Sprintf("Услуга: %s\nДата: %s\nВремя: %s\nСтатус: %s", service, date, time, status)
		msg := tgbotapi.NewMessage(chatID, text)
		if status == "Подтверждено" {
			btn := tgbotapi.NewInlineKeyboardButtonData("❌ Отменить", fmt.Sprintf("cancel_%d", id))
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		}
		bot.Send(msg)
		found = true
	}
	if !found {
		bot.Send(tgbotapi.NewMessage(chatID, "У вас нет записей."))
	}
}

func handleCancelBooking(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Ваша запись отменена.")
	bot.Send(msg)
}

func validatePhone(phone string) bool {
	log.Printf("Validating phone number: %q", phone)
	num, err := phonenumbers.Parse(phone, "RU")
	if err != nil {
		log.Printf("Error parsing phone number: %v", err)
		return false
	}
	isValid := phonenumbers.IsValidNumber(num)
	log.Printf("Phone number validation result: %v", isValid)
	return isValid
}
