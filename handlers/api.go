package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func GetServicesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT id, name, category, duration, price
			FROM services
			ORDER BY category, name
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
			return
		}
		defer rows.Close()

		var services []struct {
			ID       int64   `json:"id"`
			Name     string  `json:"name"`
			Category string  `json:"category"`
			Duration int     `json:"duration"`
			Price    float64 `json:"price"`
		}

		for rows.Next() {
			var s struct {
				ID       int64   `json:"id"`
				Name     string  `json:"name"`
				Category string  `json:"category"`
				Duration int     `json:"duration"`
				Price    float64 `json:"price"`
			}
			if err := rows.Scan(&s.ID, &s.Name, &s.Category, &s.Duration, &s.Price); err == nil {
				services = append(services, s)
			}
		}

		c.JSON(http.StatusOK, services)
	}
}

func GetAvailableDatesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Получаем дату начала и конца периода
		startDate := c.Query("start_date")
		endDate := c.Query("end_date")

		if startDate == "" {
			startDate = time.Now().Format("2006-01-02")
		}
		if endDate == "" {
			// По умолчанию показываем доступные даты на 2 недели вперед
			endDate = time.Now().AddDate(0, 0, 14).Format("2006-01-02")
		}

		// Получаем все записи в указанном периоде
		rows, err := db.Query(`
			SELECT date, COUNT(*) as booking_count
			FROM bookings
			WHERE date BETWEEN ? AND ?
			GROUP BY date
		`, startDate, endDate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
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

		// Максимальное количество записей в день
		maxBookingsPerDay := 8

		// Формируем список доступных дат
		var availableDates []string
		start, _ := time.Parse("2006-01-02", startDate)
		end, _ := time.Parse("2006-01-02", endDate)

		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			dateStr := d.Format("2006-01-02")
			if bookedDates[dateStr] < maxBookingsPerDay {
				availableDates = append(availableDates, dateStr)
			}
		}

		c.JSON(http.StatusOK, availableDates)
	}
}

func GetAvailableTimesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		date := c.Query("date")
		if date == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Дата не указана"})
			return
		}

		// Получаем существующие записи на указанную дату
		rows, err := db.Query(`
			SELECT time
			FROM bookings
			WHERE date = ?
		`, date)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
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

		// Формируем список доступных времен
		var availableTimes []string
		start, _ := time.Parse("15:04", "09:00")
		end, _ := time.Parse("15:04", "18:00")

		for t := start; t.Before(end); t = t.Add(30 * time.Minute) {
			timeStr := t.Format("15:04")
			if !bookedTimes[timeStr] {
				availableTimes = append(availableTimes, timeStr)
			}
		}

		c.JSON(http.StatusOK, availableTimes)
	}
}
