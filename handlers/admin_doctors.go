package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Doctor представляет информацию о враче
type Doctor struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Specialization string `json:"specialization"`
	Description    string `json:"description"`
	PhotoURL       string `json:"photo_url"`
	IsActive       bool   `json:"is_active"`
}

// DoctorSchedule представляет расписание врача
type DoctorSchedule struct {
	ID           int64  `json:"id"`
	DoctorID     int64  `json:"doctor_id"`
	DayOfWeek    int    `json:"day_of_week"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	IsWorkingDay bool   `json:"is_working_day"`
}

// adminDoctorsHandler обрабатывает страницу управления врачами
func adminDoctorsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			// Получаем список врачей
			rows, err := db.Query("SELECT id, name, specialization, description, photo_url, is_active FROM doctors ORDER BY name")
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"error": "Ошибка при получении списка врачей",
				})
				return
			}
			defer rows.Close()

			var doctors []Doctor
			for rows.Next() {
				var d Doctor
				if err := rows.Scan(&d.ID, &d.Name, &d.Specialization, &d.Description, &d.PhotoURL, &d.IsActive); err != nil {
					c.HTML(http.StatusInternalServerError, "error.html", gin.H{
						"error": "Ошибка при сканировании данных врача",
					})
					return
				}
				doctors = append(doctors, d)
			}

			c.HTML(http.StatusOK, "admin_doctors.html", gin.H{
				"doctors": doctors,
			})
			return
		}

		if c.Request.Method == http.MethodPost {
			// Добавление нового врача
			name := c.PostForm("name")
			specialization := c.PostForm("specialization")
			description := c.PostForm("description")
			photoURL := c.PostForm("photo_url")

			_, err := db.Exec(
				"INSERT INTO doctors (name, specialization, description, photo_url, is_active) VALUES (?, ?, ?, ?, true)",
				name, specialization, description, photoURL,
			)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"error": "Ошибка при добавлении врача",
				})
				return
			}

			c.Redirect(http.StatusSeeOther, "/admin/doctors")
			return
		}
	}
}

// adminEditDoctorHandler обрабатывает страницу редактирования врача
func adminEditDoctorHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Получаем ID врача из URL
		doctorID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Неверный ID врача",
			})
			return
		}

		if c.Request.Method == http.MethodGet {
			// Получаем информацию о враче
			var doctor Doctor
			err := db.QueryRow(
				"SELECT id, name, specialization, description, photo_url, is_active FROM doctors WHERE id = ?",
				doctorID,
			).Scan(&doctor.ID, &doctor.Name, &doctor.Specialization, &doctor.Description, &doctor.PhotoURL, &doctor.IsActive)
			if err != nil {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"error": "Врач не найден",
				})
				return
			}

			c.HTML(http.StatusOK, "admin_doctor_edit.html", gin.H{
				"doctor": doctor,
			})
			return
		}

		if c.Request.Method == http.MethodPost {
			// Обновление информации о враче
			name := c.PostForm("name")
			specialization := c.PostForm("specialization")
			description := c.PostForm("description")
			photoURL := c.PostForm("photo_url")
			isActive := c.PostForm("is_active") == "on"

			_, err := db.Exec(
				"UPDATE doctors SET name = ?, specialization = ?, description = ?, photo_url = ?, is_active = ? WHERE id = ?",
				name, specialization, description, photoURL, isActive, doctorID,
			)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"error": "Ошибка при обновлении врача",
				})
				return
			}

			c.Redirect(http.StatusSeeOther, "/admin/doctors")
			return
		}
	}
}

// adminDeleteDoctorHandler обрабатывает удаление врача
func adminDeleteDoctorHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.HTML(http.StatusMethodNotAllowed, "error.html", gin.H{
				"error": "Метод не поддерживается",
			})
			return
		}

		// Получаем ID врача из URL
		doctorID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Неверный ID врача",
			})
			return
		}

		// Удаляем врача
		_, err = db.Exec("DELETE FROM doctors WHERE id = ?", doctorID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Ошибка при удалении врача",
			})
			return
		}

		c.Redirect(http.StatusSeeOther, "/admin/doctors")
	}
}

// adminDoctorScheduleHandler обрабатывает страницу управления расписанием врача
func adminDoctorScheduleHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Получаем ID врача из URL
		doctorID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Неверный ID врача",
			})
			return
		}

		if c.Request.Method == http.MethodGet {
			// Получаем информацию о враче
			var doctor Doctor
			err := db.QueryRow(
				"SELECT id, name, specialization, description, photo_url, is_active FROM doctors WHERE id = ?",
				doctorID,
			).Scan(&doctor.ID, &doctor.Name, &doctor.Specialization, &doctor.Description, &doctor.PhotoURL, &doctor.IsActive)
			if err != nil {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"error": "Врач не найден",
				})
				return
			}

			// Получаем расписание врача
			rows, err := db.Query(
				"SELECT id, doctor_id, day_of_week, start_time, end_time, is_working_day FROM doctor_schedules WHERE doctor_id = ? ORDER BY day_of_week, start_time",
				doctorID,
			)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"error": "Ошибка при получении расписания",
				})
				return
			}
			defer rows.Close()

			var schedules []DoctorSchedule
			for rows.Next() {
				var s DoctorSchedule
				if err := rows.Scan(&s.ID, &s.DoctorID, &s.DayOfWeek, &s.StartTime, &s.EndTime, &s.IsWorkingDay); err != nil {
					c.HTML(http.StatusInternalServerError, "error.html", gin.H{
						"error": "Ошибка при сканировании данных расписания",
					})
					return
				}
				schedules = append(schedules, s)
			}

			c.HTML(http.StatusOK, "admin_doctor_schedule.html", gin.H{
				"doctor":    doctor,
				"schedules": schedules,
			})
			return
		}

		if c.Request.Method == http.MethodPost {
			// Добавление записи в расписание
			dayOfWeek, err := strconv.Atoi(c.PostForm("day_of_week"))
			if err != nil {
				c.HTML(http.StatusBadRequest, "error.html", gin.H{
					"error": "Неверный день недели",
				})
				return
			}
			startTime := c.PostForm("start_time")
			endTime := c.PostForm("end_time")
			isWorkingDay := c.PostForm("is_working_day") == "on"

			_, err = db.Exec(
				"INSERT INTO doctor_schedules (doctor_id, day_of_week, start_time, end_time, is_working_day) VALUES (?, ?, ?, ?, ?)",
				doctorID, dayOfWeek, startTime, endTime, isWorkingDay,
			)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"error": "Ошибка при добавлении расписания",
				})
				return
			}

			c.Redirect(http.StatusSeeOther, "/admin/doctors/"+strconv.FormatInt(doctorID, 10)+"/schedule")
			return
		}
	}
}

// adminDeleteScheduleHandler обрабатывает удаление записи из расписания
func adminDeleteScheduleHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.HTML(http.StatusMethodNotAllowed, "error.html", gin.H{
				"error": "Метод не поддерживается",
			})
			return
		}

		// Получаем ID врача и ID записи из URL
		doctorID, err := strconv.ParseInt(c.Param("doctor_id"), 10, 64)
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Неверный ID врача",
			})
			return
		}
		scheduleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Неверный ID записи расписания",
			})
			return
		}

		// Удаляем запись из расписания
		_, err = db.Exec("DELETE FROM doctor_schedules WHERE id = ? AND doctor_id = ?", scheduleID, doctorID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Ошибка при удалении записи расписания",
			})
			return
		}

		c.Redirect(http.StatusSeeOther, "/admin/doctors/"+strconv.FormatInt(doctorID, 10)+"/schedule")
	}
}
