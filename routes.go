package main

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func setupRoutes(r *gin.Engine, db *sql.DB, bot *tgbotapi.BotAPI) {
	// Редирект с корневого пути на админку
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/admin/login")
	})

	// Админка
	r.GET("/admin/login", adminLoginHandler())
	r.POST("/admin/login", adminLoginHandler())
	r.GET("/admin/logout", adminLogoutHandler())

	// Защищенные маршруты
	admin := r.Group("/admin")
	admin.Use(adminAuthMiddleware())
	{
		// Бронирования
		admin.GET("/bookings", adminBookingsHandler(db))
		admin.POST("/bookings/delete/:id", adminDeleteBookingHandler(db, bot))

		// Услуги
		admin.GET("/services", adminServicesHandler(db))
		admin.POST("/services", adminServicesHandler(db))
		admin.GET("/services/edit/:id", adminEditServiceHandler(db))
		admin.POST("/services/edit/:id", adminEditServiceHandler(db))
		admin.POST("/services/delete/:id", adminDeleteServiceHandler(db))
		admin.GET("/export_pdf", adminExportPDFHandler(db))

		// Врачи
		admin.GET("/doctors", adminDoctorsHandler(db))
		admin.POST("/doctors", adminDoctorsHandler(db))
		admin.GET("/doctors/edit/:doctor_id", adminEditDoctorHandler(db))
		admin.POST("/doctors/edit/:doctor_id", adminEditDoctorHandler(db))
		admin.POST("/doctors/delete/:doctor_id", adminDeleteDoctorHandler(db))
		admin.GET("/doctors/:doctor_id/schedule", adminDoctorScheduleHandler(db))
		admin.POST("/doctors/:doctor_id/schedule", adminDoctorScheduleHandler(db))
		admin.POST("/doctors/:doctor_id/schedule/delete/:schedule_id", adminDeleteScheduleHandler(db))
	}

	// Публичный API
	api := r.Group("/api")
	{
		api.GET("/services", getServicesHandler(db))
		api.GET("/available_dates", getAvailableDatesHandler(db))
		api.GET("/available_times", getAvailableTimesHandler(db))
	}
}
