package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"MVP_ChatBot/models"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
)

func isPortInUse(port string) bool {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return true
	}
	ln.Close()
	return false
}

func main() {
	// Создаем файл блокировки
	lockFile := "bot.lock"

	// Пытаемся создать файл блокировки
	lock, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		log.Printf("Another instance is already running (lock file exists)")
		return
	}
	defer os.Remove(lockFile) // Удаляем файл блокировки при завершении
	defer lock.Close()

	// Проверяем порт
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if isPortInUse(port) {
		log.Printf("Port %s is already in use, another instance is running", port)
		return
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// Пробуем получить обновления с повторными попытками
	updates := bot.GetUpdatesChan(u)

	// Используем локальную базу данных
	dbPath := "mvp_chatbot.db"
	log.Printf("Using database at: %s", dbPath)
	db := InitDB(dbPath, "migrations.sql")

	// Запускаем веб-сервер в отдельной горутине
	go func() {
		r := gin.Default()

		// Настраиваем CORS
		config := cors.DefaultConfig()
		config.AllowAllOrigins = true
		config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
		config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
		config.AllowCredentials = true
		r.Use(cors.New(config))

		r.LoadHTMLGlob("templates/*.html")

		store := cookie.NewStore([]byte("secret"))
		r.Use(sessions.Sessions("adminsession", store))

		// Редирект с корневого пути на админку
		r.GET("/", func(c *gin.Context) {
			c.Redirect(302, "/admin/login")
		})

		// Админка
		r.GET("/admin/login", adminLoginHandler())
		r.POST("/admin/login", adminLoginHandler())
		r.GET("/admin/bookings", adminAuthMiddleware(), adminBookingsHandler(db))
		r.GET("/admin/logout", adminLogoutHandler())
		r.GET("/admin/services", adminAuthMiddleware(), adminServicesHandler(db))
		r.POST("/admin/services", adminAuthMiddleware(), adminServicesHandler(db))
		r.GET("/admin/export_pdf", adminAuthMiddleware(), adminExportPDFHandler(db))
		r.GET("/admin/services/edit/:id", adminAuthMiddleware(), adminEditServiceHandler(db))
		r.POST("/admin/services/edit/:id", adminAuthMiddleware(), adminEditServiceHandler(db))
		r.POST("/admin/services/delete/:id", adminAuthMiddleware(), adminDeleteServiceHandler(db))
		r.POST("/admin/bookings/delete/:id", adminAuthMiddleware(), adminDeleteBookingHandler(db, bot))

		// Маршруты для управления врачами
		r.GET("/admin/doctors", adminAuthMiddleware(), adminDoctorsHandler(db))
		r.POST("/admin/doctors", adminAuthMiddleware(), adminDoctorsHandler(db))
		r.GET("/admin/doctors/edit/:id", adminAuthMiddleware(), adminEditDoctorHandler(db))
		r.POST("/admin/doctors/edit/:id", adminAuthMiddleware(), adminEditDoctorHandler(db))
		r.POST("/admin/doctors/delete/:id", adminAuthMiddleware(), adminDeleteDoctorHandler(db))
		r.GET("/admin/doctors/:id/schedule", adminAuthMiddleware(), adminDoctorScheduleHandler(db))
		r.POST("/admin/doctors/:id/schedule", adminAuthMiddleware(), adminDoctorScheduleHandler(db))
		r.POST("/admin/doctors/:doctor_id/schedule/delete/:schedule_id", adminAuthMiddleware(), adminDeleteScheduleHandler(db))

		// Публичный API для WebApp: список стоматологических услуг
		r.GET("/api/services", func(c *gin.Context) {
			rows, err := db.Query(`SELECT id, name, category, duration, price FROM services WHERE category = 'Стоматология'`)
			if err != nil {
				c.JSON(500, gin.H{"error": "DB error"})
				return
			}
			defer rows.Close()
			var services []models.Service
			for rows.Next() {
				var s models.Service
				if err := rows.Scan(&s.ID, &s.Name, &s.Category, &s.Duration, &s.Price); err == nil {
					services = append(services, s)
				}
			}
			c.JSON(200, services)
		})

		// Публичный API: получение доступных дат
		r.GET("/api/available_dates", func(c *gin.Context) {
			// Получаем даты на следующие 14 дней
			type DateInfo struct {
				Date     string `json:"date"`
				Status   string `json:"status"` // "available", "booked", "weekend"
				IsActive bool   `json:"isActive"`
			}
			var dates []DateInfo
			now := time.Now()

			// Получаем все записи на следующие 14 дней
			rows, err := db.Query(`
				SELECT date, COUNT(*) as booking_count
				FROM bookings
				WHERE date >= ? AND date <= ?
				AND status != 'Отменено'
				GROUP BY date
			`, now.Format("2006-01-02"), now.AddDate(0, 0, 14).Format("2006-01-02"))
			if err != nil {
				c.JSON(500, gin.H{"error": "DB error"})
				return
			}
			defer rows.Close()

			// Создаем карту занятых дат
			bookedDates := make(map[string]int)
			for rows.Next() {
				var date string
				var count int
				if err := rows.Scan(&date, &count); err == nil {
					bookedDates[date] = count
				}
			}

			// Генерируем информацию о датах
			for i := 0; i < 14; i++ {
				date := now.AddDate(0, 0, i)
				dateStr := date.Format("2006-01-02")

				// Проверяем, является ли день выходным
				if date.Weekday() == time.Sunday || date.Weekday() == time.Saturday {
					dates = append(dates, DateInfo{
						Date:     dateStr,
						Status:   "weekend",
						IsActive: false,
					})
					continue
				}

				// Проверяем количество записей на эту дату
				bookingCount := bookedDates[dateStr]
				if bookingCount >= 8 { // Предполагаем максимум 8 записей в день
					dates = append(dates, DateInfo{
						Date:     dateStr,
						Status:   "booked",
						IsActive: false,
					})
				} else {
					dates = append(dates, DateInfo{
						Date:     dateStr,
						Status:   "available",
						IsActive: true,
					})
				}
			}

			c.JSON(200, dates)
		})

		// Публичный API: получение доступного времени на конкретную дату
		r.GET("/api/available_times", func(c *gin.Context) {
			date := c.Query("date")
			if date == "" {
				c.JSON(400, gin.H{"error": "Date is required"})
				return
			}

			// Получаем все записи на эту дату
			rows, err := db.Query(`
				SELECT time, COUNT(*) as booking_count
				FROM bookings
				WHERE date = ? AND status != 'Отменено'
				GROUP BY time
			`, date)
			if err != nil {
				c.JSON(500, gin.H{"error": "DB error"})
				return
			}
			defer rows.Close()

			// Создаем карту занятого времени
			bookedTimes := make(map[string]int)
			for rows.Next() {
				var time string
				var count int
				if err := rows.Scan(&time, &count); err == nil {
					bookedTimes[time] = count
				}
			}

			// Генерируем доступное время (с 9:00 до 18:00, с интервалом в 30 минут)
			type TimeSlot struct {
				Time   string `json:"time"`
				Status string `json:"status"` // "free" или "busy"
			}
			var timeSlots []TimeSlot
			startTime := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
			endTime := time.Date(2000, 1, 1, 18, 0, 0, 0, time.UTC)

			for t := startTime; t.Before(endTime); t = t.Add(30 * time.Minute) {
				timeStr := t.Format("15:04")
				bookingCount := bookedTimes[timeStr]

				// Если на это время уже есть 2 записи, считаем слот занятым
				status := "free"
				if bookingCount >= 2 {
					status = "busy"
				}

				timeSlots = append(timeSlots, TimeSlot{
					Time:   timeStr,
					Status: status,
				})
			}

			c.JSON(200, timeSlots)
		})

		// Публичный API: отправка SMS-кода (заглушка, возвращает код)
		r.POST("/api/send_sms", func(c *gin.Context) {
			log.Printf("Received /api/send_sms request")
			log.Printf("Request headers: %v", c.Request.Header)
			log.Printf("Request body: %v", c.Request.Body)

			var req struct {
				Phone      string `json:"phone"`
				TelegramID int64  `json:"telegram_id"`
			}

			body, _ := c.GetRawData()
			log.Printf("Raw request body: %s", string(body))

			// Восстанавливаем body для повторного чтения
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

			if err := c.BindJSON(&req); err != nil {
				log.Printf("Error binding JSON: %v", err)
				c.JSON(400, gin.H{"error": "Некорректные данные"})
				return
			}

			log.Printf("Parsed request: phone=%q, telegram_id=%d", req.Phone, req.TelegramID)

			if req.Phone == "" || req.TelegramID == 0 {
				log.Printf("Invalid request: phone=%q, telegram_id=%d", req.Phone, req.TelegramID)
				c.JSON(400, gin.H{"error": "Некорректные данные"})
				return
			}

			log.Printf("Received phone number: %q", req.Phone)
			if !validatePhone(req.Phone) {
				log.Printf("Invalid phone number format: %q", req.Phone)
				c.JSON(400, gin.H{"error": "Некорректный формат номера телефона"})
				return
			}

			code := sendSMSCode(req.Phone, req.TelegramID)
			log.Printf("Generated SMS code for phone %q: %s", req.Phone, code)

			c.JSON(200, gin.H{"ok": true, "code": code})
		})

		// Публичный API: создание записи (с проверкой кода)
		r.POST("/api/bookings", func(c *gin.Context) {
			var req struct {
				Phone      string `json:"phone"`
				TelegramID int64  `json:"telegram_id"`
				Name       string `json:"name"`
				ServiceID  int    `json:"service_id"`
				Date       string `json:"date"`
				Time       string `json:"time"`
				Code       string `json:"code"`
			}
			log.Printf("Received booking request body: %+v", c.Request.Body)
			if err := c.ShouldBindJSON(&req); err != nil {
				log.Printf("Error binding JSON: %v", err)
				c.JSON(400, gin.H{"error": "Некорректные данные"})
				return
			}
			log.Printf("Parsed booking request: %+v", req)
			log.Printf("userID: %d, serviceID: %d, name: %s, phone: %s, date: %s, time: %s, code: %s", req.TelegramID, req.ServiceID, req.Name, req.Phone, req.Date, req.Time, req.Code)
			if req.Phone == "" || req.TelegramID == 0 || req.ServiceID == 0 || req.Date == "" || req.Time == "" || req.Code == "" {
				log.Printf("Invalid request data: phone=%q, telegram_id=%d, service_id=%d, date=%q, time=%q, code=%q",
					req.Phone, req.TelegramID, req.ServiceID, req.Date, req.Time, req.Code)
				c.JSON(400, gin.H{"error": "Некорректные данные"})
				return
			}
			if code, ok := smsCodes[req.TelegramID]; !ok || code != req.Code {
				log.Printf("Invalid or wrong code: got %q, expected %q", req.Code, smsCodes[req.TelegramID])
				c.JSON(400, gin.H{"error": "Неверный код"})
				return
			}
			userID := getOrCreateUserID(db, req.TelegramID)
			log.Printf("Resolved userID: %d", userID)
			db.Exec(`UPDATE users SET name = ?, phone = ? WHERE id = ?`, req.Name, req.Phone, userID)
			_, err := db.Exec(`INSERT INTO bookings (user_id, service_id, date, time, status, phone_confirmed) VALUES (?, ?, ?, ?, 'Подтверждено', 1)`, userID, req.ServiceID, req.Date, req.Time)
			if err != nil {
				log.Printf("DB error: %v", err)
				c.JSON(500, gin.H{"error": "Ошибка при сохранении записи"})
				return
			}
			delete(smsCodes, req.TelegramID)
			c.JSON(200, gin.H{"ok": true})
		})

		log.Printf("Starting web server on port %s", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatal("Failed to start web server:", err)
		}
	}()

	startAutoCancelCron(db)

	// Обработка сообщений от пользователя
	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Получаем или создаем ID пользователя
		userID := getOrCreateUserID(db, update.Message.From.ID)
		if userID == 0 {
			continue
		}

		// Получаем текущее состояние пользователя
		state, _ := GetUserState(db, int64(userID))

		// Обработка команды /start
		if update.Message.IsCommand() && update.Message.Command() == "start" {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Добро пожаловать! Я помогу вам записаться на прием к стоматологу.")
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Записаться на прием", "book"),
				),
			)
			bot.Send(msg)
			continue
		}

		// Обработка нажатия на кнопку "Записаться на прием"
		if update.CallbackQuery != nil && update.CallbackQuery.Data == "book" {
			// Создаем новое состояние для пользователя
			state = &models.UserState{
				TelegramID: int64(userID),
				Step:       "service",
				CreatedAt:  time.Now(),
			}
			SetUserState(db, state)

			// Получаем список услуг
			services := getServiceNames(db)
			if len(services) == 0 {
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Извините, в данный момент нет доступных услуг.")
				bot.Send(msg)
				continue
			}

			// Создаем клавиатуру с услугами
			var keyboard [][]tgbotapi.InlineKeyboardButton
			for _, service := range services {
				keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonData(service, "service:"+service),
				})
			}

			msg := tgbotapi.NewEditMessageTextAndMarkup(
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				"Выберите услугу:",
				tgbotapi.NewInlineKeyboardMarkup(keyboard...),
			)
			bot.Send(msg)
			continue
		}

		// Обработка выбора услуги
		if update.CallbackQuery != nil && strings.HasPrefix(update.CallbackQuery.Data, "service:") {
			service := strings.TrimPrefix(update.CallbackQuery.Data, "service:")
			state.Service = service
			state.Step = "doctor"
			SetUserState(db, state)

			// Получаем список врачей
			rows, err := db.Query("SELECT id, name FROM doctors WHERE is_active = 1")
			if err != nil {
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Извините, произошла ошибка при получении списка врачей.")
				bot.Send(msg)
				continue
			}
			defer rows.Close()

			var keyboard [][]tgbotapi.InlineKeyboardButton
			for rows.Next() {
				var id int64
				var name string
				if err := rows.Scan(&id, &name); err != nil {
					continue
				}
				keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonData(name, fmt.Sprintf("doctor:%d", id)),
				})
			}

			msg := tgbotapi.NewEditMessageTextAndMarkup(
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				"Выберите врача:",
				tgbotapi.NewInlineKeyboardMarkup(keyboard...),
			)
			bot.Send(msg)
			continue
		}

		// Обработка выбора врача
		if update.CallbackQuery != nil && strings.HasPrefix(update.CallbackQuery.Data, "doctor:") {
			doctorID, _ := strconv.ParseInt(strings.TrimPrefix(update.CallbackQuery.Data, "doctor:"), 10, 64)
			state.DoctorID = doctorID
			state.Step = "date"
			SetUserState(db, state)

			// Получаем доступные даты
			var dates []string
			for i := 0; i < 14; i++ {
				date := time.Now().AddDate(0, 0, i)
				if !models.IsWeekend(date) {
					dates = append(dates, date.Format("2006-01-02"))
				}
			}

			var keyboard [][]tgbotapi.InlineKeyboardButton
			for _, date := range dates {
				keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonData(date, "date:"+date),
				})
			}

			msg := tgbotapi.NewEditMessageTextAndMarkup(
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				"Выберите дату:",
				tgbotapi.NewInlineKeyboardMarkup(keyboard...),
			)
			bot.Send(msg)
			continue
		}

		// Обработка выбора даты
		if update.CallbackQuery != nil && strings.HasPrefix(update.CallbackQuery.Data, "date:") {
			date := strings.TrimPrefix(update.CallbackQuery.Data, "date:")
			state.Date = date
			state.Step = "time"
			SetUserState(db, state)

			// Получаем доступное время для выбранного врача и даты
			rows, err := db.Query(`
				SELECT DISTINCT time
				FROM doctor_schedules ds
				LEFT JOIN bookings b ON b.doctor_id = ds.doctor_id AND b.date = ? AND b.time = ds.start_time
				WHERE ds.doctor_id = ? AND ds.is_working_day = 1 AND b.id IS NULL
				ORDER BY ds.start_time
			`, date, state.DoctorID)
			if err != nil {
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Извините, произошла ошибка при получении доступного времени.")
				bot.Send(msg)
				continue
			}
			defer rows.Close()

			var keyboard [][]tgbotapi.InlineKeyboardButton
			for rows.Next() {
				var time string
				if err := rows.Scan(&time); err != nil {
					continue
				}
				keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonData(time, "time:"+time),
				})
			}

			msg := tgbotapi.NewEditMessageTextAndMarkup(
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				"Выберите время:",
				tgbotapi.NewInlineKeyboardMarkup(keyboard...),
			)
			bot.Send(msg)
			continue
		}

		// Обработка выбора времени
		if update.CallbackQuery != nil && strings.HasPrefix(update.CallbackQuery.Data, "time:") {
			time := strings.TrimPrefix(update.CallbackQuery.Data, "time:")
			state.Time = time
			state.Step = "phone"
			SetUserState(db, state)

			msg := tgbotapi.NewEditMessageText(
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				"Пожалуйста, введите ваш номер телефона:",
			)
			bot.Send(msg)
			continue
		}

		// Обработка ввода номера телефона
		if state != nil && state.Step == "phone" {
			phone := update.Message.Text
			state.Phone = phone
			state.Step = "confirm"
			SetUserState(db, state)

			// Получаем информацию об услуге и враче
			var serviceName, doctorName string
			err := db.QueryRow("SELECT name FROM services WHERE name = ?", state.Service).Scan(&serviceName)
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Извините, произошла ошибка при получении информации об услуге.")
				bot.Send(msg)
				continue
			}

			err = db.QueryRow("SELECT name FROM doctors WHERE id = ?", state.DoctorID).Scan(&doctorName)
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Извините, произошла ошибка при получении информации о враче.")
				bot.Send(msg)
				continue
			}

			// Создаем клавиатуру для подтверждения
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Подтвердить", "confirm"),
					tgbotapi.NewInlineKeyboardButtonData("Отменить", "cancel"),
				),
			)

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf(
				"Пожалуйста, проверьте данные:\n\nУслуга: %s\nВрач: %s\nДата: %s\nВремя: %s\nТелефон: %s",
				serviceName, doctorName, state.Date, state.Time, phone,
			))
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
			continue
		}

		// Обработка подтверждения записи
		if update.CallbackQuery != nil && update.CallbackQuery.Data == "confirm" {
			// Получаем ID услуги
			var serviceID int64
			err := db.QueryRow("SELECT id FROM services WHERE name = ?", state.Service).Scan(&serviceID)
			if err != nil {
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Извините, произошла ошибка при создании записи.")
				bot.Send(msg)
				continue
			}

			// Создаем запись
			_, err = db.Exec(`
				INSERT INTO bookings (user_id, doctor_id, service_id, date, time, status, phone_confirmed, created_at)
				VALUES (?, ?, ?, ?, ?, 'Ожидает подтверждения', 0, datetime('now'))
			`, userID, state.DoctorID, serviceID, state.Date, state.Time)
			if err != nil {
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Извините, произошла ошибка при создании записи.")
				bot.Send(msg)
				continue
			}

			// Очищаем состояние пользователя
			ClearUserState(db, int64(userID))

			msg := tgbotapi.NewEditMessageText(
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				"Спасибо! Ваша запись создана и ожидает подтверждения. Мы свяжемся с вами в ближайшее время.",
			)
			bot.Send(msg)
			continue
		}

		// Обработка отмены записи
		if update.CallbackQuery != nil && update.CallbackQuery.Data == "cancel" {
			// Очищаем состояние пользователя
			ClearUserState(db, int64(userID))

			msg := tgbotapi.NewEditMessageText(
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				"Запись отменена. Вы можете начать заново, нажав кнопку 'Записаться на прием'.",
			)
			bot.Send(msg)
			continue
		}
	}
}

func sendMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	menu := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Услуги"),
			tgbotapi.NewKeyboardButton("Записаться"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Мои записи"),
			tgbotapi.NewKeyboardButton("Контакты"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, "Добро пожаловать! Выберите действие:")
	msg.ReplyMarkup = menu
	bot.Send(msg)
}

func adminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		if session.Get("admin") != true {
			c.Redirect(302, "/admin/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func getOrCreateUserID(db *sql.DB, telegramID int64) int64 {
	var userID int64
	err := db.QueryRow("SELECT id FROM users WHERE telegram_id = ?", telegramID).Scan(&userID)
	if err == sql.ErrNoRows {
		result, err := db.Exec("INSERT INTO users (telegram_id) VALUES (?)", telegramID)
		if err != nil {
			log.Printf("Error creating user: %v", err)
			return 0
		}
		userID, _ = result.LastInsertId()
	} else if err != nil {
		log.Printf("Error getting user: %v", err)
		return 0
	}
	return userID
}

func startAutoCancelCron(db *sql.DB) {
	c := cron.New()
	c.AddFunc("0 0 * * *", func() {
		// Отменяем записи, которые не были подтверждены в течение 24 часов
		_, err := db.Exec(`
			UPDATE bookings 
			SET status = 'Отменено' 
			WHERE status = 'Ожидает подтверждения' 
			AND created_at < datetime('now', '-24 hours')
		`)
		if err != nil {
			log.Printf("Error in auto-cancel cron: %v", err)
		}
	})
	c.Start()
}

func getServiceNames(db *sql.DB) []string {
	rows, err := db.Query("SELECT name FROM services")
	if err != nil {
		log.Printf("Error getting services: %v", err)
		return nil
	}
	defer rows.Close()

	var services []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			services = append(services, name)
		}
	}
	return services
}
