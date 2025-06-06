package main

import (
	"database/sql"
	"log"
	"net"

	"MVP_ChatBot/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Загружаем конфигурацию
	config := LoadConfig()

	// Проверяем, не запущен ли уже экземпляр приложения
	if ln, err := net.Listen("tcp", ":"+config.Port); err != nil {
		log.Printf("Another instance is already running (port in use)")
		return
	} else {
		ln.Close()
	}

	// Инициализация бота
	if config.BotToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Настройка обновлений бота
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// Инициализация базы данных
	log.Printf("Using database at: %s", config.DatabasePath)
	db := InitDB(config.DatabasePath, config.MigrationsPath)

	// Запуск веб-сервера
	go startWebServer(db, bot, config)

	// Запуск обработки обновлений бота
	handlers.ProcessBotUpdates(bot, updates, db)
}

func startWebServer(db *sql.DB, bot *tgbotapi.BotAPI, config *Config) {
	r := gin.Default()

	// Настройка CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	corsConfig.AllowCredentials = true
	r.Use(cors.New(corsConfig))

	// Настройка сессий
	r.LoadHTMLGlob("templates/*.html")
	store := cookie.NewStore([]byte(config.SessionSecret))
	r.Use(sessions.Sessions("adminsession", store))

	// Редирект с корневого пути на админку
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/admin/login")
	})

	// Админка
	r.GET("/admin/login", handlers.AdminLoginHandler())
	r.POST("/admin/login", handlers.AdminLoginHandler())
	r.GET("/admin/logout", handlers.AdminLogoutHandler())

	// Защищенные маршруты
	admin := r.Group("/admin")
	admin.Use(handlers.AdminAuthMiddleware())
	{
		// Бронирования
		admin.GET("/bookings", handlers.AdminBookingsHandler(db))
		admin.POST("/bookings/delete/:id", handlers.AdminDeleteBookingHandler(db, bot))

		// Услуги
		admin.GET("/services", handlers.AdminServicesHandler(db))
		admin.POST("/services", handlers.AdminServicesHandler(db))
		admin.GET("/services/edit/:id", handlers.AdminEditServiceHandler(db))
		admin.POST("/services/edit/:id", handlers.AdminEditServiceHandler(db))
		admin.POST("/services/delete/:id", handlers.AdminDeleteServiceHandler(db))
		admin.GET("/export_pdf", handlers.AdminExportPDFHandler(db))

		// Врачи
		admin.GET("/doctors", handlers.AdminDoctorsHandler(db))
		admin.POST("/doctors", handlers.AdminDoctorsHandler(db))
		admin.GET("/doctors/edit/:doctor_id", handlers.AdminEditDoctorHandler(db))
		admin.POST("/doctors/edit/:doctor_id", handlers.AdminEditDoctorHandler(db))
		admin.POST("/doctors/delete/:doctor_id", handlers.AdminDeleteDoctorHandler(db))
		admin.GET("/doctors/:doctor_id/schedule", handlers.AdminDoctorScheduleHandler(db))
		admin.POST("/doctors/:doctor_id/schedule", handlers.AdminDoctorScheduleHandler(db))
		admin.POST("/doctors/:doctor_id/schedule/delete/:schedule_id", handlers.AdminDeleteScheduleHandler(db))
	}

	// Публичный API
	api := r.Group("/api")
	{
		api.GET("/services", handlers.GetServicesHandler(db))
		api.GET("/available_dates", handlers.GetAvailableDatesHandler(db))
		api.GET("/available_times", handlers.GetAvailableTimesHandler(db))
	}

	// Запуск сервера
	if err := r.Run(":" + config.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
