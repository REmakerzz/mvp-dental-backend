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

		// Получаем максимальное количество записей в день
		var maxBookingsPerDay int
		err = db.QueryRow("SELECT COUNT(*) FROM doctors").Scan(&maxBookingsPerDay)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
			return
		}

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

		// Получаем день недели для указанной даты
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат даты"})
			return
		}
		dayOfWeek := int(t.Weekday())

		// Получаем расписание врачей на указанный день недели
		rows, err := db.Query(`
			SELECT DISTINCT start_time, end_time
			FROM doctor_schedule
			WHERE day_of_week = ?
			ORDER BY start_time
		`, dayOfWeek)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
			return
		}
		defer rows.Close()

		// Получаем существующие записи на указанную дату
		bookings, err := db.Query(`
			SELECT time
			FROM bookings
			WHERE date = ?
		`, date)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
			return
		}
		defer bookings.Close()

		// Создаем карту занятых времен
		bookedTimes := make(map[string]bool)
		for bookings.Next() {
			var time string
			if err := bookings.Scan(&time); err == nil {
				bookedTimes[time] = true
			}
		}

		// Формируем список доступных времен
		var availableTimes []string
		for rows.Next() {
			var startTime, endTime string
			if err := rows.Scan(&startTime, &endTime); err == nil {
				// Генерируем временные слоты с интервалом в 30 минут
				start, _ := time.Parse("15:04", startTime)
				end, _ := time.Parse("15:04", endTime)

				for t := start; t.Before(end); t = t.Add(30 * time.Minute) {
					timeStr := t.Format("15:04")
					if !bookedTimes[timeStr] {
						availableTimes = append(availableTimes, timeStr)
					}
				}
			}
		}

		c.JSON(http.StatusOK, availableTimes)
	}
}
