package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func init() {
	// Инициализируем генератор случайных чисел
	rand.Seed(time.Now().UnixNano())
}

func ProcessBotUpdates(bot *tgbotapi.BotAPI, updates tgbotapi.UpdatesChannel, db *sql.DB) {
	for update := range updates {
		// Обрабатываем callback queries
		if update.CallbackQuery != nil {
			handleCallbackQuery(bot, update.CallbackQuery, db)
			continue
		}

		// Пропускаем обновления без сообщений
		if update.Message == nil {
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
		handleTextMessage(update, bot, db)
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

func handleTextMessage(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	if update.Message == nil {
		return
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

	// Получаем состояние пользователя
	var state string
	err := db.QueryRow("SELECT state FROM users WHERE telegram_id = ?", update.Message.Chat.ID).Scan(&state)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error getting user state: %v", err)
		msg.Text = "Произошла ошибка. Попробуйте позже."
		bot.Send(msg)
		return
	}

	switch state {
	case "waiting_for_phone":
		// Проверяем формат номера телефона
		phone := update.Message.Text
		if !isValidPhone(phone) {
			msg.Text = "Неверный формат номера телефона. Пожалуйста, введите номер в формате +7XXXXXXXXXX"
			bot.Send(msg)
			return
		}

		// Обновляем номер телефона
		_, err = db.Exec("UPDATE users SET phone = ? WHERE telegram_id = ?", phone, update.Message.Chat.ID)
		if err != nil {
			log.Printf("Error updating phone: %v", err)
			msg.Text = "Произошла ошибка при сохранении номера телефона. Попробуйте позже."
			bot.Send(msg)
			return
		}

		// Генерируем и отправляем код подтверждения
		code := generateConfirmationCode()
		_, err = db.Exec("UPDATE users SET confirmation_code = ?, code_expires_at = datetime('now', '+5 minutes') WHERE telegram_id = ?", code, update.Message.Chat.ID)
		if err != nil {
			log.Printf("Error saving confirmation code: %v", err)
			msg.Text = "Произошла ошибка при генерации кода. Попробуйте позже."
			bot.Send(msg)
			return
		}

		// TODO: Отправить код через SMS
		msg.Text = fmt.Sprintf("Ваш код подтверждения: %s\nКод действителен 5 минут.", code)
		bot.Send(msg)

		// Обновляем состояние
		_, err = db.Exec("UPDATE users SET state = 'waiting_for_code' WHERE telegram_id = ?", update.Message.Chat.ID)
		if err != nil {
			log.Printf("Error updating state: %v", err)
		}

	case "waiting_for_code":
		// Проверяем код подтверждения
		code := update.Message.Text
		var storedCode string
		var expiresAt time.Time
		err := db.QueryRow("SELECT confirmation_code, code_expires_at FROM users WHERE telegram_id = ?", update.Message.Chat.ID).Scan(&storedCode, &expiresAt)
		if err != nil {
			log.Printf("Error getting confirmation code: %v", err)
			msg.Text = "Произошла ошибка. Попробуйте позже."
			bot.Send(msg)
			return
		}

		if code != storedCode {
			msg.Text = "Неверный код подтверждения. Пожалуйста, проверьте код и попробуйте снова."
			bot.Send(msg)
			return
		}

		if time.Now().After(expiresAt) {
			msg.Text = "Срок действия кода истек. Пожалуйста, запросите новый код."
			bot.Send(msg)
			return
		}

		// Обновляем состояние
		_, err = db.Exec("UPDATE users SET state = 'ready', confirmation_code = NULL, code_expires_at = NULL WHERE telegram_id = ?", update.Message.Chat.ID)
		if err != nil {
			log.Printf("Error updating state: %v", err)
			msg.Text = "Произошла ошибка. Попробуйте позже."
			bot.Send(msg)
			return
		}

		msg.Text = "Номер телефона успешно подтвержден! Теперь вы можете использовать все функции бота."
		bot.Send(msg)

	default:
		msg.Text = "Пожалуйста, используйте команды для взаимодействия с ботом. /help для получения списка команд."
		bot.Send(msg)
	}
}

func handleCallbackQuery(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, db *sql.DB) {
	if callback == nil || callback.Data == "" {
		log.Printf("Invalid callback query received")
		return
	}

	// Обрабатываем callback в зависимости от его типа
	switch {
	case strings.HasPrefix(callback.Data, "service_"):
		// Пользователь выбрал услугу
		serviceID := strings.TrimPrefix(callback.Data, "service_")
		startDateSelection(bot, callback.Message.Chat.ID, serviceID, db)

	case strings.HasPrefix(callback.Data, "date_"):
		// Пользователь выбрал дату
		parts := strings.Split(callback.Data, "_")
		if len(parts) != 3 {
			callbackConfig := tgbotapi.NewCallback(callback.ID, "Ошибка: неверный формат данных")
			bot.Request(callbackConfig)
			return
		}
		serviceID := parts[1]
		date := parts[2]
		startTimeSelection(bot, callback.Message.Chat.ID, serviceID, date, db)

	case strings.HasPrefix(callback.Data, "time_"):
		// Пользователь выбрал время
		parts := strings.Split(callback.Data, "_")
		if len(parts) != 4 {
			callbackConfig := tgbotapi.NewCallback(callback.ID, "Ошибка: неверный формат данных")
			bot.Request(callbackConfig)
			return
		}
		serviceID := parts[1]
		date := parts[2]
		time := parts[3]
		confirmBooking(bot, callback.Message.Chat.ID, serviceID, date, time, db)

	case strings.HasPrefix(callback.Data, "confirm_"):
		// Пользователь подтвердил запись
		parts := strings.Split(callback.Data, "_")
		if len(parts) != 4 {
			callbackConfig := tgbotapi.NewCallback(callback.ID, "Ошибка: неверный формат данных")
			bot.Request(callbackConfig)
			return
		}
		serviceID := parts[1]
		date := parts[2]
		time := parts[3]
		createBooking(bot, callback.Message.Chat.ID, serviceID, date, time, db)

	case strings.HasPrefix(callback.Data, "cancel_"):
		// Пользователь отменил запись
		bookingID := strings.TrimPrefix(callback.Data, "cancel_")
		cancelBooking(bot, callback.Message.Chat.ID, bookingID, db)

	default:
		// Неизвестный тип callback
		callbackConfig := tgbotapi.NewCallback(callback.ID, "Неизвестная команда")
		bot.Request(callbackConfig)
	}

	// Отвечаем на callback query
	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	bot.Request(callbackConfig)
}

func showServices(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB) {
	// Получаем список услуг
	rows, err := db.Query(`
		SELECT id, name, category, duration, price
		FROM services
		ORDER BY category, name
	`)
	if err != nil {
		log.Printf("Error getting services: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при получении списка услуг. Попробуйте позже.")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	// Группируем услуги по категориям
	servicesByCategory := make(map[string][]struct {
		ID       int64
		Name     string
		Duration int
		Price    float64
	})

	for rows.Next() {
		var service struct {
			ID       int64
			Name     string
			Category string
			Duration int
			Price    float64
		}
		if err := rows.Scan(&service.ID, &service.Name, &service.Category, &service.Duration, &service.Price); err != nil {
			log.Printf("Error scanning service: %v", err)
			continue
		}
		servicesByCategory[service.Category] = append(servicesByCategory[service.Category], struct {
			ID       int64
			Name     string
			Duration int
			Price    float64
		}{
			ID:       service.ID,
			Name:     service.Name,
			Duration: service.Duration,
			Price:    service.Price,
		})
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating services: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при получении списка услуг. Попробуйте позже.")
		bot.Send(msg)
		return
	}

	// Создаем сообщение с услугами
	var text strings.Builder
	text.WriteString("Доступные услуги:\n\n")

	for category, services := range servicesByCategory {
		text.WriteString(fmt.Sprintf("📌 %s:\n", category))
		for _, service := range services {
			text.WriteString(fmt.Sprintf("• %s (%d мин.) - %.2f ₽\n", service.Name, service.Duration, service.Price))
		}
		text.WriteString("\n")
	}

	// Создаем клавиатуру с кнопками для каждой услуги
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, services := range servicesByCategory {
		// Добавляем кнопки для каждой услуги
		for _, service := range services {
			callbackData := fmt.Sprintf("service_%d", service.ID)
			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
				{
					Text:         fmt.Sprintf("%s - %.2f ₽", service.Name, service.Price),
					CallbackData: &callbackData,
				},
			})
		}
	}

	msg := tgbotapi.NewMessage(chatID, text.String())
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending services message: %v", err)
	}
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

func isValidPhone(phone string) bool {
	// Проверяем, что номер начинается с +7 и содержит 11 цифр
	if len(phone) != 12 || !strings.HasPrefix(phone, "+7") {
		return false
	}

	// Проверяем, что остальные символы - цифры
	for _, c := range phone[2:] {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

func generateConfirmationCode() string {
	// Генерируем 6-значный код
	code := ""
	for i := 0; i < 6; i++ {
		code += fmt.Sprintf("%d", rand.Intn(10))
	}
	return code
}

func startDateSelection(bot *tgbotapi.BotAPI, chatID int64, serviceID string, db *sql.DB) {
	// Получаем доступные даты на ближайшие 14 дней
	var dates []string
	rows, err := db.Query(`
		SELECT date
		FROM bookings
		WHERE date BETWEEN date('now') AND date('now', '+14 days')
		GROUP BY date
		HAVING COUNT(*) < 8
		ORDER BY date
	`)
	if err != nil {
		log.Printf("Error getting available dates: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при получении доступных дат. Попробуйте позже.")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err == nil {
			dates = append(dates, date)
		}
	}

	// Создаем клавиатуру с датами
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, date := range dates {
		callbackData := fmt.Sprintf("date_%s_%s", serviceID, date)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			{
				Text:         date,
				CallbackData: &callbackData,
			},
		})
	}

	msg := tgbotapi.NewMessage(chatID, "Выберите удобную дату:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)
}

func startTimeSelection(bot *tgbotapi.BotAPI, chatID int64, serviceID, date string, db *sql.DB) {
	// Получаем доступные времена для выбранной даты
	rows, err := db.Query(`
		SELECT time
		FROM bookings
		WHERE date = ?
		ORDER BY time
	`, date)
	if err != nil {
		log.Printf("Error getting booked times: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при получении доступного времени. Попробуйте позже.")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	// Создаем карту занятых времен
	bookedTimes := make(map[string]bool)
	for rows.Next() {
		var time string
		if err := rows.Scan(&time); err == nil {
			bookedTimes[time] = true
		}
	}

	// Генерируем временные слоты с 9:00 до 18:00 с интервалом в 30 минут
	start, _ := time.Parse("15:04", "09:00")
	end, _ := time.Parse("15:04", "18:00")
	var keyboard [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for t := start; t.Before(end); t = t.Add(30 * time.Minute) {
		timeStr := t.Format("15:04")
		if !bookedTimes[timeStr] {
			callbackData := fmt.Sprintf("time_%s_%s_%s", serviceID, date, timeStr)
			row = append(row, tgbotapi.InlineKeyboardButton{
				Text:         timeStr,
				CallbackData: &callbackData,
			})
			if len(row) == 2 {
				keyboard = append(keyboard, row)
				row = []tgbotapi.InlineKeyboardButton{}
			}
		}
	}
	if len(row) > 0 {
		keyboard = append(keyboard, row)
	}

	msg := tgbotapi.NewMessage(chatID, "Выберите удобное время:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)
}

func confirmBooking(bot *tgbotapi.BotAPI, chatID int64, serviceID, date, time string, db *sql.DB) {
	// Получаем информацию об услуге
	var service struct {
		Name     string
		Duration int
		Price    float64
	}
	err := db.QueryRow("SELECT name, duration, price FROM services WHERE id = ?", serviceID).Scan(&service.Name, &service.Duration, &service.Price)
	if err != nil {
		log.Printf("Error getting service info: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при получении информации об услуге. Попробуйте позже.")
		bot.Send(msg)
		return
	}

	// Создаем клавиатуру для подтверждения
	confirmData := fmt.Sprintf("confirm_%s_%s_%s", serviceID, date, time)
	cancelData := "cancel"
	keyboard := [][]tgbotapi.InlineKeyboardButton{
		{
			{
				Text:         "Подтвердить",
				CallbackData: &confirmData,
			},
			{
				Text:         "Отменить",
				CallbackData: &cancelData,
			},
		},
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
		"Подтвердите запись:\n\nУслуга: %s\nДата: %s\nВремя: %s\nДлительность: %d мин.\nСтоимость: %.2f ₽",
		service.Name, date, time, service.Duration, service.Price,
	))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)
}

func createBooking(bot *tgbotapi.BotAPI, chatID int64, serviceID, date, time string, db *sql.DB) {
	// Получаем ID пользователя
	var userID int64
	err := db.QueryRow("SELECT id FROM users WHERE telegram_id = ?", chatID).Scan(&userID)
	if err != nil {
		log.Printf("Error getting user ID: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при создании записи. Попробуйте позже.")
		bot.Send(msg)
		return
	}

	// Создаем запись
	_, err = db.Exec(`
		INSERT INTO bookings (user_id, service_id, date, time)
		VALUES (?, ?, ?, ?)
	`, userID, serviceID, date, time)
	if err != nil {
		log.Printf("Error creating booking: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при создании записи. Попробуйте позже.")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, "Запись успешно создана! Мы свяжемся с вами для подтверждения.")
	bot.Send(msg)
}

func cancelBooking(bot *tgbotapi.BotAPI, chatID int64, bookingID string, db *sql.DB) {
	// Отменяем запись
	_, err := db.Exec("UPDATE bookings SET status = 'Отменена' WHERE id = ?", bookingID)
	if err != nil {
		log.Printf("Error canceling booking: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при отмене записи. Попробуйте позже.")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, "Запись успешно отменена.")
	bot.Send(msg)
}
