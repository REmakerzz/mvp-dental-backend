package main

import (
	"database/sql"

	"MVP_ChatBot/handlers"

	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func setupRoutes(r *gin.Engine, db *sql.DB, bot *tgbotapi.BotAPI) {
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
}
