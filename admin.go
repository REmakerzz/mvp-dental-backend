package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

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
			var services []Service
			for rows.Next() {
				var s Service
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
			var s Service
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
		date := c.Query("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		rows, err := db.Query(`SELECT u.name, s.name, b.date, b.time, b.status FROM bookings b LEFT JOIN users u ON b.user_id = u.id LEFT JOIN services s ON b.service_id = s.id WHERE b.date = ? ORDER BY b.time`, date)
		if err != nil {
			c.String(500, "DB error")
			return
		}
		defer rows.Close()
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		pdf.SetFont("Arial", "", 14)
		pdf.Cell(40, 10, "Расписание на "+date)
		pdf.Ln(12)
		pdf.SetFont("Arial", "", 10)
		pdf.Cell(40, 8, "Клиент")
		pdf.Cell(40, 8, "Услуга")
		pdf.Cell(30, 8, "Время")
		pdf.Cell(30, 8, "Статус")
		pdf.Ln(8)
		for rows.Next() {
			var user, service, date, time, status string
			rows.Scan(&user, &service, &date, &time, &status)
			pdf.Cell(40, 8, user)
			pdf.Cell(40, 8, service)
			pdf.Cell(30, 8, time)
			pdf.Cell(30, 8, status)
			pdf.Ln(8)
		}
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", "attachment; filename=schedule.pdf")
		_ = pdf.Output(c.Writer)
	}
}

func adminDeleteBookingHandler(db *sql.DB, bot *tgbotapi.BotAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		// Получаем информацию о записи и пользователе перед удалением
		var telegramID int64
		var service, date, time string
		err := db.QueryRow(`
			SELECT u.telegram_id, s.name, b.date, b.time 
			FROM bookings b 
			LEFT JOIN users u ON b.user_id = u.id 
			LEFT JOIN services s ON b.service_id = s.id 
			WHERE b.id = ?`, id).Scan(&telegramID, &service, &date, &time)

		if err != nil {
			log.Printf("Error getting booking info: %v", err)
			c.String(500, "DB error")
			return
		}

		// Удаляем запись
		_, err = db.Exec("DELETE FROM bookings WHERE id = ?", id)
		if err != nil {
			log.Printf("Error deleting booking: %v", err)
			c.String(500, "DB error")
			return
		}

		// Отправляем уведомление клиенту
		if telegramID != 0 {
			msg := tgbotapi.NewMessage(telegramID, fmt.Sprintf(
				"❌ Ваша запись отменена администратором:\nУслуга: %s\nДата: %s\nВремя: %s",
				service, date, time))
			bot.Send(msg)
		}

		c.Redirect(302, "/admin/bookings")
	}
}
