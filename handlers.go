package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"

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

func handleDateSelection(bot *tgbotapi.BotAPI, chatID int64) {
	// Получаем доступные даты
	resp, err := http.Get("https://mvp-dental-backend.onrender.com/api/available_dates")
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка при получении доступных дат")
		bot.Send(msg)
		return
	}
	defer resp.Body.Close()

	var dates []string
	if err := json.NewDecoder(resp.Body).Decode(&dates); err != nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка при обработке дат")
		bot.Send(msg)
		return
	}

	// Создаем клавиатуру с датами
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, date := range dates {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(date, date),
		})
	}

	msg := tgbotapi.NewMessage(chatID, "Выберите дату:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	addCancelHint(&msg)
	bot.Send(msg)
}

func handleTimeSelection(bot *tgbotapi.BotAPI, chatID int64, date string) {
	// Получаем доступное время
	resp, err := http.Get(fmt.Sprintf("https://mvp-dental-backend.onrender.com/api/available_times?date=%s", date))
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка при получении доступного времени")
		bot.Send(msg)
		return
	}
	defer resp.Body.Close()

	var times []string
	if err := json.NewDecoder(resp.Body).Decode(&times); err != nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка при обработке времени")
		bot.Send(msg)
		return
	}

	// Создаем клавиатуру со временем
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, time := range times {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(time, time),
		})
	}

	msg := tgbotapi.NewMessage(chatID, "Выберите время:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
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
