package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"

	"MVP_ChatBot/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jung-kurt/gofpdf"
)

type BookingView struct {
	ID       int
	UserName string
	Service  string
	Date     string
	Time     string
	Status   string
}

type Service struct {
	ID       int
	Name     string
	Category string
	Duration int
	Price    int
}

type DoctorView struct {
	ID             int
	Name           string
	Specialization string
	Description    string
	PhotoURL       string
}

type DoctorScheduleView struct {
	ID         int
	DoctorID   int
	DoctorName string
	Weekday    int
	StartTime  string
	EndTime    string
	BreakStart string
	BreakEnd   string
}

// Handler представляет обработчик HTTP-запросов
type Handler struct {
	db        *sql.DB
	templates *template.Template
}

// NewHandler создает новый обработчик
func NewHandler(db *sql.DB, templates *template.Template) *Handler {
	return &Handler{
		db:        db,
		templates: templates,
	}
}

func adminBookingsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		date := c.Query("date")
		client := c.Query("client")
		query := `SELECT b.id, u.name, s.name, b.date, b.time, b.status FROM bookings b LEFT JOIN users u ON b.user_id = u.id LEFT JOIN services s ON b.service_id = s.id WHERE 1=1`
		args := []interface{}{}
		if date != "" {
			query += " AND b.date = ?"
			args = append(args, date)
		}
		if client != "" {
			query += " AND u.name LIKE ?"
			args = append(args, "%"+client+"%")
		}
		query += " ORDER BY b.date, b.time"
		rows, err := db.Query(query, args...)
		if err != nil {
			c.String(http.StatusInternalServerError, "DB error")
			return
		}
		defer rows.Close()

		var bookings []BookingView
		for rows.Next() {
			var b BookingView
			if err := rows.Scan(&b.ID, &b.UserName, &b.Service, &b.Date, &b.Time, &b.Status); err == nil {
				bookings = append(bookings, b)
			}
		}
		c.HTML(http.StatusOK, "admin_bookings.html", gin.H{
			"bookings":      bookings,
			"filter_date":   date,
			"filter_client": client,
		})
	}
}

func adminLoginHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			c.HTML(http.StatusOK, "admin_login.html", nil)
			return
		}
		login := c.PostForm("login")
		password := c.PostForm("password")
		if login == "admin" && password == "admin" { // TODO: заменить на безопасную проверку
			session := sessions.Default(c)
			session.Set("admin", true)
			session.Save()
			c.Redirect(http.StatusFound, "/admin/bookings")
			return
		}
		c.HTML(http.StatusUnauthorized, "admin_login.html", gin.H{"error": "Неверный логин или пароль"})
	}
}

func adminLogoutHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()
		c.Redirect(302, "/admin/login")
	}
}

func adminServicesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			rows, err := db.Query(`SELECT id, name, category, duration, price FROM services`)
			if err != nil {
				c.String(http.StatusInternalServerError, "DB error")
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
			c.HTML(http.StatusOK, "admin_services.html", gin.H{"services": services})
			return
		}
		// POST: добавление новой услуги
		name := c.PostForm("name")
		category := c.PostForm("category")
		duration := c.PostForm("duration")
		price := c.PostForm("price")
		_, err := db.Exec(`INSERT INTO services (name, category, duration, price) VALUES (?, ?, ?, ?)`, name, category, duration, price)
		if err != nil {
			c.String(http.StatusInternalServerError, "DB error")
			return
		}
		c.Redirect(http.StatusFound, "/admin/services")
	}
}

func adminEditServiceHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if c.Request.Method == http.MethodGet {
			row := db.QueryRow(`SELECT id, name, category, duration, price FROM services WHERE id = ?`, id)
			var s models.Service
			if err := row.Scan(&s.ID, &s.Name, &s.Category, &s.Duration, &s.Price); err != nil {
				c.String(404, "Услуга не найдена")
				return
			}
			c.HTML(200, "admin_edit_service.html", gin.H{"service": s})
			return
		}
		// POST: обновление услуги
		name := c.PostForm("name")
		category := c.PostForm("category")
		duration := c.PostForm("duration")
		price := c.PostForm("price")
		_, err := db.Exec(`UPDATE services SET name=?, category=?, duration=?, price=? WHERE id=?`, name, category, duration, price, id)
		if err != nil {
			c.String(500, "DB error")
			return
		}
		c.Redirect(302, "/admin/services")
	}
}

func adminDeleteServiceHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		_, err := db.Exec(`DELETE FROM services WHERE id = ?`, id)
		if err != nil {
			c.String(500, "DB error")
			return
		}
		c.Redirect(302, "/admin/services")
	}
}

func adminExportPDFHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Получаем все записи
		rows, err := db.Query(`
			SELECT b.id, u.name, s.name, b.date, b.time, b.status
			FROM bookings b
			LEFT JOIN users u ON b.user_id = u.id
			LEFT JOIN services s ON b.service_id = s.id
			ORDER BY b.date, b.time
		`)
		if err != nil {
			c.String(http.StatusInternalServerError, "DB error")
			return
		}
		defer rows.Close()

		// Создаем PDF
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		pdf.Cell(40, 10, "Записи на прием")
		pdf.Ln(20)

		// Заголовки таблицы
		pdf.SetFont("Arial", "B", 12)
		headers := []string{"ID", "Клиент", "Услуга", "Дата", "Время", "Статус"}
		widths := []float64{10, 40, 40, 30, 20, 30}
		for i, header := range headers {
			pdf.CellFormat(widths[i], 10, header, "1", 0, "C", false, 0, "")
		}
		pdf.Ln(-1)

		// Данные таблицы
		pdf.SetFont("Arial", "", 12)
		for rows.Next() {
			var b BookingView
			if err := rows.Scan(&b.ID, &b.UserName, &b.Service, &b.Date, &b.Time, &b.Status); err == nil {
				pdf.CellFormat(widths[0], 10, fmt.Sprintf("%d", b.ID), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[1], 10, b.UserName, "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[2], 10, b.Service, "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[3], 10, b.Date, "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[4], 10, b.Time, "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[5], 10, b.Status, "1", 0, "", false, 0, "")
				pdf.Ln(-1)
			}
		}

		// Отправляем PDF
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", "attachment; filename=bookings.pdf")
		err = pdf.Output(c.Writer)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error generating PDF")
			return
		}
	}
}

func adminDeleteBookingHandler(db *sql.DB, bot *tgbotapi.BotAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		// Получаем информацию о записи
		var booking models.Booking
		err := db.QueryRow(`
			SELECT b.id, b.user_id, b.service_id, b.date, b.time, b.status, u.telegram_id
			FROM bookings b
			JOIN users u ON b.user_id = u.id
			WHERE b.id = ?
		`, id).Scan(&booking.ID, &booking.UserID, &booking.ServiceID, &booking.Date, &booking.Time, &booking.Status, &booking.TelegramID)
		if err != nil {
			c.String(500, "DB error")
			return
		}

		// Удаляем запись
		_, err = db.Exec(`DELETE FROM bookings WHERE id = ?`, id)
		if err != nil {
			c.String(500, "DB error")
			return
		}

		// Отправляем уведомление пользователю
		if booking.TelegramID != 0 {
			msg := tgbotapi.NewMessage(booking.TelegramID, fmt.Sprintf("Ваша запись на %s %s была отменена администратором.", booking.Date, booking.Time))
			bot.Send(msg)
		}

		c.Redirect(302, "/admin/bookings")
	}
}

func adminDoctorsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			rows, err := db.Query(`SELECT id, name, specialization, description, photo_url, is_active FROM doctors`)
			if err != nil {
				c.String(http.StatusInternalServerError, "DB error")
				return
			}
			defer rows.Close()
			var doctors []models.Doctor
			for rows.Next() {
				var d models.Doctor
				if err := rows.Scan(&d.ID, &d.Name, &d.Specialization, &d.Description, &d.PhotoURL, &d.IsActive); err == nil {
					doctors = append(doctors, d)
				}
			}
			c.HTML(http.StatusOK, "admin_doctors.html", gin.H{"doctors": doctors})
			return
		}
		// POST: добавление нового врача
		name := c.PostForm("name")
		specialization := c.PostForm("specialization")
		description := c.PostForm("description")
		photoURL := c.PostForm("photo_url")
		isActive := c.PostForm("is_active") == "on"
		_, err := db.Exec(`INSERT INTO doctors (name, specialization, description, photo_url, is_active) VALUES (?, ?, ?, ?, ?)`,
			name, specialization, description, photoURL, isActive)
		if err != nil {
			c.String(http.StatusInternalServerError, "DB error")
			return
		}
		c.Redirect(http.StatusFound, "/admin/doctors")
	}
}

func adminEditDoctorHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if c.Request.Method == http.MethodGet {
			row := db.QueryRow(`SELECT id, name, specialization, description, photo_url, is_active FROM doctors WHERE id = ?`, id)
			var d models.Doctor
			if err := row.Scan(&d.ID, &d.Name, &d.Specialization, &d.Description, &d.PhotoURL, &d.IsActive); err != nil {
				c.String(404, "Врач не найден")
				return
			}
			c.HTML(200, "admin_edit_doctor.html", gin.H{"doctor": d})
			return
		}
		// POST: обновление врача
		name := c.PostForm("name")
		specialization := c.PostForm("specialization")
		description := c.PostForm("description")
		photoURL := c.PostForm("photo_url")
		isActive := c.PostForm("is_active") == "on"
		_, err := db.Exec(`UPDATE doctors SET name=?, specialization=?, description=?, photo_url=?, is_active=? WHERE id=?`,
			name, specialization, description, photoURL, isActive, id)
		if err != nil {
			c.String(500, "DB error")
			return
		}
		c.Redirect(302, "/admin/doctors")
	}
}

func adminDeleteDoctorHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		_, err := db.Exec(`DELETE FROM doctors WHERE id = ?`, id)
		if err != nil {
			c.String(500, "DB error")
			return
		}
		c.Redirect(302, "/admin/doctors")
	}
}

func adminDoctorScheduleHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.Param("id")
		if c.Request.Method == http.MethodGet {
			rows, err := db.Query(`
				SELECT ds.id, ds.doctor_id, d.name as doctor_name, ds.weekday, ds.start_time, ds.end_time, ds.break_start, ds.break_end, ds.is_working_day
				FROM doctor_schedules ds
				JOIN doctors d ON ds.doctor_id = d.id
				WHERE ds.doctor_id = ?
				ORDER BY ds.weekday, ds.start_time
			`, doctorID)
			if err != nil {
				c.String(http.StatusInternalServerError, "DB error")
				return
			}
			defer rows.Close()
			var schedules []models.DoctorSchedule
			for rows.Next() {
				var s models.DoctorSchedule
				var doctorName string
				if err := rows.Scan(&s.ID, &s.DoctorID, &doctorName, &s.Weekday, &s.StartTime, &s.EndTime, &s.BreakStart, &s.BreakEnd, &s.IsWorkingDay); err == nil {
					schedules = append(schedules, s)
				}
			}
			c.HTML(http.StatusOK, "admin_doctor_schedule.html", gin.H{
				"schedules": schedules,
				"doctor_id": doctorID,
			})
			return
		}
		// POST: добавление/обновление расписания
		weekday := c.PostForm("weekday")
		startTime := c.PostForm("start_time")
		endTime := c.PostForm("end_time")
		breakStart := c.PostForm("break_start")
		breakEnd := c.PostForm("break_end")
		isWorkingDay := c.PostForm("is_working_day") == "on"
		scheduleID := c.PostForm("schedule_id")

		if scheduleID == "" {
			// Добавление нового расписания
			_, err := db.Exec(`
				INSERT INTO doctor_schedules (doctor_id, weekday, start_time, end_time, break_start, break_end, is_working_day)
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`, doctorID, weekday, startTime, endTime, breakStart, breakEnd, isWorkingDay)
			if err != nil {
				c.String(http.StatusInternalServerError, "DB error")
				return
			}
		} else {
			// Обновление существующего расписания
			_, err := db.Exec(`
				UPDATE doctor_schedules
				SET weekday = ?, start_time = ?, end_time = ?, break_start = ?, break_end = ?, is_working_day = ?
				WHERE id = ? AND doctor_id = ?
			`, weekday, startTime, endTime, breakStart, breakEnd, isWorkingDay, scheduleID, doctorID)
			if err != nil {
				c.String(http.StatusInternalServerError, "DB error")
				return
			}
		}

		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/doctors/%s/schedule", doctorID))
	}
}

func adminDeleteScheduleHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.Param("doctor_id")
		scheduleID := c.Param("id")
		_, err := db.Exec(`DELETE FROM doctor_schedules WHERE id = ? AND doctor_id = ?`, scheduleID, doctorID)
		if err != nil {
			c.String(500, "DB error")
			return
		}
		c.Redirect(302, fmt.Sprintf("/admin/doctors/%s/schedule", doctorID))
	}
}
