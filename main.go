package main

import (
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/robfig/cron/v3"
)

func main() {
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

	updates, err := bot.GetUpdatesChan(u)

	db := InitDB("mvp_chatbot.db", "migrations.sql")

	// Запускаем веб-сервер в отдельной горутине
	go func() {
		r := gin.Default()
		r.Use(cors.Default())
		r.LoadHTMLGlob("templates/*.html")

		store := cookie.NewStore([]byte("secret"))
		r.Use(sessions.Sessions("adminsession", store))

		r.GET("/admin/login", adminLoginHandler())
		r.POST("/admin/login", adminLoginHandler())

		// Получаем порт из переменной окружения или используем 8080 по умолчанию
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}

		log.Printf("Starting web server on port %s", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatal("Failed to start web server:", err)
		}
	}()

	startAutoCancelCron(db)

	for update := range updates {

		if update.CallbackQuery != nil {
			data := update.CallbackQuery.Data
			if strings.HasPrefix(data, "cancel_") {
				idStr := strings.TrimPrefix(data, "cancel_")
				id, _ := strconv.Atoi(idStr)
				db.Exec(`UPDATE bookings SET status = 'Отменено' WHERE id = ?`, id)
				bot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID, "Запись отменена"))
				bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Ваша запись отменена."))
				continue
			}
			userID := update.CallbackQuery.From.ID
			chatID := update.CallbackQuery.Message.Chat.ID
			state, _ := GetUserState(db, int64(userID))
			if state != nil && state.Step == "date" {
				state.Date = update.CallbackQuery.Data
				handleTimeSelection(bot, chatID)
				state.Step = "time"
				SetUserState(db, state)
				bot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID, "Дата выбрана"))
			} else if state != nil && state.Step == "time" {
				state.Time = update.CallbackQuery.Data
				msg := tgbotapi.NewMessage(chatID, "Введите ваш номер телефона для подтверждения:")
				addCancelHint(&msg)
				bot.Send(msg)
				state.Step = "phone"
				SetUserState(db, state)
				bot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID, "Время выбрано"))
			}
			continue
		}

		if update.Message == nil { // ignore non-Message updates
			continue
		}

		chatID := update.Message.Chat.ID
		userID := update.Message.From.ID

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				sendMainMenu(bot, chatID)
			case "cancel":
				ClearUserState(db, int64(userID))
				handleCancelBooking(bot, chatID)
			}
			continue
		}

		if update.Message.Text == "Записаться" {
			ClearUserState(db, int64(userID))
			handleBooking(bot, chatID, db)
			SetUserState(db, &UserState{TelegramID: int64(userID), Step: "service"})
			continue
		}
		if update.Message.Text == "Услуги" {
			handleServices(bot, chatID, db)
			continue
		}
		if update.Message.Text == "Контакты" {
			handleContacts(bot, chatID)
			continue
		}
		if update.Message.Text == "Мои записи" {
			handleMyBookings(bot, chatID, db, int64(userID))
			continue
		}

		// Пошаговый сценарий записи
		state, _ := GetUserState(db, int64(userID))
		if state != nil {
			switch state.Step {
			case "service":
				state.Service = update.Message.Text
				handleDateSelection(bot, chatID)
				state.Step = "date"
				SetUserState(db, state)
			case "date":
				state.Date = update.Message.Text
				handleTimeSelection(bot, chatID)
				state.Step = "time"
				SetUserState(db, state)
			case "time":
				state.Time = update.Message.Text
				msg := tgbotapi.NewMessage(chatID, "Введите ваш номер телефона для подтверждения:")
				bot.Send(msg)
				state.Step = "phone"
				SetUserState(db, state)
			case "phone":
				if !validatePhone(update.Message.Text) {
					msg := tgbotapi.NewMessage(chatID, "Некорректный номер. Введите еще раз:")
					bot.Send(msg)
					continue
				}
				state.Phone = update.Message.Text
				handleSMSConfirmation(bot, chatID, state.Phone, int64(userID))
				state.Step = "sms"
				SetUserState(db, state)
			case "sms":
				if update.Message.Text != smsCodes[int64(userID)] {
					msg := tgbotapi.NewMessage(chatID, "Неверный код. Попробуйте еще раз:")
					addCancelHint(&msg)
					bot.Send(msg)
					continue
				}
				// Сохраняем запись в bookings
				userIDVal := getOrCreateUserID(db, int64(userID))
				// Обновляем имя и телефон пользователя
				db.Exec(`UPDATE users SET name = ?, phone = ? WHERE id = ?`, update.Message.From.FirstName, state.Phone, userIDVal)
				_, err := db.Exec(`INSERT INTO bookings (user_id, service_id, date, time, status, phone_confirmed) VALUES (?, (SELECT id FROM services WHERE name = ?), ?, ?, 'Подтверждено', 1)`,
					userIDVal, state.Service, state.Date, state.Time)
				if err != nil {
					msg := tgbotapi.NewMessage(chatID, "Ошибка при сохранении записи. Попробуйте позже.")
					bot.Send(msg)
				} else {
					handleBookingConfirmed(bot, chatID)
					// Push-уведомление админам
					admins, _ := GetAdminTelegramIDs(db)
					for _, adminID := range admins {
						msg := tgbotapi.NewMessage(adminID, "Новая запись: "+state.Service+", "+state.Date+" "+state.Time+", клиент: "+state.Phone)
						bot.Send(msg)
					}
				}
				delete(smsCodes, int64(userID))
				ClearUserState(db, int64(userID))
			}
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

func runAdminWeb(db *sql.DB) {
	r := gin.Default()
	r.Use(cors.Default())
	r.LoadHTMLGlob("templates/*.html")

	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("adminsession", store))

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
	r.POST("/admin/bookings/delete/:id", adminAuthMiddleware(), adminDeleteBookingHandler(db))

	// Публичный API для WebApp: список стоматологических услуг
	r.GET("/api/services", func(c *gin.Context) {
		rows, err := db.Query(`SELECT id, name, category, duration, price FROM services WHERE category = 'Стоматология'`)
		if err != nil {
			c.JSON(500, gin.H{"error": "DB error"})
			return
		}
		defer rows.Close()
		var services []Service
		for rows.Next() {
			var s Service
			if err := rows.Scan(&s.ID, &s.Name, &s.Category, &s.Duration, &s.Price); err == nil {
				services = append(services, s)
			}
		}
		c.JSON(200, services)
	})

	// Публичный API: отправка SMS-кода (заглушка, возвращает код)
	r.POST("/api/send_sms", func(c *gin.Context) {
		var req struct {
			Phone      string `json:"phone"`
			TelegramID int64  `json:"telegram_id"`
		}
		log.Printf("Received request body: %+v", c.Request.Body)
		if err := c.ShouldBindJSON(&req); err != nil {
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
		if req.Phone == "" || req.TelegramID == 0 || req.ServiceID == 0 || req.Date == "" || req.Time == "" || req.Code == "" {
			log.Printf("Invalid request data: phone=%q, telegram_id=%d, service_id=%d, date=%q, time=%q, code=%q",
				req.Phone, req.TelegramID, req.ServiceID, req.Date, req.Time, req.Code)
			c.JSON(400, gin.H{"error": "Некорректные данные"})
			return
		}
		if code, ok := smsCodes[req.TelegramID]; !ok || code != req.Code {
			c.JSON(400, gin.H{"error": "Неверный код"})
			return
		}
		userID := getOrCreateUserID(db, req.TelegramID)
		db.Exec(`UPDATE users SET name = ?, phone = ? WHERE id = ?`, req.Name, req.Phone, userID)
		_, err := db.Exec(`INSERT INTO bookings (user_id, service_id, date, time, status, phone_confirmed) VALUES (?, ?, ?, ?, 'Подтверждено', 1)`, userID, req.ServiceID, req.Date, req.Time)
		if err != nil {
			c.JSON(500, gin.H{"error": "Ошибка при сохранении записи"})
			return
		}
		delete(smsCodes, req.TelegramID)
		c.JSON(200, gin.H{"ok": true})
	})

	r.Run(":8080")
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
	var id int64
	err := db.QueryRow(`SELECT id FROM users WHERE telegram_id = ?`, telegramID).Scan(&id)
	if err == sql.ErrNoRows {
		res, err := db.Exec(`INSERT INTO users (telegram_id) VALUES (?)`, telegramID)
		if err == nil {
			id, _ = res.LastInsertId()
		}
	} else if err != nil {
		return 0
	}
	return id
}

func startAutoCancelCron(db *sql.DB) {
	c := cron.New()
	c.AddFunc("@every 1m", func() {
		_, err := db.Exec(`UPDATE bookings SET status = 'Отменено' WHERE status = 'Ожидание' AND phone_confirmed = 0 AND created_at <= ?`, time.Now().Add(-15*time.Minute).Format("2006-01-02 15:04:05"))
		if err != nil {
			log.Println("cron auto-cancel error:", err)
		}
	})
	c.Start()
}

func getServiceNames(db *sql.DB) []string {
	rows, err := db.Query("SELECT name FROM services")
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
