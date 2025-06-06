package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ProcessBotUpdates(bot *tgbotapi.BotAPI, updates tgbotapi.UpdatesChannel, db *sql.DB) {
	for update := range updates {
		if update.Message == nil && update.CallbackQuery == nil {
			continue
		}

		// Обрабатываем callback queries
		if update.CallbackQuery != nil {
			handleCallbackQuery(bot, update.CallbackQuery, db)
			continue
		}

		// Получаем или создаем пользователя
		userID, err := getOrCreateUser(db, update.Message.From.ID, update.Message.From.UserName)
		if err != nil {
			log.Printf("Error getting/creating user: %v", err)
			continue
		}

		// Обрабатываем команды
		if update.Message.IsCommand() {
			handleCommand(bot, update.Message, db, userID)
			continue
		}

		// Обрабатываем текстовые сообщения
		handleTextMessage(bot, update.Message, db, userID)
	}
}

func getOrCreateUser(db *sql.DB, telegramID int64, username string) (int64, error) {
	var userID int64
	err := db.QueryRow("SELECT id FROM users WHERE telegram_id = ?", telegramID).Scan(&userID)
	if err == sql.ErrNoRows {
		result, err := db.Exec("INSERT INTO users (telegram_id, username) VALUES (?, ?)", telegramID, username)
		if err != nil {
			return 0, err
		}
		userID, err = result.LastInsertId()
		if err != nil {
			return 0, err
		}
	} else if err != nil {
		return 0, err
	}
	return userID, nil
}

func handleCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, db *sql.DB, userID int64) {
	switch message.Command() {
	case "start":
		msg := tgbotapi.NewMessage(message.Chat.ID, "Добро пожаловать! Я помогу вам записаться на прием. Используйте /help для получения списка доступных команд.")
		bot.Send(msg)

	case "help":
		helpText := `Доступные команды:
/start - Начать работу с ботом
/help - Показать это сообщение
/services - Показать список услуг
/book - Записаться на прием
/my_bookings - Показать мои записи
/cancel - Отменить запись`
		msg := tgbotapi.NewMessage(message.Chat.ID, helpText)
		bot.Send(msg)

	case "services":
		showServices(bot, message.Chat.ID, db)

	case "book":
		startBookingProcess(bot, message.Chat.ID, db, userID)

	case "my_bookings":
		showUserBookings(bot, message.Chat.ID, db, userID)

	case "cancel":
		startCancellationProcess(bot, message.Chat.ID, db, userID)

	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "Неизвестная команда. Используйте /help для получения списка доступных команд.")
		bot.Send(msg)
	}
}

func handleTextMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, db *sql.DB, userID int64) {
	// TODO: Реализовать обработку текстовых сообщений для процесса бронирования
	msg := tgbotapi.NewMessage(message.Chat.ID, "Пожалуйста, используйте команды для взаимодействия с ботом. /help для получения списка команд.")
	bot.Send(msg)
}

func handleCallbackQuery(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, db *sql.DB) {
	// Обрабатываем callback в зависимости от его типа
	switch {
	case strings.HasPrefix(callback.Data, "service_") ||
		strings.HasPrefix(callback.Data, "date_") ||
		strings.HasPrefix(callback.Data, "time_") ||
		strings.HasPrefix(callback.Data, "doctor_") ||
		callback.Data == "confirm_booking" ||
		callback.Data == "cancel_booking":
		handleBookingCallback(bot, callback, db)

	case strings.HasPrefix(callback.Data, "cancel_") ||
		strings.HasPrefix(callback.Data, "confirm_cancel_") ||
		callback.Data == "cancel_cancellation":
		handleCancellationCallback(bot, callback, db)
	}
}

func showServices(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB) {
	rows, err := db.Query(`
		SELECT name, category, duration, price
		FROM services
		ORDER BY category, name
	`)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка при получении списка услуг")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	var services []string
	currentCategory := ""

	for rows.Next() {
		var name, category string
		var duration int
		var price float64
		if err := rows.Scan(&name, &category, &duration, &price); err != nil {
			continue
		}

		if category != currentCategory {
			if currentCategory != "" {
				services = append(services, "")
			}
			services = append(services, fmt.Sprintf("*%s*:", category))
			currentCategory = category
		}

		services = append(services, fmt.Sprintf("• %s\n  Длительность: %d мин.\n  Цена: %.2f ₽", name, duration, price))
	}

	if len(services) == 0 {
		msg := tgbotapi.NewMessage(chatID, "Список услуг пуст")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, strings.Join(services, "\n"))
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

func showUserBookings(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB, userID int64) {
	rows, err := db.Query(`
		SELECT b.id, b.date, b.time, s.name as service_name, b.status
		FROM bookings b
		JOIN services s ON b.service_id = s.id
		WHERE b.user_id = ?
		ORDER BY b.date DESC, b.time DESC
	`, userID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка при получении списка записей")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	var bookings []string
	for rows.Next() {
		var id int64
		var date, time, serviceName, status string
		if err := rows.Scan(&id, &date, &time, &serviceName, &status); err != nil {
			continue
		}

		bookings = append(bookings, fmt.Sprintf(
			"• %s\n  Дата: %s\n  Время: %s\n  Статус: %s",
			serviceName, date, time, status,
		))
	}

	if len(bookings) == 0 {
		msg := tgbotapi.NewMessage(chatID, "У вас нет активных записей")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, strings.Join(bookings, "\n\n"))
	bot.Send(msg)
}

func startCancellationProcess(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB, userID int64) {
	// TODO: Реализовать процесс отмены записи
	msg := tgbotapi.NewMessage(chatID, "Функция отмены записи в разработке")
	bot.Send(msg)
}
