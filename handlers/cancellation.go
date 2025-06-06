package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func startCancellationProcessV2(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB, userID int64) {
	// Получаем активные записи пользователя
	rows, err := db.Query(`
		SELECT b.id, b.date, b.time, s.name as service_name, d.name as doctor_name
		FROM bookings b
		JOIN services s ON b.service_id = s.id
		JOIN doctors d ON b.doctor_id = d.id
		WHERE b.user_id = ? AND b.date >= ? AND b.status = 'Подтверждено'
		ORDER BY b.date, b.time
	`, userID, time.Now().Format("2006-01-02"))
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка при получении списка записей")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	var bookings []struct {
		ID          int64
		Date        string
		Time        string
		ServiceName string
		DoctorName  string
	}

	for rows.Next() {
		var b struct {
			ID          int64
			Date        string
			Time        string
			ServiceName string
			DoctorName  string
		}
		if err := rows.Scan(&b.ID, &b.Date, &b.Time, &b.ServiceName, &b.DoctorName); err == nil {
			bookings = append(bookings, b)
		}
	}

	if len(bookings) == 0 {
		msg := tgbotapi.NewMessage(chatID, "У вас нет активных записей для отмены")
		bot.Send(msg)
		return
	}

	// Создаем клавиатуру с записями
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, booking := range bookings {
		buttonText := fmt.Sprintf("%s - %s (%s, %s)", booking.Date, booking.Time, booking.ServiceName, booking.DoctorName)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(buttonText, fmt.Sprintf("cancel_%d", booking.ID)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, "Выберите запись для отмены:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)
}

func handleCancellationCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, db *sql.DB) {
	// Извлекаем ID записи из callback data
	var bookingID int64
	fmt.Sscanf(callback.Data[7:], "%d", &bookingID)

	// Получаем информацию о записи
	var date, time, serviceName, doctorName string
	err := db.QueryRow(`
		SELECT b.date, b.time, s.name, d.name
		FROM bookings b
		JOIN services s ON b.service_id = s.id
		JOIN doctors d ON b.doctor_id = d.id
		WHERE b.id = ?
	`, bookingID).Scan(&date, &time, &serviceName, &doctorName)
	if err != nil {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Ошибка при получении данных о записи")
		bot.Send(msg)
		return
	}

	// Создаем клавиатуру подтверждения
	keyboard := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("Подтвердить отмену", fmt.Sprintf("confirm_cancel_%d", bookingID)),
			tgbotapi.NewInlineKeyboardButtonData("Отменить", "cancel_cancellation"),
		},
	}

	// Формируем сообщение с подтверждением
	confirmationText := fmt.Sprintf(
		"Вы действительно хотите отменить запись?\n\n"+
			"Услуга: %s\n"+
			"Дата: %s\n"+
			"Время: %s\n"+
			"Врач: %s",
		serviceName, date, time, doctorName,
	)

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, confirmationText)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)

	// Отвечаем на callback
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(callbackResponse)
}

func confirmCancellation(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, db *sql.DB) {
	// Извлекаем ID записи из callback data
	var bookingID int64
	fmt.Sscanf(callback.Data[15:], "%d", &bookingID)

	// Обновляем статус записи
	_, err := db.Exec("UPDATE bookings SET status = 'Отменено' WHERE id = ?", bookingID)
	if err != nil {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Ошибка при отмене записи")
		bot.Send(msg)
		return
	}

	// Получаем информацию о записи для уведомления
	var date, time, serviceName, doctorName string
	err = db.QueryRow(`
		SELECT b.date, b.time, s.name, d.name
		FROM bookings b
		JOIN services s ON b.service_id = s.id
		JOIN doctors d ON b.doctor_id = d.id
		WHERE b.id = ?
	`, bookingID).Scan(&date, &time, &serviceName, &doctorName)
	if err != nil {
		log.Printf("Error getting booking info: %v", err)
	}

	// Отправляем подтверждение
	confirmationText := fmt.Sprintf(
		"✅ Запись успешно отменена!\n\n"+
			"Услуга: %s\n"+
			"Дата: %s\n"+
			"Время: %s\n"+
			"Врач: %s",
		serviceName, date, time, doctorName,
	)

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, confirmationText)
	bot.Send(msg)

	// Отвечаем на callback
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(callbackResponse)
}
