package handlers

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Состояния пользователя в процессе бронирования
type BookingState struct {
	ServiceID  int64
	Date       string
	Time       string
	DoctorID   int64
	LastUpdate time.Time
}

var userStates = make(map[int64]*BookingState)

func startBookingProcess(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB, userID int64) {
	// Получаем список услуг
	rows, err := db.Query(`
		SELECT id, name, category, duration, price
		FROM services
		ORDER BY category, name
	`)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка при получении списка услуг")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	var keyboard [][]tgbotapi.InlineKeyboardButton
	currentCategory := ""

	for rows.Next() {
		var id int64
		var name, category string
		var duration int
		var price float64
		if err := rows.Scan(&id, &name, &category, &duration, &price); err != nil {
			continue
		}

		if category != currentCategory {
			if currentCategory != "" {
				keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{})
			}
			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(category, fmt.Sprintf("category_%s", category)),
			})
			currentCategory = category
		}

		buttonText := fmt.Sprintf("%s (%d мин., %.2f ₽)", name, duration, price)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(buttonText, fmt.Sprintf("service_%d", id)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, "Выберите услугу:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)
}

func handleBookingCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, db *sql.DB) {
	// Обрабатываем callback в зависимости от его типа
	switch {
	case strings.HasPrefix(callback.Data, "service_"):
		handleServiceSelection(bot, callback, db)

	case strings.HasPrefix(callback.Data, "date_"):
		handleDateSelection(bot, callback, db)

	case strings.HasPrefix(callback.Data, "time_"):
		handleTimeSelection(bot, callback, db)

	case strings.HasPrefix(callback.Data, "doctor_"):
		handleDoctorSelection(bot, callback, db)

	case callback.Data == "confirm_booking":
		handleBookingConfirmation(bot, callback, db)

	case callback.Data == "cancel_booking":
		handleBookingCancellation(bot, callback)
	}
}

func handleServiceSelection(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, db *sql.DB) {
	// Извлекаем ID услуги из callback data
	var serviceID int64
	fmt.Sscanf(callback.Data[8:], "%d", &serviceID)

	// Получаем информацию об услуге
	var serviceName string
	err := db.QueryRow("SELECT name FROM services WHERE id = ?", serviceID).Scan(&serviceName)
	if err != nil {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Ошибка при получении информации об услуге")
		bot.Send(msg)
		return
	}

	// Получаем доступные даты (следующие 14 дней)
	var keyboard [][]tgbotapi.InlineKeyboardButton
	today := time.Now()
	for i := 0; i < 14; i++ {
		date := today.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")
		displayStr := date.Format("02.01.2006")
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(displayStr, fmt.Sprintf("date_%s", dateStr)),
		})
	}

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("Выбрана услуга: %s\n\nВыберите дату:", serviceName))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)

	// Отвечаем на callback
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(callbackResponse)
}

func handleDateSelection(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, db *sql.DB) {
	// Извлекаем дату из callback data
	dateStr := callback.Data[5:]

	// Получаем доступное время для выбранной даты
	var keyboard [][]tgbotapi.InlineKeyboardButton
	startTime := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC) // Начало рабочего дня
	endTime := time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC)  // Конец рабочего дня

	for t := startTime; t.Before(endTime); t = t.Add(30 * time.Minute) {
		timeStr := t.Format("15:04")
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(timeStr, fmt.Sprintf("time_%s_%s", dateStr, timeStr)),
		})
	}

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("Выбрана дата: %s\n\nВыберите время:", dateStr))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)

	// Отвечаем на callback
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(callbackResponse)
}

func handleTimeSelection(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, db *sql.DB) {
	// Извлекаем дату и время из callback data
	parts := strings.Split(callback.Data[5:], "_")
	if len(parts) != 2 {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Ошибка при обработке выбора времени")
		bot.Send(msg)
		return
	}
	dateStr := parts[0]
	timeStr := parts[1]

	// Получаем список доступных врачей
	rows, err := db.Query(`
		SELECT id, name, specialization
		FROM doctors
		WHERE id NOT IN (
			SELECT doctor_id
			FROM bookings
			WHERE date = ? AND time = ? AND status = 'Подтверждено'
		)
		ORDER BY name
	`, dateStr, timeStr)
	if err != nil {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Ошибка при получении списка врачей")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for rows.Next() {
		var id int64
		var name, specialization string
		if err := rows.Scan(&id, &name, &specialization); err != nil {
			continue
		}

		buttonText := fmt.Sprintf("%s (%s)", name, specialization)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(buttonText, fmt.Sprintf("doctor_%d_%s_%s", id, dateStr, timeStr)),
		})
	}

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("Выбрано время: %s\n\nВыберите врача:", timeStr))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)

	// Отвечаем на callback
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(callbackResponse)
}

func handleDoctorSelection(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, db *sql.DB) {
	// Извлекаем данные из callback data
	parts := strings.Split(callback.Data[8:], "_")
	if len(parts) != 3 {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Ошибка при обработке выбора врача")
		bot.Send(msg)
		return
	}
	doctorID := parts[0]
	dateStr := parts[1]
	timeStr := parts[2]

	// Получаем информацию о враче
	var doctorName, specialization string
	err := db.QueryRow("SELECT name, specialization FROM doctors WHERE id = ?", doctorID).Scan(&doctorName, &specialization)
	if err != nil {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Ошибка при получении информации о враче")
		bot.Send(msg)
		return
	}

	// Создаем клавиатуру подтверждения
	keyboard := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("Подтвердить", "confirm_booking"),
			tgbotapi.NewInlineKeyboardButtonData("Отменить", "cancel_booking"),
		},
	}

	// Формируем сообщение с подтверждением
	confirmationText := fmt.Sprintf(
		"Пожалуйста, подтвердите запись:\n\n"+
			"Дата: %s\n"+
			"Время: %s\n"+
			"Врач: %s (%s)",
		dateStr, timeStr, doctorName, specialization,
	)

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, confirmationText)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)

	// Отвечаем на callback
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(callbackResponse)
}

func handleBookingConfirmation(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, db *sql.DB) {
	// TODO: Реализовать сохранение записи в базу данных
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Запись успешно создана!")
	bot.Send(msg)

	// Отвечаем на callback
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(callbackResponse)
}

func handleBookingCancellation(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Запись отменена")
	bot.Send(msg)

	// Отвечаем на callback
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(callbackResponse)
}
