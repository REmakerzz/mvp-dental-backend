package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"

	"MVP_ChatBot/models"

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
	// Генерируем даты на следующие 14 дней
	var dates []string
	now := time.Now()
	for i := 0; i < 14; i++ {
		date := now.AddDate(0, 0, i)
		// Пропускаем выходные
		if date.Weekday() != time.Sunday && date.Weekday() != time.Saturday {
			dates = append(dates, date.Format("2006-01-02"))
		}
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
	// Генерируем доступное время (с 9:00 до 18:00, с интервалом в 30 минут)
	var availableTimes []string
	startTime := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
	endTime := time.Date(2000, 1, 1, 18, 0, 0, 0, time.UTC)

	for t := startTime; t.Before(endTime); t = t.Add(30 * time.Minute) {
		timeStr := t.Format("15:04")
		availableTimes = append(availableTimes, timeStr)
	}

	// Создаем клавиатуру со временем
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, time := range availableTimes {
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
		SELECT b.id, s.name, d.name as doctor_name, b.date, b.time, b.status
		FROM bookings b
		LEFT JOIN services s ON b.service_id = s.id
		LEFT JOIN doctors d ON b.doctor_id = d.id
		LEFT JOIN users u ON b.user_id = u.id
		WHERE u.telegram_id = ?
		AND s.name IS NOT NULL
		AND d.name IS NOT NULL
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
		var service, doctorName, date, time, status string
		if err := rows.Scan(&id, &service, &doctorName, &date, &time, &status); err != nil {
			continue // Skip invalid records
		}

		// Skip if any required field is empty
		if service == "" || doctorName == "" || date == "" || time == "" || status == "" {
			continue
		}

		text := fmt.Sprintf("Услуга: %s\nВрач: %s\nДата: %s\nВремя: %s\nСтатус: %s",
			service, doctorName, date, time, status)
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

// getAvailableDoctors возвращает список доступных врачей для услуги
func getAvailableDoctors(db *sql.DB, serviceName string) ([]models.Doctor, error) {
	rows, err := db.Query(`
		SELECT d.id, d.name, d.specialization, d.description, d.photo_url, d.created_at
		FROM doctors d
		JOIN doctor_services ds ON d.id = ds.doctor_id
		JOIN services s ON ds.service_id = s.id
		WHERE s.name = ?
	`, serviceName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var doctors []models.Doctor
	for rows.Next() {
		var d models.Doctor
		if err := rows.Scan(&d.ID, &d.Name, &d.Specialization, &d.Description, &d.PhotoURL, &d.CreatedAt); err != nil {
			return nil, err
		}
		doctors = append(doctors, d)
	}
	return doctors, nil
}

// getAvailableDates возвращает список доступных дат для записи
func getAvailableDates(db *sql.DB, doctorID int64) ([]string, error) {
	var dates []string
	now := time.Now()

	// Получаем расписание врача
	rows, err := db.Query(`
		SELECT weekday, start_time, end_time
		FROM doctor_schedules
		WHERE doctor_id = ?
	`, doctorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Создаем карту рабочих дней
	workingDays := make(map[time.Weekday]bool)
	for rows.Next() {
		var weekday int
		var startTime, endTime string
		if err := rows.Scan(&weekday, &startTime, &endTime); err != nil {
			return nil, err
		}
		workingDays[time.Weekday(weekday)] = true
	}

	// Генерируем даты на следующие 14 дней
	for i := 0; i < 14; i++ {
		date := now.AddDate(0, 0, i)
		if workingDays[date.Weekday()] {
			dates = append(dates, date.Format("2006-01-02"))
		}
	}

	return dates, nil
}

// getAvailableTimes возвращает список доступного времени для записи
func getAvailableTimes(db *sql.DB, doctorID int64, date string, serviceDuration int) ([]models.AvailableTimeSlot, error) {
	// Получаем расписание врача на этот день недели
	weekday := time.Now().Weekday()
	rows, err := db.Query(`
		SELECT start_time, end_time, break_start, break_end
		FROM doctor_schedules
		WHERE doctor_id = ? AND weekday = ?
	`, doctorID, weekday)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("no schedule found for doctor %d on weekday %d", doctorID, weekday)
	}

	var startTime, endTime, breakStart, breakEnd string
	if err := rows.Scan(&startTime, &endTime, &breakStart, &breakEnd); err != nil {
		return nil, err
	}

	// Получаем существующие записи на эту дату
	bookings, err := getBookingsForDate(db, doctorID, date)
	if err != nil {
		return nil, err
	}

	// Создаем карту занятого времени
	bookedTimes := make(map[string]bool)
	for _, booking := range bookings {
		bookedTimes[booking.Time] = true
	}

	// Генерируем доступное время
	var availableTimes []models.AvailableTimeSlot
	start, _ := models.ParseTime(startTime)
	end, _ := models.ParseTime(endTime)

	for t := start; t.Before(end); t = AddMinutes(t, 30) {
		timeStr := models.FormatTime(t)

		// Проверяем, не в перерыве ли это время
		if IsTimeInBreak(t, breakStart, breakEnd) {
			continue
		}

		// Проверяем, не занято ли это время
		if bookedTimes[timeStr] {
			continue
		}

		// Проверяем, достаточно ли времени для процедуры
		procedureEnd := AddMinutes(t, serviceDuration)
		if procedureEnd.After(end) {
			continue
		}

		// Проверяем, не пересекается ли с перерывом
		if IsTimeInBreak(procedureEnd, breakStart, breakEnd) {
			continue
		}

		// Проверяем, не пересекается ли с другими записями
		isAvailable := true
		for i := 0; i < serviceDuration; i += 30 {
			checkTime := AddMinutes(t, i)
			if bookedTimes[models.FormatTime(checkTime)] {
				isAvailable = false
				break
			}
		}

		if isAvailable {
			availableTimes = append(availableTimes, models.AvailableTimeSlot{
				Time:     timeStr,
				DoctorID: doctorID,
			})
		}
	}

	return availableTimes, nil
}

// getBookingsForDate возвращает все записи на указанную дату
func getBookingsForDate(db *sql.DB, doctorID int64, date string) ([]models.Booking, error) {
	rows, err := db.Query(`
		SELECT id, user_id, service_id, time, status
		FROM bookings
		WHERE doctor_id = ? AND date = ? AND status != 'Отменено'
	`, doctorID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		var b models.Booking
		if err := rows.Scan(&b.ID, &b.UserID, &b.ServiceID, &b.Time, &b.Status); err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	return bookings, nil
}

// handleDoctorSelection обрабатывает выбор врача
func handleDoctorSelection(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB, serviceName string) {
	doctors, err := getAvailableDoctors(db, serviceName)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при получении списка врачей.")
		bot.Send(msg)
		return
	}

	if len(doctors) == 0 {
		msg := tgbotapi.NewMessage(chatID, "Нет доступных врачей для этой услуги.")
		bot.Send(msg)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, doctor := range doctors {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s (%s)", doctor.Name, doctor.Specialization),
				fmt.Sprintf("doctor_%d", doctor.ID),
			),
		})
	}

	msg := tgbotapi.NewMessage(chatID, "Выберите врача:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	addCancelHint(&msg)
	bot.Send(msg)
}

func AddMinutes(t time.Time, minutes int) time.Time {
	return t.Add(time.Duration(minutes) * time.Minute)
}

func IsTimeInBreak(t time.Time, breakStart, breakEnd string) bool {
	// TODO: реализовать корректную проверку пересечения с перерывом
	return false
}
