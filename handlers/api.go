package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Структура для услуги
type Service struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Duration int     `json:"duration"`
	Price    float64 `json:"price"`
}

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

// Получение списка врачей
func GetDoctorsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT id, name, specialization, description, photo_url
			FROM doctors
			ORDER BY name
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
			return
		}
		defer rows.Close()

		var doctors []Doctor
		for rows.Next() {
			var d Doctor
			if err := rows.Scan(&d.ID, &d.Name, &d.Specialization, &d.Description, &d.PhotoURL); err == nil {
				doctors = append(doctors, d)
			}
		}

		c.JSON(http.StatusOK, doctors)
	}
}

// Добавление нового врача
func AddDoctorHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var doctor Doctor
		if err := c.ShouldBindJSON(&doctor); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
			return
		}

		result, err := db.Exec(`
			INSERT INTO doctors (name, specialization, description, photo_url)
			VALUES (?, ?, ?, ?)
		`, doctor.Name, doctor.Specialization, doctor.Description, doctor.PhotoURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при добавлении врача"})
			return
		}

		id, err := result.LastInsertId()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении ID"})
			return
		}

		doctor.ID = id
		c.JSON(http.StatusOK, doctor)
	}
}

// Обновление врача
func UpdateDoctorHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var doctor Doctor
		if err := c.ShouldBindJSON(&doctor); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
			return
		}

		_, err := db.Exec(`
			UPDATE doctors
			SET name = ?, specialization = ?, description = ?, photo_url = ?
			WHERE id = ?
		`, doctor.Name, doctor.Specialization, doctor.Description, doctor.PhotoURL, doctor.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при обновлении врача"})
			return
		}

		c.JSON(http.StatusOK, doctor)
	}
}

// Удаление врача
func DeleteDoctorHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		_, err := db.Exec("DELETE FROM doctors WHERE id = ?", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при удалении врача"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Врач успешно удален"})
	}
}

// Добавление новой услуги
func AddServiceHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var service Service
		if err := c.ShouldBindJSON(&service); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
			return
		}

		result, err := db.Exec(`
			INSERT INTO services (name, category, duration, price)
			VALUES (?, ?, ?, ?)
		`, service.Name, service.Category, service.Duration, service.Price)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при добавлении услуги"})
			return
		}

		id, err := result.LastInsertId()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении ID"})
			return
		}

		service.ID = id
		c.JSON(http.StatusOK, service)
	}
}

// Обновление услуги
func UpdateServiceHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var service Service
		if err := c.ShouldBindJSON(&service); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
			return
		}

		_, err := db.Exec(`
			UPDATE services
			SET name = ?, category = ?, duration = ?, price = ?
			WHERE id = ?
		`, service.Name, service.Category, service.Duration, service.Price, service.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при обновлении услуги"})
			return
		}

		c.JSON(http.StatusOK, service)
	}
}

// Удаление услуги
func DeleteServiceHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		_, err := db.Exec("DELETE FROM services WHERE id = ?", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при удалении услуги"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Услуга успешно удалена"})
	}
}

// Создание записи
func CreateBookingHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var booking struct {
			UserID    int64  `json:"user_id"`
			ServiceID int64  `json:"service_id"`
			Date      string `json:"date"`
			Time      string `json:"time"`
		}
		if err := c.ShouldBindJSON(&booking); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
			return
		}

		result, err := db.Exec(`
			INSERT INTO bookings (user_id, service_id, date, time)
			VALUES (?, ?, ?, ?)
		`, booking.UserID, booking.ServiceID, booking.Date, booking.Time)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при создании записи"})
			return
		}

		id, err := result.LastInsertId()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении ID"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"id": id})
	}
}

// Получение записей пользователя
func GetUserBookingsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")
		rows, err := db.Query(`
			SELECT b.id, b.date, b.time, b.status, s.name as service_name
			FROM bookings b
			JOIN services s ON b.service_id = s.id
			WHERE b.user_id = ?
			ORDER BY b.date DESC, b.time DESC
		`, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
			return
		}
		defer rows.Close()

		var bookings []struct {
			ID          int64  `json:"id"`
			Date        string `json:"date"`
			Time        string `json:"time"`
			Status      string `json:"status"`
			ServiceName string `json:"service_name"`
		}

		for rows.Next() {
			var b struct {
				ID          int64  `json:"id"`
				Date        string `json:"date"`
				Time        string `json:"time"`
				Status      string `json:"status"`
				ServiceName string `json:"service_name"`
			}
			if err := rows.Scan(&b.ID, &b.Date, &b.Time, &b.Status, &b.ServiceName); err == nil {
				bookings = append(bookings, b)
			}
		}

		c.JSON(http.StatusOK, bookings)
	}
}

// Отмена записи
func CancelBookingHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		_, err := db.Exec("UPDATE bookings SET status = 'Отменена' WHERE id = ?", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при отмене записи"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Запись успешно отменена"})
	}
}
